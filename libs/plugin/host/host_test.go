package host_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	"github.com/mavioai/mavio/libs/plugin/host"
	"github.com/mavioai/mavio/libs/plugin/host/process"
	"github.com/mavioai/mavio/libs/plugin/host/wasm"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

var (
	wasmBin, nativeBin string
	buildErr           error
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "mavio-plugin-test-")
	if err != nil {
		panic(err)
	}
	wasmBin = filepath.Join(dir, "plugin.wasm")
	nativeBin = filepath.Join(dir, "plugin")
	if runtime.GOOS == "windows" {
		nativeBin += ".exe"
	}
	pkg := "github.com/mavioai/mavio/libs/plugin/internal/testplugin"
	wasmBuild := exec.Command("go", "build", "-buildmode=c-shared", "-o", wasmBin, pkg)
	wasmBuild.Env = append(os.Environ(), "GOOS=wasip1", "GOARCH=wasm")
	if out, err := wasmBuild.CombinedOutput(); err != nil {
		buildErr = fmt.Errorf("build wasm test plugin: %w\n%s", err, out)
	} else if out, err := exec.Command("go", "build", "-o", nativeBin, pkg).CombinedOutput(); err != nil {
		buildErr = fmt.Errorf("build native test plugin: %w\n%s", err, out)
	}
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

const configSchema = `{"type":"object","properties":{"key":{"type":"string"}},"required":["key"]}`

// pluginDir lays out a plugin directory for the runtime with the given HTTP
// permissions.
func pluginDir(t *testing.T, rt string, hosts ...string) string {
	t.Helper()
	if buildErr != nil {
		t.Fatal(buildErr)
	}
	dir := t.TempDir()
	src, dst := nativeBin, filepath.Join(dir, filepath.Base(nativeBin))
	runtimeName := "RUNTIME_PROCESS"
	if rt == "wasm" {
		src, dst, runtimeName = wasmBin, filepath.Join(dir, "plugin.wasm"), "RUNTIME_WASM"
	}
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, data, 0o755); err != nil {
		t.Fatal(err)
	}
	quoted := make([]string, len(hosts))
	for i, h := range hosts {
		quoted[i] = fmt.Sprintf("%q", h)
	}
	manifest := fmt.Sprintf(`{"id":"org.mavio.testplugin","name":"Test","version":"0.1.0","runtime":%q,
		"capabilities":["CAPABILITY_METADATA_PROVIDER","CAPABILITY_TASK_RUNNER","CAPABILITY_EVENT_CONSUMER","CAPABILITY_HTTP_HANDLER"],"apiVersion":"1.0","configPage":"echo",
		"tasks":[{"id":"write","name":"Write"},{"id":"slow","name":"Slow","timeout":"5s"}],
		"permissions":{"httpHosts":[%s],"events":["item.*"]},"configSchema":%q}`, runtimeName, strings.Join(quoted, ","), configSchema)
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

var (
	cacheOnce sync.Once
	cacheDir  string
)

func open(t *testing.T, rt string, hosts ...string) host.Plugin {
	t.Helper()
	return openWithTimeout(t, rt, 10*time.Second, hosts...)
}

func openWithTimeout(t *testing.T, rt string, callTimeout time.Duration, hosts ...string) host.Plugin {
	t.Helper()
	return openWith(t, rt, func(o *host.Options) { o.WASM.CallTimeout = callTimeout }, hosts...)
}

func openWith(t *testing.T, rt string, configure func(*host.Options), hosts ...string) host.Plugin {
	t.Helper()
	cacheOnce.Do(func() { cacheDir, _ = os.MkdirTemp("", "mavio-wasm-cache-") })
	opts := host.Options{
		WASM:    wasm.Options{CacheDir: cacheDir, CallTimeout: 10 * time.Second, Instances: 3, Logger: slog.New(slog.DiscardHandler)},
		Process: process.Options{HealthInterval: -1, Logger: slog.New(slog.DiscardHandler)},
	}
	if configure != nil {
		configure(&opts)
	}
	p, err := host.Open(t.Context(), pluginDir(t, rt, hosts...), opts)
	if err != nil {
		t.Fatalf("open %s plugin: %v", rt, err)
	}
	t.Cleanup(func() { _ = p.Close(context.Background()) })
	return p
}

func search(ctx context.Context, p host.Plugin, name string) (string, error) {
	req := pluginv1.SearchRequest_builder{Lookup: pluginv1.Lookup_builder{Name: proto.String(name)}.Build()}.Build()
	resp, err := p.Metadata().Search(ctx, req)
	if err != nil {
		return "", err
	}
	return resp.GetResults()[0].GetOverview(), nil
}

func eachRuntime(t *testing.T, fn func(t *testing.T, rt string)) {
	for _, rt := range []string{"wasm", "process"} {
		t.Run(rt, func(t *testing.T) {
			t.Parallel()
			fn(t, rt)
		})
	}
}

