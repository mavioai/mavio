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
	"sync/atomic"
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
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1/pluginv1connect"
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
	// Disabled plugins are installed but not started, as an administrator
	// chose.
	Disabled
	// Starting plugins run but have not been given their stored
	// configuration yet (Config.ConfigureLater).
	Starting
)

// Info describes a plugin found in the plugin folder.
type Info struct {
	// Manifest is the plugin's manifest; for a folder whose manifest
	// cannot be read, only the ID is set, from the folder's name.
	Manifest *pluginv1.Manifest
	// Locales are the translations of the plugin's strings.
	Locales manifest.Locales
	State   State
	// Err is why the plugin failed.
	Err error
	// DevFolder is the folder a plugin under development runs from, in
	// place (Config.Dev); empty for an installed plugin.
	DevFolder string
}

// Config configures a Manager.
type Config struct {
	// Dir holds one folder per plugin, with its manifest.json; empty means
	// no plugins, and none can be installed.
	Dir string
	// Dev are the plugins under development, run in place from their
	// folders and restarted when their files change: a plugin's project,
	// whose development folder dist/dev its dev target builds, or a plugin
	// folder. Each takes the place of an installed plugin of its ID.
	Dev []string
	// DevPoll is how often the development folders are checked for
	// changes; zero means every half second.
	DevPoll time.Duration
	// CacheDir keeps compiled WASM modules across restarts; empty keeps
	// them in memory.
	CacheDir string
	// DataDir holds the plugins' data folders, kept across upgrades and
	// deleted on uninstall; empty gives plugins none.
	DataDir string
	// DeviceID is the UUID plugins are given for this machine
	// (identity.DeviceID); empty gives none.
	DeviceID string
	Store    core.Store
	// Catalogs returns the URLs of the plugin catalogs; nil means none.
	Catalogs func() []string
	// Disabled are the IDs of the plugins not to start.
	Disabled []string
	// SaveDisabled stores the IDs of the plugins not to start, when they
	// change; nil stores nothing.
	SaveDisabled func(ctx context.Context, ids []string) error
	// Client downloads catalogs and plugin packages; nil uses one with a
	// five-minute timeout.
	Client *http.Client
	// Fetch performs the WASM plugins' outbound HTTP requests; nil uses a
	// client with a 30-second timeout.
	Fetch *http.Client
	// HostAPI serves the plugins' host API requests, which it sees with the
	// plugin's grant (auth.PluginHandler); nil serves none.
	HostAPI http.Handler
	// ConfigureLater makes Open start the plugins without delivering their
	// stored configurations, which ConfigureStarted delivers: plugins may
	// call the host API while configured, and a HostAPI that serves only
	// once Open returns would never answer them.
	ConfigureLater bool
	// Env returns what a process plugin's environment adds to the
	// server's when it starts; nil adds nothing.
	Env func(m *pluginv1.Manifest) []string
	// Track records each process plugin's process started until it exits;
	// nil records none.
	Track func(pid int) (done func())
	// Activity records plugin failures and task runs; nil records none.
	Activity *activity.Log
	// DevicesChanged is told the sessions of the devices that plugins
	// listed, changed or no longer list; nil tells nobody.
	DevicesChanged func(sessions []core.ID)
	// Containers runs the plugins' managed container dependencies while
	// the plugins run; nil runs none.
	Containers ContainerHooks
	// Now is the clock of task schedules; nil means time.Now.
	Now    func() time.Time
	Logger *slog.Logger
}

// ContainerHooks are told when plugins start and stop, to run their
// managed container dependencies meanwhile. They return without waiting
// for the containers.
type ContainerHooks interface {
	// PluginStarted is told of a plugin started, upgraded or enabled.
	PluginStarted(ctx context.Context, m *pluginv1.Manifest)
	// PluginStopped is told of a plugin disabled.
	PluginStopped(ctx context.Context, id string)
	// PluginRemoved is told of a plugin uninstalled.
	PluginRemoved(ctx context.Context, id string)
	// AllowsHost reports whether host is that of one of a plugin's
	// dependencies' addresses, which its WASM module may fetch.
	AllowsHost(ctx context.Context, id, host string) bool
}

