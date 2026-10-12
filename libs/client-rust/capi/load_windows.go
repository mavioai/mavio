//go:build windows

package capi

import (
	"syscall"

	"github.com/ebitengine/purego"
)

func openLibrary(path string) (uintptr, error) {
	h, err := syscall.LoadLibrary(path)
	return uintptr(h), err
}

func register(fn any, handle uintptr, name string) {
	purego.RegisterLibFunc(fn, handle, name)
}

func newCallback(fn func(context uintptr, message *byte, length uintptr) uintptr) uintptr {
	return syscall.NewCallback(fn)
}
