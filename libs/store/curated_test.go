package store_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store"
)

func TestCuratedItems(t *testing.T) {
	eachBackend(t, func(t *testing.T, s *store.Store) {
		ctx := t.Context()
		films := newLibrary(t, s, "/films")
		collections := core.Library{Name: "Collections", Kind: core.LibraryCollections}
		playlists := core.Library{Name: "Playlists", Kind: core.LibraryPlaylists}
		for _, lib := range []*core.Library{&collections, &playlists} {
			if err := s.Libraries().Create(ctx, lib); err != nil {
				t.Fatalf("create %s library: %v", lib.Kind, err)
			}
		}
		if err := s.Libraries().Create(ctx, &core.Library{Name: "More", Kind: core.LibraryCollections}); !errors.Is(err, core.ErrConflict) {
			t.Errorf("second collections library: %v, want ErrConflict", err)
		}

		alien, aliens, up := newItem(films, core.KindMovie, "Alien"), newItem(films, core.KindMovie, "Aliens"), newItem(films, core.KindMovie, "Up")
		aliens.ParentalRating = ptr(17)
		saga := newItem(collections, core.KindCollection, "Alien Collection")
		u := core.User{Name: "listener", PasswordHash: "x"}
		if err := s.Users().Create(ctx, &u); err != nil {
			t.Fatal(err)
		}
		mix := newItem(playlists, core.KindPlaylist, "Mix")
		mix.UserID = u.ID
		upsert(t, s, alien, aliens, up, saga, mix)
		if got, err := s.Items().Get(ctx, mix.ID); err != nil || got.UserID != u.ID {
			t.Errorf("playlist = %+v, %v", got, err)
		}

		// Entries keep their order, IDs and duplicates.
		entries := []core.Link{{ItemID: up.ID}, {ItemID: alien.ID}, {ItemID: up.ID}}
		if err := s.Items().ReplaceLinks(ctx, mix.ID, entries); err != nil {
			t.Fatal(err)
		}
		links, err := s.Items().Links(ctx, mix.ID)
		if err != nil || len(links) != 3 || links[0].ItemID != up.ID || links[1].ItemID != alien.ID || links[2].ItemID != up.ID ||
			links[0].ID == links[2].ID || links[0].ContainerID != mix.ID {
			t.Fatalf("links = %+v, %v", links, err)
		}
		// Moving an entry keeps its ID.
		moved := []core.Link{links[1], links[0], links[2]}
		if err := s.Items().ReplaceLinks(ctx, mix.ID, moved); err != nil {
			t.Fatal(err)
		}
		if links, err := s.Items().Links(ctx, mix.ID); err != nil || links[0].ID != moved[0].ID || links[1].ID != moved[1].ID {
			t.Errorf("moved links = %+v, %v", links, err)
		}

		if err := s.Items().ReplaceLinks(ctx, saga.ID, []core.Link{{ItemID: aliens.ID}, {ItemID: alien.ID}}); err != nil {
			t.Fatal(err)
		}
		listOrder := []core.SortSpec{{Field: core.SortListOrder}}
		tests := []struct {
			name string
			q    core.ItemQuery
			want []string
		}{
			{"members in list order", core.ItemQuery{MemberOf: saga.ID, Sort: listOrder}, []string{"Aliens", "Alien"}},
			{"members by name", core.ItemQuery{MemberOf: saga.ID, Sort: []core.SortSpec{{Field: core.SortName}}}, []string{"Alien", "Aliens"}},
			{"members within a rating", core.ItemQuery{MemberOf: saga.ID, MaxRating: ptr(12)}, []string{"Alien"}},
			{"playlist items once, by first entry", core.ItemQuery{MemberOf: mix.ID, Sort: []core.SortSpec{{Field: core.SortListOrder, Desc: true}}}, []string{"Up", "Alien"}},
			{"members are not children", core.ItemQuery{ParentID: saga.ID}, []string{}},
		}
		for _, tt := range tests {
			if got := query(t, s, tt.q); !slices.Equal(got, tt.want) {
				t.Errorf("%s: got = %q, want = %q", tt.name, got, tt.want)
			}
		}

		// Deleting an item removes its entries; deleting the user their
		// playlists.
		if err := s.Items().Delete(ctx, up.ID); err != nil {
			t.Fatal(err)
		}
		if links, err := s.Items().Links(ctx, mix.ID); err != nil || len(links) != 1 || links[0].ItemID != alien.ID {
			t.Errorf("links after deleting an item = %+v, %v", links, err)
		}
		if err := s.Users().Delete(ctx, u.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Items().Get(ctx, mix.ID); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("playlist after deleting its user: %v, want ErrNotFound", err)
		}
	})
}
