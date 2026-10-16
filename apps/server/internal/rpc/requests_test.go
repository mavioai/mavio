package rpc

import (
	"context"
	"net/http"
	"reflect"
	"slices"
	"strconv"
	"testing"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/apps/server/internal/activity"
	"github.com/mavioai/mavio/apps/server/internal/auth"
	"github.com/mavioai/mavio/apps/server/internal/requests"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
	"github.com/mavioai/mavio/libs/metadata"
	acquisitionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1/acquisitionv1connect"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	systemv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1/systemv1connect"
	userv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1"
)

// ratedProvider rates movies by TMDB ID: 1 is PG, 2 is R.
type ratedProvider struct{}

func (ratedProvider) Name() string { return "rated" }

func (ratedProvider) Metadata(_ context.Context, l library.Lookup) (*metadata.Result, error) {
	ratings := map[string]string{"1": "PG", "2": "R"}
	tmdb := l.ExternalIDs[core.ProviderTMDB]
	rating, ok := ratings[tmdb]
	if !ok {
		return nil, nil
	}
	return &metadata.Result{Item: core.Item{Name: "Movie " + tmdb, OfficialRating: rating}}, nil
}

// stubDiscovery lists movies 1 and 2 in a catalog.
type stubDiscovery struct{}

func (stubDiscovery) ListCatalogs(context.Context, *pluginv1.ListCatalogsRequest) (*pluginv1.ListCatalogsResponse, error) {
	return pluginv1.ListCatalogsResponse_builder{Catalogs: []*pluginv1.Catalog{pluginv1.Catalog_builder{
		Id: new("popular"), Name: new("Popular"), Kind: new(pluginv1.CatalogKind_CATALOG_KIND_POPULAR),
		MediaKinds: []pluginv1.MediaKind{pluginv1.MediaKind_MEDIA_KIND_UNSPECIFIED, pluginv1.MediaKind_MEDIA_KIND_MOVIE},
	}.Build()}}.Build(), nil
}

func (stubDiscovery) SearchTitles(context.Context, *pluginv1.SearchTitlesRequest) (*pluginv1.SearchTitlesResponse, error) {
	page, _ := stubDiscovery{}.Discover(context.Background(), nil)
	return pluginv1.SearchTitlesResponse_builder{
		Titles: page.GetTitles(), Page: new(int32(1)), TotalPages: new(int32(1)), TotalResults: new(int32(2)),
	}.Build(), nil
}

func (stubDiscovery) Discover(context.Context, *pluginv1.DiscoverRequest) (*pluginv1.DiscoverResponse, error) {
	var titles []*pluginv1.DiscoveredTitle
	for _, id := range []int{1, 2} {
		titles = append(titles, pluginv1.DiscoveredTitle_builder{
			MediaKind: new(pluginv1.MediaKind_MEDIA_KIND_MOVIE), Name: new("Movie " + strconv.Itoa(id)),
			ExternalIds: map[string]string{"tmdb": strconv.Itoa(id)},
		}.Build())
	}
	return pluginv1.DiscoverResponse_builder{Titles: titles, Page: new(int32(1)), TotalPages: new(int32(1)), TotalResults: new(int32(2))}.Build(), nil
}

func (stubDiscovery) ListFilterValues(context.Context, *pluginv1.ListFilterValuesRequest) (*pluginv1.ListFilterValuesResponse, error) {
	return &pluginv1.ListFilterValuesResponse{}, nil
}

// userHeader names the stored user a test request is made as.
const userHeader = "Test-User"

