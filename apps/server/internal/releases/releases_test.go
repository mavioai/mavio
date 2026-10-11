package releases

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mavioai/mavio/libs/core"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1/pluginv1connect"
)

// items is an item repository of a few items.
type items struct {
	core.ItemRepository
	byID map[core.ID]core.Item
}

func (r items) Get(_ context.Context, id core.ID) (core.Item, error) {
	if it, ok := r.byID[id]; ok {
		return it, nil
	}
	return core.Item{}, fmt.Errorf("item %s: %w", id, core.ErrNotFound)
}

// Walk returns the children of q's parent of q's kinds.
func (r items) Walk(_ context.Context, q core.ItemQuery) iter.Seq2[core.Item, error] {
	return func(yield func(core.Item, error) bool) {
		for _, it := range r.byID {
			if it.ParentID == q.ParentID && slices.Contains(q.Kinds, it.Kind) && !yield(it, nil) {
				return
			}
		}
	}
}

// fakeProvider answers searches with the releases found for each request,
// keyed by describe, and records the requests.
type fakeProvider struct {
	pluginv1connect.IndexerProviderServiceClient
	indexers []*pluginv1.Indexer
	found    map[string][]*pluginv1.Release
	failure  string

	mu       sync.Mutex
	requests []string
	listed   int
}

func (p *fakeProvider) ListIndexers(context.Context, *pluginv1.ListIndexersRequest) (*pluginv1.ListIndexersResponse, error) {
	p.mu.Lock()
	p.listed++
	p.mu.Unlock()
	return pluginv1.ListIndexersResponse_builder{Indexers: p.indexers}.Build(), nil
}

func (p *fakeProvider) SearchReleases(_ context.Context, req *pluginv1.SearchReleasesRequest) (*pluginv1.SearchReleasesResponse, error) {
	key := describe(req)
	p.mu.Lock()
	p.requests = append(p.requests, key)
	p.mu.Unlock()
	if p.failure != "" {
		return pluginv1.SearchReleasesResponse_builder{Failures: []*pluginv1.IndexerFailure{
			pluginv1.IndexerFailure_builder{IndexerId: proto.String(req.GetIndexerIds()[0]), Message: &p.failure}.Build(),
		}}.Build(), nil
	}
	return pluginv1.SearchReleasesResponse_builder{Releases: p.found[key]}.Build(), nil
}

// describe writes a request as "indexer type q=… ids=… s=… e=… cat=…".
func describe(req *pluginv1.SearchReleasesRequest) string {
	var ids []string
	for k, v := range req.GetExternalIds() {
		ids = append(ids, k+":"+v)
	}
	slices.Sort(ids)
	s := fmt.Sprintf("%s %s q=%q ids=%s", strings.Join(req.GetIndexerIds(), ","),
		strings.TrimPrefix(req.GetType().String(), "INDEXER_SEARCH_TYPE_"), req.GetQuery(), strings.Join(ids, ","))
	if req.HasSeason() {
		s += fmt.Sprintf(" s=%d", req.GetSeason())
	}
	if req.GetEpisode() != "" {
		s += " e=" + req.GetEpisode()
	}
	if req.GetTitle() != "" {
		s += fmt.Sprintf(" title=%q", req.GetTitle())
	}
	if req.HasMinAge() || req.HasMaxAge() || req.HasMinSize() || req.HasMaxSize() {
		s += fmt.Sprintf(" age=%d-%d size=%d-%d", req.GetMinAge(), req.GetMaxAge(), req.GetMinSize(), req.GetMaxSize())
	}
	return s + fmt.Sprint(" cat=", req.GetCategories())
}

func caps(basic, tv, movie []string) *pluginv1.IndexerCapabilities {
	return pluginv1.IndexerCapabilities_builder{SearchParams: basic, TvSearchParams: tv, MovieSearchParams: movie}.Build()
}

func newIndexer(id string, priority int32, c *pluginv1.IndexerCapabilities) *pluginv1.Indexer {
	return pluginv1.Indexer_builder{Id: &id, Name: proto.String(strings.ToUpper(id)), Enabled: proto.Bool(true), Priority: &priority, Capabilities: c}.Build()
}

