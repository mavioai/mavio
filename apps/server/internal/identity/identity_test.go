package identity

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	file := filepath.Join(t.TempDir(), "keys", "identity.key")
	first, err := Load(file)
	if err != nil {
		t.Fatalf("Load (create) = %v", err)
	}
	if info, err := os.Stat(file); err != nil || info.Mode().Perm()&0o077 != 0 {
		t.Errorf("key file = %v, %v; want readable by the owner only", info, err)
	}
	again, err := Load(file)
	if err != nil || !bytes.Equal(again.Public(), first.Public()) {
		t.Errorf("Load again = %x, %v, want = %x", again.Public(), err, first.Public())
	}

	if err := os.WriteFile(file, []byte("not a key"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(file); err == nil {
		t.Error("Load(garbage) = nil, want an error")
	}
}

func TestDerive(t *testing.T) {
	k := Generate()
	a, b := k.Derive("a"), k.Derive("b")
	if len(a) != 32 || bytes.Equal(a, b) || !bytes.Equal(a, k.Derive("a")) {
		t.Errorf("Derive = %x, %x; want 32 stable bytes per purpose", a, b)
	}
	if bytes.Equal(a, Generate().Derive("a")) {
		t.Error("two keys derive the same secret")
	}
}

func TestID(t *testing.T) {
	k := Generate()
	id := k.ID()
	if len(id) != 32 || strings.ToLower(id) != id || strings.ContainsRune(id, '=') {
		t.Errorf("ID = %q, want 32 lowercase base32 characters", id)
	}
	if k.ID() != id {
		t.Errorf("ID is not stable")
	}
	if Generate().ID() == id {
		t.Errorf("two keys share ID %q", id)
	}
}
