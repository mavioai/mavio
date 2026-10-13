package rclone

import (
	"errors"
	"os"
	"syscall"
	"unsafe"
)

// Controls of DeviceIoControl.
const (
	fsctlSetSparse   = 0x000900C4
	fsctlSetZeroData = 0x000980C8
)

// errInvalidFunction is what a file system without sparse files answers.
const errInvalidFunction = syscall.Errno(1)

// punchHole frees the n bytes at off of f, sparse, which read as zeros
// after; errors.ErrUnsupported when the file system cannot.
func punchHole(f *os.File, off, n int64) error {
	// FILE_ZERO_DATA_INFORMATION
	in := struct{ offset, beyond int64 }{off, off + n}
	var done uint32
	err := syscall.DeviceIoControl(syscall.Handle(f.Fd()), fsctlSetZeroData,
		(*byte)(unsafe.Pointer(&in)), uint32(unsafe.Sizeof(in)), nil, 0, &done, nil)
	if errors.Is(err, errInvalidFunction) {
		return errors.ErrUnsupported
	}
	return err
}

// markSparse makes f sparse, so that its unwritten ranges and the ranges
// punchHole frees take no disk space. A file system without sparse files
// leaves f as it is.
func markSparse(f *os.File) error {
	var done uint32
	err := syscall.DeviceIoControl(syscall.Handle(f.Fd()), fsctlSetSparse, nil, 0, nil, 0, &done, nil)
	if errors.Is(err, errInvalidFunction) {
		return nil
	}
	return err
}
