package rclone

import (
	"errors"
	"os"
	"syscall"
	"unsafe"
)

// fPunchHole is fcntl(2)'s F_PUNCHHOLE.
const fPunchHole = 99

// fpunchholeT is fcntl(2)'s fpunchhole_t.
type fpunchholeT struct {
	flags, reserved uint32
	offset, length  int64
}

// punchHole frees the n bytes at off of f, which read as zeros after;
// errors.ErrUnsupported when the file system cannot. off and n are
// multiples of the file system's block size.
func punchHole(f *os.File, off, n int64) error {
	args := fpunchholeT{offset: off, length: n}
	_, _, errno := syscall.Syscall(syscall.SYS_FCNTL, f.Fd(), fPunchHole, uintptr(unsafe.Pointer(&args)))
	switch errno {
	case 0:
		return nil
	case syscall.ENOTSUP, syscall.EINVAL:
		return errors.ErrUnsupported
	}
	return errno
}

// markSparse makes f sparse where that needs asking: not on macOS.
func markSparse(*os.File) error { return nil }
