package store_test

import (
	"bytes"
	"errors"
	"io"
	"path/filepath"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store"
)

// another opens a new empty store of s's dialect.
func another(t *testing.T, s *store.Store) *store.Store {
	t.Helper()
	dsn := "sqlite:" + filepath.Join(t.TempDir(), "restored.db")
	if s.Dialect() == store.DialectPostgres {
		dsn = freshPostgresDB(t)
	}
	r, err := store.Open(t.Context(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Close() })
	return r
}

func TestDumpAndRestore(t *testing.T) {
	eachBackend(t, func(t *testing.T, s *store.Store) {
		ctx := t.Context()
		u := core.User{ID: core.NewID(), Name: "admin", PasswordHash: "x", Admin: true}
		if err := s.Users().Create(ctx, &u); err != nil {
			t.Fatal(err)
		}
		lib := core.Library{Name: "Shows", Kind: core.LibraryShows, Paths: []string{"/media/shows"}, SaveLocalMetadata: true}
		if err := s.Libraries().Create(ctx, &lib); err != nil {
			t.Fatal(err)
		}
		// Children listed before their parents still restore.
		one := 1
		series := core.Item{ID: core.NewID(), LibraryID: lib.ID, Kind: core.KindSeries, Name: "Lost", Path: "/media/shows/Lost", DateAdded: time.Now(), Genres: []string{"Drama"}}
		season := core.Item{ID: core.NewID(), LibraryID: lib.ID, ParentID: series.ID, Kind: core.KindSeason, Name: "Season 1", IndexNumber: &one, Path: "/media/shows/Lost/S1", DateAdded: time.Now()}
		if err := s.Items().Upsert(ctx, series, season); err != nil {
			t.Fatal(err)
		}
		set := core.DefaultServerSettings()
		set.Transcoding.EncoderPreset = "fast"
		if err := s.Settings().Put(ctx, &set); err != nil {
			t.Fatal(err)
		}
		if err := s.Activities().Add(ctx, &core.Activity{Type: "x", Severity: core.SeverityInfo, Title: "x", Attributes: map[string]string{"a": "b"}}); err != nil {
			t.Fatal(err)
		}

		files := map[string]*bytes.Buffer{}
		info, err := s.Dump(ctx, func(name string) (io.Writer, error) {
			files[name] = &bytes.Buffer{}
			return files[name], nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if info.Rows["items"] != 2 || info.Rows["users"] != 1 || len(info.Migrations) == 0 {
			t.Errorf("dump info = %+v", info)
		}
		r := another(t, s)
		open := func(name string) (io.ReadCloser, error) {
			b, ok := files[name]
			if !ok {
				return nil, core.ErrNotFound
			}
			return io.NopCloser(bytes.NewReader(b.Bytes())), nil
		}
		if err := r.Restore(ctx, info, open); err != nil {
			t.Fatal(err)
		}
		got, err := r.Items().Get(ctx, season.ID)
		if err != nil || got.ParentID != series.ID || got.Name != "Season 1" {
			t.Errorf("season = %+v, %v", got, err)
		}
		if page, err := r.Items().Query(ctx, core.ItemQuery{Search: "lost"}); err != nil || page.Total != 1 {
			t.Errorf("search after restoring = %+v, %v", page, err)
		}
		if l, err := r.Libraries().Get(ctx, lib.ID); err != nil || !l.SaveLocalMetadata {
			t.Errorf("library = %+v, %v", l, err)
		}
		if got, _ := r.Settings().Get(ctx); got.Transcoding.EncoderPreset != "fast" {
			t.Errorf("settings = %+v", got.Transcoding)
		}
		if page, _ := r.Activities().List(ctx, core.ActivityQuery{}); page.Total != 1 || page.Items[0].Attributes["a"] != "b" {
			t.Errorf("activities = %+v", page)
		}
		// A database with users is not restored over.
		if err := r.Restore(ctx, info, open); !errors.Is(err, core.ErrConflict) {
			t.Errorf("Restore into a used database: %v, want ErrConflict", err)
		}
		info.Migrations = append(info.Migrations, "99999999999999")
		if err := another(t, s).Restore(ctx, info, open); !errors.Is(err, core.ErrInvalid) {
			t.Errorf("Restore of a newer dump: %v, want ErrInvalid", err)
		}
	})
}
