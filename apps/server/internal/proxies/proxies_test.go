package proxies

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/mavioai/mavio/apps/server/internal/sysproxy"

	"github.com/mavioai/mavio/libs/core"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/store"
)

func newService(t *testing.T, plugins Plugins) *Service {
	t.Helper()
	st, err := store.Open(t.Context(), "sqlite:"+filepath.Join(t.TempDir(), "mavio.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	s := New(st.Proxies(), plugins, slog.New(slog.DiscardHandler))
	s.detect = func(context.Context) (sysproxy.Setting, error) { return sysproxy.Setting{}, nil }
	return s
}

// mutableSystem makes s read the system proxy from the returned
// function's last call, which sets the proxy u, none for "".
func mutableSystem(t *testing.T, s *Service) func(u string, bypass ...string) {
	var current atomic.Pointer[sysproxy.Setting]
	current.Store(&sysproxy.Setting{})
	s.detect = func(context.Context) (sysproxy.Setting, error) { return *current.Load(), nil }
	return func(u string, bypass ...string) {
		t.Helper()
		set := sysproxy.Setting{Bypass: bypass}
		if u != "" {
			parsed, err := url.Parse(u)
			if err != nil {
				t.Fatal(err)
			}
			set.URL = parsed
		}
		current.Store(&set)
	}
}

type fakePlugins struct {
	users   []string
	solvers []Solver

	mu           sync.Mutex
	reconfigured []string
}

func (f *fakePlugins) UsesProxies(id string) bool { return slices.Contains(f.users, id) }
func (f *fakePlugins) ProxyUsers() []string       { return f.users }
func (f *fakePlugins) ChallengeSolvers() []Solver { return f.solvers }

func (f *fakePlugins) Reconfigure(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reconfigured = append(f.reconfigured, id)
	return nil
}

func (f *fakePlugins) takeReconfigured() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := f.reconfigured
	f.reconfigured = nil
	return out
}

type fakeSolver struct {
	sol   *pluginv1.ChallengeSolution
	err   error
	calls int
	proxy *pluginv1.Proxy
}

func (f *fakeSolver) Solve(_ context.Context, req *pluginv1.SolveRequest) (*pluginv1.SolveResponse, error) {
	f.calls++
	f.proxy = req.GetProxy()
	if f.err != nil {
		return nil, f.err
	}
	return pluginv1.SolveResponse_builder{Solution: f.sol}.Build(), nil
}

func solution(solver string) *pluginv1.ChallengeSolution {
	return pluginv1.ChallengeSolution_builder{
		Cookies:   []*pluginv1.Cookie{pluginv1.Cookie_builder{Name: new("cf_clearance"), Value: new("x")}.Build()},
		UserAgent: new("Mozilla/5.0"),
		Solver:    &solver,
	}.Build()
}

func socks5(name string) core.Proxy {
	return core.Proxy{
		Name: name, Kind: core.ProxySOCKS5, Host: "proxy.example", Port: 1080,
		Username: "u", Password: "secret",
	}
}

func TestUpdateKeepsPassword(t *testing.T) {
	plugins := &fakePlugins{users: []string{"indexers", "subtitles"}}
	s := newService(t, plugins)
	ctx := t.Context()
	p, err := s.Create(ctx, socks5(" Home "))
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "Home" {
		t.Errorf("Name = %q, want = %q", p.Name, "Home")
	}
	tests := []struct {
		name     string
		username string
		password string
		clear    bool
		want     string
	}{
		{name: "kept", username: "u", want: "secret"},
		{name: "replaced", username: "u", password: "new", want: "new"},
		{name: "cleared", username: "u", clear: true, want: ""},
		{name: "other user", username: "v", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := s.Update(ctx, p, false); err != nil { // restore
				t.Fatal(err)
			}
			q := p
			q.Username, q.Password = tt.username, tt.password
			if _, err := s.Update(ctx, q, tt.clear); err != nil {
				t.Fatal(err)
			}
			got, err := s.Get(ctx, p.ID)
			if err != nil {
				t.Fatal(err)
			}
			if got.Password != tt.want {
				t.Errorf("Password = %q, want = %q", got.Password, tt.want)
			}
		})
	}
	if got := plugins.takeReconfigured(); len(got) < 2 || got[0] != "indexers" || got[1] != "subtitles" {
		t.Errorf("reconfigured = %v, want = indexers and subtitles after each update", got)
	}
	if err := s.Delete(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	if got, want := plugins.takeReconfigured(), []string{"indexers", "subtitles"}; !slices.Equal(got, want) {
		t.Errorf("reconfigured after delete = %v, want = %v", got, want)
	}
}

func TestCreateConflict(t *testing.T) {
	s := newService(t, nil)
	ctx := t.Context()
	if _, err := s.Create(ctx, socks5("A")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create(ctx, socks5("B")); !errors.Is(err, core.ErrConflict) {
		t.Errorf("same address: err = %v, want = %v", err, core.ErrConflict)
	}
	other := socks5("a")
	other.Port = 1081
	if _, err := s.Create(ctx, other); !errors.Is(err, core.ErrConflict) {
		t.Errorf("same name: err = %v, want = %v", err, core.ErrConflict)
	}
}

func TestForPlugin(t *testing.T) {
	s := newService(t, nil)
	ctx := t.Context()
	a, err := s.Create(ctx, socks5("A"))
	if err != nil {
		t.Fatal(err)
	}
	b := socks5("B")
	b.Port = 1081
	if _, err := s.Create(ctx, b); err != nil {
		t.Fatal(err)
	}
	all, err := s.ForPlugin(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 || all[0].GetId() != SystemID {
		t.Errorf("all = %v, want = the system proxy and both", all)
	}
	some, err := s.ForPlugin(ctx, []string{a.ID.String(), core.NewID().String()})
	if err != nil {
		t.Fatal(err)
	}
	if len(some) != 1 || some[0].GetId() != a.ID.String() || some[0].GetPassword() != "secret" ||
		some[0].GetKind() != pluginv1.ProxyKind_PROXY_KIND_SOCKS5 {
		t.Errorf("some = %v, want = A with its password", some)
	}
}

func TestSolve(t *testing.T) {
	ctx := t.Context()
	challenge := pluginv1.Challenge_builder{Url: new("https://tracker.example/")}.Build()

	t.Run("no solver", func(t *testing.T) {
		s := newService(t, &fakePlugins{})
		if _, err := s.Solve(ctx, challenge, ""); !errors.Is(err, ErrNoSolver) {
			t.Errorf("err = %v, want = %v", err, ErrNoSolver)
		}
	})

	t.Run("unknown proxy", func(t *testing.T) {
		s := newService(t, &fakePlugins{solvers: []Solver{{ID: "solvers", Client: &fakeSolver{sol: solution("")}}}})
		if _, err := s.Solve(ctx, challenge, core.NewID().String()); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("err = %v, want = %v", err, core.ErrNotFound)
		}
	})

	t.Run("in turn", func(t *testing.T) {
		failing := &fakeSolver{err: errors.New("timed out")}
		empty := &fakeSolver{sol: pluginv1.ChallengeSolution_builder{UserAgent: new("ua")}.Build()}
		solving := &fakeSolver{sol: solution("")}
		after := &fakeSolver{sol: solution("after")}
		s := newService(t, &fakePlugins{solvers: []Solver{
			{ID: "a", Client: failing}, {ID: "b", Client: empty}, {ID: "c", Client: solving}, {ID: "d", Client: after},
		}})
		p, err := s.Create(ctx, socks5("Home"))
		if err != nil {
			t.Fatal(err)
		}
		sol, err := s.Solve(ctx, challenge, p.ID.String())
		if err != nil {
			t.Fatal(err)
		}
		if sol.GetSolver() != "c" {
			t.Errorf("Solver = %q, want = %q", sol.GetSolver(), "c")
		}
		if after.calls != 0 {
			t.Errorf("solver after the solving one called %d times, want = 0", after.calls)
		}
		if got := solving.proxy; got.GetHost() != "proxy.example" || got.GetPassword() != "secret" {
			t.Errorf("proxy = %v, want = Home with its password", got)
		}
	})

	t.Run("all fail", func(t *testing.T) {
		s := newService(t, &fakePlugins{solvers: []Solver{
			{ID: "a", Client: &fakeSolver{err: errors.New("timed out")}},
			{ID: "b", Client: &fakeSolver{sol: pluginv1.ChallengeSolution_builder{}.Build()}},
		}})
		_, err := s.Solve(ctx, challenge, "")
		if err == nil || err.Error() != "solver a: timed out\nsolver b: no cookies" {
			t.Errorf("err = %v, want = both solvers' errors", err)
		}
	})
}

func TestTest(t *testing.T) {
	// An HTTP proxy answering every request itself, as Cloudflare's trace.
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Proxy-Authorization")
		if !r.URL.IsAbs() {
			http.Error(w, "not a proxy request", http.StatusBadRequest)
			return
		}
		_, _ = io.WriteString(w, "fl=1\nip=203.0.113.7\nloc=DE\n")
	}))
	t.Cleanup(srv.Close)
	host, port, err := net.SplitHostPort(srv.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	n, err := strconv.Atoi(port)
	if err != nil {
		t.Fatal(err)
	}
	s := newService(t, nil)
	p, err := s.Create(t.Context(), core.Proxy{
		Name: "Local", Kind: core.ProxyHTTP, Host: host, Port: n, Username: "u", Password: "secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	p.Password = "" // the stored one is used
	got, err := s.Test(t.Context(), p, "http://trace.example/cdn-cgi/trace")
	if err != nil {
		t.Fatal(err)
	}
	if !got.OK || got.Status != http.StatusOK || got.ExitIP != "203.0.113.7" || got.Country != "DE" {
		t.Errorf("Test = %+v, want = OK 200 from 203.0.113.7 in DE", got)
	}
	if auth == "" {
		t.Error("Proxy-Authorization not sent, want = the stored credentials")
	}

	unreachable := core.Proxy{Name: "Gone", Kind: core.ProxyHTTP, Host: "127.0.0.1", Port: 1}
	got, err = s.Test(t.Context(), unreachable, "http://trace.example/")
	if err != nil {
		t.Fatal(err)
	}
	if got.OK || got.Err == "" {
		t.Errorf("Test = %+v, want = not OK with an error", got)
	}

	if _, err := s.Test(t.Context(), core.Proxy{Name: "Bad", Kind: core.ProxyHTTP}, ""); !errors.Is(err, core.ErrInvalid) {
		t.Errorf("err = %v, want = %v", err, core.ErrInvalid)
	}
}

func TestSystem(t *testing.T) {
	plugins := &fakePlugins{users: []string{"indexers"}}
	s := newService(t, plugins)
	ctx := t.Context()
	if got := s.System(ctx); got.GetId() != SystemID || got.GetKind() != pluginv1.ProxyKind_PROXY_KIND_UNSPECIFIED {
		t.Errorf("System() with none set = %v, want = system of no kind", got)
	}
	if s.refresh(ctx) {
		t.Error("refresh() unchanged = true, want = false")
	}
	setSystem := mutableSystem(t, s)
	setSystem("http://u:p@127.0.0.1:1080", "192.168.0.0/16", "<local>")
	if !s.refresh(ctx) {
		t.Error("refresh() changed = false, want = true")
	}
	got := s.System(ctx)
	if got.GetKind() != pluginv1.ProxyKind_PROXY_KIND_HTTP || got.GetHost() != "127.0.0.1" || got.GetPort() != 1080 ||
		got.GetUsername() != "u" || got.GetPassword() != "p" || !slices.Equal(got.GetBypass(), []string{"192.168.0.0/16", "<local>"}) {
		t.Errorf("System() = %v, want = http://u:p@127.0.0.1:1080 bypassing two", got)
	}
	only, err := s.ForPlugin(ctx, []string{SystemID})
	if err != nil {
		t.Fatal(err)
	}
	if len(only) != 1 || only[0].GetHost() != "127.0.0.1" {
		t.Errorf("ForPlugin(system) = %v, want = the system proxy", only)
	}

	solver := &fakeSolver{sol: solution("")}
	plugins.solvers = []Solver{{ID: "solvers", Client: solver}}
	if _, err := s.Solve(ctx, pluginv1.Challenge_builder{Url: new("https://tracker.example/")}.Build(), SystemID); err != nil {
		t.Fatal(err)
	}
	if solver.proxy.GetHost() != "127.0.0.1" {
		t.Errorf("solved through %v, want = the system proxy", solver.proxy)
	}
	setSystem("")
	s.refresh(ctx)
	if _, err := s.Solve(ctx, pluginv1.Challenge_builder{Url: new("https://tracker.example/")}.Build(), SystemID); err != nil {
		t.Fatal(err)
	}
	if solver.proxy != nil {
		t.Errorf("solved through %v with the system setting none, want = directly", solver.proxy)
	}
}

func TestRunReconfigures(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		plugins := &fakePlugins{users: []string{"indexers"}}
		s := newService(t, plugins)
		setSystem := mutableSystem(t, s)
		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan error)
		go func() { done <- s.Run(ctx) }()
		synctest.Wait()
		if got := plugins.takeReconfigured(); len(got) != 0 {
			t.Errorf("reconfigured on start = %v, want = none", got)
		}
		setSystem("socks5://127.0.0.1:1086")
		time.Sleep(systemPoll)
		synctest.Wait()
		if got := plugins.takeReconfigured(); !slices.Equal(got, []string{"indexers"}) {
			t.Errorf("reconfigured = %v, want = [indexers]", got)
		}
		time.Sleep(systemPoll)
		synctest.Wait()
		if got := plugins.takeReconfigured(); len(got) != 0 {
			t.Errorf("reconfigured unchanged = %v, want = none", got)
		}
		cancel()
		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Errorf("Run() = %v, want = %v", err, context.Canceled)
		}
	})
}

