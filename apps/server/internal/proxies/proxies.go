package proxies

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/mavioai/mavio/apps/server/internal/sysproxy"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/plugin/proxy"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1/pluginv1connect"
)

// ErrNoSolver is a challenge with no ready solver plugin to solve it.
var ErrNoSolver = errors.New("no challenge solver is ready")

// DefaultTestURL is what TestProxy fetches when given no URL: Cloudflare's
// trace tells the address requests leave from and its country.
const DefaultTestURL = "https://www.cloudflare.com/cdn-cgi/trace"

// Solver is a started CHALLENGE_SOLVER plugin.
type Solver struct {
	ID     string
	Client pluginv1connect.ChallengeSolverServiceClient
}

// Plugins are the server's plugins as proxies concern them;
// *plugins.Manager implements it.
type Plugins interface {
	// UsesProxies reports whether a plugin's configuration takes proxies.
	UsesProxies(id string) bool
	// ProxyUsers lists the plugins whose configuration takes proxies.
	ProxyUsers() []string
	// Reconfigure delivers a ready plugin's configuration again.
	Reconfigure(ctx context.Context, id string) error
	// ChallengeSolvers lists the ready CHALLENGE_SOLVER plugins by ID.
	ChallengeSolvers() []Solver
}

// SystemID is the ID of the system proxy.
const SystemID = proxy.SystemID

// SystemName is the system proxy's name.
const SystemName = "System proxy"

// systemPoll is how often Run reads the system proxy again.
const systemPoll = 30 * time.Second

// Service keeps the server's proxies.
type Service struct {
	repo    core.ProxyRepository
	plugins Plugins
	log     *slog.Logger
	// detect reads the system proxy; sysproxy.Detect but in tests.
	detect func(context.Context) (sysproxy.Setting, error)
	// system is the system proxy last read, nil before the first.
	system atomic.Pointer[pluginv1.Proxy]
}

// New returns a Service keeping proxies in repo; plugins may be nil.
func New(repo core.ProxyRepository, plugins Plugins, log *slog.Logger) *Service {
	return &Service{repo: repo, plugins: plugins, log: log, detect: sysproxy.Detect}
}

// Run reads the system proxy every 30 seconds until ctx ends, configuring
// the plugins taking proxies again when it changes.
func (s *Service) Run(ctx context.Context) error {
	t := time.NewTicker(systemPoll)
	defer t.Stop()
	for {
		if s.refresh(ctx) {
			s.changed(ctx)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
		}
	}
}

// refresh reads the system proxy, reporting whether it changed since last
// read; one failing to read is none, logged.
func (s *Service) refresh(ctx context.Context) bool {
	set, err := s.detect(ctx)
	if err != nil {
		s.log.WarnContext(ctx, "read the system proxy", "err", err)
		set = sysproxy.Setting{}
	}
	p := systemProxy(set)
	old := s.system.Swap(p)
	return old != nil && !proto.Equal(old, p)
}

// System returns the system proxy, its kind unspecified when none is set.
func (s *Service) System(ctx context.Context) *pluginv1.Proxy {
	if p := s.system.Load(); p != nil {
		return p
	}
	s.refresh(ctx)
	return s.system.Load()
}

func systemProxy(set sysproxy.Setting) *pluginv1.Proxy {
	p := pluginv1.Proxy_builder{Id: new(SystemID), Name: new(SystemName)}.Build()
	if set.URL == nil {
		return p
	}
	port, _ := strconv.Atoi(set.URL.Port())
	p.SetKind(kinds[core.ProxyKind(set.URL.Scheme)])
	p.SetHost(set.URL.Hostname())
	p.SetPort(int32(port))
	p.SetUsername(set.URL.User.Username())
	password, _ := set.URL.User.Password()
	p.SetPassword(password)
	p.SetBypass(set.Bypass)
	return p
}

// List returns the proxies ordered by name.
func (s *Service) List(ctx context.Context) ([]core.Proxy, error) { return s.repo.List(ctx) }

// Get returns a proxy.
func (s *Service) Get(ctx context.Context, id core.ID) (core.Proxy, error) {
	return s.repo.Get(ctx, id)
}

// Create adds a proxy, giving it an ID.
func (s *Service) Create(ctx context.Context, p core.Proxy) (core.Proxy, error) {
	p.ID = core.NewID()
	clean(&p)
	if err := s.repo.Put(ctx, &p); err != nil {
		return core.Proxy{}, err
	}
	return p, nil
}

// Update replaces a proxy, keeping its stored password when p has none
// unless clearPassword, and configures the plugins taking proxies again.
func (s *Service) Update(ctx context.Context, p core.Proxy, clearPassword bool) (core.Proxy, error) {
	old, err := s.repo.Get(ctx, p.ID)
	if err != nil {
		return core.Proxy{}, err
	}
	clean(&p)
	keepPassword(&p, &old, clearPassword)
	if err := s.repo.Put(ctx, &p); err != nil {
		return core.Proxy{}, err
	}
	s.changed(ctx)
	return p, nil
}

// Delete deletes a proxy and configures the plugins taking proxies again,
// for them to fail what used it.
func (s *Service) Delete(ctx context.Context, id core.ID) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	s.changed(ctx)
	return nil
}

func clean(p *core.Proxy) {
	p.Name = strings.TrimSpace(p.Name)
	p.Host = strings.Trim(strings.TrimSpace(p.Host), "[]")
	p.Username = strings.TrimSpace(p.Username)
}

// keepPassword keeps old's password for p when p has none, the username
// unchanged and SOCKS4 not taking none.
func keepPassword(p, old *core.Proxy, clear bool) {
	if !clear && p.Password == "" && p.Username == old.Username && p.Kind != core.ProxySOCKS4 {
		p.Password = old.Password
	}
}

