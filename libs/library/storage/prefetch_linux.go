//go:build linux

package storage

import (
	"os"

	"golang.org/x/sys/unix"
)

// prefetchRanges asks the kernel to read the ranges ahead, without
// waiting for them or buffering them in the process.
func prefetchRanges(f *os.File, ranges []byteRange) error {
	fd := int(f.Fd())
	for _, r := range ranges {
		_ = unix.Fadvise(fd, r.off, r.n, unix.FADV_WILLNEED)
	}
	return nil
}
