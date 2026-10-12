//go:build !(darwin || dragonfly || freebsd || linux || netbsd || openbsd || windows)

package thumbnails

import "os/exec"

// lowPriority leaves the priority of cmd as it is.
func lowPriority(*exec.Cmd) func() { return func() {} }
