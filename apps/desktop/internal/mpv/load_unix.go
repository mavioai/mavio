//go:build darwin || linux

package mpv

import "github.com/ebitengine/purego"

// openLibrary loads a shared library with its symbols global, so that
// libmpv finds the libraries loaded before it, as MoltenVK.
func openLibrary(path string) (uintptr, error) {
	return purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_GLOBAL)
}
