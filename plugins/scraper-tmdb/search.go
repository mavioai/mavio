package main

import (
	"cmp"
	"context"
	"errors"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"google.golang.org/protobuf/proto"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

// Search follows Seerr's GET /search (server/routes/search.ts) and its
// search providers (server/lib/search.ts): "tmdb:<id>" looks the movie
// and series of a TMDB ID up, "imdb:<id>" and "tvdb:<id>" find titles by
// another provider's ID, "year:<yyyy>" searches movies and series of a
// year by the rest of the query; other queries search TMDB's multi search,
// or its movie or series search when a kind is asked for. Persons are
// left out.

// The prefixes of Seerr's search providers.
var (
	tmdbPrefix = regexp.MustCompile(`tmdb:(\d+)`)
	imdbPrefix = regexp.MustCompile(`imdb:(tt\d+)`)
	tvdbPrefix = regexp.MustCompile(`tvdb:(\d+)`)
	yearPrefix = regexp.MustCompile(`year:(\d{4})`)
)

func (p *plugin) SearchTitles(ctx context.Context, req *pluginv1.SearchTitlesRequest) (*pluginv1.SearchTitlesResponse, error) {
	s, err := p.session()
	if err != nil {
		return nil, err
	}
	query, kind := req.GetQuery(), req.GetMediaKind()
	movies := kind != pluginv1.MediaKind_MEDIA_KIND_SERIES
	series := kind != pluginv1.MediaKind_MEDIA_KIND_MOVIE
	adult := req.GetIncludeAdult() && s.adult
	var results []tmdbListResult
	switch {
	case tmdbPrefix.MatchString(query):
		id := tmdbPrefix.FindStringSubmatch(query)[1]
		results, err = s.byTMDBID(ctx, id, req.GetLanguage(), movies, series)
	case imdbPrefix.MatchString(query):
		results, err = s.byExternalID(ctx, imdbPrefix.FindStringSubmatch(query)[1], "imdb_id", req.GetLanguage())
	case tvdbPrefix.MatchString(query):
		results, err = s.byExternalID(ctx, tvdbPrefix.FindStringSubmatch(query)[1], "tvdb_id", req.GetLanguage())
	case yearPrefix.MatchString(query):
		year, _ := strconv.Atoi(yearPrefix.FindStringSubmatch(query)[1])
		name := strings.TrimSpace(yearPrefix.ReplaceAllString(query, ""))
		results, err = s.byYear(ctx, name, year, req.GetLanguage(), adult, movies, series)
	default:
		return s.searchPage(ctx, req, adult)
	}
	if err != nil {
		return nil, err
	}
	resp := pluginv1.SearchTitlesResponse_builder{Page: proto.Int32(1), TotalPages: proto.Int32(1)}.Build()
	for _, r := range results {
		t := r.title(r.MediaType)
		if t == nil || r.Adult && !adult || !movies && r.MediaType == "movie" || !series && r.MediaType == "tv" {
			continue
		}
		resp.SetTitles(append(resp.GetTitles(), t))
	}
	resp.SetTotalResults(int32(len(resp.GetTitles())))
	return resp, nil
}

// searchPage searches a page of TMDB's multi, movie or series search.
func (s session) searchPage(ctx context.Context, req *pluginv1.SearchTitlesRequest, adult bool) (*pluginv1.SearchTitlesResponse, error) {
	q := url.Values{
		"query": {req.GetQuery()}, "page": {strconv.Itoa(int(max(req.GetPage(), 1)))},
		"include_adult": {strconv.FormatBool(adult)},
	}
	setLanguage(q, req.GetLanguage())
	path, mediaType := "/search/multi", ""
	switch req.GetMediaKind() {
	case pluginv1.MediaKind_MEDIA_KIND_MOVIE:
		path, mediaType = "/search/movie", "movie"
		if req.GetYear() > 0 {
			q.Set("primary_release_year", strconv.Itoa(int(req.GetYear())))
		}
	case pluginv1.MediaKind_MEDIA_KIND_SERIES:
		path, mediaType = "/search/tv", "tv"
		if req.GetYear() > 0 {
			q.Set("first_air_date_year", strconv.Itoa(int(req.GetYear())))
		}
	}
	var page tmdbPage
	if err := s.get(ctx, path, q, &page); err != nil {
		return nil, err
	}
	resp := pluginv1.SearchTitlesResponse_builder{
		Page: proto.Int32(int32(page.Page)), TotalPages: proto.Int32(int32(page.TotalPages)), TotalResults: proto.Int32(int32(page.TotalResults)),
	}.Build()
	for _, r := range page.Results {
		if t := r.title(cmp.Or(r.MediaType, mediaType)); t != nil && (adult || !r.Adult) {
			resp.SetTitles(append(resp.GetTitles(), t))
		}
	}
	return resp, nil
}

// byTMDBID looks the movie and series of a TMDB ID up, as Seerr's tmdb
// search provider; IDs TMDB does not know are left out.
func (s session) byTMDBID(ctx context.Context, id, language string, movies, series bool) ([]tmdbListResult, error) {
	q := url.Values{}
	setLanguage(q, language)
	var kinds []string
	if movies {
		kinds = append(kinds, "movie")
	}
	if series {
		kinds = append(kinds, "tv")
	}
	var out []tmdbListResult
	for _, kind := range kinds {
		var r tmdbListResult
		switch err := s.get(ctx, "/"+kind+"/"+id, q, &r); {
		case errors.Is(err, errNotFound):
			continue
		case err != nil:
			return nil, err
		}
		r.MediaType = kind
		out = append(out, r)
	}
	return out, nil
}

// byExternalID finds the movies and series of an IMDb or TVDB ID, as
// Seerr's imdb and tvdb search providers.
func (s session) byExternalID(ctx context.Context, id, source, language string) ([]tmdbListResult, error) {
	q := url.Values{"external_source": {source}}
	setLanguage(q, language)
	var f struct {
		MovieResults []tmdbListResult `json:"movie_results"`
		TVResults    []tmdbListResult `json:"tv_results"`
	}
	switch err := s.get(ctx, "/find/"+url.PathEscape(id), q, &f); {
	case errors.Is(err, errNotFound):
		return nil, nil
	case err != nil:
		return nil, err
	}
	out := make([]tmdbListResult, 0, len(f.MovieResults)+len(f.TVResults))
	for _, r := range f.MovieResults {
		r.MediaType = "movie"
		out = append(out, r)
	}
	for _, r := range f.TVResults {
		r.MediaType = "tv"
		out = append(out, r)
	}
	return out, nil
}

// byYear searches the first pages of TMDB's movie and series searches of
// a year, as Seerr's year search provider.
func (s session) byYear(ctx context.Context, name string, year int, language string, adult, movies, series bool) ([]tmdbListResult, error) {
	type search struct{ path, param, mediaType string }
	var searches []search
	if movies {
		searches = append(searches, search{"/search/movie", "primary_release_year", "movie"})
	}
	if series {
		searches = append(searches, search{"/search/tv", "first_air_date_year", "tv"})
	}
	var out []tmdbListResult
	for _, sr := range searches {
		q := url.Values{"query": {name}, sr.param: {strconv.Itoa(year)}, "include_adult": {strconv.FormatBool(adult)}}
		setLanguage(q, language)
		var page tmdbPage
		if err := s.get(ctx, sr.path, q, &page); err != nil {
			return nil, err
		}
		for _, r := range page.Results {
			r.MediaType = sr.mediaType
			out = append(out, r)
		}
	}
	return out, nil
}
