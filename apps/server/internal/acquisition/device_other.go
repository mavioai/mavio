//go:build !(darwin || linux || freebsd)

package acquisition

import "path/filepath"

// sameDevice reports whether two paths are on one volume, which hard
// links need, by their volume names where devices are not read.
func sameDevice(a, b string) (same, ok bool) {
	va, vb := filepath.VolumeName(a), filepath.VolumeName(b)
	if va == "" || vb == "" {
		return false, false
	}
	return va == vb, true
}
