package app

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"

	acquisitionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1/acquisitionv1connect"
	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	systemv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1/systemv1connect"
)

// fakeDiscover serves the server's catalogs and the titles in them.
type fakeDiscover struct {
	acquisitionv1connect.UnimplementedDiscoverServiceHandler
	f *fakeCore
}

// fakeRequests serves the requests made of titles, which it keeps.
type fakeRequests struct {
	acquisitionv1connect.UnimplementedRequestServiceHandler
	f *fakeCore
}

func (f *fakeCore) handleDiscover(mux *http.ServeMux) {
	mux.Handle(acquisitionv1connect.NewDiscoverServiceHandler(&fakeDiscover{f: f}))
	mux.Handle(acquisitionv1connect.NewRequestServiceHandler(&fakeRequests{f: f}))
	mux.Handle(systemv1connect.NewLocalizationServiceHandler(fakeLocalization{}))
}

var (
	movieKind  = acquisitionv1.MediaKind_MEDIA_KIND_MOVIE
	seriesKind = acquisitionv1.MediaKind_MEDIA_KIND_SERIES
)

// fakeTitles are the titles discovery offers: a movie the library has, a
// movie to request, and a series of two seasons to request.
var fakeTitles = []struct {
	kind   acquisitionv1.MediaKind
	tmdb   int32
	name   string
	year   int32
	itemID string
}{
	{movieKind, 101, "The Long Orbit", 2024, "m1"},
	{movieKind, 102, "Starfall", 2025, ""},
	{seriesKind, 201, "Northern Lights", 2023, ""},
}

func (d *fakeDiscover) status(kind acquisitionv1.MediaKind, tmdb int32, itemID string) acquisitionv1.MediaStatus {
	if itemID != "" {
		return acquisitionv1.MediaStatus_MEDIA_STATUS_AVAILABLE
	}
	if slices.ContainsFunc(d.f.titleRequests, func(r *acquisitionv1.Request) bool { return r.GetMediaKind() == kind && r.GetTmdbId() == tmdb }) {
		return acquisitionv1.MediaStatus_MEDIA_STATUS_REQUESTED
	}
	return acquisitionv1.MediaStatus_MEDIA_STATUS_UNSPECIFIED
}

func (d *fakeDiscover) ListCatalogs(context.Context, *acquisitionv1.ListCatalogsRequest) (*acquisitionv1.ListCatalogsResponse, error) {
	cat := func(id, name string, kind pluginv1.CatalogKind, mixed bool) *acquisitionv1.Catalog {
		return acquisitionv1.Catalog_builder{Id: &id, Name: &name, Kind: &kind, Mixed: &mixed, MediaKinds: []acquisitionv1.MediaKind{movieKind, seriesKind}}.Build()
	}
	return acquisitionv1.ListCatalogsResponse_builder{Catalogs: []*acquisitionv1.Catalog{
		cat("trending", "Trending", pluginv1.CatalogKind_CATALOG_KIND_TRENDING, true),
		acquisitionv1.Catalog_builder{
			Id: new("popular"), Name: new("Popular"), PluginId: new("dev.mavio.tmdb"), Kind: new(pluginv1.CatalogKind_CATALOG_KIND_POPULAR), MediaKinds: []acquisitionv1.MediaKind{movieKind, seriesKind},
			Filters: []pluginv1.DiscoverFilter{
				pluginv1.DiscoverFilter_DISCOVER_FILTER_GENRE, pluginv1.DiscoverFilter_DISCOVER_FILTER_KEYWORD, pluginv1.DiscoverFilter_DISCOVER_FILTER_STUDIO,
				pluginv1.DiscoverFilter_DISCOVER_FILTER_ORIGINAL_LANGUAGE, pluginv1.DiscoverFilter_DISCOVER_FILTER_RELEASE_DATE, pluginv1.DiscoverFilter_DISCOVER_FILTER_RATING,
			},
			Sorts: []pluginv1.DiscoverSort{pluginv1.DiscoverSort_DISCOVER_SORT_POPULARITY, pluginv1.DiscoverSort_DISCOVER_SORT_RELEASE_DATE},
		}.Build(),
		cat("similar", "Similar", pluginv1.CatalogKind_CATALOG_KIND_SIMILAR, false),
	}}.Build(), nil
}

