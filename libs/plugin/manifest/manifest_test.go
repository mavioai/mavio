package manifest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

func valid() *pluginv1.Manifest {
	return pluginv1.Manifest_builder{
		Id:           proto.String("org.mavio.scraper-tmdb"),
		Name:         proto.String("TMDB"),
		Version:      proto.String("1.2.3"),
		Runtime:      pluginv1.Runtime_RUNTIME_WASM.Enum(),
		Capabilities: []pluginv1.Capability{pluginv1.Capability_CAPABILITY_METADATA_PROVIDER},
		ApiVersion:   proto.String("1.0"),
		Permissions:  pluginv1.Permissions_builder{HttpHosts: []string{"api.themoviedb.org", "*.tmdb.org"}}.Build(),
		ConfigSchema: proto.String(`{"type":"object","properties":{"api_key":{"type":"string"}},"required":["api_key"]}`),
	}.Build()
}

func task(id string, interval, timeout time.Duration) *pluginv1.Task {
	t := pluginv1.Task_builder{Id: proto.String(id), Name: proto.String("Task " + id)}.Build()
	if interval != 0 {
		t.SetInterval(durationpb.New(interval))
	}
	if timeout != 0 {
		t.SetTimeout(durationpb.New(timeout))
	}
	return t
}

func withTasks(m *pluginv1.Manifest, tasks ...*pluginv1.Task) {
	m.SetCapabilities(append(m.GetCapabilities(), pluginv1.Capability_CAPABILITY_TASK_RUNNER))
	m.SetTasks(tasks)
}

func TestTasks(t *testing.T) {
	m := valid()
	withTasks(m, task("sync", time.Hour, 0), task("rebuild", 0, 2*time.Hour))
	if err := Validate(m); err != nil {
		t.Fatalf("Validate() = %v", err)
	}
	if got := TaskTimeout(m.GetTasks()[0]); got != DefaultTaskTimeout {
		t.Errorf("TaskTimeout(sync) = %v, want %v", got, DefaultTaskTimeout)
	}
	if got := TaskTimeout(m.GetTasks()[1]); got != 2*time.Hour {
		t.Errorf("TaskTimeout(rebuild) = %v, want 2h", got)
	}
}

func TestValidate(t *testing.T) {
	if err := Validate(valid()); err != nil {
		t.Fatalf("valid manifest: %v", err)
	}
	tests := []struct {
		name string
		mut  func(m *pluginv1.Manifest)
		want string
	}{
		{"bad id", func(m *pluginv1.Manifest) { m.SetId("TMDB") }, "reverse-DNS"},
		{"bad version", func(m *pluginv1.Manifest) { m.SetVersion("v1") }, "semantic version"},
		{"no runtime", func(m *pluginv1.Manifest) { m.ClearRuntime() }, "runtime"},
		{"no capabilities", func(m *pluginv1.Manifest) { m.SetCapabilities(nil) }, "capability"},
		{"future api", func(m *pluginv1.Manifest) { m.SetApiVersion("2.0") }, "not compatible"},
		{"bad host", func(m *pluginv1.Manifest) { m.GetPermissions().SetHttpHosts([]string{"https://x.org/"}) }, "http host"},
		{"relative path", func(m *pluginv1.Manifest) { m.GetPermissions().SetReadPaths([]string{"media"}) }, "absolute"},
		{"bad scope", func(m *pluginv1.Manifest) { m.GetPermissions().SetApi([]string{"mavio.library.v1.ItemService:write"}) }, "api scope"},
		{"tasks without capability", func(m *pluginv1.Manifest) { m.SetTasks([]*pluginv1.Task{task("a", 0, 0)}) }, "go together"},
		{"capability without tasks", func(m *pluginv1.Manifest) {
			m.SetCapabilities(append(m.GetCapabilities(), pluginv1.Capability_CAPABILITY_TASK_RUNNER))
		}, "go together"},
		{"bad task id", func(m *pluginv1.Manifest) { withTasks(m, task("Sync", 0, 0)) }, "task id"},
		{"duplicate task", func(m *pluginv1.Manifest) { withTasks(m, task("a", 0, 0), task("a", 0, 0)) }, "not unique"},
		{"short interval", func(m *pluginv1.Manifest) { withTasks(m, task("a", time.Second, 0)) }, "interval"},
		{"long timeout", func(m *pluginv1.Manifest) { withTasks(m, task("a", 0, 7*time.Hour)) }, "timeout"},
		{"bad schema", func(m *pluginv1.Manifest) { m.SetConfigSchema(`{"type":"nope"}`) }, "config_schema"},
	}
	for _, tt := range tests {
		m := valid()
		tt.mut(m)
		if err := Validate(m); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s: Validate() = %v, want error mentioning %q", tt.name, err, tt.want)
		}
	}
}

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	doc := `{"id":"org.example.hello","name":"Hello","version":"0.1.0","runtime":"RUNTIME_PROCESS",
		"capabilities":["CAPABILITY_NOTIFIER"],"apiVersion":"1.0"}`
	if err := os.WriteFile(filepath.Join(dir, File), []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if m.GetId() != "org.example.hello" || !HasCapability(m, pluginv1.Capability_CAPABILITY_NOTIFIER) {
		t.Errorf("Load = %v", m)
	}
	if !strings.HasPrefix(filepath.Base(Executable(dir, m)), "plugin") {
		t.Errorf("Executable = %s", Executable(dir, m))
	}
}