func TestRequestServices(t *testing.T) {
	ctx := t.Context()
	db := openStore(t)
	users := map[string]core.User{}
	for _, u := range []core.User{
		{Name: "admin", Kind: core.UserAdmin},
		{Name: "pg", Access: core.LibraryAccess{ContentLimits: core.ContentLimits{
			MaxParentalRating: new(func() int { s, _ := metadata.RatingScore("PG", "US"); return s }()),
		}}, Policy: core.UserPolicy{
			Requests:   core.RequestPermissions{Movies: true, ReportIssues: true},
			MovieQuota: &core.Quota{Limit: 1},
		}},
		{Name: "none"},
	} {
		u.ID, u.AuthProvider = core.NewID(), "test"
		if u.Kind == "" {
			u.Kind = core.UserMember
		}
		if err := db.Users().Create(ctx, &u); err != nil {
			t.Fatal(err)
		}
		users[u.Name] = u
	}
	log := activity.New(activity.Config{Store: db})
	svc := &requests.Service{
		Store: db, Metadata: func() []library.Provider { return []library.Provider{ratedProvider{}} },
		Discovery: func() []requests.DiscoveryProvider {
			return []requests.DiscoveryProvider{{ID: "stub", Client: stubDiscovery{}}}
		},
		Activity: log,
	}
	validate, err := NewValidateInterceptor()
	if err != nil {
		t.Fatal(err)
	}
	authn := connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			u, ok := users[req.Header().Get(userHeader)]
			if !ok {
				return nil, connect.NewError(connect.CodeUnauthenticated, nil)
			}
			return next(auth.WithPrincipal(ctx, auth.Principal{User: u}), req)
		}
	})
	mux := http.NewServeMux()
	opts := connect.WithInterceptors(authn, validate)
	mux.Handle(acquisitionv1connect.NewRequestServiceHandler(NewRequestService(svc), opts))
	mux.Handle(acquisitionv1connect.NewDiscoverServiceHandler(NewDiscoverService(svc), opts))
	mux.Handle(acquisitionv1connect.NewIssueServiceHandler(NewIssueService(svc), opts))
	mux.Handle(systemv1connect.NewActivityServiceHandler(NewActivityService(db), opts))
	url := testHandlers(t, func(connect.HandlerOption) (string, http.Handler) { return "/", mux })
	asUser := func(name string) connect.ClientOption {
		return connect.WithInterceptors(connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
			return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
				req.Header().Set(userHeader, name)
				return next(ctx, req)
			}
		}))
	}
	reqs := func(name string) acquisitionv1connect.RequestServiceClient {
		return acquisitionv1connect.NewRequestServiceClient(http.DefaultClient, url, asUser(name))
	}
	movie := func(tmdb int32) *acquisitionv1.CreateRequestRequest {
		return acquisitionv1.CreateRequestRequest_builder{MediaKind: new(acquisitionv1.MediaKind_MEDIA_KIND_MOVIE), TmdbId: new(tmdb)}.Build()
	}

	// Discovery hides the movie rated above the user's limit.
	page, err := acquisitionv1connect.NewDiscoverServiceClient(http.DefaultClient, url, asUser("pg")).
		Discover(ctx, acquisitionv1.DiscoverRequest_builder{Catalog: new("stub/popular")}.Build())
	if err != nil {
		t.Fatal(err)
	}
	if got := page.GetTitles(); len(got) != 1 || got[0].GetName() != "Movie 1" || !got[0].GetRequestable() ||
		got[0].GetStatus() != acquisitionv1.MediaStatus_MEDIA_STATUS_UNSPECIFIED {
		t.Errorf("Discover() = %v, want Movie 1 requestable", got)
	}
	if page.GetHiddenResults() != 1 || page.GetTotalResults() != 1 {
		t.Errorf("Discover() hidden = %d of %d, want 1 of 1", page.GetHiddenResults(), page.GetTotalResults())
	}
	discover := acquisitionv1connect.NewDiscoverServiceClient(http.DefaultClient, url, asUser("pg"))
	found, err := discover.SearchTitles(ctx, acquisitionv1.SearchTitlesRequest_builder{Query: new("movie")}.Build())
	if err != nil || len(found.GetTitles()) != 1 || found.GetHiddenResults() != 1 || found.GetPluginId() != "stub" {
		t.Errorf("SearchTitles() = %v, %v", found, err)
	}
	_, err = discover.SearchTitles(ctx, &acquisitionv1.SearchTitlesRequest{})
	wantCode(t, "SearchTitles without a query", err, connect.CodeInvalidArgument)
	_, err = discover.GetTitle(ctx, acquisitionv1.GetTitleRequest_builder{MediaKind: new(acquisitionv1.MediaKind_MEDIA_KIND_MOVIE), TmdbId: new(int32(2))}.Build())
	wantCode(t, "GetTitle above the rating limit", err, connect.CodePermissionDenied)
	_, err = discover.GetTitle(ctx, acquisitionv1.GetTitleRequest_builder{MediaKind: new(acquisitionv1.MediaKind_MEDIA_KIND_MOVIE), TmdbId: new(int32(404))}.Build())
	wantCode(t, "GetTitle of an unknown title", err, connect.CodeNotFound)
	_, err = discover.GetSeason(ctx, acquisitionv1.GetSeasonRequest_builder{TmdbId: new(int32(1)), SeasonNumber: new(int32(1))}.Build())
	wantCode(t, "GetSeason without an episode lister", err, connect.CodeNotFound)
	title, err := discover.GetTitle(ctx, acquisitionv1.GetTitleRequest_builder{MediaKind: new(acquisitionv1.MediaKind_MEDIA_KIND_MOVIE), TmdbId: new(int32(1))}.Build())
	if err != nil || title.GetTitle().GetName() != "Movie 1" || title.GetTitle().GetOfficialRating() != "PG" || !title.GetTitle().GetRequestable() ||
		title.GetTitle().GetExternalIds()["tmdb"] != "1" {
		t.Errorf("GetTitle() = %v, %v", title, err)
	}
	cats, err := acquisitionv1connect.NewDiscoverServiceClient(http.DefaultClient, url, asUser("none")).
		ListCatalogs(ctx, &acquisitionv1.ListCatalogsRequest{})
	if err != nil || len(cats.GetCatalogs()) != 1 || cats.GetCatalogs()[0].GetId() != "stub/popular" || !cats.GetCatalogs()[0].GetMixed() {
		t.Errorf("ListCatalogs() = %v, %v", cats, err)
	}

	_, err = reqs("none").CreateRequest(ctx, movie(1))
	wantCode(t, "CreateRequest without permission", err, connect.CodePermissionDenied)
	_, err = reqs("pg").CreateRequest(ctx, movie(2))
	wantCode(t, "CreateRequest above the rating limit", err, connect.CodePermissionDenied)
	_, err = reqs("pg").CreateRequest(ctx, acquisitionv1.CreateRequestRequest_builder{TmdbId: new(int32(1))}.Build())
	wantCode(t, "CreateRequest without a kind", err, connect.CodeInvalidArgument)
	created, err := reqs("pg").CreateRequest(ctx, movie(1))
	if err != nil {
		t.Fatal(err)
	}
	r := created.GetRequest()
	if r.GetStatus() != acquisitionv1.RequestStatus_REQUEST_STATUS_PENDING || r.GetTitle() != "Movie 1" ||
		r.GetRequestedBy() != users["pg"].ID.String() || r.GetMediaKind() != acquisitionv1.MediaKind_MEDIA_KIND_MOVIE {
		t.Errorf("CreateRequest() = %v", r)
	}
	_, err = reqs("admin").CreateRequest(ctx, movie(1))
	wantCode(t, "CreateRequest of a requested movie", err, connect.CodeAlreadyExists)

	// The blocklist.
	block := acquisitionv1.BlockTitleRequest_builder{MediaKind: new(acquisitionv1.MediaKind_MEDIA_KIND_MOVIE), TmdbId: new(int32(2))}.Build()
	_, err = reqs("pg").BlockTitle(ctx, block)
	wantCode(t, "BlockTitle by a user", err, connect.CodePermissionDenied)
	blocked, err := reqs("admin").BlockTitle(ctx, block)
	if err != nil || blocked.GetBlockedTitle().GetTitle() != "Movie 2" || blocked.GetBlockedTitle().GetUserId() != users["admin"].ID.String() {
		t.Errorf("BlockTitle() = %v, %v", blocked, err)
	}
	_, err = reqs("admin").BlockTitle(ctx, block)
	wantCode(t, "BlockTitle twice", err, connect.CodeAlreadyExists)
	_, err = reqs("admin").CreateRequest(ctx, movie(2))
	wantCode(t, "CreateRequest of a blocked movie", err, connect.CodePermissionDenied)
	if l, err := reqs("admin").ListBlockedTitles(ctx, &acquisitionv1.ListBlockedTitlesRequest{}); err != nil || l.GetTotal() != 1 {
		t.Errorf("ListBlockedTitles() = %v, %v", l, err)
	}
	_, err = reqs("pg").ListBlockedTitles(ctx, &acquisitionv1.ListBlockedTitlesRequest{})
	wantCode(t, "ListBlockedTitles by a user", err, connect.CodePermissionDenied)
	if _, err := reqs("admin").UnblockTitle(ctx, acquisitionv1.UnblockTitleRequest_builder{
		MediaKind: new(acquisitionv1.MediaKind_MEDIA_KIND_MOVIE), TmdbId: new(int32(2)),
	}.Build()); err != nil {
		t.Errorf("UnblockTitle() = %v", err)
	}

	quota, err := reqs("pg").GetQuota(ctx, &acquisitionv1.GetQuotaRequest{})
	if err != nil || quota.GetMovie().GetLimit() != 1 || quota.GetMovie().GetUsed() != 1 || !quota.GetMovie().GetRestricted() {
		t.Errorf("GetQuota() = %v, %v", quota, err)
	}
	_, err = reqs("pg").GetQuota(ctx, acquisitionv1.GetQuotaRequest_builder{UserId: new(users["admin"].ID.String())}.Build())
	wantCode(t, "GetQuota of another user", err, connect.CodePermissionDenied)
	if q, err := reqs("admin").GetQuota(ctx, acquisitionv1.GetQuotaRequest_builder{UserId: new(users["pg"].ID.String())}.Build()); err != nil || q.GetMovie().GetUsed() != 1 {
		t.Errorf("GetQuota() of a user by an administrator = %v, %v", q, err)
	}

	list, err := reqs("none").ListRequests(ctx, &acquisitionv1.ListRequestsRequest{})
	if err != nil || len(list.GetRequests()) != 0 {
		t.Errorf("ListRequests() of a user without requests = %v, %v", list, err)
	}
	list, err = reqs("admin").ListRequests(ctx, acquisitionv1.ListRequestsRequest_builder{
		Statuses: []acquisitionv1.RequestStatus{acquisitionv1.RequestStatus_REQUEST_STATUS_PENDING},
	}.Build())
	if err != nil || list.GetTotal() != 1 {
		t.Errorf("ListRequests() of pending requests = %v, %v", list, err)
	}

	byID := func(id string) *acquisitionv1.ApproveRequestRequest {
		return acquisitionv1.ApproveRequestRequest_builder{Id: new(id)}.Build()
	}
	_, err = reqs("pg").ApproveRequest(ctx, byID(r.GetId()))
	wantCode(t, "ApproveRequest by a user", err, connect.CodePermissionDenied)
	_, err = reqs("admin").RetryRequest(ctx, acquisitionv1.RetryRequestRequest_builder{Id: new(r.GetId())}.Build())
	wantCode(t, "RetryRequest of a pending request", err, connect.CodeFailedPrecondition)
	// Without acquisition, approval fails the request.
	approved, err := reqs("admin").ApproveRequest(ctx, byID(r.GetId()))
	if err != nil || approved.GetRequest().GetStatus() != acquisitionv1.RequestStatus_REQUEST_STATUS_FAILED || approved.GetRequest().GetMessage() == "" {
		t.Errorf("ApproveRequest() without acquisition = %v, %v", approved, err)
	}
	_, err = reqs("none").GetRequest(ctx, acquisitionv1.GetRequestRequest_builder{Id: new(r.GetId())}.Build())
	wantCode(t, "GetRequest of another's request", err, connect.CodePermissionDenied)

	// Issues.
	item := core.Item{ID: core.NewID(), LibraryID: core.NewID(), Kind: core.KindMovie, Name: "Film", Path: "/m/film.mkv"}
	lib := core.Library{ID: item.LibraryID, Name: "Movies", Kind: core.LibraryMovies, Paths: []string{t.TempDir()}}
	if err := db.Libraries().Create(ctx, &lib); err != nil {
		t.Fatal(err)
	}
	if err := db.Items().Upsert(ctx, item); err != nil {
		t.Fatal(err)
	}
	issues := func(name string) acquisitionv1connect.IssueServiceClient {
		return acquisitionv1connect.NewIssueServiceClient(http.DefaultClient, url, asUser(name))
	}
	newIssue := acquisitionv1.CreateIssueRequest_builder{
		ItemId: new(item.ID.String()), Type: new(acquisitionv1.IssueType_ISSUE_TYPE_SUBTITLES), Message: new("Out of sync"),
	}.Build()
	_, err = issues("none").CreateIssue(ctx, newIssue)
	wantCode(t, "CreateIssue without permission", err, connect.CodePermissionDenied)
	issue, err := issues("pg").CreateIssue(ctx, newIssue)
	if err != nil {
		t.Fatal(err)
	}
	id := issue.GetIssue().GetId()
	if c := issue.GetIssue().GetComments(); len(c) != 1 || c[0].GetMessage() != "Out of sync" ||
		issue.GetIssue().GetType() != acquisitionv1.IssueType_ISSUE_TYPE_SUBTITLES {
		t.Errorf("CreateIssue() = %v", issue)
	}
	if _, err := issues("admin").AddComment(ctx, acquisitionv1.AddCommentRequest_builder{Id: new(id), Message: new("Fixed")}.Build()); err != nil {
		t.Fatal(err)
	}
	resolved, err := issues("admin").ResolveIssue(ctx, acquisitionv1.ResolveIssueRequest_builder{Id: new(id)}.Build())
	if err != nil || resolved.GetIssue().GetStatus() != acquisitionv1.IssueStatus_ISSUE_STATUS_RESOLVED {
		t.Errorf("ResolveIssue() = %v, %v", resolved, err)
	}
	if l, err := issues("pg").ListIssues(ctx, &acquisitionv1.ListIssuesRequest{}); err != nil || l.GetTotal() != 1 {
		t.Errorf("ListIssues() = %v, %v", l, err)
	}

	// The user reads their notifications.
	activities := systemv1connect.NewActivityServiceClient(http.DefaultClient, url, asUser("pg"))
	mine, err := activities.ListMyActivities(ctx, &systemv1.ListMyActivitiesRequest{})
	if err != nil {
		t.Fatal(err)
	}
	var types []string
	for _, a := range mine.GetActivities() {
		if a.GetUserId() != users["pg"].ID.String() {
			t.Errorf("ListMyActivities() has another user's activity %v", a)
		}
		types = append(types, a.GetType())
	}
	for _, want := range []string{requests.TypeFailed, requests.TypeIssueCommented, requests.TypeIssueResolved} {
		if !slices.Contains(types, want) {
			t.Errorf("ListMyActivities() types = %v, want %s", types, want)
		}
	}
	only, err := activities.ListMyActivities(ctx, systemv1.ListMyActivitiesRequest_builder{Types: []string{requests.TypeFailed}}.Build())
	if err != nil || len(only.GetActivities()) != 1 {
		t.Errorf("ListMyActivities() of a type = %v, %v", only, err)
	}
}

