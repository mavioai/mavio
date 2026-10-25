package rclone

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestLockConfig(t *testing.T) {
	conf := filepath.Join(t.TempDir(), "rclone", "rclone.conf")
	first, err := lockConfig(conf)
	if err != nil {
		t.Fatalf("lockConfig = %v", err)
	}
	if _, err := lockConfig(conf); !errors.Is(err, ErrConfigInUse) {
		t.Errorf("lockConfig(held) error = %v, want = %v", err, ErrConfigInUse)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	again, err := lockConfig(conf)
	if err != nil {
		t.Fatalf("lockConfig(released) = %v", err)
	}
	_ = again.Close()
}
