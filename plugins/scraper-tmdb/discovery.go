package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

// Discovery follows Seerr's use of TMDB: trending titles of the day,
// popular and upcoming titles through TMDB's discover endpoints with
// their filters, top rated titles, and the recommendations and similar
// titles of a title.

const (
	catalogTrending        = "trending"
	catalogPopular         = "popular"
	catalogUpcoming        = "upcoming"
	catalogTopRated        = "top_rated"
	catalogRecommendations = "recommendations"
	catalogSimilar         = "similar"
)

var (
	movieAndSeries  = []pluginv1.MediaKind{pluginv1.MediaKind_MEDIA_KIND_MOVIE, pluginv1.MediaKind_MEDIA_KIND_SERIES}
	discoverFilters = []pluginv1.DiscoverFilter{
		pluginv1.DiscoverFilter_DISCOVER_FILTER_GENRE, pluginv1.DiscoverFilter_DISCOVER_FILTER_STUDIO,
		pluginv1.DiscoverFilter_DISCOVER_FILTER_NETWORK, pluginv1.DiscoverFilter_DISCOVER_FILTER_KEYWORD,
		pluginv1.DiscoverFilter_DISCOVER_FILTER_WATCH_PROVIDER, pluginv1.DiscoverFilter_DISCOVER_FILTER_ORIGINAL_LANGUAGE,
		pluginv1.DiscoverFilter_DISCOVER_FILTER_RELEASE_DATE, pluginv1.DiscoverFilter_DISCOVER_FILTER_RUNTIME,
		pluginv1.DiscoverFilter_DISCOVER_FILTER_RATING, pluginv1.DiscoverFilter_DISCOVER_FILTER_VOTE_COUNT,
		pluginv1.DiscoverFilter_DISCOVER_FILTER_CERTIFICATION,
	}
	discoverSorts = []pluginv1.DiscoverSort{
		pluginv1.DiscoverSort_DISCOVER_SORT_POPULARITY, pluginv1.DiscoverSort_DISCOVER_SORT_RELEASE_DATE,
		pluginv1.DiscoverSort_DISCOVER_SORT_RATING, pluginv1.DiscoverSort_DISCOVER_SORT_VOTE_COUNT,
		pluginv1.DiscoverSort_DISCOVER_SORT_TITLE,
	}
)

func (p *plugin) ListCatalogs(context.Context, *pluginv1.ListCatalogsRequest) (*pluginv1.ListCatalogsResponse, error) {
	catalog := func(id, name string, kind pluginv1.CatalogKind, kinds []pluginv1.MediaKind, filtered bool) *pluginv1.Catalog {
		c := pluginv1.Catalog_builder{Id: proto.String(id), Name: proto.String(name), Kind: kind.Enum(), MediaKinds: kinds}.Build()
		if filtered {
			c.SetFilters(discoverFilters)
			c.SetSorts(discoverSorts)
		}
		return c
	}
	return pluginv1.ListCatalogsResponse_builder{Catalogs: []*pluginv1.Catalog{
		catalog(catalogTrending, "Trending", pluginv1.CatalogKind_CATALOG_KIND_TRENDING,
			append([]pluginv1.MediaKind{pluginv1.MediaKind_MEDIA_KIND_UNSPECIFIED}, movieAndSeries...), false),
		catalog(catalogPopular, "Popular", pluginv1.CatalogKind_CATALOG_KIND_POPULAR, movieAndSeries, true),
		catalog(catalogUpcoming, "Upcoming", pluginv1.CatalogKind_CATALOG_KIND_UPCOMING, movieAndSeries, true),
		catalog(catalogTopRated, "Top Rated", pluginv1.CatalogKind_CATALOG_KIND_TOP_RATED, movieAndSeries, false),
		catalog(catalogRecommendations, "Recommendations", pluginv1.CatalogKind_CATALOG_KIND_RECOMMENDATIONS, movieAndSeries, false),
		catalog(catalogSimilar, "Similar", pluginv1.CatalogKind_CATALOG_KIND_SIMILAR, movieAndSeries, false),
	}}.Build(), nil
}

