// Package settings keeps the server settings administrators change while
// the server runs, and applies each change to the parts of the server it
// concerns before storing it.
package settings

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/mavioai/mavio/libs/core"
)

// Applier puts settings into effect; an error rejects them.
type Applier func(ctx context.Context, s core.ServerSettings) error

// Manager holds the current settings.
type Manager struct {
	store core.Store

	mu       sync.Mutex
	current  core.ServerSettings
	appliers []Applier
}

// Open loads the stored settings.
func Open(ctx context.Context, store core.Store) (*Manager, error) {
	s, err := store.Settings().Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("load settings: %w", err)
	}
	return &Manager{store: store, current: s}, nil
}

// Get returns the current settings.
func (m *Manager) Get() core.ServerSettings {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.current
}

// Register applies the current settings with a, and every later change.
// When the current settings fail, as when stored settings name hardware
// gone since, the error is returned and a still gets later changes.
func (m *Manager) Register(ctx context.Context, a Applier) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.appliers = append(m.appliers, a)
	return a(ctx, m.current)
}

// Update applies new settings and stores them. When an applier rejects
// them, the appliers that took them get the current settings back and the
// error is returned.
func (m *Manager) Update(ctx context.Context, s core.ServerSettings) (core.ServerSettings, error) {
	if err := s.Validate(); err != nil {
		return core.ServerSettings{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	restore := func(applied []Applier, cause error) error {
		var errs []error
		for _, undo := range applied {
			errs = append(errs, undo(ctx, m.current))
		}
		if err := errors.Join(errs...); err != nil {
			return fmt.Errorf("%w; restoring the settings: %w", cause, err)
		}
		return cause
	}
	for i, a := range m.appliers {
		if err := a(ctx, s); err != nil {
			return core.ServerSettings{}, restore(m.appliers[:i], err)
		}
	}
	if err := m.store.Settings().Put(ctx, &s); err != nil {
		return core.ServerSettings{}, restore(m.appliers, err)
	}
	m.current = s
	return s, nil
}
