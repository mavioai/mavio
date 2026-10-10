//go:build linux

package storage

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

// uncachedRead reads buf at offset with O_DIRECT, bypassing the page
// cache. File systems without O_DIRECT, such as some FUSE mounts, get the
// range dropped from the cache and read normally.
func uncachedRead(path string, offset int64, buf []byte) error {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_DIRECT|unix.O_CLOEXEC, 0)
	if err == nil {
		_, err = unix.Pread(fd, buf, offset)
		_ = unix.Close(fd)
		if !errors.Is(err, unix.EINVAL) {
			return err
		}
	} else if !errors.Is(err, unix.EINVAL) {
		return err
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_ = unix.Fadvise(int(f.Fd()), offset, int64(len(buf)), unix.FADV_DONTNEED)
	_, err = f.ReadAt(buf, offset)
	return err
}
