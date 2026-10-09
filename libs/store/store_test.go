package store_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store"
)

func ptr[T any](v T) *T { return &v }

func newLibrary(t *testing.T, s *store.Store, path string) core.Library {
	t.Helper()
	lib := core.Library{Name: "Library " + path, Kind: core.LibraryMixed, Paths: []string{path}}
	if err := s.Libraries().Create(t.Context(), &lib); err != nil {
		t.Fatalf("create library: %v", err)
	}
	return lib
}

func newItem(lib core.Library, kind core.ItemKind, name string) core.Item {
	return core.Item{ID: core.NewID(), LibraryID: lib.ID, Kind: kind, Name: name, DateAdded: time.Now()}
}

func upsert(t *testing.T, s *store.Store, items ...core.Item) {
	t.Helper()
	if err := s.Items().Upsert(t.Context(), items...); err != nil {
		t.Fatalf("upsert: %v", err)
	}
}

func names(items []core.Item) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.Name
	}
	return out
}

func query(t *testing.T, s *store.Store, q core.ItemQuery) []string {
	t.Helper()
	page, err := s.Items().Query(t.Context(), q)
	if err != nil {
		t.Fatalf("query %+v: %v", q, err)
	}
	return names(page.Items)
}

func TestLibraries(t *testing.T) {
	eachBackend(t, func(t *testing.T, s *store.Store) {
		ctx := t.Context()
		lib := core.Library{Name: "Movies", Kind: core.LibraryMovies, Paths: []string{"/media/movies/"}, ScanInterval: time.Hour, PreferredLanguage: "en"}
		if err := s.Libraries().Create(ctx, &lib); err != nil {
			t.Fatal(err)
		}
		if lib.ID.IsZero() || lib.CreatedAt.IsZero() || lib.Paths[0] != filepath.Clean("/media/movies") {
			t.Errorf("created library = %+v", lib)
		}
		got, err := s.Libraries().Get(ctx, lib.ID)
		if err != nil || got.Name != "Movies" || got.ScanInterval != time.Hour || got.PreferredLanguage != "en" {
			t.Errorf("Get = %+v, %v", got, err)
		}

		for _, p := range []string{"/media/movies", "/media/movies/4k", "/media"} {
			other := core.Library{Name: "Other", Kind: core.LibraryShows, Paths: []string{p}}
			if err := s.Libraries().Create(ctx, &other); !errors.Is(err, core.ErrConflict) {
				t.Errorf("overlapping path %s: error = %v, want ErrConflict", p, err)
			}
		}
		shows := core.Library{Name: "Shows", Kind: core.LibraryShows, Paths: []string{"/media/movies-old"}}
		if err := s.Libraries().Create(ctx, &shows); err != nil {
			t.Errorf("sibling path rejected: %v", err)
		}

		lib.Name = "Films"
		if err := s.Libraries().Update(ctx, &lib); err != nil || lib.Name != "Films" {
			t.Errorf("Update = %v", err)
		}
		list, err := s.Libraries().List(ctx)
		if err != nil || len(list) != 2 || list[0].Name != "Films" {
			t.Errorf("List = %+v, %v", list, err)
		}

		movie := newItem(lib, core.KindMovie, "Heat")
		upsert(t, s, movie)
		if err := s.Libraries().Delete(ctx, lib.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Items().Get(ctx, movie.ID); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("item after library delete: %v, want ErrNotFound", err)
		}
		if _, err := s.Libraries().Get(ctx, lib.ID); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("Get deleted library: %v", err)
		}
	})
}

