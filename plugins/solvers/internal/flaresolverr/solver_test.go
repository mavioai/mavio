package flaresolverr

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"
)

// fake is a FlareSolverr in memory.
type fake struct {
	mu sync.Mutex
	// sessions are the live sessions with their proxies.
	sessions map[string]*Proxy
	// calls are the commands received, "cmd session".
	calls []string
	// gets are the request.get and request.post commands received.
	gets []command
	// block, when set, holds requests until closed.
	block chan struct{}
	// fail answers requests with this message.
	fail string
	// noCookies answers requests without cookies.
	noCookies bool
	// autoCreate creates a missing session without its proxy, as
	// FlareSolverr does.
	autoCreate bool
}

func newFake() *fake { return &fake{sessions: map[string]*Proxy{}} }

func (f *fake) RoundTrip(r *http.Request) (*http.Response, error) {
	w := httptest.NewRecorder()
	f.ServeHTTP(w, r)
	return w.Result(), nil
}

func (f *fake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var cmd command
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil || r.URL.Path != "/v1" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	f.mu.Lock()
	f.calls = append(f.calls, strings.TrimSpace(cmd.Cmd+" "+cmd.Session))
	block := f.block
	f.mu.Unlock()
	answer := func(status int, a map[string]any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(a)
	}
	fail := func(msg string) {
		answer(http.StatusInternalServerError, map[string]any{"status": "error", "message": msg})
	}
	switch cmd.Cmd {
	case "sessions.create":
		f.mu.Lock()
		_, exists := f.sessions[cmd.Session]
		if !exists {
			f.sessions[cmd.Session] = cmd.Proxy
		}
		f.mu.Unlock()
		if exists {
			fail("Session already exists.")
			return
		}
		answer(http.StatusOK, map[string]any{"status": "ok", "session": cmd.Session})
	case "sessions.destroy":
		f.mu.Lock()
		_, exists := f.sessions[cmd.Session]
		delete(f.sessions, cmd.Session)
		f.mu.Unlock()
		if !exists {
			fail("The session doesn't exist.")
			return
		}
		answer(http.StatusOK, map[string]any{"status": "ok"})
	case "sessions.list":
		f.mu.Lock()
		names := make([]string, 0, len(f.sessions))
		for n := range f.sessions {
			names = append(names, n)
		}
		f.mu.Unlock()
		answer(http.StatusOK, map[string]any{"status": "ok", "sessions": names})
	case "request.get", "request.post":
		if block != nil {
			<-block
		}
		f.mu.Lock()
		f.gets = append(f.gets, cmd)
		_, exists := f.sessions[cmd.Session]
		if !exists && f.autoCreate {
			f.sessions[cmd.Session], exists = nil, true
		}
		msg, noCookies := f.fail, f.noCookies
		f.mu.Unlock()
		switch {
		case !exists:
			fail("This session does not exist.")
		case msg != "":
			fail(msg)
		default:
			sol := map[string]any{"url": cmd.URL, "status": 200, "userAgent": "Mozilla/5.0 Chrome", "cookies": []any{}}
			if !noCookies {
				sol["cookies"] = []map[string]any{{"name": "cf_clearance", "value": "v", "domain": ".tracker.example", "path": "/", "httpOnly": true}}
			}
			answer(http.StatusOK, map[string]any{"status": "ok", "message": "Challenge solved!", "solution": sol})
		}
	default:
		fail("unknown command")
	}
}

func (f *fake) took() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := f.calls
	f.calls = nil
	return out
}

func newSolver(f *fake, o Options) *Solver {
	return New(&Client{URL: "http://flaresolverr:8191", HTTP: &http.Client{Transport: f}}, o)
}

var proxy = &Proxy{URL: "http://proxy.example:3128", Username: "u", Password: "p"}

