package ssdp

import "syscall"

// soReusePort is SO_REUSEPORT, which package syscall lacks on Linux.
const soReusePort = 0xf

func setReusePort(fd int) error {
	return syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, soReusePort, 1)
}
