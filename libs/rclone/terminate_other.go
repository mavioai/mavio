//go:build !unix

package rclone

import "os"

// terminate kills the process: Windows has no SIGTERM to deliver.
func terminate(p *os.Process) error { return p.Kill() }
