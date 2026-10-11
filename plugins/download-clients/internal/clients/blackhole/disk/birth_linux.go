//go:build linux

package disk

import (
	"io/fs"
	"syscall"
	"time"
)

// creationTime falls back as .NET does without a birth time: the earlier
// of the change and modification times.
func creationTime(fi fs.FileInfo) (time.Time, bool) {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return time.Time{}, false
	}
	ctime, mtime := time.Unix(st.Ctim.Unix()), time.Unix(st.Mtim.Unix())
	if ctime.Before(mtime) {
		return ctime, true
	}
	return mtime, true
}