func TestItemRoundTrip(t *testing.T) {
	eachBackend(t, func(t *testing.T, s *store.Store) {
		ctx := t.Context()
		lib := newLibrary(t, s, "/media")
		premiere := time.Date(1995, 12, 15, 0, 0, 0, 0, time.UTC)
		modified := time.Now().Truncate(time.Microsecond).UTC()
		movie := newItem(lib, core.KindMovie, "Heat")
		movie.OriginalTitle = "Heat"
		movie.Path = "/media/Heat (1995)/Heat.mkv"
		movie.ProductionYear = 1995
		movie.PremiereDate = &premiere
		movie.Runtime = 170 * time.Minute
		movie.OfficialRating = "R"
		movie.ParentalRating = 17
		movie.CommunityRating = 8.3
		movie.Genres = []string{"Crime", "Drama", "Action"}
		movie.Studios = []string{"Warner Bros."}
		movie.ExternalIDs = map[core.Provider]string{core.ProviderTMDB: "949", core.ProviderIMDb: "tt0113277"}
		movie.FileModified = modified
		upsert(t, s, movie)

		got, err := s.Items().Get(ctx, movie.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.SortName != "Heat" || !slices.Equal(got.Genres, movie.Genres) || !slices.Equal(got.Studios, movie.Studios) ||
			got.ExternalIDs[core.ProviderIMDb] != "tt0113277" || !got.PremiereDate.Equal(premiere) ||
			got.Runtime != movie.Runtime || got.ParentalRating != 17 || !got.FileModified.Equal(modified) {
			t.Errorf("Get = %+v", got)
		}

		// Metadata fields from NFO files round-trip.
		meta := newItem(lib, core.KindEpisode, "Pilot")
		meta.CustomRating = "TV-MA"
		meta.ProductionLocations = []string{"US", "CA"}
		meta.RemoteTrailers = []string{"https://www.youtube.com/watch?v=x"}
		meta.CollectionName = "Set"
		meta.AspectRatio = "2.35:1"
		meta.Video3DFormat = core.Video3DHalfSideBySide
		meta.Album = "Arrival"
		meta.AirDays = []time.Weekday{time.Friday, time.Sunday}
		meta.AirTime = "9 PM"
		meta.DisplayOrder = "dvd"
		meta.AirsBeforeSeasonNumber, meta.AirsAfterSeasonNumber, meta.AirsBeforeEpisodeNumber = ptr(3), ptr(2), ptr(1)
		meta.MetadataLanguage, meta.MetadataCountry = "en", "us"
		meta.Locked = true
		meta.LockedFields = []core.MetadataField{core.FieldCast, core.FieldOverview}
		upsert(t, s, meta)
		gotMeta, err := s.Items().Get(ctx, meta.ID)
		if err != nil {
			t.Fatal(err)
		}
		meta.SortName = "Pilot"
		meta.DateAdded, gotMeta.DateAdded = time.Time{}, time.Time{}
		if !reflect.DeepEqual(gotMeta, meta) {
			t.Errorf("metadata fields: got = %+v\nwant = %+v", gotMeta, meta)
		}

		if byPath, err := s.Items().GetByPath(ctx, lib.ID, movie.Path); err != nil || byPath.ID != movie.ID {
			t.Errorf("GetByPath = %v, %v", byPath.ID, err)
		}

		// Upsert replaces fields and multi-valued attributes.
		movie.Genres = []string{"Thriller"}
		movie.Overview = "A group of professional bank robbers…"
		upsert(t, s, movie)
		got, _ = s.Items().Get(ctx, movie.ID)
		if !slices.Equal(got.Genres, []string{"Thriller"}) || got.Overview != movie.Overview {
			t.Errorf("after re-upsert = %+v", got)
		}

		// A path belongs to one item per library.
		dup := newItem(lib, core.KindMovie, "Heat copy")
		dup.Path = movie.Path
		if err := s.Items().Upsert(ctx, dup); !errors.Is(err, core.ErrConflict) {
			t.Errorf("duplicate path: %v, want ErrConflict", err)
		}
		if err := s.Items().Upsert(ctx, core.Item{ID: core.NewID()}); !errors.Is(err, core.ErrInvalid) {
			t.Errorf("invalid item: %v, want ErrInvalid", err)
		}
	})
}

// showTree creates a series with two seasons of two episodes each.
func showTree(t *testing.T, s *store.Store, lib core.Library) (series core.Item, episodes []core.Item) {
	t.Helper()
	series = newItem(lib, core.KindSeries, "The Wire")
	upsert(t, s, series)
	for sn := 1; sn <= 2; sn++ {
		season := newItem(lib, core.KindSeason, fmt.Sprintf("Season %d", sn))
		season.ParentID = series.ID
		season.IndexNumber = ptr(sn)
		upsert(t, s, season)
		for en := 2; en >= 1; en-- {
			ep := newItem(lib, core.KindEpisode, fmt.Sprintf("S%02dE%02d", sn, en))
			ep.ParentID = season.ID
			ep.ParentIndexNumber = ptr(sn)
			ep.IndexNumber = ptr(en)
			upsert(t, s, ep)
			episodes = append(episodes, ep)
		}
	}
	return series, episodes
}

func TestItemQuery(t *testing.T) {
	eachBackend(t, func(t *testing.T, s *store.Store) {
		ctx := t.Context()
		lib := newLibrary(t, s, "/media")
		series, episodes := showTree(t, s, lib)

		movies := []core.Item{
			newItem(lib, core.KindMovie, "amélie"),
			newItem(lib, core.KindMovie, "Brazil"),
			newItem(lib, core.KindMovie, "千と千尋の神隠し"),
			newItem(lib, core.KindMovie, "Ｃａｒｏｌ"),
		}
		movies[0].Genres, movies[0].ProductionYear, movies[0].ParentalRating = []string{"Comedy", "Romance"}, 2001, 12
		movies[1].Genres, movies[1].ProductionYear, movies[1].ParentalRating = []string{"Drama"}, 1985, 15
		movies[2].Genres, movies[2].ProductionYear, movies[2].OriginalTitle = []string{"Animation"}, 2001, "Spirited Away"
		movies[3].ProductionYear, movies[3].ParentalRating = 2015, 15
		trailer := newItem(lib, core.KindVideo, "Brazil Trailer")
		trailer.Extra, trailer.OwnerID = core.ExtraTrailer, movies[1].ID
		upsert(t, s, append(movies, trailer)...)

		byName := []core.SortSpec{{Field: core.SortName}}
		moviesOnly := []core.ItemKind{core.KindMovie}
		tests := []struct {
			name string
			q    core.ItemQuery
			want []string
		}{
			{"by kind, case-insensitive name order", core.ItemQuery{Kinds: moviesOnly, Sort: byName}, []string{"amélie", "Brazil", "Ｃａｒｏｌ", "千と千尋の神隠し"}},
			{"descending", core.ItemQuery{Kinds: moviesOnly, Sort: []core.SortSpec{{Field: core.SortName, Desc: true}}}, []string{"千と千尋の神隠し", "Ｃａｒｏｌ", "Brazil", "amélie"}},
			{"children", core.ItemQuery{ParentID: series.ID, Sort: byName}, []string{"Season 1", "Season 2"}},
			{"descendants by index", core.ItemQuery{ParentID: series.ID, Recursive: true, Kinds: []core.ItemKind{core.KindEpisode}, Sort: []core.SortSpec{{Field: core.SortIndex}}}, []string{"S01E01", "S01E02", "S02E01", "S02E02"}},
			{"search ignores accents", core.ItemQuery{Search: "AMELIE"}, []string{"amélie"}},
			{"search full-width", core.ItemQuery{Search: "carol"}, []string{"Ｃａｒｏｌ"}},
			{"search CJK substring", core.ItemQuery{Search: "千尋"}, []string{"千と千尋の神隠し"}},
			{"search original title", core.ItemQuery{Search: "spirited"}, []string{"千と千尋の神隠し"}},
			{"extras excluded", core.ItemQuery{Search: "brazil"}, []string{"Brazil"}},
			{"extras included", core.ItemQuery{Search: "brazil", IncludeExtras: true, Sort: byName}, []string{"Brazil", "Brazil Trailer"}},
			{"genres any of", core.ItemQuery{Genres: []string{"Drama", "Animation"}, Sort: byName}, []string{"Brazil", "千と千尋の神隠し"}},
			{"year range", core.ItemQuery{Kinds: moviesOnly, YearFrom: 2000, YearTo: 2010, Sort: byName}, []string{"amélie", "千と千尋の神隠し"}},
			{"max rating includes unrated", core.ItemQuery{Kinds: moviesOnly, MaxRating: 12, Sort: byName}, []string{"amélie", "千と千尋の神隠し"}},
			{"max rating skips unrated", core.ItemQuery{Kinds: moviesOnly, MaxRating: 12, SkipUnrated: true}, []string{"amélie"}},
			{"year then name", core.ItemQuery{Kinds: moviesOnly, Sort: []core.SortSpec{{Field: core.SortProductionYear, Desc: true}, {Field: core.SortName}}}, []string{"Ｃａｒｏｌ", "amélie", "千と千尋の神隠し", "Brazil"}},
		}
		for _, tt := range tests {
			if got := query(t, s, tt.q); !slices.Equal(got, tt.want) {
				t.Errorf("%s: got %q, want %q", tt.name, got, tt.want)
			}
		}

		// Paging is stable and reports the total.
		page, err := s.Items().Query(ctx, core.ItemQuery{Kinds: moviesOnly, Sort: byName, Limit: 2, Offset: 1})
		if err != nil || page.Total != 4 || !slices.Equal(names(page.Items), []string{"Brazil", "Ｃａｒｏｌ"}) {
			t.Errorf("page = %q total %d, %v", names(page.Items), page.Total, err)
		}
		if _, err := s.Items().Query(ctx, core.ItemQuery{Limit: -1}); !errors.Is(err, core.ErrInvalid) {
			t.Errorf("invalid query: %v", err)
		}

		// Walk streams everything regardless of limits.
		var walked int
		for _, err := range s.Items().Walk(ctx, core.ItemQuery{LibraryIDs: []core.ID{lib.ID}, Limit: 1}) {
			if err != nil {
				t.Fatal(err)
			}
			walked++
		}
		if want := 1 + 2 + len(episodes) + len(movies); walked != want {
			t.Errorf("Walk yielded %d items, want %d", walked, want)
		}

		// Deleting the series removes seasons and episodes.
		if err := s.Items().Delete(ctx, series.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Items().Get(ctx, episodes[0].ID); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("episode after series delete: %v", err)
		}
		// Deleting a movie removes its extras.
		if err := s.Items().Delete(ctx, movies[1].ID); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Items().Get(ctx, trailer.ID); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("extra after owner delete: %v", err)
		}
	})
}