func (p *plugin) Discover(ctx context.Context, req *pluginv1.DiscoverRequest) (*pluginv1.DiscoverResponse, error) {
	s, err := p.session()
	if err != nil {
		return nil, err
	}
	series := req.GetMediaKind() == pluginv1.MediaKind_MEDIA_KIND_SERIES
	kind := "movie"
	if series {
		kind = "tv"
	}
	q := url.Values{"page": {strconv.Itoa(int(max(req.GetPage(), 1)))}}
	setLanguage(q, req.GetLanguage())
	var path string
	switch req.GetCatalog() {
	case catalogTrending:
		if req.GetMediaKind() == pluginv1.MediaKind_MEDIA_KIND_UNSPECIFIED {
			kind = "all"
		}
		path = "/trending/" + kind + "/day"
	case catalogPopular, catalogUpcoming:
		path = "/discover/" + kind
		discoverQuery(q, req, series, p.clock())
	case catalogTopRated:
		path = "/" + kind + "/top_rated"
	case catalogRecommendations, catalogSimilar:
		id, err := s.titleID(ctx, req.GetExternalIds(), series)
		if err != nil {
			return nil, err
		}
		if id == 0 {
			return &pluginv1.DiscoverResponse{}, nil
		}
		path = fmt.Sprintf("/%s/%d/%s", kind, id, req.GetCatalog())
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("unknown catalog %q", req.GetCatalog()))
	}
	if req.GetRegion() != "" && !series {
		q.Set("region", req.GetRegion())
	}
	var page tmdbPage
	if err := s.get(ctx, path, q, &page); err != nil {
		if errors.Is(err, errNotFound) {
			return &pluginv1.DiscoverResponse{}, nil
		}
		return nil, err
	}
	resp := pluginv1.DiscoverResponse_builder{
		Page: proto.Int32(int32(page.Page)), TotalPages: proto.Int32(int32(page.TotalPages)), TotalResults: proto.Int32(int32(page.TotalResults)),
	}.Build()
	for _, r := range page.Results {
		mediaKind := r.MediaType
		if mediaKind == "" {
			mediaKind = kind
		}
		if t := r.title(mediaKind); t != nil && (s.adult || !r.Adult) {
			resp.SetTitles(append(resp.GetTitles(), t))
		}
	}
	return resp, nil
}

// titleID returns the TMDB ID of a movie or series named by its external
// IDs; 0 when TMDB does not know it.
func (s session) titleID(ctx context.Context, ids map[string]string, series bool) (int, error) {
	if id, ok := parseTMDBID(ids[keyTMDB]); ok {
		return id, nil
	}
	for _, src := range []struct{ key, source string }{{keyIMDb, "imdb_id"}, {keyTVDB, "tvdb_id"}} {
		if ids[src.key] == "" {
			continue
		}
		f, err := s.find(ctx, ids[src.key], src.source)
		if err != nil && !errors.Is(err, errNotFound) {
			return 0, err
		}
		if !series && len(f.MovieResults) > 0 {
			return f.MovieResults[0].ID, nil
		}
		if series && len(f.TVResults) > 0 {
			return f.TVResults[0].ID, nil
		}
	}
	return 0, nil
}

// discoverQuery sets the parameters of TMDB's discover endpoints, as
// Seerr's discover routes do; upcoming titles are released from today.
func discoverQuery(q url.Values, req *pluginv1.DiscoverRequest, series bool, now time.Time) {
	set := func(key, value string) {
		if value != "" {
			q.Set(key, value)
		}
	}
	anyOf := func(key string, values []string) { set(key, strings.Join(values, "|")) }
	anyOf("with_genres", req.GetGenres())
	if series {
		anyOf("with_networks", req.GetNetworks())
	} else {
		anyOf("with_companies", req.GetStudios())
	}
	anyOf("with_keywords", req.GetKeywords())
	set("without_keywords", strings.Join(req.GetExcludedKeywords(), ","))
	if len(req.GetWatchProviders()) > 0 {
		anyOf("with_watch_providers", req.GetWatchProviders())
		set("watch_region", cmp.Or(req.GetWatchRegion(), req.GetRegion(), "US"))
	}
	set("with_original_language", req.GetOriginalLanguage())
	date := "primary_release_date"
	if series {
		date = "first_air_date"
	}
	from, to := req.GetReleasedFrom(), req.GetReleasedTo()
	if req.GetCatalog() == catalogUpcoming {
		if today := now.UTC().Format(time.DateOnly); from < today {
			from = today
		}
	}
	// TMDB needs both ends of a range when one is set.
	if from != "" || to != "" {
		set(date+".gte", cmp.Or(from, "1900-01-01"))
		set(date+".lte", cmp.Or(to, now.UTC().AddDate(1, 6, 0).Format(time.DateOnly)))
	}
	if req.HasRuntimeMin() {
		set("with_runtime.gte", strconv.Itoa(int(req.GetRuntimeMin())))
	}
	if req.HasRuntimeMax() {
		set("with_runtime.lte", strconv.Itoa(int(req.GetRuntimeMax())))
	}
	if req.HasRatingMin() {
		set("vote_average.gte", strconv.FormatFloat(req.GetRatingMin(), 'f', -1, 64))
	}
	if req.HasRatingMax() {
		set("vote_average.lte", strconv.FormatFloat(req.GetRatingMax(), 'f', -1, 64))
	}
	if req.HasVotesMin() {
		set("vote_count.gte", strconv.Itoa(int(req.GetVotesMin())))
	}
	if req.GetCertificationCountry() != "" && len(req.GetCertifications()) > 0 {
		set("certification_country", req.GetCertificationCountry())
		anyOf("certification", req.GetCertifications())
	}
	q.Set("include_adult", strconv.FormatBool(req.GetIncludeAdult()))
	q.Set("sort_by", sortBy(req.GetSort(), req.GetAscending(), series))
}

