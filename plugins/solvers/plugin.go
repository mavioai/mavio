package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	"github.com/mavioai/mavio/libs/plugin/guest"
	"github.com/mavioai/mavio/libs/plugin/proxy"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1/pluginv1connect"
	"github.com/mavioai/mavio/plugins/solvers/internal/flaresolverr"
)

// FlareSolverr is the solver of FlareSolverr and the name of its
// container dependency.
const FlareSolverr = "flaresolverr"

// hostName is the name a container dependency reaches the host by.
const hostName = "host.docker.internal"

// sweepEvery is how often idle sessions are destroyed.
const sweepEvery = time.Minute

// config is the plugin's configuration.
type config struct {
	// Solvers are tried in turn; empty means FlareSolverr alone.
	Solvers []solverConfig `json:"solvers"`
	// SessionTTLMinutes rotates sessions, 30 when unset; 0 never.
	SessionTTLMinutes *int `json:"session_ttl_minutes"`
	// MaxConcurrent caps the challenges solved at once, 2 when 0.
	MaxConcurrent int `json:"max_concurrent"`
}

type solverConfig struct {
	Name    string `json:"name"`
	Enabled *bool  `json:"enabled"`
	// TimeoutSeconds is how long a challenge may take, 60 when 0.
	TimeoutSeconds int `json:"timeout_seconds"`
	// DisableMedia leaves images, fonts and styles unloaded; true when
	// unset.
	DisableMedia *bool `json:"disable_media"`
}

// solver is an enabled solver.
type solver struct {
	name         string
	timeout      time.Duration
	disableMedia bool
	fs           *flaresolverr.Solver
	// managed is whether the server runs it in a container.
	managed bool
	// err is why it cannot solve.
	err error
}

// plugin implements the plugin's services.
type plugin struct {
	ctx     context.Context
	log     *slog.Logger
	version string
	http    *http.Client
	host    func() *http.Client

	mu      sync.RWMutex
	solvers []*solver
	// stop stops the sweeping of the solvers' idle sessions.
	stop context.CancelFunc
}

func newPlugin(ctx context.Context, log *slog.Logger, version string, hc *http.Client, host func() *http.Client) *plugin {
	return &plugin{ctx: ctx, log: log, version: version, http: hc, host: host, stop: func() {}}
}

// Describe implements PluginService.
func (p *plugin) Describe(context.Context, *pluginv1.DescribeRequest) (*pluginv1.DescribeResponse, error) {
	m := pluginv1.Manifest_builder{Id: proto.String(ID), Version: proto.String(p.version)}.Build()
	return pluginv1.DescribeResponse_builder{Manifest: m}.Build(), nil
}

// Health implements PluginService: healthy while an enabled solver can
// solve.
func (p *plugin) Health(context.Context, *pluginv1.HealthRequest) (*pluginv1.HealthResponse, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	var errs []error
	for _, s := range p.solvers {
		if s.err == nil {
			return pluginv1.HealthResponse_builder{Healthy: proto.Bool(true)}.Build(), nil
		}
		errs = append(errs, s.err)
	}
	msg := "no solver is enabled"
	if len(errs) > 0 {
		msg = errors.Join(errs...).Error()
	}
	return pluginv1.HealthResponse_builder{Healthy: proto.Bool(false), Message: &msg}.Build(), nil
}

// Shutdown implements PluginService.
func (p *plugin) Shutdown(context.Context, *pluginv1.ShutdownRequest) (*pluginv1.ShutdownResponse, error) {
	p.close()
	return &pluginv1.ShutdownResponse{}, nil
}

// close stops sweeping and destroys the sessions created.
func (p *plugin) close() {
	p.mu.Lock()
	old := p.solvers
	p.solvers = nil
	p.stop()
	p.mu.Unlock()
	closeSolvers(p.log, old)
}

func closeSolvers(log *slog.Logger, solvers []*solver) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for _, s := range solvers {
		if s.fs != nil {
			if err := s.fs.Close(ctx); err != nil {
				log.Warn("destroy sessions", "solver", s.name, "err", err)
			}
		}
	}
}