func TestUserScopedQueries(t *testing.T) {
	eachBackend(t, func(t *testing.T, s *store.Store) {
		ctx := t.Context()
		lib := newLibrary(t, s, "/media")
		a, b, c := newItem(lib, core.KindMovie, "A"), newItem(lib, core.KindMovie, "B"), newItem(lib, core.KindMovie, "C")
		upsert(t, s, a, b, c)
		alice := core.User{Name: "alice", PasswordHash: "$argon2id$x"}
		bob := core.User{Name: "bob", PasswordHash: "$argon2id$y"}
		for _, u := range []*core.User{&alice, &bob} {
			if err := s.Users().Create(ctx, u); err != nil {
				t.Fatal(err)
			}
		}
		earlier, later := time.Now().Add(-time.Hour), time.Now()
		for _, d := range []core.UserData{
			{UserID: alice.ID, ItemID: a.ID, Played: true, PlayCount: 3, LastPlayedAt: &earlier},
			{UserID: alice.ID, ItemID: b.ID, Position: 10 * time.Minute, Favorite: true, LastPlayedAt: &later, PlayCount: 1},
			{UserID: bob.ID, ItemID: c.ID, Played: true},
		} {
			if err := s.UserData().Put(ctx, &d); err != nil {
				t.Fatal(err)
			}
		}

		yes, no := true, false
		base := core.ItemQuery{UserID: alice.ID, Kinds: []core.ItemKind{core.KindMovie}, Sort: []core.SortSpec{{Field: core.SortName}}}
		with := func(f func(*core.ItemQuery)) core.ItemQuery { q := base; f(&q); return q }
		tests := []struct {
			name string
			q    core.ItemQuery
			want []string
		}{
			{"played", with(func(q *core.ItemQuery) { q.Played = &yes }), []string{"A"}},
			{"unplayed", with(func(q *core.ItemQuery) { q.Played = &no }), []string{"B", "C"}},
			{"favorite", with(func(q *core.ItemQuery) { q.Favorite = &yes }), []string{"B"}},
			{"resumable", with(func(q *core.ItemQuery) { q.Resumable = true }), []string{"B"}},
			{"last played", with(func(q *core.ItemQuery) { q.Sort = []core.SortSpec{{Field: core.SortLastPlayed, Desc: true}} }), []string{"B", "A", "C"}},
			{"play count", with(func(q *core.ItemQuery) { q.Sort = []core.SortSpec{{Field: core.SortPlayCount, Desc: true}} }), []string{"A", "B", "C"}},
		}
		for _, tt := range tests {
			if got := query(t, s, tt.q); !slices.Equal(got, tt.want) {
				t.Errorf("%s: got %q, want %q", tt.name, got, tt.want)
			}
		}

		got, err := s.UserData().Get(ctx, alice.ID, b.ID)
		if err != nil || got.Position != 10*time.Minute || !got.Favorite || got.LastPlayedAt == nil {
			t.Errorf("Get user data = %+v, %v", got, err)
		}
		if _, err := s.UserData().Get(ctx, bob.ID, a.ID); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("missing user data: %v", err)
		}
		// Put replaces the state, clearing unset optional fields.
		reset := core.UserData{UserID: bob.ID, ItemID: c.ID, AudioStream: new(1), SubtitleStream: new(2), Rating: new(7.0), LastPlayedAt: &later}
		if err := s.UserData().Put(ctx, &reset); err != nil {
			t.Fatal(err)
		}
		reset = core.UserData{UserID: bob.ID, ItemID: c.ID}
		if err := s.UserData().Put(ctx, &reset); err != nil {
			t.Fatal(err)
		}
		if got, err := s.UserData().Get(ctx, bob.ID, c.ID); err != nil || got.Played || got.AudioStream != nil || got.SubtitleStream != nil || got.Rating != nil || got.LastPlayedAt != nil {
			t.Errorf("after replacing with empty state = %+v, %v", got, err)
		}
		many, err := s.UserData().GetMany(ctx, alice.ID, []core.ID{a.ID, b.ID, c.ID})
		if err != nil || len(many) != 2 || !many[a.ID].Played {
			t.Errorf("GetMany = %v, %v", many, err)
		}
		// Deleting a user removes their state.
		if err := s.Users().Delete(ctx, alice.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := s.UserData().Get(ctx, alice.ID, a.ID); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("user data after user delete: %v", err)
		}
	})
}

