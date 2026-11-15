package store_test

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store"
)

func TestSearches(t *testing.T) {
	eachBackend(t, func(t *testing.T, s *store.Store) {
		ctx := t.Context()
		repo := s.Searches()
		base := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
		alice, bob := core.NewID(), core.NewID()
		rec := func(user core.ID, text string, at time.Time) core.SearchRecord {
			key := core.SearchKey(text)
			return core.SearchRecord{Key: key, Text: core.SearchText(text), Voter: core.SearchVoter(user, key), SearchedAt: at}
		}
		for _, r := range []core.SearchRecord{
			rec(alice, "Heat", base),
			rec(bob, "heat", base.Add(time.Hour)),
			rec(alice, "Up", base.Add(2*time.Hour)),
			// Alice searches Heat again, which replaces her record.
			rec(alice, "HEAT", base.Add(3*time.Hour)),
		} {
			if err := repo.Record(ctx, &r); err != nil {
				t.Fatal(err)
			}
		}
		if err := repo.Record(ctx, &core.SearchRecord{Key: "x", Text: "y", SearchedAt: base}); !errors.Is(err, core.ErrInvalid) {
			t.Errorf("Record(invalid) got = %v, want = ErrInvalid", err)
		}
		got, err := repo.List(ctx, time.Time{}, 0)
		if err != nil {
			t.Fatal(err)
		}
		want := []core.SearchRecord{rec(alice, "HEAT", base.Add(3*time.Hour)), rec(alice, "Up", base.Add(2*time.Hour)), rec(bob, "heat", base.Add(time.Hour))}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("List() got = %+v, want = %+v", got, want)
		}
		if got, err := repo.List(ctx, base.Add(90*time.Minute), 1); err != nil || len(got) != 1 || got[0].Text != "HEAT" {
			t.Errorf("List(since, 1) got = %+v, %v, want = HEAT", got, err)
		}

		// Records older than a time go, and then all but the newest kept.
		if err := repo.Record(ctx, new(rec(bob, "Lost", base.Add(-time.Hour)))); err != nil {
			t.Fatal(err)
		}
		n, err := repo.Prune(ctx, base, 2)
		if err != nil || n != 2 {
			t.Errorf("Prune() got = %d, %v, want = 2", n, err)
		}
		got, err = repo.List(ctx, time.Time{}, 0)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want[:2]) {
			t.Errorf("List() after Prune got = %+v, want = %+v", got, want[:2])
		}
		if n, err := repo.Prune(ctx, base, 5); err != nil || n != 0 {
			t.Errorf("Prune(nothing) got = %d, %v, want = 0", n, err)
		}
	})
}
