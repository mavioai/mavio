package backup

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store"
)

func open(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(t.Context(), "sqlite:"+filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func write(t *testing.T, p, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestBackupAndRestore(t *testing.T) {
	ctx := t.Context()
	src := open(t)
	u := core.User{ID: core.NewID(), Name: "admin", PasswordHash: "x", Admin: true}
	if err := src.Users().Create(ctx, &u); err != nil {
		t.Fatal(err)
	}
	meta, plugins, data := t.TempDir(), t.TempDir(), t.TempDir()
	write(t, filepath.Join(data, "smoke", "state.json"), "{}")
	write(t, filepath.Join(meta, "ab", "abcd", "primary.png"), "png")
	write(t, filepath.Join(plugins, "smoke", "manifest.json"), "{}")
	write(t, filepath.Join(plugins, ".install-x", "manifest.json"), "{}")
	m := New(Config{
		Store: src, Dir: t.TempDir(), Folders: Folders{Metadata: meta, Plugins: plugins, PluginData: data}, Version: "v1",
		Now: func() time.Time { return time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC) },
	})
	b, err := m.Create(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if b.Name != "mavio-20261010-120000.zip" || b.Version != "v1" || b.Size == 0 {
		t.Errorf("backup = %+v", b)
	}
	list, err := m.List()
	if err != nil || len(list) != 1 || list[0].Name != b.Name {
		t.Errorf("List = %+v, %v", list, err)
	}

	dst := open(t)
	meta2, plugins2, data2 := t.TempDir(), filepath.Join(t.TempDir(), "plugins"), t.TempDir()
	if err := Restore(ctx, filepath.Join(m.cfg.Dir, b.Name), dst, Folders{Metadata: meta2, Plugins: plugins2, PluginData: data2}); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(data2, "smoke", "state.json")); err != nil || string(got) != "{}" {
		t.Errorf("restored plugin data = %q, %v", got, err)
	}
	if got, err := dst.Users().GetByName(ctx, "admin"); err != nil || got.ID != u.ID {
		t.Errorf("restored user = %+v, %v", got, err)
	}
	if data, err := os.ReadFile(filepath.Join(meta2, "ab", "abcd", "primary.png")); err != nil || string(data) != "png" {
		t.Errorf("restored artwork = %q, %v", data, err)
	}
	if _, err := os.Stat(filepath.Join(plugins2, "smoke", "manifest.json")); err != nil {
		t.Errorf("restored plugin: %v", err)
	}
	if _, err := os.Stat(filepath.Join(plugins2, ".install-x")); err == nil {
		t.Error("a hidden folder was backed up")
	}

	if _, err := m.Open("../x.zip"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("Open outside the folder: %v", err)
	}
	if err := m.Delete(b.Name); err != nil {
		t.Fatal(err)
	}
	if list, _ := m.List(); len(list) != 0 {
		t.Errorf("after Delete: %+v", list)
	}
}
