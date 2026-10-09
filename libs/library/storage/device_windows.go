//go:build windows

package storage

import (
	"fmt"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

func detectDevice(path string) (DeviceInfo, error) {
	volPath := filepath.VolumeName(path)
	if volPath == "" {
		volPath = path
	}
	rootPath := volPath + `\`

	rootPtr, err := syscall.UTF16PtrFromString(rootPath)
	if err != nil {
		return DeviceInfo{}, fmt.Errorf("utf16 root path: %w", err)
	}

	driveType := windows.GetDriveType(rootPtr)
	isRemote := driveType == windows.DRIVE_REMOTE || strings.HasPrefix(path, `\\`)

	var volSerial uint32
	var fsNameBuf [256]uint16
	_ = windows.GetVolumeInformation(rootPtr, nil, 0, &volSerial, nil, nil, &fsNameBuf[0], uint32(len(fsNameBuf)))
	fsType := syscall.UTF16ToString(fsNameBuf[:])
	if fsType == "" {
		fsType = "ntfs"
	}

	devID := fmt.Sprintf("vol:%08X", volSerial)
	if volSerial == 0 {
		devID = fmt.Sprintf("path:%s", strings.ToLower(volPath))
	}

	rotational := false
	if !isRemote {
		rotational = checkWindowsSeekPenalty(volPath)
	}

	var kind DeviceKind
	switch {
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
		MountPoint: rootPath,
		Rotational: rotational,
		Remote:     isRemote,
	}, nil
}

// StorageDeviceSeekPenaltyProperty query for Windows.
const (
	ioctlStorageQueryProperty        = 0x002D1400 // CTL_CODE(IOCTL_STORAGE_BASE, 0x0500, METHOD_BUFFERED, FILE_ANY_ACCESS)
	storageDeviceSeekPenaltyProperty = 7
	propertyStandardQuery           = 0
)

type storagePropertyQuery struct {
	PropertyID uint32
	QueryType  uint32
	Additional uint32
}

type deviceSeekPenaltyDescriptor struct {
	Version           uint32
	Size              uint32
	IncursSeekPenalty bool
}

func checkWindowsSeekPenalty(volPath string) bool {
	devicePath := `\\.\` + strings.TrimSuffix(volPath, `\`)
	p, err := syscall.UTF16PtrFromString(devicePath)
	if err != nil {
		return false
	}

	h, err := windows.CreateFile(p, 0, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		return false
	}
	defer windows.CloseHandle(h)

	query := storagePropertyQuery{
		PropertyID: storageDeviceSeekPenaltyProperty,
		QueryType:  propertyStandardQuery,
	}

	var desc deviceSeekPenaltyDescriptor
	var bytesReturned uint32

	err = windows.DeviceIoControl(
		h,
		ioctlStorageQueryProperty,
		(*byte)(unsafe.Pointer(&query)),
		uint32(unsafe.Sizeof(query)),
		(*byte)(unsafe.Pointer(&desc)),
		uint32(unsafe.Sizeof(desc)),
		&bytesReturned,
		nil,
	)
	if err != nil {
		return false
	}

	return desc.IncursSeekPenalty
}
