package store_test

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store"
)

func TestEpisodeStates(t *testing.T) {
	eachBackend(t, func(t *testing.T, s *store.Store) {
		ctx := t.Context()
		lib := newLibrary(t, s, "/shows")
		show, other := newItem(lib, core.KindSeries, "Show"), newItem(lib, core.KindSeries, "Other")
		upsert(t, s, show, other)
		repo := s.EpisodeStates()
		if list, err := repo.List(ctx, show.ID); err != nil || len(list) != 0 {
			t.Errorf("List before any = %+v, %v; want = none", list, err)
		}
		at := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
		if err := repo.Searched(ctx, show.ID, at, core.EpisodeNumber{Season: 1, Episode: 2}, core.EpisodeNumber{Season: 2, Episode: 1}); err != nil {
			t.Fatal(err)
		}
		numbering := []core.EpisodeState{
			{EpisodeNumber: core.EpisodeNumber{Season: 1, Episode: 1}, SceneSeason: new(1), SceneEpisode: new(1), SceneAbsolute: new(1)},
			{EpisodeNumber: core.EpisodeNumber{Season: 2, Episode: 1}, SceneSeason: new(1), SceneEpisode: new(3)},
			{EpisodeNumber: core.EpisodeNumber{Season: 2, Episode: 2}, UnverifiedSceneNumbering: true},
		}
		if err := repo.PutSceneNumbering(ctx, show.ID, numbering); err != nil {
			t.Fatal(err)
		}
		if err := repo.Searched(ctx, other.ID, at, core.EpisodeNumber{Season: 1, Episode: 1}); err != nil {
			t.Fatal(err)
		}
		want := []core.EpisodeState{
			{SeriesID: show.ID, EpisodeNumber: core.EpisodeNumber{Season: 1, Episode: 1}, SceneSeason: new(1), SceneEpisode: new(1), SceneAbsolute: new(1)},
			{SeriesID: show.ID, EpisodeNumber: core.EpisodeNumber{Season: 1, Episode: 2}, LastSearchedAt: at},
			{SeriesID: show.ID, EpisodeNumber: core.EpisodeNumber{Season: 2, Episode: 1}, SceneSeason: new(1), SceneEpisode: new(3), LastSearchedAt: at},
			{SeriesID: show.ID, EpisodeNumber: core.EpisodeNumber{Season: 2, Episode: 2}, UnverifiedSceneNumbering: true},
		}
		if got, err := repo.List(ctx, show.ID); err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("List = %+v, %v; want = %+v", got, err, want)
		}

		// A later numbering replaces the earlier; search times stay, and
		// states keeping nothing go.
		later := at.Add(time.Hour)
		if err := repo.Searched(ctx, show.ID, later, core.EpisodeNumber{Season: 1, Episode: 1}); err != nil {
			t.Fatal(err)
		}
		if err := repo.PutSceneNumbering(ctx, show.ID, []core.EpisodeState{
			{EpisodeNumber: core.EpisodeNumber{Season: 1, Episode: 2}, SceneSeason: new(1), SceneEpisode: new(3)},
		}); err != nil {
			t.Fatal(err)
		}
		want = []core.EpisodeState{
			{SeriesID: show.ID, EpisodeNumber: core.EpisodeNumber{Season: 1, Episode: 1}, LastSearchedAt: later},
			{SeriesID: show.ID, EpisodeNumber: core.EpisodeNumber{Season: 1, Episode: 2}, SceneSeason: new(1), SceneEpisode: new(3), LastSearchedAt: at},
			{SeriesID: show.ID, EpisodeNumber: core.EpisodeNumber{Season: 2, Episode: 1}, LastSearchedAt: at},
		}
		if got, err := repo.List(ctx, show.ID); err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("List after a later numbering = %+v, %v; want = %+v", got, err, want)
		}
		if err := repo.Searched(ctx, show.ID, at, core.EpisodeNumber{Season: -1}); !errors.Is(err, core.ErrInvalid) {
			t.Errorf("Searched of season -1: %v, want ErrInvalid", err)
		}
		if err := repo.PutSceneNumbering(ctx, core.ID{}, nil); err != nil {
			t.Errorf("PutSceneNumbering of nothing: %v", err)
		}
		if err := repo.Searched(ctx, core.NewID(), at, core.EpisodeNumber{Season: 1, Episode: 1}); err == nil {
			t.Error("Searched of an unknown series succeeded")
		}

		// The states go with their series.
		if err := s.Items().Delete(ctx, show.ID); err != nil {
			t.Fatal(err)
		}
		if got, err := repo.List(ctx, show.ID); err != nil || len(got) != 0 {
			t.Errorf("List of a deleted series = %+v, %v; want = none", got, err)
		}
		if got, err := repo.List(ctx, other.ID); err != nil || len(got) != 1 {
			t.Errorf("List(other) = %+v, %v; want = 1 state", got, err)
		}
	})
}