// allowHost lets WASM plugins fetch their container dependencies.
func (c Config) allowHost(ctx context.Context, id, host string) bool {
	return c.Containers != nil && c.Containers.AllowsHost(ctx, id, host)
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
	// setConfig serializes SetConfig, so that the stored configuration is
	// the one delivered last; unlike mu, it is held while storing.
	// Uninstall holds it too, after install, so that no configuration is
	// stored for a plugin it removed.
	setConfig sync.Mutex

	// ctx ends with Close, stopping event delivery.
	ctx    context.Context
	cancel context.CancelFunc

	mu      sync.Mutex
	plugins []*entry // by ID
	// disabled holds the IDs of the plugins not to start.
	disabled map[string]bool
	// queues hold the events waiting for each event consumer.
	queues map[string]*eventQueue

	// manifests holds the manifests of the plugins started, by ID, apart
	// from mu: plugins ask for what their manifests declare while mu is
	// held to configure them.
	manifests sync.Map

	// devMu guards devices, the devices of each device controller.
	devMu   sync.Mutex
	devices map[string][]Device

	// revisions numbers the plugins' starts and configurations.
	revisions atomic.Uint64

	// runs holds the tasks' runs in progress.
	runs taskRuns
}

type entry struct {
	// folder is the plugin's folder in the plugin folder, or for a plugin
	// under development the absolute path of its project or folder.
	folder   string
	dev      bool
	manifest *pluginv1.Manifest
	locales  manifest.Locales
	plugin   host.Plugin // nil when it failed to start
	state    State
	err      error
	// revision changes when the plugin starts and is configured, for
	// those keeping what it provides.
	revision uint64
}

