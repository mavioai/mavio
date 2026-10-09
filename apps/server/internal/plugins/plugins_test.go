package plugins_test

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/mavioai/mavio/apps/server/internal/plugins"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
	"github.com/mavioai/mavio/libs/store"
)

// wasmPlugin is the smoke tests' metadata provider built for WASM.
var wasmPlugin string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "mavio-plugins-")
	if err != nil {
		panic(err)
	}
	wasmPlugin = filepath.Join(dir, "plugin.wasm")
	build := exec.Command("go", "build", "-buildmode=c-shared", "-o", wasmPlugin,
		"github.com/mavioai/mavio/apps/server/internal/smoke/smokeplugin")
	build.Env = append(os.Environ(), "GOOS=wasip1", "GOARCH=wasm")
	if out, err := build.CombinedOutput(); err != nil {
		panic(fmt.Sprintf("build plugin: %v\n%s", err, out))
	}
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

// requiresKey is a configuration schema with a required API key.
const requiresKey = `{"type":"object","properties":{"api_key":{"type":"string","minLength":1}},"required":["api_key"]}`

// install lays out the smoke plugin in a folder of root with the given
// configuration schema.
func install(t *testing.T, root, folder, schema string) {
	t.Helper()
	dir := filepath.Join(root, folder)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(wasmPlugin)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugin.wasm"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := fmt.Sprintf(`{"id":"org.mavio.smoke","name":"Smoke","version":"0.1.0","runtime":"RUNTIME_WASM",
		"capabilities":["CAPABILITY_METADATA_PROVIDER"],"configSchema":%q,"apiVersion":"1.0"}`, schema)
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
}

func open(t *testing.T, dir string, s core.Store) *plugins.Manager {
	t.Helper()
	m, err := plugins.Open(t.Context(), plugins.Config{Dir: dir, CacheDir: t.TempDir(), Store: s})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Close(t.Context()) })
	return m
}

func state(t *testing.T, m *plugins.Manager, id string) plugins.Info {
	t.Helper()
	for _, p := range m.Plugins() {
		if p.Manifest.GetId() == id {
			return p
		}
	}
	t.Fatalf("plugin %s not listed", id)
	return plugins.Info{}
}

// concubine is the film the smoke plugin knows.
var concubine = library.Lookup{Kind: core.KindMovie, Name: "Farewell My Concubine"}

func TestManager(t *testing.T) {
	ctx := t.Context()
	s, err := store.Open(ctx, "sqlite:"+filepath.Join(t.TempDir(), "mavio.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	dir := t.TempDir()
	install(t, dir, "smoke", requiresKey)
	broken := filepath.Join(dir, "broken")
	if err := os.MkdirAll(broken, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(broken, "manifest.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := open(t, dir, s)
	list := m.Plugins()
	if len(list) != 2 || list[0].Manifest.GetId() != "broken" || list[0].State != plugins.Failed || list[0].Err == nil {
		t.Fatalf("Plugins() = %+v, want the broken folder failed, by its name", list)
	}
	if got := state(t, m, "org.mavio.smoke"); got.State != plugins.Unconfigured {
		t.Errorf("before configuration: state = %v, want Unconfigured", got.State)
	}
	providers := m.MetadataProviders()
	if len(providers) != 1 {
		t.Fatalf("MetadataProviders() = %d, want 1", len(providers))
	}
	if res, err := providers[0].Metadata(ctx, concubine); res != nil || err != nil {
		t.Errorf("unconfigured provider: Metadata = %v, %v; want nothing", res, err)
	}

	if _, err := m.SetConfig(ctx, "org.mavio.smoke", `{"api_key":""}`); !errors.Is(err, core.ErrInvalid) {
		t.Errorf("SetConfig against the schema: %v, want ErrInvalid", err)
	}
	if _, err := m.SetConfig(ctx, "org.example.missing", `{}`); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("SetConfig of an unknown plugin: %v, want ErrNotFound", err)
	}
	info, err := m.SetConfig(ctx, "org.mavio.smoke", `{"api_key":"secret"}`)
	if err != nil || info.State != plugins.Ready {
		t.Fatalf("SetConfig = %+v, %v; want Ready", info, err)
	}
	// The provider handed out before takes part now.
	if res, err := providers[0].Metadata(ctx, concubine); err != nil || res == nil {
		t.Errorf("configured provider: Metadata = %v, %v; want the film", res, err)
	}
	if c, err := m.Config(ctx, "org.mavio.smoke"); err != nil || c.JSON != `{"api_key":"secret"}` {
		t.Errorf("Config = %+v, %v", c, err)
	}
	if err := m.Close(ctx); err != nil {
		t.Fatal(err)
	}

	// The stored configuration is delivered when the server starts again.
	if got := state(t, open(t, dir, s), "org.mavio.smoke"); got.State != plugins.Ready {
		t.Errorf("after a restart: state = %v, %v; want Ready", got.State, got.Err)
	}
	// An upgrade whose schema the stored configuration no longer fits.
	install(t, dir, "smoke", `{"type":"object","properties":{"token":{"type":"string"}},"required":["token"]}`)
	if got := state(t, open(t, dir, s), "org.mavio.smoke"); got.State != plugins.Failed || got.Err == nil {
		t.Errorf("stale configuration: state = %v, %v; want Failed", got.State, got.Err)
	}
}

func TestManagerWithoutPlugins(t *testing.T) {
	m, err := plugins.Open(t.Context(), plugins.Config{})
	if err != nil || len(m.Plugins()) != 0 || len(m.MetadataProviders()) != 0 {
		t.Errorf("Open without a folder = %v, %v", m.Plugins(), err)
	}
	if _, err := m.Config(t.Context(), "org.mavio.smoke"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("Config: %v, want ErrNotFound", err)
	}
}
