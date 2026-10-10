package plugins

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/apps/server/internal/activity"
	"github.com/mavioai/mavio/apps/server/internal/auth"
	"github.com/mavioai/mavio/libs/core"
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
	// no plugins, and none can be installed.
	Dir string
	// CacheDir keeps compiled WASM modules across restarts; empty keeps
	// them in memory.
	CacheDir string
	// DataDir holds the plugins' data folders, kept across upgrades and
	// deleted on uninstall; empty gives plugins none.
	DataDir string
	Store   core.Store
	// Catalogs returns the URLs of the plugin catalogs; nil means none.
	Catalogs func() []string
	// Client downloads catalogs and plugin packages; nil uses one with a
	// five-minute timeout.
	Client *http.Client
	// HostAPI serves the plugins' host API requests, which it sees with the
	// plugin's grant (auth.PluginHandler); nil serves none.
	HostAPI http.Handler
	// Activity records plugin failures and task runs; nil records none.
	Activity *activity.Log
	// Now is the clock of task schedules; nil means time.Now.
	Now    func() time.Time
	Logger *slog.Logger
}

// Manager runs the plugins of a plugin folder, and installs, upgrades and
// uninstalls them while the server runs.
type Manager struct {
	cfg   Config
	store core.Store
	log   *slog.Logger
	opts  host.Options

	// install serializes installations, which take long, apart from mu.
	install sync.Mutex

	// ctx ends with Close, stopping event delivery.
	ctx    context.Context
	cancel context.CancelFunc

	mu      sync.Mutex
	plugins []*entry // by ID
	// queues hold the events waiting for each event consumer.
	queues map[string]*eventQueue
}

type entry struct {
	// folder is the plugin's folder in the plugin folder.
	folder   string
	manifest *pluginv1.Manifest
	plugin   host.Plugin // nil when it failed to start
	state    State
	err      error
}

// Open starts the plugins of cfg.Dir and delivers their stored
// configurations. A plugin that fails is reported by Plugins and left out;
// only an unreadable folder is an error.
func Open(ctx context.Context, cfg Config) (*Manager, error) {
	if cfg.Client == nil {
		cfg.Client = &http.Client{Timeout: 5 * time.Minute}
	}
	m := &Manager{cfg: cfg, store: cfg.Store, log: cmp.Or(cfg.Logger, slog.New(slog.DiscardHandler)), queues: map[string]*eventQueue{}}
	// Event delivery outlives the request that installs a plugin.
	m.ctx, m.cancel = context.WithCancel(context.WithoutCancel(ctx))
	m.opts = host.Options{
		WASM:    wasm.Options{CacheDir: cfg.CacheDir, Logger: m.log},
		Process: process.Options{Logger: m.log},
		DataDir: cfg.DataDir,
	}
	if cfg.HostAPI != nil {
		m.opts.HostAPI = func(man *pluginv1.Manifest) http.Handler {
			return auth.PluginHandler(auth.Plugin{
				ID: man.GetId(),
				Allows: func(procedure string, readOnly bool) bool {
					return manifest.AllowsProcedure(man, procedure, readOnly)
				},
				ActAsUsers: man.GetPermissions().GetActAsUsers(),
			}, cfg.HostAPI)
		}
	}
	if cfg.Dir == "" {
		return m, nil
	}
	dirs, err := os.ReadDir(cfg.Dir)
	if err != nil {
		return nil, fmt.Errorf("plugin folder: %w", err)
	}
	seen := map[string]bool{}
	for _, d := range dirs {
		// Folders of installations in progress start with a dot.
		if !d.IsDir() || strings.HasPrefix(d.Name(), ".") {
			continue
		}
		if _, err := os.Stat(filepath.Join(cfg.Dir, d.Name(), manifest.File)); err != nil {
			continue
		}
		e := m.start(ctx, d.Name(), seen)
		seen[e.manifest.GetId()] = true
		m.plugins = append(m.plugins, e)
	}
	m.sort()
	return m, nil
}

func (m *Manager) now() time.Time {
	if m.cfg.Now != nil {
		return m.cfg.Now()
	}
	return time.Now()
}

func (m *Manager) sort() {
	slices.SortFunc(m.plugins, func(a, b *entry) int { return cmp.Compare(a.manifest.GetId(), b.manifest.GetId()) })
}

// start starts the plugin in a folder of the plugin folder and delivers
// its configuration; seen are the IDs other folders hold.
func (m *Manager) start(ctx context.Context, folder string, seen map[string]bool) *entry {
	dir := filepath.Join(m.cfg.Dir, folder)
	e := &entry{folder: folder}
	e.manifest, e.err = manifest.Load(dir)
	switch {
	case e.err != nil:
		e.manifest = pluginv1.Manifest_builder{Id: new(folder)}.Build()
	case seen[e.manifest.GetId()]:
		e.err = fmt.Errorf("another folder holds plugin %s", e.manifest.GetId())
	default:
		e.plugin, e.err = host.Open(ctx, dir, m.opts)
	}
	if e.err == nil {
		e.state, e.err = m.configure(ctx, e)
	}
	if e.err != nil {
		e.state = Failed
		m.log.ErrorContext(ctx, "plugin failed", "plugin", e.manifest.GetId(), "err", e.err)
		m.cfg.Activity.Record(ctx, core.Activity{
			Type: "plugin.failed", Severity: core.SeverityError, Title: "Plugin " + e.manifest.GetId() + " failed to start",
			Message: e.err.Error(), Attributes: map[string]string{"plugin": e.manifest.GetId()},
		})
	} else {
		m.scheduleTasks(ctx, e.manifest)
		m.log.InfoContext(ctx, "plugin started", "plugin", e.manifest.GetId(), "version", e.manifest.GetVersion(),
			"configured", e.state == Ready)
	}
	return e
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

// running returns a ready plugin by ID.
func (m *Manager) running(id string) (host.Plugin, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, err := m.find(id)
	if err != nil || e.plugin == nil || e.state != Ready {
		return nil, false
	}
	return e.plugin, true
}

// Routes returns the handler of a started plugin's HTTP routes, or nil.
// Unconfigured plugins serve them too, for their configuration pages.
func (m *Manager) Routes(id string) http.Handler {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, err := m.find(id)
	if err != nil || e.plugin == nil || e.state == Failed {
		return nil
	}
	return e.plugin.HTTP()
}

// withCapability lists the IDs of the started plugins declaring c.
func (m *Manager) withCapability(c pluginv1.Capability) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []string
	for _, e := range m.plugins {
		if e.plugin != nil && manifest.HasCapability(e.manifest, c) {
			out = append(out, e.manifest.GetId())
		}
	}
	return out
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

// Close stops the plugins, dropping the events they wait for.
func (m *Manager) Close(ctx context.Context) error {
	m.cancel()
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
