//go:build unix

package library

import (
	"fmt"
	"io/fs"
	"syscall"
)

// fileID identifies a file by device and inode, which renaming keeps and
// replacing changes.
func fileID(info fs.FileInfo) string {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return ""
	}
	// Dev's type differs between systems.
	return fmt.Sprintf("%d:%d", st.Dev, st.Ino)
}
