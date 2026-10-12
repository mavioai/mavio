//go:build unix

package rclone

import (
	"os"
	"syscall"
)

func terminate(p *os.Process) error { return p.Signal(syscall.SIGTERM) }
