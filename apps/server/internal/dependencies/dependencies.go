package dependencies

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mavioai/mavio/libs/containers"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/plugin/manifest"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

// ErrNoEngine reports that a managed dependency needs a container engine
// and none is available.
var ErrNoEngine = errors.New("no container engine is available")

// healthTTL is how long a health check's result is used.
const healthTTL = 30 * time.Second

// healthTimeout bounds a health check.
const healthTimeout = 3 * time.Second

// Plugins are the server's plugins; *plugins.Manager implements it.
type Plugins interface {
	// Manifest returns a started plugin's manifest.
	Manifest(id string) (*pluginv1.Manifest, bool)
	// Reconfigure tells a ready plugin that its dependencies changed.
	Reconfigure(ctx context.Context, id string) error
}

// Config configures a Service.
type Config struct {
	// Store keeps administrators' choices.
	Store core.PluginContainerRepository
	// Plugins are the server's plugins; Bind sets them when they are
	// opened after the Service.
	Plugins Plugins
	// Server is the server's ID, which labels the containers it runs, and
	// Names names them.
	Server string
	Names  containers.Names
	// Detect finds the container engine; containers.Detect with the
	// server's options in production.
	Detect func(ctx context.Context) (containers.Runtime, error)
	// Engine returns the engine managed dependencies run on; nil uses
	// the detected runtime's client.
	Engine func(rt containers.Runtime) containers.Engine
	// Jobs runs image pulls as JobPull jobs, which RunPull handles; nil
	// pulls at once.
	Jobs core.JobQueue
	// Probe checks a dependency's health; nil uses containers.Probe.
	Probe func(ctx context.Context, address, path string) error
	// Poll is the time between health checks of a starting managed
	// dependency; 0 is the Supervisor's default.
	Poll time.Duration
	// ProxyEnv returns the environment a managed dependency's container
	// reaches sites with through the proxy its choice names, nil when it
	// reaches them directly, choosing none or a system proxy setting none;
	// nil Config.ProxyEnv proxies nothing.
	ProxyEnv func(ctx context.Context, m *pluginv1.Manifest, c core.PluginContainer) func(host string) map[string]string
	// CheckProxy rejects a proxy choice naming no proxy; nil accepts any.
	CheckProxy func(ctx context.Context, id string) error
	// ProxyChanged is told the container of a dependency whose proxy
	// changed, for its connections to go through the new one; nil tells
	// nobody.
	ProxyChanged func(container string)
	// Now returns the time; nil uses time.Now.
	Now    func() time.Time
	Logger *slog.Logger
}

// Service keeps plugins' container dependencies: it runs the managed ones
// on the container engine while their plugins run, and checks the
// external ones.
type Service struct {
	cfg Config
	// ctx ends with Close; managed dependencies start in its background.
	ctx    context.Context
	cancel context.CancelFunc

	mu       sync.Mutex
	plugins  Plugins
	detected bool
	runtime  containers.Runtime
	err      error
	sup      *containers.Supervisor
	health   map[string]health
	// started are the plugins running, whose managed dependencies run.
	started map[string]bool
	// told are the addresses of managed dependencies their plugins were
	// configured with, by key.
	told map[string]string
	// pulls are the images being pulled as jobs, and progress the
	// fraction done of each running JobPull job.
	pulls    map[string]*pull
	progress map[core.ID]float64
}

// health is the result of a health check.
type health struct {
	at    time.Time
	state pluginv1.ContainerState
	err   string
}

// New returns a Service.
func New(cfg Config) *Service {
	if cfg.Probe == nil {
		client := &http.Client{Timeout: healthTimeout}
		cfg.Probe = func(ctx context.Context, address, path string) error {
			return containers.Probe(ctx, client, address, path)
		}
	}
	if cfg.Engine == nil {
		cfg.Engine = func(rt containers.Runtime) containers.Engine { return rt.Client }
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Service{
		cfg: cfg, ctx: ctx, cancel: cancel, plugins: cfg.Plugins,
		health: map[string]health{}, started: map[string]bool{}, told: map[string]string{},
		pulls: map[string]*pull{}, progress: map[core.ID]float64{},
	}
}

// Bind sets the server's plugins.
func (s *Service) Bind(p Plugins) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.plugins = p
}

