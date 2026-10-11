//go:build darwin || linux || freebsd

package acquisition

import (
	"math"
	"path/filepath"
	"syscall"
)

// freeSpace returns the bytes free to unprivileged users on the volume
// of path, or of its nearest existing parent; unknown reports false.
func freeSpace(path string) (int64, bool) {
	if path == "" {
		return 0, false
	}
	p := filepath.Clean(path)
	for {
		var st syscall.Statfs_t
		if err := syscall.Statfs(p, &st); err == nil {
			avail, size := nonNegative(st.Bavail), nonNegative(st.Bsize)
			if size > 0 && avail > math.MaxInt64/size {
				return math.MaxInt64, true
			}
			return avail * size, true
		}
		parent := filepath.Dir(p)
		if parent == p {
			return 0, false
		}
		p = parent
	}
}

// nonNegative returns a statfs field, whose type differs by platform, as
// an int64, clamped to [0, MaxInt64].
func nonNegative[T ~int32 | ~int64 | ~uint32 | ~uint64](v T) int64 {
	switch {
	case v <= 0:
		return 0
	case uint64(v) > math.MaxInt64:
		return math.MaxInt64
	}
	return int64(v)
}
