//go:build darwin || dragonfly || freebsd || netbsd || openbsd

package ssdp

import "syscall"

func setReusePort(fd int) error {
	return syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_REUSEPORT, 1)
}