func TestSolve(t *testing.T) {
	f := newFake()
	s := newSolver(f, Options{})
	ctx := t.Context()
	sol, err := s.Solve(ctx, Job{URL: "https://tracker.example/login", Post: true, PostData: "a=b", Proxy: proxy, Timeout: time.Minute, DisableMedia: true})
	if err != nil {
		t.Fatal(err)
	}
	if sol.UserAgent != "Mozilla/5.0 Chrome" || len(sol.Cookies) != 1 || sol.Cookies[0].Name != "cf_clearance" {
		t.Errorf("Solve = %+v, want cf_clearance and the user agent", sol)
	}
	name := SessionName(proxy)
	if got, want := f.took(), []string{"sessions.create " + name, "request.post " + name}; !slices.Equal(got, want) {
		t.Errorf("calls = %q, want = %q", got, want)
	}
	if got := f.sessions[name]; got == nil || *got != *proxy {
		t.Errorf("session proxy = %+v, want = %+v", got, proxy)
	}
	if g := f.gets[0]; !g.ReturnOnlyCookies || !g.DisableMedia || g.MaxTimeout != 60000 || g.PostData != "a=b" || g.Proxy != nil {
		t.Errorf("request = %+v, want only cookies, without media, in a minute, the body and no proxy of its own", g)
	}

	// Another host reuses the session, checking FlareSolverr still has it.
	if _, err := s.Solve(ctx, Job{URL: "https://other.example/", Proxy: proxy}); err != nil {
		t.Fatal(err)
	}
	if got, want := f.took(), []string{"sessions.list", "request.get " + name}; !slices.Equal(got, want) {
		t.Errorf("calls = %q, want = %q", got, want)
	}

	// FlareSolverr started again and lost the session: it is made again
	// with the proxy, not left for FlareSolverr to make without.
	f.mu.Lock()
	clear(f.sessions)
	f.autoCreate = true
	f.mu.Unlock()
	if _, err := s.Solve(ctx, Job{URL: "https://third.example/", Proxy: proxy}); err != nil {
		t.Fatal(err)
	}
	if got, want := f.took(), []string{"sessions.list", "sessions.create " + name, "request.get " + name}; !slices.Equal(got, want) {
		t.Errorf("calls = %q, want = %q", got, want)
	}
	if got := f.sessions[name]; got == nil || *got != *proxy {
		t.Errorf("session proxy = %+v, want = %+v", got, proxy)
	}

	// Without a proxy, the direct session.
	if _, err := s.Solve(ctx, Job{URL: "https://tracker.example/"}); err != nil {
		t.Fatal(err)
	}
	if got := f.took(); !slices.Contains(got, "sessions.create mavio-direct") {
		t.Errorf("calls = %q, want the direct session created", got)
	}
}

func TestSolveErrors(t *testing.T) {
	ctx := t.Context()
	f := newFake()
	f.fail = "Error: Error solving the challenge. Timeout after 60.0 seconds."
	s := newSolver(f, Options{})
	_, err := s.Solve(ctx, Job{URL: "https://tracker.example/"})
	if fe := (*Error)(nil); !errors.As(err, &fe) || !strings.Contains(fe.Message, "Timeout after 60.0 seconds") {
		t.Errorf("err = %v, want FlareSolverr's message", err)
	}

	f = newFake()
	f.noCookies = true
	s = newSolver(f, Options{})
	if _, err := s.Solve(ctx, Job{URL: "https://tracker.example/"}); err == nil || !strings.Contains(err.Error(), "no cookies") {
		t.Errorf("err = %v, want no cookies", err)
	}

	// A session lost between the check and the request is made again once.
	f = newFake()
	s = newSolver(f, Options{})
	if _, err := s.Solve(ctx, Job{URL: "https://tracker.example/"}); err != nil {
		t.Fatal(err)
	}
	f.took()
	f.fail = "This session does not exist."
	if _, err := s.Solve(ctx, Job{URL: "https://other.example/"}); err == nil {
		t.Error("err = nil, want the session error after one retry")
	}
	if got, want := f.took(), []string{
		"sessions.list", "request.get mavio-direct",
		"sessions.create mavio-direct", "sessions.destroy mavio-direct", "sessions.create mavio-direct", "request.get mavio-direct",
	}; !slices.Equal(got, want) {
		t.Errorf("calls = %q, want = %q", got, want)
	}

	if _, err := s.Solve(ctx, Job{URL: "not a url"}); err == nil {
		t.Error("err = nil, want an invalid URL")
	}
}

func TestCoalesce(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newFake()
		f.block = make(chan struct{})
		s := newSolver(f, Options{})
		var wg sync.WaitGroup
		results := make([]error, 3)
		for i := range results {
			wg.Go(func() {
				_, results[i] = s.Solve(t.Context(), Job{URL: "https://tracker.example/" + string(rune('a'+i))})
			})
		}
		synctest.Wait()
		close(f.block)
		wg.Wait()
		for i, err := range results {
			if err != nil {
				t.Errorf("Solve %d: %v", i, err)
			}
		}
		if len(f.gets) != 1 {
			t.Errorf("requests = %d, want = 1 for concurrent jobs of a host", len(f.gets))
		}

		// Within Reuse the solution answers; after, it is solved again.
		f.block = nil
		time.Sleep(20 * time.Second)
		if _, err := s.Solve(t.Context(), Job{URL: "https://tracker.example/"}); err != nil {
			t.Fatal(err)
		}
		if len(f.gets) != 1 {
			t.Errorf("requests = %d, want = 1 within 30 seconds", len(f.gets))
		}
		time.Sleep(11 * time.Second)
		if _, err := s.Solve(t.Context(), Job{URL: "https://tracker.example/"}); err != nil {
			t.Fatal(err)
		}
		if len(f.gets) != 2 {
			t.Errorf("requests = %d, want = 2 after 30 seconds", len(f.gets))
		}
	})
}

