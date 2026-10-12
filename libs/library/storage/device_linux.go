//go:build linux

package storage

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/unix"
)

// Magic numbers for network and FUSE filesystems in Linux.
const (
	magicSMB  = 0x517B
	magicSMB2 = 0xfe534d42
	magicCIFS = 0xff534d42
	magicNFS  = 0x6969
	magicFUSE = 0x65735546
)

func detectDevice(path string) (DeviceInfo, error) {
	var sfs unix.Statfs_t
	if err := unix.Statfs(path, &sfs); err != nil {
		return DeviceInfo{}, fmt.Errorf("statfs %s: %w", path, err)
	}

	var st unix.Stat_t
	if err := unix.Stat(path, &st); err != nil {
		return DeviceInfo{}, fmt.Errorf("stat %s: %w", path, err)
	}

	devID := fmt.Sprintf("dev:%d", st.Dev)

	fsType := linuxFSTypeName(sfs.Type)
	mountPoint := path
	var kind DeviceKind
	switch sfs.Type {
	case magicSMB, magicSMB2, magicCIFS:
		kind = KindRemoteNAS
	case magicNFS, magicFUSE:
		// The mount tells a userspace file system and the server of a share.
		kind = KindCloudMount
		if sfs.Type == magicNFS {
			kind = KindRemoteNAS
		}
		if data, err := os.ReadFile("/proc/self/mountinfo"); err == nil {
			if m, ok := findMount(data, path); ok {
				fsType, mountPoint = m.FSType, m.Point
				kind = mountKind(m)
			}
		}
	}

	rotational := false
	switch kind {
	case KindRemoteNAS, KindLocalHDD:
		rotational = true
	case KindCloudMount:
	default:
		if rotational = checkLinuxRotational(st.Dev); rotational {
			kind = KindLocalHDD
		} else {
			kind = KindLocalSSD
		}
	}

	return DeviceInfo{
		ID:         devID,
		Kind:       kind,
		FSType:     fsType,
		MountPoint: mountPoint,
		Rotational: rotational,
		Remote:     kind == KindRemoteNAS,
	}, nil
}

func checkLinuxRotational(dev uint64) bool {
	major := unix.Major(dev)
	minor := unix.Minor(dev)
	rotPath := fmt.Sprintf("/sys/dev/block/%d:%d/queue/rotational", major, minor)
	data, err := os.ReadFile(rotPath)
	if err != nil {
		return false
	}
	content := strings.TrimSpace(string(data))
	return content == "1"
}

func linuxFSTypeName(t int64) string {
	switch t {
	case magicSMB, magicSMB2, magicCIFS:
		return "cifs/smb"
	case magicNFS:
		return "nfs"
	case magicFUSE:
		return "fuse"
	case 0xef53: // EXT2/EXT3/EXT4
		return "ext4"
	case 0x58465342: // XFS
		return "xfs"
	case 0x9123683e: // BTRFS
		return "btrfs"
	case 0x2fc12fc1: // ZFS
		return "zfs"
	default:
		return fmt.Sprintf("fs_0x%x", t)
	}
}
