package ttyd

import (
	_ "embed"

	"compress/flate"
	"io"
	"net/http"
	"os/exec"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsflate"
)

// DefaultHTML is used to serve the default HTML page for the ttyd server.
// Advanced users may choose a different HTML page that implements ttyd protocol.
//
//go:embed static/ttyd.html
var DefaultHTML string

// DefaultTokenHandler is used to serve the default token for the ttyd server.
func DefaultTokenHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = io.WriteString(w, "{\"token\": \"\"}")
}

func wsProtocol(string) bool {
	return true
}

// Handler handles each ttyd session.
type Handler struct {
	cmd       *exec.Cmd
	upgrader  *ws.HTTPUpgrader
	extension *wsflate.Extension
}

// A HandlerOption sets an option on a handler.
type HandlerOption func(*Handler)

// DisableCompression disables the compression negotiation.
func DisableCompression() HandlerOption {
	return func(h *Handler) {
		h.extension = nil
		h.upgrader.Negotiate = nil
	}
}

// NewHandler returns a new Handler with specified options applied.
// cmd mustn't be nil.
// By default, compression with context takeover is enabled.
func NewHandler(cmd *exec.Cmd, options ...HandlerOption) *Handler {
	h := &Handler{
		cmd: cmd,
		upgrader: &ws.HTTPUpgrader{
			Protocol:  wsProtocol,
			Negotiate: dummyNegotiate,
		},
	}
	for _, option := range options {
		option(h)
	}
	if h.upgrader.Negotiate != nil && h.extension == nil {
		h.extension = &wsflate.Extension{}
		h.upgrader.Negotiate = h.extension.Negotiate
	}
	return h
}

// ServeHTTP upgrades the HTTP connection to a WebSocket connection and serve ttyd protocol on it.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, bw, _, err := h.upgrader.Upgrade(r, w)
	if err != nil {
		return
	}

	d := &daemon{
		conn: &wsConn{
			brw:  bw,
			conn: conn,
		},
		cmd:    h.cmd,
		resume: make(chan struct{}),
	}

	if h.extension != nil {
		e, accepted := h.extension.Accepted()
		d.conn.e = e
		d.conn.accepted = accepted

		if accepted {
			d.conn.fr = flate.NewReader(&d.conn.lr)
			d.conn.r = d.conn.fr.(flate.Resetter)
			d.conn.fw, _ = flate.NewWriter(&d.conn.wb, flate.DefaultCompression)
		}
	}

	d.readLoop()
	close(d.resume)
	d.cleanup()
}
