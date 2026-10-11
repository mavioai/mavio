package store_test

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store"
)

func TestExtraFiles(t *testing.T) {
	eachBackend(t, func(t *testing.T, s *store.Store) {
		ctx := t.Context()
		lib := newLibrary(t, s, "/movies")
		heat, alien := newItem(lib, core.KindMovie, "Heat"), newItem(lib, core.KindMovie, "Alien")
		upsert(t, s, heat, alien)
		repo := s.ExtraFiles()
		base := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
		sub := core.ExtraFile{
			ItemID: heat.ID, Kind: core.ExtraFileSubtitle, Path: "/movies/Heat (1995)/Heat (1995).en.forced.srt", Extension: ".srt",
			Language: "English", LanguageTags: []string{"forced"}, Added: base, LastUpdated: base.Add(time.Hour),
		}
		nfo := core.ExtraFile{ID: core.NewID(), ItemID: heat.ID, Kind: core.ExtraFileOther, Path: "/movies/Heat (1995)/Heat (1995).nfo", Extension: ".nfo", Added: base, LastUpdated: base}
		other := core.ExtraFile{ID: core.NewID(), ItemID: alien.ID, Kind: core.ExtraFileOther, Path: "/movies/Alien (1979)/Alien (1979).nfo", Added: base, LastUpdated: base}
		for _, e := range []*core.ExtraFile{&sub, &nfo, &other} {
			if err := repo.Put(ctx, e); err != nil {
				t.Fatal(err)
			}
		}
		if sub.ID.IsZero() {
			t.Error("Put() left the ID zero")
		}
		got, err := repo.List(ctx, heat.ID)
		if err != nil || !reflect.DeepEqual(got, []core.ExtraFile{sub, nfo}) {
			t.Errorf("List(Heat) = %+v, %v, want = %+v", got, err, []core.ExtraFile{sub, nfo})
		}

		// Put replaces.
		sub.LanguageTags, sub.LastUpdated = nil, base.Add(2*time.Hour)
		if err := repo.Put(ctx, &sub); err != nil {
			t.Fatal(err)
		}
		if got, err := repo.List(ctx, heat.ID); err != nil || len(got) != 2 || !reflect.DeepEqual(got[0], sub) {
			t.Errorf("List(after Put) = %+v, %v, want = %+v first", got, err, sub)
		}

		for _, bad := range []core.ExtraFile{
			{ItemID: heat.ID, Kind: "sample", Path: "/movies/x"},
			{ItemID: heat.ID, Kind: core.ExtraFileOther, Path: "relative/x"},
			{Kind: core.ExtraFileOther, Path: "/movies/x"},
		} {
			if err := repo.Put(ctx, &bad); !errors.Is(err, core.ErrInvalid) {
				t.Errorf("Put(%+v) = %v, want = ErrInvalid", bad, err)
			}
		}

		if err := repo.Delete(ctx, nfo.ID); err != nil {
			t.Fatal(err)
		}
		if got, err := repo.List(ctx, heat.ID); err != nil || len(got) != 1 || got[0].ID != sub.ID {
			t.Errorf("List(after Delete) = %+v, %v, want = the subtitle", got, err)
		}
		// Deleting the item deletes its extra files.
		if err := s.Items().Delete(ctx, alien.ID); err != nil {
			t.Fatal(err)
		}
		if got, err := repo.List(ctx, alien.ID); err != nil || len(got) != 0 {
			t.Errorf("List(deleted item) = %+v, %v, want = none", got, err)
		}
	})
}
