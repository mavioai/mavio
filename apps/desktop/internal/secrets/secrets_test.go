package secrets

import (
	"errors"
	"maps"
	"path/filepath"
	"testing"
)

var errNotFound = errors.New("not found")

type memory map[string][]byte

func (m memory) Get(key string) ([]byte, error) {
	v, ok := m[key]
	if !ok {
		return nil, errNotFound
	}
	return v, nil
}

func (m memory) Set(key string, value []byte) error { m[key] = value; return nil }

func (m memory) Delete(key string) error {
	if _, ok := m[key]; !ok {
		return errNotFound
	}
	delete(m, key)
	return nil
}

func TestStore(t *testing.T) {
	backend := memory{}
	index := filepath.Join(t.TempDir(), "secrets.json")
	s := New(backend, index, errNotFound)
	for _, step := range []struct{ name, value string }{
		{"device-key", "k"},
		{"server-a", "a1"},
		{"server-b", "b"},
		{"server-a", "a2"},
		{"server-b", ""},
		{"server-c", ""},
	} {
		if err := s.Set(step.name, step.value); err != nil {
			t.Fatalf("Set(%q, %q) = %v", step.name, step.value, err)
		}
	}
	// A secret the backend lost is left out.
	backend["core.lost"] = nil
	delete(backend, "core.lost")
	got, err := New(backend, index, errNotFound).Load()
	if err != nil {
		t.Fatal(err)
	}
	if want := map[string]string{"device-key": "k", "server-a": "a2"}; !maps.Equal(got, want) {
		t.Errorf("Load() = %v, want = %v", got, want)
	}
	if _, ok := backend["core.server-b"]; ok {
		t.Errorf("the forgotten secret is still kept")
	}
}

func TestLoadEmpty(t *testing.T) {
	got, err := New(memory{}, filepath.Join(t.TempDir(), "secrets.json"), errNotFound).Load()
	if err != nil || len(got) != 0 {
		t.Errorf("Load() = %v, %v, want = empty", got, err)
	}
}
