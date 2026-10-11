package disk

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTicks(t *testing.T) {
	tests := []struct {
		t    time.Time
		want int64
	}{
		{time.Unix(0, 0), 621_355_968_000_000_000},
		{time.Date(2024, 1, 2, 3, 4, 5, 600, time.UTC), 638_397_614_450_000_006},
		{time.Date(2024, 1, 2, 4, 4, 5, 600, time.FixedZone("CET", 3600)), 638_397_614_450_000_006},
	}
	for _, tt := range tests {
		if got := Ticks(tt.t); got != tt.want {
			t.Errorf("Ticks(%v) = %d, want = %d", tt.t, got, tt.want)
		}
	}
}

func TestCreationTime(t *testing.T) {
	dir := t.TempDir()
	before := time.Now().Add(-time.Minute)
	fi, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := CreationTime(fi); got.Before(before) || got.After(time.Now().Add(time.Minute)) {
		t.Errorf("CreationTime() = %v, want = about now", got)
	}
}

func TestIsLockedUnheld(t *testing.T) {
	name := filepath.Join(t.TempDir(), "file.mkv")
	if err := os.WriteFile(name, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if IsLocked(name) {
		t.Errorf("IsLocked() = true, want = false")
	}
	if IsLocked(name + ".missing") {
		t.Errorf("IsLocked(missing) = true, want = false")
	}
}
