package rclone

import (
	"os/exec"
	"syscall"
)

// configure makes the kernel kill the rcd if the server dies.
func configure(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
}
