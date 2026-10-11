package main

import (
	"net/url"
	"slices"
	"testing"

	"google.golang.org/protobuf/proto"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

// newSearchPlugin returns a plugin over the TMDB fake serving the
// discovery routes and searches.
func newSearchPlugin(t *testing.T) (*plugin, *fakeTMDB) {
	t.Helper()
	p, fake := newDiscoveryPlugin(t)
	fake.routes["/3/search/multi"] = `{"page":2,"total_pages":4,"total_results":70,"results":[
		{"id":603,"media_type":"movie","title":"The Matrix","release_date":"1999-03-30"},
		{"id":1396,"media_type":"tv","name":"Breaking Bad","first_air_date":"2008-01-20"},
		{"id":6384,"media_type":"person","name":"Keanu Reeves"},
		{"id":9,"media_type":"movie","title":"Adult","adult":true}]}`
	fake.routes["/3/search/tv"] = `{"page":1,"total_pages":1,"total_results":1,"results":[{"id":5,"name":"Mulan Show","first_air_date":"1998-01-01"}]}`
	return p, fake
}

func search(t *testing.T, p *plugin, req *pluginv1.SearchTitlesRequest) (titles []string, resp *pluginv1.SearchTitlesResponse) {
	t.Helper()
	resp, err := p.SearchTitles(t.Context(), req)
	if err != nil {
		t.Fatalf("SearchTitles(%v) = %v", req, err)
	}
	for _, title := range resp.GetTitles() {
		kind := "movie"
		if title.GetMediaKind() == pluginv1.MediaKind_MEDIA_KIND_SERIES {
			kind = "tv"
		}
		titles = append(titles, kind+":"+title.GetExternalIds()[keyTMDB])
	}
	return titles, resp
}

// queries returns the query of each request to TMDB, by path.
func (f *fakeTMDB) queries() map[string]url.Values {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := map[string]url.Values{}
	for _, r := range f.requests {
		out[r.URL.Path] = r.URL.Query()
	}
	return out
}

func TestSearchTitles(t *testing.T) {
	series := pluginv1.MediaKind_MEDIA_KIND_SERIES.Enum()
	movie := pluginv1.MediaKind_MEDIA_KIND_MOVIE.Enum()
	tests := []struct {
		name  string
		req   *pluginv1.SearchTitlesRequest
		want  []string
		paths []string
		// params are query parameters of the last request to their path.
		params map[string]map[string]string
		page   [3]int32
	}{
		{
			name: "multi search", req: pluginv1.SearchTitlesRequest_builder{Query: proto.String("matrix"), Page: proto.Int32(2), Language: proto.String("de")}.Build(),
			want: []string{"movie:603", "tv:1396"}, paths: []string{"/3/search/multi"},
			params: map[string]map[string]string{"/3/search/multi": {"query": "matrix", "page": "2", "include_adult": "false", "language": "de"}},
			page:   [3]int32{2, 4, 70},
		},
		{
			name: "movie search of a year", req: pluginv1.SearchTitlesRequest_builder{Query: proto.String("mulan"), MediaKind: movie, Year: proto.Int32(2020)}.Build(),
			want: []string{"movie:10674", "movie:337401", "movie:752662"}, paths: []string{"/3/search/movie"},
			params: map[string]map[string]string{"/3/search/movie": {"query": "mulan", "primary_release_year": "2020", "page": "1"}},
		},
		{
			name: "series search", req: pluginv1.SearchTitlesRequest_builder{Query: proto.String("mulan"), MediaKind: series, Year: proto.Int32(1998)}.Build(),
			want: []string{"tv:5"}, paths: []string{"/3/search/tv"},
			params: map[string]map[string]string{"/3/search/tv": {"first_air_date_year": "1998"}},
			page:   [3]int32{1, 1, 1},
		},
		{
			name: "TMDB ID", req: pluginv1.SearchTitlesRequest_builder{Query: proto.String("tmdb:1396")}.Build(),
			want: []string{"tv:1396"}, paths: []string{"/3/movie/1396", "/3/tv/1396"}, page: [3]int32{1, 1, 1},
		},
		{
			name: "TMDB ID of a movie", req: pluginv1.SearchTitlesRequest_builder{Query: proto.String("tmdb:1396"), MediaKind: movie}.Build(),
			paths: []string{"/3/movie/1396"}, page: [3]int32{1, 1, 0},
		},
		{
			name: "IMDb ID", req: pluginv1.SearchTitlesRequest_builder{Query: proto.String("imdb:tt0120762")}.Build(),
			want: []string{"movie:10674"}, paths: []string{"/3/find/tt0120762"},
			params: map[string]map[string]string{"/3/find/tt0120762": {"external_source": "imdb_id"}},
			page:   [3]int32{1, 1, 1},
		},
		{
			name: "TVDB ID", req: pluginv1.SearchTitlesRequest_builder{Query: proto.String("tvdb:81189")}.Build(),
			want: []string{"tv:1396"}, paths: []string{"/3/find/81189"},
			params: map[string]map[string]string{"/3/find/81189": {"external_source": "tvdb_id"}},
			page:   [3]int32{1, 1, 1},
		},
		{
			name: "year", req: pluginv1.SearchTitlesRequest_builder{Query: proto.String("mulan year:1998")}.Build(),
			want: []string{"movie:10674", "movie:337401", "movie:752662", "tv:5"}, paths: []string{"/3/search/movie", "/3/search/tv"},
			params: map[string]map[string]string{
				"/3/search/movie": {"query": "mulan", "primary_release_year": "1998"},
				"/3/search/tv":    {"query": "mulan", "first_air_date_year": "1998"},
			},
			page: [3]int32{1, 1, 4},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, fake := newSearchPlugin(t)
			got, resp := search(t, p, tt.req)
			if !slices.Equal(got, tt.want) {
				t.Errorf("titles = %v, want = %v", got, tt.want)
			}
			if paths := fake.paths(); !slices.Equal(paths, tt.paths) {
				t.Errorf("requests = %v, want = %v", paths, tt.paths)
			}
			queries := fake.queries()
			for path, params := range tt.params {
				for k, v := range params {
					if got := queries[path].Get(k); got != v {
						t.Errorf("%s %s = %q, want = %q", path, k, got, v)
					}
				}
			}
			if page := [3]int32{resp.GetPage(), resp.GetTotalPages(), resp.GetTotalResults()}; tt.page != [3]int32{} && page != tt.page {
				t.Errorf("page = %v, want = %v", page, tt.page)
			}
		})
	}
}
