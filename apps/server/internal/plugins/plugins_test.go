package plugins_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"uuid"

	"google.golang.org/protobuf/proto"

	"github.com/mavioai/mavio/apps/server/internal/plugins"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/store"
)

// wasmPlugin is the smoke tests' metadata provider built for WASM.
var wasmPlugin string

// compiled holds the compiled WASM plugins, which every manager of the
// test binary shares so that each plugin compiles once.
var compiled string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "mavio-plugins-")
	if err != nil {
		panic(err)
	}
	wasmPlugin = filepath.Join(dir, "plugin.wasm")
	compiled = filepath.Join(dir, "compiled")
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
	installWith(t, root, folder, schema, `"capabilities":["CAPABILITY_METADATA_PROVIDER"]`)
}

// installWith lays out the smoke plugin with the given manifest fields
// besides its ID, name, version, runtime, schema and API version.
func installWith(t *testing.T, root, folder, schema, fields string) {
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
	manifest := fmt.Sprintf(`{"id":"dev.mavio.smoke","name":"Smoke","version":"0.1.0","runtime":"RUNTIME_WASM",
		%s,"configSchema":%q,"apiVersion":"1.0"}`, fields, schema)
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
}

func open(t *testing.T, dir string, s core.Store) *plugins.Manager {
	t.Helper()
	m, err := plugins.Open(t.Context(), plugins.Config{Dir: dir, CacheDir: compiled, DataDir: t.TempDir(), Store: s})
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
	if got := state(t, m, "dev.mavio.smoke"); got.State != plugins.Unconfigured {
		t.Errorf("before configuration: state = %v, want Unconfigured", got.State)
	}
	providers := m.MetadataProviders()
	if len(providers) != 1 {
		t.Fatalf("MetadataProviders() = %d, want 1", len(providers))
	}
	if res, err := providers[0].Metadata(ctx, concubine); res != nil || !errors.Is(err, library.ErrProviderUnavailable) {
		t.Errorf("unconfigured provider: Metadata = %v, %v; want = %v", res, err, library.ErrProviderUnavailable)
	}

	if _, _, err := m.SetConfig(ctx, "dev.mavio.smoke", `{"api_key":""}`); !errors.Is(err, core.ErrInvalid) {
		t.Errorf("SetConfig against the schema: %v, want ErrInvalid", err)
	}
	if _, _, err := m.SetConfig(ctx, "org.example.missing", `{}`); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("SetConfig of an unknown plugin: %v, want ErrNotFound", err)
	}
	info, _, err := m.SetConfig(ctx, "dev.mavio.smoke", `{"api_key":"secret"}`)
	if err != nil || info.State != plugins.Ready {
		t.Fatalf("SetConfig = %+v, %v; want Ready", info, err)
	}
	// The provider handed out before takes part now.
	if res, err := providers[0].Metadata(ctx, concubine); err != nil || res == nil {
		t.Errorf("configured provider: Metadata = %v, %v; want the film", res, err)
	}
	if c, err := m.Config(ctx, "dev.mavio.smoke"); err != nil || c.JSON != `{"api_key":"secret"}` {
		t.Errorf("Config = %+v, %v", c, err)
	}
	if err := m.Close(ctx); err != nil {
		t.Fatal(err)
	}

	// The stored configuration is delivered when the server starts again.
	if got := state(t, open(t, dir, s), "dev.mavio.smoke"); got.State != plugins.Ready {
		t.Errorf("after a restart: state = %v, %v; want Ready", got.State, got.Err)
	}
	// Or once the host API serves, when Open leaves it for later.
	later, err := plugins.Open(ctx, plugins.Config{Dir: dir, CacheDir: compiled, DataDir: t.TempDir(), Store: s, ConfigureLater: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = later.Close(t.Context()) })
	if got := state(t, later, "dev.mavio.smoke"); got.State != plugins.Starting {
		t.Errorf("configured later: state = %v, want Starting", got.State)
	}
	later.ConfigureStarted(ctx)
	if got := state(t, later, "dev.mavio.smoke"); got.State != plugins.Ready {
		t.Errorf("ConfigureStarted: state = %v, %v; want Ready", got.State, got.Err)
	}
	if err := later.Close(ctx); err != nil {
		t.Fatal(err)
	}
	// An upgrade whose schema the stored configuration no longer fits.
	install(t, dir, "smoke", `{"type":"object","properties":{"token":{"type":"string"}},"required":["token"]}`)
	if got := state(t, open(t, dir, s), "dev.mavio.smoke"); got.State != plugins.Failed || got.Err == nil {
		t.Errorf("stale configuration: state = %v, %v; want Failed", got.State, got.Err)
	}
}

