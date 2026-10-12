package storage

import (
	"bufio"
	"bytes"
	"strconv"
	"strings"
)

// fuseKind classifies a userspace (FUSE) file system by its name: the
// subtype of a Linux mount ("rclone" of "fuse.rclone"), the file system
// name of a macFUSE mount or the name of a WinFsp volume. File systems
// layered over local disks are local, those reaching a server by a
// network protocol network, and every other, rclone and the cloud drive
// clients among them, cloud: an unknown one is taken for the costliest.
func fuseKind(name string) DeviceKind {
	name = strings.ToLower(name)
	name = strings.TrimPrefix(name, "fuse.")
	name = strings.TrimPrefix(name, "fuse-")
	switch name {
	case "mergerfs", "unionfs", "unionfs-fuse", "bindfs", "fuse-overlayfs", "overlayfs",
		"fuseblk", "ntfs-3g", "ntfs", "exfat", "exfat-fuse", "apfs-fuse", "squashfuse", "zfs-fuse",
		"gocryptfs", "encfs", "securefs", "lxcfs":
		return KindLocalHDD
	case "sshfs", "curlftpfs", "smbnetfs", "davfs", "davfs2", "nfs", "glusterfs", "cephfs", "ceph-fuse":
		return KindRemoteNAS
	}
	return KindCloudMount
}

// loopbackSource reports whether a network mount's source, such as
// "127.0.0.1:/", "localhost:/remote" or "//guest@localhost:8445/share", is
// served by this machine: a userspace server such as rclone nfsmount or
// FUSE-T, not a share.
func loopbackSource(source string) bool {
	if strings.HasPrefix(source, "fuse-t:") {
		return true
	}
	host := strings.TrimPrefix(source, "//")
	if _, after, ok := strings.Cut(host, "@"); ok {
		host = after
	}
	if strings.HasPrefix(host, "[") {
		host, _, _ = strings.Cut(host[1:], "]")
	} else if i := strings.IndexAny(host, ":/"); i >= 0 {
		host = host[:i]
	}
	return host == "localhost" || host == "::1" || strings.HasPrefix(host, "127.")
}

// mountKind classifies a Linux FUSE or NFS mount: FUSE mounts by their
// subtype, and NFS mounts served by this machine, such as rclone
// nfsmount, as cloud.
func mountKind(m mountEntry) DeviceKind {
	switch {
	case strings.HasPrefix(m.FSType, "nfs"):
		if loopbackSource(m.Source) {
			return KindCloudMount
		}
		return KindRemoteNAS
	case m.FSType == "fuse":
		// No subtype: the source may name the file system.
		return fuseKind(m.Source)
	}
	return fuseKind(m.FSType)
}

// mountEntry is a mount of /proc/self/mountinfo.
type mountEntry struct {
	Point, FSType, Source string
}

// findMount returns the mount of Linux mountinfo data holding path, the one
// with the longest mount point above it.
func findMount(mountinfo []byte, path string) (mountEntry, bool) {
	var best mountEntry
	found := false
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
		point := unescapeMount(f[4])
		if !inMount(path, point) || (found && len(point) < len(best.Point)) {
			continue
		}
		best, found = mountEntry{Point: point, FSType: g[0], Source: unescapeMount(g[1])}, true
	}
	return best, found
}

func inMount(path, point string) bool {
	if point == "/" {
		return strings.HasPrefix(path, "/")
	}
	return path == point || strings.HasPrefix(path, point+"/")
}

// unescapeMount undoes the octal escapes of spaces, tabs, newlines and
// backslashes in mountinfo fields.
func unescapeMount(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+4 <= len(s) {
			if n, err := strconv.ParseUint(s[i+1:i+4], 8, 8); err == nil {
				b.WriteByte(byte(n))
				i += 3
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
