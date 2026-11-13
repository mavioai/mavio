package main

import (
	"runtime/debug"
	"strings"
)

// version is a release's version, v0.1.0 or 0.1.0, linked by the build as
// -X main.version=$MAVIO_VERSION; empty in other builds.
var version string

// appVersion returns the app's version as the server's and the core's
// (docs/development.md §7): a release's, else 0.0.0- and the first 12
// characters of the commit it is built at, else dev.
func appVersion() string {
	return versionOf(version, debug.ReadBuildInfo)
}

// versionOf returns the version of a release's linked version, else of the
// commit the build information records.
func versionOf(linked string, info func() (*debug.BuildInfo, bool)) string {
	if v := strings.TrimPrefix(linked, "v"); v != "" {
		return v
	}
	if bi, ok := info(); ok {
		for _, s := range bi.Settings {
			if s.Key == "vcs.revision" && len(s.Value) >= 12 {
				return "0.0.0-" + s.Value[:12]
			}
		}
	}
	return "dev"
}
