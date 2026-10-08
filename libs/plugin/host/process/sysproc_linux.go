package process

import (
	"os/exec"
	"syscall"
)

// configure makes the kernel kill the plugin if the host dies.
func configure(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
}
