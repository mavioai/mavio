package plugins_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mavioai/mavio/apps/server/internal/plugins"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store"
)

func TestTasks(t *testing.T) {
	ctx := t.Context()
	s, err := store.Open(ctx, "sqlite:"+filepath.Join(t.TempDir(), "mavio.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	dir, data := t.TempDir(), t.TempDir()
	installWith(t, dir, "smoke", "{}", `"capabilities":["CAPABILITY_TASK_RUNNER"],
		"tasks":[{"id":"count","name":"Count","interval":"3600s"},{"id":"fail","name":"Fail"}]`)
	now := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	m, err := plugins.Open(ctx, plugins.Config{
		Dir: dir, CacheDir: t.TempDir(), DataDir: data, Store: s, Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Close(t.Context()) })

	if tasks := m.Tasks(); len(tasks) != 2 || tasks[0].Plugin != "org.mavio.smoke" || tasks[0].Task.GetId() != "count" {
		t.Fatalf("Tasks() = %v", tasks)
	}
	// Starting the plugin scheduled the task with an interval.
	pending, err := s.Jobs().List(ctx, core.JobQuery{Kinds: []string{plugins.JobTask}})
	if err != nil {
		t.Fatal(err)
	}
	if len(pending.Items) != 1 || !pending.Items[0].RunAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("scheduled runs = %+v, want one in an hour", pending.Items)
	}
	var p plugins.TaskPayload
	if err := json.Unmarshal(pending.Items[0].Payload, &p); err != nil || p.Task != "count" || !p.Scheduled {
		t.Errorf("scheduled payload = %+v, %v", p, err)
	}

	// A scheduled run queues the next; its plugin keeps state in its data
	// folder.
	for range 2 {
		next, err := m.RunTask(ctx, pending.Items[0])
		if err != nil {
			t.Fatalf("RunTask: %v", err)
		}
		if len(next) != 1 || !next[0].RunAt.Equal(now.Add(time.Hour)) {
			t.Errorf("next runs = %+v", next)
		}
	}
	if got, err := os.ReadFile(filepath.Join(data, "org.mavio.smoke", "runs")); err != nil || string(got) != "2" {
		t.Errorf("runs counted in the data folder = %q, %v", got, err)
	}
	// Requested runs queue nothing; failures are errors.
	if next, err := m.RunTask(ctx, plugins.TaskJob("org.mavio.smoke", "count", now, false)); err != nil || len(next) != 0 {
		t.Errorf("requested run = %v, %v", next, err)
	}
	if _, err := m.RunTask(ctx, plugins.TaskJob("org.mavio.smoke", "fail", now, false)); err == nil {
		t.Error("failing task succeeded")
	}
	// Runs of tasks that are gone are dropped.
	if next, err := m.RunTask(ctx, plugins.TaskJob("org.mavio.smoke", "gone", now, true)); err != nil || len(next) != 0 {
		t.Errorf("run of a task that is gone = %v, %v", next, err)
	}

	// Uninstalling deletes the data folder.
	if err := m.Uninstall(ctx, "org.mavio.smoke"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(data, "org.mavio.smoke")); !os.IsNotExist(err) {
		t.Errorf("data folder after uninstalling: %v", err)
	}
}
