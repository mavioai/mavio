package settings

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store"
)

func TestChangeApplied(t *testing.T) {
	ctx := t.Context()
	db, err := store.Open(ctx, "sqlite:"+filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	m, err := Open(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	var applied []string
	if err := m.Register(ctx, func(_ context.Context, s core.ServerSettings) error {
		applied = append(applied, s.Transcoding.EncoderPreset)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	reject := errors.New("no slow presets")
	if err := m.Register(ctx, func(_ context.Context, s core.ServerSettings) error {
		if s.Transcoding.EncoderPreset == "veryslow" {
			return reject
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	preset := func(p string) func(*core.ServerSettings) error {
		return func(s *core.ServerSettings) error {
			s.Transcoding.EncoderPreset = p
			return nil
		}
	}
	if _, err := m.Change(ctx, preset("fast")); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Change(ctx, preset("veryslow")); !errors.Is(err, reject) {
		t.Errorf("Change rejected = %v, want %v", err, reject)
	}
	// The first applier got the fast preset back.
	if want := []string{"", "fast", "veryslow", "fast"}; len(applied) != 4 || applied[3] != "fast" {
		t.Errorf("applied = %q, want = %q", applied, want)
	}
	if got := m.Get().Transcoding.EncoderPreset; got != "fast" {
		t.Errorf("current preset = %q", got)
	}
	stored, _ := db.Settings().Get(ctx)
	if stored.Transcoding.EncoderPreset != "fast" {
		t.Errorf("stored preset = %q", stored.Transcoding.EncoderPreset)
	}
	if _, err := m.Change(ctx, preset("unknown")); !errors.Is(err, core.ErrInvalid) {
		t.Errorf("invalid settings: %v", err)
	}
}

func TestChange(t *testing.T) {
	ctx := t.Context()
	db, err := store.Open(ctx, "sqlite:"+filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	m, err := Open(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Change(ctx, func(s *core.ServerSettings) error {
		s.DisabledPlugins = []string{"org.mavio.smoke"}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if got := m.Get().DisabledPlugins; len(got) != 1 || got[0] != "org.mavio.smoke" {
		t.Errorf("current disabled plugins = %q", got)
	}
	stored, _ := db.Settings().Get(ctx)
	if len(stored.DisabledPlugins) != 1 {
		t.Errorf("stored disabled plugins = %q", stored.DisabledPlugins)
	}
	// A change failing, or altering what it was given in place and then
	// failing, leaves the current settings as they were.
	fail := errors.New("refused")
	if _, err := m.Change(ctx, func(s *core.ServerSettings) error {
		s.DisabledPlugins[0] = "org.mavio.dlna"
		return fail
	}); !errors.Is(err, fail) {
		t.Errorf("failing change = %v, want = %v", err, fail)
	}
	if got := m.Get().DisabledPlugins; got[0] != "org.mavio.smoke" {
		t.Errorf("disabled plugins after a failing change = %q", got)
	}
}
