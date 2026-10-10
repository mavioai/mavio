package main

import (
	"maps"
	"net/http/httptest"
	"net/url"
	"slices"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

var discoveryRoutes = map[string]string{
	"/3/trending/all/day": `{"page":1,"total_pages":3,"total_results":60,"results":[
		{"id":1,"media_type":"movie","title":"Dune","original_title":"Dune","original_language":"en","release_date":"2021-09-15",
			"genre_ids":[878,12],"poster_path":"/dune.jpg","vote_average":7.8,"vote_count":12000,"popularity":99.5},
		{"id":2,"media_type":"tv","name":"Severance","original_name":"Severance","first_air_date":"2022-02-17","backdrop_path":"/sev.jpg"},
		{"id":3,"media_type":"person","name":"Someone"},
		{"id":4,"media_type":"movie","title":"Adult","adult":true}]}`,
	"/3/discover/tv":    `{"page":2,"total_pages":2,"total_results":21,"results":[{"id":5,"name":"Andor","first_air_date":"2022-09-21"}]}`,
	"/3/discover/movie": `{"page":1,"total_pages":1,"total_results":0,"results":[]}`,
	"/3/find/tt1160419": `{"movie_results":[{"id":438631,"title":"Dune"}],"tv_results":[]}`,
	"/3/movie/438631/recommendations": `{"page":1,"total_pages":1,"total_results":1,"results":[
		{"id":693134,"title":"Dune: Part Two","release_date":"2024-02-27"}]}`,
	"/3/genre/tv/list":      `{"genres":[{"id":18,"name":"Drama"},{"id":10765,"name":"Sci-Fi & Fantasy"}]}`,
	"/3/watch/providers/tv": `{"results":[{"provider_id":8,"provider_name":"Netflix","logo_path":"/nf.jpg"}]}`,
	"/3/search/keyword":     `{"results":[{"id":9882,"name":"space"}]}`,
	"/3/network/49":         `{"id":49,"name":"HBO","logo_path":"/hbo.png"}`,
}

// newDiscoveryPlugin returns a plugin over the TMDB fake serving the
// discovery routes, at a fixed time.
func newDiscoveryPlugin(t *testing.T) (*plugin, *fakeTMDB) {
	t.Helper()
	fake := &fakeTMDB{routes: maps.Clone(routes)}
	maps.Copy(fake.routes, discoveryRoutes)
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	p := &plugin{baseURL: srv.URL + "/3", now: func() time.Time { return time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC) }}
	if _, err := p.Configure(t.Context(), pluginv1.ConfigureRequest_builder{ConfigJson: proto.String(`{"api_key":"key"}`)}.Build()); err != nil {
		t.Fatal(err)
	}
	return p, fake
}

func discover(t *testing.T, p *plugin, req *pluginv1.DiscoverRequest) *pluginv1.DiscoverResponse {
	t.Helper()
	resp, err := p.Discover(t.Context(), req)
	if err != nil {
		t.Fatalf("Discover(%v) = %v", req, err)
	}
	return resp
}

