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
	isRemote := false
	isCloud := false

	switch sfs.Type {
	case magicSMB, magicSMB2, magicCIFS, magicNFS:
		isRemote = true
	case magicFUSE:
		isCloud = true
	}

	rotational := false
	if !isRemote && !isCloud {
		rotational = checkLinuxRotational(st.Dev)
	}

	var kind DeviceKind
	switch {
	case isCloud:
		kind = KindCloudMount
	case isRemote:
		kind = KindRemoteNAS
		rotational = true
	case rotational:
		kind = KindLocalHDD
	default:
		kind = KindLocalSSD
	}

	return DeviceInfo{
		ID:         devID,
		Kind:       kind,
		FSType:     fsType,
		MountPoint: path,
		Rotational: rotational,
		Remote:     isRemote,
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
