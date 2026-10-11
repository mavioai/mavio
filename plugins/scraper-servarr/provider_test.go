package main

import (
	"maps"
	"net/url"
	"slices"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/mavioai/mavio/libs/plugin/manifest"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

func TestManifest(t *testing.T) {
	if err := manifest.Validate(shipped); err != nil {
		t.Fatalf("Validate() = %v", err)
	}
	if err := manifest.ValidateConfig(shipped, "{}"); err != nil {
		t.Errorf("ValidateConfig({}) = %v, want nil: the plugin needs no keys", err)
	}
}

func TestUserAgent(t *testing.T) {
	t.Setenv("MAVIO_HOST_OS", "ubuntu 22.04")
	p, f := newTestPlugin(t, map[string]string{
		"GET /radarr/movie/603":       matrix,
		"GET /skyhook/shows/en/81189": breakingBad,
		"GET /services/dailyseries":   `[]`,
	})
	tests := []struct {
		name string
		call func() error
		want string
	}{
		{"radarr", func() error { _, err := p.movie(t.Context(), 603); return err }, "Radarr/6.4.4.10684 (ubuntu 22.04)"},
		{"skyhook", func() error { _, err := p.show(t.Context(), 81189); return err }, "Sonarr/4.0.20.3012 (ubuntu 22.04)"},
		{"services", func() error { _, err := p.ListDailySeries(t.Context(), nil); return err }, "Sonarr/4.0.20.3012 (ubuntu 22.04)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.call(); err != nil {
				t.Fatal(err)
			}
			if got := f.last().userAgent; got != tt.want {
				t.Errorf("User-Agent = %q, want = %q", got, tt.want)
			}
		})
	}
}

func lookup(kind pluginv1.MediaKind, name string, year int32, ids map[string]string) *pluginv1.Lookup {
	return pluginv1.Lookup_builder{Kind: kind.Enum(), Name: proto.String(name), Year: proto.Int32(year), ExternalIds: ids}.Build()
}

func TestSearchMovies(t *testing.T) {
	routes := map[string]string{
		"GET /radarr/search": `[
			{"TmdbId": 1, "Title": "The Matrix Resurrections", "Year": 2021},
			{"TmdbId": 603, "Title": "The Matrix", "OriginalTitle": "The Matrix", "Year": 1999,
			 "Images": [{"CoverType": "Poster", "Url": "https://image.tmdb.org/t/p/original/poster.jpg"}]},
			{"TmdbId": 2, "Title": "Matrix", "Year": 1998}]`,
		"GET /radarr/movie/603":            matrix,
		"GET /radarr/movie/imdb/tt0133093": "[" + matrix + "]",
	}
	tests := []struct {
		name      string
		lookup    *pluginv1.Lookup
		wantIDs   []string
		wantPath  string
		wantQuery url.Values
	}{
		{
			name:     "by title and year",
			lookup:   lookup(pluginv1.MediaKind_MEDIA_KIND_MOVIE, "The.Matrix, The", 1999, nil),
			wantIDs:  []string{"603", "2", "1"},
			wantPath: "GET /radarr/search", wantQuery: url.Values{"q": {"the+matrix"}, "year": {"1999"}},
		},
		{
			name:    "by TMDB ID",
			lookup:  lookup(pluginv1.MediaKind_MEDIA_KIND_MOVIE, "Whatever", 0, map[string]string{"tmdb": "603"}),
			wantIDs: []string{"603"}, wantPath: "GET /radarr/movie/603", wantQuery: url.Values{},
		},
		{
			name:    "by IMDb ID",
			lookup:  lookup(pluginv1.MediaKind_MEDIA_KIND_MOVIE, "Whatever", 0, map[string]string{"imdb": "tt0133093"}),
			wantIDs: []string{"603"}, wantPath: "GET /radarr/movie/imdb/tt0133093", wantQuery: url.Values{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, f := newTestPlugin(t, routes)
			resp, err := p.Search(t.Context(), pluginv1.SearchRequest_builder{Lookup: tt.lookup}.Build())
			if err != nil {
				t.Fatal(err)
			}
			var ids []string
			for _, r := range resp.GetResults() {
				ids = append(ids, r.GetExternalIds()["tmdb"])
			}
			if !slices.Equal(ids, tt.wantIDs) {
				t.Errorf("results = %v, want = %v", ids, tt.wantIDs)
			}
			last := f.last()
			if got := last.method + " " + last.path; got != tt.wantPath {
				t.Errorf("request = %q, want = %q", got, tt.wantPath)
			}
			if q, _ := url.ParseQuery(last.query); len(tt.wantQuery) > 0 && !equalValues(q, tt.wantQuery) {
				t.Errorf("query = %v, want = %v", q, tt.wantQuery)
			}
		})
	}
}

