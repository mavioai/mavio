package storage

import "testing"

func TestFuseKind(t *testing.T) {
	tests := []struct {
		name string
		want DeviceKind
	}{
		{"fuse.rclone", KindCloudMount},
		{"rclone", KindCloudMount},
		{"FUSE-rclone", KindCloudMount},
		{"fuse.s3fs", KindCloudMount},
		{"fuse.alist", KindCloudMount},
		{"fuse.mergerfs", KindLocalHDD},
		{"fuseblk", KindLocalHDD},
		{"fuse.sshfs", KindRemoteNAS},
		{"FUSE-SSHFS", KindRemoteNAS},
	}
	for _, tt := range tests {
		if got := fuseKind(tt.name); got != tt.want {
			t.Errorf("fuseKind(%q) = %q, want = %q", tt.name, got, tt.want)
		}
	}
}

func TestLoopbackSource(t *testing.T) {
	tests := []struct {
		source string
		want   bool
	}{
		{"127.0.0.1:/", true},
		{"localhost:/gdrive", true},
		{"[::1]:/", true},
		{"//guest@localhost:8445/share", true},
		{"fuse-t:/rclone", true},
		{"nas.lan:/volume1/media", false},
		{"192.168.1.10:/export", false},
		{"//user@nas/media", false},
	}
	for _, tt := range tests {
		if got := loopbackSource(tt.source); got != tt.want {
			t.Errorf("loopbackSource(%q) = %v, want = %v", tt.source, got, tt.want)
		}
	}
}

const mountinfo = `22 1 8:1 / / rw,relatime shared:1 - ext4 /dev/sda1 rw
40 22 0:35 / /mnt/cloud rw,nosuid,nodev,relatime shared:20 - fuse.rclone gdrive: rw,user_id=0,group_id=0
41 22 0:36 / /mnt/pool rw,relatime shared:21 - fuse.mergerfs disk1:disk2 rw
42 22 0:37 / /mnt/nas rw,relatime shared:22 - nfs4 nas.lan:/volume1 rw
43 22 0:38 / /mnt/rnfs rw,relatime shared:23 - nfs 127.0.0.1:/ rw
44 40 0:39 / /mnt/cloud/My\040Films rw,relatime shared:24 - fuse.sshfs user@host:/films rw
`

func TestFindMount(t *testing.T) {
	tests := []struct {
		path string
		want mountEntry
		kind DeviceKind
	}{
		{"/mnt/cloud/films/a.mkv", mountEntry{"/mnt/cloud", "fuse.rclone", "gdrive:"}, KindCloudMount},
		{"/mnt/cloud", mountEntry{"/mnt/cloud", "fuse.rclone", "gdrive:"}, KindCloudMount},
		{"/mnt/cloudy", mountEntry{"/", "ext4", "/dev/sda1"}, ""},
		{"/mnt/pool/tv", mountEntry{"/mnt/pool", "fuse.mergerfs", "disk1:disk2"}, KindLocalHDD},
		{"/mnt/nas/a", mountEntry{"/mnt/nas", "nfs4", "nas.lan:/volume1"}, KindRemoteNAS},
		{"/mnt/rnfs/a", mountEntry{"/mnt/rnfs", "nfs", "127.0.0.1:/"}, KindCloudMount},
		{"/mnt/cloud/My Films/a.mkv", mountEntry{"/mnt/cloud/My Films", "fuse.sshfs", "user@host:/films"}, KindRemoteNAS},
	}
	for _, tt := range tests {
		got, ok := findMount([]byte(mountinfo), tt.path)
		if !ok || got != tt.want {
			t.Errorf("findMount(%q) = %+v, %v, want = %+v", tt.path, got, ok, tt.want)
		}
		if tt.kind != "" {
			if k := mountKind(got); k != tt.kind {
				t.Errorf("mountKind(%+v) = %q, want = %q", got, k, tt.kind)
			}
		}
	}
}