func TestMediaSourcesAndImages(t *testing.T) {
	eachBackend(t, func(t *testing.T, s *store.Store) {
		ctx := t.Context()
		lib := newLibrary(t, s, "/media")
		movie := newItem(lib, core.KindMovie, "Dune")
		upsert(t, s, movie)

		sources := []core.MediaSource{
			{
				Path: "/media/Dune/Dune 4K.mkv", Name: "4K", Container: "matroska,webm", Size: 60 << 30, Duration: 155 * time.Minute,
				Streams: []core.MediaStream{
					{
						Index: 0, Kind: core.StreamVideo, Codec: "hevc", Width: 3840, Height: 1600, ColorTransfer: "smpte2084", ColorPrimaries: "bt2020", ColorSpace: "bt2020nc",
						FrameRate: core.Rational{Num: 24000, Den: 1001}, DolbyVision: &core.DolbyVision{Profile: 8, Level: 6, BLCompatibilityID: 1, RPUPresent: true, BLPresent: true},
					},
					{Index: 1, Kind: core.StreamAudio, Codec: "truehd", Channels: 8, Language: "eng", Default: true},
					{Index: 2, Kind: core.StreamSubtitle, Codec: "hdmv_pgs_subtitle", Language: "eng"},
				},
				Chapters:  []core.Chapter{{Start: 0, Title: "Opening"}, {Start: 5 * time.Minute, Title: "Arrakis"}},
				Keyframes: []time.Duration{0, 2 * time.Second, 4 * time.Second},
			},
			{Path: "/media/Dune/Dune 1080p.mkv", Name: "1080p", Container: "matroska,webm"},
		}
		if err := s.MediaSources().Replace(ctx, movie.ID, sources); err != nil {
			t.Fatal(err)
		}
		got, err := s.MediaSources().ListForItem(ctx, movie.ID)
		if err != nil || len(got) != 2 || got[0].Name != "4K" {
			t.Fatalf("ListForItem = %+v, %v", got, err)
		}
		v := got[0].Streams[0]
		if v.DolbyVision == nil || v.DolbyVision.Profile != 8 || v.VideoRangeType() != core.RangeTypeDOVIWithHDR10 || v.FrameRate.Den != 1001 || len(got[0].Chapters) != 2 || len(got[0].Keyframes) != 3 {
			t.Errorf("source round trip = %+v", got[0])
		}
		if err := s.MediaSources().Replace(ctx, movie.ID, sources[1:]); err != nil {
			t.Fatal(err)
		}
		if got, _ := s.MediaSources().ListForItem(ctx, movie.ID); len(got) != 1 || got[0].Name != "1080p" || got[0].Keyframes != nil {
			t.Errorf("after Replace = %+v", got)
		}
		// Keyframes read from a file without any differ from none read.
		sources[1].Keyframes = []time.Duration{}
		if err := s.MediaSources().Replace(ctx, movie.ID, sources[1:]); err != nil {
			t.Fatal(err)
		}
		if got, _ := s.MediaSources().ListForItem(ctx, movie.ID); len(got) != 1 || got[0].Keyframes == nil || len(got[0].Keyframes) != 0 {
			t.Errorf("empty keyframes round trip = %#v", got[0].Keyframes)
		}

		person := core.Person{ID: core.NewID(), Name: "Denis Villeneuve"}
		if err := s.People().Upsert(ctx, person); err != nil {
			t.Fatal(err)
		}
		for _, owner := range []core.ID{movie.ID, person.ID} {
			imgs := []core.Image{
				{Kind: core.ImagePrimary, Path: "/cache/a.webp", Width: 400, Height: 600, Blurhash: "LEHV6nWB2yk8", Thumbhash: []byte{1, 2, 3}},
				{Kind: core.ImageBackdrop, Index: 1, RemoteURL: "https://example.com/b.jpg"},
			}
			if err := s.Images().Replace(ctx, owner, imgs); err != nil {
				t.Fatal(err)
			}
			got, err := s.Images().ListForOwner(ctx, owner)
			if err != nil || len(got) != 2 {
				t.Fatalf("images of %s = %+v, %v", owner, got, err)
			}
			// Ordered by kind, then index.
			backdrop, primary := got[0], got[1]
			if backdrop.Kind != core.ImageBackdrop || backdrop.RemoteURL != "https://example.com/b.jpg" || backdrop.OwnerID != owner ||
				primary.Kind != core.ImagePrimary || primary.Width != 400 || !slices.Equal(primary.Thumbhash, []byte{1, 2, 3}) {
				t.Errorf("images of %s = %+v", owner, got)
			}
		}
		if err := s.Images().Replace(ctx, core.NewID(), nil); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("images for unknown owner: %v", err)
		}

		// Deleting the item removes its sources and images.
		if err := s.Items().Delete(ctx, movie.ID); err != nil {
			t.Fatal(err)
		}
		if got, _ := s.MediaSources().ListForItem(ctx, movie.ID); len(got) != 0 {
			t.Errorf("sources after delete = %d", len(got))
		}
		if got, _ := s.Images().ListForOwner(ctx, movie.ID); len(got) != 0 {
			t.Errorf("images after delete = %d", len(got))
		}
	})
}

