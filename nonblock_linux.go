//go:build linux

package ttyd

import (
	"os"
	"syscall"
)

func setNonblock(file *os.File) error {
	return syscall.SetNonblock(int(file.Fd()), true)
}
