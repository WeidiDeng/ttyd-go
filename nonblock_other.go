//go:build !linux

package ttyd

import "os"

func setNonblock(*os.File) error {
	return nil
}
