// Package osinfo names the host's operating system and its version as
// Servarr applications do in their User-Agent: lowercased, such as
// "ubuntu" and "22.04", "macos" and "10.0", or "windows" and
// "10.0.22631.0".
package osinfo

import (
	"runtime"
	"strings"
	"sync"
)

// Get returns the operating system's name and version; the version is
// empty when unknown.
var Get = sync.OnceValues(func() (string, string) {
	name, version, ok := read()
	if !ok {
		name, version = fallback(runtime.GOOS), ""
	}
	return strings.ToLower(name), strings.ToLower(version)
})

// String is the name and version as the User-Agent holds them.
func String() string {
	name, version := Get()
	return name + " " + version
}

// fallback is .NET's name of an operating system whose version is not
// read.
func fallback(goos string) string {
	switch goos {
	case "windows":
		return "Windows"
	case "darwin":
		return "Osx"
	case "freebsd", "openbsd", "netbsd", "dragonfly":
		return "Bsd"
	}
	return "Linux"
}