func TestLocales(t *testing.T) {
	s, err := store.Open(t.Context(), "sqlite:"+filepath.Join(t.TempDir(), "mavio.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	dir := t.TempDir()
	install(t, dir, "smoke", `{"type":"object"}`)
	locales := filepath.Join(dir, "smoke", "locales")
	if err := os.MkdirAll(locales, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(locales, "zh-Hans.json"), []byte(`{"Smoke":"冒烟"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	m := open(t, dir, s)
	if got := state(t, m, "dev.mavio.smoke"); got.State != plugins.Ready || got.Locales["zh-Hans"]["Smoke"] != "冒烟" {
		t.Errorf("Plugins() = %+v, want the plugin ready with its translations", got)
	}
	if err := m.Close(t.Context()); err != nil {
		t.Fatal(err)
	}

	// Translations that cannot be read fail the plugin.
	if err := os.WriteFile(filepath.Join(locales, "zh-Hans.json"), []byte(`{"Smoke":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := state(t, open(t, dir, s), "dev.mavio.smoke"); got.State != plugins.Failed || got.Err == nil {
		t.Errorf("broken translations: state = %v, %v; want Failed", got.State, got.Err)
	}
}

func TestManagerWithoutPlugins(t *testing.T) {
	m, err := plugins.Open(t.Context(), plugins.Config{})
	if err != nil || len(m.Plugins()) != 0 || len(m.MetadataProviders()) != 0 {
		t.Errorf("Open without a folder = %v, %v", m.Plugins(), err)
	}
	if _, err := m.Config(t.Context(), "dev.mavio.smoke"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("Config: %v, want ErrNotFound", err)
	}
}

func TestAuthAndNotifiers(t *testing.T) {
	ctx := t.Context()
	root := t.TempDir()
	install(t, root, "smoke", `{"type":"object"}`)
	manifest := filepath.Join(root, "smoke", "manifest.json")
	data, _ := os.ReadFile(manifest)
	data = []byte(strings.Replace(string(data), `"CAPABILITY_METADATA_PROVIDER"`,
		`"CAPABILITY_METADATA_PROVIDER","CAPABILITY_AUTH_PROVIDER","CAPABILITY_NOTIFIER"`, 1))
	if err := os.WriteFile(manifest, data, 0o644); err != nil {
		t.Fatal(err)
	}
	db, err := store.Open(ctx, "sqlite:"+filepath.Join(t.TempDir(), "mavio.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	m := open(t, root, db)
	res, err := m.Authenticate(ctx, "dev.mavio.smoke", "admin-ann", "directory")
	if err != nil || !res.Authenticated || !res.Admin || res.DisplayName != "ADMIN-ANN" {
		t.Errorf("Authenticate = %+v, %v", res, err)
	}
	if res, err := m.Authenticate(ctx, "dev.mavio.smoke", "bob", "wrong"); err != nil || res.Authenticated {
		t.Errorf("Authenticate with a wrong password = %+v, %v", res, err)
	}
	if _, err := m.Authenticate(ctx, "org.example.none", "bob", "directory"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("Authenticate with an unknown plugin: %v", err)
	}
	notifiers := m.Notifiers()
	if len(notifiers) != 1 {
		t.Fatalf("notifiers = %d, want 1", len(notifiers))
	}
	if err := notifiers[0].Notify(ctx, core.Activity{ID: core.NewID(), Type: "user.login", Title: "x", Time: time.Now()}); err != nil {
		t.Errorf("Notify: %v", err)
	}
	if err := notifiers[0].Notify(ctx, core.Activity{ID: core.NewID(), Title: "x", Time: time.Now()}); err == nil {
		t.Error("the plugin's rejection was lost")
	}
}

func TestGeneratedIDs(t *testing.T) {
	s, err := store.Open(t.Context(), "sqlite:"+filepath.Join(t.TempDir(), "mavio.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	dir := t.TempDir()
	install(t, dir, "smoke", `{"type":"object","properties":{"clients":{"type":"array","items":{"type":"object",`+
		`"properties":{"id":{"type":"string","format":"uuid","readOnly":true},"name":{"type":"string"}},"required":["id"]}}}}`)
	m := open(t, dir, s)
	_, got, err := m.SetConfig(t.Context(), "dev.mavio.smoke", `{"clients":[{"name":"a"},{"id":"kept","name":"b"}]}`)
	if err != nil {
		t.Fatal(err)
	}
	var c struct{ Clients []struct{ ID string } }
	if err := json.Unmarshal([]byte(got), &c); err != nil {
		t.Fatal(err)
	}
	if _, err := uuid.Parse(c.Clients[0].ID); err != nil || c.Clients[1].ID != "kept" {
		t.Errorf("SetConfig() = %s, want the first client given a UUID and the second's ID kept", got)
	}
	if stored, err := m.Config(t.Context(), "dev.mavio.smoke"); err != nil || stored.JSON != got {
		t.Errorf("Config() = %+v, %v, want = %s", stored, err, got)
	}
}

func TestSettingsSchema(t *testing.T) {
	text := pluginv1.SettingFieldType_SETTING_FIELD_TYPE_TEXT
	fields := []*pluginv1.SettingField{
		pluginv1.SettingField_builder{Name: proto.String("base_url"), Label: proto.String("URL"), Type: &text, DefaultJson: proto.String(`"https://a.b"`)}.Build(),
		pluginv1.SettingField_builder{Name: proto.String("api_key"), Label: proto.String("API Key"), Type: &text, Secret: proto.Bool(true), HelpText: proto.String("<key> & more")}.Build(),
		pluginv1.SettingField_builder{
			Name: proto.String("sort"), Label: proto.String("Sort"), Type: pluginv1.SettingFieldType_SETTING_FIELD_TYPE_SELECT.Enum(),
			Options: []*pluginv1.SettingFieldOption{pluginv1.SettingFieldOption_builder{ValueJson: proto.String("0"), Label: proto.String("Date")}.Build()},
		}.Build(),
		pluginv1.SettingField_builder{Name: proto.String("note"), Label: proto.String("Note"), Type: pluginv1.SettingFieldType_SETTING_FIELD_TYPE_INFO.Enum()}.Build(),
		pluginv1.SettingField_builder{Name: proto.String("flags"), Label: proto.String("Flags"), Type: pluginv1.SettingFieldType_SETTING_FIELD_TYPE_TAG.Enum()}.Build(),
	}
	want := `{"type":"object","properties":{` +
		`"base_url":{"title":"URL","type":"string","default":"https://a.b"},` +
		`"api_key":{"title":"API Key","description":"<key> & more","type":"string","writeOnly":true},` +
		`"sort":{"title":"Sort","oneOf":[{"const":0,"title":"Date"}]},` +
		`"note":{"title":"Note","readOnly":true},` +
		`"flags":{"title":"Flags","type":"array","items":{"type":"string"}}}}`
	if got := plugins.SettingsSchema(fields); got != want {
		t.Errorf("SettingsSchema() = %s, want = %s", got, want)
	}
	if got := plugins.SettingsSchema(nil); got != "" {
		t.Errorf("SettingsSchema(nil) = %s, want = empty", got)
	}
}
