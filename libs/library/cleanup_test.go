package library

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

func TestCleanupSweepsMetadata(t *testing.T) {
	ctx := t.Context()
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	st := newMemStore()
	dir := t.TempDir()
	kept := core.Item{ID: core.NewID(), LibraryID: core.NewID(), Kind: core.KindMovie, Name: "Kept"}
	if err := st.Items().Upsert(ctx, kept); err != nil {
		t.Fatal(err)
	}
	deleted, recent := core.NewID(), core.NewID()
	folder := func(id core.ID, age time.Duration) string {
		d := extrasFolder(dir, id, "subtitles")
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "movie.fr.srt"), []byte("1"), 0o644); err != nil {
			t.Fatal(err)
		}
		item := extrasFolder(dir, id)
		if err := os.Chtimes(item, now.Add(-age), now.Add(-age)); err != nil {
			t.Fatal(err)
		}
		return item
	}
	keptDir, deletedDir, recentDir := folder(kept.ID, 48*time.Hour), folder(deleted, 48*time.Hour), folder(recent, time.Hour)
	// Folders that are not items' are left alone.
	other := filepath.Join(dir, "zz", "not-an-id")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}

	j := &Jobs{Store: st, MetadataDir: filepath.ToSlash(dir), Now: func() time.Time { return now }}
	if _, err := j.cleanup(ctx, CleanupJob(now, false)); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name, dir string
		want      bool
	}{
		{"existing item", keptDir, true},
		{"deleted item", deletedDir, false},
		{"deleted item changed within the grace", recentDir, true},
		{"other folder", other, true},
	} {
		_, err := os.Stat(tt.dir)
		if got := err == nil; got != tt.want {
			t.Errorf("%s: folder kept got = %v, want = %v", tt.name, got, tt.want)
		}
	}

	// Without a metadata folder there is nothing to sweep.
	j.MetadataDir = filepath.ToSlash(filepath.Join(dir, "missing"))
	if _, err := j.cleanup(ctx, CleanupJob(now, false)); err != nil {
		t.Errorf("cleanup without a metadata folder got = %v, want = nil", err)
	}
}