func TestPeopleAndCredits(t *testing.T) {
	eachBackend(t, func(t *testing.T, s *store.Store) {
		ctx := t.Context()
		lib := newLibrary(t, s, "/media")
		heat, ronin := newItem(lib, core.KindMovie, "Heat"), newItem(lib, core.KindMovie, "Ronin")
		upsert(t, s, heat, ronin)
		deNiro := core.Person{ID: core.NewID(), Name: "Robert De Niro", ExternalIDs: map[core.Provider]string{core.ProviderTMDB: "380"}}
		pacino := core.Person{ID: core.NewID(), Name: "Al Pacino"}
		if err := s.People().Upsert(ctx, deNiro, pacino); err != nil {
			t.Fatal(err)
		}
		if p, err := s.People().FindByName(ctx, "robert de niro"); err != nil || p.ID != deNiro.ID || p.ExternalIDs[core.ProviderTMDB] != "380" {
			t.Errorf("FindByName = %+v, %v", p, err)
		}
		credits := []core.Credit{
			{PersonID: pacino.ID, Kind: core.CreditActor, Role: "Vincent Hanna", Order: 0},
			{PersonID: deNiro.ID, Kind: core.CreditActor, Role: "Neil McCauley", Order: 1},
		}
		if err := s.People().ReplaceCredits(ctx, heat.ID, credits); err != nil {
			t.Fatal(err)
		}
		if err := s.People().ReplaceCredits(ctx, ronin.ID, []core.Credit{{PersonID: deNiro.ID, Kind: core.CreditActor, Role: "Sam"}}); err != nil {
			t.Fatal(err)
		}
		got, err := s.People().CreditsForItem(ctx, heat.ID)
		if err != nil || len(got) != 2 || got[0].Role != "Vincent Hanna" {
			t.Errorf("CreditsForItem = %+v, %v", got, err)
		}
		if got := query(t, s, core.ItemQuery{PersonID: deNiro.ID, Sort: []core.SortSpec{{Field: core.SortName}}}); !slices.Equal(got, []string{"Heat", "Ronin"}) {
			t.Errorf("items of person = %q", got)
		}
	})
}