func (s *Service) boundPlugins() Plugins {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.plugins
}

// Close stops the managed dependencies and closes the engine's client.
func (s *Service) Close(ctx context.Context) error {
	s.cancel()
	s.mu.Lock()
	sup, client := s.sup, s.runtime.Client
	s.sup = nil
	s.mu.Unlock()
	var err error
	if sup != nil {
		err = sup.Close(ctx)
	}
	if client != nil {
		client.Close()
	}
	return err
}

// Runtime returns the container engine found, detecting it on first use
// or when detect is set; the error, wrapping containers.ErrUnavailable,
// tells why there is none. Once managed dependencies run on an engine,
// detecting again keeps it.
func (s *Service) Runtime(ctx context.Context, detect bool) (containers.Runtime, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.detected && (!detect || s.sup != nil) {
		return s.runtime, s.err
	}
	rt, err := s.cfg.Detect(ctx)
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return rt, err
	}
	if s.runtime.Client != nil {
		s.runtime.Client.Close()
	}
	s.detected, s.runtime, s.err = true, rt, err
	if err != nil {
		s.cfg.Logger.InfoContext(ctx, "no container engine", "err", err)
	} else {
		s.cfg.Logger.InfoContext(ctx, "container engine found", "endpoint", rt.Client.Endpoint().String(),
			"engine", rt.Version.Engine, "version", rt.Version.Version, "api", rt.APIVersion, "rootless", rt.Rootless,
			"on_host", rt.OnHost())
	}
	return rt, err
}

// supervisor returns the Supervisor of managed dependencies, detecting
// the engine on first use: ErrNoEngine without one, or for a server in a
// container whose own container the engine does not know.
func (s *Service) supervisor(ctx context.Context) (*containers.Supervisor, error) {
	rt, err := s.Runtime(ctx, false)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrNoEngine, err)
	}
	var self string
	if !rt.OnHost() {
		if rt.Self == nil {
			return nil, fmt.Errorf("%w: the server runs in a container the engine does not know; give the container its own ID as host name", ErrNoEngine)
		}
		self = rt.Self.ID
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sup == nil && s.ctx.Err() == nil {
		s.sup = containers.NewSupervisor(containers.SupervisorOptions{
			Engine: s.cfg.Engine(rt), Pull: s.pullImage(s.cfg.Engine(rt)), Names: s.cfg.Names, Server: s.cfg.Server, Self: self, OnChange: s.changed,
			HostGateway: rt.Version.Engine == "docker",
			Probe:       s.cfg.Probe, Poll: s.cfg.Poll, Logger: s.cfg.Logger,
		})
	}
	if s.sup == nil {
		return nil, fmt.Errorf("%w: the server is stopping", ErrNoEngine)
	}
	return s.sup, nil
}

// changed is told of a managed dependency's status and configures its
// plugin again when the address it reports changed.
func (s *Service) changed(plugin, name string, st containers.Status) {
	addr := managedAddress(st)
	k := key(plugin, name)
	s.mu.Lock()
	old, ok := s.told[k]
	s.told[k] = addr
	p := s.plugins
	s.mu.Unlock()
	if (ok || addr != "") && old != addr && p != nil {
		if err := p.Reconfigure(s.ctx, plugin); err != nil {
			s.cfg.Logger.WarnContext(s.ctx, "reconfigure plugin after its container's address changed", "plugin", plugin, "container", name, "err", err)
		}
	}
}

// managedAddress is the address a plugin is told of a managed dependency:
// none until it is ready.
func managedAddress(st containers.Status) string {
	if st.State == containers.Ready || st.State == containers.Unhealthy {
		return st.Address
	}
	return ""
}

