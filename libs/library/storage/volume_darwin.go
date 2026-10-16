//go:build darwin

package storage

import (
	"context"
	"fmt"
	"path"
	"strings"

	"golang.org/x/sys/unix"
)

func volumes(ctx context.Context) ([]Volume, error) {
	// MNT_NOWAIT takes the sizes the system has, without asking shares.
	n, err := unix.Getfsstat(nil, unix.MNT_NOWAIT)
	if err != nil {
		return nil, fmt.Errorf("list mounts: %w", err)
	}
	buf := make([]unix.Statfs_t, n)
	if n, err = unix.Getfsstat(buf, unix.MNT_NOWAIT); err != nil {
		return nil, fmt.Errorf("list mounts: %w", err)
	}
	var vs []Volume
	for _, s := range buf[:n] {
		fstype := charsToString(s.Fstypename[:])
		// The system's own volumes, as /System/Volumes/Data, are not browsed.
		if s.Flags&unix.MNT_DONTBROWSE != 0 || fstype == "devfs" || fstype == "autofs" || fstype == "nullfs" {
			continue
		}
		point := charsToString(s.Mntonname[:])
		// Read-only volumes besides the sealed system's are disk images
		// mounted, such as installers.
		if s.Flags&unix.MNT_RDONLY != 0 && point != "/" {
			continue
		}
		v := Volume{Path: point, FSType: fstype, Kind: KindUnknown}
		if strings.HasPrefix(point, "/Volumes/") {
			v.Name = path.Base(point)
		}
		v.Total, v.Available = s.Blocks*uint64(s.Bsize), s.Bavail*uint64(s.Bsize)
		v.Used = v.Total - min(v.Available, v.Total)
		vs = append(vs, v)
	}
	// Telling the device asks the volume, which a share may not answer.
	probeAll(ctx, vs, func(v *Volume) error {
		d, err := DetectDevice(v.Path)
		if err == nil {
			v.Kind = d.Kind
		}
		return err
	})
	return vs, nil
}