func TestUsers(t *testing.T) {
	eachBackend(t, func(t *testing.T, s *store.Store) {
		ctx := t.Context()
		lib := newLibrary(t, s, "/media")
		u := core.User{
			Name: "Alice", PasswordHash: "$argon2id$x", Admin: true,
			Policy:      core.UserPolicy{Libraries: []core.ID{lib.ID}, MaxParentalRating: 13, AllowTranscoding: true},
			Preferences: core.UserPreferences{AudioLanguages: []string{"jpn", "eng"}, SubtitleMode: core.SubtitlesSmart},
		}
		if err := s.Users().Create(ctx, &u); err != nil {
			t.Fatal(err)
		}
		got, err := s.Users().GetByName(ctx, "ALICE")
		if err != nil || got.ID != u.ID || !got.Admin || got.Policy.MaxParentalRating != 13 ||
			!slices.Equal(got.Preferences.AudioLanguages, []string{"jpn", "eng"}) || !got.Policy.CanAccessLibrary(lib.ID) {
			t.Errorf("GetByName = %+v, %v", got, err)
		}
		dup := core.User{Name: "alice", AuthProvider: "ldap"}
		if err := s.Users().Create(ctx, &dup); !errors.Is(err, core.ErrConflict) {
			t.Errorf("duplicate name: %v, want ErrConflict", err)
		}
		login := time.Now()
		u.Disabled, u.LastLoginAt = true, &login
		if err := s.Users().Update(ctx, &u); err != nil || !u.Disabled || u.LastLoginAt == nil {
			t.Errorf("Update = %+v, %v", u, err)
		}
		if list, err := s.Users().List(ctx); err != nil || len(list) != 1 {
			t.Errorf("List = %v, %v", list, err)
		}
		if _, err := s.Users().Get(ctx, core.NewID()); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("unknown user: %v", err)
		}
	})
}

