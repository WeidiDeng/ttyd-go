package ttyd

import (
	"bufio"
	_ "embed"
	"net"

	"compress/flate"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsflate"
	"net/http"
)

// DefaultHTML is used to serve the default HTML page for the ttyd server.
// Advanced users may choose a different HTML page that implements ttyd protocol.
//
//go:embed static/ttyd.html
var DefaultHTML string

func wsProtocol(string) bool {
	return true
}

// Handler handles each ttyd session.
type Handler struct {
	tokenHandler     TokenHandler
	extension        *wsflate.Extension
	writable         bool
	options          map[string]any
	messageSizeLimit int64
}

// A HandlerOption sets an option on a handler.
type HandlerOption func(*Handler)

// EnableCompressionWithContextTakeover enables compression with context takeover.
func EnableCompressionWithContextTakeover() HandlerOption {
	return EnableCompressionWithExtension(&wsflate.Extension{})
}

// EnableCompressionWithNoContextTakeover enables compression with no context takeover.
func EnableCompressionWithNoContextTakeover() HandlerOption {
	return EnableCompressionWithExtension(&wsflate.Extension{
		Parameters: wsflate.Parameters{
			ServerNoContextTakeover: true,
			ClientNoContextTakeover: true,
		},
	})
}

// EnableCompressionWithExtension enables compression with the specified extension.
// It can be used to set the compression parameter when upgrade is handled manually.
func EnableCompressionWithExtension(extension *wsflate.Extension) HandlerOption {
	return func(h *Handler) {
		h.extension = extension
	}
}

// EnableClientInput enables client inputs to the tty.
func EnableClientInput() HandlerOption {
	return func(h *Handler) {
		h.writable = true
	}
}

// WithClientOptions sets the client options to be sent to the client.
// These options can also be set by the client using the URL query parameters,
// and they have a higher priority than these options.
func WithClientOptions(options map[string]any) HandlerOption {
	return func(h *Handler) {
		h.options = options
	}
}

// WithMessageSizeLimit sets the maximum size of messages that can be sent to the server.
// Zero or negative value means no limit.
func WithMessageSizeLimit(limit int64) HandlerOption {
	return func(h *Handler) {
		h.messageSizeLimit = limit
	}
}

// NewHandler returns a new Handler with specified options applied.
// tokenHandler mustn't be nil.
// By default, client input is not forwarded to the tty and no compression is negotiated and no message size limit.
func NewHandler(tokenHandler TokenHandler, options ...HandlerOption) *Handler {
	h := &Handler{
		tokenHandler: tokenHandler,
	}
	for _, option := range options {
		option(h)
	}
	return h
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

// HandleTTYD handles a websocket connection upgraded through other means. Normally NewHandler should be used instead.
func (h *Handler) HandleTTYD(conn net.Conn, brw *bufio.ReadWriter) {
	d := &daemon{
		conn: &wsConn{
			brw:  brw,
			conn: conn,
		},
		tokenHandler:     h.tokenHandler,
		resume:           make(chan struct{}),
		writable:         h.writable,
		options:          h.options,
		messageSizeLimit: h.messageSizeLimit,
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