// sortBy returns TMDB's sort_by of an order; descending unless asked,
// popularity by default.
func sortBy(sort pluginv1.DiscoverSort, ascending, series bool) string {
	field := "popularity"
	switch sort {
	case pluginv1.DiscoverSort_DISCOVER_SORT_RELEASE_DATE:
		field = "primary_release_date"
		if series {
			field = "first_air_date"
		}
	case pluginv1.DiscoverSort_DISCOVER_SORT_RATING:
		field = "vote_average"
	case pluginv1.DiscoverSort_DISCOVER_SORT_VOTE_COUNT:
		field = "vote_count"
	case pluginv1.DiscoverSort_DISCOVER_SORT_TITLE:
		field = "title"
		if series {
			field = "name"
		}
	}
	if ascending {
		return field + ".asc"
	}
	return field + ".desc"
}

func (p *plugin) ListFilterValues(ctx context.Context, req *pluginv1.ListFilterValuesRequest) (*pluginv1.ListFilterValuesResponse, error) {
	s, err := p.session()
	if err != nil {
		return nil, err
	}
	kind := "movie"
	if req.GetMediaKind() == pluginv1.MediaKind_MEDIA_KIND_SERIES {
		kind = "tv"
	}
	q := url.Values{}
	setLanguage(q, req.GetLanguage())
	var values []tmdbFilterValue
	switch req.GetFilter() {
	case pluginv1.DiscoverFilter_DISCOVER_FILTER_GENRE:
		var list struct {
			Genres []tmdbFilterValue `json:"genres"`
		}
		err, values = s.get(ctx, "/genre/"+kind+"/list", q, &list), list.Genres
	case pluginv1.DiscoverFilter_DISCOVER_FILTER_WATCH_PROVIDER:
		set := func(k, v string) {
			if v != "" {
				q.Set(k, v)
			}
		}
		set("watch_region", req.GetRegion())
		var list struct {
			Results []tmdbFilterValue `json:"results"`
		}
		err, values = s.get(ctx, "/watch/providers/"+kind, q, &list), list.Results
	case pluginv1.DiscoverFilter_DISCOVER_FILTER_KEYWORD:
		values, err = s.filterValues(ctx, "keyword", req.GetQuery(), req.GetIds())
	case pluginv1.DiscoverFilter_DISCOVER_FILTER_STUDIO:
		values, err = s.filterValues(ctx, "company", req.GetQuery(), req.GetIds())
	case pluginv1.DiscoverFilter_DISCOVER_FILTER_NETWORK:
		// TMDB searches no networks: they are looked up by ID only.
		values, err = s.filterValues(ctx, "network", "", req.GetIds())
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("filter %v has no values", req.GetFilter()))
	}
	if err != nil {
		return nil, err
	}
	resp := &pluginv1.ListFilterValuesResponse{}
	query := strings.ToLower(req.GetQuery())
	for _, v := range values {
		if req.GetFilter() != pluginv1.DiscoverFilter_DISCOVER_FILTER_KEYWORD && req.GetFilter() != pluginv1.DiscoverFilter_DISCOVER_FILTER_STUDIO &&
			query != "" && !strings.Contains(strings.ToLower(v.name()), query) {
			continue
		}
		fv := pluginv1.FilterValue_builder{Id: proto.String(strconv.Itoa(v.id())), Name: proto.String(v.name())}.Build()
		if v.Logo != "" {
			fv.SetImageUrl(imageURL(v.Logo))
		}
		resp.SetValues(append(resp.GetValues(), fv))
	}
	return resp, nil
}

