package capi_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/client-rust/capi"
	capiv1 "github.com/mavioai/mavio/libs/client-rust/capi/gen/mavio/capi/v1"
)

// The core, loaded once for the tests as an app loads it, since its
// callback is never freed; nil, with why, when there is none.
var (
	lib     *capi.Library
	missing string
)

// TestMain loads the core that cargo built into ../target/debug, or the one
// at $MAVIO_CORE_LIB.
func TestMain(m *testing.M) {
	path := os.Getenv("MAVIO_CORE_LIB")
	if path == "" {
		name := map[string]string{"darwin": "libmaviocore.dylib", "windows": "maviocore.dll"}[runtime.GOOS]
		if name == "" {
			name = "libmaviocore.so"
		}
		path = filepath.Join("..", "target", "debug", name)
	}
	if _, err := os.Stat(path); err != nil {
		missing = fmt.Sprintf("no core at %s (cargo build -p mavio-capi): %v", path, err)
	} else if lib, err = capi.Load(path); err != nil {
		fmt.Fprintf(os.Stderr, "Load(%s) error = %v\n", path, err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func device() *capiv1.Device {
	return capiv1.Device_builder{
		Id: "capi-test", Name: "capi test", Client: "Mavio capi test", ClientVersion: "0.0.0",
	}.Build()
}

// events collects a client's events.
type events struct {
	mu   sync.Mutex
	list []*capiv1.Event
}

func (e *events) add(ev *capiv1.Event) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.list = append(e.list, ev)
}

// wait waits for an event matching f.
func (e *events) wait(t *testing.T, what string, f func(*capiv1.Event) bool) *capiv1.Event {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		e.mu.Lock()
		for _, ev := range e.list {
			if f(ev) {
				e.mu.Unlock()
				return ev
			}
		}
		e.mu.Unlock()
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
	return nil
}

func open(t *testing.T, tokens map[string]string, ev *events) *capi.Client {
	t.Helper()
	o := capiv1.Open_builder{
		DatabasePath: filepath.Join(t.TempDir(), "core.db"),
		Device:       device(),
		Tokens:       tokens,
	}.Build()
	if lib == nil {
		t.Skip(missing)
	}
	c, err := lib.Open(t.Context(), o, ev.add)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(c.Close)
	return c
}

func TestCalls(t *testing.T) {
	c := open(t, nil, &events{})
	version := capiv1.Request_builder{GetVersion: &capiv1.GetVersion{}}.Build()

	tests := []struct {
		name     string
		request  *capiv1.Request
		check    func(*capiv1.Message) bool
		wantCode capiv1.Error_Code
	}{
		{"version", version, func(m *capiv1.Message) bool { return m.GetVersion() == "0.0.0" }, 0},
		{
			"no servers", capiv1.Request_builder{ListServers: &capiv1.ListServers{}}.Build(),
			func(m *capiv1.Message) bool { return m.HasServers() && len(m.GetServers().GetServers()) == 0 }, 0,
		},
		{
			"start without servers", capiv1.Request_builder{Start: &capiv1.Start{}}.Build(),
			func(m *capiv1.Message) bool { return m.HasDone() }, 0,
		},
		{"unreachable server", capiv1.Request_builder{
			Probe: capiv1.Probe_builder{Address: "http://127.0.0.1:1"}.Build(),
		}.Build(), nil, capiv1.Error_CODE_UNREACHABLE},
		{"empty request", &capiv1.Request{}, nil, capiv1.Error_CODE_INVALID},
		{"discover", capiv1.Request_builder{
			Discover: capiv1.Discover_builder{WaitMs: 100}.Build(),
		}.Build(), func(m *capiv1.Message) bool { return m.HasFoundServers() }, 0},
		{"poll an unknown Quick Connect", capiv1.Request_builder{
			PollQuickConnect: capiv1.PollQuickConnect_builder{Request: 7}.Build(),
		}.Build(), nil, capiv1.Error_CODE_NOT_FOUND},
		{"cancel an unknown Quick Connect", capiv1.Request_builder{
			CancelQuickConnect: capiv1.CancelQuickConnect_builder{Request: 7}.Build(),
		}.Build(), func(m *capiv1.Message) bool { return m.HasDone() }, 0},
		{"update an unknown playback", capiv1.Request_builder{
			UpdatePlayback: capiv1.UpdatePlayback_builder{Session: 7, PositionMs: 1000}.Build(),
		}.Build(), nil, capiv1.Error_CODE_NOT_FOUND},
		{"stop an unknown playback", capiv1.Request_builder{
			StopPlayback: capiv1.StopPlayback_builder{Session: 7}.Build(),
		}.Build(), nil, capiv1.Error_CODE_NOT_FOUND},
		{"play with invalid capabilities", capiv1.Request_builder{
			Play: capiv1.Play_builder{ServerId: "s", ItemId: "i", Capabilities: []byte{0xff}}.Build(),
		}.Build(), nil, capiv1.Error_CODE_INVALID},
		{"opened twice", capiv1.Request_builder{Open: capiv1.Open_builder{
			DatabasePath: filepath.Join(t.TempDir(), "core.db"),
		}.Build()}.Build(), nil, capiv1.Error_CODE_FAILED_PRECONDITION},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := c.Call(t.Context(), tt.request)
			if tt.wantCode != 0 {
				var e *capi.Error
				if !errors.As(err, &e) || e.Code != tt.wantCode {
					t.Fatalf("Call() error = %v, want code %v", err, tt.wantCode)
				}
				return
			}
			if err != nil || !tt.check(m) {
				t.Fatalf("Call() = %v, %v", m, err)
			}
		})
	}
}

