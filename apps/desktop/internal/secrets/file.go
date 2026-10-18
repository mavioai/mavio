package secrets

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// File is a Backend keeping secrets in a file only its user may read, for
// development: a build of `go run` is signed anew each time, so the
// credential store would ask again each launch to let it read what an
// earlier build kept. A key it does not have it reads once from from,
// when set, and keeps, so that development moves off the credential store
// without signing in again.
type File struct {
	path     string
	from     Backend
	notFound error

	mu sync.Mutex
}

// NewFile returns a File keeping secrets in path, moving them over from
// from (nil for none), which answers notFound for a key it does not have;
// File answers notFound too.
func NewFile(path string, from Backend, notFound error) *File {
	return &File{path: path, from: from, notFound: notFound}
}

// Get returns the secret kept under key.
func (f *File) Get(key string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	m, err := f.read()
	if err != nil {
		return nil, err
	}
	if v, ok := m[key]; ok {
		return []byte(v), nil
	}
	if f.from == nil {
		return nil, f.notFound
	}
	v, err := f.from.Get(key)
	if err != nil {
		return nil, err
	}
	m[key] = string(v)
	if err := f.write(m); err != nil {
		return nil, err
	}
	return v, nil
}

// Set keeps value under key.
func (f *File) Set(key string, value []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	m, err := f.read()
	if err != nil {
		return err
	}
	m[key] = string(value)
	return f.write(m)
}

// Delete forgets key.
func (f *File) Delete(key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	m, err := f.read()
	if err != nil {
		return err
	}
	if _, ok := m[key]; !ok {
		return f.notFound
	}
	delete(m, key)
	return f.write(m)
}

func (f *File) read() (map[string]string, error) {
	m := map[string]string{}
	b, err := os.ReadFile(f.path)
	if errors.Is(err, os.ErrNotExist) {
		return m, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read the secrets: %w", err)
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("read the secrets: %w", err)
	}
	return m, nil
}

func (f *File) write(m map[string]string) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(f.path), 0o700); err != nil {
		return fmt.Errorf("write the secrets: %w", err)
	}
	tmp := f.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return fmt.Errorf("write the secrets: %w", err)
	}
	if err := os.Rename(tmp, f.path); err != nil {
		return fmt.Errorf("write the secrets: %w", err)
	}
	return nil
}