// Configure implements PluginService: the solvers it enables replace
// those enabled, whose sessions are destroyed.
func (p *plugin) Configure(ctx context.Context, req *pluginv1.ConfigureRequest) (*pluginv1.ConfigureResponse, error) {
	var c config
	if s := req.GetConfigJson(); s != "" {
		if err := json.Unmarshal([]byte(s), &c); err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
	}
	if len(c.Solvers) == 0 {
		c.Solvers = []solverConfig{{Name: FlareSolverr}}
	}
	ttl := 30 * time.Minute
	if c.SessionTTLMinutes != nil {
		ttl = time.Duration(*c.SessionTTLMinutes) * time.Minute
	}
	deps := p.containers(ctx)
	var solvers []*solver
	seen := map[string]bool{}
	for i, sc := range c.Solvers {
		switch {
		case sc.Name != FlareSolverr:
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("solvers[%d]: unknown %q", i, sc.Name))
		case seen[sc.Name]:
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("solvers[%d]: %q is repeated", i, sc.Name))
		}
		seen[sc.Name] = true
		if sc.Enabled != nil && !*sc.Enabled {
			continue
		}
		s := &solver{name: sc.Name, timeout: 60 * time.Second, disableMedia: sc.DisableMedia == nil || *sc.DisableMedia}
		if sc.TimeoutSeconds > 0 {
			s.timeout = time.Duration(sc.TimeoutSeconds) * time.Second
		}
		dep := deps[FlareSolverr]
		switch {
		case dep == nil:
			s.err = errors.New("FlareSolverr is not set up: choose how it runs in the plugin's settings")
		case dep.GetAddress() == "":
			s.err = fmt.Errorf("FlareSolverr is not running (%s)", depState(dep))
		default:
			s.managed = dep.GetMode() == pluginv1.ContainerMode_CONTAINER_MODE_MANAGED
			s.fs = flaresolverr.New(&flaresolverr.Client{URL: dep.GetAddress(), HTTP: p.http}, flaresolverr.Options{
				TTL: ttl, MaxConcurrent: c.MaxConcurrent, Logger: p.log.With("solver", sc.Name),
			})
		}
		if s.err != nil {
			p.log.Warn("configure solver", "solver", s.name, "err", s.err)
		}
		solvers = append(solvers, s)
	}
	sweepCtx, stop := context.WithCancel(p.ctx)
	p.mu.Lock()
	old := p.solvers
	p.solvers = solvers
	p.stop()
	p.stop = stop
	p.mu.Unlock()
	go closeSolvers(p.log, old)
	go p.sweep(sweepCtx, solvers)
	return &pluginv1.ConfigureResponse{}, nil
}

func depState(dep *pluginv1.ContainerStatus) string {
	state := strings.ToLower(strings.TrimPrefix(dep.GetState().String(), "CONTAINER_STATE_"))
	if dep.GetError() != "" {
		return state + ": " + dep.GetError()
	}
	return state
}

