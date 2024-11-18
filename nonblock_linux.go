//go:build linux

package ttyd

import "os"

func setNonblock(file *os.File) error {
	return syscall.SetNonblock(int(d.file.Fd()), true)
}
