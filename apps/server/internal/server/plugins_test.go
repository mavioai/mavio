package server_test

import (
	"fmt"
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
// plugin platform, without ffmpeg: its tasks and data folder.
func TestPluginPlatform(t *testing.T) {
	ctx := t.Context()
	pluginDir := installSmoke(t, `"capabilities":["CAPABILITY_TASK_RUNNER"],
		"tasks":[{"id":"count","name":"Count","description":"Counts its runs.","interval":"86400s"}]`)
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
}
