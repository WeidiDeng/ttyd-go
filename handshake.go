package ttyd

import (
	"bufio"
	"bytes"
	_ "embed"
	"io"
	"net"
	"os/exec"
	"strings"
	"time"

	"compress/flate"
	"net/http"

	"github.com/gobwas/httphead"
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
	pingInterval     time.Duration
}

// NewHandler returns a new Handler with specified options applied.
// cmd mustn't be nil.
// By default, client input is not forwarded to the tty, no compression is negotiated, message has the size limit of 4096,
// and no ping is sent by the server.
func NewHandler(cmd *exec.Cmd, options ...HandlerOption) *Handler {
	h := &Handler{
		cmd:              cmd,
		messageSizeLimit: 4096,
	}
	for _, option := range options {
		option(h)
	}
	return h
}

// ServeHTTP upgrades the HTTP connection to a WebSocket connection and serve ttyd protocol on it.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var (
		conn net.Conn
		brw  *bufio.ReadWriter
		hs   ws.Handshake
		err  error
	)
	if r.ProtoMajor == 2 && r.Method == http.MethodConnect && r.Header.Get(":protocol") != "" {
		if h.extension != nil {
			if extension := r.Header.Get("Sec-WebSocket-Extensions"); extension != "" {
				options, _ := httphead.ParseOptions([]byte(extension), nil)
				for _, opt := range options {
					if bytes.Equal(opt.Name, wsflate.ExtensionNameBytes) {
						var negotiated httphead.Option
						negotiated, err = h.extension.Negotiate(opt)
						if err != nil {
							return
						}
						hs.Extensions = append(hs.Extensions, negotiated)
					}
				}
			}
		}

		if protocol := r.Header.Get("Sec-WebSocket-Protocol"); protocol != "" {
			hs.Protocol = protocol
			w.Header().Set("Sec-WebSocket-Protocol", protocol)
		}

		if len(hs.Extensions) > 0 {
			var sb strings.Builder
			_, _ = httphead.WriteOptions(&sb, hs.Extensions)
			w.Header().Set("Sec-WebSocket-Extensions", sb.String())
		}

		w.WriteHeader(http.StatusOK)
		err = http.NewResponseController(w).Flush()
		if err != nil {
			return
		}

		localAddr := r.Context().Value(http.LocalAddrContextKey).(net.Addr)
		conn = &http2Conn{
			w:          w,
			ReadCloser: r.Body,
			localAddr:  localAddr,
			remoteAddr: &http2Addr{
				network: localAddr.Network(),
				addr:    r.RemoteAddr,
			},
		}
		brw = bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))
	} else {
		upgrader := &ws.HTTPUpgrader{
			Protocol: wsProtocol,
		}
		if h.extension != nil {
			upgrader.Negotiate = h.extension.Negotiate
		}
		conn, brw, hs, err = upgrader.Upgrade(r, w)
		if err != nil {
			if conn != nil {
				_ = conn.Close()
			}
			return
		}
	}

	h.HandleTTYD(conn, brw, hs)
}

// HandleTTYD handles a WebSocket connection upgraded through other means. Normally NewHandler should be used instead.
// Provided bufio.ReadReadWriter should have buffers with the size of at least 512.
// The writer buffer size will also impact how much data is read from the process per read operation.
func (h *Handler) HandleTTYD(conn net.Conn, brw *bufio.ReadWriter, hs ws.Handshake) {
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

	if len(hs.Extensions) > 0 {
		var (
			e        wsflate.Parameters
			accepted bool
		)
		for _, ext := range hs.Extensions {
			if bytes.Equal(ext.Name, wsflate.ExtensionNameBytes) {
				_ = e.Parse(ext)
				accepted = true
				break
			}
		}

		d.conn.e = e
		d.conn.accepted = accepted

		if accepted {
			level := h.compressionLevel
			if level < -2 || level > 9 || level == flate.NoCompression {
				level = flate.DefaultCompression
			}

			d.conn.fr = flate.NewReader(&d.conn.lr).(flateReader)
			d.conn.fw, _ = flate.NewWriter(&d.conn.wb, level)
		}
	}

	var done chan struct{}
	if h.pingInterval > 0 {
		done = make(chan struct{})
		go d.pingLoop(time.NewTicker(h.pingInterval), done)
	}
	d.readLoop()
	close(d.resume)
	d.cleanup()
	if h.pingInterval > 0 {
		close(done)
	}
}
