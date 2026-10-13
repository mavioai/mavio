//go:build !linux && !darwin && !windows

package rclone

import (
	"errors"
	"os"
)

// punchHole reports that ranges of files are not freed here.
func punchHole(*os.File, int64, int64) error { return errors.ErrUnsupported }

// markSparse makes f sparse where that needs asking: not here.
func markSparse(*os.File) error { return nil }