// PluginStarted runs a started plugin's managed dependencies in the
// background and stops the containers of those no longer managed.
func (s *Service) PluginStarted(_ context.Context, m *pluginv1.Manifest) {
	id := m.GetId()
	s.mu.Lock()
	s.started[id] = true
	s.mu.Unlock()
	if len(m.GetContainers()) == 0 {
		return
	}
	go func() {
		if err := s.runPlugin(s.ctx, m); err != nil && s.ctx.Err() == nil {
			s.cfg.Logger.WarnContext(s.ctx, "run plugin's containers", "plugin", id, "err", err)
		}
	}()
}

// runPlugin starts a plugin's managed dependencies and removes the
// containers of the others.
func (s *Service) runPlugin(ctx context.Context, m *pluginv1.Manifest) error {
	kept, err := s.cfg.Store.List(ctx, m.GetId())
	if err != nil {
		return err
	}
	managed := map[string]core.PluginContainer{}
	for _, c := range kept {
		if c.Mode == core.ContainerManaged {
			managed[c.Name] = c
		}
	}
	if len(managed) == 0 {
		return nil
	}
	sup, err := s.supervisor(ctx)
	if err != nil {
		return err
	}
	for _, name := range sup.Running(m.GetId()) {
		if _, ok := managed[name]; !ok {
			if err := sup.Remove(ctx, m.GetId(), name, false); err != nil {
				s.cfg.Logger.WarnContext(ctx, "remove container no longer managed", "plugin", m.GetId(), "container", name, "err", err)
			}
		}
	}
	for _, spec := range m.GetContainers() {
		if c, ok := managed[spec.GetName()]; ok {
			sup.Start(s.resolve(ctx, m, spec, c))
		}
	}
	return nil
}

// resolve is Resolve, with the environment of the dependency's proxy.
func (s *Service) resolve(ctx context.Context, m *pluginv1.Manifest, spec *pluginv1.ContainerDependency, c core.PluginContainer) containers.Dependency {
	d := Resolve(s.cfg.Names, spec, c)
	if s.cfg.ProxyEnv != nil && !spec.GetDirect() {
		d.ProxyEnv = s.cfg.ProxyEnv(ctx, m, c)
	}
	return d
}

// ProxiesChanged starts again the managed dependencies whose containers
// go from reaching sites directly to through a proxy, or back, as the
// system proxy is set or not.
func (s *Service) ProxiesChanged(ctx context.Context) {
	p := s.boundPlugins()
	if p == nil {
		return
	}
	s.mu.Lock()
	ids := slices.Collect(maps.Keys(s.started))
	s.mu.Unlock()
	for _, id := range ids {
		if m, ok := p.Manifest(id); ok && len(m.GetContainers()) > 0 {
			if err := s.runPlugin(ctx, m); err != nil {
				s.cfg.Logger.WarnContext(ctx, "run plugin's containers after proxies changed", "plugin", id, "err", err)
			}
		}
	}
}

// ProxyOf returns the proxy choice of the managed dependency whose
// container is named so, of a running plugin; false for none.
func (s *Service) ProxyOf(ctx context.Context, container string) (string, bool) {
	p := s.boundPlugins()
	if p == nil {
		return "", false
	}
	s.mu.Lock()
	ids := slices.Collect(maps.Keys(s.started))
	s.mu.Unlock()
	for _, id := range ids {
		m, ok := p.Manifest(id)
		if !ok {
			continue
		}
		for _, spec := range m.GetContainers() {
			if s.cfg.Names.Container(id, spec.GetName()) != container {
				continue
			}
			c, err := s.kept(ctx, id, spec.GetName())
			if err != nil || c.Mode != core.ContainerManaged || spec.GetDirect() {
				return "", false
			}
			return c.Proxy, true
		}
	}
	return "", false
}

// PluginStopped stops a plugin's managed dependencies.
func (s *Service) PluginStopped(ctx context.Context, id string) {
	s.mu.Lock()
	delete(s.started, id)
	sup := s.sup
	s.mu.Unlock()
	if sup == nil {
		return
	}
	if err := sup.Stop(ctx, id, ""); err != nil {
		s.cfg.Logger.WarnContext(ctx, "stop plugin's containers", "plugin", id, "err", err)
	}
}

