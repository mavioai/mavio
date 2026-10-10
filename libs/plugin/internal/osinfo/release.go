package osinfo

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// readRelease reads the files of dir named "*release", such as
// os-release, as Sonarr's ReleaseFileVersionAdapter does: the last ID is
// the name and the last VERSION_ID the version, values as written.
func readRelease(dir string) (name, version string, ok bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", "", false
	}
	name = "Linux"
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), "release") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		for line := range strings.Lines(string(data)) {
			key, value, _ := strings.Cut(strings.TrimRight(line, "\r\n"), "=")
			// Like String.Split('=')[1], the value ends at another '='.
			value, _, _ = strings.Cut(value, "=")
			if strings.TrimSpace(value) == "" {
				continue
			}
			switch key {
			case "ID":
				name, ok = value, true
			case "VERSION_ID":
				version, ok = value, true
			case "PRETTY_NAME":
				ok = true
			}
		}
	}
	return name, version, ok
}

// macVersion matches the product version of a version property list, as
// Sonarr's MacOsVersionAdapter does.
var macVersion = regexp.MustCompile(`(?i)<key>ProductVersion</key>\s*<string>(1\d\.\d{1,2}\.?\d{0,2}?)</string>`)

// readMac reads the version property list of dir, as Sonarr's
// MacOsVersionAdapter does: "macOS" or "macOS Server", and "10.0" unless
// the list holds a version it matches.
func readMac(dir string) (name, version string, ok bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", "", false
	}
	i := slices.IndexFunc(entries, func(e os.DirEntry) bool {
		return e.Name() == "SystemVersion.plist" || e.Name() == "ServerVersion.plist"
	})
	if i < 0 {
		return "", "", false
	}
	name, version = "macOS", "10.0"
	if entries[i].Name() == "ServerVersion.plist" {
		name = "macOS Server"
	}
	if data, err := os.ReadFile(filepath.Join(dir, entries[i].Name())); err == nil {
		if m := macVersion.FindSubmatch(data); m != nil {
			version = string(m[1])
		}
	}
	return name, version, true
}
