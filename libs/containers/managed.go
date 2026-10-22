package containers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"path"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

// The labels the server puts on what it creates.
const (
	// LabelServer is the ID of the server that created it.
	LabelServer = "dev.mavio.server"
	// LabelPlugin is the ID of the plugin declaring the dependency.
	LabelPlugin = "dev.mavio.plugin"
	// LabelDependency is the dependency's name in the plugin.
	LabelDependency = "dev.mavio.dependency"
	// LabelSpec is the hash of the dependency the container was created
	// for (Hash).
	LabelSpec = "dev.mavio.spec"
)

// DefaultPrefix starts the names of what the server creates.
const DefaultPrefix = "mavio"

// Names names what the server creates for dependencies.
type Names struct {
	// Prefix starts every name; empty is DefaultPrefix.
	Prefix string
}

// Validate checks that the prefix is lowercase letters, digits and
// hyphens, starting and ending with a letter or digit.
func (n Names) Validate() error {
	p := n.prefix()
	ok := p[0] != '-' && p[len(p)-1] != '-' && !strings.Contains(p, "--")
	for _, r := range p {
		ok = ok && (r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-')
	}
	if !ok {
		return fmt.Errorf("%w: container prefix %q is not lowercase letters, digits and hyphens", core.ErrInvalid, p)
	}
	return nil
}

func (n Names) prefix() string {
	if n.Prefix == "" {
		return DefaultPrefix
	}
	return n.Prefix
}

// Container names the container of a plugin's dependency:
// <prefix>-<plugin>-<name>, <plugin> being the last part of the plugin's ID.
func (n Names) Container(plugin, name string) string {
	return n.prefix() + "-" + plugin[strings.LastIndex(plugin, ".")+1:] + "-" + name
}

// Volume names a volume of a plugin's dependency.
func (n Names) Volume(plugin, name, volume string) string {
	return n.Container(plugin, name) + "-" + volume
}

// Network names the network dependencies join.
func (n Names) Network() string { return n.prefix() }

// Dependency is a container dependency resolved to run: its environment
// holds its secrets and its own address, its image is the one it runs.
type Dependency struct {
	Plugin, Name string
	Image        string
	Cmd          []string
	Env          map[string]string
	// Port is the TCP port the service listens on in the container;
	// HealthPath answers 2xx when it is ready, empty when the port
	// accepting connections is enough.
	Port       int
	HealthPath string
	// Volumes maps the dependency's volume names to their mount paths.
	Volumes map[string]string
	// Folders maps the server's folders, slash-separated absolute paths,
	// to the paths they are mounted at: a server on the host mounts them,
	// one in a container (SupervisorOptions.Self) what its own container
	// mounts them from.
	Folders map[string]string
	// PeerPort is a port published at its own number on every interface
	// of the host, TCP and UDP, for peers to connect to; 0 for none.
	PeerPort int
	// Entrypoint replaces the image's entrypoint when not empty.
	Entrypoint []string
	Memory     int64
	CPUs       float64
	GPU        GPU
	// HostAccess is whether the service reaches the host by the name
	// HostName.
	HostAccess bool
	// ProxyEnv returns the environment sending the service's requests to
	// the server's proxy gateway at host, its name or address from the
	// container; nil sends them nowhere but where they go. Env wins over
	// it.
	ProxyEnv func(host string) map[string]string `json:"-"`
}

// HostName is the name a container reaches the host by.
const HostName = "host.docker.internal"

// Hash identifies what a container is created from: a container whose
// LabelSpec differs is replaced.
func Hash(d Dependency) string {
	data, _ := json.Marshal(struct {
		Dependency
		Proxied bool
	}{d, d.ProxyEnv != nil})
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:16])
}

// State is whether a managed dependency can be used.
type State int

// The states of a managed dependency.
const (
	// Unavailable: it is not running, and Status.Err tells why.
	Unavailable State = iota
	// Pulling: its image is being pulled.
	Pulling
	// Starting: it started and its health check has not passed yet.
	Starting
	// Ready: its health check passes.
	Ready
	// Unhealthy: its health check fails.
	Unhealthy
)

func (s State) String() string {
	switch s {
	case Pulling:
		return "pulling"
	case Starting:
		return "starting"
	case Ready:
		return "ready"
	case Unhealthy:
		return "unhealthy"
	}
	return "unavailable"
}

