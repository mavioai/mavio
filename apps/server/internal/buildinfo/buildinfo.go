// Package buildinfo reports the version of the running binary.
package buildinfo

import "runtime/debug"

// version is set at link time with -ldflags "-X ...buildinfo.version=v1.2.3".
var version string

// Version returns the release version, or the VCS revision for development
// builds, or "dev" when neither is known.
func Version() string {
	if version != "" {
		return version
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}
	for _, s := range info.Settings {
		if s.Key == "vcs.revision" && len(s.Value) >= 12 {
			return s.Value[:12]
		}
	}
	return "dev"
}
