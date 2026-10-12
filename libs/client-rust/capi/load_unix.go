//go:build darwin || linux

package capi

import "github.com/ebitengine/purego"

func openLibrary(path string) (uintptr, error) {
	return purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_LOCAL)
}

func register(fn any, handle uintptr, name string) {
	purego.RegisterLibFunc(fn, handle, name)
}

func newCallback(fn func(context uintptr, message *byte, length uintptr) uintptr) uintptr {
	return purego.NewCallback(fn)
}
