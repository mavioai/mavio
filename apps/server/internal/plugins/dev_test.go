package plugins_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/apps/server/internal/plugins"
	"github.com/mavioai/mavio/libs/store"
)

func TestDevPlugins(t *testing.T) {
	ctx := t.Context()
	s, err := store.Open(ctx, "sqlite:"+filepath.Join(t.TempDir(), "mavio.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	installed := t.TempDir()
	install(t, installed, "smoke", `{"type":"object"}`)
	// A project, whose dev target built the plugin into dist/dev.
	project := filepath.Join(t.TempDir(), "smoke")
	install(t, project, "", `{"type":"object"}`)
	if err := os.Remove(filepath.Join(project, "plugin.wasm")); err != nil {
		t.Fatal(err)
	}
	install(t, project, plugins.DevBuild, `{"type":"object"}`)
	built := filepath.Join(project, filepath.FromSlash(plugins.DevBuild))

	m, err := plugins.Open(ctx, plugins.Config{
		Dir: installed, Dev: []string{project}, DevPoll: 20 * time.Millisecond,
		CacheDir: compiled, DataDir: t.TempDir(), Store: s,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Close(ctx) })
	const id = "dev.mavio.smoke"
	if n := len(m.Plugins()); n != 1 {
		t.Fatalf("plugins = %d, want = 1: the development folder takes the installed plugin's place", n)
	}
	if got := state(t, m, id); got.State != plugins.Ready || got.DevFolder != built {
		t.Fatalf("plugin = %v, %q; want = Ready, %q", got.State, got.DevFolder, built)
	}
	if _, err := m.Install(ctx, id, ""); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("Install = %v, want FailedPrecondition", err)
	}
	if err := m.Uninstall(ctx, id); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("Uninstall = %v, want FailedPrecondition", err)
	}

	// A build that renames it restarts it.
	file := filepath.Join(built, "manifest.json")
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(strings.Replace(string(data), `"Smoke"`, `"Smoke 2"`, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool {
		p := state(t, m, id)
		return p.Manifest.GetName() == "Smoke 2" && p.State == plugins.Ready
	})

	// A broken build fails it, and the next one starts it again.
	if err := os.Rename(filepath.Join(built, "plugin.wasm"), filepath.Join(built, "plugin.old")); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return state(t, m, id).State == plugins.Failed })
	if err := os.Rename(filepath.Join(built, "plugin.old"), filepath.Join(built, "plugin.wasm")); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return state(t, m, id).State == plugins.Ready })
	if err := m.Restart(ctx, id); err != nil {
		t.Errorf("Restart = %v", err)
	}
}

func waitFor(t *testing.T, done func() bool) {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); !done(); {
		if time.Now().After(deadline) {
			t.Fatal("timed out")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
