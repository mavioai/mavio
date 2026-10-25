//go:build unix

package rclone

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

// lockFile takes the exclusive lock of an open file without waiting;
// ok is false when another process holds it.
func lockFile(f *os.File) (ok bool, err error) {
	err = unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	if errors.Is(err, unix.EWOULDBLOCK) {
		return false, nil
	}
	return err == nil, err
}