// PluginRemoved removes an uninstalled plugin's containers and volumes.
func (s *Service) PluginRemoved(ctx context.Context, id string) {
	s.mu.Lock()
	delete(s.started, id)
	s.mu.Unlock()
	sup, err := s.supervisor(ctx)
	if err != nil {
		return
	}
	if err := sup.Remove(ctx, id, "", true); err != nil {
		s.cfg.Logger.WarnContext(ctx, "remove plugin's containers", "plugin", id, "err", err)
	}
}

// Reconcile removes the containers the server runs for dependencies no
// longer declared or no longer managed, their volumes staying, and stops
// those of plugins not running that a server before left running. It
// does nothing without an engine.
func (s *Service) Reconcile(ctx context.Context) error {
	sup, err := s.supervisor(ctx)
	if err != nil {
		return nil
	}
	p := s.boundPlugins()
	managed := map[string]map[string]bool{}
	var errs []error
	keep := func(plugin, name string) bool {
		if p == nil {
			return true
		}
		m, ok := p.Manifest(plugin)
		if !ok {
			return false
		}
		if _, ok := manifest.Container(m, name); !ok {
			return false
		}
		names, ok := managed[plugin]
		if !ok {
			kept, err := s.cfg.Store.List(ctx, plugin)
			if err != nil {
				errs = append(errs, err)
				return true
			}
			names = map[string]bool{}
			for _, c := range kept {
				names[c.Name] = c.Mode == core.ContainerManaged
			}
			managed[plugin] = names
		}
		return names[name]
	}
	started := func(plugin string) bool {
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.started[plugin]
	}
	errs = append(errs, sup.Reconcile(ctx, keep, started))
	return errors.Join(errs...)
}

// Resolve returns a managed dependency as it runs: its image, with a GPU
// when it may use one, and its environment with its secrets and its own
// address, http://<container>:<port> on the network it joins.
func Resolve(names containers.Names, spec *pluginv1.ContainerDependency, c core.PluginContainer) containers.Dependency {
	gpu := c.UseGPU && spec.GetGpu() != pluginv1.GpuSupport_GPU_SUPPORT_UNSPECIFIED
	image := spec.GetImage()
	if gpu && spec.GetGpuImage() != "" {
		image = spec.GetGpuImage()
	}
	pairs := []string{"${address}", "http://" + names.Container(c.PluginID, c.Name) + ":" + strconv.Itoa(int(spec.GetPort()))}
	for k, v := range c.Secrets {
		pairs = append(pairs, "${secret:"+k+"}", v)
	}
	r := strings.NewReplacer(pairs...)
	env := make(map[string]string, len(spec.GetEnv()))
	for k, v := range spec.GetEnv() {
		env[k] = r.Replace(v)
	}
	return containers.Dependency{
		Plugin: c.PluginID, Name: c.Name, Image: image, Cmd: spec.GetCommand(), Env: env,
		Port: int(spec.GetPort()), HealthPath: spec.GetHealthPath(), Volumes: maps.Clone(spec.GetVolumes()),
		Memory: int64(spec.GetMemoryMib()) << 20, CPUs: spec.GetCpus(), GPU: gpu, HostAccess: spec.GetHostAccess(),
	}
}

// Dependency is a container dependency a plugin's manifest declares, what
// an administrator chose for it, and whether it can be used.
type Dependency struct {
	Spec   *pluginv1.ContainerDependency
	Kept   core.PluginContainer
	Status *pluginv1.ContainerStatus
	// Commands run it without a GPU, nil when it needs one; GPUCommands
	// with one, nil when it cannot use one.
	Commands, GPUCommands *containers.Commands
	// Command is the docker run command equivalent to how a managed
	// dependency runs, empty without a container engine.
	Command string
}

