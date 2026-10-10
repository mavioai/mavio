package storage

import (
	"fmt"
	"os"
)

// Default budgets matching Khua's headBudget (1MB) and tailBudget (256KB).
const (
	DefaultHeadBudget int64 = 1 << 20   // 1 MB
	DefaultTailBudget int64 = 256 << 10 // 256 KB
)

// PrefetchHeadTail warms the operating system page cache for the beginning and end of a file.
// The head covers container signatures, streams, and tracks (EBML, ftyp, moov).
// The tail covers trailing index structures (MP4 trailing moov, Matroska Cues, AVI idx1).
func PrefetchHeadTail(path string, headBudget, tailBudget int64) error {
	if headBudget <= 0 {
		headBudget = DefaultHeadBudget
	}
	if tailBudget <= 0 {
		tailBudget = DefaultTailBudget
	}

	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("prefetch open %s: %w", path, err)
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return fmt.Errorf("prefetch stat %s: %w", path, err)
	}
	size := fi.Size()
	if size <= 0 {
		return nil
	}

	head := min(headBudget, size)
	ranges := []byteRange{{0, head}}
	if size > head {
		tail := min(tailBudget, size-head)
		ranges = append(ranges, byteRange{size - tail, tail})
	}
	return prefetchRanges(f, ranges)
}

// PrefetchRange warms the page cache for n bytes of a file from offset,
// clipped to the file, such as the media around where a playback resumes.
func PrefetchRange(path string, offset, n int64) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("prefetch open %s: %w", path, err)
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return fmt.Errorf("prefetch stat %s: %w", path, err)
	}
	offset = max(0, min(offset, fi.Size()))
	n = min(n, fi.Size()-offset)
	if n <= 0 {
		return nil
	}
	return prefetchRanges(f, []byteRange{{offset, n}})
}

// byteRange is n bytes from off.
type byteRange struct{ off, n int64 }
