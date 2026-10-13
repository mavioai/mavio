//go:build darwin || linux

package rclone

import (
	"os"
	"syscall"
)

// diskUsage returns the bytes of disk the file at path takes.
func diskUsage(path string) (int64, bool) {
	fi, err := os.Stat(path)
	if err != nil {
		return 0, false
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, false
	}
	return st.Blocks * 512, true
}
