package ttyd

import (
	"time"

	"github.com/gobwas/ws/wsflate"
)

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
// Caller should make sure the options can be serialized to JSON.
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

// WithCompressionLevel sets the compression level for the flate writer if compression is negotiated with the peer.
// Invalid levels or NoCompression will be treated as default compression level.
func WithCompressionLevel(level int) HandlerOption {
	return func(h *Handler) {
		h.compressionLevel = level
	}
}

// WithTitle sets the title of the terminal. By default, the title is set to the command being run joined with the hostname.
func WithTitle(title string) HandlerOption {
	return func(h *Handler) {
		h.title = title
	}
}

// WithPingInterval sets the interval at which ping frames are sent to clients.
// Zero or negative value disables the sending of pings. It's used to keep the connection alive when ttyd
// is used over a proxy.
func WithPingInterval(interval time.Duration) HandlerOption {
	return func(h *Handler) {
		h.pingInterval = interval
	}
}
