package rclone

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

// lockFile takes the exclusive lock of an open file without waiting;
// ok is false when another process holds it.
func lockFile(f *os.File) (ok bool, err error) {
	ol := new(windows.Overlapped)
	err = windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, ol)
	if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
		return false, nil
	}
	return err == nil, err
}
