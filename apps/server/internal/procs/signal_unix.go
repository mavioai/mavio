//go:build unix

package procs

import (
	"errors"
	"syscall"
)

func terminate(pid int) error { return send(pid, syscall.SIGTERM) }

func kill(pid int) error { return send(pid, syscall.SIGKILL) }

func send(pid int, sig syscall.Signal) error {
	if err := syscall.Kill(pid, sig); errors.Is(err, syscall.ESRCH) {
		return errGone
	} else if err != nil {
		return err
	}
	return nil
}
