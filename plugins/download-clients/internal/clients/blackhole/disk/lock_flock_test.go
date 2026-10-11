//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package disk

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestIsLockedHeld(t *testing.T) {
	name := filepath.Join(t.TempDir(), "file.mkv")
	if err := os.WriteFile(name, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(name)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		t.Fatal(err)
	}
	if !IsLocked(name) {
		t.Errorf("IsLocked() = false, want = true")
	}
}
