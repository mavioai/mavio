package identity

import (
	"os"
	"strings"
)

// machineID returns Linux's machine-id, or empty in a container, whose
// machine-id is its image's or none, or when the system has none.
func machineID() string {
	if inContainer() {
		return ""
	}
	for _, f := range []string{"/etc/machine-id", "/var/lib/dbus/machine-id"} {
		if b, err := os.ReadFile(f); err == nil && usable(string(b)) {
			return strings.TrimSpace(string(b))
		}
	}
	return ""
}

// inContainer reports whether the server runs in a Docker or Podman
// container, or one that names itself in $container, as systemd-nspawn,
// LXC and Podman do.
func inContainer() bool {
	if os.Getenv("container") != "" {
		return true
	}
	for _, f := range []string{"/.dockerenv", "/run/.containerenv"} {
		if _, err := os.Stat(f); err == nil {
			return true
		}
	}
	return false
}
