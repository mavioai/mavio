//go:build windows

package disk

import (
	"errors"
	"syscall"
)

// The errors of opening a file another process holds.
const (
	errorSharingViolation syscall.Errno = 32
	errorLockViolation    syscall.Errno = 33
)

// isLocked opens the file without sharing, as Radarr's IsFileLocked.
func isLocked(path string) bool {
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return false
	}
	h, err := syscall.CreateFile(name, syscall.GENERIC_READ, 0, nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return errors.Is(err, errorSharingViolation) || errors.Is(err, errorLockViolation)
	}
	_ = syscall.CloseHandle(h)
	return false
}