// changed configures the plugins taking proxies again; failures are
// logged, the plugins reporting them as they do any.
func (s *Service) changed(ctx context.Context) {
	if s.plugins == nil {
		return
	}
	for _, id := range s.plugins.ProxyUsers() {
		if err := s.plugins.Reconfigure(ctx, id); err != nil {
			s.log.WarnContext(ctx, "reconfigure plugin after proxies changed", "plugin", id, "err", err)
		}
	}
}

// UsesProxies reports whether a plugin's configuration takes proxies.
func (s *Service) UsesProxies(pluginID string) bool {
	return s.plugins != nil && s.plugins.UsesProxies(pluginID)
}

// ForPlugin returns the proxies of the IDs, all when none, the system
// proxy first; IDs not found are left out.
func (s *Service) ForPlugin(ctx context.Context, ids []string) ([]*pluginv1.Proxy, error) {
	list, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	var out []*pluginv1.Proxy
	if len(ids) == 0 || slices.Contains(ids, SystemID) {
		out = append(out, s.System(ctx))
	}
	for i := range list {
		if len(ids) == 0 || slices.Contains(ids, list[i].ID.String()) {
			out = append(out, ToPlugin(&list[i]))
		}
	}
	return out, nil
}

var kinds = map[core.ProxyKind]pluginv1.ProxyKind{
	core.ProxyHTTP:   pluginv1.ProxyKind_PROXY_KIND_HTTP,
	core.ProxySOCKS4: pluginv1.ProxyKind_PROXY_KIND_SOCKS4,
	core.ProxySOCKS5: pluginv1.ProxyKind_PROXY_KIND_SOCKS5,
}

// ToPlugin returns a proxy as plugins take it, with its credentials.
func ToPlugin(p *core.Proxy) *pluginv1.Proxy {
	return pluginv1.Proxy_builder{
		Id: new(p.ID.String()), Name: &p.Name, Kind: new(kinds[p.Kind]), Host: &p.Host, Port: new(int32(p.Port)),
		Username: &p.Username, Password: &p.Password,
	}.Build()
}

// Solve solves a challenge through the proxy of the ID ("" for none),
// asking the ready solver plugins in turn by ID until one solves it.
func (s *Service) Solve(ctx context.Context, c *pluginv1.Challenge, proxyID string) (*pluginv1.ChallengeSolution, error) {
	req := pluginv1.SolveRequest_builder{Challenge: c}.Build()
	switch proxyID {
	case "":
	case SystemID:
		if p := s.System(ctx); proxy.URL(p) != nil {
			req.SetProxy(p)
		}
	default:
		id, err := core.ParseID(proxyID)
		if err != nil {
			return nil, err
		}
		p, err := s.repo.Get(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("proxy %s: %w", proxyID, err)
		}
		req.SetProxy(ToPlugin(&p))
	}
	var solvers []Solver
	if s.plugins != nil {
		solvers = s.plugins.ChallengeSolvers()
	}
	if len(solvers) == 0 {
		return nil, ErrNoSolver
	}
	var errs []error
	for _, sv := range solvers {
		resp, err := sv.Client.Solve(ctx, req)
		if err == nil && len(resp.GetSolution().GetCookies()) > 0 {
			sol := resp.GetSolution()
			if sol.GetSolver() == "" {
				sol.SetSolver(sv.ID)
			}
			return sol, nil
		}
		if err == nil {
			err = errors.New("no cookies")
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		errs = append(errs, fmt.Errorf("solver %s: %w", sv.ID, err))
	}
	return nil, errors.Join(errs...)
}

// TestResult is what fetching a URL through a proxy gave.
type TestResult struct {
	// OK is whether the URL answered, with any status.
	OK bool
	// Err is why not.
	Err     string
	Status  int
	Latency time.Duration
	// ExitIP and Country are read from Cloudflare's trace.
	ExitIP, Country string
}

// TestSystem fetches a URL, DefaultTestURL when empty, through the system
// proxy, directly when none is set.
func (s *Service) TestSystem(ctx context.Context, url string) (TestResult, error) {
	return fetch(ctx, s.System(ctx), url)
}

// Test fetches a URL, DefaultTestURL when empty, through p; with p's ID
// and no password, the stored proxy's password is used.
func (s *Service) Test(ctx context.Context, p core.Proxy, url string) (TestResult, error) {
	clean(&p)
	if !p.ID.IsZero() && p.Password == "" {
		if old, err := s.repo.Get(ctx, p.ID); err == nil {
			keepPassword(&p, &old, false)
		}
	}
	if p.ID.IsZero() {
		p.ID = core.NewID() // only for Validate
	}
	if err := p.Validate(); err != nil {
		return TestResult{}, err
	}
	return fetch(ctx, ToPlugin(&p), url)
}

func fetch(ctx context.Context, p *pluginv1.Proxy, url string) (TestResult, error) {
	if url == "" {
		url = DefaultTestURL
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return TestResult{}, fmt.Errorf("%w: %w", core.ErrInvalid, err)
	}
	tr := proxy.Transport(p, nil)
	defer tr.CloseIdleConnections()
	start := time.Now()
	resp, err := (&http.Client{Transport: tr}).Do(req)
	if err != nil {
		return TestResult{Err: err.Error(), Latency: time.Since(start)}, nil
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	out := TestResult{OK: err == nil, Status: resp.StatusCode, Latency: time.Since(start)}
	if err != nil {
		out.Err = err.Error()
	}
	sc := bufio.NewScanner(bytes.NewReader(body))
	for sc.Scan() {
		k, v, _ := strings.Cut(sc.Text(), "=")
		switch k {
		case "ip":
			out.ExitIP = v
		case "loc":
			out.Country = v
		}
	}
	return out, nil
}
