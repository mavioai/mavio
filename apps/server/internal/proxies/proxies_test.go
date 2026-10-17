package proxies

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strconv"
	"testing"

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
	return New(st.Proxies(), plugins, slog.New(slog.DiscardHandler))
}

type fakePlugins struct {
	users        []string
	reconfigured []string
	solvers      []Solver
}

func (f *fakePlugins) UsesProxies(id string) bool { return slices.Contains(f.users, id) }
func (f *fakePlugins) ProxyUsers() []string       { return f.users }
func (f *fakePlugins) ChallengeSolvers() []Solver { return f.solvers }

func (f *fakePlugins) Reconfigure(_ context.Context, id string) error {
	f.reconfigured = append(f.reconfigured, id)
	return nil
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
	if len(plugins.reconfigured) == 0 || plugins.reconfigured[0] != "indexers" || plugins.reconfigured[1] != "subtitles" {
		t.Errorf("reconfigured = %v, want = indexers and subtitles after each update", plugins.reconfigured)
	}
	plugins.reconfigured = nil
	if err := s.Delete(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	if want := []string{"indexers", "subtitles"}; !slices.Equal(plugins.reconfigured, want) {
		t.Errorf("reconfigured after delete = %v, want = %v", plugins.reconfigured, want)
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
	if len(all) != 2 {
		t.Errorf("len(all) = %d, want = 2", len(all))
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