func TestCallsAndConfiguration(t *testing.T) {
	eachRuntime(t, func(t *testing.T, rt string) {
		ctx := t.Context()
		p := open(t, rt)
		if p.Auth() != nil || p.Notifier() != nil || p.Metadata() == nil {
			t.Error("capability clients do not follow the manifest")
		}
		if got, err := search(ctx, p, "hello"); err != nil || got != "hello" {
			t.Fatalf("search = %q, %v", got, err)
		}

		if err := host.Configure(ctx, p, `{"nope":1}`); err == nil {
			t.Error("configuration violating the schema was accepted")
		}
		if err := host.Configure(ctx, p, `{"key":"v1"}`); err != nil {
			t.Fatal(err)
		}
		if err := host.Configure(ctx, p, `{"key":"reject"}`); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("plugin rejection = %v", err)
		}
		// Every instance sees the accepted configuration.
		var wg sync.WaitGroup
		for range 8 {
			wg.Go(func() {
				if got, err := search(ctx, p, "config"); err != nil || got != `{"key":"v1"}` {
					t.Errorf("config seen by plugin = %q, %v", got, err)
				}
			})
		}
		wg.Wait()
	})
}

func TestFetchPermissions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "fetched "+r.URL.Path)
	}))
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)

	p := open(t, "wasm", u.Hostname())
	ctx := t.Context()
	if got, err := search(ctx, p, "fetch:"+srv.URL+"/a"); err != nil || got != "fetched /a" {
		t.Errorf("allowed fetch = %q, %v", got, err)
	}
	denied := strings.Replace(srv.URL, u.Hostname(), "localhost", 1)
	if _, err := search(ctx, p, "fetch:"+denied+"/b"); connect.CodeOf(err) != connect.CodePermissionDenied || !strings.Contains(err.Error(), "not in the plugin's permissions") {
		t.Errorf("denied fetch error = %v", err)
	}
}

func TestHostAPI(t *testing.T) {
	eachRuntime(t, func(t *testing.T, rt string) {
		var p host.Plugin
		hostAPI := func(m *pluginv1.Manifest) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/reenter" {
					// The calling plugin is busy in this request; the host
					// may still call it.
					got, err := search(r.Context(), p, "inner")
					fmt.Fprintf(w, "%s %v", got, err)
					return
				}
				w.WriteHeader(http.StatusTeapot)
				fmt.Fprintf(w, "%s %s %s auth=%q", m.GetId(), r.Method, r.URL.RequestURI(), r.Header.Get("Authorization"))
			})
		}
		p = openWith(t, rt, func(o *host.Options) { o.HostAPI = hostAPI })
		ctx := t.Context()
		want := `418 org.mavio.testplugin GET /a/b?c=d auth=""`
		if got, err := search(ctx, p, "host:/a/b?c=d"); err != nil || got != want {
			t.Errorf("host call = %q, %v; want %q", got, err, want)
		}
		if got, err := search(ctx, p, "host:/reenter"); err != nil || got != "200 inner <nil>" {
			t.Errorf("reentrant host call = %q, %v", got, err)
		}
	})
}

func TestTasksAndDataFolder(t *testing.T) {
	eachRuntime(t, func(t *testing.T, rt string) {
		data := t.TempDir()
		p := openWith(t, rt, func(o *host.Options) { o.DataDir, o.WASM.CallTimeout = data, time.Second })
		ctx := t.Context()
		run := func(ctx context.Context, id string) (string, error) {
			resp, err := p.Tasks().RunTask(ctx, pluginv1.RunTaskRequest_builder{TaskId: proto.String(id)}.Build())
			return resp.GetMessage(), err
		}
		msg, err := run(ctx, "write")
		if err != nil {
			t.Fatalf("write task: %v", err)
		}
		if !strings.HasSuffix(msg, ": kept") {
			t.Errorf("write task = %q", msg)
		}
		if got, err := os.ReadFile(filepath.Join(host.DataDir(data, "org.mavio.testplugin"), "note.txt")); err != nil || string(got) != "kept" {
			t.Errorf("file in the data folder = %q, %v", got, err)
		}

		// A task's timeout replaces the WASM call timeout.
		if rt == "wasm" {
			if _, err := run(ctx, "slow"); err == nil {
				t.Error("slow task within the call timeout succeeded")
			}
		}
		if msg, err := run(host.WithCallTimeout(ctx, 5*time.Second), "slow"); err != nil || msg != "slept" {
			t.Errorf("slow task with its timeout = %q, %v", msg, err)
		}
	})
}

func TestEvents(t *testing.T) {
	eachRuntime(t, func(t *testing.T, rt string) {
		p := openWith(t, rt, func(o *host.Options) { o.DataDir = t.TempDir() })
		ctx := t.Context()
		events := []*pluginv1.Event{
			pluginv1.Event_builder{Id: proto.String("1"), Type: proto.String("item.added")}.Build(),
			pluginv1.Event_builder{Id: proto.String("2"), Type: proto.String("item.removed")}.Build(),
		}
		if _, err := p.Events().Consume(ctx, pluginv1.ConsumeRequest_builder{Events: events}.Build()); err != nil {
			t.Fatalf("Consume() = %v", err)
		}
		if got, err := search(ctx, p, "events"); err != nil || got != "item.added item.removed" {
			t.Errorf("consumed events = %q, %v, want item.added item.removed", got, err)
		}
	})
}

