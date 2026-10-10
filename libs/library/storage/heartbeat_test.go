package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestVolumeHeartbeat(t *testing.T) {
	dir := t.TempDir()
	for _, size := range []int{0, 100, 4096, 64 << 10, 1<<20 + 123} {
		p := filepath.Join(dir, "f.bin")
		if err := os.WriteFile(p, make([]byte, size), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := VolumeHeartbeat(p); err != nil {
			t.Errorf("VolumeHeartbeat(%d bytes) = %v, want nil", size, err)
		}
	}
	if err := VolumeHeartbeat(filepath.Join(dir, "missing")); err == nil {
		t.Error("VolumeHeartbeat(missing) = nil, want an error")
	}
}

func TestAlignedBlock(t *testing.T) {
	for range 10 {
		b := alignedBlock()
		if len(b) != heartbeatBlock {
			t.Fatalf("len = %d, want %d", len(b), heartbeatBlock)
		}
	}
}

func TestHeartbeatsClaim(t *testing.T) {
	h := &Heartbeats{Interval: time.Second}
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		volume string
		at     time.Duration
		want   bool
	}{
		{"dev:1", 0, true},
		{"dev:1", 500 * time.Millisecond, false},
		{"dev:2", 500 * time.Millisecond, true},
		{"dev:1", time.Second, true},
		{"dev:1", 1500 * time.Millisecond, false},
	}
	for _, c := range cases {
		if got := h.Claim(c.volume, t0.Add(c.at)); got != c.want {
			t.Errorf("Claim(%s, +%v) = %v, want %v", c.volume, c.at, got, c.want)
		}
	}
}
