//go:build unix

package library

import (
	"io/fs"
	"strconv"
	"syscall"
)

// fileID identifies a file by device and inode, which renaming keeps and
// replacing changes.
func fileID(info fs.FileInfo) string {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return ""
	}
	return strconv.FormatUint(uint64(st.Dev), 10) + ":" + strconv.FormatUint(st.Ino, 10)
}