func TestHTTPRoutes(t *testing.T) {
	eachRuntime(t, func(t *testing.T, rt string) {
		p := open(t, rt)
		do := func(method, target, auth, body string) *httptest.ResponseRecorder {
			req := httptest.NewRequestWithContext(t.Context(), method, target, strings.NewReader(body))
			if auth != "" {
				req.Header.Set("Authorization", auth)
			}
			w := httptest.NewRecorder()
			p.HTTP().ServeHTTP(w, req)
			return w
		}
		// Routes see the method, path, query, the client's Authorization
		// and body, never the host's token.
		w := do(http.MethodPut, "/echo?a=1", "Basic eDp5", "hello")
		want := `PUT /echo a=1 auth="Basic eDp5" token="" body=hello`
		if w.Code != http.StatusOK || w.Body.String() != want {
			t.Errorf("PUT /echo = %d %q, want %q", w.Code, w.Body, want)
		}
		if w := do(http.MethodGet, "/missing", "", ""); w.Code != http.StatusNotFound {
			t.Errorf("GET /missing = %d, want 404", w.Code)
		}
		// WASM bodies are bounded.
		if rt == "wasm" {
			if w := do(http.MethodGet, "/big", "", ""); w.Code != http.StatusBadGateway {
				t.Errorf("GET /big = %d, want 502", w.Code)
			}
			if w := do(http.MethodPost, "/echo", "", strings.Repeat("x", 17<<20)); w.Code != http.StatusRequestEntityTooLarge {
				t.Errorf("POST of 17 MiB = %d, want 413", w.Code)
			}
		} else if w := do(http.MethodGet, "/big", "", ""); w.Code != http.StatusOK || w.Body.Len() != 17<<20 {
			t.Errorf("GET /big = %d with %d bytes, want 17 MiB", w.Code, w.Body.Len())
		}
	})
}

func TestHostAPIUnavailable(t *testing.T) {
	eachRuntime(t, func(t *testing.T, rt string) {
		p := open(t, rt)
		_, err := search(t.Context(), p, "host:/a")
		if connect.CodeOf(err) != connect.CodeUnavailable {
			t.Errorf("host call without a host API = %v, want unavailable", err)
		}
	})
}

func TestWASMFailuresAreIsolated(t *testing.T) {
	ctx := t.Context()
	// "slow" sleeps for two seconds, exceeding this timeout.
	p := openWithTimeout(t, "wasm", time.Second)
	if err := host.Configure(ctx, p, `{"key":"kept"}`); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"panic", "crash", "slow"} {
		if _, err := search(ctx, p, name); err == nil {
			t.Errorf("%s: call succeeded, want an error", name)
		}
		// The broken instance is replaced; later calls work and keep the
		// configuration.
		for range 4 {
			if got, err := search(ctx, p, "config"); err != nil || got != `{"key":"kept"}` {
				t.Errorf("after %s: search = %q, %v", name, got, err)
			}
		}
	}
}

func TestProcessRestartsAfterCrash(t *testing.T) {
	ctx := t.Context()
	p := open(t, "process")
	if _, err := search(ctx, p, "crash"); err == nil {
		t.Fatal("crashing call succeeded")
	}
	deadline := time.Now().Add(30 * time.Second)
	for {
		got, err := search(ctx, p, "back")
		if err == nil && got == "back" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("plugin did not come back: %v", err)
		}
		time.Sleep(100 * time.Millisecond)
	}
	if n := p.(*process.Plugin).Restarts(); n < 1 {
		t.Errorf("Restarts() = %d, want ≥ 1", n)
	}

	if err := p.Close(ctx); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := search(ctx, p, "after close"); connect.CodeOf(err) != connect.CodeUnavailable {
		t.Errorf("call after Close = %v, want Unavailable", err)
	}
}

func TestOpenRejectsMismatchedManifest(t *testing.T) {
	for _, rt := range []string{"wasm", "process"} {
		dir := pluginDir(t, rt)
		path := filepath.Join(dir, "manifest.json")
		data, _ := os.ReadFile(path)
		_ = os.WriteFile(path, []byte(strings.Replace(string(data), `"version":"0.1.0"`, `"version":"9.9.9"`, 1)), 0o644)
		_, err := host.Open(t.Context(), dir, host.Options{
			WASM:    wasm.Options{Logger: slog.New(slog.DiscardHandler)},
			Process: process.Options{Logger: slog.New(slog.DiscardHandler)},
		})
		if err == nil || !strings.Contains(err.Error(), "9.9.9") {
			t.Errorf("%s: Open with mismatched version = %v", rt, err)
		}
	}
	if _, err := host.Open(t.Context(), t.TempDir(), host.Options{}); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Open without manifest = %v", err)
	}
}
