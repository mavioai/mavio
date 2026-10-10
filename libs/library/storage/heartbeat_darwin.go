//go:build darwin

package storage

import (
	"os"

	"golang.org/x/sys/unix"
)

// uncachedRead reads buf at offset with F_NOCACHE set, bypassing the
// unified buffer cache.
func uncachedRead(path string, offset int64, buf []byte) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, _ = unix.FcntlInt(f.Fd(), unix.F_NOCACHE, 1)
	_, err = f.ReadAt(buf, offset)
	return err
}
