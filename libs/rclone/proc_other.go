//go:build !linux

package rclone

import "os/exec"

// configure has nothing to set outside Linux; Close stops the rcd.
func configure(*exec.Cmd) {}
