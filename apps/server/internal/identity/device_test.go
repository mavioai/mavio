package identity

import (
	"regexp"
	"testing"
)

func TestDeviceID(t *testing.T) {
	v5 := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-5[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	const server = "unf52jqyqsbe5lml6amwpzaxihp2qlwd"
	machine := deviceID("4C4C4544-0042-3510-8051-B4C04F4D3732", server)
	fromServer := deviceID("", server)
	for _, got := range []string{machine, fromServer} {
		if !v5.MatchString(got) {
			t.Errorf("deviceID = %q, want a version 5 UUID", got)
		}
	}
	if got := deviceID(" 4c4c4544-0042-3510-8051-b4c04f4d3732\n", "other"); got != machine {
		t.Errorf("deviceID of the same machine = %q, want = %q", got, machine)
	}
	if machine == fromServer {
		t.Errorf("deviceID of the machine and of the server are both %q", machine)
	}
	if got := deviceID("", "other"); got == fromServer {
		t.Errorf("deviceID of two servers are both %q", got)
	}
	if got := deviceID("4C4C4544-0042-3510-8051-B4C04F4D3732", server); got != machine {
		t.Errorf("deviceID = %q, want the same again = %q", got, machine)
	}
}

func TestUUIDv5(t *testing.T) {
	// RFC 9562 Appendix A.4: the name "www.example.com" in the DNS namespace.
	dns := [16]byte{0x6b, 0xa7, 0xb8, 0x10, 0x9d, 0xad, 0x11, 0xd1, 0x80, 0xb4, 0x00, 0xc0, 0x4f, 0xd4, 0x30, 0xc8}
	if got, want := uuidV5(dns, "www.example.com"), "2ed6657d-e927-568b-95e1-2665a8aea6a2"; got != want {
		t.Errorf("uuidV5 = %q, want = %q", got, want)
	}
}

func TestUsable(t *testing.T) {
	for _, tc := range []struct {
		id   string
		want bool
	}{
		{"", false},
		{"uninitialized\n", false},
		{"00000000-0000-0000-0000-000000000000", false},
		{"0123456789abcdef0123456789abcdef", true},
	} {
		if got := usable(tc.id); got != tc.want {
			t.Errorf("usable(%q) = %v, want = %v", tc.id, got, tc.want)
		}
	}
}

func TestMachineIDStable(t *testing.T) {
	if got, again := machineID(), machineID(); got != again {
		t.Errorf("machineID = %q, then %q", got, again)
	}
}

func TestRuntimeCgroup(t *testing.T) {
	for _, tc := range []struct {
		name, cgroup string
		want         bool
	}{
		{"host v2", "0::/init.scope\n", false},
		{"namespaced v2", "0::/\n", false},
		{"host v1", "12:pids:/\n11:memory:/init.scope\n", false},
		{"docker v1", "12:pids:/docker/3f1a2b\n", true},
		{"kubernetes v2", "0::/kubepods.slice/kubepods-burstable.slice/cri-containerd-9a.scope\n", true},
		{"podman v2", "0::/machine.slice/libpod-4c2e.scope/container\n", true},
	} {
		if got := runtimeCgroup(tc.cgroup); got != tc.want {
			t.Errorf("%s: runtimeCgroup = %v, want = %v", tc.name, got, tc.want)
		}
	}
}

func TestOverlayRoot(t *testing.T) {
	for _, tc := range []struct {
		name, mountinfo string
		want            bool
	}{
		{"host", "22 1 259:2 / / rw,relatime shared:1 - ext4 /dev/nvme0n1p2 rw\n24 22 0:5 / /dev rw - devtmpfs udev rw\n", false},
		{"container", "612 541 0:52 / / rw,relatime master:301 - overlay overlay rw,lowerdir=/var/lib/docker/overlay2/l/A\n613 612 0:56 / /proc rw - proc proc rw\n", true},
		{"overlay elsewhere", "22 1 259:2 / / rw - btrfs /dev/sda2 rw\n30 22 0:52 / /mnt rw - overlay overlay rw\n", false},
	} {
		if got := overlayRoot(tc.mountinfo); got != tc.want {
			t.Errorf("%s: overlayRoot = %v, want = %v", tc.name, got, tc.want)
		}
	}
}
