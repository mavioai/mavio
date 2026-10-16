package storage

import (
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestMountPoints(t *testing.T) {
	mountinfo := `22 1 259:2 / / rw,relatime shared:1 - ext4 /dev/nvme0n1p2 rw
23 22 0:21 / /proc rw - proc proc rw
24 22 0:22 / /sys rw - sysfs sysfs rw
25 22 0:5 / /dev rw - devtmpfs udev rw
26 22 0:24 / /run rw - tmpfs tmpfs rw
27 22 259:1 / /boot/efi rw - vfat /dev/nvme0n1p1 rw
28 22 8:17 / /mnt/media\040disk rw - xfs /dev/sdb1 rw
29 22 8:17 /movies /srv/movies rw - xfs /dev/sdb1 rw
30 22 7:0 / /snap/core/1 ro - squashfs /dev/loop0 ro
31 22 0:50 / /mnt/nas rw - nfs4 nas:/export rw
32 22 0:51 / /mnt/115 rw - fuse.rclone 115: rw
33 22 8:33 / /run/media/me/USB rw - exfat /dev/sdc1 rw
34 22 259:2 /etc/hosts /etc/hosts rw - ext4 /dev/nvme0n1p2 rw
35 22 0:60 / /var/lib/docker/overlay2/x/merged rw - overlay overlay rw
`
	isDir := func(m mountEntry) bool { return m.Point != "/etc/hosts" }
	var got []string
	for _, m := range mountPoints([]byte(mountinfo), isDir) {
		got = append(got, m.Point+" "+m.FSType)
	}
	want := []string{"/ ext4", "/mnt/media disk xfs", "/mnt/nas nfs4", "/mnt/115 fuse.rclone", "/run/media/me/USB exfat"}
	if !slices.Equal(got, want) {
		t.Errorf("got = %q, want = %q", got, want)
	}
}

func TestMountPointsKeepAFileSystemsRoot(t *testing.T) {
	mountinfo := `29 1 8:17 /movies /srv/media/movies rw - xfs /dev/sdb1 rw
28 1 8:17 / /data rw - xfs /dev/sdb1 rw
`
	got := mountPoints([]byte(mountinfo), func(mountEntry) bool { return true })
	if len(got) != 1 || got[0].Point != "/data" {
		t.Errorf("got = %v, want = /data alone", got)
	}
}

func TestUnescapeLabel(t *testing.T) {
	if got := unescapeLabel(`My\x20Disk`); got != "My Disk" {
		t.Errorf("got = %q, want = %q", got, "My Disk")
	}
}

func TestVolumesHoldTheTempFolder(t *testing.T) {
	switch runtime.GOOS {
	case "linux", "darwin", "windows":
	default:
		t.Skip("volumes are not read on " + runtime.GOOS)
	}
	vs, err := Volumes(t.Context())
	if err != nil {
		t.Fatalf("Volumes: %v", err)
	}
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range vs {
		if v.Total == 0 || v.Used+v.Available != v.Total {
			continue
		}
		p := strings.TrimRight(v.Path, `/\`)
		if p == "" || strings.HasPrefix(strings.ToLower(dir), strings.ToLower(p)+string(filepath.Separator)) {
			return
		}
	}
	t.Errorf("no volume with sizes holds %s: %+v", dir, vs)
}
