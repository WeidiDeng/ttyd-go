package ttyd

import "github.com/gobwas/httphead"

const (
	input          = '0'
	resizeTerminal = '1'
	pause          = '2'
	resume         = '3'
	jsonData       = '{'

	output         = '0'
	setWindowTitle = '1'
	setPreference  = '2'
)

var (
	compressionReadTail = []byte{
		0, 0, 0xff, 0xff, 1, 0, 0, 0xff, 0xff,
	}
	dummyNegotiate = func(httphead.Option) (httphead.Option, error) {
		return httphead.Option{}, nil
	}
)

type resizeRequest struct {
	Columns uint16 `json:"columns"`
	Rows    uint16 `json:"rows"`
}
