//go:build darwin

package storage

import (
	"fmt"
	"strings"

	"golang.org/x/sys/unix"
)

func detectDevice(path string) (DeviceInfo, error) {
	var sfs unix.Statfs_t
	if err := unix.Statfs(path, &sfs); err != nil {
		return DeviceInfo{}, fmt.Errorf("statfs %s: %w", path, err)
	}

	fstype := charsToString(sfs.Fstypename[:])
	mpath := charsToString(sfs.Mntonname[:])
	source := charsToString(sfs.Mntfromname[:])
	devID := fmt.Sprintf("fsid:%d:%d", sfs.Fsid.Val[0], sfs.Fsid.Val[1])

	isLocal := (sfs.Flags & unix.MNT_LOCAL) != 0
	isRemote := !isLocal || isRemoteFSType(fstype)

	var kind DeviceKind
	rotational := false

	switch {
	case isCloudPath(path) || darwinCloudMount(fstype, source):
		kind = KindCloudMount
		isRemote = false
	case isRemote:
		kind = KindRemoteNAS
		rotational = true // Treat remote network storage with rotational queue semantics to serialize I/O
	case strings.HasPrefix(path, "/Volumes/"):
		// External drives on macOS may be rotational mechanical HDDs (USB/Thunderbolt enclosures)
		kind = KindLocalHDD
		rotational = true
	default:
		// Internal system volumes on modern Apple Silicon Macs are fast NVMe/APFS SSDs
		kind = KindLocalSSD
		rotational = false
	}

	return DeviceInfo{
		ID:         devID,
		Kind:       kind,
		FSType:     fstype,
		MountPoint: mpath,
		Rotational: rotational,
		Remote:     isRemote,
	}, nil
}

// darwinCloudMount reports whether a mount is a userspace file system
// taken for a cloud drive: a macFUSE mount, or an NFS or SMB share this
// machine serves, as rclone nfsmount and FUSE-T mounts are.
func darwinCloudMount(fstype, source string) bool {
	switch strings.ToLower(fstype) {
	case "macfuse", "osxfuse", "fusefs":
		return true
	case "nfs", "smbfs", "webdav":
		return loopbackSource(source)
	}
	return false
}

func isRemoteFSType(fstype string) bool {
	f := strings.ToLower(fstype)
	return f == "smbfs" || f == "nfs" || f == "afpfs" || f == "webdav"
}

func charsToString(ca []byte) string {
	for i, c := range ca {
		if c == 0 {
			return string(ca[:i])
		}
	}
	return string(ca)
}