// fakeProxy is an HTTP proxy answering every request itself with its
// name, recording the hosts asked for.
func fakeProxy(t *testing.T, name string) (*httptest.Server, func() []string) {
	t.Helper()
	var mu sync.Mutex
	var hosts []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hosts = append(hosts, r.URL.Host)
		mu.Unlock()
		_, _ = io.WriteString(w, name)
	}))
	t.Cleanup(srv.Close)
	return srv, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(hosts)
	}
}

func httpProxy(name string, srv *httptest.Server) core.Proxy {
	host, port, _ := net.SplitHostPort(srv.Listener.Addr().String())
	n, _ := strconv.Atoi(port)
	return core.Proxy{Name: name, Kind: core.ProxyHTTP, Host: host, Port: n}
}

func TestThrough(t *testing.T) {
	s := newService(t, nil)
	setSystem := mutableSystem(t, s)
	ctx := t.Context()
	one, _ := fakeProxy(t, "one")
	two, _ := fakeProxy(t, "two")
	system, systemHosts := fakeProxy(t, "system")
	direct := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "direct")
	}))
	t.Cleanup(direct.Close)

	p, err := s.Create(ctx, httpProxy("office", one))
	if err != nil {
		t.Fatal(err)
	}
	var choice atomic.Value
	choice.Store(p.ID.String())
	client := &http.Client{Transport: s.Through(func(*http.Request) string { return choice.Load().(string) })}
	get := func(u string) (string, error) {
		t.Helper()
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		resp, err := client.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		return string(body), err
	}
	expect := func(step, u, want string) {
		t.Helper()
		if got, err := get(u); err != nil || got != want {
			t.Errorf("%s: GET %s got = %q, %v; want = %q", step, u, got, err, want)
		}
	}

	expect("stored proxy", "http://api.themoviedb.org/3", "one")
	// This machine is reached directly whichever the proxy.
	expect("local address", direct.URL, "direct")
	// A changed proxy is used from the next request.
	changed := httpProxy("office", two)
	changed.ID = p.ID
	if _, err := s.Update(ctx, changed, false); err != nil {
		t.Fatal(err)
	}
	expect("changed proxy", "http://api.themoviedb.org/3", "two")

	// The system proxy follows the system's settings.
	choice.Store(SystemID)
	setSystem(system.URL)
	s.refresh(ctx)
	expect("system proxy", "http://image.tmdb.org/t/p/w500", "system")
	if got := systemHosts(); !slices.Equal(got, []string{"image.tmdb.org"}) {
		t.Errorf("system proxy hosts got = %q, want = [image.tmdb.org]", got)
	}
	setSystem("")
	s.refresh(ctx)
	expect("system proxy setting none", direct.URL, "direct")

	// A proxy the settings use is not deleted; one deleted fails requests.
	choice.Store(p.ID.String())
	s.SetUses(func(id string) []string {
		if id == p.ID.String() {
			return []string{"images"}
		}
		return nil
	})
	if err := s.Delete(ctx, p.ID); !errors.Is(err, ErrInUse) {
		t.Errorf("Delete of a proxy in use got = %v, want = ErrInUse", err)
	}
	s.SetUses(nil)
	if err := s.Delete(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := get("http://api.themoviedb.org/3"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("GET through a deleted proxy got = %v, want = ErrNotFound", err)
	}
	choice.Store("")
	expect("no proxy", direct.URL, "direct")
}