func equalValues(a, b url.Values) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if !slices.Equal(v, b[k]) {
			return false
		}
	}
	return true
}

func TestMovieMetadata(t *testing.T) {
	p, _ := newTestPlugin(t, map[string]string{"GET /radarr/movie/603": matrix})
	get := func(language, country string) *pluginv1.Metadata {
		t.Helper()
		l := lookup(pluginv1.MediaKind_MEDIA_KIND_MOVIE, "", 0, map[string]string{"tmdb": "603"})
		l.SetLanguage(language)
		l.SetCountry(country)
		resp, err := p.GetMetadata(t.Context(), pluginv1.GetMetadataRequest_builder{Lookup: l}.Build())
		if err != nil || !resp.GetFound() {
			t.Fatalf("GetMetadata() = %v, %v", resp, err)
		}
		return resp.GetMetadata()
	}
	md := get("en", "")
	checks := []struct {
		field     string
		got, want any
	}{
		{"name", md.GetName(), "The Matrix"},
		{"overview", md.GetOverview(), "A hacker learns the truth."},
		{"production year", md.GetProductionYear(), int32(1999)},
		{"premiere date", md.GetPremiereDate(), "1999-03-31"},
		{"runtime", md.GetRuntime().AsDuration(), 136 * time.Minute},
		{"official rating", md.GetOfficialRating(), "R"},
		{"community rating", md.GetCommunityRating(), 8.261},
		{"critic rating", md.GetCriticRating(), 83.0},
		{"genres", md.GetGenres(), []string{"Action", "Science Fiction"}},
		{"tags", md.GetTags(), []string{"cyberpunk", "simulated reality"}},
		{"studios", md.GetStudios(), []string{"Village Roadshow Pictures"}},
		{"collection", md.GetCollectionName(), "The Matrix Collection"},
		{"original language", md.GetOriginalLanguage(), "en"},
		{"digital release date", md.GetDigitalReleaseDate(), "1999-09-21"},
		{"physical release date", md.GetPhysicalReleaseDate(), "1999-11-25"},
		{"secondary year", md.GetSecondaryYear(), int32(1998)},
		{"alternative titles", md.GetAlternativeTitles(), []string{"Matrix"}},
		{"translated titles", md.GetTranslatedTitles(), []string{"Matrix", "Matrix (AT)"}},
		{"external IDs", md.GetExternalIds(), map[string]string{"tmdb": "603", "imdb": "tt0133093", "tmdb_collection": "2344"}},
	}
	for _, c := range checks {
		if !equal(c.got, c.want) {
			t.Errorf("%s = %v, want = %v", c.field, c.got, c.want)
		}
	}
	var people []string
	for _, pc := range md.GetPeople() {
		people = append(people, pc.GetKind().String()+" "+pc.GetName()+" "+pc.GetRole()+" "+pc.GetImageUrl())
	}
	wantPeople := []string{
		"CREDIT_KIND_ACTOR Keanu Reeves Neo https://image.tmdb.org/t/p/original/keanu.jpg",
		"CREDIT_KIND_DIRECTOR Lana Wachowski Director ",
		"CREDIT_KIND_WRITER Lana Wachowski Writer ",
		"CREDIT_KIND_PRODUCER Joel Silver Producer ",
	}
	if !slices.Equal(people, wantPeople) {
		t.Errorf("people = %q, want = %q", people, wantPeople)
	}
	var images []string
	for _, im := range md.GetImages() {
		images = append(images, im.GetKind().String())
	}
	if want := []string{"IMAGE_KIND_PRIMARY", "IMAGE_KIND_BACKDROP", "IMAGE_KIND_LOGO"}; !slices.Equal(images, want) {
		t.Errorf("images = %v, want = %v", images, want)
	}

	// Translations replace the title and overview they have, the
	// country's first; ratings follow the country.
	for _, tt := range []struct{ language, country, name, overview, rating string }{
		{"de", "DE", "Matrix", "Ein Hacker erfährt die Wahrheit.", "16"},
		{"de", "AT", "Matrix (AT)", "A hacker learns the truth.", ""},
		{"fr", "AU", "The Matrix", "A hacker learns the truth.", "M"},
	} {
		md := get(tt.language, tt.country)
		if md.GetName() != tt.name || md.GetOverview() != tt.overview || md.GetOfficialRating() != tt.rating {
			t.Errorf("%s-%s: got = %q, %q, %q, want = %q, %q, %q", tt.language, tt.country,
				md.GetName(), md.GetOverview(), md.GetOfficialRating(), tt.name, tt.overview, tt.rating)
		}
	}
}