func (d *fakeDiscover) Discover(_ context.Context, r *acquisitionv1.DiscoverRequest) (*acquisitionv1.DiscoverResponse, error) {
	d.f.mu.Lock()
	defer d.f.mu.Unlock()
	d.f.discovers = append(d.f.discovers, r)
	var titles []*acquisitionv1.DiscoveredTitle
	for _, t := range fakeTitles {
		if r.GetMediaKind() != acquisitionv1.MediaKind_MEDIA_KIND_UNSPECIFIED && r.GetMediaKind() != t.kind || r.GetTmdbId() == t.tmdb {
			continue
		}
		titles = append(titles, acquisitionv1.DiscoveredTitle_builder{
			MediaKind: new(t.kind), Name: new(t.name), Year: new(t.year), ExternalIds: map[string]string{"tmdb": fmt.Sprint(t.tmdb)},
			Status: new(d.status(t.kind, t.tmdb, t.itemID)), ItemId: new(t.itemID), Requestable: new(t.itemID == ""),
		}.Build())
	}
	return acquisitionv1.DiscoverResponse_builder{Titles: titles, Page: new(r.GetPage()), TotalPages: new(int32(2))}.Build(), nil
}

func (d *fakeDiscover) GetTitle(_ context.Context, r *acquisitionv1.GetTitleRequest) (*acquisitionv1.GetTitleResponse, error) {
	d.f.mu.Lock()
	defer d.f.mu.Unlock()
	for _, t := range fakeTitles {
		if t.kind != r.GetMediaKind() || t.tmdb != r.GetTmdbId() {
			continue
		}
		status := d.status(t.kind, t.tmdb, t.itemID)
		b := acquisitionv1.Title_builder{
			MediaKind: new(t.kind), TmdbId: new(t.tmdb), Name: new(t.name), Year: new(t.year), ItemId: new(t.itemID),
			Status: new(status), Requestable: new(t.itemID == "" && status == acquisitionv1.MediaStatus_MEDIA_STATUS_UNSPECIFIED),
			Overview: new("A crew far from home finds the way back."), Tagline: new("Every orbit ends."), Genres: []string{"Science Fiction", "Drama"},
			GenreRefs:      []*pluginv1.Reference{pluginv1.Reference_builder{Id: new("878"), Name: new("Science Fiction")}.Build(), pluginv1.Reference_builder{Id: new("18"), Name: new("Drama")}.Build()},
			OfficialRating: new("PG-13"), RuntimeMinutes: new(int32(124)),
			OriginalLanguage: new("en"), ProductionCountries: []string{"US", "NZ"}, Budget: new(int64(90_000_000)), VoteCount: new(int32(1200)),
			CommunityRating: new(7.6), PremiereDate: new("2024-03-08"), ProductionStatus: new(pluginv1.ProductionStatus_PRODUCTION_STATUS_RELEASED),
			KeywordRefs: []*pluginv1.Reference{pluginv1.Reference_builder{Id: new("k1"), Name: new("space")}.Build(), pluginv1.Reference_builder{Id: new("k2"), Name: new("homecoming")}.Build()},
			StudioRefs:  []*pluginv1.Reference{pluginv1.Reference_builder{Id: new("s1"), Name: new("Meridian Pictures")}.Build()},
			Credits: []*acquisitionv1.TitleCredit{
				acquisitionv1.TitleCredit_builder{Name: new("Mara Quinn"), Kind: new(libraryv1.CreditKind_CREDIT_KIND_ACTOR), Role: new("Commander Vale"), Order: new(int32(0)), ExternalIds: map[string]string{"tmdb": "301"}}.Build(),
				acquisitionv1.TitleCredit_builder{Name: new("Iris Okafor"), Kind: new(libraryv1.CreditKind_CREDIT_KIND_DIRECTOR), ExternalIds: map[string]string{"tmdb": "302"}}.Build(),
				acquisitionv1.TitleCredit_builder{Name: new("Jonah Reyes"), Kind: new(libraryv1.CreditKind_CREDIT_KIND_WRITER), Role: new("Screenplay")}.Build(),
				acquisitionv1.TitleCredit_builder{Name: new("Theo Park"), Kind: new(libraryv1.CreditKind_CREDIT_KIND_ACTOR), Role: new("Ito"), Order: new(int32(1))}.Build(),
			},
		}
		if t.kind == movieKind {
			b.ExternalIds = map[string]string{"tmdb": fmt.Sprint(t.tmdb), "imdb": fmt.Sprintf("tt%07d", t.tmdb), "tmdb_collection": "500"}
			b.CollectionName = new("The Orbit Collection")
		}
		for _, req := range d.f.titleRequests {
			if req.GetMediaKind() == t.kind && req.GetTmdbId() == t.tmdb {
				b.Requests = append(b.Requests, req)
			}
		}
		if t.kind == seriesKind {
			for n := int32(1); n <= 2; n++ {
				b.Seasons = append(b.Seasons, acquisitionv1.SeasonStatus_builder{SeasonNumber: new(n), EpisodeCount: new(int32(8)), Requestable: new(status == acquisitionv1.MediaStatus_MEDIA_STATUS_UNSPECIFIED)}.Build())
			}
		}
		return acquisitionv1.GetTitleResponse_builder{Title: b.Build()}.Build(), nil
	}
	return &acquisitionv1.GetTitleResponse{}, nil
}