func TestConcurrencyLimits(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newFake()
		f.block = make(chan struct{})
		s := newSolver(f, Options{MaxConcurrent: 2})
		var wg sync.WaitGroup
		// Two hosts through one proxy share one session, a third host
		// through another waits for the cap.
		for _, j := range []Job{
			{URL: "https://a.example/", Proxy: proxy},
			{URL: "https://b.example/", Proxy: proxy},
			{URL: "https://c.example/"},
			{URL: "https://d.example/", Proxy: &Proxy{URL: "socks5://other:1080"}},
		} {
			wg.Go(func() {
				if _, err := s.Solve(t.Context(), j); err != nil {
					t.Error(err)
				}
			})
		}
		synctest.Wait()
		f.mu.Lock()
		var waiting int
		for _, c := range f.calls {
			if strings.HasPrefix(c, "request.") {
				waiting++
			}
		}
		f.mu.Unlock()
		if waiting != 2 {
			t.Errorf("requests at once = %d, want = 2", waiting)
		}
		close(f.block)
		wg.Wait()
		if len(f.gets) != 4 {
			t.Errorf("requests = %d, want = 4", len(f.gets))
		}
	})
}

func TestRotateAndSweep(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newFake()
		f.sessions["mavio-left"] = nil
		f.sessions["someone-else"] = nil
		s := newSolver(f, Options{TTL: 30 * time.Minute, Idle: 10 * time.Minute})
		ctx := t.Context()
		if _, err := s.Solve(ctx, Job{URL: "https://tracker.example/"}); err != nil {
			t.Fatal(err)
		}
		if err := s.Sweep(ctx); err != nil {
			t.Fatal(err)
		}
		if _, ok := f.sessions["mavio-left"]; ok {
			t.Error("a session left from before survived Sweep")
		}
		if _, ok := f.sessions["someone-else"]; !ok {
			t.Error("Sweep destroyed a session of another prefix")
		}
		if _, ok := f.sessions["mavio-direct"]; !ok {
			t.Error("Sweep destroyed a session in use")
		}

		time.Sleep(9 * time.Minute)
		if _, err := s.Solve(ctx, Job{URL: "https://other.example/"}); err != nil {
			t.Fatal(err)
		}
		time.Sleep(9 * time.Minute)
		_ = s.Sweep(ctx)
		if _, ok := f.sessions["mavio-direct"]; !ok {
			t.Error("Sweep destroyed a session used 9 minutes ago")
		}

		// Past the TTL it is rotated.
		time.Sleep(13 * time.Minute)
		f.took()
		if _, err := s.Solve(ctx, Job{URL: "https://third.example/"}); err != nil {
			t.Fatal(err)
		}
		if got, want := f.took(), []string{"sessions.destroy mavio-direct", "sessions.create mavio-direct", "request.get mavio-direct"}; !slices.Equal(got, want) {
			t.Errorf("calls = %q, want = %q", got, want)
		}

		time.Sleep(11 * time.Minute)
		_ = s.Sweep(ctx)
		if _, ok := f.sessions["mavio-direct"]; ok {
			t.Error("a session idle for 11 minutes survived Sweep")
		}

		if _, err := s.Solve(ctx, Job{URL: "https://fourth.example/"}); err != nil {
			t.Fatal(err)
		}
		if err := s.Close(context.Background()); err != nil {
			t.Fatal(err)
		}
		if len(f.sessions) != 1 {
			t.Errorf("sessions after Close = %v, want = someone-else's", f.sessions)
		}
	})
}

func TestSessionName(t *testing.T) {
	a := SessionName(&Proxy{URL: "http://h:1", Username: "u", Password: "p"})
	b := SessionName(&Proxy{URL: "http://h:1", Username: "u", Password: "q"})
	if a == b || !strings.HasPrefix(a, SessionPrefix) || len(a) != len(SessionPrefix)+16 {
		t.Errorf("SessionName = %q and %q, want distinct names of the prefix", a, b)
	}
	if got := SessionName(nil); got != "mavio-direct" {
		t.Errorf("SessionName(nil) = %q, want = mavio-direct", got)
	}
}
