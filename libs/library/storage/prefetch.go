package storage

import (
	"crypto/rand"
	"fmt"
	"math/big"
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

	return prefetchPlatform(f, size, headBudget, tailBudget)
}

// VolumeHeartbeat issues an un-cached 4KB micro-read at a random offset to prevent drive spin-down.
func VolumeHeartbeat(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("heartbeat open %s: %w", path, err)
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return fmt.Errorf("heartbeat stat %s: %w", path, err)
	}
	size := fi.Size()
	if size <= 0 {
		return nil
	}

	var offset int64
	maxOffset := size - 4096
	if maxOffset > 0 {
		n, err := rand.Int(rand.Reader, big.NewInt(maxOffset))
		if err == nil {
			offset = (n.Int64() / 4096) * 4096
		}
	}

	buf := make([]byte, 4096)
	_, err = f.ReadAt(buf, offset)
	if err != nil {
		return fmt.Errorf("heartbeat read %s: %w", path, err)
	}
	return nil
}
