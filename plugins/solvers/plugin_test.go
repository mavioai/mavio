package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

// flare is a FlareSolverr recording the proxies of its sessions.
type flare struct {
	mu       sync.Mutex
	sessions map[string]json.RawMessage
	gets     []map[string]any
}

func (f *flare) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var cmd map[string]json.RawMessage
	_ = json.NewDecoder(r.Body).Decode(&cmd)
	var name, session string
	_ = json.Unmarshal(cmd["cmd"], &name)
	_ = json.Unmarshal(cmd["session"], &session)
	f.mu.Lock()
	defer f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	switch name {
	case "sessions.create":
		f.sessions[session] = cmd["proxy"]
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	case "sessions.destroy":
		delete(f.sessions, session)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	case "sessions.list":
		names := []string{}
		for n := range f.sessions {
			names = append(names, n)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "sessions": names})
	default:
		var get map[string]any
		raw, _ := json.Marshal(cmd)
		_ = json.Unmarshal(raw, &get)
		f.gets = append(f.gets, get)
		_, _ = w.Write([]byte(`{"status":"ok","solution":{"url":"https://tracker.example/","status":200,` +
			`"cookies":[{"name":"cf_clearance","value":"v","domain":".tracker.example","path":"/"}],"userAgent":"Mozilla/5.0"}}`))
	}
}

// hostAPI answers ListContainers with the containers.
func hostAPI(t *testing.T, containers ...*pluginv1.ContainerStatus) func() *http.Client {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/mavio.plugin.v1.HostService/ListContainers" {
			http.NotFound(w, r)
			return
		}
		out, _ := proto.Marshal(pluginv1.ListContainersResponse_builder{Containers: containers}.Build())
		w.Header().Set("Content-Type", "application/proto")
		_, _ = w.Write(out)
	}))
	t.Cleanup(srv.Close)
	// The host client reaches guest.HostURL; send it to srv.
	c := srv.Client()
	c.Transport = rewrite{to: srv.URL, next: c.Transport}
	return func() *http.Client { return c }
}

type rewrite struct {
	to   string
	next http.RoundTripper
}

func (r rewrite) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.URL.Scheme, req.URL.Host = "http", strings.TrimPrefix(r.to, "http://")
	return r.next.RoundTrip(req)
}

func dep(mode pluginv1.ContainerMode, address string) *pluginv1.ContainerStatus {
	return pluginv1.ContainerStatus_builder{
		Name: proto.String(FlareSolverr), Mode: &mode, Address: &address, State: pluginv1.ContainerState_CONTAINER_STATE_READY.Enum(),
	}.Build()
}

func newTestPlugin(t *testing.T, host func() *http.Client, cfg string) *plugin {
	t.Helper()
	p := newPlugin(t.Context(), slog.New(slog.DiscardHandler), "test", &http.Client{}, host)
	t.Cleanup(p.close)
	if _, err := p.Configure(t.Context(), pluginv1.ConfigureRequest_builder{ConfigJson: &cfg}.Build()); err != nil {
		t.Fatal(err)
	}
	return p
}

func challenge(url string) *pluginv1.Challenge {
	return pluginv1.Challenge_builder{Url: &url}.Build()
}

func proxyOf(kind pluginv1.ProxyKind, host string, port int32, user, password string) *pluginv1.Proxy {
	return pluginv1.Proxy_builder{
		Id: proto.String("p1"), Name: proto.String("Home"), Kind: &kind, Host: &host, Port: &port,
		Username: &user, Password: &password,
	}.Build()
}

func TestSolve(t *testing.T) {
	f := &flare{sessions: map[string]json.RawMessage{}}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	p := newTestPlugin(t, hostAPI(t, dep(pluginv1.ContainerMode_CONTAINER_MODE_MANAGED, srv.URL)), `{"solvers":[{"name":"flaresolverr","timeout_seconds":90}]}`)

	health, _ := p.Health(t.Context(), &pluginv1.HealthRequest{})
	if !health.GetHealthy() {
		t.Errorf("Health = %v, want healthy", health)
	}
	resp, err := p.Solve(t.Context(), pluginv1.SolveRequest_builder{
		Challenge: challenge("https://tracker.example/"),
		Proxy:     proxyOf(pluginv1.ProxyKind_PROXY_KIND_HTTP, "127.0.0.1", 7890, "u", "p"),
	}.Build())
	if err != nil {
		t.Fatal(err)
	}
	sol := resp.GetSolution()
	if sol.GetSolver() != "dev.mavio.solvers/flaresolverr" || sol.GetUserAgent() != "Mozilla/5.0" ||
		len(sol.GetCookies()) != 1 || sol.GetCookies()[0].GetDomain() != ".tracker.example" {
		t.Errorf("solution = %v", sol)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.sessions) != 1 {
		t.Fatalf("sessions = %v, want one", f.sessions)
	}
	for _, px := range f.sessions {
		// The managed container reaches the host's loopback proxy by name.
		if want := `{"url":"http://host.docker.internal:7890","username":"u","password":"p"}`; string(px) != want {
			t.Errorf("session proxy = %s, want = %s", px, want)
		}
	}
	if g := f.gets[0]; g["maxTimeout"] != 90000.0 || g["disableMedia"] != true || g["returnOnlyCookies"] != true {
		t.Errorf("request = %v, want 90 s, without media, only cookies", g)
	}
}