func equal(a, b any) bool {
	switch a := a.(type) {
	case []string:
		return slices.Equal(a, b.([]string))
	case map[string]string:
		b := b.(map[string]string)
		if len(a) != len(b) {
			return false
		}
		for k, v := range a {
			if b[k] != v {
				return false
			}
		}
		return true
	}
	return a == b
}

func TestMovieNotFound(t *testing.T) {
	p, _ := newTestPlugin(t, nil)
	for _, ids := range []map[string]string{{"tmdb": "1"}, {"imdb": "tt0000001"}, {"imdb": "bad"}, {}} {
		resp, err := p.GetMetadata(t.Context(), pluginv1.GetMetadataRequest_builder{
			Lookup: lookup(pluginv1.MediaKind_MEDIA_KIND_MOVIE, "", 0, ids),
		}.Build())
		if err != nil || resp.GetFound() {
			t.Errorf("GetMetadata(%v) = %v, %v, want not found", ids, resp, err)
		}
	}
}

func TestIdentify(t *testing.T) {
	routes := map[string]string{
		"GET /radarr/search": `[
			{"TmdbId": 1, "Title": "The Matrix Resurrections", "Year": 2021},
			{"TmdbId": 604, "Title": "The Matrix", "Year": 2003},
			{"TmdbId": 603, "Title": "The Matrix", "Year": 1999}]`,
	}
	maps.Copy(routes, seriesRoutes)
	tests := []struct {
		name   string
		lookup *pluginv1.Lookup
		want   string // the TMDB ID of a movie, the TVDB ID of a series
	}{
		{"movie by title and year", lookup(pluginv1.MediaKind_MEDIA_KIND_MOVIE, "The Matrix", 1999, nil), "603"},
		{"movie by title", lookup(pluginv1.MediaKind_MEDIA_KIND_MOVIE, "The Matrix", 0, nil), "604"},
		// A year without a matching title identifies nothing.
		{"movie whose title matches none", lookup(pluginv1.MediaKind_MEDIA_KIND_MOVIE, "Matrix Reloaded", 2021, nil), ""},
		{"series by title", lookup(pluginv1.MediaKind_MEDIA_KIND_SERIES, "Breaking Bad", 0, nil), "81189"},
		{"series whose title matches none", lookup(pluginv1.MediaKind_MEDIA_KIND_SERIES, "Breaking", 2008, nil), ""},
		{"series without a name", lookup(pluginv1.MediaKind_MEDIA_KIND_SERIES, "", 0, nil), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, _ := newTestPlugin(t, routes)
			resp, err := p.GetMetadata(t.Context(), pluginv1.GetMetadataRequest_builder{Lookup: tt.lookup}.Build())
			if err != nil {
				t.Fatal(err)
			}
			key := "tmdb"
			if tt.lookup.GetKind() == pluginv1.MediaKind_MEDIA_KIND_SERIES {
				key = "tvdb"
			}
			if got := resp.GetMetadata().GetExternalIds()[key]; got != tt.want || resp.GetFound() != (tt.want != "") {
				t.Errorf("identified = %v %q, want = %q", resp.GetFound(), got, tt.want)
			}
		})
	}
}
