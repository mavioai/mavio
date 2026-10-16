//go:build linux

package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

func volumes(ctx context.Context) ([]Volume, error) {
	data, err := os.ReadFile("/proc/self/mountinfo")
	if err != nil {
		return nil, fmt.Errorf("read mounts: %w", err)
	}
	mounts := mountPoints(data, func(m mountEntry) bool {
		if networkMount(m) {
			return true
		}
		info, err := os.Stat(m.Point)
		return err == nil && info.IsDir()
	})
	labels := diskLabels()
	vs := make([]Volume, len(mounts))
	for i, m := range mounts {
		vs[i] = Volume{Path: m.Point, Name: labels[m.Source], FSType: m.FSType, Kind: KindUnknown}
		if networkMount(m) {
			vs[i].Kind = mountKind(m)
		}
	}
	probeAll(ctx, vs, func(v *Volume) error {
		var sfs unix.Statfs_t
		if err := unix.Statfs(v.Path, &sfs); err != nil {
			return err
		}
		v.Total, v.Available = sfs.Blocks*uint64(sfs.Bsize), sfs.Bavail*uint64(sfs.Bsize)
		if d, err := DetectDevice(v.Path); err == nil {
			v.Kind = d.Kind
		}
		return nil
	})
	return vs, nil
}

// networkMount reports whether a Linux mount reaches a server or a
// userspace file system, whose folders are not looked at, as one gone may
// never answer.
func networkMount(m mountEntry) bool {
	switch {
	case strings.HasPrefix(m.FSType, "nfs"), strings.HasPrefix(m.FSType, "fuse"),
		m.FSType == "cifs", m.FSType == "smb3", m.FSType == "smbfs", m.FSType == "9p",
		m.FSType == "ceph", m.FSType == "glusterfs", m.FSType == "virtiofs":
		return true
	}
	return false
}

// diskLabels maps the devices of partitions to their labels, from
// /dev/disk/by-label; none where it is missing, as in containers.
func diskLabels() map[string]string {
	labels := map[string]string{}
	entries, _ := os.ReadDir("/dev/disk/by-label")
	for _, e := range entries {
		if dev, err := filepath.EvalSymlinks(filepath.Join("/dev/disk/by-label", e.Name())); err == nil {
			labels[dev] = unescapeLabel(e.Name())
		}
	}
	return labels
}