func rel(guid, title string, published time.Time, categories ...int32) *pluginv1.Release {
	return pluginv1.Release_builder{Guid: &guid, Title: &title, PublishDate: timestamppb.New(published), Categories: categories}.Build()
}

var (
	movieID                                                                            = core.NewID()
	seriesID                                                                           = core.NewID()
	seasonID                                                                           = core.NewID()
	episodeID, dailyID, animeID, dailySeriesID, animeSeriesID, animeSeasonID, artistID = core.NewID(), core.NewID(), core.NewID(), core.NewID(), core.NewID(), core.NewID(), core.NewID()
	animeNextID, animeUnairedID, animeUnnumberedID                                     = core.NewID(), core.NewID(), core.NewID()
)

func library() items {
	airDate := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)
	return items{byID: map[core.ID]core.Item{
		movieID: {
			ID: movieID, Kind: core.KindMovie, Name: "The Matrix", OriginalTitle: "The Matrix", ProductionYear: 1999,
			ExternalIDs: map[core.Provider]string{core.ProviderIMDb: "tt0133093", core.ProviderTMDB: "603"},
		},
		seriesID: {
			ID: seriesID, Kind: core.KindSeries, Name: "Marvel's Agents of S.H.I.E.L.D.",
			ExternalIDs: map[core.Provider]string{core.ProviderTVDB: "263365", core.ProviderIMDb: "tt2364582"},
		},
		seasonID:  {ID: seasonID, ParentID: seriesID, Kind: core.KindSeason, IndexNumber: new(2)},
		episodeID: {ID: episodeID, ParentID: seasonID, Kind: core.KindEpisode, IndexNumber: new(5), ParentIndexNumber: new(2)},
		dailySeriesID: {
			ID: dailySeriesID, Kind: core.KindSeries, Name: "The Daily Show", SeriesType: core.SeriesDaily,
			ExternalIDs: map[core.Provider]string{core.ProviderTVDB: "71256"},
		},
		dailyID: {ID: dailyID, ParentID: dailySeriesID, Kind: core.KindEpisode, IndexNumber: new(40), ParentIndexNumber: new(29), PremiereDate: &airDate},
		animeSeriesID: {
			ID: animeSeriesID, Kind: core.KindSeries, Name: "Café Enchanté", SeriesType: core.SeriesAnime,
			ExternalIDs: map[core.Provider]string{core.ProviderTVDB: "400001"},
		},
		animeSeasonID: {ID: animeSeasonID, ParentID: animeSeriesID, Kind: core.KindSeason, IndexNumber: new(1)},
		animeID:       {ID: animeID, ParentID: animeSeasonID, Kind: core.KindEpisode, IndexNumber: new(3), ParentIndexNumber: new(1), AbsoluteNumber: new(3), PremiereDate: new(airDate.AddDate(0, 0, 14))},
		animeNextID:   {ID: animeNextID, ParentID: animeSeasonID, Kind: core.KindEpisode, IndexNumber: new(4), ParentIndexNumber: new(1), AbsoluteNumber: new(4), PremiereDate: new(airDate.AddDate(0, 0, 21))},
		animeUnairedID: {
			ID: animeUnairedID, ParentID: animeSeasonID, Kind: core.KindEpisode, IndexNumber: new(5), ParentIndexNumber: new(1), AbsoluteNumber: new(5),
			PremiereDate: new(time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC)),
		},
		animeUnnumberedID: {ID: animeUnnumberedID, ParentID: animeSeasonID, Kind: core.KindEpisode, IndexNumber: new(6), ParentIndexNumber: new(1), PremiereDate: new(airDate)},
		artistID:          {ID: artistID, Kind: core.KindMusicArtist, Name: "Artist"},
	}}
}

