//go:build !linux

package storage

import (
	"io"
	"os"
)

func prefetchPlatform(f *os.File, size, headBudget, tailBudget int64) error {
	buf := make([]byte, 128<<10) // 128 KB buffer

	// Read head into page cache
	headLen := headBudget
	if headLen > size {
		headLen = size
	}
	var readHead int64
	for readHead < headLen {
		toRead := int64(len(buf))
		if toRead > headLen-readHead {
			toRead = headLen - readHead
		}
		n, err := f.ReadAt(buf[:toRead], readHead)
		if n > 0 {
			readHead += int64(n)
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
	}

	// Read tail into page cache
	if size > headBudget {
		tailLen := tailBudget
		if tailLen > size-headBudget {
			tailLen = size - headBudget
		}
		tailOffset := size - tailLen
		var readTail int64
		for readTail < tailLen {
			toRead := int64(len(buf))
			if toRead > tailLen-readTail {
				toRead = tailLen - readTail
			}
			n, err := f.ReadAt(buf[:toRead], tailOffset+readTail)
			if n > 0 {
				readTail += int64(n)
			}
			if err != nil {
				if err == io.EOF {
					break
				}
				return err
			}
		}
	}

	return nil
}
