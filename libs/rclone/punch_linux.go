package rclone

import (
	"errors"
	"os"
	"syscall"
)

// Modes of fallocate(2).
const (
	fallocKeepSize  = 0x01
	fallocPunchHole = 0x02
)

// punchHole frees the n bytes at off of f, which read as zeros after;
// errors.ErrUnsupported when the file system cannot.
func punchHole(f *os.File, off, n int64) error {
	err := syscall.Fallocate(int(f.Fd()), fallocKeepSize|fallocPunchHole, off, n)
	if errors.Is(err, syscall.EOPNOTSUPP) || errors.Is(err, syscall.ENOSYS) {
		return errors.ErrUnsupported
	}
	return err
}

// markSparse makes f sparse where that needs asking: not on Linux.
func markSparse(*os.File) error { return nil }