func TestTiers(t *testing.T) {
	all := caps([]string{"q"}, []string{"q", "season", "ep", "tvdbid", "imdbid"}, []string{"q", "imdbid", "tmdbid"})
	textOnly := caps([]string{"q"}, []string{"q", "season", "ep"}, []string{"q"})
	none := caps(nil, nil, nil)
	for _, tt := range []struct {
		name string
		item core.ID
		caps *pluginv1.IndexerCapabilities
		want [][]string
	}{
		{"movie by IDs, then by title", movieID, all, [][]string{
			{` MOVIE q="" ids=imdb:tt0133093,tmdb:603 cat=[2000]`},
			{` UNSPECIFIED q="Matrix 1999" ids= cat=[2000]`},
		}},
		{"movie by title", movieID, textOnly, [][]string{nil, {` UNSPECIFIED q="Matrix 1999" ids= cat=[2000]`}}},
		{"movie not searchable", movieID, none, [][]string{nil, nil}},
		{"episode", episodeID, all, [][]string{
			{` TV q="" ids=imdb:tt2364582,tvdb:263365 s=2 e=5 cat=[5000]`},
			{` TV q="Marvels Agents of SHIELD" ids= s=2 e=5 cat=[5000]`},
		}},
		{"season", seasonID, textOnly, [][]string{nil, {` TV q="Marvels Agents of SHIELD" ids= s=2 cat=[5000]`}}},
		{"daily episode by air date", dailyID, all, [][]string{
			{` TV q="" ids=tvdb:71256 s=2024 e=03/01 cat=[5000]`},
			{` TV q="Daily Show" ids= s=2024 e=03/01 cat=[5000]`},
		}},
		{"anime by absolute number", animeID, all, [][]string{{
			` TV q="03" ids=tvdb:400001 cat=[5070]`,
			` UNSPECIFIED q="Cafe Enchante 03" ids= cat=[5070]`,
		}}},
		{"episode without season and ep", episodeID, caps([]string{"q"}, []string{"q", "tvdbid"}, nil), nil},
		{"episode by Newznab's title", episodeID, caps([]string{"q"}, []string{"q", "title", "season", "ep"}, nil), [][]string{
			nil, {` TV q="" ids= s=2 e=5 title="Marvel's Agents of S.H.I.E.L.D." cat=[5000]`},
		}},
		{"season by Newznab's title alone", seasonID, caps(nil, []string{"title", "season"}, nil), [][]string{
			nil, {` TV q="" ids= s=2 title="Marvel's Agents of S.H.I.E.L.D." cat=[5000]`},
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := &Searcher{Items: library()}
			c, err := s.criteria(t.Context(), Search{ItemID: tt.item})
			if err != nil {
				t.Fatal(err)
			}
			var got [][]string
			for _, tier := range c.tiers(tt.caps) {
				var reqs []string
				for _, r := range tier {
					reqs = append(reqs, describe(r))
				}
				got = append(got, reqs)
			}
			if fmt.Sprint(got) != fmt.Sprint(tt.want) {
				t.Errorf("tiers = %q, want = %q", got, tt.want)
			}
		})
	}
}

