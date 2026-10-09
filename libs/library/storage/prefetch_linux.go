//go:build linux

package storage

import (
	"os"

	"golang.org/x/sys/unix"
)

func prefetchPlatform(f *os.File, size, headBudget, tailBudget int64) error {
	fd := int(f.Fd())

	// Prefetch head
	headLen := headBudget
	if headLen > size {
		headLen = size
	}
	_ = unix.Fadvise(fd, 0, headLen, unix.FADV_WILLNEED)

	// Prefetch tail
	if size > headBudget {
		tailLen := tailBudget
		if tailLen > size-headBudget {
			tailLen = size - headBudget
		}
		tailOffset := size - tailLen
		_ = unix.Fadvise(fd, tailOffset, tailLen, unix.FADV_WILLNEED)
	}

	return nil
}
