package secrets

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
)

// Backend is a credential store: mygo.Secrets in the app.
type Backend interface {
	Get(key string) ([]byte, error)
	Set(key string, value []byte) error
	Delete(key string) error
}

// prefix keeps the core's secrets apart from others of the app.
const prefix = "core."

// Store keeps the core's secrets in a Backend, which cannot list them, and
// the names of those it keeps in an index file.
type Store struct {
	backend  Backend
	index    string
	notFound error

	mu sync.Mutex
}

// New returns a Store keeping secrets in backend, which answers notFound
// for a key it does not have, and their names in the file index.
func New(backend Backend, index string, notFound error) *Store {
	return &Store{backend: backend, index: index, notFound: notFound}
}

// Load returns the secrets by name; those the backend lost are left out.
func (s *Store) Load() (map[string]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	names, err := s.names()
	if err != nil {
		return nil, err
	}
	secrets := make(map[string]string, len(names))
	for _, name := range names {
		v, err := s.backend.Get(prefix + name)
		if errors.Is(err, s.notFound) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read the secret %s: %w", name, err)
		}
		secrets[name] = string(v)
	}
	return secrets, nil
}

// Set keeps value under name, or forgets name when value is empty.
func (s *Store) Set(name, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	names, err := s.names()
	if err != nil {
		return err
	}
	if value == "" {
		if err := s.backend.Delete(prefix + name); err != nil && !errors.Is(err, s.notFound) {
			return fmt.Errorf("delete the secret %s: %w", name, err)
		}
		return s.write(slices.DeleteFunc(names, func(n string) bool { return n == name }))
	}
	if err := s.backend.Set(prefix+name, []byte(value)); err != nil {
		return fmt.Errorf("keep the secret %s: %w", name, err)
	}
	if slices.Contains(names, name) {
		return nil
	}
	return s.write(append(names, name))
}

func (s *Store) names() ([]string, error) {
	b, err := os.ReadFile(s.index)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read the secrets' index: %w", err)
	}
	var names []string
	if err := json.Unmarshal(b, &names); err != nil {
		return nil, fmt.Errorf("read the secrets' index: %w", err)
	}
	return names, nil
}

func (s *Store) write(names []string) error {
	b, err := json.Marshal(names)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.index), 0o700); err != nil {
		return fmt.Errorf("write the secrets' index: %w", err)
	}
	tmp := s.index + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return fmt.Errorf("write the secrets' index: %w", err)
	}
	if err := os.Rename(tmp, s.index); err != nil {
		return fmt.Errorf("write the secrets' index: %w", err)
	}
	return nil
}
