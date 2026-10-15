package server_test

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"

	acquisitionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1/acquisitionv1connect"
	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
	systemv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1/systemv1connect"
	userv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1"
	"github.com/mavioai/mavio/libs/release"
)

// requestFlowRoutes are the answers of the services for the request
// flows test, besides those of the requests test: TMDB lists The Matrix
// Revolutions as trending too, unrated.
func requestFlowRoutes(t *testing.T) map[string]string {
	routes := requestRoutes(t)
	routes["api.themoviedb.org/3/movie/605"] = `{"id": 605, "title": "The Matrix Revolutions", "overview": "TMDB: the war ends.",
		"release_date": "2003-11-05", "external_ids": {"imdb_id": "tt0242653"}, "release_dates": {"results": []}}`
	routes["api.radarr.video/v1/movie/605"] = `{"TmdbId": 605, "ImdbId": "tt0242653", "Title": "The Matrix Revolutions", "Year": 2003,
		"Overview": "Radarr: the war ends.", "Runtime": 129}`
	routes["api.themoviedb.org/3/trending/movie/day"] = `{"page": 1, "total_pages": 1, "total_results": 3, "results": [
		{"id": 603, "title": "The Matrix", "release_date": "1999-03-30", "popularity": 90},
		{"id": 604, "title": "The Matrix Reloaded", "release_date": "2003-05-15", "popularity": 80},
		{"id": 605, "title": "The Matrix Revolutions", "release_date": "2003-11-05", "popularity": 70}]}`
	return routes
}