func TestAllowsHost(t *testing.T) {
	m := valid()
	for host, want := range map[string]bool{
		"api.themoviedb.org":  true,
		"API.THEMOVIEDB.ORG":  true,
		"image.tmdb.org":      true,
		"tmdb.org":            false,
		"evil.com":            false,
		"api.themoviedb.org.": false,
	} {
		if got := AllowsHost(m, host); got != want {
			t.Errorf("AllowsHost(%q) = %v, want %v", host, got, want)
		}
	}
}

func TestAllowsProcedure(t *testing.T) {
	m := valid()
	m.GetPermissions().SetApi([]string{"mavio.library.v1.ItemService:read", "mavio.user.v1.UserService"})
	tests := []struct {
		procedure string
		readOnly  bool
		want      bool
	}{
		{"/mavio.library.v1.ItemService/GetItem", true, true},
		{"/mavio.library.v1.ItemService/UpdateItem", false, false},
		{"/mavio.user.v1.UserService/CreateUser", false, true},
		{"/mavio.user.v1.UserService/ListUsers", true, true},
		{"/mavio.library.v1.LibraryService/ListLibraries", true, false},
		{"/mavio.library.v1.ItemServiceX/GetItem", true, false},
		{"mavio.library.v1.ItemService", true, false},
	}
	for _, tt := range tests {
		if got := AllowsProcedure(m, tt.procedure, tt.readOnly); got != tt.want {
			t.Errorf("AllowsProcedure(%q, %v) = %v, want %v", tt.procedure, tt.readOnly, got, tt.want)
		}
	}
	if err := Validate(m); err != nil {
		t.Errorf("Validate() = %v, want nil", err)
	}
}

func TestValidateConfig(t *testing.T) {
	m := valid()
	if err := ValidateConfig(m, `{"api_key":"k"}`); err != nil {
		t.Errorf("valid config: %v", err)
	}
	for _, cfg := range []string{`{}`, `{"api_key":1}`, `not json`} {
		if err := ValidateConfig(m, cfg); err == nil {
			t.Errorf("config %s accepted", cfg)
		}
	}
	m.ClearConfigSchema()
	if err := ValidateConfig(m, `{"anything":true}`); err != nil {
		t.Errorf("schemaless object: %v", err)
	}
	if err := ValidateConfig(m, `[1]`); err == nil {
		t.Error("schemaless non-object accepted")
	}
}