// sweep destroys the solvers' idle sessions, and those left from before,
// every minute until ctx ends.
func (p *plugin) sweep(ctx context.Context, solvers []*solver) {
	t := time.NewTicker(sweepEvery)
	defer t.Stop()
	for {
		for _, s := range solvers {
			if s.fs != nil {
				if err := s.fs.Sweep(ctx); err != nil && ctx.Err() == nil {
					p.log.Warn("destroy idle sessions", "solver", s.name, "err", err)
				}
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// containers returns the plugin's container dependencies by name; none
// when the host cannot tell. The server configures the plugin again when
// one's address changes.
func (p *plugin) containers(ctx context.Context) map[string]*pluginv1.ContainerStatus {
	if p.host == nil || p.host() == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	resp, err := pluginv1connect.NewHostServiceClient(p.host(), guest.HostURL).ListContainers(ctx, &pluginv1.ListContainersRequest{})
	if err != nil {
		p.log.Warn("list container dependencies", "err", err)
		return nil
	}
	out := map[string]*pluginv1.ContainerStatus{}
	for _, c := range resp.GetContainers() {
		out[c.GetName()] = c
	}
	return out
}

// Solve implements ChallengeSolverService: the enabled solvers are tried
// in turn.
func (p *plugin) Solve(ctx context.Context, req *pluginv1.SolveRequest) (*pluginv1.SolveResponse, error) {
	c := req.GetChallenge()
	post := c.GetMethod() == http.MethodPost
	if post && c.GetContentType() != "" {
		if mt, _, err := mime.ParseMediaType(c.GetContentType()); err != nil || mt != "application/x-www-form-urlencoded" {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("FlareSolverr posts only forms, not %q", c.GetContentType()))
		}
	}
	p.mu.RLock()
	solvers := p.solvers
	p.mu.RUnlock()
	if len(solvers) == 0 {
		return nil, connect.NewError(connect.CodeUnavailable, errors.New("no solver is enabled"))
	}
	var errs []error
	for _, s := range solvers {
		if s.err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", s.name, s.err))
			continue
		}
		px, err := flareProxy(req.GetProxy(), s.managed)
		if err != nil {
			return nil, connect.NewError(connect.CodeFailedPrecondition, err)
		}
		timeout := s.timeout
		if t := c.GetTimeoutSeconds(); t > 0 {
			timeout = time.Duration(t) * time.Second
		}
		// FlareSolverr's own timeout covers the challenge alone.
		jctx, cancel := context.WithTimeout(ctx, timeout+30*time.Second)
		sol, err := s.fs.Solve(jctx, flaresolverr.Job{
			URL: c.GetUrl(), Post: post, PostData: string(c.GetBody()), Proxy: px, Timeout: timeout, DisableMedia: s.disableMedia,
		})
		cancel()
		if err == nil {
			return pluginv1.SolveResponse_builder{Solution: solution(sol, ID+"/"+s.name)}.Build(), nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		p.log.WarnContext(ctx, "solve challenge", "solver", s.name, "url", c.GetUrl(), "err", err)
		errs = append(errs, fmt.Errorf("%s: %w", s.name, err))
	}
	return nil, connect.NewError(connect.CodeUnavailable, errors.Join(errs...))
}

func solution(sol flaresolverr.Solution, solver string) *pluginv1.ChallengeSolution {
	cookies := make([]*pluginv1.Cookie, len(sol.Cookies))
	for i, c := range sol.Cookies {
		cookies[i] = pluginv1.Cookie_builder{Name: &c.Name, Value: &c.Value, Domain: &c.Domain, Path: &c.Path}.Build()
	}
	return pluginv1.ChallengeSolution_builder{Cookies: cookies, UserAgent: &sol.UserAgent, Solver: &solver}.Build()
}

// flareProxy returns a proxy as FlareSolverr takes it, nil for none. A
// proxy on the loopback interface is reached at hostName from a managed
// FlareSolverr's container. FlareSolverr's browser cannot authenticate
// to a SOCKS proxy.
func flareProxy(p *pluginv1.Proxy, managed bool) (*flaresolverr.Proxy, error) {
	u := proxy.URL(p)
	if u == nil {
		return nil, nil
	}
	if u.Scheme != "http" && p.GetUsername() != "" {
		return nil, fmt.Errorf("proxy %s: FlareSolverr's browser cannot sign in to a SOCKS proxy; use one without a username", p.GetName())
	}
	host := p.GetHost()
	if managed && loopback(host) {
		host = hostName
	}
	return &flaresolverr.Proxy{
		URL:      u.Scheme + "://" + net.JoinHostPort(host, strconv.Itoa(int(p.GetPort()))),
		Username: p.GetUsername(), Password: p.GetPassword(),
	}, nil
}

func loopback(host string) bool {
	if strings.EqualFold(host, "localhost") || strings.HasSuffix(strings.ToLower(host), ".localhost") {
		return true
	}
	addr, err := netip.ParseAddr(host)
	return err == nil && addr.Unmap().IsLoopback()
}