func TestSolveRefused(t *testing.T) {
	f := &flare{sessions: map[string]json.RawMessage{}}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	p := newTestPlugin(t, hostAPI(t, dep(pluginv1.ContainerMode_CONTAINER_MODE_EXTERNAL, srv.URL)), `{}`)
	tests := []struct {
		name string
		req  *pluginv1.SolveRequest
		code connect.Code
	}{
		{
			name: "SOCKS with credentials", code: connect.CodeFailedPrecondition,
			req: pluginv1.SolveRequest_builder{
				Challenge: challenge("https://tracker.example/"),
				Proxy:     proxyOf(pluginv1.ProxyKind_PROXY_KIND_SOCKS5, "proxy", 1080, "u", "p"),
			}.Build(),
		},
		{
			name: "JSON body", code: connect.CodeInvalidArgument,
			req: pluginv1.SolveRequest_builder{Challenge: pluginv1.Challenge_builder{
				Url: proto.String("https://tracker.example/"), Method: proto.String("POST"), ContentType: proto.String("application/json"),
			}.Build()}.Build(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := p.Solve(t.Context(), tt.req); connect.CodeOf(err) != tt.code {
				t.Errorf("err = %v, want = %v", err, tt.code)
			}
		})
	}

	// An external FlareSolverr takes a loopback proxy as it is.
	_, err := p.Solve(t.Context(), pluginv1.SolveRequest_builder{
		Challenge: challenge("https://tracker.example/"),
		Proxy:     proxyOf(pluginv1.ProxyKind_PROXY_KIND_SOCKS5, "::1", 1080, "", ""),
	}.Build())
	if err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, px := range f.sessions {
		if want := `{"url":"socks5://[::1]:1080"}`; string(px) != want {
			t.Errorf("session proxy = %s, want = %s", px, want)
		}
	}
}

func TestUnavailable(t *testing.T) {
	tests := []struct {
		name, cfg string
		deps      []*pluginv1.ContainerStatus
		message   string
	}{
		{name: "not set up", cfg: `{}`, message: "not set up"},
		{name: "not running", cfg: `{}`, deps: []*pluginv1.ContainerStatus{dep(pluginv1.ContainerMode_CONTAINER_MODE_MANAGED, "")}, message: "not running"},
		{name: "disabled", cfg: `{"solvers":[{"name":"flaresolverr","enabled":false}]}`, message: "no solver is enabled"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := newTestPlugin(t, hostAPI(t, tt.deps...), tt.cfg)
			health, _ := p.Health(t.Context(), &pluginv1.HealthRequest{})
			if health.GetHealthy() || !strings.Contains(health.GetMessage(), tt.message) {
				t.Errorf("Health = %v, want unhealthy: %s", health, tt.message)
			}
			_, err := p.Solve(t.Context(), pluginv1.SolveRequest_builder{Challenge: challenge("https://tracker.example/")}.Build())
			if connect.CodeOf(err) != connect.CodeUnavailable || !strings.Contains(err.Error(), tt.message) {
				t.Errorf("err = %v, want unavailable: %s", err, tt.message)
			}
		})
	}
}

func TestConfigureRefused(t *testing.T) {
	p := newPlugin(t.Context(), slog.New(slog.DiscardHandler), "test", &http.Client{}, nil)
	t.Cleanup(p.close)
	for _, cfg := range []string{
		`{"solvers":[{"name":"byparr"}]}`,
		`{"solvers":[{"name":"flaresolverr"},{"name":"flaresolverr"}]}`,
		`not json`,
	} {
		if _, err := p.Configure(t.Context(), pluginv1.ConfigureRequest_builder{ConfigJson: &cfg}.Build()); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("Configure(%s) err = %v, want invalid argument", cfg, err)
		}
	}
}

// TestLive solves a page with the FlareSolverr at MAVIO_FLARESOLVERR_URL.
func TestLive(t *testing.T) {
	addr := os.Getenv("MAVIO_FLARESOLVERR_URL")
	if addr == "" {
		t.Skip("MAVIO_FLARESOLVERR_URL is not set")
	}
	p := newTestPlugin(t, hostAPI(t, dep(pluginv1.ContainerMode_CONTAINER_MODE_EXTERNAL, addr)), `{}`)
	resp, err := p.Solve(context.WithoutCancel(t.Context()), pluginv1.SolveRequest_builder{Challenge: challenge("https://www.cloudflare.com/")}.Build())
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetSolution().GetUserAgent() == "" || len(resp.GetSolution().GetCookies()) == 0 {
		t.Errorf("solution = %v, want cookies and a user agent", resp.GetSolution())
	}
}
