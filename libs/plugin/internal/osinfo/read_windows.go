package osinfo

import (
	"fmt"
	"syscall"
	"unsafe"
)

// osVersionInfo is Windows' RTL_OSVERSIONINFOW.
type osVersionInfo struct {
	size                          uint32
	major, minor, build, platform uint32
	servicePack                   [128]uint16
}

// read returns "Windows" and the version as .NET's Version formats it.
func read() (string, string, bool) {
	info := osVersionInfo{}
	info.size = uint32(unsafe.Sizeof(info))
	proc := syscall.NewLazyDLL("ntdll.dll").NewProc("RtlGetVersion")
	if err := proc.Find(); err != nil {
		return "", "", false
	}
	_, _, _ = proc.Call(uintptr(unsafe.Pointer(&info)))
	return "Windows", fmt.Sprintf("%d.%d.%d.0", info.major, info.minor, info.build), true
}
