//go:build !unix && !windows

package rclone

import "os"

// lockFile takes no lock where the system has none.
func lockFile(*os.File) (bool, error) { return true, nil }
