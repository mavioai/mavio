//go:build !darwin && !linux && !windows

package storage

import (
	"path/filepath"
)

func detectDevice(path string) (DeviceInfo, error) {
	vol := filepath.VolumeName(path)
	if vol == "" {
		vol = "/"
	}
	return DeviceInfo{
		ID:         "dev:" + vol,
		Kind:       KindLocalHDD, // Default to conservative rotational handling
		FSType:     "generic",
		MountPoint: vol,
		Rotational: true,
		Remote:     false,
	}, nil
}
