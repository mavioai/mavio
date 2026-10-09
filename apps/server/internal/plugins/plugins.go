package plugins

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/apps/server/internal/providers"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
	"github.com/mavioai/mavio/libs/metadata"
	"github.com/mavioai/mavio/libs/plugin/host"
	"github.com/mavioai/mavio/libs/plugin/host/process"
	"github.com/mavioai/mavio/libs/plugin/host/wasm"
	"github.com/mavioai/mavio/libs/plugin/manifest"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

// State is whether a plugin is in use.
type State int

// Plugin states.
const (
	// Ready plugins run with their configuration, or need none.
	Ready State = iota + 1
	// Unconfigured plugins run but wait for a configuration their schema
	// requires.
	Unconfigured
	// Failed plugins could not be started or rejected their stored
	// configuration.
	Failed
)

// Info describes a plugin found in the plugin folder.
type Info struct {
	// Manifest is the plugin's manifest; for a folder whose manifest
	// cannot be read, only the ID is set, from the folder's name.
	Manifest *pluginv1.Manifest
	State    State
	// Err is why the plugin failed.
	Err error
}

// Config configures a Manager.
type Config struct {
	// Dir holds one folder per plugin, with its manifest.json; empty means
	// no plugins.
	Dir string
	// CacheDir keeps compiled WASM modules across restarts; empty keeps
	// them in memory.
	CacheDir string
	Store    core.Store
	Logger   *slog.Logger
}

// Manager runs the plugins of a plugin folder.
type Manager struct {
	store core.Store
	log   *slog.Logger

	mu      sync.Mutex
	plugins []*entry // by ID
}

type entry struct {
	manifest *pluginv1.Manifest
	plugin   host.Plugin // nil when it failed to start
	state    State
	err      error
}

// Open starts the plugins of cfg.Dir and delivers their stored
// configurations. A plugin that fails is reported by Plugins and left out;
// only an unreadable folder is an error.
func Open(ctx context.Context, cfg Config) (*Manager, error) {
	m := &Manager{store: cfg.Store, log: cmp.Or(cfg.Logger, slog.New(slog.DiscardHandler))}
	if cfg.Dir == "" {
		return m, nil
	}
	dirs, err := os.ReadDir(cfg.Dir)
	if err != nil {
		return nil, fmt.Errorf("plugin folder: %w", err)
	}
	opts := host.Options{
		WASM:    wasm.Options{CacheDir: cfg.CacheDir, Logger: m.log},
		Process: process.Options{Logger: m.log},
	}
	seen := map[string]bool{}
	for _, d := range dirs {
		dir := filepath.Join(cfg.Dir, d.Name())
		if !d.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, "manifest.json")); err != nil {
			continue
		}
		e := &entry{}
		e.manifest, e.err = manifest.Load(dir)
		switch {
		case e.err != nil:
			e.manifest = pluginv1.Manifest_builder{Id: new(d.Name())}.Build()
		case seen[e.manifest.GetId()]:
			e.err = fmt.Errorf("another folder holds plugin %s", e.manifest.GetId())
		default:
			e.plugin, e.err = host.Open(ctx, dir, opts)
		}
		seen[e.manifest.GetId()] = true
		if e.err == nil {
			e.state, e.err = m.configure(ctx, e)
		}
		if e.err != nil {
			e.state = Failed
			m.log.ErrorContext(ctx, "plugin failed", "plugin", e.manifest.GetId(), "err", e.err)
		} else {
			m.log.InfoContext(ctx, "plugin started", "plugin", e.manifest.GetId(), "version", e.manifest.GetVersion(),
				"configured", e.state == Ready)
		}
		m.plugins = append(m.plugins, e)
	}
	slices.SortFunc(m.plugins, func(a, b *entry) int { return cmp.Compare(a.manifest.GetId(), b.manifest.GetId()) })
	return m, nil
}

