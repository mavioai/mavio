//go:build darwin || linux || freebsd

package rclone

import (
	"math"
	"syscall"
)

// diskSpace returns the bytes free to unprivileged users and the bytes
// of the volume of dir; unknown reports false.
func diskSpace(dir string) (free, total int64, ok bool) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(dir, &st); err != nil {
		return 0, 0, false
	}
	size := nonNegative(st.Bsize)
	return mul(nonNegative(st.Bavail), size), mul(nonNegative(st.Blocks), size), true
}

// mul multiplies non-negative a and b, clamped to MaxInt64.
func mul(a, b int64) int64 {
	if b > 0 && a > math.MaxInt64/b {
		return math.MaxInt64
	}
	return a * b
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
