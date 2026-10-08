//go:build !linux

package process

import "os/exec"

// configure has nothing to set outside Linux; Close stops the plugin.
func configure(*exec.Cmd) {}
