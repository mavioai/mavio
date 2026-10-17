package procs

import "os"

// terminate kills the process: Windows has no SIGTERM to deliver.
func terminate(pid int) error { return kill(pid) }

func kill(pid int) error {
	p, err := os.FindProcess(pid)
	if err != nil {
		return errGone
	}
	defer p.Release()
	return p.Kill()
}
