package identity

import "strings"

// runtimeCgroup reports whether a /proc/1/cgroup names a container
// runtime, as cgroup v1 paths and v2 paths without a cgroup namespace do.
func runtimeCgroup(cgroup string) bool {
	for _, line := range strings.Split(cgroup, "\n") {
		_, path, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		_, path, _ = strings.Cut(path, ":")
		for _, r := range []string{"docker", "kubepods", "containerd", "libpod", "crio", "lxc", "garden"} {
			if strings.Contains(path, r) {
				return true
			}
		}
	}
	return false
}

// overlayRoot reports whether a /proc/self/mountinfo mounts / from an
// overlay, as container runtimes mount images.
func overlayRoot(mountinfo string) bool {
	for _, line := range strings.Split(mountinfo, "\n") {
		fields := strings.Fields(line)
		// The mount point is the fifth field; the file system type follows
		// the separator "-".
		if len(fields) < 5 || fields[4] != "/" {
			continue
		}
		for i, f := range fields {
			if f == "-" && i+1 < len(fields) {
				return fields[i+1] == "overlay"
			}
		}
	}
	return false
}