func TestRequestSettingsRoundTrip(t *testing.T) {
	in := core.DefaultRequestSettings()
	in.DefaultPermissions.AutoApproveSeries = true
	in.MovieQuota, in.SeriesQuota = core.Quota{Limit: 3, Days: 7}, core.Quota{Limit: 10}
	in.MovieLibraryID, in.SeriesQualityProfileID = core.NewID(), core.NewID()
	in.MinimumAvailability, in.MonitorNewItems, in.Specials = core.AvailabilityInCinemas, core.NewItemsAll, true
	in.AnimeLibraryID, in.AnimeQualityProfileID = core.NewID(), core.NewID()
	in.BlocklistedTags, in.BlocklistedTagsLimit = []string{"tmdb/9882"}, 100
	out, err := requestSettingsFromProto(requestSettingsToProto(&in))
	if err != nil || !reflect.DeepEqual(out, in) {
		t.Errorf("round trip = %+v, %v, want %+v", out, err, in)
	}
	bad := requestSettingsToProto(&in)
	bad.SetMovieLibraryId("nope")
	if _, err := requestSettingsFromProto(bad); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("bad library ID: err = %v, want InvalidArgument", err)
	}
	// Settings without requests keep the current ones.
	current := core.DefaultServerSettings()
	current.Requests = in
	p := settingsToProto(&current)
	p.ClearRequests()
	got, err := settingsFromProto(p, &current)
	if err != nil || !reflect.DeepEqual(got.Requests, in) {
		t.Errorf("settings without requests = %+v, %v", got.Requests, err)
	}
	// Metadata settings round trip; settings without them keep the
	// current ones.
	current.Metadata = core.MetadataSettings{PreferredLanguage: "de", Country: "DE"}
	if got, err := settingsFromProto(settingsToProto(&current), &core.ServerSettings{}); err != nil || got.Metadata != current.Metadata {
		t.Errorf("metadata settings = %+v, %v, want %+v", got.Metadata, err, current.Metadata)
	}
	p = settingsToProto(&current)
	p.ClearMetadata()
	if got, err := settingsFromProto(p, &current); err != nil || got.Metadata != current.Metadata {
		t.Errorf("settings without metadata = %+v, %v", got.Metadata, err)
	}
}

func TestPolicyRequestPermissions(t *testing.T) {
	base := core.RequestPermissions{Movies: true, Series: true}
	p := userv1.UserPolicy_builder{
		RequestSeries: new(false), ReportIssues: new(true),
		MovieQuota: userv1.RequestQuota_builder{Limit: new(int32(2)), Days: new(int32(7))}.Build(),
	}.Build()
	got, _ := policyFromProto(p, base)
	if want := (core.RequestPermissions{Movies: true, ReportIssues: true}); got.Requests != want {
		t.Errorf("Requests = %+v, want %+v", got.Requests, want)
	}
	if got.MovieQuota == nil || *got.MovieQuota != (core.Quota{Limit: 2, Days: 7}) || got.SeriesQuota != nil {
		t.Errorf("quotas = %v, %v", got.MovieQuota, got.SeriesQuota)
	}
	u := core.User{ID: core.NewID(), Name: "u", Policy: got}
	back := userToProto(&u, nil).GetPolicy()
	if !back.GetRequestMovies() || back.GetRequestSeries() || back.GetMovieQuota().GetLimit() != 2 || back.HasSeriesQuota() {
		t.Errorf("userToProto() policy = %v", back)
	}
}
