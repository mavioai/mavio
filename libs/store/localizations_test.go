package store_test

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store"
)

func TestLocalizations(t *testing.T) {
	eachBackend(t, func(t *testing.T, s *store.Store) {
		ctx := t.Context()
		lib := newLibrary(t, s, "/media/movies")
		movie, other := newItem(lib, core.KindMovie, "Spirited Away"), newItem(lib, core.KindMovie, "Up")
		upsert(t, s, movie, other)
		fetched := time.Date(2026, 10, 22, 8, 0, 0, 0, time.UTC)
		for _, l := range []core.ItemLocalization{
			{ItemID: movie.ID, Language: "zh", Name: "千与千寻", Overview: "少女千寻", FetchedAt: fetched},
			{ItemID: movie.ID, Language: "ja", Name: "千と千尋の神隠し", FetchedAt: fetched},
			{ItemID: movie.ID, Language: "zh", Name: "千与千寻", Overview: "十岁的千寻", Tagline: "隧道的另一边", FetchedAt: fetched.Add(time.Hour)},
		} {
			if err := s.Localizations().Put(ctx, &l); err != nil {
				t.Fatal(err)
			}
		}
		got, err := s.Localizations().Get(ctx, movie.ID, "zh")
		if err != nil || got.Name != "千与千寻" || got.Overview != "十岁的千寻" || got.Tagline != "隧道的另一边" || !got.FetchedAt.Equal(fetched.Add(time.Hour)) {
			t.Errorf("Get(zh) = %+v, %v, want the later one", got, err)
		}
		if _, err := s.Localizations().Get(ctx, other.ID, "zh"); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("Get of an item without one: err = %v, want ErrNotFound", err)
		}
		many, err := s.Localizations().GetMany(ctx, []core.ID{movie.ID, other.ID}, "ja")
		if err != nil || len(many) != 1 || many[movie.ID].Name != "千と千尋の神隠し" {
			t.Errorf("GetMany(ja) = %+v, %v", many, err)
		}
		if langs, err := s.Localizations().Languages(ctx, movie.ID); err != nil || !slices.Equal(langs, []string{"ja", "zh"}) {
			t.Errorf("Languages = %v, %v, want [ja zh]", langs, err)
		}
		if err := s.Localizations().Put(ctx, &core.ItemLocalization{ItemID: movie.ID, Language: "zh-CN"}); !errors.Is(err, core.ErrInvalid) {
			t.Errorf("Put of zh-CN: err = %v, want ErrInvalid", err)
		}

		for _, term := range []string{"千寻", "qian yu"} {
			page, err := s.Items().Query(ctx, core.ItemQuery{Search: term})
			if err != nil || page.Total != 1 || page.Items[0].ID != movie.ID {
				t.Errorf("search %q = %v, %v, want the movie by its Chinese name", term, names(page.Items), err)
			}
		}
		if page, err := s.Items().Query(ctx, core.ItemQuery{Search: "spirited"}); err != nil || page.Total != 1 {
			t.Errorf("search by its own name = %v, %v", names(page.Items), err)
		}

		if err := s.Items().Delete(ctx, movie.ID); err != nil {
			t.Fatal(err)
		}
		if langs, err := s.Localizations().Languages(ctx, movie.ID); err != nil || len(langs) != 0 {
			t.Errorf("Languages of a deleted item = %v, %v, want none", langs, err)
		}
	})
}