// List returns a plugin's container dependencies in manifest order;
// core.ErrNotFound when there is no such plugin.
func (s *Service) List(ctx context.Context, pluginID string) ([]Dependency, error) {
	m, err := s.manifest(pluginID)
	if err != nil {
		return nil, err
	}
	kept, err := s.cfg.Store.List(ctx, pluginID)
	if err != nil {
		return nil, err
	}
	byName := make(map[string]core.PluginContainer, len(kept))
	for _, c := range kept {
		byName[c.Name] = c
	}
	rt, _ := s.Runtime(ctx, false)
	out := make([]Dependency, len(m.GetContainers()))
	for i, spec := range m.GetContainers() {
		c, ok := byName[spec.GetName()]
		if !ok {
			c = unset(pluginID, spec.GetName())
		}
		out[i] = Dependency{Spec: spec, Kept: c, Status: s.status(ctx, spec, c)}
		out[i].Commands, out[i].GPUCommands = s.commands(spec, c, rt.OnHost())
		out[i].Command = s.command(ctx, m, spec, c)
	}
	return out, nil
}

// command returns the docker run command of a managed dependency, empty
// for another or without a container engine.
func (s *Service) command(ctx context.Context, m *pluginv1.Manifest, spec *pluginv1.ContainerDependency, c core.PluginContainer) string {
	if c.Mode != core.ContainerManaged {
		return ""
	}
	sup, err := s.supervisor(ctx)
	if err != nil {
		return ""
	}
	return sup.Command(s.resolve(ctx, m, spec, c))
}

// unset is a dependency nothing is kept of: unset, through the system
// proxy once managed.
func unset(pluginID, name string) core.PluginContainer {
	return core.PluginContainer{PluginID: pluginID, Name: name, Mode: core.ContainerUnset, Proxy: core.SystemProxyID}
}

// commands returns the commands running a dependency without and with a
// GPU, nil for the variants it cannot run as.
func (s *Service) commands(spec *pluginv1.ContainerDependency, c core.PluginContainer, onHost bool) (plain, gpu *containers.Commands) {
	if spec.GetGpu() != pluginv1.GpuSupport_GPU_SUPPORT_REQUIRED {
		c.UseGPU = false
		cmds := containers.RunCommands(s.cfg.Names, Resolve(s.cfg.Names, spec, c), onHost)
		plain = &cmds
	}
	if spec.GetGpu() != pluginv1.GpuSupport_GPU_SUPPORT_UNSPECIFIED {
		c.UseGPU = true
		cmds := containers.RunCommands(s.cfg.Names, Resolve(s.cfg.Names, spec, c), onHost)
		gpu = &cmds
	}
	return plain, gpu
}

// AllowsHost reports whether host (a host name with an optional port) is
// that of one of a plugin's dependencies' addresses, which its WASM
// module may then fetch.
func (s *Service) AllowsHost(ctx context.Context, pluginID, host string) bool {
	kept, err := s.cfg.Store.List(ctx, pluginID)
	if err != nil {
		s.cfg.Logger.WarnContext(ctx, "list container dependencies", "plugin", pluginID, "err", err)
		return false
	}
	for _, c := range kept {
		addr := c.Address
		if c.Mode == core.ContainerManaged {
			s.mu.Lock()
			addr = s.told[key(pluginID, c.Name)]
			s.mu.Unlock()
		}
		if u, err := url.Parse(addr); err == nil && addr != "" && (u.Host == host || u.Hostname() == host) {
			return true
		}
	}
	return false
}

