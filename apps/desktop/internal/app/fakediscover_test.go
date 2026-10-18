package app

import (
	"context"
	"fmt"
	"net/http"
	"slices"

	acquisitionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1/acquisitionv1connect"
	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
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
		cat("popular", "Popular", pluginv1.CatalogKind_CATALOG_KIND_POPULAR, false),
		cat("similar", "Similar", pluginv1.CatalogKind_CATALOG_KIND_SIMILAR, false),
	}}.Build(), nil
}

func (d *fakeDiscover) Discover(_ context.Context, r *acquisitionv1.DiscoverRequest) (*acquisitionv1.DiscoverResponse, error) {
	d.f.mu.Lock()
	defer d.f.mu.Unlock()
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
			OfficialRating: new("PG-13"), RuntimeMinutes: new(int32(124)),
			Credits: []*acquisitionv1.TitleCredit{
				acquisitionv1.TitleCredit_builder{Name: new("Mara Quinn"), Kind: new(libraryv1.CreditKind_CREDIT_KIND_ACTOR), Role: new("Commander Vale"), Order: new(int32(0))}.Build(),
				acquisitionv1.TitleCredit_builder{Name: new("Theo Park"), Kind: new(libraryv1.CreditKind_CREDIT_KIND_ACTOR), Role: new("Ito"), Order: new(int32(1))}.Build(),
			},
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

func (d *fakeDiscover) GetSeason(_ context.Context, r *acquisitionv1.GetSeasonRequest) (*acquisitionv1.GetSeasonResponse, error) {
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
