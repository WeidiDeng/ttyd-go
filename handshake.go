package ttyd

import (
	"bufio"
	_ "embed"
	"io"
	"net"
	"os/exec"

	"compress/flate"
	"net/http"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsflate"
)

// DefaultHTML is used to serve the default HTML page for the ttyd server.
// Advanced users may choose a different HTML page that implements ttyd protocol.
//
//go:embed static/ttyd.html
var DefaultHTML string

// DefaultTokenHandlerFunc is used to serve the default token for the ttyd server.
// ttyd protocol requires a token to be sent in the first message, but there are
// other ways to authenticate the client, such as using the URL query parameters
// and standard HTTP authentications.
func DefaultTokenHandlerFunc(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = io.WriteString(w, "{\"token\": \"\"}")
}

func wsProtocol(string) bool {
	return true
}

// Handler handles each ttyd session.
type Handler struct {
	cmd              *exec.Cmd
	extension        *wsflate.Extension
	writable         bool
	options          map[string]any
	messageSizeLimit int64
	compressionLevel int
	title            string
}

// ServeHTTP upgrades the HTTP connection to a WebSocket connection and serve ttyd protocol on it.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	upgrader := &ws.HTTPUpgrader{
		Protocol: wsProtocol,
	}
	if h.extension != nil {
		upgrader.Negotiate = h.extension.Negotiate
	}
	conn, brw, _, err := upgrader.Upgrade(r, w)
	if err != nil {
		return
	}

	h.HandleTTYD(conn, brw)
}

// HandleTTYD handles a WebSocket connection upgraded through other means. Normally NewHandler should be used instead.
// Provided bufio.ReadReadWriter should have buffers with the size of at least 512.
// The writer buffer size will also impact how much data is read from the process per read operation.
func (h *Handler) HandleTTYD(conn net.Conn, brw *bufio.ReadWriter) {
	d := &daemon{
		conn: &wsConn{
			brw:  brw,
			conn: conn,
		},
		cmd:              h.cmd,
		resume:           make(chan struct{}),
		writable:         h.writable,
		options:          h.options,
		messageSizeLimit: h.messageSizeLimit,
		title:            h.title,
	}

	if h.extension != nil {
		e, accepted := h.extension.Accepted()
		d.conn.e = e
		d.conn.accepted = accepted

		if accepted {
			level := h.compressionLevel
			if level < -2 || level > 9 || level == flate.NoCompression {
				level = flate.DefaultCompression
			}

			d.conn.fr = flate.NewReader(&d.conn.lr)
			d.conn.r = d.conn.fr.(flate.Resetter)
			d.conn.fw, _ = flate.NewWriter(&d.conn.wb, level)
		}
	}

	d.readLoop()
	close(d.resume)
	d.cleanup()
}
