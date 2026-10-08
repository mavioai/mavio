package store_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store"
)

func TestItemSearch(t *testing.T) {
	eachBackend(t, func(t *testing.T, s *store.Store) {
		lib := newLibrary(t, s, "/media")
		var list []core.Item
		for _, name := range []string{"The Matrix", "Matrix Reloaded", "Animatrix", "Return of the Matrix Hunters", "Spider-Man", "Rocky 20", "Rocky 2", "霸王别姬", "100% Wolf", "Ip_Man"} {
			list = append(list, newItem(lib, core.KindMovie, name))
		}
		list[7].OriginalTitle = "Farewell My Concubine"
		upsert(t, s, list...)

		byName := []core.SortSpec{{Field: core.SortName}}
		tests := []struct {
			name string
			q    core.ItemQuery
			want []string
		}{
			{"prefix, then word, then substring", core.ItemQuery{Search: "matrix", Sort: byName}, []string{"Matrix Reloaded", "Return of the Matrix Hunters", "Animatrix", "The Matrix"}},
			{"exact match first, articles ignored in sort form", core.ItemQuery{Search: "the matrix", Sort: byName}, []string{"The Matrix", "Return of the Matrix Hunters", "Animatrix", "Matrix Reloaded"}},
			{"punctuation matches spaces", core.ItemQuery{Search: "spider man"}, []string{"Spider-Man"}},
			{"relevance precedes sort", core.ItemQuery{Search: "rocky 2", Sort: []core.SortSpec{{Field: core.SortName, Desc: true}}}, []string{"Rocky 2", "Rocky 20"}},
			{"punctuation ignored in sort form", core.ItemQuery{Search: "spiderman"}, []string{"Spider-Man"}},
			{"pinyin matches sort form", core.ItemQuery{Search: "Ba Wang Bie"}, []string{"霸王别姬"}},
			{"original title prefix", core.ItemQuery{Search: "farewell"}, []string{"霸王别姬"}},
			{"underscore wildcard in sort form", core.ItemQuery{Search: "m_trix rel"}, []string{"Matrix Reloaded"}},
			{"wildcard in original title", core.ItemQuery{Search: "fare%concubine"}, []string{"霸王别姬"}},
			{"wildcards are punctuation in the name", core.ItemQuery{Search: "%spider%man%"}, []string{"Spider-Man"}},
			{"trailing percent", core.ItemQuery{Search: "100%"}, []string{"100% Wolf"}},
			{"underscore is punctuation in the name", core.ItemQuery{Search: "ip_man"}, []string{"Ip_Man"}},
			{"punctuation only matches all", core.ItemQuery{Search: "%", Kinds: []core.ItemKind{core.KindMovie}, Limit: 1, Sort: byName}, []string{"100% Wolf"}},
			{"blank search matches all", core.ItemQuery{Search: "  ", Kinds: []core.ItemKind{core.KindMovie}, Limit: 1, Sort: byName}, []string{"100% Wolf"}},
		}
		for _, tt := range tests {
			if got := query(t, s, tt.q); !slices.Equal(got, tt.want) {
				t.Errorf("%s: got %q, want %q", tt.name, got, tt.want)
			}
		}
	})
}