func TestTransactions(t *testing.T) {
	eachBackend(t, func(t *testing.T, s *store.Store) {
		ctx := t.Context()
		lib := newLibrary(t, s, "/media")
		committed, rolledBack := newItem(lib, core.KindMovie, "Committed"), newItem(lib, core.KindMovie, "Rolled back")
		if err := s.InTx(ctx, func(tx core.Store) error { return tx.Items().Upsert(ctx, committed) }); err != nil {
			t.Fatal(err)
		}
		boom := errors.New("boom")
		err := s.InTx(ctx, func(tx core.Store) error {
			if err := tx.Items().Upsert(ctx, rolledBack); err != nil {
				return err
			}
			if _, err := tx.Jobs().Enqueue(ctx, &core.Job{Kind: "x", MaxAttempts: 1}); err != nil {
				return err
			}
			return boom
		})
		if !errors.Is(err, boom) {
			t.Fatalf("InTx error = %v", err)
		}
		if _, err := s.Items().Get(ctx, committed.ID); err != nil {
			t.Errorf("committed item: %v", err)
		}
		if _, err := s.Items().Get(ctx, rolledBack.ID); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("rolled back item: %v", err)
		}
		if _, err := s.Jobs().Lease(ctx, "w", []string{"x"}, time.Minute); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("job enqueued in rolled back transaction: %v", err)
		}
	})
}

func TestJobQueue(t *testing.T) {
	eachBackend(t, func(t *testing.T, s *store.Store) {
		ctx := t.Context()
		q := s.Jobs()
		enqueue := func(j core.Job) core.Job {
			t.Helper()
			if j.MaxAttempts == 0 {
				j.MaxAttempts = 3
			}
			added, err := q.Enqueue(ctx, &j)
			if err != nil || !added {
				t.Fatalf("Enqueue(%s) = %v, %v", j.Kind, added, err)
			}
			return j
		}

		low := enqueue(core.Job{Kind: "scan", Priority: 0, UniqueKey: "scan:lib1"})
		high := enqueue(core.Job{Kind: "scan", Priority: 5})
		enqueue(core.Job{Kind: "scan", RunAt: time.Now().Add(time.Hour)}) // not due
		enqueue(core.Job{Kind: "other"})

		if added, err := q.Enqueue(ctx, &core.Job{Kind: "scan", UniqueKey: "scan:lib1", MaxAttempts: 1}); err != nil || added {
			t.Errorf("duplicate unique key: added=%v err=%v", added, err)
		}

		j, err := q.Lease(ctx, "w1", []string{"scan"}, time.Minute)
		if err != nil || j.ID != high.ID || j.State != core.JobRunning || j.Attempts != 1 || j.LeaseOwner != "w1" {
			t.Fatalf("first lease = %+v, %v; want the high-priority job", j, err)
		}
		if j2, err := q.Lease(ctx, "w2", []string{"scan"}, time.Minute); err != nil || j2.ID != low.ID {
			t.Fatalf("second lease = %+v, %v", j2, err)
		}
		if _, err := q.Lease(ctx, "w3", []string{"scan"}, time.Minute); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("third lease: %v, want ErrNotFound (remaining job not due)", err)
		}
		// Workers lease the kinds they handle at once.
		if _, err := q.Lease(ctx, "w3", []string{"probe", "scan", "refresh"}, time.Minute); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("lease of several kinds: %v, want ErrNotFound (remaining job not due)", err)
		}
		if j, err := q.Lease(ctx, "w3", []string{"probe", "other", "refresh"}, time.Minute); err != nil || j.Kind != "other" {
			t.Errorf("lease of several kinds = %+v, %v, want = the other job", j, err)
		}

		// While running, the unique key still deduplicates.
		if added, _ := q.Enqueue(ctx, &core.Job{Kind: "scan", UniqueKey: "scan:lib1", MaxAttempts: 1}); added {
			t.Error("duplicate of a running job was enqueued")
		}
		if err := q.Complete(ctx, high.ID, "intruder"); !errors.Is(err, core.ErrConflict) {
			t.Errorf("complete by non-owner: %v, want ErrConflict", err)
		}
		if err := q.Extend(ctx, high.ID, "w1", time.Minute); err != nil {
			t.Errorf("Extend: %v", err)
		}
		if err := q.Complete(ctx, high.ID, "w1"); err != nil {
			t.Errorf("Complete: %v", err)
		}
		if err := q.Complete(ctx, high.ID, "w1"); !errors.Is(err, core.ErrConflict) {
			t.Errorf("complete twice: %v, want ErrConflict", err)
		}

		// A failure schedules a retry with backoff; the key is free again
		// only after the job finishes for good.
		if err := q.Fail(ctx, low.ID, "w2", errors.New("disk offline")); err != nil {
			t.Fatal(err)
		}
		if _, err := q.Lease(ctx, "w2", []string{"scan"}, time.Minute); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("lease during backoff: %v, want ErrNotFound", err)
		}
	})
}