// Set sets how a dependency is reached and returns it, generating its
// secrets the first time, and configures the plugin again when its mode
// or address changed. A managed dependency needs a container engine
// (ErrNoEngine); it runs while its plugin runs, reaching sites through
// the proxy of ID proxy, which nil keeps. Leaving managed removes its
// container and keeps its volumes.
func (s *Service) Set(ctx context.Context, pluginID, name string, mode core.ContainerMode, address string, useGPU bool, proxy *string) (Dependency, error) {
	m, err := s.manifest(pluginID)
	if err != nil {
		return Dependency{}, err
	}
	spec, ok := manifest.Container(m, name)
	if !ok {
		return Dependency{}, fmt.Errorf("container %s of plugin %s: %w", name, pluginID, core.ErrNotFound)
	}
	switch {
	case useGPU && spec.GetGpu() == pluginv1.GpuSupport_GPU_SUPPORT_UNSPECIFIED:
		return Dependency{}, fmt.Errorf("%w: container %s of plugin %s uses no GPU", core.ErrInvalid, name, pluginID)
	case mode == core.ContainerManaged && !useGPU && spec.GetGpu() == pluginv1.GpuSupport_GPU_SUPPORT_REQUIRED:
		return Dependency{}, fmt.Errorf("%w: container %s of plugin %s needs a GPU", core.ErrInvalid, name, pluginID)
	}
	var sup *containers.Supervisor
	if mode == core.ContainerManaged {
		if sup, err = s.supervisor(ctx); err != nil {
			return Dependency{}, err
		}
	}
	old, err := s.kept(ctx, pluginID, name)
	if err != nil {
		return Dependency{}, err
	}
	c := core.PluginContainer{
		PluginID: pluginID, Name: name, Mode: mode, Address: strings.TrimSuffix(address, "/"),
		Secrets: secrets(spec, old.Secrets), UseGPU: useGPU, Proxy: old.Proxy,
	}
	if proxy != nil && *proxy != old.Proxy {
		if spec.GetDirect() {
			return Dependency{}, fmt.Errorf("%w: container %s of plugin %s reaches sites directly", core.ErrInvalid, name, pluginID)
		}
		if s.cfg.CheckProxy != nil {
			if err := s.cfg.CheckProxy(ctx, *proxy); err != nil {
				return Dependency{}, err
			}
		}
		c.Proxy = *proxy
	}
	if err := s.cfg.Store.Put(ctx, &c); err != nil {
		return Dependency{}, err
	}
	k := key(pluginID, name)
	s.mu.Lock()
	delete(s.health, k)
	delete(s.told, k)
	started := s.started[pluginID]
	s.mu.Unlock()
	if old.Mode == core.ContainerManaged && mode != core.ContainerManaged {
		if sup, err := s.supervisor(ctx); err == nil {
			if err := sup.Remove(ctx, pluginID, name, false); err != nil {
				s.cfg.Logger.WarnContext(ctx, "remove container no longer managed", "plugin", pluginID, "container", name, "err", err)
			}
		}
	}
	if sup != nil && started {
		sup.Start(s.resolve(ctx, m, spec, c))
	}
	if c.Proxy != old.Proxy && s.cfg.ProxyChanged != nil {
		s.cfg.ProxyChanged(s.cfg.Names.Container(pluginID, name))
	}
	if old.Mode != c.Mode || old.Address != c.Address {
		if p := s.boundPlugins(); p != nil {
			if err := p.Reconfigure(ctx, pluginID); err != nil {
				s.cfg.Logger.WarnContext(ctx, "reconfigure plugin after its container changed", "plugin", pluginID, "container", name, "err", err)
			}
		}
	}
	return Dependency{Spec: spec, Kept: c, Status: s.status(ctx, spec, c), Command: s.command(ctx, m, spec, c)}, nil
}

// manifest returns a plugin's manifest.
func (s *Service) manifest(pluginID string) (*pluginv1.Manifest, error) {
	if p := s.boundPlugins(); p != nil {
		if m, ok := p.Manifest(pluginID); ok {
			return m, nil
		}
	}
	return nil, fmt.Errorf("plugin %s: %w", pluginID, core.ErrNotFound)
}

// kept returns what is kept of a dependency; an unset one when nothing is.
func (s *Service) kept(ctx context.Context, pluginID, name string) (core.PluginContainer, error) {
	kept, err := s.cfg.Store.List(ctx, pluginID)
	if err != nil {
		return core.PluginContainer{}, err
	}
	for _, c := range kept {
		if c.Name == name {
			return c, nil
		}
	}
	return unset(pluginID, name), nil
}

