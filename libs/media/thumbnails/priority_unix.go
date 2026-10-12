//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package thumbnails

import (
	"os/exec"
	"syscall"
)

// lowPriority returns what lowers the priority of cmd once it started:
// a nice value of 10.
func lowPriority(cmd *exec.Cmd) func() {
	return func() {
		// A process that could not be reniced runs as it is.
		_ = syscall.Setpriority(syscall.PRIO_PROCESS, cmd.Process.Pid, 10)
	}
}
