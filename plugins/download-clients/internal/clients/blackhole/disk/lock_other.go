//go:build !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !windows

package disk

func isLocked(string) bool { return false }
