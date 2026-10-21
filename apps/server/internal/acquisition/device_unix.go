//go:build darwin || linux || freebsd

package acquisition

import "syscall"

// sameDevice reports whether two paths are on one volume, which hard
// links need; ok is false when either cannot be read.
func sameDevice(a, b string) (same, ok bool) {
	var sa, sb syscall.Stat_t
	if syscall.Stat(a, &sa) != nil || syscall.Stat(b, &sb) != nil {
		return false, false
	}
	return sa.Dev == sb.Dev, true
}
