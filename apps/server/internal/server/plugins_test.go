package server_test

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/mavioai/mavio/apps/server/internal/server"
	authv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1/authv1connect"
	systemv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1/systemv1connect"
	userv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1/userv1connect"
)

// installSmoke builds the smoke plugin for WASM into a plugin folder, with
// the given manifest fields besides its ID, name, version, runtime and API
// version.
func installSmoke(t *testing.T, fields string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "smoke")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	build := exec.CommandContext(t.Context(), "go", "build", "-buildmode=c-shared", "-o", filepath.Join(dir, "plugin.wasm"),
		"github.com/mavioai/mavio/apps/server/internal/smoke/smokeplugin")
	build.Env = append(os.Environ(), "GOOS=wasip1", "GOARCH=wasm")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build plugin: %v\n%s", err, out)
	}
	manifest := fmt.Sprintf(`{"id":"org.mavio.smoke","name":"Smoke","version":"0.1.0","runtime":"RUNTIME_WASM",
		"apiVersion":"1.0",%s}`, fields)
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// TestPluginPlatform runs the assembled server with a plugin using the
// plugin platform, without ffmpeg: its tasks, data folder, events and HTTP
// routes.
func TestPluginPlatform(t *testing.T) {
	ctx := t.Context()
	pluginDir := installSmoke(t, `"capabilities":["CAPABILITY_TASK_RUNNER","CAPABILITY_EVENT_CONSUMER","CAPABILITY_HTTP_HANDLER"],
		"tasks":[{"id":"count","name":"Count","description":"Counts its runs.","interval":"86400s"}],
		"permissions":{"events":["task.*","user.created"]}`)
	dataDir := t.TempDir()
	srv := start(t, server.Config{
		Database: "sqlite:" + filepath.Join(t.TempDir(), "mavio.db"), FFprobe: "no-ffprobe", FFmpeg: "no-ffmpeg",
		TranscodeDir: t.TempDir(), CacheDir: t.TempDir(), MetadataDir: t.TempDir(), PluginDir: pluginDir, PluginDataDir: dataDir,
	})
	first, err := authv1connect.NewAuthServiceClient(http.DefaultClient, srv.url).CreateFirstUser(ctx, authv1.CreateFirstUserRequest_builder{
		Name: new("admin"), Password: new("secret"),
		Device: authv1.Device_builder{Id: new("e2e"), Name: new("Test"), Client: new("Test"), ClientVersion: new("0")}.Build(),
	}.Build())
	if err != nil {
		t.Fatalf("CreateFirstUser: %v", err)
	}
	tasks := systemv1connect.NewTaskServiceClient(http.DefaultClient, srv.url, withToken(first.GetAccessToken()))

	// The plugin's task is listed, scheduled, and runs on demand.
	const id = "plugin:org.mavio.smoke:count"
	task := func() *systemv1.Task {
		t.Helper()
		list, err := tasks.ListTasks(ctx, &systemv1.ListTasksRequest{})
		if err != nil {
			t.Fatalf("ListTasks: %v", err)
		}
		for _, task := range list.GetTasks() {
			if task.GetId() == id {
				return task
			}
		}
		t.Fatalf("ListTasks = %v, want %s", list, id)
		return nil
	}
	if got := task(); got.GetPluginId() != "org.mavio.smoke" || got.GetName() != "Count" ||
		got.GetInterval().AsDuration() != 24*time.Hour || got.GetNextRunTime() == nil {
		t.Errorf("plugin task = %v", got)
	}
	if _, err := tasks.RunTask(ctx, systemv1.RunTaskRequest_builder{Id: new(id)}.Build()); err != nil {
		t.Fatalf("RunTask: %v", err)
	}
	for deadline := time.Now().Add(30 * time.Second); task().GetLastRun() == nil; time.Sleep(100 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the task did not run within 30 s")
		}
	}
	if run := task().GetLastRun(); run.GetState() != systemv1.JobState_JOB_STATE_SUCCEEDED {
		t.Errorf("task run = %v", run)
	}
	if got, err := os.ReadFile(filepath.Join(dataDir, "org.mavio.smoke", "runs")); err != nil || string(got) != "1" {
		t.Errorf("runs in the plugin's data folder = %q, %v", got, err)
	}

	// The plugin consumes the events it asked for: the task's run and the
	// new account, but not sign-ins.
	users := userv1connect.NewUserServiceClient(http.DefaultClient, srv.url, withToken(first.GetAccessToken()))
	kid, err := users.CreateUser(ctx, userv1.CreateUserRequest_builder{Name: new("kid"), Password: new("secret")}.Build())
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	want := "task.completed plugin=org.mavio.smoke task=plugin:org.mavio.smoke:count\n" +
		"user.created by=" + first.GetUser().GetId() + "\n"
	events := filepath.Join(dataDir, "org.mavio.smoke", "events")
	for deadline := time.Now().Add(30 * time.Second); ; time.Sleep(100 * time.Millisecond) {
		got, _ := os.ReadFile(events)
		if string(got) == want {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("consumed events = %q, want %q (kid %s)", got, want, kid.GetUser().GetId())
		}
	}

	// The plugin's routes see the caller a valid token names, never the
	// token, and serve sandboxed pages; forged user headers are dropped.
	whoami := func(header, value string) (string, string) {
		t.Helper()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.url+"/plugins/org.mavio.smoke/whoami", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set(header, value)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil || resp.StatusCode != http.StatusOK {
			t.Fatalf("GET whoami = %d %s, %v", resp.StatusCode, body, err)
		}
		return string(body), resp.Header.Get("Content-Security-Policy")
	}
	body, csp := whoami("Authorization", "Bearer "+first.GetAccessToken())
	if want := `user="admin" admin="true" authorization=""`; body != want {
		t.Errorf("whoami with a token = %s, want %s", body, want)
	}
	if csp != "sandbox allow-scripts allow-forms allow-popups" {
		t.Errorf("Content-Security-Policy = %q", csp)
	}
	if body, _ := whoami("Mavio-User-Name", "admin"); body != `user="" admin="" authorization=""` {
		t.Errorf("whoami with a forged user = %s", body)
	}
	if body, _ := whoami("Authorization", "Basic eDp5"); body != `user="" admin="" authorization="Basic eDp5"` {
		t.Errorf("whoami with the plugin's own credentials = %s", body)
	}
	resp, err := http.Get(srv.url + "/plugins/org.mavio.missing/whoami")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("route of a missing plugin = %d, want 404", resp.StatusCode)
	}
}