func TestItemValues(t *testing.T) {
	eachBackend(t, func(t *testing.T, s *store.Store) {
		ctx := t.Context()
		films, music := newLibrary(t, s, "/films"), newLibrary(t, s, "/music")
		a, b := newItem(films, core.KindMovie, "A"), newItem(films, core.KindMovie, "B")
		a.Genres, a.Tags = []string{"Drama", "Science Fiction"}, []string{"4K"}
		b.Genres, b.Studios = []string{"drama", "Documentary", "Comédie"}, []string{"A24"}
		song := newItem(music, core.KindTrack, "Song")
		song.Genres, song.Artists, song.AlbumArtists = []string{"Dream Pop", "Melodrama", "Sci-Fi"}, []string{"Beach House"}, []string{"Various Artists"}
		upsert(t, s, a, b, song)

		tests := []struct {
			name string
			q    core.ValueQuery
			want []string
		}{
			{"case variants merge", core.ValueQuery{Kind: core.ValueGenre, LibraryIDs: []core.ID{films.ID}}, []string{"Comédie", "Documentary", "Drama", "Science Fiction"}},
			{"all libraries", core.ValueQuery{Kind: core.ValueGenre, Limit: 3}, []string{"Comédie", "Documentary", "Drama"}},
			{"search ranks prefix first", core.ValueQuery{Kind: core.ValueGenre, Search: "dr"}, []string{"Drama", "Dream Pop", "Melodrama"}},
			{"search ignores accents", core.ValueQuery{Kind: core.ValueGenre, Search: "comedie"}, []string{"Comédie"}},
			{"substring", core.ValueQuery{Kind: core.ValueGenre, Search: "fiction"}, []string{"Science Fiction"}},
			{"sort form", core.ValueQuery{Kind: core.ValueGenre, Search: "scifi"}, []string{"Sci-Fi"}},
			{"punctuation matches spaces", core.ValueQuery{Kind: core.ValueGenre, Search: "sci fi"}, []string{"Sci-Fi"}},
			{"artists include album artists", core.ValueQuery{Kind: core.ValueArtist}, []string{"Beach House", "Various Artists"}},
			{"studios", core.ValueQuery{Kind: core.ValueStudio}, []string{"A24"}},
			{"tags", core.ValueQuery{Kind: core.ValueTag, LibraryIDs: []core.ID{music.ID}}, []string{}},
		}
		for _, tt := range tests {
			got, err := s.Items().Values(ctx, tt.q)
			if err != nil {
				t.Fatalf("%s: %v", tt.name, err)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("%s: got %q, want %q", tt.name, got, tt.want)
			}
		}
		if _, err := s.Items().Values(ctx, core.ValueQuery{Kind: "mood"}); !errors.Is(err, core.ErrInvalid) {
			t.Errorf("unknown kind: %v, want ErrInvalid", err)
		}

		// Genre filters match case- and accent-insensitively.
		if got := query(t, s, core.ItemQuery{Genres: []string{"DRAMA"}, Sort: []core.SortSpec{{Field: core.SortName}}}); !slices.Equal(got, []string{"A", "B"}) {
			t.Errorf("genre filter: got %q", got)
		}
	})
}

func TestPersonSearch(t *testing.T) {
	eachBackend(t, func(t *testing.T, s *store.Store) {
		ctx := t.Context()
		var list []core.Person
		for _, p := range []struct{ name, sort string }{
			{"Tom Hanks", "Hanks, Tom"},
			{"Tom Holland", ""},
			{"Atom Egoyan", ""},
			{"Thomas Newman", ""},
			{"Zoë Kravitz", ""},
			{"张国荣", ""},
		} {
			list = append(list, core.Person{ID: core.NewID(), Name: p.name, SortName: p.sort})
		}
		if err := s.People().Upsert(ctx, list...); err != nil {
			t.Fatal(err)
		}
		tests := []struct {
			name string
			q    core.PersonQuery
			want []string
		}{
			{"all by sort name", core.PersonQuery{}, []string{"Atom Egoyan", "Tom Hanks", "Thomas Newman", "Tom Holland", "张国荣", "Zoë Kravitz"}},
			{"limit", core.PersonQuery{Limit: 2}, []string{"Atom Egoyan", "Tom Hanks"}},
			{"word prefix first", core.PersonQuery{Search: "tom"}, []string{"Tom Hanks", "Tom Holland", "Atom Egoyan"}},
			{"accents", core.PersonQuery{Search: "zoe"}, []string{"Zoë Kravitz"}},
			{"sort name", core.PersonQuery{Search: "hanks tom"}, []string{"Tom Hanks"}},
			{"pinyin", core.PersonQuery{Search: "zhang guo"}, []string{"张国荣"}},
		}
		for _, tt := range tests {
			got, err := s.People().Search(ctx, tt.q)
			if err != nil {
				t.Fatalf("%s: %v", tt.name, err)
			}
			var gotNames []string
			for _, p := range got {
				gotNames = append(gotNames, p.Name)
			}
			if !slices.Equal(gotNames, tt.want) {
				t.Errorf("%s: got %q, want %q", tt.name, gotNames, tt.want)
			}
		}
	})
}