// Answers come back on the core's threads, many at once.
func TestConcurrentCalls(t *testing.T) {
	c := open(t, nil, &events{})
	var wg sync.WaitGroup
	errs := make(chan error, 200)
	for range 200 {
		wg.Go(func() {
			m, err := c.Call(t.Context(), capiv1.Request_builder{GetVersion: &capiv1.GetVersion{}}.Build())
			if err == nil && m.GetVersion() != "0.0.0" {
				err = errors.New("got version " + m.GetVersion())
			}
			if err != nil {
				errs <- err
			}
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("Call() error = %v", err)
	}
}

// Home is answered in parts, the rows each time a server answers, then
// Done.
func TestStream(t *testing.T) {
	c := open(t, nil, &events{})
	parts := 0
	m, err := c.Stream(t.Context(), capiv1.Request_builder{LoadHome: &capiv1.LoadHome{}}.Build(),
		func(m *capiv1.Message) {
			if !m.HasRows() {
				t.Errorf("part = %v, want rows", m)
			}
			parts++
		})
	if err != nil || !m.HasDone() {
		t.Fatalf("Stream() = %v, %v, want Done", m, err)
	}
	t.Logf("%d parts", parts)
}

func TestClosed(t *testing.T) {
	c := open(t, nil, &events{})
	c.Close()
	_, err := c.Call(context.Background(), capiv1.Request_builder{GetVersion: &capiv1.GetVersion{}}.Build())
	if !errors.Is(err, capi.ErrClosed) {
		t.Fatalf("Call() error = %v, want ErrClosed", err)
	}
}

// Signs in to the server at $MAVIO_TEST_SERVER as $MAVIO_TEST_USER with
// $MAVIO_TEST_PASSWORD and follows its event stream; skipped without them.
func TestServer(t *testing.T) {
	address, user := os.Getenv("MAVIO_TEST_SERVER"), os.Getenv("MAVIO_TEST_USER")
	if address == "" || user == "" {
		t.Skip("no MAVIO_TEST_SERVER and MAVIO_TEST_USER")
	}
	ev := &events{}
	c := open(t, nil, ev)
	m, err := c.Call(t.Context(), capiv1.Request_builder{SignIn: capiv1.SignIn_builder{
		Address: address, Name: user, Password: os.Getenv("MAVIO_TEST_PASSWORD"),
	}.Build()}.Build())
	if err != nil {
		t.Fatalf("SignIn error = %v", err)
	}
	server := m.GetServer()
	if server.GetId() == "" || server.GetUserName() != user {
		t.Fatalf("SignIn = %v, want a server signed in as %s", server, user)
	}
	ev.wait(t, "the token", func(e *capiv1.Event) bool {
		return e.GetServerId() == server.GetId() && e.GetTokenChanged().GetToken() != ""
	})
	if _, err := c.Call(t.Context(), capiv1.Request_builder{Start: &capiv1.Start{}}.Build()); err != nil {
		t.Fatalf("Start error = %v", err)
	}
	ev.wait(t, "the stream online", func(e *capiv1.Event) bool {
		return e.GetServerId() == server.GetId() && e.GetState().GetState() == capiv1.StreamState_STATE_ONLINE
	})
	ev.wait(t, "Connected", func(e *capiv1.Event) bool {
		return e.GetServerId() == server.GetId() && e.HasConnected()
	})
	m, err = c.Call(t.Context(), capiv1.Request_builder{ListServers: &capiv1.ListServers{}}.Build())
	if err != nil || len(m.GetServers().GetServers()) != 1 {
		t.Fatalf("ListServers = %v, %v, want the server", m, err)
	}
	var rows *capiv1.Rows
	_, err = c.Stream(t.Context(), capiv1.Request_builder{LoadHome: &capiv1.LoadHome{}}.Build(),
		func(m *capiv1.Message) { rows = m.GetRows() })
	if err != nil || len(rows.GetRows()) == 0 {
		t.Fatalf("LoadHome = %v, %v, want rows", rows, err)
	}
	m, err = c.Call(t.Context(), capiv1.Request_builder{
		ListLibraries: capiv1.ListLibraries_builder{ServerId: server.GetId()}.Build(),
	}.Build())
	if err != nil || !m.HasLibraries() {
		t.Fatalf("ListLibraries = %v, %v", m, err)
	}
}