func TestListCatalogs(t *testing.T) {
	p, _ := newDiscoveryPlugin(t)
	resp, err := p.ListCatalogs(t.Context(), &pluginv1.ListCatalogsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, c := range resp.GetCatalogs() {
		ids = append(ids, c.GetId())
	}
	if want := []string{"trending", "popular", "upcoming", "top_rated", "recommendations", "similar"}; !slices.Equal(ids, want) {
		t.Errorf("catalogs = %v, want = %v", ids, want)
	}
	if popular := resp.GetCatalogs()[1]; len(popular.GetFilters()) == 0 || len(popular.GetSorts()) == 0 {
		t.Errorf("popular catalog = %v, want = filters and sorts", popular)
	}
}

func TestDiscoverTrending(t *testing.T) {
	p, fake := newDiscoveryPlugin(t)
	resp := discover(t, p, pluginv1.DiscoverRequest_builder{Catalog: proto.String("trending")}.Build())
	if got := fake.paths(); !slices.Equal(got, []string{"/3/trending/all/day"}) {
		t.Errorf("requests = %v", got)
	}
	if resp.GetTotalPages() != 3 || resp.GetTotalResults() != 60 || len(resp.GetTitles()) != 2 {
		t.Fatalf("Discover() = %v, want = a movie and a series of 3 pages", resp)
	}
	dune, sev := resp.GetTitles()[0], resp.GetTitles()[1]
	if dune.GetMediaKind() != pluginv1.MediaKind_MEDIA_KIND_MOVIE || dune.GetName() != "Dune" || dune.GetYear() != 2021 ||
		dune.GetExternalIds()["tmdb"] != "1" || !slices.Equal(dune.GetGenres(), []string{"878", "12"}) ||
		dune.GetPosterUrl() != imageURL("/dune.jpg") || dune.GetCommunityRating() != 7.8 || dune.GetVoteCount() != 12000 {
		t.Errorf("movie = %v", dune)
	}
	if sev.GetMediaKind() != pluginv1.MediaKind_MEDIA_KIND_SERIES || sev.GetName() != "Severance" ||
		sev.GetPremiereDate() != "2022-02-17" || sev.GetBackdropUrl() != imageURL("/sev.jpg") {
		t.Errorf("series = %v", sev)
	}
}

func TestDiscoverFilters(t *testing.T) {
	p, fake := newDiscoveryPlugin(t)
	resp := discover(t, p, pluginv1.DiscoverRequest_builder{
		Catalog: proto.String("popular"), MediaKind: pluginv1.MediaKind_MEDIA_KIND_SERIES.Enum(), Page: proto.Int32(2),
		Genres: []string{"18", "10765"}, Networks: []string{"49"}, Studios: []string{"ignored"}, Keywords: []string{"9882"},
		ExcludedKeywords: []string{"1", "2"}, WatchProviders: []string{"8"}, Region: proto.String("DE"),
		ReleasedFrom: proto.String("2020-01-01"), RatingMin: proto.Float64(7.5), VotesMin: proto.Int32(100),
		CertificationCountry: proto.String("US"), Certifications: []string{"TV-14", "TV-MA"},
		Sort: pluginv1.DiscoverSort_DISCOVER_SORT_RELEASE_DATE.Enum(), Ascending: proto.Bool(true), Language: proto.String("de"),
	}.Build())
	if len(resp.GetTitles()) != 1 || resp.GetTitles()[0].GetMediaKind() != pluginv1.MediaKind_MEDIA_KIND_SERIES || resp.GetPage() != 2 {
		t.Errorf("Discover() = %v", resp)
	}
	r := fake.requests[0]
	if r.URL.Path != "/3/discover/tv" {
		t.Fatalf("path = %s", r.URL.Path)
	}
	want := url.Values{
		"api_key": {"key"}, "page": {"2"}, "language": {"de"}, "with_genres": {"18|10765"}, "with_networks": {"49"},
		"with_keywords": {"9882"}, "without_keywords": {"1,2"}, "with_watch_providers": {"8"}, "watch_region": {"DE"},
		"first_air_date.gte": {"2020-01-01"}, "first_air_date.lte": {"2028-04-11"}, "vote_average.gte": {"7.5"},
		"vote_count.gte": {"100"}, "certification_country": {"US"}, "certification": {"TV-14|TV-MA"},
		"include_adult": {"false"}, "sort_by": {"first_air_date.asc"},
	}
	if got := r.URL.Query(); !maps.EqualFunc(got, want, slices.Equal) {
		t.Errorf("query = %v, want = %v", got, want)
	}
}

func TestDiscoverUpcoming(t *testing.T) {
	p, fake := newDiscoveryPlugin(t)
	discover(t, p, pluginv1.DiscoverRequest_builder{
		Catalog: proto.String("upcoming"), MediaKind: pluginv1.MediaKind_MEDIA_KIND_MOVIE.Enum(), ReleasedFrom: proto.String("2000-01-01"),
		Studios: []string{"420"}, Region: proto.String("GB"),
	}.Build())
	q := fake.requests[0].URL.Query()
	for k, v := range map[string]string{
		"primary_release_date.gte": "2026-10-11", "primary_release_date.lte": "2028-04-11", "with_companies": "420",
		"region": "GB", "sort_by": "popularity.desc",
	} {
		if q.Get(k) != v {
			t.Errorf("%s = %q, want = %q", k, q.Get(k), v)
		}
	}
}

func TestDiscoverRelated(t *testing.T) {
	p, fake := newDiscoveryPlugin(t)
	resp := discover(t, p, pluginv1.DiscoverRequest_builder{
		Catalog: proto.String("recommendations"), MediaKind: pluginv1.MediaKind_MEDIA_KIND_MOVIE.Enum(),
		ExternalIds: map[string]string{"imdb": "tt1160419"},
	}.Build())
	if got, want := fake.paths(), []string{"/3/find/tt1160419", "/3/movie/438631/recommendations"}; !slices.Equal(got, want) {
		t.Errorf("requests = %v, want = %v", got, want)
	}
	if len(resp.GetTitles()) != 1 || resp.GetTitles()[0].GetName() != "Dune: Part Two" {
		t.Errorf("Discover() = %v", resp)
	}
	// TMDB not knowing the title lists nothing.
	resp = discover(t, p, pluginv1.DiscoverRequest_builder{
		Catalog: proto.String("similar"), MediaKind: pluginv1.MediaKind_MEDIA_KIND_SERIES.Enum(), ExternalIds: map[string]string{"tmdb": "7"},
	}.Build())
	if len(resp.GetTitles()) != 0 {
		t.Errorf("Discover(unknown) = %v, want = nothing", resp)
	}
	if _, err := p.Discover(t.Context(), pluginv1.DiscoverRequest_builder{Catalog: proto.String("nope")}.Build()); err == nil {
		t.Error("Discover(unknown catalog) = nil error")
	}
}

func TestDiscoverAdult(t *testing.T) {
	p, _ := newDiscoveryPlugin(t)
	if _, err := p.Configure(t.Context(), pluginv1.ConfigureRequest_builder{ConfigJson: proto.String(`{"api_key":"key","include_adult":true}`)}.Build()); err != nil {
		t.Fatal(err)
	}
	if got := discover(t, p, pluginv1.DiscoverRequest_builder{Catalog: proto.String("trending")}.Build()).GetTitles(); len(got) != 3 {
		t.Errorf("titles with adult ones = %d, want = 3", len(got))
	}
}

func TestListFilterValues(t *testing.T) {
	p, fake := newDiscoveryPlugin(t)
	series := pluginv1.MediaKind_MEDIA_KIND_SERIES.Enum()
	for _, tt := range []struct {
		req       *pluginv1.ListFilterValuesRequest
		path      string
		ids, logo []string
	}{
		{
			req:  pluginv1.ListFilterValuesRequest_builder{Filter: pluginv1.DiscoverFilter_DISCOVER_FILTER_GENRE.Enum(), MediaKind: series, Query: proto.String("sci")}.Build(),
			path: "/3/genre/tv/list", ids: []string{"10765"}, logo: []string{""},
		},
		{
			req: pluginv1.ListFilterValuesRequest_builder{
				Filter: pluginv1.DiscoverFilter_DISCOVER_FILTER_WATCH_PROVIDER.Enum(), MediaKind: series, Region: proto.String("US"),
			}.Build(),
			path: "/3/watch/providers/tv", ids: []string{"8"}, logo: []string{imageURL("/nf.jpg")},
		},
		{
			req:  pluginv1.ListFilterValuesRequest_builder{Filter: pluginv1.DiscoverFilter_DISCOVER_FILTER_KEYWORD.Enum(), Query: proto.String("space")}.Build(),
			path: "/3/search/keyword", ids: []string{"9882"}, logo: []string{""},
		},
		{
			req:  pluginv1.ListFilterValuesRequest_builder{Filter: pluginv1.DiscoverFilter_DISCOVER_FILTER_NETWORK.Enum(), Ids: []string{"49"}}.Build(),
			path: "/3/network/49", ids: []string{"49"}, logo: []string{imageURL("/hbo.png")},
		},
	} {
		fake.requests = nil
		resp, err := p.ListFilterValues(t.Context(), tt.req)
		if err != nil {
			t.Fatalf("ListFilterValues(%v) = %v", tt.req, err)
		}
		var ids, logos []string
		for _, v := range resp.GetValues() {
			ids, logos = append(ids, v.GetId()), append(logos, v.GetImageUrl())
		}
		if got := fake.paths(); !slices.Equal(got, []string{tt.path}) || !slices.Equal(ids, tt.ids) || !slices.Equal(logos, tt.logo) {
			t.Errorf("ListFilterValues(%v) = %v %v after %v, want = %v %v after %s", tt.req, ids, logos, got, tt.ids, tt.logo, tt.path)
		}
	}
	if q := fake.requests[0].URL.Query(); q.Has("query") {
		t.Errorf("network lookup query = %v", q)
	}
}
