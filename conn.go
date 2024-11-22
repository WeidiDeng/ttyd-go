package ttyd

import (
	"bufio"
	"bytes"
	"compress/flate"
	"errors"
	"io"
	"net"
	"sync"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsflate"
)

var errFrameTooLarge = errors.New("frame too large")

type wsConn struct {
	brw  *bufio.ReadWriter
	conn net.Conn

	lr io.LimitedReader

	rb bytes.Buffer
	wb bytes.Buffer

	e        wsflate.Parameters
	accepted bool

	sw bytes.Buffer
	fr io.ReadCloser
	r  flate.Resetter
	fw *flate.Writer

	hdr  ws.Header
	lock sync.Mutex
}

func (w *wsConn) Close() {
	w.lock.Lock()
	_, _ = w.conn.Write(ws.CompiledCloseNormalClosure)
	w.lock.Unlock()
	_ = w.conn.Close()
}

func (w *wsConn) handleControl(hdr ws.Header) error {
	data, err := w.brw.Peek(int(hdr.Length))
	if err != nil {
		return err
	}

	switch hdr.OpCode {
	case ws.OpPing:
		ws.Cipher(data, hdr.Mask, 0)
		w.lock.Lock()
		_ = ws.WriteFrame(w.brw, ws.NewPongFrame(data))
		err = w.brw.Flush()
		w.lock.Unlock()
	case ws.OpClose:
		err = io.EOF
	}
	_, _ = w.brw.Discard(int(hdr.Length))
	return err
}

func (w *wsConn) nextFrame() error {
	for {
		hdr, err := ws.ReadHeader(w.brw)
		if err != nil {
			return err
		}

		if !hdr.Masked {
			return ws.ErrProtocolMaskRequired
		}

		if hdr.OpCode.IsControl() {
			err = w.handleControl(hdr)
			if err != nil {
				return err
			}
			continue
		}

		w.hdr = hdr
		return nil
	}
}

func (w *wsConn) readFrame(limit int64) error {
	r1 := w.hdr.Rsv1()
	for {
		idx := w.rb.Len()
		w.lr.N = w.hdr.Length
		if limit > 0 && int64(idx)+w.lr.N > limit {
			return errFrameTooLarge
		}
		_, err := w.rb.ReadFrom(&w.lr)
		if err != nil {
			return err
		}

		ws.Cipher(w.rb.Bytes()[idx:], w.hdr.Mask, 0)
		if w.hdr.Fin {
			break
		}
		err = w.nextFrame()
		if err != nil {
			return err
		}
	}

	if !w.accepted || !r1 {
		return nil
	}

	w.rb.Write(compressionReadTail)
	_ = w.r.Reset(&w.rb, w.sw.Bytes())

	n, err := w.rb.ReadFrom(w.fr)
	if err != nil {
		return err
	}
	if remaining := w.rb.Len() - int(n); remaining > 0 {
		w.sw.Reset()
		w.rb.Next(remaining)
	}
	if !w.e.ClientNoContextTakeover {
		w.sw.Next(max(w.sw.Len()+w.rb.Len()-32768, 0))
		w.sw.Write(w.rb.Bytes())
	}
	return nil
}

func (w *wsConn) Write(p []byte) (n int, err error) {
	w.wb.Reset()
	w.lock.Lock()
	var frame ws.Frame
	if !w.accepted {
		frame = ws.NewBinaryFrame(p)
	} else {
		if w.e.ServerNoContextTakeover {
			w.fw.Reset(&w.wb)
		}
		_, _ = w.fw.Write(p)
		if w.e.ServerNoContextTakeover {
			_ = w.fw.Close()
		} else {
			_ = w.fw.Flush()
		}

		frame = ws.NewBinaryFrame(w.wb.Bytes()[:w.wb.Len()-4])
		frame.Header.Rsv = ws.Rsv(true, false, false)
	}
	_ = ws.WriteFrame(w.brw, frame)
	err = w.brw.Flush()
	w.lock.Unlock()
	if err == nil {
		n = len(p)
	}
	return
}