// TestEpisodeQueryTiers covers the requests of episode queries, as
// Sonarr's NewznabRequestGeneratorFixture expects them by search mode.
func TestEpisodeQueryTiers(t *testing.T) {
	all := caps([]string{"q"}, []string{"q", "season", "ep", "tvdbid", "imdbid"}, nil)
	for _, tt := range []struct {
		name string
		q    EpisodeQuery
		want [][]string
	}{
		{"single episode by scene numbers and titles", EpisodeQuery{SeriesID: seriesID, Season: 3, Episode: 1, Titles: []string{"Agents of SHIELD"}}, [][]string{
			{` TV q="" ids=imdb:tt2364582,tvdb:263365 s=3 e=1 cat=[5000]`},
			{` TV q="Agents of SHIELD" ids= s=3 e=1 cat=[5000]`},
		}},
		{"title only", EpisodeQuery{SeriesID: seriesID, Season: 3, Episode: 1, Mode: SearchByTitle, Titles: []string{"Scene Title"}}, [][]string{
			{` TV q="Scene Title" ids= s=3 e=1 cat=[5000]`}, nil,
		}},
		{"ID only", EpisodeQuery{Kind: QuerySeason, SeriesID: seriesID, Season: 2, Mode: SearchByID}, [][]string{
			{` TV q="" ids=imdb:tt2364582,tvdb:263365 s=2 cat=[5000]`}, nil,
		}},
		{"daily season", EpisodeQuery{Kind: QueryDailySeason, SeriesID: dailySeriesID, Year: 2024}, [][]string{
			{` TV q="" ids=tvdb:71256 s=2024 cat=[5000]`},
			{` TV q="Daily Show" ids= s=2024 cat=[5000]`},
		}},
		{"anime episode", EpisodeQuery{Kind: QueryAnimeEpisode, SeriesID: animeSeriesID, Absolute: 13}, [][]string{{
			` TV q="13" ids=tvdb:400001 cat=[5070]`,
			` UNSPECIFIED q="Cafe Enchante 13" ids= cat=[5070]`,
		}}},
		{"special", EpisodeQuery{Kind: QuerySpecial, SeriesID: seriesID, SpecialTitles: []string{"Agents of SHIELD Slingshot"}}, [][]string{{
			` UNSPECIFIED q="Agents of SHIELD Slingshot" ids= cat=[5000 5070]`,
		}}},
		// Sonarr's should_search_by_title_if_supported: scene titles as
		// they are.
		{"title taken", EpisodeQuery{SeriesID: seriesID, Season: 3, Episode: 1, Mode: SearchByTitle, Titles: []string{"Agents of S.H.I.E.L.D.", "Agents of S.H.I.E.L.D."}}, [][]string{
			{` TV q="" ids= s=3 e=1 title="Agents of S.H.I.E.L.D." cat=[5000]`}, nil,
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := &Searcher{Items: library()}
			c, err := s.criteria(t.Context(), Search{Episodes: &tt.q})
			if err != nil {
				t.Fatal(err)
			}
			tvCaps := all
			if strings.HasPrefix(tt.name, "title taken") {
				tvCaps = caps([]string{"q"}, []string{"q", "title", "season", "ep"}, nil)
			}
			var got [][]string
			for _, tier := range c.tiers(tvCaps) {
				var reqs []string
				for _, r := range tier {
					reqs = append(reqs, describe(r))
				}
				got = append(got, reqs)
			}
			if fmt.Sprint(got) != fmt.Sprint(tt.want) {
				t.Errorf("tiers = %q, want = %q", got, tt.want)
			}
		})
	}
}

func TestCriteriaErrors(t *testing.T) {
	s := &Searcher{Items: library()}
	for _, tt := range []struct {
		name string
		id   core.ID
		want error
	}{
		{"unknown item", core.NewID(), core.ErrNotFound},
		{"music", artistID, core.ErrInvalid},
		{"series", seriesID, core.ErrInvalid},
	} {
		if _, err := s.criteria(t.Context(), Search{ItemID: tt.id}); !errors.Is(err, tt.want) {
			t.Errorf("%s: err = %v, want = %v", tt.name, err, tt.want)
		}
	}
}

func TestCleanTitle(t *testing.T) {
	for _, tt := range []struct{ in, want string }{
		{"The Matrix", "Matrix"},
		{"Theodore Rex", "Theodore Rex"},
		{"Law & Order: Special Victims Unit", "Law and Order Special Victims Unit"},
		{"Marvel's Agents of S.H.I.E.L.D.", "Marvels Agents of SHIELD"},
		{"Amélie", "Amelie"},
		{"9-1-1: Lone Star", "9 1 1 Lone Star"},
		{"Shōgun", "Shogun"},
		{"進撃の巨人", "進撃の巨人"},
	} {
		if got := cleanTitle(tt.in); got != tt.want {
			t.Errorf("cleanTitle(%q) = %q, want = %q", tt.in, got, tt.want)
		}
	}
}