// filterValues searches keywords or companies by name, or looks keywords,
// companies or networks up by ID.
func (s session) filterValues(ctx context.Context, kind, query string, ids []string) ([]tmdbFilterValue, error) {
	if query != "" {
		var page struct {
			Results []tmdbFilterValue `json:"results"`
		}
		err := s.get(ctx, "/search/"+kind, url.Values{"query": {query}}, &page)
		return page.Results, err
	}
	var out []tmdbFilterValue
	for _, id := range ids {
		n, ok := parseTMDBID(id)
		if !ok {
			continue
		}
		var v tmdbFilterValue
		switch err := s.get(ctx, fmt.Sprintf("/%s/%d", kind, n), nil, &v); {
		case errors.Is(err, errNotFound):
		case err != nil:
			return nil, err
		default:
			out = append(out, v)
		}
	}
	return out, nil
}

// The subset of TMDB's lists the plugin uses.
type (
	tmdbPage struct {
		Page         int              `json:"page"`
		TotalPages   int              `json:"total_pages"`
		TotalResults int              `json:"total_results"`
		Results      []tmdbListResult `json:"results"`
	}
	tmdbListResult struct {
		ID               int     `json:"id"`
		MediaType        string  `json:"media_type"`
		Title            string  `json:"title"`
		Name             string  `json:"name"`
		OriginalTitle    string  `json:"original_title"`
		OriginalName     string  `json:"original_name"`
		OriginalLanguage string  `json:"original_language"`
		Overview         string  `json:"overview"`
		ReleaseDate      string  `json:"release_date"`
		FirstAirDate     string  `json:"first_air_date"`
		GenreIDs         []int   `json:"genre_ids"`
		PosterPath       string  `json:"poster_path"`
		BackdropPath     string  `json:"backdrop_path"`
		VoteAverage      float64 `json:"vote_average"`
		VoteCount        int     `json:"vote_count"`
		Popularity       float64 `json:"popularity"`
		Adult            bool    `json:"adult"`
	}
	// tmdbFilterValue is a genre, keyword, company, network or watch
	// provider.
	tmdbFilterValue struct {
		ID           int    `json:"id"`
		ProviderID   int    `json:"provider_id"`
		Name         string `json:"name"`
		ProviderName string `json:"provider_name"`
		Logo         string `json:"logo_path"`
	}
)

func (v tmdbFilterValue) id() int {
	if v.ProviderID != 0 {
		return v.ProviderID
	}
	return v.ID
}

func (v tmdbFilterValue) name() string { return cmp.Or(v.Name, v.ProviderName) }

// title returns a movie ("movie") or series ("tv"); nil for persons.
func (r tmdbListResult) title(mediaType string) *pluginv1.DiscoveredTitle {
	t := pluginv1.DiscoveredTitle_builder{
		OriginalLanguage: proto.String(r.OriginalLanguage), Overview: proto.String(r.Overview),
		ExternalIds:     map[string]string{keyTMDB: strconv.Itoa(r.ID)},
		CommunityRating: proto.Float64(r.VoteAverage), VoteCount: proto.Int32(int32(r.VoteCount)),
		Popularity: proto.Float64(r.Popularity), Adult: proto.Bool(r.Adult),
	}.Build()
	var date string
	switch mediaType {
	case "movie":
		t.SetMediaKind(pluginv1.MediaKind_MEDIA_KIND_MOVIE)
		t.SetName(r.Title)
		t.SetOriginalTitle(r.OriginalTitle)
		date = r.ReleaseDate
	case "tv":
		t.SetMediaKind(pluginv1.MediaKind_MEDIA_KIND_SERIES)
		t.SetName(r.Name)
		t.SetOriginalTitle(r.OriginalName)
		date = r.FirstAirDate
	default:
		return nil
	}
	if date != "" {
		t.SetPremiereDate(date)
		if y, ok := yearOf(date); ok {
			t.SetYear(int32(y))
		}
	}
	for _, g := range r.GenreIDs {
		t.SetGenres(append(t.GetGenres(), strconv.Itoa(g)))
	}
	if r.PosterPath != "" {
		t.SetPosterUrl(imageURL(r.PosterPath))
	}
	if r.BackdropPath != "" {
		t.SetBackdropUrl(imageURL(r.BackdropPath))
	}
	return t
}
