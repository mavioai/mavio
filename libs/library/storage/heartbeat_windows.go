//go:build windows

package storage

import (
	"golang.org/x/sys/windows"
)

// uncachedRead reads buf at offset with FILE_FLAG_NO_BUFFERING, bypassing
// the system cache.
func uncachedRead(path string, offset int64, buf []byte) error {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	h, err := windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_FLAG_NO_BUFFERING|windows.FILE_FLAG_OPEN_NO_RECALL, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	ov := windows.Overlapped{Offset: uint32(offset), OffsetHigh: uint32(offset >> 32)}
	var n uint32
	err = windows.ReadFile(h, buf, &n, &ov)
	if err == windows.ERROR_HANDLE_EOF {
		return nil
	}
	return err
}
