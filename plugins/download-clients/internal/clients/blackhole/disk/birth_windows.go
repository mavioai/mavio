//go:build windows

package disk

import (
	"io/fs"
	"syscall"
	"time"
)

func creationTime(fi fs.FileInfo) (time.Time, bool) {
	d, ok := fi.Sys().(*syscall.Win32FileAttributeData)
	if !ok {
		return time.Time{}, false
	}
	return time.Unix(0, d.CreationTime.Nanoseconds()), true
}
