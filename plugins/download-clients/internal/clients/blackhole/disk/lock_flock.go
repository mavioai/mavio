//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package disk

import (
	"errors"
	"os"
	"syscall"
)

// isLocked takes and releases an exclusive flock, as .NET opens a file
// without sharing on Unix.
func isLocked(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()
	fd := int(f.Fd())
	if err := syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return errors.Is(err, syscall.EWOULDBLOCK)
	}
	_ = syscall.Flock(fd, syscall.LOCK_UN)
	return false
}
