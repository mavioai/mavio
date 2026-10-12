package thumbnails

import (
	"os/exec"
	"syscall"
)

// belowNormalPriorityClass is BELOW_NORMAL_PRIORITY_CLASS.
const belowNormalPriorityClass = 0x00004000

// lowPriority starts cmd in the below normal priority class.
func lowPriority(cmd *exec.Cmd) func() {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= belowNormalPriorityClass
	return func() {}
}
