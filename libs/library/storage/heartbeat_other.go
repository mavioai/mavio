//go:build !linux && !darwin && !windows

package storage

import "os"

// uncachedRead reads buf at offset; this platform has no way to bypass
// the page cache.
func uncachedRead(path string, offset int64, buf []byte) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.ReadAt(buf, offset)
	return err
}
