package main

import (
	"fmt"
	"net/url"
	"slices"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

var seriesRoutes = map[string]string{
	"GET /skyhook/shows/en/81189": breakingBad,
	"GET /skyhook/search/en/": `[
		{"tvdbId": 1, "title": "Breaking Bad Habits", "firstAired": "2020-01-01"},
		{"tvdbId": 81189, "title": "Breaking Bad", "firstAired": "2008-01-20", "tmdbId": 1396, "imdbId": "tt0903747"}]`,
	"GET /skyhook/search/en/?term=imdb%3Att0903747": `[{"tvdbId": 81189, "title": "Breaking Bad"}]`,
	"GET /skyhook/search/en/?term=tmdb%3A1396":      `[{"tvdbId": 81189, "title": "Breaking Bad"}]`,
}

func TestSearchSeries(t *testing.T) {
	tests := []struct {
		name      string
		lookup    *pluginv1.Lookup
		wantIDs   []string
		wantTerms []string
	}{
		{"by title", lookup(pluginv1.MediaKind_MEDIA_KIND_SERIES, " Breaking Bad ", 2008, nil), []string{"81189", "1"}, []string{"breaking bad"}},
		{"by TVDB ID", lookup(pluginv1.MediaKind_MEDIA_KIND_SERIES, "x", 0, map[string]string{"tvdb": "81189"}), []string{"81189"}, nil},
		{"by IMDb ID", lookup(pluginv1.MediaKind_MEDIA_KIND_SERIES, "x", 0, map[string]string{"imdb": "tt0903747"}), []string{"81189"}, []string{"imdb:tt0903747"}},
		{"by TMDB ID", lookup(pluginv1.MediaKind_MEDIA_KIND_SERIES, "x", 0, map[string]string{"tmdb": "1396"}), []string{"81189"}, []string{"tmdb:1396"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, f := newTestPlugin(t, seriesRoutes)
			resp, err := p.Search(t.Context(), pluginv1.SearchRequest_builder{Lookup: tt.lookup}.Build())
			if err != nil {
				t.Fatal(err)
			}
			var ids []string
			for _, r := range resp.GetResults() {
				ids = append(ids, r.GetExternalIds()["tvdb"])
			}
			if !slices.Equal(ids, tt.wantIDs) {
				t.Errorf("results = %v, want = %v", ids, tt.wantIDs)
			}
			var terms []string
			for _, r := range f.requests {
				if r.path == "/skyhook/search/en/" {
					terms = append(terms, r.query)
				}
			}
			var want []string
			for _, term := range tt.wantTerms {
				want = append(want, url.Values{"term": {term}}.Encode())
			}
			if !slices.Equal(terms, want) {
				t.Errorf("search queries = %v, want = %v", terms, want)
			}
		})
	}
}

func TestSeriesMetadata(t *testing.T) {
	p, f := newTestPlugin(t, seriesRoutes)
	get := func(l *pluginv1.Lookup) *pluginv1.Metadata {
		t.Helper()
		resp, err := p.GetMetadata(t.Context(), pluginv1.GetMetadataRequest_builder{Lookup: l}.Build())
		if err != nil {
			t.Fatal(err)
		}
		if !resp.GetFound() {
			return nil
		}
		return resp.GetMetadata()
	}
	series := map[string]string{"tvdb": "81189"}
	md := get(lookup(pluginv1.MediaKind_MEDIA_KIND_SERIES, "", 0, series))
	checks := []struct {
		field     string
		got, want any
	}{
		{"name", md.GetName(), "Breaking Bad"},
		{"premiere date", md.GetPremiereDate(), "2008-01-20"},
		{"end date", md.GetEndDate(), "2013-09-29"},
		{"production year", md.GetProductionYear(), int32(2008)},
		{"runtime", md.GetRuntime().AsDuration(), 48 * time.Minute},
		{"official rating", md.GetOfficialRating(), "TV-MA"},
		{"community rating", md.GetCommunityRating(), 9.5},
		{"status", md.GetSeriesStatus(), pluginv1.SeriesStatus_SERIES_STATUS_ENDED},
		{"studios", md.GetStudios(), []string{"AMC"}},
		{"genres", md.GetGenres(), []string{"Crime", "Drama"}},
		{"external IDs", md.GetExternalIds(), map[string]string{"tvdb": "81189", "imdb": "tt0903747", "tmdb": "1396", "tvrage": "18164", "tvmaze": "169"}},
		{"actors", len(md.GetPeople()), 2},
		{"actor", md.GetPeople()[0].GetRole() + " " + md.GetPeople()[0].GetImageUrl(), "Walter White https://artworks.thetvdb.com/banners/person/1.jpg"},
		{"images", len(md.GetImages()), 2},
		{"banner", md.GetImages()[0].GetKind(), pluginv1.ImageKind_IMAGE_KIND_BANNER},
	}
	for _, c := range checks {
		if !equal(c.got, c.want) {
			t.Errorf("%s = %v, want = %v", c.field, c.got, c.want)
		}
	}

	season := func(n int32) *pluginv1.Lookup {
		return pluginv1.Lookup_builder{Kind: pluginv1.MediaKind_MEDIA_KIND_SEASON.Enum(), SeriesExternalIds: series, SeasonNumber: proto.Int32(n)}.Build()
	}
	if md := get(season(1)); md == nil || md.GetPremiereDate() != "2008-01-20" || len(md.GetImages()) != 1 {
		t.Errorf("season 1 = %v", md)
	}
	// Season 2 has episodes but no images; season 3 neither.
	if md := get(season(2)); md == nil || md.GetPremiereDate() != "2009-03-08" {
		t.Errorf("season 2 = %v", md)
	}
	if md := get(season(3)); md != nil {
		t.Errorf("season 3 = %v, want not found", md)
	}

	episode := func(s, e int32, ids map[string]string) *pluginv1.Lookup {
		return pluginv1.Lookup_builder{
			Kind: pluginv1.MediaKind_MEDIA_KIND_EPISODE.Enum(), SeriesExternalIds: series, ExternalIds: ids,
			SeasonNumber: proto.Int32(s), EpisodeNumber: proto.Int32(e), DisplayOrder: proto.String("dvd"),
		}.Build()
	}
	md = get(episode(1, 1, nil))
	if md == nil || md.GetName() != "Pilot" || md.GetPremiereDate() != "2008-01-20" || md.GetRuntime().AsDuration() != 58*time.Minute ||
		md.GetCommunityRating() != 8.2 || md.GetExternalIds()["tvdb"] != "349232" || len(md.GetImages()) != 1 {
		t.Errorf("episode 1x1 = %v", md)
	}
	// A file of two episodes has their names joined, as Jellyfin's
	// TmdbEpisodeProvider; the second has no overview to join.
	both := episode(1, 1, nil)
	both.SetEpisodeNumberEnd(2)
	if md := get(both); md.GetName() != "Pilot / Cat's in the Bag..." || md.GetOverview() != "Walter is diagnosed." || md.GetExternalIds()["tvdb"] != "349232" {
		t.Errorf("episodes 1x1-1x2 = %q, %q, %v; want = \"Pilot / Cat's in the Bag...\", the first's overview and ID", md.GetName(), md.GetOverview(), md.GetExternalIds())
	}
	// An episode's TVDB ID comes before its numbers.
	if md := get(episode(9, 9, map[string]string{"tvdb": "349236"})); md.GetName() != "Seven Thirty-Seven" {
		t.Errorf("episode by TVDB ID = %v", md)
	}
	if md := get(episode(5, 1, nil)); md != nil {
		t.Errorf("episode 5x1 = %v, want not found", md)
	}
	// The series is fetched once for all of them.
	if n := slices.Index(f.paths(), "GET /skyhook/shows/en/81189"); n < 0 || slices.Index(f.paths()[n+1:], "GET /skyhook/shows/en/81189") >= 0 {
		t.Errorf("requests = %v, want the series fetched once", f.paths())
	}
}

func TestListEpisodes(t *testing.T) {
	p, _ := newTestPlugin(t, seriesRoutes)
	resp, err := p.ListEpisodes(t.Context(), pluginv1.ListEpisodesRequest_builder{
		Lookup: lookup(pluginv1.MediaKind_MEDIA_KIND_SERIES, "", 0, map[string]string{"tmdb": "1396"}),
	}.Build())
	if err != nil {
		t.Fatal(err)
	}
	if !resp.GetFound() || resp.GetProvider() != "tvdb" {
		t.Fatalf("ListEpisodes() = %v", resp)
	}
	var seasons []int32
	for _, s := range resp.GetSeasons() {
		seasons = append(seasons, s.GetSeasonNumber())
	}
	if want := []int32{0, 1, 2}; !slices.Equal(seasons, want) {
		t.Errorf("seasons = %v, want = %v", seasons, want)
	}
	var episodes []string
	for _, e := range resp.GetEpisodes() {
		episodes = append(episodes, fmt.Sprintf("%dx%d #%d %s", e.GetSeasonNumber(), e.GetEpisodeNumber(), e.GetAbsoluteNumber(), e.GetMetadata().GetName()))
	}
	want := []string{"0x1 #0 Good Cop / Bad Cop", "1x1 #1 Pilot", "1x2 #2 Cat's in the Bag...", "2x1 #8 Seven Thirty-Seven"}
	if !slices.Equal(episodes, want) {
		t.Errorf("episodes = %q, want = %q", episodes, want)
	}

	resp, err = p.ListEpisodes(t.Context(), pluginv1.ListEpisodesRequest_builder{
		Lookup: lookup(pluginv1.MediaKind_MEDIA_KIND_SERIES, "", 0, map[string]string{"tvdb": "2"}),
	}.Build())
	if err != nil || resp.GetFound() {
		t.Errorf("ListEpisodes(unknown) = %v, %v, want not found", resp, err)
	}
}
