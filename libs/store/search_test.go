package store_test

import (
	"errors"
	"fmt"
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
		a.Genres, a.Tags, a.ProductionYear, a.ParentalRating = []string{"Drama", "Science Fiction"}, []string{"4K"}, 1999, ptr(17)
		b.Genres, b.Studios, b.ProductionYear = []string{"drama", "Documentary", "Comédie"}, []string{"A24"}, 2015
		trailer := newItem(films, core.KindVideo, "A Trailer")
		trailer.Extra, trailer.OwnerID, trailer.Genres, trailer.ProductionYear = core.ExtraTrailer, a.ID, []string{"Drama"}, 1999
		song := newItem(music, core.KindTrack, "Song")
		song.Genres, song.Artists, song.AlbumArtists, song.ProductionYear = []string{"Dream Pop", "Melodrama", "Sci-Fi"}, []string{"Beach House"}, []string{"Various Artists", "Beach House"}, 2015
		upsert(t, s, a, b, trailer, song)

		films1 := core.ItemFilter{LibraryIDs: []core.ID{films.ID}}
		tests := []struct {
			name string
			q    core.ValueQuery
			want []string
		}{
			{"case variants merge, counted once per item", core.ValueQuery{Kind: core.ValueGenre, Items: films1}, []string{"Comédie 1", "Documentary 1", "Drama 2", "Science Fiction 1"}},
			{"all libraries", core.ValueQuery{Kind: core.ValueGenre, Limit: 3}, []string{"Comédie 1", "Documentary 1", "Drama 2"}},
			{"page", core.ValueQuery{Kind: core.ValueGenre, Limit: 2, Offset: 2}, []string{"Drama 2", "Dream Pop 1"}},
			{"search ranks prefix first", core.ValueQuery{Kind: core.ValueGenre, Search: "dr"}, []string{"Drama 2", "Dream Pop 1", "Melodrama 1"}},
			{"search ignores accents", core.ValueQuery{Kind: core.ValueGenre, Search: "comedie"}, []string{"Comédie 1"}},
			{"substring", core.ValueQuery{Kind: core.ValueGenre, Search: "fiction"}, []string{"Science Fiction 1"}},
			{"sort form", core.ValueQuery{Kind: core.ValueGenre, Search: "scifi"}, []string{"Sci-Fi 1"}},
			{"punctuation matches spaces", core.ValueQuery{Kind: core.ValueGenre, Search: "sci fi"}, []string{"Sci-Fi 1"}},
			{"rating", core.ValueQuery{Kind: core.ValueGenre, Items: core.ItemFilter{LibraryIDs: []core.ID{films.ID}, MaxRating: ptr(12)}}, []string{"Comédie 1", "Documentary 1", "drama 1"}},
			{"item kinds", core.ValueQuery{Kind: core.ValueGenre, Items: core.ItemFilter{Kinds: []core.ItemKind{core.KindTrack}}, Search: "dra"}, []string{"Melodrama 1"}},
			{"artists include album artists", core.ValueQuery{Kind: core.ValueArtist}, []string{"Beach House 1", "Various Artists 1"}},
			{"studios", core.ValueQuery{Kind: core.ValueStudio}, []string{"A24 1"}},
			{"tags", core.ValueQuery{Kind: core.ValueTag, Items: core.ItemFilter{LibraryIDs: []core.ID{music.ID}}}, []string{}},
			{"years", core.ValueQuery{Kind: core.ValueYear}, []string{"1999 1", "2015 2"}},
			{"years of a library", core.ValueQuery{Kind: core.ValueYear, Items: films1, Offset: 1}, []string{"2015 1"}},
		}
		for _, tt := range tests {
			got, err := s.Items().Values(ctx, tt.q)
			if err != nil {
				t.Fatalf("%s: %v", tt.name, err)
			}
			gotValues := []string{}
			for _, v := range got {
				gotValues = append(gotValues, fmt.Sprintf("%s %d", v.Value, v.Count))
			}
			if !slices.Equal(gotValues, tt.want) {
				t.Errorf("%s: got = %q, want = %q", tt.name, gotValues, tt.want)
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
		films, shows := newLibrary(t, s, "/films"), newLibrary(t, s, "/shows")
		movie, rated, episode := newItem(films, core.KindMovie, "Movie"), newItem(films, core.KindMovie, "Rated"), newItem(shows, core.KindEpisode, "Episode")
		rated.ParentalRating = ptr(17)
		upsert(t, s, movie, rated, episode)
		var list []core.Person
		for _, p := range []struct{ name, sort string }{
			{"Tom Hanks", "Hanks, Tom"},
			{"Tom Holland", ""},
			{"Atom Egoyan", ""},
			{"Thomas Newman", ""},
			{"Zoë Kravitz", ""},
			{"张国荣", ""},
			{"Uncredited", ""},
		} {
			list = append(list, core.Person{ID: core.NewID(), Name: p.name, SortName: p.sort})
		}
		if err := s.People().Upsert(ctx, list...); err != nil {
			t.Fatal(err)
		}
		// Everyone but the last is an actor in the movie; Tom Hanks also
		// directs it and acts in the rated film, Thomas Newman scores the
		// episode.
		var cast []core.Credit
		for i, p := range list[:6] {
			cast = append(cast, core.Credit{PersonID: p.ID, Kind: core.CreditActor, Order: i})
		}
		cast = append(cast, core.Credit{PersonID: list[0].ID, Kind: core.CreditDirector})
		for item, credits := range map[core.ID][]core.Credit{
			movie.ID:   cast,
			rated.ID:   {{PersonID: list[0].ID, Kind: core.CreditActor}},
			episode.ID: {{PersonID: list[3].ID, Kind: core.CreditComposer}},
		} {
			if err := s.People().ReplaceCredits(ctx, item, credits); err != nil {
				t.Fatal(err)
			}
		}

		tests := []struct {
			name string
			q    core.PersonQuery
			want []string
		}{
			{"credited people by sort name", core.PersonQuery{}, []string{"Atom Egoyan 1", "Tom Hanks 2", "Thomas Newman 2", "Tom Holland 1", "张国荣 1", "Zoë Kravitz 1"}},
			{"page", core.PersonQuery{Limit: 2, Offset: 1}, []string{"Tom Hanks 2", "Thomas Newman 2"}},
			{"word prefix first", core.PersonQuery{Search: "tom"}, []string{"Tom Hanks 2", "Tom Holland 1", "Atom Egoyan 1"}},
			{"accents", core.PersonQuery{Search: "zoe"}, []string{"Zoë Kravitz 1"}},
			{"sort name", core.PersonQuery{Search: "hanks tom"}, []string{"Tom Hanks 2"}},
			{"pinyin", core.PersonQuery{Search: "zhang guo"}, []string{"张国荣 1"}},
			{"library", core.PersonQuery{Items: core.ItemFilter{LibraryIDs: []core.ID{shows.ID}}}, []string{"Thomas Newman 1"}},
			{"rating", core.PersonQuery{Items: core.ItemFilter{MaxRating: ptr(12)}, Search: "hanks"}, []string{"Tom Hanks 1"}},
			{"credit kinds", core.PersonQuery{CreditKinds: []core.CreditKind{core.CreditDirector, core.CreditComposer}}, []string{"Tom Hanks 1", "Thomas Newman 1"}},
		}
		for _, tt := range tests {
			got, err := s.People().Search(ctx, tt.q)
			if err != nil {
				t.Fatalf("%s: %v", tt.name, err)
			}
			var gotNames []string
			for _, p := range got {
				gotNames = append(gotNames, fmt.Sprintf("%s %d", p.Person.Name, p.Count))
			}
			if !slices.Equal(gotNames, tt.want) {
				t.Errorf("%s: got = %q, want = %q", tt.name, gotNames, tt.want)
			}
		}
	})
}
