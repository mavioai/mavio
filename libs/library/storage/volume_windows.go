//go:build windows

package storage

import (
	"context"
	"errors"
	"fmt"

	"golang.org/x/sys/windows"
)

func volumes(ctx context.Context) ([]Volume, error) {
	mask, err := windows.GetLogicalDrives()
	if err != nil {
		return nil, fmt.Errorf("list drives: %w", err)
	}
	var vs []Volume
	var removable []bool
	for i := range 26 {
		if mask&(1<<i) == 0 {
			continue
		}
		root := string(rune('A'+i)) + `:\`
		p, err := windows.UTF16PtrFromString(root)
		if err != nil {
			continue
		}
		switch t := windows.GetDriveType(p); t {
		case windows.DRIVE_FIXED, windows.DRIVE_REMOTE, windows.DRIVE_REMOVABLE, windows.DRIVE_RAMDISK:
			vs = append(vs, Volume{Path: root, Kind: KindUnknown})
			removable = append(removable, t == windows.DRIVE_REMOVABLE)
		}
	}
	errs := probeAll(ctx, vs, func(v *Volume) error {
		p, err := windows.UTF16PtrFromString(v.Path)
		if err != nil {
			return err
		}
		var label, fs [windows.MAX_PATH + 1]uint16
		if err := windows.GetVolumeInformation(p, &label[0], uint32(len(label)), nil, nil, nil, &fs[0], uint32(len(fs))); err != nil {
			return err
		}
		v.Name, v.FSType = windows.UTF16ToString(label[:]), windows.UTF16ToString(fs[:])
		var total, free uint64
		if err := windows.GetDiskFreeSpaceEx(p, &v.Available, &total, &free); err != nil {
			return err
		}
		v.Total = total
		if d, err := DetectDevice(v.Path); err == nil {
			v.Kind = d.Kind
		}
		return nil
	})
	// A card reader or an optical drive without a medium is no volume.
	out := vs[:0]
	for i, v := range vs {
		if removable[i] && errs[i] != nil && !errors.Is(errs[i], context.DeadlineExceeded) {
			continue
		}
		out = append(out, v)
	}
	return out, nil
}
