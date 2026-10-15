package plugins_test

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"testing"

	"github.com/mavioai/mavio/apps/server/internal/plugins"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store"
)

func TestSetEnabled(t *testing.T) {
	ctx := t.Context()
	s, err := store.Open(ctx, "sqlite:"+filepath.Join(t.TempDir(), "mavio.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	dir := t.TempDir()
	install(t, dir, "smoke", `{"type":"object"}`)
	var saved []string
	var fail error
	openWith := func(disabled []string) *plugins.Manager {
		m, err := plugins.Open(ctx, plugins.Config{
			Dir: dir, CacheDir: compiled, DataDir: t.TempDir(), Store: s, Disabled: disabled,
			SaveDisabled: func(_ context.Context, ids []string) error {
				if fail != nil {
					return fail
				}
				saved = ids
				return nil
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = m.Close(ctx) })
		return m
	}

	m := openWith(nil)
	const id = "dev.mavio.smoke"
	if _, err := m.SetEnabled(ctx, "org.example.missing", false); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("SetEnabled of an unknown plugin: %v, want ErrNotFound", err)
	}
	fail = errors.New("disk full")
	if _, err := m.SetEnabled(ctx, id, false); !errors.Is(err, fail) {
		t.Errorf("SetEnabled unsaved = %v, want %v", err, fail)
	}
	if got := state(t, m, id); got.State != plugins.Ready {
		t.Errorf("unsaved: state = %v, want Ready", got.State)
	}
	fail = nil

	info, err := m.SetEnabled(ctx, id, false)
	if err != nil || info.State != plugins.Disabled {
		t.Fatalf("disable: SetEnabled = %+v, %v; want Disabled", info, err)
	}
	if !slices.Equal(saved, []string{id}) {
		t.Errorf("saved = %q, want = %q", saved, []string{id})
	}
	if n := len(m.MetadataProviders()); n != 0 {
		t.Errorf("disabled: MetadataProviders() = %d, want 0", n)
	}
	if m.Routes(id) != nil {
		t.Error("disabled: Routes() is set, want nil")
	}
	if info, err := m.SetEnabled(ctx, id, false); err != nil || info.State != plugins.Disabled {
		t.Errorf("disable again: SetEnabled = %+v, %v; want Disabled", info, err)
	}
	if err := m.Close(ctx); err != nil {
		t.Fatal(err)
	}

	// The server starts again without it.
	m = openWith(saved)
	if got := state(t, m, id); got.State != plugins.Disabled || got.Manifest.GetName() != "Smoke" {
		t.Errorf("after a restart: %+v, want Disabled with its manifest", got)
	}
	info, err = m.SetEnabled(ctx, id, true)
	if err != nil || info.State != plugins.Ready {
		t.Fatalf("enable: SetEnabled = %+v, %v; want Ready", info, err)
	}
	if len(saved) != 0 {
		t.Errorf("saved = %q, want none", saved)
	}
	providers := m.MetadataProviders()
	if len(providers) != 1 {
		t.Fatalf("enabled: MetadataProviders() = %d, want 1", len(providers))
	}
	if res, err := providers[0].Metadata(ctx, concubine); err != nil || res == nil {
		t.Errorf("enabled: Metadata = %v, %v; want the film", res, err)
	}

	// Uninstalling forgets that it was disabled.
	if _, err := m.SetEnabled(ctx, id, false); err != nil {
		t.Fatal(err)
	}
	if err := m.Uninstall(ctx, id); err != nil {
		t.Fatal(err)
	}
	if len(saved) != 0 {
		t.Errorf("after uninstalling: saved = %q, want none", saved)
	}
}