// Status is a managed dependency's state, the base URL the server
// reaches it at once its container runs, and the last error.
type Status struct {
	State   State
	Address string
	Err     error
	// Progress is the fraction of the image downloaded while Pulling.
	Progress float64
	// Queued reports, while Pulling, a pull waiting for others to end.
	Queued bool
}

// Engine is the part of the Engine API the Supervisor uses; *Client
// implements it.
type Engine interface {
	InspectImage(ctx context.Context, ref string) (Image, error)
	PullImage(ctx context.Context, ref string, progress func(PullProgress)) error
	InspectNetwork(ctx context.Context, id string) (Network, error)
	CreateNetwork(ctx context.Context, name string, labels map[string]string) (string, error)
	ConnectNetwork(ctx context.Context, network, container string, aliases []string) error
	CreateContainer(ctx context.Context, s ContainerSpec) (string, error)
	StartContainer(ctx context.Context, id string) error
	StopContainer(ctx context.Context, id string, timeout time.Duration) error
	InspectContainer(ctx context.Context, id string) (Container, error)
	ListContainers(ctx context.Context, labels map[string]string) ([]ContainerSummary, error)
	RemoveContainer(ctx context.Context, id string) error
	ListVolumes(ctx context.Context, labels map[string]string) ([]Volume, error)
	RemoveVolume(ctx context.Context, name string) error
}

var _ Engine = (*Client)(nil)

// SupervisorOptions configure a Supervisor.
type SupervisorOptions struct {
	Engine Engine
	Names  Names
	// Server is the server's ID, which labels what it creates; the
	// Supervisor touches nothing labeled with another.
	Server string
	// HostGateway maps HostName to the host in the containers of
	// dependencies with HostAccess, for Docker; Docker Desktop and Podman
	// name it themselves.
	HostGateway bool
	// Self is the server's own container when the server runs in one off
	// the host's network: it joins the network of Names.Network, and
	// dependencies are reached there by name rather than through ports
	// published on the loopback interface. Empty for a server on the host.
	Self string
	// Pull pulls an image, telling of its progress; nil pulls it with
	// Engine.PullImage. The server runs pulls as jobs through it, telling
	// PullQueued while a job waits for others, and cancels a pull that
	// nothing waits for any longer.
	Pull func(ctx context.Context, ref string, progress func(PullProgress)) error
	// OnChange is called with a dependency's status whenever it changes,
	// never concurrently for one dependency; nil calls nothing.
	OnChange func(plugin, name string, st Status)
	// Probe checks a dependency's health; nil uses Probe with a client
	// of the Supervisor's own.
	Probe func(ctx context.Context, address, path string) error
	// Interval is the time between health checks of a ready dependency,
	// 30 seconds when 0; Poll the time between those of a starting one,
	// a second when 0.
	Interval, Poll time.Duration
	// Failures is how many health checks in a row fail before the
	// container is restarted, 3 when 0.
	Failures int
	// MaxBackoff caps the delay between restarts, a minute when 0.
	MaxBackoff time.Duration
	Logger     *slog.Logger
}

// Supervisor runs managed dependencies on a container engine. Each
// container joins the network of Names.Network under its name; for a
// server on the host it publishes its port on 127.0.0.1, which is its
// address, and for a server in a container (SupervisorOptions.Self) its
// address is http://<name>:<port> on that network, which the server's
// container joins. It
// replaces a container created from another Hash, waits for the health
// check, checks it every Interval and restarts the container after
// Failures failures in a row, backing off up to MaxBackoff.
type Supervisor struct {
	o      SupervisorOptions
	ctx    context.Context
	cancel context.CancelFunc

	mu     sync.Mutex
	runs   map[string]*run
	closed bool
}

// run is a dependency the Supervisor runs.
type run struct {
	dep    Dependency
	hash   string
	cancel context.CancelFunc
	done   chan struct{}
	status Status
	// gateway is the host the container reaches the proxy gateway at, of a
	// dependency with ProxyEnv, once known.
	gateway string
}

// stopTimeout is how long a container has to stop before it is killed.
const stopTimeout = 10 * time.Second

