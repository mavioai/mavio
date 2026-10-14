package store_test

import (
	"slices"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store"
)

func TestPlayActivity(t *testing.T) {
	eachBackend(t, func(t *testing.T, s *store.Store) {
		ctx := t.Context()
		u := core.User{Name: "viewer", Kind: core.UserMember}
		if err := s.Users().Create(ctx, &u); err != nil {
			t.Fatal(err)
		}
		lib := newLibrary(t, s, "/media")
		watched, episodes := showTree(t, s, lib)
		fresh := newItem(lib, core.KindSeries, "Fresh")
		old := newItem(lib, core.KindMovie, "Old")
		recent := newItem(lib, core.KindMovie, "Recent")
		never := newItem(lib, core.KindMovie, "Never")
		upsert(t, s, fresh, old, recent, never)

		now := time.Now().UTC().Truncate(time.Second)
		play := func(it core.Item, ago time.Duration) {
			t.Helper()
			at := now.Add(-ago)
			if err := s.UserData().Put(ctx, &core.UserData{UserID: u.ID, ItemID: it.ID, Played: true, PlayCount: 1, LastPlayedAt: &at}); err != nil {
				t.Fatal(err)
			}
		}
		day := 24 * time.Hour
		play(episodes[0], 200*day)
		play(episodes[1], 300*day)
		play(old, 400*day)
		play(recent, 10*day)

		byName := []core.SortSpec{{Field: core.SortName}}
		tests := []struct {
			name string
			q    core.ItemQuery
			want []string
		}{
			{"started series", core.ItemQuery{Kinds: []core.ItemKind{core.KindSeries}, Started: ptr(true)}, []string{watched.Name}},
			{"series not begun", core.ItemQuery{Kinds: []core.ItemKind{core.KindSeries}, Started: ptr(false)}, []string{"Fresh"}},
			{"movies never played", core.ItemQuery{Kinds: []core.ItemKind{core.KindMovie}, Started: ptr(false)}, []string{"Never"}},
			{"series by its last episode", core.ItemQuery{Kinds: []core.ItemKind{core.KindSeries}, PlayedBefore: now.Add(-250 * day)}, nil},
			{"series watched long ago", core.ItemQuery{Kinds: []core.ItemKind{core.KindSeries}, PlayedBefore: now.Add(-180 * day)}, []string{watched.Name}},
			{"played over a year ago", core.ItemQuery{Kinds: []core.ItemKind{core.KindMovie}, PlayedBefore: now.Add(-365 * day)}, []string{"Old"}},
			{"played lately", core.ItemQuery{PlayedAfter: now.Add(-90 * day), Sort: byName}, []string{"Recent"}},
			{"played within a range", core.ItemQuery{Kinds: []core.ItemKind{core.KindEpisode, core.KindMovie}, PlayedAfter: now.Add(-350 * day), PlayedBefore: now.Add(-100 * day), Sort: byName}, []string{episodes[1].Name, episodes[0].Name}},
		}
		for _, tt := range tests {
			tt.q.UserID = u.ID
			if got := query(t, s, tt.q); !slices.Equal(got, tt.want) {
				t.Errorf("%s: got = %q, want = %q", tt.name, got, tt.want)
			}
		}
	})
}
