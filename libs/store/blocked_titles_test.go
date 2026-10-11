package store_test

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store"
)

func TestBlockedTitles(t *testing.T) {
	eachBackend(t, func(t *testing.T, s *store.Store) {
		ctx := t.Context()
		admin := newUser(t, s, "admin")
		repo := s.BlockedTitles()
		base := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
		matrix := core.BlockedTitle{Kind: core.KindMovie, TmdbID: 603, Title: "The Matrix", UserID: admin.ID, CreatedAt: base}
		series := core.BlockedTitle{Kind: core.KindSeries, TmdbID: 603, Title: "Matrix Series", Tags: []string{"tmdb/1", "tmdb/2"}, CreatedAt: base.Add(time.Hour)}
		heat := core.BlockedTitle{Kind: core.KindMovie, TmdbID: 949, Title: "Heat", UserID: admin.ID, CreatedAt: base.Add(2 * time.Hour)}
		for _, b := range []*core.BlockedTitle{&matrix, &series, &heat} {
			if err := repo.Add(ctx, b); err != nil {
				t.Fatal(err)
			}
		}
		if err := repo.Add(ctx, &core.BlockedTitle{Kind: core.KindMovie, TmdbID: 603}); !errors.Is(err, core.ErrConflict) {
			t.Errorf("Add(twice) = %v, want = ErrConflict", err)
		}
		if err := repo.Add(ctx, &core.BlockedTitle{Kind: core.KindEpisode, TmdbID: 1}); !errors.Is(err, core.ErrInvalid) {
			t.Errorf("Add(episode) = %v, want = ErrInvalid", err)
		}
		got, err := repo.Get(ctx, core.KindMovie, 603)
		if err != nil || !reflect.DeepEqual(got, matrix) {
			t.Fatalf("Get() = %+v, %v, want = %+v", got, err, matrix)
		}
		if _, err := repo.Get(ctx, core.KindMovie, 550); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("Get(unknown) = %v, want = ErrNotFound", err)
		}

		lists := []struct {
			name  string
			q     core.BlockedTitleQuery
			want  []int
			total int
		}{
			{"all", core.BlockedTitleQuery{}, []int{949, 603, 603}, 3},
			{"page", core.BlockedTitleQuery{Limit: 1, Offset: 1}, []int{603}, 3},
			{"search", core.BlockedTitleQuery{Search: "matrix"}, []int{603, 603}, 2},
			{"by hand", core.BlockedTitleQuery{Source: core.BlockedByHand}, []int{949, 603}, 2},
			{"by tags", core.BlockedTitleQuery{Source: core.BlockedByTags}, []int{603}, 1},
		}
		for _, tt := range lists {
			page, err := repo.List(ctx, tt.q)
			if err != nil {
				t.Fatalf("List(%s) = %v", tt.name, err)
			}
			ids := make([]int, len(page.Items))
			for i, b := range page.Items {
				ids[i] = b.TmdbID
			}
			if !reflect.DeepEqual(ids, tt.want) || page.Total != tt.total {
				t.Errorf("List(%s) = %v (%d), want = %v (%d)", tt.name, ids, page.Total, tt.want, tt.total)
			}
		}

		if err := s.Users().Delete(ctx, admin.ID); err != nil {
			t.Fatal(err)
		}
		got, err = repo.Get(ctx, core.KindMovie, 949)
		if err != nil || got.UserID != (core.ID{}) {
			t.Errorf("Get(after deleting the user) = %+v, %v, want = no user", got, err)
		}

		if err := repo.Delete(ctx, core.KindMovie, 603); err != nil {
			t.Fatal(err)
		}
		if err := repo.Delete(ctx, core.KindMovie, 603); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("Delete(twice) = %v, want = ErrNotFound", err)
		}
		if got, err := repo.Get(ctx, core.KindSeries, 603); err != nil || !reflect.DeepEqual(got, series) {
			t.Errorf("Get(series) = %+v, %v, want = %+v", got, err, series)
		}
		if _, err := repo.List(ctx, core.BlockedTitleQuery{Source: "other"}); !errors.Is(err, core.ErrInvalid) {
			t.Errorf("List(unknown source) = %v, want = ErrInvalid", err)
		}
		if n, err := repo.DeleteTagged(ctx); err != nil || n != 1 {
			t.Errorf("DeleteTagged() = %d, %v, want = 1", n, err)
		}
		if _, err := repo.Get(ctx, core.KindSeries, 603); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("Get(series after DeleteTagged) = %v, want = ErrNotFound", err)
		}
		if _, err := repo.Get(ctx, core.KindMovie, 949); err != nil {
			t.Errorf("Get(by hand after DeleteTagged) = %v, want = kept", err)
		}
	})
}
