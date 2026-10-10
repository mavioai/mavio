//go:build !linux

package storage

import (
	"errors"
	"io"
	"os"
)

// prefetchRanges reads the ranges in 128 KB chunks, which leaves them in
// the page cache.
func prefetchRanges(f *os.File, ranges []byteRange) error {
	buf := make([]byte, 128<<10)
	for _, r := range ranges {
		for done := int64(0); done < r.n; {
			n, err := f.ReadAt(buf[:min(int64(len(buf)), r.n-done)], r.off+done)
			done += int64(n)
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return err
			}
		}
	}
	return nil
}
