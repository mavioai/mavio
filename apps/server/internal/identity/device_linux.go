package identity

import (
	"os"
	"strings"
)

// machineID returns Linux's machine-id, or empty when the system has none
// or in a container, whose machine-id is its image's, the same in every
// container made from it, or none: the server's ID stands for it instead.
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

// inContainer reports whether the server runs in a container: one Docker
// or Podman marks, one that names itself in $container (systemd-nspawn,
// LXC, Podman), a pod of Kubernetes, one whose first process's cgroup
// names its runtime, or one whose root is an overlay, as images are
// mounted. Taking a host for a container only costs it the hardware ID.
func inContainer() bool {
	if os.Getenv("container") != "" || os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
		return true
	}
	for _, f := range []string{"/.dockerenv", "/run/.containerenv"} {
		if _, err := os.Stat(f); err == nil {
			return true
		}
	}
	if b, err := os.ReadFile("/proc/1/cgroup"); err == nil && runtimeCgroup(string(b)) {
		return true
	}
	if b, err := os.ReadFile("/proc/self/mountinfo"); err == nil && overlayRoot(string(b)) {
		return true
	}
	return false
}
