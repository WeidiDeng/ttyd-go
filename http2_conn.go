package ttyd

import (
	"io"
	"net"
	"net/http"
	"time"
)

type http2Addr struct {
	network string
	addr    string
}

func (h http2Addr) Network() string {
	return h.network
}

func (h http2Addr) String() string {
	return h.addr
}

type http2Conn struct {
	w http.ResponseWriter
	io.ReadCloser

	localAddr  net.Addr
	remoteAddr *http2Addr
}

func (h *http2Conn) Write(b []byte) (n int, err error) {
	_, err = h.w.Write(b)
	if err != nil {
		return 0, err
	}

	err = http.NewResponseController(h.w).Flush()
	if err != nil {
		return 0, err
	}
	return len(b), nil
}

func (h *http2Conn) LocalAddr() net.Addr {
	return h.localAddr
}

func (h *http2Conn) RemoteAddr() net.Addr {
	return h.remoteAddr
}

func (h *http2Conn) SetDeadline(t time.Time) error {
	return http.NewResponseController(h.w).SetReadDeadline(t)
}

func (h *http2Conn) SetReadDeadline(t time.Time) error {
	err := h.SetReadDeadline(t)
	if err != nil {
		return err
	}
	return h.SetWriteDeadline(t)
}

func (h *http2Conn) SetWriteDeadline(t time.Time) error {
	return http.NewResponseController(h.w).SetWriteDeadline(t)
}
