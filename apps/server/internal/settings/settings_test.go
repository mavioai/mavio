package settings

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store"
)

func TestUpdate(t *testing.T) {
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
	s := m.Get()
	s.Transcoding.EncoderPreset = "fast"
	if _, err := m.Update(ctx, s); err != nil {
		t.Fatal(err)
	}
	s.Transcoding.EncoderPreset = "veryslow"
	if _, err := m.Update(ctx, s); !errors.Is(err, reject) {
		t.Errorf("Update rejected = %v, want %v", err, reject)
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
	s.Transcoding.EncoderPreset = "unknown"
	if _, err := m.Update(ctx, s); !errors.Is(err, core.ErrInvalid) {
		t.Errorf("invalid settings: %v", err)
	}
}
