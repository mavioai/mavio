package dependencies

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
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
	Store   core.PluginContainerRepository
	Plugins Plugins
	// Detect finds the container engine; containers.Detect with the
	// server's options in production.
	Detect func(ctx context.Context) (containers.Runtime, error)
	// HTTP checks external dependencies' health; nil uses a client of its own.
	HTTP *http.Client
	// Now returns the time; nil uses time.Now.
	Now    func() time.Time
	Logger *slog.Logger
}

// Service keeps plugins' container dependencies.
type Service struct {
	cfg Config

	mu       sync.Mutex
	detected bool
	runtime  containers.Runtime
	err      error
	health   map[string]health
}

// health is the result of a health check.
type health struct {
	at    time.Time
	state pluginv1.ContainerState
	err   string
}

// New returns a Service.
func New(cfg Config) *Service {
	if cfg.HTTP == nil {
		cfg.HTTP = &http.Client{Timeout: healthTimeout}
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &Service{cfg: cfg, health: map[string]health{}}
}

// Close closes the engine's client.
func (s *Service) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.runtime.Client != nil {
		s.runtime.Client.Close()
	}
}

// Runtime returns the container engine found, detecting it on first use
// or when detect is set; the error, wrapping containers.ErrUnavailable,
// tells why there is none.
func (s *Service) Runtime(ctx context.Context, detect bool) (containers.Runtime, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.detected && !detect {
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
			"engine", rt.Version.Engine, "version", rt.Version.Version, "api", rt.APIVersion, "rootless", rt.Rootless)
	}
	return rt, err
}

// Dependency is a container dependency a plugin's manifest declares, what
// an administrator chose for it, and whether it can be used.
type Dependency struct {
	Spec   *pluginv1.ContainerDependency
	Kept   core.PluginContainer
	Status *pluginv1.ContainerStatus
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
	out := make([]Dependency, len(m.GetContainers()))
	for i, spec := range m.GetContainers() {
		c, ok := byName[spec.GetName()]
		if !ok {
			c = core.PluginContainer{PluginID: pluginID, Name: spec.GetName(), Mode: core.ContainerUnset}
		}
		out[i] = Dependency{Spec: spec, Kept: c, Status: s.status(ctx, spec, c)}
	}
	return out, nil
}

// Set sets how a dependency is reached and returns it, generating its
// secrets the first time, and configures the plugin again when its
// address changed. Managed dependencies need a container engine
// (ErrNoEngine); running them is not supported yet (errors.ErrUnsupported).
func (s *Service) Set(ctx context.Context, pluginID, name string, mode core.ContainerMode, address string, useGPU bool) (Dependency, error) {
	m, err := s.manifest(pluginID)
	if err != nil {
		return Dependency{}, err
	}
	spec, ok := manifest.Container(m, name)
	if !ok {
		return Dependency{}, fmt.Errorf("container %s of plugin %s: %w", name, pluginID, core.ErrNotFound)
	}
	if mode == core.ContainerManaged {
		if _, err := s.Runtime(ctx, false); err != nil {
			return Dependency{}, fmt.Errorf("%w: %w", ErrNoEngine, err)
		}
		return Dependency{}, fmt.Errorf("managed container dependencies: %w", errors.ErrUnsupported)
	}
	if useGPU && spec.GetGpu() == pluginv1.GpuSupport_GPU_SUPPORT_UNSPECIFIED {
		return Dependency{}, fmt.Errorf("%w: container %s of plugin %s uses no GPU", core.ErrInvalid, name, pluginID)
	}
	old, err := s.kept(ctx, pluginID, name)
	if err != nil {
		return Dependency{}, err
	}
	c := core.PluginContainer{
		PluginID: pluginID, Name: name, Mode: mode, Address: strings.TrimSuffix(address, "/"),
		Secrets: secrets(spec, old.Secrets), UseGPU: useGPU,
	}
	if err := s.cfg.Store.Put(ctx, &c); err != nil {
		return Dependency{}, err
	}
	s.mu.Lock()
	delete(s.health, key(pluginID, name))
	s.mu.Unlock()
	if old.Mode != c.Mode || old.Address != c.Address {
		if err := s.cfg.Plugins.Reconfigure(ctx, pluginID); err != nil {
			s.cfg.Logger.WarnContext(ctx, "reconfigure plugin after its container changed", "plugin", pluginID, "container", name, "err", err)
		}
	}
	return Dependency{Spec: spec, Kept: c, Status: s.status(ctx, spec, c)}, nil
}

// manifest returns a plugin's manifest.
func (s *Service) manifest(pluginID string) (*pluginv1.Manifest, error) {
	if m, ok := s.cfg.Plugins.Manifest(pluginID); ok {
		return m, nil
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
	return core.PluginContainer{PluginID: pluginID, Name: name, Mode: core.ContainerUnset}, nil
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

// status tells whether a dependency can be used.
func (s *Service) status(ctx context.Context, spec *pluginv1.ContainerDependency, c core.PluginContainer) *pluginv1.ContainerStatus {
	st := pluginv1.ContainerStatus_builder{Name: new(c.Name)}.Build()
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
		st.SetState(pluginv1.ContainerState_CONTAINER_STATE_UNAVAILABLE)
		st.SetError("managed container dependencies are not supported yet")
	default:
		st.SetMode(pluginv1.ContainerMode_CONTAINER_MODE_UNSPECIFIED)
		st.SetState(pluginv1.ContainerState_CONTAINER_STATE_UNAVAILABLE)
		st.SetError("not set up")
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
	if err := s.probe(ctx, address, path); err != nil {
		h.state, h.err = pluginv1.ContainerState_CONTAINER_STATE_UNHEALTHY, err.Error()
	}
	s.mu.Lock()
	s.health[k] = h
	s.mu.Unlock()
	return h
}

func (s *Service) probe(ctx context.Context, address, path string) error {
	ctx, cancel := context.WithTimeout(ctx, healthTimeout)
	defer cancel()
	u, err := url.Parse(address)
	if err != nil {
		return err
	}
	if path == "" {
		host := u.Host
		if u.Port() == "" {
			port := "80"
			if u.Scheme == "https" {
				port = "443"
			}
			host = net.JoinHostPort(u.Hostname(), port)
		}
		var d net.Dialer
		conn, err := d.DialContext(ctx, "tcp", host)
		if err != nil {
			return err
		}
		return conn.Close()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address+path, nil)
	if err != nil {
		return err
	}
	resp, err := s.cfg.HTTP.Do(req)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("GET %s: %s", path, resp.Status)
	}
	return nil
}