// configure delivers the stored configuration of a started plugin. A
// plugin never configured is ready when its schema accepts an empty
// configuration.
func (m *Manager) configure(ctx context.Context, e *entry) (State, error) {
	c, err := m.store.PluginConfigs().Get(ctx, e.manifest.GetId())
	switch {
	case errors.Is(err, core.ErrNotFound):
		if manifest.ValidateConfig(e.manifest, "{}") != nil {
			return Unconfigured, nil
		}
		return Ready, nil
	case err != nil:
		return Failed, err
	}
	if err := host.Configure(ctx, e.plugin, c.JSON); err != nil {
		return Failed, fmt.Errorf("stored configuration: %w", err)
	}
	return Ready, nil
}

// Plugins lists the plugins of the folder by ID.
func (m *Manager) Plugins() []Info {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Info, len(m.plugins))
	for i, e := range m.plugins {
		out[i] = e.info()
	}
	return out
}

func (e *entry) info() Info { return Info{Manifest: e.manifest, State: e.state, Err: e.err} }

func (m *Manager) find(id string) (*entry, error) {
	i := slices.IndexFunc(m.plugins, func(e *entry) bool { return e.manifest.GetId() == id })
	if i < 0 {
		return nil, fmt.Errorf("plugin %s: %w", id, core.ErrNotFound)
	}
	return m.plugins[i], nil
}

// Config returns a plugin's stored configuration; its JSON is empty when
// it was never configured.
func (m *Manager) Config(ctx context.Context, id string) (core.PluginConfig, error) {
	m.mu.Lock()
	_, err := m.find(id)
	m.mu.Unlock()
	if err != nil {
		return core.PluginConfig{}, err
	}
	c, err := m.store.PluginConfigs().Get(ctx, id)
	if errors.Is(err, core.ErrNotFound) {
		return core.PluginConfig{PluginID: id}, nil
	}
	return c, err
}

// SetConfig validates configJSON against the plugin's schema, delivers it
// to the running plugin and stores it once the plugin accepts it. The
// plugin is ready afterwards, even if its stored configuration had failed.
func (m *Manager) SetConfig(ctx context.Context, id, configJSON string) (Info, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, err := m.find(id)
	if err != nil {
		return Info{}, err
	}
	if e.plugin == nil {
		return Info{}, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("plugin %s is not running: %w", id, e.err))
	}
	if err := manifest.ValidateConfig(e.manifest, configJSON); err != nil {
		return Info{}, fmt.Errorf("%w: plugin %s: %w", core.ErrInvalid, id, err)
	}
	if err := host.Configure(ctx, e.plugin, configJSON); err != nil {
		if connect.CodeOf(err) == connect.CodeInvalidArgument {
			return Info{}, fmt.Errorf("%w: plugin %s rejected the configuration: %w", core.ErrInvalid, id, err)
		}
		return Info{}, fmt.Errorf("configure plugin %s: %w", id, err)
	}
	if err := m.store.PluginConfigs().Put(ctx, &core.PluginConfig{PluginID: id, JSON: configJSON}); err != nil {
		return Info{}, err
	}
	e.state, e.err = Ready, nil
	m.log.InfoContext(ctx, "plugin configured", "plugin", id)
	return e.info(), nil
}

// MetadataProviders returns the metadata providers among the started
// plugins. A provider whose plugin is not ready knows nothing, so that a
// plugin configured later takes part without a restart.
func (m *Manager) MetadataProviders() []library.Provider {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []library.Provider
	for _, e := range m.plugins {
		if e.plugin == nil || e.plugin.Metadata() == nil {
			continue
		}
		out = append(out, &provider{
			m: m, e: e,
			p: &providers.Plugin{ID: e.manifest.GetId(), Client: e.plugin.Metadata()},
		})
	}
	return out
}

// Close stops the plugins.
func (m *Manager) Close(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	var errs []error
	for _, e := range m.plugins {
		if e.plugin != nil {
			errs = append(errs, e.plugin.Close(ctx))
		}
	}
	return errors.Join(errs...)
}

// provider is a plugin's metadata provider that knows nothing until the
// plugin is ready.
type provider struct {
	m *Manager
	e *entry
	p *providers.Plugin
}

func (p *provider) Name() string { return p.p.Name() }

func (p *provider) Metadata(ctx context.Context, l library.Lookup) (*metadata.Result, error) {
	p.m.mu.Lock()
	ready := p.e.state == Ready
	p.m.mu.Unlock()
	if !ready {
		return nil, nil
	}
	return p.p.Metadata(ctx, l)
}