func TestJobRetriesAndExpiry(t *testing.T) {
	eachBackend(t, func(t *testing.T, s *store.Store) {
		ctx := t.Context()
		q := s.Jobs()
		job := core.Job{Kind: "refresh", MaxAttempts: 1, UniqueKey: "refresh:1"}
		if _, err := q.Enqueue(ctx, &job); err != nil {
			t.Fatal(err)
		}

		// An expired lease makes the job available to another worker.
		if _, err := q.Lease(ctx, "crashed", []string{"refresh"}, 10*time.Millisecond); err != nil {
			t.Fatal(err)
		}
		time.Sleep(30 * time.Millisecond)
		j, err := q.Lease(ctx, "w2", []string{"refresh"}, time.Minute)
		if err != nil || j.ID != job.ID || j.Attempts != 2 {
			t.Fatalf("re-lease after expiry = %+v, %v", j, err)
		}
		if err := q.Complete(ctx, job.ID, "crashed"); !errors.Is(err, core.ErrConflict) {
			t.Errorf("stale owner completed the job: %v", err)
		}

		// Attempts exhausted: the job fails for good and frees its key.
		if err := q.Fail(ctx, job.ID, "w2", errors.New("bad metadata")); err != nil {
			t.Fatal(err)
		}
		if added, err := q.Enqueue(ctx, &core.Job{Kind: "refresh", UniqueKey: "refresh:1", MaxAttempts: 1}); err != nil || !added {
			t.Errorf("enqueue after failure: added=%v err=%v", added, err)
		}
	})
}

func TestConcurrentLeases(t *testing.T) {
	eachBackend(t, func(t *testing.T, s *store.Store) {
		ctx := context.Background()
		const n = 20
		for range n {
			if _, err := s.Jobs().Enqueue(ctx, &core.Job{Kind: "work", MaxAttempts: 1}); err != nil {
				t.Fatal(err)
			}
		}
		var (
			mu     sync.Mutex
			leased = map[core.ID]string{}
			wg     sync.WaitGroup
		)
		for w := range 5 {
			wg.Go(func() {
				owner := fmt.Sprintf("w%d", w)
				for {
					j, err := s.Jobs().Lease(ctx, owner, []string{"work"}, time.Minute)
					if errors.Is(err, core.ErrNotFound) {
						return
					}
					if err != nil {
						t.Errorf("lease: %v", err)
						return
					}
					mu.Lock()
					if prev, dup := leased[j.ID]; dup {
						t.Errorf("job %s leased by %s and %s", j.ID, prev, owner)
					}
					leased[j.ID] = owner
					mu.Unlock()
				}
			})
		}
		wg.Wait()
		if len(leased) != n {
			t.Errorf("leased %d jobs, want %d", len(leased), n)
		}
	})
}

func TestReopenSQLite(t *testing.T) {
	ctx := t.Context()
	dsn := "sqlite:" + t.TempDir() + "/mavio.db"
	s, err := store.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	lib := newLibrary(t, s, "/media")
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	// Migrations already applied are skipped; data survives.
	s, err = store.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s.Close()
	if _, err := s.Libraries().Get(ctx, lib.ID); err != nil {
		t.Errorf("library after reopen: %v", err)
	}
	if s.Dialect() != store.DialectSQLite {
		t.Errorf("Dialect() = %q", s.Dialect())
	}
}

func TestOpenRejectsUnknownDSN(t *testing.T) {
	if _, err := store.Open(t.Context(), "mysql://localhost/mavio"); !errors.Is(err, core.ErrInvalid) {
		t.Errorf("Open(mysql) error = %v, want ErrInvalid", err)
	}
}
