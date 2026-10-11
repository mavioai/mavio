//go:build !darwin && !freebsd && !netbsd && !linux && !windows

package disk

import (
	"io/fs"
	"time"
)

func creationTime(fs.FileInfo) (time.Time, bool) { return time.Time{}, false }
