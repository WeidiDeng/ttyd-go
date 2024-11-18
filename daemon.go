package ttyd

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"

	"github.com/creack/pty"
	"github.com/gobwas/ws"
)

type daemon struct {
	conn *wsConn
	cmd  *exec.Cmd
	file *os.File

	paused atomic.Bool
	resume chan struct{}
	ioErr  atomic.Bool
}

func (d *daemon) cleanup() {
	if d.ioErr.CompareAndSwap(false, true) {
		d.conn.Close()
		if d.file != nil {
			_ = d.file.Close()
			_ = d.cmd.Wait()
		}
	}
}

func (d *daemon) initWrite() error {
	d.conn.wb.Grow(d.conn.brw.Writer.Size() - ws.MaxHeaderSize)
	hostname, _ := os.Hostname()
	d.conn.wb.WriteByte(setWindowTitle)
	d.conn.wb.WriteString(strings.Join(d.cmd.Args, " "))
	d.conn.wb.WriteString(" (")
	d.conn.wb.WriteString(hostname)
	d.conn.wb.WriteByte(')')
	_, err := d.conn.wb.WriteTo(d.conn)
	if err != nil {
		return err
	}

	d.conn.wb.WriteByte(setPreference)
	d.conn.wb.WriteString("{ }")
	_, err = d.conn.wb.WriteTo(d.conn)
	return err
}

func (d *daemon) readLoop() {
	err := d.initWrite()
	if err != nil {
		return
	}
	d.conn.lr.R = d.conn.brw
	for !d.ioErr.Load() {
		d.conn.rb.Reset()
		for d.conn.rb.Len() == 0 {
			err = d.conn.nextFrame()
			if err != nil {
				return
			}

			err = d.conn.readFrame()
			if err != nil {
				return
			}
		}

		cmd, _ := d.conn.rb.ReadByte()
		if (cmd == jsonData && d.file != nil) || (cmd != jsonData && d.file == nil) {
			continue
		}

		switch cmd {
		case input:
			_, err = d.conn.rb.WriteTo(d.file)
			if err != nil {
				return
			}
		case resizeTerminal:
			var rr resizeRequest
			err = json.NewDecoder(&d.conn.rb).Decode(&rr)
			if err != nil {
				return
			}

			err = pty.Setsize(d.file, &pty.Winsize{
				Rows: rr.Rows,
				Cols: rr.Columns,
			})
			if err != nil {
				return
			}

			err = setNonblock(d.file)
			if err != nil {
				return
			}
		case pause:
			d.paused.Store(true)
		case resume:
			d.paused.Store(false)
			select {
			case d.resume <- struct{}{}:
			default:
			}
		case jsonData:
			_ = d.conn.rb.UnreadByte()
			var rr resizeRequest
			err = json.NewDecoder(&d.conn.rb).Decode(&rr)
			if err != nil {
				return
			}

			d.file, err = pty.StartWithSize(d.cmd, &pty.Winsize{
				Rows: rr.Rows,
				Cols: rr.Columns,
			})
			if err != nil {
				return
			}

			err = setNonblock(d.file)
			if err != nil {
				return
			}
			go d.writeLoop()
		}
	}
}

func (d *daemon) writeLoop() {
	buf := d.conn.wb.Bytes()[:d.conn.wb.Cap()]
	for !d.ioErr.Load() {
		buf[0] = output
		n, err := d.file.Read(buf[1:])
		if err != nil {
			break
		}

		_, err = d.conn.Write(buf[:1+n])
		if err != nil {
			break
		}

		if d.paused.Load() {
			<-d.resume
		}
	}
	d.cleanup()
}