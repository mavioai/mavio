//go:build unix && !linux && !darwin && !dragonfly && !freebsd && !netbsd && !openbsd

package ssdp

func setReusePort(int) error { return nil }
