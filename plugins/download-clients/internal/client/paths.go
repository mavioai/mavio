package client

import (
	"regexp"
	"strings"
)

// drivePattern matches a Windows path's drive letter.
var drivePattern = regexp.MustCompile(`^[A-Za-z]:`)

// isWindowsPath reports whether a path of a client's machine is Windows'
// rather than Unix', as Radarr's OsPath tells: it has no slashes, and
// backslashes or a drive letter.
func isWindowsPath(p string) bool {
	return !strings.Contains(p, "/") && (strings.Contains(p, `\`) || drivePattern.MatchString(p))
}

// JoinRemote joins a folder of a client's machine, Windows' or not, and a
// relative path, as Radarr's OsPath and Path.Combine do: with the
// folder's separator, and the name alone for no folder.
func JoinRemote(dir, name string) string {
	if dir == "" {
		return name
	}
	sep := "/"
	if isWindowsPath(dir) {
		sep = `\`
	}
	return strings.TrimRight(dir, `/\`) + sep + strings.TrimLeft(name, `/\`)
}

// RemoteBase returns the last element of a path of a client's machine,
// with either separator.
func RemoteBase(p string) string {
	p = strings.TrimRight(p, `/\`)
	if i := strings.LastIndexAny(p, `/\`); i >= 0 {
		return p[i+1:]
	}
	return p
}
