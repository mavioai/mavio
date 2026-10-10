package plugins_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mavioai/mavio/apps/server/internal/plugins"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store"
)

func TestPublish(t *testing.T) {
	ctx := t.Context()
	s, err := store.Open(ctx, "sqlite:"+filepath.Join(t.TempDir(), "mavio.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	dir, data := t.TempDir(), t.TempDir()
	installWith(t, dir, "smoke", "{}", `"capabilities":["CAPABILITY_EVENT_CONSUMER"],"permissions":{"events":["item.*"]}`)
	m, err := plugins.Open(ctx, plugins.Config{Dir: dir, CacheDir: t.TempDir(), DataDir: data, Store: s})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Close(t.Context()) })

	if !m.Wants("item.added") || m.Wants("user.login") {
		t.Errorf("Wants(item.added), Wants(user.login) = %v, %v, want true, false", m.Wants("item.added"), m.Wants("user.login"))
	}
	m.Publish(core.Activity{ID: core.NewID(), Type: "user.login", Title: "admin signed in"})
	m.Publish(core.Activity{ID: core.NewID(), Type: "item.added", Title: "Movie", Attributes: map[string]string{"library": "l1"}})
	m.Publish(core.Activity{ID: core.NewID(), Type: "item.removed", Title: "Movie"})
	file := filepath.Join(data, "org.mavio.smoke", "events")
	want := "item.added library=l1\nitem.removed\n"
	for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(50 * time.Millisecond) {
		got, _ := os.ReadFile(file)
		if string(got) == want {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("consumed events = %q, want %q", got, want)
		}
	}
}