// TestRequestFlows runs the assembled server with the metadata plugins
// against fakes of their services (P20's done criteria for the decisions
// on requests, quotas, seasons and unrated titles), as Seerr's request
// routes and User.getQuota: a request beyond the user's quota is refused;
// a declined one frees the quota it used; a request whose acquisition
// fails, for want of a library, fails with why, and is retried once the
// administrator sets one; a pending series request changes its seasons,
// paying for those added; a request of every season asks for those but
// specials; a user whose policy blocks unrated titles does not find them,
// while one whose policy does not does.
func TestRequestFlows(t *testing.T) {
	t.Parallel()
	s := startAcquisition(t, requestFlowRoutes(t), false)
	ctx := t.Context()
	movies, shows := t.TempDir(), t.TempDir()
	movieLib := s.library("Movies", libraryv1.LibraryKind_LIBRARY_KIND_MOVIES, movies)
	showLib := s.library("Shows", libraryv1.LibraryKind_LIBRARY_KIND_SHOWS, shows)
	movieProfile := s.profile(release.Movie, release.WEBDL1080p, release.HDTV720p, release.WEBDL1080p)
	seriesProfile := s.profile(release.Series, release.WEBDL1080p, release.HDTV720p, release.WEBDL1080p)

	// Users may request a movie and two seasons; requests of series go to
	// the shows, while no library nor profile is set for those of movies.
	s.settings(func(settings *systemv1.ServerSettings) {
		settings.SetRequests(systemv1.RequestSettings_builder{
			MovieQuotaLimit: new(int32(1)), SeriesQuotaLimit: new(int32(2)),
			MinimumAvailability: new(acquisitionv1.MinimumAvailability_MINIMUM_AVAILABILITY_RELEASED),
			SeriesLibraryId:     new(showLib), SeriesQualityProfileId: new(seriesProfile), SeasonFolder: new(true),
			MonitorNewItems: new(acquisitionv1.NewItemMonitor_NEW_ITEM_MONITOR_NONE),
		}.Build())
	})
	viewerID, viewerToken := s.user("viewer", userv1.UserPolicy_builder{
		AllLibraries: new(true), RequestMovies: new(true), RequestSeries: new(true), AutoApproveMovies: new(false), AutoApproveSeries: new(false),
	}.Build())
	viewer := acquisitionv1connect.NewRequestServiceClient(http.DefaultClient, s.url, viewerToken)
	viewerActivity := systemv1connect.NewActivityServiceClient(http.DefaultClient, s.url, viewerToken)

	request := func(client acquisitionv1connect.RequestServiceClient, b acquisitionv1.CreateRequestRequest_builder) (*acquisitionv1.Request, error) {
		resp, err := client.CreateRequest(ctx, b.Build())
		return resp.GetRequest(), err
	}
	movie := func(tmdbID int32) acquisitionv1.CreateRequestRequest_builder {
		return acquisitionv1.CreateRequestRequest_builder{MediaKind: new(acquisitionv1.MediaKind_MEDIA_KIND_MOVIE), TmdbId: new(tmdbID)}
	}
	// breakingBad requests seasons of Breaking Bad, or all of them.
	breakingBad := func(all bool, seasons ...int32) acquisitionv1.CreateRequestRequest_builder {
		return acquisitionv1.CreateRequestRequest_builder{
			MediaKind: new(acquisitionv1.MediaKind_MEDIA_KIND_SERIES), TmdbId: new(int32(1396)), Seasons: seasons, AllSeasons: new(all),
		}
	}
	quota := func() (movies, series string) {
		t.Helper()
		q, err := viewer.GetQuota(ctx, &acquisitionv1.GetQuotaRequest{})
		if err != nil {
			t.Fatalf("GetQuota: %v", err)
		}
		m, sr := q.GetMovie(), q.GetSeries()
		return fmt.Sprintf("%d/%d restricted=%v", m.GetUsed(), m.GetLimit(), m.GetRestricted()),
			fmt.Sprintf("%d/%d restricted=%v", sr.GetUsed(), sr.GetLimit(), sr.GetRestricted())
	}
	// told waits for the viewer to be told of a request.
	told := func(typ string, r *acquisitionv1.Request) {
		t.Helper()
		var seen []string
		waitFor(t, fmt.Sprintf("the viewer told of %s of %s", typ, r.GetTitle()), func() bool {
			resp, err := viewerActivity.ListMyActivities(ctx, systemv1.ListMyActivitiesRequest_builder{Types: []string{typ}}.Build())
			if err != nil {
				t.Fatalf("ListMyActivities: %v", err)
			}
			seen = nil
			for _, a := range resp.GetActivities() {
				seen = append(seen, a.GetAttributes()["request_id"])
				if a.GetAttributes()["request_id"] == r.GetId() && a.GetUserId() == viewerID {
					return true
				}
			}
			return false
		}, func() any { return seen })
	}

	// 1. A movie uses the quota: another is refused, until the first is
	// declined.
	matrix, err := request(viewer, movie(603))
	if err != nil {
		t.Fatalf("CreateRequest of The Matrix: %v", err)
	}
	if _, err := request(viewer, movie(604)); connect.CodeOf(err) != connect.CodeResourceExhausted {
		t.Errorf("CreateRequest past the quota: %v, want = resource exhausted", err)
	}
	if m, _ := quota(); m != "1/1 restricted=true" {
		t.Errorf("movie quota = %s, want = 1/1 restricted", m)
	}
	declined, err := s.requests.DeclineRequest(ctx, acquisitionv1.DeclineRequestRequest_builder{Id: new(matrix.GetId())}.Build())
	if err != nil {
		t.Fatalf("DeclineRequest: %v", err)
	}
	if r := declined.GetRequest(); r.GetStatus() != acquisitionv1.RequestStatus_REQUEST_STATUS_DECLINED || r.GetModifiedBy() != s.adminID || r.GetItemId() != "" {
		t.Errorf("declined request = %v, want = declined by the administrator, nothing wanted", r)
	}
	told("request.declined", matrix)
	if m, _ := quota(); m != "0/1 restricted=false" {
		t.Errorf("movie quota after the decline = %s, want = 0/1", m)
	}
	if _, err := s.requests.ApproveRequest(ctx, acquisitionv1.ApproveRequestRequest_builder{Id: new(matrix.GetId())}.Build()); err == nil {
		t.Errorf("ApproveRequest of a declined request succeeded")
	}

	// 2. Approved without a movie library nor profile, a request fails
	// with why; retried once they are set, it is approved.
	reloaded, err := request(viewer, movie(604))
	if err != nil {
		t.Fatalf("CreateRequest of The Matrix Reloaded: %v", err)
	}
	failed, err := s.requests.ApproveRequest(ctx, acquisitionv1.ApproveRequestRequest_builder{Id: new(reloaded.GetId())}.Build())
	if err != nil {
		t.Fatalf("ApproveRequest: %v", err)
	}
	if r := failed.GetRequest(); r.GetStatus() != acquisitionv1.RequestStatus_REQUEST_STATUS_FAILED || !strings.Contains(r.GetMessage(), "no quality profile") || r.GetItemId() != "" {
		t.Errorf("request approved without a profile = %v, want = failed for want of a profile", r)
	}
	if m, _ := quota(); m != "1/1 restricted=true" {
		t.Errorf("movie quota with a failed request = %s, want = 1/1 restricted", m)
	}
	s.settings(func(settings *systemv1.ServerSettings) {
		settings.GetRequests().SetMovieLibraryId(movieLib)
		settings.GetRequests().SetMovieQualityProfileId(movieProfile)
	})
	retried, err := s.requests.RetryRequest(ctx, acquisitionv1.RetryRequestRequest_builder{Id: new(reloaded.GetId())}.Build())
	if err != nil {
		t.Fatalf("RetryRequest: %v", err)
	}
	if r := retried.GetRequest(); r.GetStatus() != acquisitionv1.RequestStatus_REQUEST_STATUS_APPROVED || r.GetItemId() == "" || r.GetMessage() != "" {
		t.Fatalf("retried request = %v, want = approved, with its item", r)
	}
	if it := s.item(retried.GetRequest().GetItemId()); it.GetName() != "The Matrix Reloaded" || it.GetLibraryId() != movieLib || !it.GetMonitored() {
		t.Errorf("requested movie = %q in %s monitored=%v, want = The Matrix Reloaded, wanted in Movies", it.GetName(), it.GetLibraryId(), it.GetMonitored())
	}
	told("request.approved", reloaded)
	if _, err := s.requests.RetryRequest(ctx, acquisitionv1.RetryRequestRequest_builder{Id: new(reloaded.GetId())}.Build()); err == nil {
		t.Errorf("RetryRequest of an approved request succeeded")
	}

	// 3. A pending series request of the first season adds the second,
	// paying for it; deleted, it frees the quota, and a request of every
	// season asks for both.
	first, err := request(viewer, breakingBad(false, 1))
	if err != nil {
		t.Fatalf("CreateRequest of Breaking Bad's first season: %v", err)
	}
	seasons := func(r *acquisitionv1.Request) []int32 {
		var out []int32
		for _, sr := range r.GetSeasons() {
			out = append(out, sr.GetSeasonNumber())
		}
		return out
	}
	if _, sq := quota(); sq != "1/2 restricted=false" || !slices.Equal(seasons(first), []int32{1}) {
		t.Errorf("request of season 1 = %v with series quota %s, want = 1/2", seasons(first), sq)
	}
	updated, err := viewer.UpdateRequest(ctx, acquisitionv1.UpdateRequestRequest_builder{Id: new(first.GetId()), Seasons: []int32{1, 2}}.Build())
	if err != nil {
		t.Fatalf("UpdateRequest: %v", err)
	}
	if r := updated.GetRequest(); !slices.Equal(seasons(r), []int32{1, 2}) || r.GetStatus() != acquisitionv1.RequestStatus_REQUEST_STATUS_PENDING {
		t.Errorf("updated request = %v, want = seasons 1 and 2, pending", r)
	}
	if _, sq := quota(); sq != "2/2 restricted=true" {
		t.Errorf("series quota after adding a season = %s, want = 2/2 restricted", sq)
	}
	if _, err := viewer.DeleteRequest(ctx, acquisitionv1.DeleteRequestRequest_builder{Id: new(first.GetId())}.Build()); err != nil {
		t.Fatalf("DeleteRequest: %v", err)
	}
	if _, sq := quota(); sq != "0/2 restricted=false" {
		t.Errorf("series quota after the delete = %s, want = 0/2", sq)
	}
	all, err := request(viewer, breakingBad(true))
	if err != nil {
		t.Fatalf("CreateRequest of every season: %v", err)
	}
	if !slices.Equal(seasons(all), []int32{1, 2}) {
		t.Errorf("request of every season = %v, want = seasons 1 and 2", seasons(all))
	}
	approved, err := s.requests.ApproveRequest(ctx, acquisitionv1.ApproveRequestRequest_builder{Id: new(all.GetId())}.Build())
	if err != nil {
		t.Fatalf("ApproveRequest: %v", err)
	}
	seriesID := approved.GetRequest().GetItemId()
	if approved.GetRequest().GetStatus() != acquisitionv1.RequestStatus_REQUEST_STATUS_APPROVED || seriesID == "" {
		t.Fatalf("approved request = %v, want = approved, with its series", approved.GetRequest())
	}
	describeSeasons := func() any {
		var out []string
		for _, it := range s.children(seriesID, libraryv1.ItemKind_ITEM_KIND_SEASON) {
			out = append(out, fmt.Sprintf("%d monitored=%v", it.GetIndexNumber(), it.GetMonitored()))
		}
		return out
	}
	waitFor(t, "both seasons of Breaking Bad monitored", func() bool {
		got := describeSeasons().([]string)
		slices.Sort(got)
		return slices.Equal(got, []string{"1 monitored=true", "2 monitored=true"})
	}, describeSeasons)

	// 4. Of two users limited to PG, the one whose policy blocks unrated
	// titles does not find the unrated film, nor may request it.
	_, kidToken := s.user("kid", userv1.UserPolicy_builder{
		AllLibraries: new(true), MaxParentalRating: new(int32(10)), RequestMovies: new(true), RequestSeries: new(true),
	}.Build())
	_, teenToken := s.user("teen", userv1.UserPolicy_builder{
		AllLibraries: new(true), MaxParentalRating: new(int32(10)), BlockUnrated: new(true), RequestMovies: new(true), RequestSeries: new(true),
	}.Build())
	const trending, popular = "dev.mavio.scraper-tmdb/trending", "dev.mavio.scraper-tmdb/popular"
	names := func(discover acquisitionv1connect.DiscoverServiceClient, catalog string, kind acquisitionv1.MediaKind) []string {
		t.Helper()
		resp, err := discover.Discover(ctx, acquisitionv1.DiscoverRequest_builder{Catalog: new(catalog), MediaKind: new(kind)}.Build())
		if err != nil {
			t.Fatalf("Discover %s: %v", catalog, err)
		}
		var out []string
		for _, d := range resp.GetTitles() {
			out = append(out, d.GetName())
		}
		return out
	}
	kid := acquisitionv1connect.NewDiscoverServiceClient(http.DefaultClient, s.url, kidToken)
	teen := acquisitionv1connect.NewDiscoverServiceClient(http.DefaultClient, s.url, teenToken)
	if got, want := names(kid, trending, acquisitionv1.MediaKind_MEDIA_KIND_MOVIE), []string{"The Matrix", "The Matrix Revolutions"}; !slices.Equal(got, want) {
		t.Errorf("trending films for the user allowing unrated titles = %q, want = %q", got, want)
	}
	if got, want := names(teen, trending, acquisitionv1.MediaKind_MEDIA_KIND_MOVIE), []string{"The Matrix"}; !slices.Equal(got, want) {
		t.Errorf("trending films for the user blocking unrated titles = %q, want = %q", got, want)
	}
	// The popular catalog filters by certification: it is asked for those
	// within the limit, which leaves the unrated titles out.
	if got := names(teen, popular, acquisitionv1.MediaKind_MEDIA_KIND_SERIES); !slices.Equal(got, []string{"Breaking Bad"}) {
		t.Errorf("popular series for the user blocking unrated titles = %q, want = Breaking Bad", got)
	}
	s.fake.mu.Lock()
	q := s.fake.queries["api.themoviedb.org/3/discover/tv"]
	s.fake.mu.Unlock()
	if q.Get("certification_country") != "US" || !slices.Contains(strings.Split(q.Get("certification"), "|"), "TV-PG") {
		t.Errorf("TMDB's discover query = %v, want = the US certifications up to TV-PG", q)
	}
	getTitle := func(discover acquisitionv1connect.DiscoverServiceClient, tmdbID int32) (*acquisitionv1.Title, error) {
		resp, err := discover.GetTitle(ctx, acquisitionv1.GetTitleRequest_builder{MediaKind: new(acquisitionv1.MediaKind_MEDIA_KIND_MOVIE), TmdbId: new(tmdbID)}.Build())
		return resp.GetTitle(), err
	}
	if got, err := getTitle(kid, 605); err != nil || got.GetName() != "The Matrix Revolutions" || got.GetOfficialRating() != "" {
		t.Errorf("GetTitle of the unrated film for the user allowing unrated titles = %v, %v; want = it, unrated", got, err)
	}
	if _, err := getTitle(teen, 605); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("GetTitle of the unrated film for the user blocking unrated titles: %v, want = permission denied", err)
	}
	teenRequests := acquisitionv1connect.NewRequestServiceClient(http.DefaultClient, s.url, teenToken)
	if _, err := request(teenRequests, movie(605)); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("CreateRequest of the unrated film by the user blocking unrated titles: %v, want = permission denied", err)
	}
}
