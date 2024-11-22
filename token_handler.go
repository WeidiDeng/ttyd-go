package ttyd

import (
	"encoding/base64"
	"net/http"
	"os/exec"
)

// AuthenticationManger is used to handle WebSocket authentication tokens.
type AuthenticationManger struct {
	TokenHandler
}

// ServeToken serves the token for the WebSocket authentication.
func (am *AuthenticationManger) ServeToken(w http.ResponseWriter, r *http.Request) {
	token, ok := am.Authenticate(w, r)
	if !ok {
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write([]byte("{\"token\": \"" + token + "\"}"))
}

// TokenHandler manages WebSocket authentication tokens
type TokenHandler interface {
	// Authenticate determines if the request is eligible.
	// If the request is not eligible, implementations should write an appropriate response.
	Authenticate(w http.ResponseWriter, r *http.Request) (string, bool)
	// GetCommand returns the command to be executed for the given token.
	// It is nil if the token is invalid.
	GetCommand(token string) *exec.Cmd
}

// NewUnprotectedTokenHandler returns a new TokenHandler that does not require authentication and will always execute the same command.
// You can still check the request headers to determine if the request is eligible in your handler.
func NewUnprotectedTokenHandler(cmd *exec.Cmd) TokenHandler {
	if cmd == nil {
		panic("cmd must not be nil")
	}
	return &unprotectedTokenHandler{cmd: cmd}
}

type unprotectedTokenHandler struct {
	cmd *exec.Cmd
}

func (u *unprotectedTokenHandler) Authenticate(w http.ResponseWriter, r *http.Request) (string, bool) {
	return "", true
}

func (u *unprotectedTokenHandler) GetCommand(token string) *exec.Cmd {
	return u.cmd
}

// NewBasicAuthTokenHandler returns a new TokenHandler that authenticates the request with the basic authentication and will
// always execute the same command
func NewBasicAuthTokenHandler(username, password string, cmd *exec.Cmd) TokenHandler {
	if cmd == nil {
		panic("cmd must not be nil")
	}
	return &basicAuthTokenHandler{user: username, pass: password, token: base64.StdEncoding.EncodeToString([]byte(username + ":" + password)), cmd: cmd}
}

type basicAuthTokenHandler struct {
	user, pass, token string
	cmd               *exec.Cmd
}

func (b *basicAuthTokenHandler) Authenticate(w http.ResponseWriter, r *http.Request) (string, bool) {
	user, pass, ok := r.BasicAuth()
	if ok && user == b.user && pass == b.pass {
		return b.token, true
	}
	w.Header().Set("WWW-Authenticate", "Basic realm=\"ttyd\"")
	http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
	return "", false
}

func (b *basicAuthTokenHandler) GetCommand(token string) *exec.Cmd {
	if token == b.token {
		return b.cmd
	}
	return nil
}

// NewProxyAuthenticationTokenHandler returns a new TokenHandler that authenticates the request with the proxy authentication,
// that is request with the specified header behind a reverse proxy. It should only be used when ttyd is listening on a unix socket.
func NewProxyAuthenticationTokenHandler(authHeader string, cmd *exec.Cmd) TokenHandler {
	if cmd == nil {
		panic("cmd must not be nil")
	}
	return &proxyAuthenticationTokenHandler{authHeader: authHeader, cmd: cmd}
}

type proxyAuthenticationTokenHandler struct {
	authHeader string
	cmd        *exec.Cmd
}

func (p *proxyAuthenticationTokenHandler) Authenticate(w http.ResponseWriter, r *http.Request) (string, bool) {
	if r.Header.Get(p.authHeader) != "" {
		return "", true
	}
	w.Header().Set("Proxy-Authenticate", "Basic realm=\"ttyd\"")
	http.Error(w, http.StatusText(http.StatusProxyAuthRequired), http.StatusProxyAuthRequired)
	return "", false
}

func (p *proxyAuthenticationTokenHandler) GetCommand(token string) *exec.Cmd {
	return p.cmd
}
