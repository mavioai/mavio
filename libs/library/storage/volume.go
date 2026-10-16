package storage

import (
	"bufio"
	"bytes"
	"context"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Volume is a file system mounted on the server's computer: a drive or a
// partition of one, or a share or cloud drive mounted.
type Volume struct {
	// Path is where it is mounted: a mount point, or the root of a drive
	// on Windows, such as `C:\`.
	Path string
	// Name is its label, empty when it has none or none is read.
	Name string
	// FSType is its file system, such as "apfs", "ext4" or "NTFS".
	FSType string
	// Kind is the device it is on, as [DetectDevice] tells; KindUnknown
	// when it did not answer.
	Kind DeviceKind
	// Total is its size in bytes; zero when unknown, as for a share that
	// did not answer in time.
	Total uint64
	// Available is the bytes free to the server; Used is the rest of
	// Total, reserved space and, where volumes share a container as on
	// APFS, what the others hold included.
	Used, Available uint64
}

// volumeTimeout is how long a volume is waited for: a share whose server
// is gone may never answer.
const volumeTimeout = 2 * time.Second

// Volumes lists the volumes of the server's computer worth showing: not
// the kernel's own file systems, nor the system's hidden volumes, nor a
// second mount of the same file system. Volumes that do not answer within
// a couple of seconds are listed without their sizes.
func Volumes(ctx context.Context) ([]Volume, error) {
	return volumes(ctx)
}

// probeAll fills in the volumes with probe at once, waiting for each at
// most volumeTimeout, and returns each one's error: probe's, or the
// context's for one that did not return in time. A probe left running
// fills in a copy, as a blocked system call cannot be cancelled. A probe
// sets Total and Available; Used follows.
func probeAll(ctx context.Context, vs []Volume, probe func(v *Volume) error) []error {
	ctx, cancel := context.WithTimeout(ctx, volumeTimeout)
	defer cancel()
	errs := make([]error, len(vs))
	var wg sync.WaitGroup
	for i := range vs {
		wg.Go(func() {
			done := make(chan error, 1)
			w := vs[i]
			go func() { done <- probe(&w) }()
			select {
			case err := <-done:
				if errs[i] = err; err == nil {
					w.Available = min(w.Available, w.Total)
					w.Used = w.Total - w.Available
					vs[i] = w
				}
			case <-ctx.Done():
				errs[i] = ctx.Err()
			}
		})
	}
	wg.Wait()
	return errs
}

// pseudoFS are the Linux file systems that hold no files of their own:
// the kernel's, memory's and read-only images such as snaps.
var pseudoFS = map[string]bool{
	"autofs": true, "binfmt_misc": true, "bpf": true, "cgroup": true, "cgroup2": true, "configfs": true,
	"debugfs": true, "devpts": true, "devtmpfs": true, "efivarfs": true, "fusectl": true, "hugetlbfs": true,
	"mqueue": true, "nsfs": true, "proc": true, "pstore": true, "ramfs": true, "rpc_pipefs": true,
	"securityfs": true, "selinuxfs": true, "squashfs": true, "sysfs": true, "tmpfs": true, "tracefs": true,
	"fuse.lxcfs": true, "fuse.portal": true, "fuse.gvfsd-fuse": true, "nfsd": true,
}

// mountPoints returns the mounts of Linux mountinfo data worth showing as
// volumes: folders (not the files a container's bind mounts give it, which
// isDir tells), no pseudo file systems, nor mounts below /proc, /sys,
// /dev, /run (but for removable drives' /run/media) or /boot, an overlay
// only as the root (a container's), and of the mounts of one file system,
// as bind mounts make, the one of its root, else the shortest.
func mountPoints(mountinfo []byte, isDir func(mountEntry) bool) []mountEntry {
	var out []mountEntry
	var wholes []bool
	seen := map[string]int{}
	sc := bufio.NewScanner(bytes.NewReader(mountinfo))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		// ID parentID major:minor root point options [optional...] - fstype source superoptions
		pre, post, ok := strings.Cut(sc.Text(), " - ")
		if !ok {
			continue
		}
		f, g := strings.Fields(pre), strings.Fields(post)
		if len(f) < 5 || len(g) < 2 {
			continue
		}
		m := mountEntry{Point: unescapeMount(f[4]), FSType: g[0], Source: unescapeMount(g[1])}
		if pseudoFS[m.FSType] || m.FSType == "overlay" && m.Point != "/" || systemMount(m.Point) || !isDir(m) {
			continue
		}
		// Of the mounts of a file system, its root's, else the shortest.
		dev, whole := f[2], f[3] == "/"
		if i, ok := seen[dev]; ok {
			if whole && !wholes[i] || whole == wholes[i] && len(m.Point) < len(out[i].Point) {
				out[i], wholes[i] = m, whole
			}
			continue
		}
		seen[dev] = len(out)
		out, wholes = append(out, m), append(wholes, whole)
	}
	return out
}

func systemMount(point string) bool {
	if strings.HasPrefix(point, "/run/media/") {
		return false
	}
	for _, dir := range []string{"/proc", "/sys", "/dev", "/run", "/boot"} {
		if point == dir || strings.HasPrefix(point, dir+"/") {
			return true
		}
	}
	return false
}

// unescapeLabel undoes the \xHH escapes of the names of /dev/disk/by-label.
func unescapeLabel(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+4 <= len(s) && s[i+1] == 'x' {
			if n, err := strconv.ParseUint(s[i+2:i+4], 16, 8); err == nil {
				b.WriteByte(byte(n))
				i += 3
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
