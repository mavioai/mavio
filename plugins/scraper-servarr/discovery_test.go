package main

import (
	"slices"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

func TestListCatalogs(t *testing.T) {
	p, _ := newTestPlugin(t, nil)
	resp, err := p.ListCatalogs(t.Context(), &pluginv1.ListCatalogsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, c := range resp.GetCatalogs() {
		ids = append(ids, c.GetId())
		if !slices.Equal(c.GetMediaKinds(), []pluginv1.MediaKind{pluginv1.MediaKind_MEDIA_KIND_MOVIE}) {
			t.Errorf("catalog %s lists %v, want movies", c.GetId(), c.GetMediaKinds())
		}
	}
	want := []string{"trending", "popular", "imdb_top250", "imdb_popular", "stevenlu", "stevenlu_imdb", "stevenlu_metacritic", "stevenlu_rottentomatoes", "recommendations"}
	if !slices.Equal(ids, want) {
		t.Errorf("catalogs = %v, want = %v", ids, want)
	}
}

func TestDiscover(t *testing.T) {
	list := `[
		{"TmdbId": 603, "Title": "The Matrix", "Year": 1999, "Images": [{"CoverType": "Poster", "Url": "p.jpg"}, {"CoverType": "Fanart", "Url": "f.jpg"}],
		 "MovieRatings": {"Tmdb": {"Count": 10, "Value": 8.2}}, "InCinema": "1999-03-31T00:00:00Z", "Genres": ["Action"]},
		{"TmdbId": 603, "Title": "The Matrix"},
		{"TmdbId": 0, "Title": "Unknown"},
		{"TmdbId": 604, "Title": "The Matrix Reloaded", "ImdbId": "tt0234215"}]`
	routes := map[string]string{
		"GET /radarr/list/tmdb/trending":          list,
		"GET /radarr/list/imdb/top250":            list,
		"GET /radarr/list/stevenlu/imdb/7":        list,
		"GET /radarr/list/stevenlu/metacritic/50": list,
		"GET /radarr/movie/603":                   matrix,
		"POST /radarr/movie/bulk":                 list,
	}
	tests := []struct {
		name     string
		req      *pluginv1.DiscoverRequest
		wantPath string
		wantBody string
		want     []string
	}{
		{"trending", pluginv1.DiscoverRequest_builder{Catalog: proto.String("trending")}.Build(), "GET /radarr/list/tmdb/trending", "", []string{"603", "604"}},
		{"IMDb Top 250", pluginv1.DiscoverRequest_builder{Catalog: proto.String("imdb_top250")}.Build(), "GET /radarr/list/imdb/top250", "", []string{"603", "604"}},
		{
			"StevenLu by IMDb rating", pluginv1.DiscoverRequest_builder{Catalog: proto.String("stevenlu_imdb"), RatingMin: proto.Float64(7.5)}.Build(),
			"GET /radarr/list/stevenlu/imdb/7", "",
			[]string{"603", "604"},
		},
		{
			"StevenLu by Metacritic score, at least 5", pluginv1.DiscoverRequest_builder{Catalog: proto.String("stevenlu_metacritic")}.Build(),
			"GET /radarr/list/stevenlu/metacritic/50", "",
			[]string{"603", "604"},
		},
		{
			"recommendations", pluginv1.DiscoverRequest_builder{Catalog: proto.String("recommendations"), ExternalIds: map[string]string{"tmdb": "603"}}.Build(),
			"POST /radarr/movie/bulk", "[604,605]",
			[]string{"603", "604"},
		},
		{"second page", pluginv1.DiscoverRequest_builder{Catalog: proto.String("trending"), Page: proto.Int32(2)}.Build(), "GET /radarr/list/tmdb/trending", "", nil},
		{
			"series", pluginv1.DiscoverRequest_builder{Catalog: proto.String("trending"), MediaKind: pluginv1.MediaKind_MEDIA_KIND_SERIES.Enum()}.Build(),
			"", "", nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, f := newTestPlugin(t, routes)
			resp, err := p.Discover(t.Context(), tt.req)
			if err != nil {
				t.Fatal(err)
			}
			var ids []string
			for _, title := range resp.GetTitles() {
				ids = append(ids, title.GetExternalIds()["tmdb"])
			}
			if !slices.Equal(ids, tt.want) {
				t.Errorf("titles = %v, want = %v", ids, tt.want)
			}
			if resp.GetTotalPages() != 1 {
				t.Errorf("total pages = %d, want = 1", resp.GetTotalPages())
			}
			if tt.wantPath == "" {
				if len(f.paths()) > 0 {
					t.Errorf("requests = %v, want none", f.paths())
				}
				return
			}
			if last := f.last(); last.method+" "+last.path != tt.wantPath || last.body != tt.wantBody {
				t.Errorf("request = %s %s %s, want = %s %s", last.method, last.path, last.body, tt.wantPath, tt.wantBody)
			}
		})
	}

	p, _ := newTestPlugin(t, routes)
	resp, err := p.Discover(t.Context(), pluginv1.DiscoverRequest_builder{Catalog: proto.String("trending")}.Build())
	if err != nil {
		t.Fatal(err)
	}
	m := resp.GetTitles()[0]
	if m.GetName() != "The Matrix" || m.GetYear() != 1999 || m.GetPremiereDate() != "1999-03-31" || m.GetPosterUrl() != "p.jpg" ||
		m.GetBackdropUrl() != "f.jpg" || m.GetCommunityRating() != 8.2 || m.GetVoteCount() != 10 || m.GetMediaKind() != pluginv1.MediaKind_MEDIA_KIND_MOVIE {
		t.Errorf("title = %v", m)
	}
	if _, err := p.Discover(t.Context(), pluginv1.DiscoverRequest_builder{Catalog: proto.String("nope")}.Build()); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("Discover(nope) = %v, want NotFound", err)
	}
}
