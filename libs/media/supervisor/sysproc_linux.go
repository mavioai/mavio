package supervisor

import (
	"os/exec"
	"syscall"
)

// configure makes the kernel kill ffmpeg if the server dies.
func configure(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
}
