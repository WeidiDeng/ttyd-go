package ttyd

import (
	_ "embed"
	"io"
	"net/http"
	"os/exec"

	"github.com/gobwas/ws"
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
	cmd      *exec.Cmd
	upgrader *ws.HTTPUpgrader
}

// A HandlerOption sets an option on a handler.
type HandlerOption func(*Handler)

// NewHandler returns a new Handler with specified options applied.
// cmd mustn't be nil.
func NewHandler(cmd *exec.Cmd, options ...HandlerOption) *Handler {
	h := &Handler{
		cmd: cmd,
		upgrader: &ws.HTTPUpgrader{
			Protocol: wsProtocol,
		},
	}
	for _, option := range options {
		option(h)
	}
	return h
}

// ServeHTTP upgrades the HTTP connection to a WebSocket connection and serve ttyd protocol on it.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
}