// NewSupervisor returns a Supervisor running nothing yet.
func NewSupervisor(o SupervisorOptions) *Supervisor {
	if o.Probe == nil {
		client := &http.Client{Timeout: 5 * time.Second}
		o.Probe = func(ctx context.Context, address, path string) error { return Probe(ctx, client, address, path) }
	}
	if o.Pull == nil {
		o.Pull = o.Engine.PullImage
	}
	if o.Interval == 0 {
		o.Interval = 30 * time.Second
	}
	if o.Poll == 0 {
		o.Poll = time.Second
	}
	if o.Failures == 0 {
		o.Failures = 3
	}
	if o.MaxBackoff == 0 {
		o.MaxBackoff = time.Minute
	}
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Supervisor{o: o, ctx: ctx, cancel: cancel, runs: map[string]*run{}}
}

func key(plugin, name string) string { return plugin + "/" + name }

// Start runs a dependency in the background, replacing the one of its
// name if it differs; it does nothing for the same dependency.
func (s *Supervisor) Start(d Dependency) {
	h := Hash(d)
	k := key(d.Plugin, d.Name)
	s.mu.Lock()
	old := s.runs[k]
	if s.closed || (old != nil && old.hash == h) {
		s.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(s.ctx)
	r := &run{dep: d, hash: h, cancel: cancel, done: make(chan struct{}), status: Status{State: Starting}}
	s.runs[k] = r
	s.mu.Unlock()
	go func() {
		defer close(r.done)
		if old != nil {
			old.cancel()
			<-old.done
		}
		s.loop(ctx, r)
	}()
}

// Status returns a running dependency's status; false when it does not
// run.
func (s *Supervisor) Status(plugin, name string) (Status, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.runs[key(plugin, name)]
	if !ok {
		return Status{}, false
	}
	return r.status, true
}

// Command returns the docker run command equivalent to how the
// Supervisor creates d's container, which reaches the proxy gateway where
// it does when it runs, or where it would.
func (s *Supervisor) Command(ctx context.Context, d Dependency) string {
	r := &run{dep: d, hash: Hash(d), gateway: HostName}
	if s.o.Self != "" {
		r.gateway = "<server>"
	}
	s.mu.Lock()
	if cur := s.runs[key(d.Plugin, d.Name)]; cur != nil && cur.gateway != "" {
		r.gateway = cur.gateway
	}
	s.mu.Unlock()
	binds, err := s.binds(ctx, d)
	if err != nil {
		binds = nil
		for _, f := range slices.Sorted(maps.Keys(d.Folders)) {
			binds = append(binds, Bind{Source: "<" + f + ">", Target: d.Folders[f]})
		}
	}
	return s.spec(r, s.o.Names.Container(d.Plugin, d.Name), binds).DockerRun()
}

// Running lists the names of a plugin's running dependencies.
func (s *Supervisor) Running(plugin string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for _, r := range s.runs {
		if r.dep.Plugin == plugin {
			out = append(out, r.dep.Name)
		}
	}
	slices.Sort(out)
	return out
}

// forget ends the runs of a plugin's dependency, or all of the plugin's
// when name is empty, and waits for them to end.
func (s *Supervisor) forget(plugin, name string) {
	s.mu.Lock()
	var ended []*run
	for k, r := range s.runs {
		if r.dep.Plugin == plugin && (name == "" || r.dep.Name == name) {
			delete(s.runs, k)
			r.cancel()
			ended = append(ended, r)
		}
	}
	s.mu.Unlock()
	for _, r := range ended {
		<-r.done
	}
}

// Stop stops running a plugin's dependency, or all of the plugin's when
// name is empty, and stops their containers.
func (s *Supervisor) Stop(ctx context.Context, plugin, name string) error {
	s.forget(plugin, name)
	return s.each(ctx, plugin, name, func(c ContainerSummary) error {
		return s.o.Engine.StopContainer(ctx, c.ID, stopTimeout)
	})
}

// Remove stops running a plugin's dependency, or all of the plugin's when
// name is empty, and removes their containers, and their volumes too with
// volumes set.
func (s *Supervisor) Remove(ctx context.Context, plugin, name string, volumes bool) error {
	s.forget(plugin, name)
	err := s.each(ctx, plugin, name, func(c ContainerSummary) error {
		return s.o.Engine.RemoveContainer(ctx, c.ID)
	})
	if err != nil || !volumes {
		return err
	}
	vs, err := s.o.Engine.ListVolumes(ctx, s.labels(plugin, name))
	if err != nil {
		return err
	}
	var errs []error
	for _, v := range vs {
		if err := s.o.Engine.RemoveVolume(ctx, v.Name); err != nil && !errors.Is(err, core.ErrNotFound) {
			errs = append(errs, fmt.Errorf("remove volume %s: %w", v.Name, err))
		}
	}
	return errors.Join(errs...)
}

// labels returns the labels selecting what the server created for a
// plugin's dependency, or for all of the plugin's when name is empty, or
// for all plugins when plugin is empty too.
func (s *Supervisor) labels(plugin, name string) map[string]string {
	l := map[string]string{LabelServer: s.o.Server}
	if plugin != "" {
		l[LabelPlugin] = plugin
	}
	if name != "" {
		l[LabelDependency] = name
	}
	return l
}

// each calls f with each container the server created for a plugin's
// dependency, or all of the plugin's when name is empty; containers gone
// meanwhile are no error.
func (s *Supervisor) each(ctx context.Context, plugin, name string, f func(ContainerSummary) error) error {
	cs, err := s.o.Engine.ListContainers(ctx, s.labels(plugin, name))
	if err != nil {
		return err
	}
	var errs []error
	for _, c := range cs {
		if err := f(c); err != nil && !errors.Is(err, core.ErrNotFound) {
			errs = append(errs, fmt.Errorf("container %s: %w", c.ID, err))
		}
	}
	return errors.Join(errs...)
}

// Reconcile removes the containers the server created for dependencies
// keep rejects, those no longer declared or no longer managed; their
// volumes stay. It stops those it keeps that run with nothing running
// them, left by a server that ended without stopping them, whose plugin
// started reports not running.
func (s *Supervisor) Reconcile(ctx context.Context, keep func(plugin, name string) bool, started func(plugin string) bool) error {
	cs, err := s.o.Engine.ListContainers(ctx, s.labels("", ""))
	if err != nil {
		return err
	}
	var errs []error
	for _, c := range cs {
		plugin, name := c.Labels[LabelPlugin], c.Labels[LabelDependency]
		if keep(plugin, name) {
			if c.State != "running" || started(plugin) {
				continue
			}
			if _, runs := s.Status(plugin, name); runs {
				continue
			}
			if err := s.o.Engine.StopContainer(ctx, c.ID, stopTimeout); err != nil && !errors.Is(err, core.ErrNotFound) {
				errs = append(errs, fmt.Errorf("stop container %s: %w", c.ID, err))
				continue
			}
			s.o.Logger.InfoContext(ctx, "stopped the container of a plugin not running", "plugin", plugin, "dependency", name)
			continue
		}
		s.forget(plugin, name)
		if err := s.o.Engine.RemoveContainer(ctx, c.ID); err != nil && !errors.Is(err, core.ErrNotFound) {
			errs = append(errs, fmt.Errorf("remove container %s: %w", c.ID, err))
			continue
		}
		s.o.Logger.InfoContext(ctx, "removed the container of a dependency no longer managed", "plugin", plugin, "dependency", name)
	}
	return errors.Join(errs...)
}

// Close stops every run and its container.
func (s *Supervisor) Close(ctx context.Context) error {
	s.mu.Lock()
	s.closed = true
	runs := slices.Collect(maps.Values(s.runs))
	s.runs = map[string]*run{}
	s.mu.Unlock()
	s.cancel()
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
	)
	for _, r := range runs {
		wg.Go(func() {
			<-r.done
			name := s.o.Names.Container(r.dep.Plugin, r.dep.Name)
			if err := s.o.Engine.StopContainer(ctx, name, stopTimeout); err != nil && !errors.Is(err, core.ErrNotFound) {
				mu.Lock()
				errs = append(errs, fmt.Errorf("stop %s: %w", name, err))
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	return errors.Join(errs...)
}

// set records a run's status and reports it, unless the run ended.
func (s *Supervisor) set(r *run, st Status) {
	s.mu.Lock()
	current := s.runs[key(r.dep.Plugin, r.dep.Name)] == r
	changed := current && (r.status.State != st.State || r.status.Address != st.Address || r.status.Progress != st.Progress ||
		r.status.Queued != st.Queued || errString(r.status.Err) != errString(st.Err))
	if current {
		r.status = st
	}
	s.mu.Unlock()
	if changed && s.o.OnChange != nil {
		s.o.OnChange(r.dep.Plugin, r.dep.Name, st)
	}
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// loop runs a dependency until ctx ends, starting its container again,
// with backoff, whenever it fails.
func (s *Supervisor) loop(ctx context.Context, r *run) {
	name := s.o.Names.Container(r.dep.Plugin, r.dep.Name)
	backoff := time.Second
	for {
		id, addr, err := s.ensure(ctx, r, name)
		if err == nil {
			var ready bool
			ready, err = s.supervise(ctx, r, id, addr)
			if ready {
				backoff = time.Second
			}
		}
		if ctx.Err() != nil {
			return
		}
		st := Status{State: Unavailable, Err: err}
		if id != "" {
			st = Status{State: Unhealthy, Address: addr, Err: err}
			if err := s.o.Engine.StopContainer(ctx, id, stopTimeout); err != nil && !errors.Is(err, core.ErrNotFound) {
				s.o.Logger.WarnContext(ctx, "stop dependency", "container", name, "err", err)
			}
		}
		s.set(r, st)
		s.o.Logger.WarnContext(ctx, "dependency failed; restarting", "container", name, "in", backoff, "err", err)
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff = min(2*backoff, s.o.MaxBackoff)
	}
}

// ensure makes the dependency's container run as its run asks, creating
// what is missing, and returns the container's ID and address.
func (s *Supervisor) ensure(ctx context.Context, r *run, name string) (id, addr string, err error) {
	e, d := s.o.Engine, r.dep
	network := s.o.Names.Network()
	if _, err := e.InspectNetwork(ctx, network); errors.Is(err, core.ErrNotFound) {
		if _, err := e.CreateNetwork(ctx, network, map[string]string{LabelServer: s.o.Server}); err != nil && !errors.Is(err, core.ErrConflict) {
			return "", "", fmt.Errorf("create network %s: %w", network, err)
		}
	} else if err != nil {
		return "", "", fmt.Errorf("inspect network %s: %w", network, err)
	}

	if s.o.Self != "" {
		if err := s.join(ctx, network); err != nil {
			return "", "", err
		}
	}
	if d.ProxyEnv != nil {
		gateway, err := s.gatewayHost(ctx, network)
		if err != nil {
			return "", "", err
		}
		s.mu.Lock()
		r.gateway = gateway
		s.mu.Unlock()
	}

	ctr, err := e.InspectContainer(ctx, name)
	switch {
	case errors.Is(err, core.ErrNotFound):
	case err != nil:
		return "", "", fmt.Errorf("inspect container %s: %w", name, err)
	case ctr.Config.Labels[LabelServer] != s.o.Server:
		return "", "", fmt.Errorf("%w: container %s is not this server's", core.ErrConflict, name)
	case ctr.Config.Labels[LabelSpec] != s.specLabel(r):
		s.o.Logger.InfoContext(ctx, "replacing a dependency's container", "container", name)
		if err := e.RemoveContainer(ctx, ctr.ID); err != nil && !errors.Is(err, core.ErrNotFound) {
			return "", "", fmt.Errorf("remove container %s: %w", name, err)
		}
	default:
		id = ctr.ID
	}

	if id == "" {
		if _, err := e.InspectImage(ctx, d.Image); errors.Is(err, core.ErrNotFound) {
			s.set(r, Status{State: Pulling})
			s.o.Logger.InfoContext(ctx, "pulling a dependency's image", "container", name, "image", d.Image)
			var (
				mu     sync.Mutex
				frac   PullFraction
				done   float64
				queued bool
			)
			err := s.o.Pull(ctx, d.Image, func(p PullProgress) {
				mu.Lock()
				q := p == PullProgress{Status: PullQueued}
				f, moved := frac.Add(p)
				if moved {
					done = f
				}
				changed := moved || q != queued
				queued = q
				st := Status{State: Pulling, Progress: done, Queued: q}
				mu.Unlock()
				if changed {
					s.set(r, st)
				}
			})
			if err != nil {
				return "", "", fmt.Errorf("pull %s: %w", d.Image, err)
			}
		} else if err != nil {
			return "", "", fmt.Errorf("inspect image %s: %w", d.Image, err)
		}
		binds, err := s.binds(ctx, d)
		if err != nil {
			return "", "", err
		}
		if id, err = e.CreateContainer(ctx, s.spec(r, name, binds)); err != nil {
			return "", "", fmt.Errorf("create container %s: %w", name, err)
		}
	}
	if err := e.StartContainer(ctx, id); err != nil {
		return id, "", fmt.Errorf("start container %s: %w", name, err)
	}
	if s.o.Self != "" {
		return id, fmt.Sprintf("http://%s:%d", name, d.Port), nil
	}
	if ctr, err = e.InspectContainer(ctx, id); err != nil {
		return id, "", fmt.Errorf("inspect container %s: %w", name, err)
	}
	port, ok := ctr.PublishedPort(d.Port)
	if !ok {
		return id, "", fmt.Errorf("container %s does not publish port %d", name, d.Port)
	}
	return id, fmt.Sprintf("http://127.0.0.1:%d", port), nil
}

// join connects the server's own container to the network unless it is
// connected.
func (s *Supervisor) join(ctx context.Context, network string) error {
	n, err := s.o.Engine.InspectNetwork(ctx, network)
	if err != nil {
		return fmt.Errorf("inspect network %s: %w", network, err)
	}
	for id, c := range n.Containers {
		if strings.HasPrefix(id, s.o.Self) || strings.TrimPrefix(c.Name, "/") == s.o.Self {
			return nil
		}
	}
	if err := s.o.Engine.ConnectNetwork(ctx, network, s.o.Self, nil); err != nil {
		return fmt.Errorf("connect the server's container to network %s: %w", network, err)
	}
	return nil
}

// gatewayHost returns the host the containers reach the server's proxy
// gateway at: HostName for a server on the host, else the address of the
// server's container on the network.
func (s *Supervisor) gatewayHost(ctx context.Context, network string) (string, error) {
	if s.o.Self == "" {
		return HostName, nil
	}
	n, err := s.o.Engine.InspectNetwork(ctx, network)
	if err != nil {
		return "", fmt.Errorf("inspect network %s: %w", network, err)
	}
	for id, c := range n.Containers {
		if strings.HasPrefix(id, s.o.Self) || strings.TrimPrefix(c.Name, "/") == s.o.Self {
			if addr, _, _ := strings.Cut(c.IPv4Address, "/"); addr != "" {
				return addr, nil
			}
		}
	}
	return "", fmt.Errorf("the server's container has no address on network %s", network)
}

// specLabel is a run's LabelSpec: its Hash, marked when its port is not
// published, so a container is replaced when the server moves between
// the host and a container, and with where it reaches the proxy gateway.
func (s *Supervisor) specLabel(r *run) string {
	label := r.hash
	if s.o.Self != "" {
		label += "-unpublished"
	}
	if r.dep.ProxyEnv != nil {
		s.mu.Lock()
		label += "-via-" + r.gateway
		s.mu.Unlock()
	}
	return label
}

// binds returns how a dependency's folders are mounted: the server's
// folders themselves for a server on the host, else where its own
// container mounts them from.
func (s *Supervisor) binds(ctx context.Context, d Dependency) ([]Bind, error) {
	if len(d.Folders) == 0 {
		return nil, nil
	}
	var mounts []Mount
	if s.o.Self != "" {
		self, err := s.o.Engine.InspectContainer(ctx, s.o.Self)
		if err != nil {
			return nil, fmt.Errorf("inspect the server's container: %w", err)
		}
		mounts = self.Mounts
	}
	out := make([]Bind, 0, len(d.Folders))
	for _, f := range slices.Sorted(maps.Keys(d.Folders)) {
		b := Bind{Source: f, Target: d.Folders[f]}
		if s.o.Self != "" {
			var err error
			if b, err = HostBind(mounts, f, d.Folders[f]); err != nil {
				return nil, err
			}
		}
		out = append(out, b)
	}
	return out, nil
}

// HostBind returns how a container mounts a folder of a container whose
// mounts are these at target: from the source of the bind mount the
// folder is on, or a subpath of its volume.
func HostBind(mounts []Mount, folder, target string) (Bind, error) {
	folder = path.Clean(folder)
	var best *Mount
	for i, m := range mounts {
		d := path.Clean(m.Destination)
		if (folder == d || strings.HasPrefix(folder, strings.TrimSuffix(d, "/")+"/")) && (best == nil || len(d) > len(path.Clean(best.Destination))) {
			best = &mounts[i]
		}
	}
	if best == nil || best.Type != "bind" && best.Type != "volume" {
		return Bind{}, fmt.Errorf("%w: folder %s is on none of the bind mounts or volumes of the server's container", core.ErrInvalid, folder)
	}
	rest := strings.TrimPrefix(strings.TrimPrefix(folder, path.Clean(best.Destination)), "/")
	if best.Type == "volume" {
		return Bind{Volume: best.Name, Subpath: rest, Target: target}, nil
	}
	return Bind{Source: path.Join(best.Source, rest), Target: target}, nil
}

// spec returns what a run's container is created with.
func (s *Supervisor) spec(r *run, name string, binds []Bind) ContainerSpec {
	d := r.dep
	volumeLabels := s.labels(d.Plugin, d.Name)
	labels := maps.Clone(volumeLabels)
	labels[LabelSpec] = s.specLabel(r)
	vars := map[string]string{}
	if d.ProxyEnv != nil {
		s.mu.Lock()
		gateway := r.gateway
		s.mu.Unlock()
		vars = d.ProxyEnv(gateway)
	}
	maps.Copy(vars, d.Env)
	env := make([]string, 0, len(vars))
	for _, k := range slices.Sorted(maps.Keys(vars)) {
		env = append(env, k+"="+vars[k])
	}
	volumes := make(map[string]string, len(d.Volumes))
	for v, target := range d.Volumes {
		volumes[s.o.Names.Volume(d.Plugin, d.Name, v)] = target
	}
	return ContainerSpec{
		Name: name, Image: d.Image, Entrypoint: d.Entrypoint, Cmd: d.Cmd, Env: env, Labels: labels, VolumeLabels: volumeLabels,
		Port: d.Port, PublishLoopback: s.o.Self == "", PeerPort: d.PeerPort, Network: s.o.Names.Network(), Aliases: []string{name},
		Volumes: volumes, Binds: binds, Memory: d.Memory, CPUs: d.CPUs, GPU: d.GPU, HostGateway: (d.HostAccess || d.ProxyEnv != nil && s.o.Self == "") && s.o.HostGateway,
	}
}

// supervise waits for a started container's health check to pass and
// checks it until it fails Failures times in a row, or the container
// stops before it first passes; ready reports that it passed.
func (s *Supervisor) supervise(ctx context.Context, r *run, id, addr string) (ready bool, err error) {
	s.set(r, Status{State: Starting, Address: addr})
	for {
		if err = s.probe(ctx, r, addr); err == nil {
			break
		}
		ctr, ierr := s.o.Engine.InspectContainer(ctx, id)
		switch {
		case ctx.Err() != nil:
			return false, ctx.Err()
		case ierr != nil:
			return false, fmt.Errorf("inspect container: %w", ierr)
		case !ctr.State.Running:
			return false, fmt.Errorf("container exited with code %d before its health check passed: %w", ctr.State.ExitCode, err)
		}
		if err := sleep(ctx, s.o.Poll); err != nil {
			return false, err
		}
	}
	s.set(r, Status{State: Ready, Address: addr})
	failures := 0
	for {
		if err := sleep(ctx, s.o.Interval); err != nil {
			return true, err
		}
		if err := s.probe(ctx, r, addr); err != nil {
			failures++
			s.set(r, Status{State: Unhealthy, Address: addr, Err: err})
			if failures >= s.o.Failures {
				return true, fmt.Errorf("health check failed %d times: %w", failures, err)
			}
			continue
		}
		failures = 0
		s.set(r, Status{State: Ready, Address: addr})
	}
}

// healthTimeout bounds a health check.
const healthTimeout = 5 * time.Second

func (s *Supervisor) probe(ctx context.Context, r *run, addr string) error {
	ctx, cancel := context.WithTimeout(ctx, healthTimeout)
	defer cancel()
	return s.o.Probe(ctx, addr, r.dep.HealthPath)
}

func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
