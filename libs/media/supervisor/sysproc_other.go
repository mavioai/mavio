//go:build !linux

package supervisor

import "os/exec"

// configure has nothing to set outside Linux, where the server ends
// ffmpeg itself, and a server started again ends what one killed left.
func configure(*exec.Cmd) {}
