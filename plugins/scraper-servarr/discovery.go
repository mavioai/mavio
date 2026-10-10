package main

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

// catalog is a list of movies Radarr's metadata service offers.
type catalog struct {
	id, name string
	kind     pluginv1.CatalogKind
	filters  []pluginv1.DiscoverFilter
	// path is the list's route; stevenLu lists take a minimum score.
	path     string
	stevenLu string
}

// catalogs are Radarr's lists: the trending and popular movies of its
// discovery, the IMDb lists and the StevenLu lists of its import lists,
// and a movie's recommendations.
var catalogs = []catalog{
	{id: "trending", name: "Trending", kind: pluginv1.CatalogKind_CATALOG_KIND_TRENDING, path: "/list/tmdb/trending"},
	{id: "popular", name: "Popular", kind: pluginv1.CatalogKind_CATALOG_KIND_POPULAR, path: "/list/tmdb/popular"},
	{id: "imdb_top250", name: "IMDb Top 250", kind: pluginv1.CatalogKind_CATALOG_KIND_LIST, path: "/list/imdb/top250"},
	{id: "imdb_popular", name: "IMDb Popular Movies", kind: pluginv1.CatalogKind_CATALOG_KIND_LIST, path: "/list/imdb/popular"},
	{id: "stevenlu", name: "StevenLu", kind: pluginv1.CatalogKind_CATALOG_KIND_LIST, path: "/list/stevenlu"},
	{
		id: "stevenlu_imdb", name: "StevenLu by IMDb rating", kind: pluginv1.CatalogKind_CATALOG_KIND_LIST, path: "/list/stevenlu",
		stevenLu: "imdb", filters: []pluginv1.DiscoverFilter{pluginv1.DiscoverFilter_DISCOVER_FILTER_RATING},
	},
	{
		id: "stevenlu_metacritic", name: "StevenLu by Metacritic score", kind: pluginv1.CatalogKind_CATALOG_KIND_LIST, path: "/list/stevenlu",
		stevenLu: "metacritic", filters: []pluginv1.DiscoverFilter{pluginv1.DiscoverFilter_DISCOVER_FILTER_RATING},
	},
	{
		id: "stevenlu_rottentomatoes", name: "StevenLu by Rotten Tomatoes score", kind: pluginv1.CatalogKind_CATALOG_KIND_LIST, path: "/list/stevenlu",
		stevenLu: "rottentomatoes", filters: []pluginv1.DiscoverFilter{pluginv1.DiscoverFilter_DISCOVER_FILTER_RATING},
	},
	{id: "recommendations", name: "Recommendations", kind: pluginv1.CatalogKind_CATALOG_KIND_RECOMMENDATIONS},
}

func (p *plugin) ListCatalogs(context.Context, *pluginv1.ListCatalogsRequest) (*pluginv1.ListCatalogsResponse, error) {
	out := make([]*pluginv1.Catalog, len(catalogs))
	for i, c := range catalogs {
		out[i] = pluginv1.Catalog_builder{
			Id: proto.String(c.id), Name: proto.String(c.name), Kind: c.kind.Enum(),
			MediaKinds: []pluginv1.MediaKind{pluginv1.MediaKind_MEDIA_KIND_MOVIE},
			Filters:    c.filters,
		}.Build()
	}
	return pluginv1.ListCatalogsResponse_builder{Catalogs: out}.Build(), nil
}

// Discover lists a catalog's movies on a single page, each once.
func (p *plugin) Discover(ctx context.Context, req *pluginv1.DiscoverRequest) (*pluginv1.DiscoverResponse, error) {
	var c *catalog
	for i := range catalogs {
		if catalogs[i].id == req.GetCatalog() {
			c = &catalogs[i]
		}
	}
	if c == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("unknown catalog %q", req.GetCatalog()))
	}
	if k := req.GetMediaKind(); k != pluginv1.MediaKind_MEDIA_KIND_UNSPECIFIED && k != pluginv1.MediaKind_MEDIA_KIND_MOVIE {
		return pluginv1.DiscoverResponse_builder{Page: proto.Int32(1), TotalPages: proto.Int32(1), TotalResults: proto.Int32(0)}.Build(), nil
	}
	movies, err := p.catalogMovies(ctx, c, req)
	if err != nil {
		return nil, err
	}
	seen := map[int]bool{}
	var titles []*pluginv1.DiscoveredTitle
	for _, m := range movies {
		if m.TmdbID <= 0 || seen[m.TmdbID] {
			continue
		}
		seen[m.TmdbID] = true
		titles = append(titles, m.discovered())
	}
	total := len(titles)
	if req.GetPage() > 1 {
		titles = nil
	}
	return pluginv1.DiscoverResponse_builder{
		Titles: titles, Page: proto.Int32(max(req.GetPage(), 1)), TotalPages: proto.Int32(1), TotalResults: proto.Int32(int32(total)),
	}.Build(), nil
}

func (p *plugin) catalogMovies(ctx context.Context, c *catalog, req *pluginv1.DiscoverRequest) ([]radarrMovie, error) {
	if c.kind == pluginv1.CatalogKind_CATALOG_KIND_RECOMMENDATIONS {
		m, err := p.findMovie(ctx, req.GetExternalIds())
		if err != nil {
			return nil, err
		}
		ids := make([]int, 0, len(m.Recommendations))
		for _, r := range m.Recommendations {
			ids = append(ids, r.TmdbID)
		}
		return p.moviesByID(ctx, ids)
	}
	path := c.path
	if c.stevenLu != "" {
		path += "/" + c.stevenLu + "/" + strconv.Itoa(stevenLuScore(c.stevenLu, req.GetRatingMin()))
	}
	var ms []radarrMovie
	return ms, p.radarr.get(ctx, path, nil, &ms)
}

// stevenLuScore is the minimum score of a StevenLu list for a minimum
// rating out of 10, within Radarr's 5 to 8: IMDb's as it is, the others'
// out of 100.
func stevenLuScore(source string, rating float64) int {
	score := min(max(int(rating), 5), 8)
	if source != "imdb" {
		score *= 10
	}
	return score
}

func (m radarrMovie) discovered() *pluginv1.DiscoveredTitle {
	t := pluginv1.DiscoveredTitle_builder{
		MediaKind:        pluginv1.MediaKind_MEDIA_KIND_MOVIE.Enum(),
		Name:             proto.String(m.Title),
		OriginalTitle:    proto.String(m.OriginalTitle),
		OriginalLanguage: proto.String(m.OriginalLanguage),
		Overview:         proto.String(m.Overview),
		ExternalIds:      m.externalIDs(),
		Genres:           m.Genres,
		PosterUrl:        proto.String(imageOf(m.Images, "poster")),
		BackdropUrl:      proto.String(imageOf(m.Images, "fanart")),
		Popularity:       proto.Float64(m.Popularity),
	}.Build()
	if m.Year > 0 {
		t.SetYear(int32(m.Year))
	}
	if d := m.premiere(); d != nil {
		t.SetPremiereDate(d.Format(time.DateOnly))
	}
	if r := m.tmdbRating(); r != nil {
		t.SetCommunityRating(r.Value)
		t.SetVoteCount(int32(r.Count))
	}
	return t
}

// ListFilterValues lists no values: the catalogs take no filters with
// values.
func (p *plugin) ListFilterValues(context.Context, *pluginv1.ListFilterValuesRequest) (*pluginv1.ListFilterValuesResponse, error) {
	return &pluginv1.ListFilterValuesResponse{}, nil
}
