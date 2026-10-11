package store_test

import (
	"slices"
	"testing"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store"
)

func TestSimilarItems(t *testing.T) {
	eachBackend(t, func(t *testing.T, s *store.Store) {
		ctx := t.Context()
		lib := newLibrary(t, s, "/media")
		seed := newItem(lib, core.KindMovie, "Seed")
		seed.Genres, seed.Tags, seed.Studios = []string{"Drama", "Crime"}, []string{"heist"}, []string{"Warner"}
		sameDirector := newItem(lib, core.KindMovie, "Same Director")
		sameActor := newItem(lib, core.KindMovie, "Same Actor")
		sameActor.Genres = []string{"Comedy"}
		twoGenres := newItem(lib, core.KindMovie, "Two Genres")
		twoGenres.Genres = []string{"drama", "Crime"} // compared in clean form
		tagAndStudio := newItem(lib, core.KindMovie, "Tag And Studio")
		tagAndStudio.Tags, tagAndStudio.Studios = []string{"heist"}, []string{"Warner"}
		oneGenre := newItem(lib, core.KindMovie, "One Genre")
		oneGenre.Genres = []string{"Drama"}
		unrelated := newItem(lib, core.KindMovie, "Unrelated")
		unrelated.Genres = []string{"Animation"}
		upsert(t, s, seed, sameDirector, sameActor, twoGenres, tagAndStudio, oneGenre, unrelated)

		director := core.Person{ID: core.NewID(), Name: "Director"}
		actor := core.Person{ID: core.NewID(), Name: "Actor"}
		if err := s.People().Upsert(ctx, director, actor); err != nil {
			t.Fatal(err)
		}
		credit := func(it core.Item, p core.Person, kind core.CreditKind) {
			t.Helper()
			if err := s.People().ReplaceCredits(ctx, it.ID, []core.Credit{{ItemID: it.ID, PersonID: p.ID, Kind: kind}}); err != nil {
				t.Fatal(err)
			}
		}
		if err := s.People().ReplaceCredits(ctx, seed.ID, []core.Credit{
			{ItemID: seed.ID, PersonID: director.ID, Kind: core.CreditDirector},
			{ItemID: seed.ID, PersonID: actor.ID, Kind: core.CreditActor, Order: 1},
		}); err != nil {
			t.Fatal(err)
		}
		credit(sameDirector, director, core.CreditDirector) // 50
		credit(sameActor, actor, core.CreditActor)          // 15
		// twoGenres 20, tagAndStudio 10, oneGenre 10.

		got := query(t, s, core.ItemQuery{SimilarTo: seed.ID, Sort: []core.SortSpec{{Field: core.SortName}}})
		want := []string{"Same Director", "Two Genres", "Same Actor", "One Genre", "Tag And Studio"}
		if !slices.Equal(got, want) {
			t.Errorf("similar: got = %q, want = %q", got, want)
		}
		got = query(t, s, core.ItemQuery{SimilarTo: seed.ID, Genres: []string{"Comedy"}})
		if want := []string{"Same Actor"}; !slices.Equal(got, want) {
			t.Errorf("similar with a filter: got = %q, want = %q", got, want)
		}
	})
}