func TestSearch(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	all := caps([]string{"q"}, []string{"q", "season", "ep", "tvdbid"}, []string{"q", "imdbid"})
	byID := &fakeProvider{
		indexers: []*pluginv1.Indexer{newIndexer("ids", 10, all), newIndexer("off", 1, all)},
		found: map[string][]*pluginv1.Release{
			`ids MOVIE q="" ids=imdb:tt0133093 cat=[2000]`: {
				rel("a", "The.Matrix.1999.1080p.BluRay.x264-GROUP", now.Add(-time.Hour)),
				rel("b", "The.Matrix.1999.2160p.UHD.BluRay.REMUX.HDR.HEVC-OTHER", now),
				rel("a", "The.Matrix.1999.1080p.BluRay.x264-GROUP", now.Add(-time.Hour)),
			},
		},
	}
	byID.indexers[1].SetEnabled(false)
	byTitle := &fakeProvider{
		indexers: []*pluginv1.Indexer{newIndexer("text", 5, caps([]string{"q"}, nil, nil))},
		found: map[string][]*pluginv1.Release{
			`text UNSPECIFIED q="Matrix 1999" ids= cat=[2000]`: {rel("c", "The Matrix 1999 720p WEB-DL", now.Add(-2*time.Hour))},
		},
	}
	failing := &fakeProvider{indexers: []*pluginv1.Indexer{newIndexer("down", 1, all)}, failure: "HTTP 503"}
	s := &Searcher{Items: library(), Providers: func() []Provider {
		return []Provider{{ID: "p1", Client: byID}, {ID: "p2", Client: byTitle}, {ID: "p3", Client: failing}}
	}}

	res, err := s.Search(t.Context(), Search{ItemID: movieID})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, r := range res.Releases {
		got = append(got, fmt.Sprintf("%s/%s %s %s %d", r.Provider, r.GetIndexerId(), r.GetGuid(), r.Movie.Quality.Quality, r.Movie.Year))
	}
	want := []string{"p2/text c WEBDL-720p 1999", "p1/ids b Remux-2160p 1999", "p1/ids a Bluray-1080p 1999"}
	if !slices.Equal(got, want) {
		t.Errorf("releases = %q, want = %q", got, want)
	}
	if len(res.Failures) != 1 || res.Failures[0].Provider != "p3" || res.Failures[0].Indexer != "down" || res.Failures[0].Message != "HTTP 503" {
		t.Errorf("failures = %+v, want = the failure of p3/down", res.Failures)
	}
	// The indexer that found releases by ID was not searched by title, and
	// the disabled indexer not at all.
	if want := []string{`ids MOVIE q="" ids=imdb:tt0133093 cat=[2000]`}; !slices.Equal(byID.requests, want) {
		t.Errorf("requests of p1 = %q, want = %q", byID.requests, want)
	}

	// Asked for by name, a disabled indexer is searched; an unknown one
	// fails.
	byID.requests = nil
	res, err = s.Search(t.Context(), Search{
		Query: "matrix", Indexers: []Ref{{"p1", "off"}, {"p1", "gone"}},
		Type: pluginv1.IndexerSearchType_INDEXER_SEARCH_TYPE_MOVIE,
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{`off MOVIE q="matrix" ids= cat=[]`}; !slices.Equal(byID.requests, want) {
		t.Errorf("requests of p1 = %q, want = %q", byID.requests, want)
	}
	if len(res.Failures) != 1 || res.Failures[0].Indexer != "gone" {
		t.Errorf("failures = %+v, want = gone unknown", res.Failures)
	}
}

// TestAnimeSeasonSearch checks that a season of anime is searched episode
// by episode, as Sonarr's SearchAnimeSeason does: those aired with
// absolute numbers, each until a tier finds releases.
func TestAnimeSeasonSearch(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	p := &fakeProvider{
		indexers: []*pluginv1.Indexer{newIndexer("ix", 1, caps([]string{"q"}, []string{"q", "tvdbid"}, nil))},
		found: map[string][]*pluginv1.Release{
			`ix TV q="03" ids=tvdb:400001 cat=[5070]`:             {rel("a", "[Group] Cafe Enchante - 03 [1080p]", now)},
			`ix UNSPECIFIED q="Cafe Enchante 04" ids= cat=[5070]`: {rel("a", "[Group] Cafe Enchante - 03 [1080p]", now), rel("b", "[Group] Cafe Enchante - 04 [1080p]", now)},
		},
	}
	s := &Searcher{Items: library(), Now: func() time.Time { return now }, Providers: func() []Provider { return []Provider{{ID: "p", Client: p}} }}
	res, err := s.Search(t.Context(), Search{ItemID: animeSeasonID})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		`ix TV q="03" ids=tvdb:400001 cat=[5070]`, `ix UNSPECIFIED q="Cafe Enchante 03" ids= cat=[5070]`,
		`ix TV q="04" ids=tvdb:400001 cat=[5070]`, `ix UNSPECIFIED q="Cafe Enchante 04" ids= cat=[5070]`,
	}
	if !slices.Equal(p.requests, want) {
		t.Errorf("requests = %q, want = %q", p.requests, want)
	}
	var guids []string
	for _, r := range res.Releases {
		guids = append(guids, r.GetGuid())
	}
	if !slices.Equal(guids, []string{"a", "b"}) {
		t.Errorf("releases = %q, want = a and b once each", guids)
	}
}

// TestSearchLimits checks that a search's age and size limits go with its
// requests.
func TestSearchLimits(t *testing.T) {
	p := &fakeProvider{indexers: []*pluginv1.Indexer{newIndexer("ix", 1, caps([]string{"q"}, nil, []string{"q", "imdbid"}))}}
	s := &Searcher{Items: library(), Providers: func() []Provider { return []Provider{{ID: "p", Client: p}} }}
	if _, err := s.Search(t.Context(), Search{ItemID: movieID, MinAge: new(int32(1)), MaxSize: new(int64(5000))}); err != nil {
		t.Fatal(err)
	}
	want := []string{
		`ix MOVIE q="" ids=imdb:tt0133093 age=1-0 size=0-5000 cat=[2000]`,
		`ix UNSPECIFIED q="Matrix 1999" ids= age=1-0 size=0-5000 cat=[2000]`,
	}
	if !slices.Equal(p.requests, want) {
		t.Errorf("requests = %q, want = %q", p.requests, want)
	}
}

// TestIndexerCache checks that searches take the indexers the cache keeps
// while the provider keeps its revision, for an hour.
func TestIndexerCache(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	p := &fakeProvider{indexers: []*pluginv1.Indexer{newIndexer("ix", 1, caps([]string{"q"}, nil, nil))}}
	revision := uint64(1)
	s := &Searcher{
		Items: library(), Cache: &IndexerCache{Now: func() time.Time { return now }},
		Providers: func() []Provider { return []Provider{{ID: "p", Client: p, Revision: revision}} },
	}
	search := func() {
		t.Helper()
		if _, err := s.Search(t.Context(), Search{Query: "x"}); err != nil {
			t.Fatal(err)
		}
	}
	for _, step := range []struct {
		name   string
		do     func()
		listed int
	}{
		{"first search", search, 1},
		{"second search", search, 1},
		{"listing", func() { s.ListIndexers(t.Context()) }, 2},
		{"search after listing", search, 2},
		{"new revision", func() { revision++; search() }, 3},
		{"an hour later", func() { now = now.Add(time.Hour); search() }, 4},
	} {
		step.do()
		if p.listed != step.listed {
			t.Errorf("%s: listed = %d, want = %d", step.name, p.listed, step.listed)
		}
	}
	if len(p.requests) != 5 {
		t.Errorf("searches = %d, want = 5", len(p.requests))
	}
}

func TestParse(t *testing.T) {
	for _, tt := range []struct {
		title      string
		categories []int32
		want       string
	}{
		{"Show.Name.S01E02.720p.HDTV.x264-GRP", nil, "episode Show Name 1 [2]"},
		{"Movie.Name.2010.1080p.BluRay.x264-GRP", nil, "movie Movie Name 2010"},
		{"Movie.Name.2010.1080p.BluRay.x264-GRP", []int32{2040}, "movie Movie Name 2010"},
		{"Show.Name.S02.1080p.WEB-DL-GRP", []int32{5040}, "episode Show Name 2 []"},
	} {
		r := parse(Release{Release: rel("g", tt.title, time.Time{}, tt.categories...)}, 0)
		var got string
		switch {
		case r.Episode != nil:
			got = fmt.Sprintf("episode %s %d %v", r.Episode.SeriesTitle, r.Episode.SeasonNumber, r.Episode.EpisodeNumbers)
		case r.Movie != nil:
			got = fmt.Sprintf("movie %s %d", r.Movie.PrimaryMovieTitle(), r.Movie.Year)
		}
		if got != tt.want {
			t.Errorf("parse(%q, %v) = %q, want = %q", tt.title, tt.categories, got, tt.want)
		}
	}
}