// secrets returns the secrets a dependency's environment names: those
// generated before, and new ones for the rest.
func secrets(spec *pluginv1.ContainerDependency, old map[string]string) map[string]string {
	keys := manifest.ContainerSecrets(spec)
	if len(keys) == 0 {
		return nil
	}
	out := make(map[string]string, len(keys))
	for _, k := range keys {
		if v, ok := old[k]; ok {
			out[k] = v
		} else {
			out[k] = rand.Text()
		}
	}
	return out
}

func key(pluginID, name string) string { return pluginID + "/" + name }

var managedStates = map[containers.State]pluginv1.ContainerState{
	containers.Unavailable: pluginv1.ContainerState_CONTAINER_STATE_UNAVAILABLE,
	containers.Pulling:     pluginv1.ContainerState_CONTAINER_STATE_PULLING,
	containers.Starting:    pluginv1.ContainerState_CONTAINER_STATE_STARTING,
	containers.Ready:       pluginv1.ContainerState_CONTAINER_STATE_READY,
	containers.Unhealthy:   pluginv1.ContainerState_CONTAINER_STATE_UNHEALTHY,
}

// status tells whether a dependency can be used.
func (s *Service) status(ctx context.Context, spec *pluginv1.ContainerDependency, c core.PluginContainer) *pluginv1.ContainerStatus {
	st := pluginv1.ContainerStatus_builder{Name: new(c.Name)}.Build()
	unavailable := func(err string) {
		st.SetState(pluginv1.ContainerState_CONTAINER_STATE_UNAVAILABLE)
		st.SetError(err)
	}
	switch c.Mode {
	case core.ContainerExternal:
		st.SetMode(pluginv1.ContainerMode_CONTAINER_MODE_EXTERNAL)
		st.SetAddress(c.Address)
		h := s.check(ctx, key(c.PluginID, c.Name), c.Address, spec.GetHealthPath())
		st.SetState(h.state)
		if h.err != "" {
			st.SetError(h.err)
		}
	case core.ContainerManaged:
		st.SetMode(pluginv1.ContainerMode_CONTAINER_MODE_MANAGED)
		s.mu.Lock()
		sup, started := s.sup, s.started[c.PluginID]
		s.mu.Unlock()
		if sup == nil {
			_, err := s.supervisor(ctx)
			unavailable(fmt.Sprint(err))
			break
		}
		ms, ok := sup.Status(c.PluginID, c.Name)
		switch {
		case ok:
			st.SetState(managedStates[ms.State])
			st.SetAddress(managedAddress(ms))
			if ms.State == containers.Pulling {
				st.SetProgress(ms.Progress)
			}
			if ms.Err != nil {
				st.SetError(ms.Err.Error())
			}
		case !started:
			unavailable("the plugin is not running")
		default:
			unavailable("not started")
		}
	default:
		st.SetMode(pluginv1.ContainerMode_CONTAINER_MODE_UNSPECIFIED)
		unavailable("not set up")
	}
	return st
}

// check checks an external dependency's health, using a result younger
// than healthTTL. With a health path, it must answer 2xx there; without
// one, its port must accept connections.
func (s *Service) check(ctx context.Context, k, address, path string) health {
	now := s.cfg.Now()
	s.mu.Lock()
	h, ok := s.health[k]
	s.mu.Unlock()
	if ok && now.Sub(h.at) < healthTTL {
		return h
	}
	h = health{at: now, state: pluginv1.ContainerState_CONTAINER_STATE_READY}
	pctx, cancel := context.WithTimeout(ctx, healthTimeout)
	defer cancel()
	if err := s.cfg.Probe(pctx, address, path); err != nil {
		h.state, h.err = pluginv1.ContainerState_CONTAINER_STATE_UNHEALTHY, err.Error()
	}
	s.mu.Lock()
	s.health[k] = h
	s.mu.Unlock()
	return h
}
