package plugins

import (
	"context"
	"fmt"
	"maps"
	"slices"
)

// SetEnabled disables a plugin, stopping it while it stays installed with
// its configuration, or enables it again, starting it. The choice is
// stored, so that it holds across restarts and upgrades.
func (m *Manager) SetEnabled(ctx context.Context, id string, enabled bool) (Info, error) {
	m.install.Lock()
	defer m.install.Unlock()
	m.mu.Lock()
	e, err := m.find(id)
	if err != nil || enabled != m.disabled[id] {
		// Unknown, or already as asked.
		var info Info
		if e != nil {
			info = e.info()
		}
		m.mu.Unlock()
		return info, err
	}
	next := maps.Clone(m.disabled)
	if enabled {
		delete(next, id)
	} else {
		next[id] = true
	}
	m.mu.Unlock()
	if err := m.saveDisabled(ctx, next); err != nil {
		return Info{}, err
	}

	if enabled {
		m.mu.Lock()
		m.disabled = next
		m.mu.Unlock()
		n := m.start(ctx, e.folder, m.otherIDs(id))
		m.put(n)
		m.log.InfoContext(ctx, "plugin enabled", "plugin", id)
		return n.info(), nil
	}
	m.mu.Lock()
	m.disabled = next
	p := e.plugin
	e.plugin, e.state, e.err, e.revision = nil, Disabled, nil, m.revisions.Add(1)
	m.dropQueue(id)
	info := e.info()
	m.mu.Unlock()
	m.dropDevices(ctx, id)
	if p != nil {
		if err := p.Close(ctx); err != nil {
			m.log.WarnContext(ctx, "stop plugin", "plugin", id, "err", err)
		}
	}
	m.log.InfoContext(ctx, "plugin disabled", "plugin", id)
	return info, nil
}

// forgetDisabled drops an uninstalled plugin from the disabled ones, so
// that installing it again starts it.
func (m *Manager) forgetDisabled(ctx context.Context, id string) error {
	m.mu.Lock()
	if !m.disabled[id] {
		m.mu.Unlock()
		return nil
	}
	next := maps.Clone(m.disabled)
	delete(next, id)
	m.mu.Unlock()
	if err := m.saveDisabled(ctx, next); err != nil {
		return err
	}
	m.mu.Lock()
	m.disabled = next
	m.mu.Unlock()
	return nil
}

func (m *Manager) saveDisabled(ctx context.Context, ids map[string]bool) error {
	if m.cfg.SaveDisabled == nil {
		return nil
	}
	if err := m.cfg.SaveDisabled(ctx, slices.Sorted(maps.Keys(ids))); err != nil {
		return fmt.Errorf("store the disabled plugins: %w", err)
	}
	return nil
}

func (m *Manager) isDisabled(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.disabled[id]
}