func (d *fakeDiscover) GetSeason(ctx context.Context, r *acquisitionv1.GetSeasonRequest) (*acquisitionv1.GetSeasonResponse, error) {
	if g := d.f.seasonGate; g != nil {
		select {
		case <-g:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	var episodes []*acquisitionv1.SeasonEpisode
	for n := int32(1); n <= 3; n++ {
		episodes = append(episodes, acquisitionv1.SeasonEpisode_builder{
			EpisodeNumber: new(n), Name: new(fmt.Sprintf("Aurora %d", n)), AirDate: new(fmt.Sprintf("2023-0%d-1%d", r.GetSeasonNumber(), n)),
			Overview: new("The lights return over the bay."),
		}.Build())
	}
	return acquisitionv1.GetSeasonResponse_builder{Episodes: episodes}.Build(), nil
}

func (q *fakeRequests) CreateRequest(_ context.Context, r *acquisitionv1.CreateRequestRequest) (*acquisitionv1.CreateRequestResponse, error) {
	q.f.mu.Lock()
	defer q.f.mu.Unlock()
	name := ""
	var year int32
	for _, t := range fakeTitles {
		if t.kind == r.GetMediaKind() && t.tmdb == r.GetTmdbId() {
			name, year = t.name, t.year
		}
	}
	seasons := r.GetSeasons()
	if r.GetAllSeasons() {
		seasons = []int32{1, 2}
	}
	b := acquisitionv1.Request_builder{
		Id: new(fmt.Sprintf("00000000-0000-4000-8000-%012d", len(q.f.titleRequests)+1)), MediaKind: new(r.GetMediaKind()), TmdbId: new(r.GetTmdbId()),
		Title: &name, Year: &year, Status: new(acquisitionv1.RequestStatus_REQUEST_STATUS_PENDING), RequestedBy: new("u1"), CreateTime: tsAgo(0),
	}
	for _, n := range seasons {
		b.Seasons = append(b.Seasons, acquisitionv1.SeasonRequest_builder{SeasonNumber: new(n)}.Build())
	}
	req := b.Build()
	q.f.titleRequests = append(q.f.titleRequests, req)
	return acquisitionv1.CreateRequestResponse_builder{Request: req}.Build(), nil
}

func (q *fakeRequests) ListRequests(_ context.Context, r *acquisitionv1.ListRequestsRequest) (*acquisitionv1.ListRequestsResponse, error) {
	q.f.mu.Lock()
	defer q.f.mu.Unlock()
	var list []*acquisitionv1.Request
	for _, req := range slices.Backward(q.f.titleRequests) {
		if len(r.GetStatuses()) == 0 || slices.Contains(r.GetStatuses(), req.GetStatus()) {
			list = append(list, req)
		}
	}
	return acquisitionv1.ListRequestsResponse_builder{Requests: list, Total: new(int32(len(list)))}.Build(), nil
}

func (q *fakeRequests) find(id string) *acquisitionv1.Request {
	for _, r := range q.f.titleRequests {
		if r.GetId() == id {
			return r
		}
	}
	return nil
}

func (q *fakeRequests) ApproveRequest(_ context.Context, r *acquisitionv1.ApproveRequestRequest) (*acquisitionv1.ApproveRequestResponse, error) {
	q.f.mu.Lock()
	defer q.f.mu.Unlock()
	q.find(r.GetId()).SetStatus(acquisitionv1.RequestStatus_REQUEST_STATUS_APPROVED)
	return &acquisitionv1.ApproveRequestResponse{}, nil
}

func (q *fakeRequests) UpdateRequest(_ context.Context, r *acquisitionv1.UpdateRequestRequest) (*acquisitionv1.UpdateRequestResponse, error) {
	q.f.mu.Lock()
	defer q.f.mu.Unlock()
	var seasons []*acquisitionv1.SeasonRequest
	for _, n := range r.GetSeasons() {
		seasons = append(seasons, acquisitionv1.SeasonRequest_builder{SeasonNumber: new(n)}.Build())
	}
	req := q.find(r.GetId())
	req.SetSeasons(seasons)
	return acquisitionv1.UpdateRequestResponse_builder{Request: req}.Build(), nil
}

func (q *fakeRequests) DeleteRequest(_ context.Context, r *acquisitionv1.DeleteRequestRequest) (*acquisitionv1.DeleteRequestResponse, error) {
	q.f.mu.Lock()
	defer q.f.mu.Unlock()
	q.f.titleRequests = slices.DeleteFunc(q.f.titleRequests, func(x *acquisitionv1.Request) bool { return x.GetId() == r.GetId() })
	return &acquisitionv1.DeleteRequestResponse{}, nil
}

func (q *fakeRequests) GetQuota(context.Context, *acquisitionv1.GetQuotaRequest) (*acquisitionv1.GetQuotaResponse, error) {
	return &acquisitionv1.GetQuotaResponse{}, nil
}

func (d *fakeDiscover) SearchTitles(_ context.Context, r *acquisitionv1.SearchTitlesRequest) (*acquisitionv1.SearchTitlesResponse, error) {
	d.f.mu.Lock()
	defer d.f.mu.Unlock()
	var titles []*acquisitionv1.DiscoveredTitle
	for i, t := range fakeTitles {
		if strings.Contains(strings.ToLower(t.name), strings.ToLower(r.GetQuery())) {
			titles = append(titles, d.discovered(i))
		}
	}
	return acquisitionv1.SearchTitlesResponse_builder{Titles: titles}.Build(), nil
}

func (d *fakeDiscover) discovered(i int) *acquisitionv1.DiscoveredTitle {
	t := fakeTitles[i]
	status := d.status(t.kind, t.tmdb, t.itemID)
	if t.kind == movieKind && slices.Contains(d.f.blocked, 500) {
		status = acquisitionv1.MediaStatus_MEDIA_STATUS_BLOCKLISTED
	}
	return acquisitionv1.DiscoveredTitle_builder{
		MediaKind: new(t.kind), Name: new(t.name), Year: new(t.year), ExternalIds: map[string]string{"tmdb": fmt.Sprint(t.tmdb)},
		Status: new(status), ItemId: new(t.itemID), Requestable: new(t.itemID == "" && status == acquisitionv1.MediaStatus_MEDIA_STATUS_UNSPECIFIED),
	}.Build()
}

func (d *fakeDiscover) GetPerson(_ context.Context, r *acquisitionv1.GetPersonRequest) (*acquisitionv1.GetPersonResponse, error) {
	d.f.mu.Lock()
	defer d.f.mu.Unlock()
	if r.GetTmdbId() != 301 {
		return &acquisitionv1.GetPersonResponse{}, nil
	}
	return acquisitionv1.GetPersonResponse_builder{Person: acquisitionv1.Person_builder{
		Name: new("Mara Quinn"), Biography: new("Mara Quinn is an actor known for quiet, exacting roles."), Birthday: new("1984-05-02"),
		PlaceOfBirth: new("Wellington, New Zealand"), AlsoKnownAs: []string{"M. Quinn"}, KnownForDepartment: new("Acting"),
		ExternalIds: map[string]string{"tmdb": "301", "imdb": "nm0000301"},
		Cast: []*acquisitionv1.PersonTitle{
			acquisitionv1.PersonTitle_builder{Title: d.discovered(0), Role: new("Commander Vale")}.Build(),
			acquisitionv1.PersonTitle_builder{Title: d.discovered(2), Role: new("Signe")}.Build(),
		},
		Crew: []*acquisitionv1.PersonTitle{acquisitionv1.PersonTitle_builder{Title: d.discovered(1), Role: new("Producer")}.Build()},
	}.Build()}.Build(), nil
}

func (d *fakeDiscover) GetCollection(_ context.Context, r *acquisitionv1.GetCollectionRequest) (*acquisitionv1.GetCollectionResponse, error) {
	d.f.mu.Lock()
	defer d.f.mu.Unlock()
	if r.GetTmdbCollectionId() != 500 {
		return &acquisitionv1.GetCollectionResponse{}, nil
	}
	return acquisitionv1.GetCollectionResponse_builder{Collection: acquisitionv1.Collection_builder{
		Name: new("The Orbit Collection"), Overview: new("Two journeys to the edge and back."),
		Parts: []*acquisitionv1.DiscoveredTitle{d.discovered(0), d.discovered(1)},
	}.Build()}.Build(), nil
}

func (d *fakeDiscover) GetRatings(context.Context, *acquisitionv1.GetRatingsRequest) (*acquisitionv1.GetRatingsResponse, error) {
	return acquisitionv1.GetRatingsResponse_builder{Ratings: []*pluginv1.Rating{
		pluginv1.Rating_builder{Source: new(pluginv1.RatingSource_RATING_SOURCE_ROTTEN_TOMATOES_CRITICS), Score: new(91.0), Url: new("https://www.rottentomatoes.com/m/the_long_orbit")}.Build(),
		pluginv1.Rating_builder{Source: new(pluginv1.RatingSource_RATING_SOURCE_IMDB), Score: new(7.8), Votes: new(int64(125_000))}.Build(),
	}}.Build(), nil
}

func (d *fakeDiscover) ListFilterValues(_ context.Context, r *acquisitionv1.ListFilterValuesRequest) (*acquisitionv1.ListFilterValuesResponse, error) {
	v := func(id, name string) *pluginv1.FilterValue {
		return pluginv1.FilterValue_builder{Id: &id, Name: &name}.Build()
	}
	switch r.GetFilter() {
	case pluginv1.DiscoverFilter_DISCOVER_FILTER_GENRE:
		return acquisitionv1.ListFilterValuesResponse_builder{Values: []*pluginv1.FilterValue{v("878", "Science Fiction"), v("18", "Drama"), v("35", "Comedy")}}.Build(), nil
	case pluginv1.DiscoverFilter_DISCOVER_FILTER_KEYWORD:
		if ids := r.GetIds(); len(ids) > 0 {
			var out []*pluginv1.FilterValue
			for _, id := range ids {
				out = append(out, v(id, "keyword "+id))
			}
			return acquisitionv1.ListFilterValuesResponse_builder{Values: out}.Build(), nil
		}
		return acquisitionv1.ListFilterValuesResponse_builder{Values: []*pluginv1.FilterValue{v("k1", "space"), v("k3", "space station")}}.Build(), nil
	}
	return &acquisitionv1.ListFilterValuesResponse{}, nil
}

func (q *fakeRequests) BlockCollection(_ context.Context, r *acquisitionv1.BlockCollectionRequest) (*acquisitionv1.BlockCollectionResponse, error) {
	q.f.mu.Lock()
	defer q.f.mu.Unlock()
	q.f.blocked = append(q.f.blocked, r.GetTmdbCollectionId())
	return &acquisitionv1.BlockCollectionResponse{}, nil
}

func (q *fakeRequests) UnblockCollection(_ context.Context, r *acquisitionv1.UnblockCollectionRequest) (*acquisitionv1.UnblockCollectionResponse, error) {
	q.f.mu.Lock()
	defer q.f.mu.Unlock()
	q.f.blocked = slices.DeleteFunc(q.f.blocked, func(id int32) bool { return id == r.GetTmdbCollectionId() })
	return &acquisitionv1.UnblockCollectionResponse{}, nil
}

// fakeLocalization serves the languages and countries a server knows.
type fakeLocalization struct {
	systemv1connect.UnimplementedLocalizationServiceHandler
}

func (fakeLocalization) ListLanguages(context.Context, *systemv1.ListLanguagesRequest) (*systemv1.ListLanguagesResponse, error) {
	return systemv1.ListLanguagesResponse_builder{Languages: []*systemv1.Language{
		systemv1.Language_builder{Code: new("en"), Name: new("English")}.Build(),
		systemv1.Language_builder{Code: new("ja"), Name: new("Japanese")}.Build(),
	}}.Build(), nil
}

func (fakeLocalization) ListCountries(context.Context, *systemv1.ListCountriesRequest) (*systemv1.ListCountriesResponse, error) {
	return systemv1.ListCountriesResponse_builder{Countries: []*systemv1.Country{
		systemv1.Country_builder{Code: new("US"), Name: new("United States")}.Build(),
	}}.Build(), nil
}

func (q *fakeRequests) GetRequestSettings(context.Context, *acquisitionv1.GetRequestSettingsRequest) (*acquisitionv1.GetRequestSettingsResponse, error) {
	q.f.mu.Lock()
	defer q.f.mu.Unlock()
	if q.f.requestSettings == nil {
		q.f.requestSettings = acquisitionv1.RequestSettings_builder{
			DefaultRequestMovies: new(true), SeasonFolder: new(true), BlocklistedTags: []string{"dev.mavio.tmdb/k9"}, BlocklistedTagsLimit: new(int32(50)),
		}.Build()
	}
	return acquisitionv1.GetRequestSettingsResponse_builder{Settings: q.f.requestSettings}.Build(), nil
}

func (q *fakeRequests) UpdateRequestSettings(_ context.Context, r *acquisitionv1.UpdateRequestSettingsRequest) (*acquisitionv1.UpdateRequestSettingsResponse, error) {
	q.f.mu.Lock()
	defer q.f.mu.Unlock()
	q.f.requestSettings = r.GetSettings()
	return acquisitionv1.UpdateRequestSettingsResponse_builder{Settings: r.GetSettings()}.Build(), nil
}
