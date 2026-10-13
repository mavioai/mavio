//go:build !(darwin || linux || freebsd)

package rclone

// diskSpace reports the space of the disk unknown where it is not read.
func diskSpace(string) (free, total int64, ok bool) { return 0, 0, false }
