package storage

import (
	"fmt"
	"path/filepath"
	"strings"
)

// DeviceKind classifies the physical or network characteristics of a storage device.
type DeviceKind string

const (
	// KindLocalSSD represents local solid-state drives with high IOPS and no rotational seek penalty.
	KindLocalSSD DeviceKind = "local_ssd"

	// KindLocalHDD represents local spinning mechanical hard drives with high seek penalty.
	KindLocalHDD DeviceKind = "local_hdd"

	// KindRemoteNAS represents network-attached storage (SMB, NFS, AFP, WebDAV).
	KindRemoteNAS DeviceKind = "remote_nas"

	// KindCloudMount represents cloud storage synchronization or FUSE mounts (iCloud, OneDrive, Rclone).
	KindCloudMount DeviceKind = "cloud_mount"

	// KindUnknown represents unclassified or undetected storage.
	KindUnknown DeviceKind = "unknown"
)

// DeviceInfo carries physical medium and protocol information for a given path.
type DeviceInfo struct {
	// ID is the unique, stable identifier of the physical drive or volume (e.g. "dev:2053", "fsid:1234:5678", "vol:08FA2B").
	// This identifier is used as the key for per-device anti-thrashing single-flight lanes.
	ID string

	// Kind classifies the device (SSD, HDD, NAS, Cloud).
	Kind DeviceKind

	// FSType is the filesystem or network protocol name (e.g. "ext4", "apfs", "ntfs", "smbfs", "nfs").
	FSType string

	// MountPoint is the filesystem mount root where the target path resides.
	MountPoint string

	// Rotational indicates whether the device has a physical rotational seek penalty (mechanical disk).
	Rotational bool

	// Remote indicates whether the filesystem resides on a remote network host.
	Remote bool
}

// DetectDevice inspects the filesystem and physical drive backing path.
func DetectDevice(path string) (DeviceInfo, error) {
	if path == "" {
		return DeviceInfo{}, fmt.Errorf("detect device: path is empty")
	}
	cleanPath, err := filepath.Abs(path)
	if err != nil {
		cleanPath = filepath.Clean(path)
	}
	return detectDevice(cleanPath)
}

func isCloudPath(path string) bool {
	return strings.Contains(path, "/Library/Mobile Documents") ||
		strings.Contains(path, "/Library/CloudStorage") ||
		strings.Contains(path, `\OneDrive`) ||
		strings.Contains(path, `\iCloudDrive`)
}
