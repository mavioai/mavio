package procs

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"

	"golang.org/x/sys/unix"
)

// sZomb is the state of a process that exited and was not waited for.
const sZomb = 5

// identify returns a process's start time and its executable, as the
// kernel tells them.
func identify(pid int) (start, path string, err error) {
	if err := unix.Kill(pid, 0); errors.Is(err, unix.ESRCH) {
		return "", "", errGone
	}
	k, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
	if errors.Is(err, unix.EIO) {
		// The kernel tells of no such process.
		return "", "", errGone
	}
	if err != nil {
		return "", "", err
	}
	if k.Proc.P_stat == sZomb {
		return "", "", errGone
	}
	// The process's arguments: their count, then the executable's path.
	args, err := unix.SysctlRaw("kern.procargs2", pid)
	if err != nil {
		return "", "", err
	}
	if len(args) < 4 {
		return "", "", fmt.Errorf("kern.procargs2 of %d: %d bytes", pid, len(args))
	}
	exe, _, _ := bytes.Cut(args[4:], []byte{0})
	t := k.Proc.P_starttime
	return strconv.FormatInt(t.Sec, 10) + "." + strconv.FormatInt(int64(t.Usec), 10), string(exe), nil
}