// Open starts the plugins of cfg.Dir and delivers their stored
// configurations. A plugin that fails is reported by Plugins and left out;
// only an unreadable folder is an error.
func Open(ctx context.Context, cfg Config) (*Manager, error) {
	if cfg.Client == nil {
		cfg.Client = &http.Client{Timeout: 5 * time.Minute}
	}
	m := &Manager{cfg: cfg, store: cfg.Store, log: cmp.Or(cfg.Logger, slog.New(slog.DiscardHandler)), queues: map[string]*eventQueue{}, devices: map[string][]Device{}, disabled: map[string]bool{}}
	for _, id := range cfg.Disabled {
		m.disabled[id] = true
	}
	// Event delivery outlives the request that installs a plugin.
	m.ctx, m.cancel = context.WithCancel(context.WithoutCancel(ctx))
	m.opts = host.Options{
		WASM:     wasm.Options{CacheDir: cfg.CacheDir, FetchClient: cfg.Fetch, AllowHost: cfg.allowHost, Logger: m.log},
		Process:  process.Options{Track: cfg.Track, Logger: m.log},
		DataDir:  cfg.DataDir,
		DeviceID: cfg.DeviceID,
		Env:      cfg.Env,
	}
	if cfg.HostAPI != nil {
		m.opts.HostAPI = func(man *pluginv1.Manifest) http.Handler {
			return auth.PluginHandler(auth.Plugin{
				ID: man.GetId(),
				Allows: func(procedure string, readOnly bool) bool {
					// Every plugin may call the host service.
					return strings.HasPrefix(strings.TrimPrefix(procedure, "/"), pluginv1connect.HostServiceName+"/") ||
						manifest.AllowsProcedure(man, procedure, readOnly)
				},
				ActAsUsers: man.GetPermissions().GetActAsUsers(),
				Device: func(id string) (core.AuthSession, bool) {
					return m.deviceSession(man.GetId(), id)
				},
			}, cfg.HostAPI)
		}
	}
	seen := map[string]bool{}
	add := func(e *entry) {
		if !cfg.ConfigureLater {
			m.ready(ctx, e)
		}
		seen[e.manifest.GetId()] = true
		m.plugins = append(m.plugins, e)
	}
	for _, p := range cfg.Dev {
		add(m.launch(ctx, p, seen))
	}
	if cfg.Dir == "" {
		m.sort()
		m.startWatching()
		return m, nil
	}
	dirs, err := os.ReadDir(cfg.Dir)
	if err != nil {
		return nil, fmt.Errorf("plugin folder: %w", err)
	}
	for _, d := range dirs {
		// Folders of installations in progress start with a dot.
		if !d.IsDir() || strings.HasPrefix(d.Name(), ".") {
			continue
		}
		dir := filepath.Join(cfg.Dir, d.Name())
		if _, err := os.Stat(filepath.Join(dir, manifest.File)); err != nil {
			continue
		}
		if mf, err := manifest.Load(dir); err == nil && m.isDev(mf.GetId()) {
			m.log.InfoContext(ctx, "installed plugin left for its development folder", "plugin", mf.GetId())
			continue
		}
		add(m.launch(ctx, d.Name(), seen))
	}
	m.sort()
	m.startWatching()
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
// its configuration, unless it is disabled; seen are the IDs other folders
// hold.
func (m *Manager) start(ctx context.Context, folder string, seen map[string]bool) *entry {
	e := m.launch(ctx, folder, seen)
	m.ready(ctx, e)
	return e
}

// ConfigureStarted delivers the stored configurations of the plugins Open
// started with Config.ConfigureLater, in turn.
func (m *Manager) ConfigureStarted(ctx context.Context) {
	m.mu.Lock()
	var starting []*entry
	for _, e := range m.plugins {
		if e.state == Starting {
			starting = append(starting, e)
		}
	}
	m.mu.Unlock()
	for _, e := range starting {
		m.ready(ctx, e)
	}
}

// launch starts the plugin in a folder of the plugin folder, or a plugin
// under development by the absolute path of its project or folder, unless
// it is disabled, leaving it Starting; seen are the IDs other folders hold.
func (m *Manager) launch(ctx context.Context, folder string, seen map[string]bool) *entry {
	e := &entry{folder: folder, dev: filepath.IsAbs(folder), revision: m.revisions.Add(1)}
	dir := m.dir(e)
	e.manifest, e.err = manifest.Load(dir)
	var localesErr error
	if e.err == nil {
		e.locales, localesErr = manifest.LoadLocales(dir)
	}
	switch {
	case e.err != nil:
		e.manifest = pluginv1.Manifest_builder{Id: new(filepath.Base(folder))}.Build()
	case seen[e.manifest.GetId()]:
		e.err = fmt.Errorf("another folder holds plugin %s", e.manifest.GetId())
	case m.isDisabled(e.manifest.GetId()):
		m.manifests.Store(e.manifest.GetId(), e.manifest)
		e.state = Disabled
		m.log.InfoContext(ctx, "plugin disabled", "plugin", e.manifest.GetId(), "version", e.manifest.GetVersion())
		return e
	case localesErr != nil:
		e.err = localesErr
	default:
		m.manifests.Store(e.manifest.GetId(), e.manifest)
		e.plugin, e.err = host.Open(ctx, dir, m.opts)
	}
	if e.err != nil {
		m.fail(ctx, e, e.err)
		return e
	}
	m.pruneContainers(ctx, e.manifest)
	if m.cfg.Containers != nil {
		m.cfg.Containers.PluginStarted(ctx, e.manifest)
	}
	e.state = Starting
	return e
}

// ready delivers the stored configuration of a plugin launch started, and
// schedules its tasks.
func (m *Manager) ready(ctx context.Context, e *entry) {
	if e.state != Starting {
		return
	}
	state, err := m.configure(ctx, e)
	if err != nil {
		m.fail(ctx, e, err)
		return
	}
	m.mu.Lock()
	e.state = state
	m.mu.Unlock()
	m.scheduleTasks(ctx, e.manifest)
	m.log.InfoContext(ctx, "plugin started", "plugin", e.manifest.GetId(), "version", e.manifest.GetVersion(),
		"configured", state == Ready)
}

// fail records why a plugin failed to start.
func (m *Manager) fail(ctx context.Context, e *entry, err error) {
	m.mu.Lock()
	e.state, e.err = Failed, err
	m.mu.Unlock()
	m.log.ErrorContext(ctx, "plugin failed", "plugin", e.manifest.GetId(), "err", err)
	m.cfg.Activity.Record(ctx, core.Activity{
		Type: "plugin.failed", Severity: core.SeverityError, Title: "Plugin " + e.manifest.GetId() + " failed to start",
		Message: err.Error(), Attributes: map[string]string{"plugin": e.manifest.GetId()},
	})
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

func (e *entry) info() Info {
	info := Info{Manifest: e.manifest, Locales: e.locales, State: e.state, Err: e.err}
	if e.dev {
		info.DevFolder = devFolder(e.folder)
	}
	return info
}

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
// to the running plugin and stores it once the plugin accepts it,
// returning the configuration stored: the properties the schema marks
// generated are given new values where configJSON lacks them. The plugin
// is ready afterwards, even if its stored configuration had failed.
//
// m.mu is not held while the configuration is stored: a store transaction
// holding SQLite's one write connection may wait on m.mu (publishing an
// event asks Wants), and waiting on that connection under m.mu would
// deadlock with it.
func (m *Manager) SetConfig(ctx context.Context, id, configJSON string) (Info, string, error) {
	m.setConfig.Lock()
	defer m.setConfig.Unlock()
	e, configJSON, err := m.deliverConfig(ctx, id, configJSON)
	if err != nil {
		return Info{}, "", err
	}
	if err := m.store.PluginConfigs().Put(ctx, &core.PluginConfig{PluginID: id, JSON: configJSON}); err != nil {
		return Info{}, "", err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	// The plugin may have been replaced or stopped meanwhile; its
	// successor reads the stored configuration when it starts.
	if cur, err := m.find(id); err == nil && cur == e && e.plugin != nil {
		e.state, e.err = Ready, nil
	}
	m.log.InfoContext(ctx, "plugin configured", "plugin", id)
	return e.info(), configJSON, nil
}

// Manifest returns the manifest of a plugin started or disabled, without
// waiting for the plugins being started or configured.
func (m *Manager) Manifest(id string) (*pluginv1.Manifest, bool) {
	v, ok := m.manifests.Load(id)
	if !ok {
		return nil, false
	}
	return v.(*pluginv1.Manifest), true
}

// Reconfigure delivers a ready plugin's stored configuration again,
// telling it that what it reads from the host, such as the addresses of
// its container dependencies, changed. A plugin not ready is left alone.
func (m *Manager) Reconfigure(ctx context.Context, id string) error {
	m.setConfig.Lock()
	defer m.setConfig.Unlock()
	c, err := m.Config(ctx, id)
	if err != nil {
		return err
	}
	configJSON := c.JSON
	if configJSON == "" {
		configJSON = "{}"
	}
	m.mu.Lock()
	e, err := m.find(id)
	ready := err == nil && e.plugin != nil && e.state == Ready
	m.mu.Unlock()
	if err != nil || !ready {
		return err
	}
	if err := host.Configure(ctx, e.plugin, configJSON); err != nil {
		return fmt.Errorf("reconfigure plugin %s: %w", id, err)
	}
	return nil
}

// pruneContainers deletes what is kept of container dependencies a
// plugin's manifest no longer declares.
func (m *Manager) pruneContainers(ctx context.Context, mf *pluginv1.Manifest) {
	kept, err := m.store.PluginContainers().List(ctx, mf.GetId())
	if err != nil {
		m.log.WarnContext(ctx, "list plugin containers", "plugin", mf.GetId(), "err", err)
		return
	}
	for _, c := range kept {
		if _, ok := manifest.Container(mf, c.Name); ok {
			continue
		}
		if err := m.store.PluginContainers().Delete(ctx, mf.GetId(), c.Name); err != nil {
			m.log.WarnContext(ctx, "delete plugin container", "plugin", mf.GetId(), "container", c.Name, "err", err)
		}
	}
}

// deliverConfig gives the properties a running plugin's schema marks
// generated new values where configJSON lacks them, validates it against
// the schema and delivers it to the plugin, returning the plugin's entry
// and the configuration delivered.
func (m *Manager) deliverConfig(ctx context.Context, id, configJSON string) (*entry, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, err := m.find(id)
	if err != nil {
		return nil, "", err
	}
	if e.plugin == nil {
		return nil, "", connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("plugin %s is not running: %w", id, e.err))
	}
	configJSON, err = manifest.GenerateIDs(e.manifest, configJSON)
	if err != nil {
		return nil, "", fmt.Errorf("%w: plugin %s: %w", core.ErrInvalid, id, err)
	}
	if err := manifest.ValidateConfig(e.manifest, configJSON); err != nil {
		return nil, "", fmt.Errorf("%w: plugin %s: %w", core.ErrInvalid, id, err)
	}
	if err := host.Configure(ctx, e.plugin, configJSON); err != nil {
		if connect.CodeOf(err) == connect.CodeInvalidArgument {
			return nil, "", fmt.Errorf("%w: plugin %s rejected the configuration: %w", core.ErrInvalid, id, err)
		}
		return nil, "", fmt.Errorf("configure plugin %s: %w", id, err)
	}
	e.revision = m.revisions.Add(1)
	return e, configJSON, nil
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
