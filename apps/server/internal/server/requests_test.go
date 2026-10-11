package server_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/apps/server/internal/server"
	"github.com/mavioai/mavio/libs/acquire"
	"github.com/mavioai/mavio/libs/core"
	acquisitionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1/acquisitionv1connect"
	authv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1/authv1connect"
	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1/libraryv1connect"
	playbackv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/playback/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/playback/v1/playbackv1connect"
	systemv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1/systemv1connect"
	userv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1/userv1connect"
	"github.com/mavioai/mavio/libs/release"
)

// requestRoutes are the answers of the services for the requests test,
// besides those of the series acquisition test: TMDB rates The Matrix PG
// and Breaking Bad TV-PG, The Matrix Reloaded R; it lists both films as
// trending and Breaking Bad as popular.
func requestRoutes(t *testing.T) map[string]string {
	routes := seriesRoutes(t)
	for path, body := range map[string]string{
		"api.themoviedb.org/3/movie/603": `{"id": 603, "title": "The Matrix", "overview": "TMDB: a hacker learns the truth.",
			"release_date": "1999-03-30", "external_ids": {"imdb_id": "tt0133093"},
			"release_dates": {"results": [{"iso_3166_1": "US", "release_dates": [{"certification": "PG"}]}]}}`,
		"api.themoviedb.org/3/movie/604": `{"id": 604, "title": "The Matrix Reloaded", "overview": "TMDB: the war goes on.",
			"release_date": "2003-05-15", "external_ids": {"imdb_id": "tt0234215"},
			"release_dates": {"results": [{"iso_3166_1": "US", "release_dates": [{"certification": "R"}]}]}}`,
		"api.radarr.video/v1/movie/604": `{"TmdbId": 604, "ImdbId": "tt0234215", "Title": "The Matrix Reloaded", "Year": 2003,
			"Overview": "Radarr: the war goes on.", "Runtime": 138}`,
		"api.themoviedb.org/3/tv/1396": `{"id": 1396, "name": "Breaking Bad", "overview": "TMDB: a teacher turns to crime.",
			"first_air_date": "2008-01-20", "external_ids": {"imdb_id": "tt0903747", "tvdb_id": 81189},
			"content_ratings": {"results": [{"iso_3166_1": "US", "rating": "TV-PG"}]}}`,
		"api.themoviedb.org/3/trending/movie/day": `{"page": 1, "total_pages": 1, "total_results": 2, "results": [
			{"id": 603, "title": "The Matrix", "release_date": "1999-03-30", "popularity": 90},
			{"id": 604, "title": "The Matrix Reloaded", "release_date": "2003-05-15", "popularity": 80}]}`,
		"api.themoviedb.org/3/discover/tv": `{"page": 1, "total_pages": 1, "total_results": 1, "results": [
			{"id": 1396, "name": "Breaking Bad", "first_air_date": "2008-01-20", "popularity": 70}]}`,
	} {
		routes[path] = body
	}
	return routes
}

// runTask runs a task and waits for its run to finish, unless a run of
// it was queued or running already.
func runTask(t *testing.T, tasks systemv1connect.TaskServiceClient, id string) {
	t.Helper()
	ctx := t.Context()
	resp, err := tasks.RunTask(ctx, systemv1.RunTaskRequest_builder{Id: new(id)}.Build())
	if err != nil {
		t.Fatalf("RunTask %s: %v", id, err)
	}
	if !resp.GetEnqueued() {
		return
	}
	var state systemv1.JobState
	waitFor(t, "the run of "+id, func() bool {
		jobs, err := tasks.ListJobs(ctx, &systemv1.ListJobsRequest{})
		if err != nil {
			t.Fatalf("ListJobs: %v", err)
		}
		for _, j := range jobs.GetJobs() {
			if j.GetId() == resp.GetJobId() {
				state = j.GetState()
			}
		}
		return state == systemv1.JobState_JOB_STATE_SUCCEEDED || state == systemv1.JobState_JOB_STATE_FAILED
	}, func() any { return state })
	if state != systemv1.JobState_JOB_STATE_SUCCEEDED {
		t.Fatalf("run of %s: %v, want = succeeded", id, state)
	}
}

// TestRequests runs the assembled server with the metadata plugins
// against fakes of their services, and the indexers and the download
// clients plugins against a Newznab indexer and SABnzbd faked on the
// loopback interface (P20's done criteria): a user limited to PG finds a
// film and a series through discovery, cannot request a film rated above
// the limit, requests the others; an administrator approves them; they
// are acquired, and the user is told they are available, and plays the
// film; the user reports an issue with it, which the administrator
// resolves.
func TestRequests(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("integration test: ffmpeg not found on PATH")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("integration test: ffprobe not found on PATH")
	}
	ctx := t.Context()

	// What the downloads hold: longer than ten minutes, no sample of The
	// Matrix nor of an episode, with audio.
	film := filepath.Join(t.TempDir(), "film.mkv")
	run(t, ffmpeg, "-loglevel", "error", "-f", "lavfi", "-i", "testsrc2=size=160x120:rate=1",
		"-f", "lavfi", "-i", "sine=frequency=440:sample_rate=8000", "-t", "610",
		"-c:v", "libx264", "-preset", "ultrafast", "-c:a", "aac", "-b:a", "16k", film)
	settled(t, film)
	filmBytes, err := os.ReadFile(film)
	if err != nil {
		t.Fatal(err)
	}

	movies, shows := t.TempDir(), t.TempDir()
	indexer := newFakeNewznab(t)
	sab := newFakeSABnzbd(t, t.TempDir())
	fake := &fakeServices{mode: tmdbAcceptsRadarr, routes: requestRoutes(t)}
	t.Cleanup(func() {
		fake.mu.Lock()
		defer fake.mu.Unlock()
		if len(fake.unknown) > 0 {
			t.Errorf("requests answered 404: %v", fake.unknown)
		}
	})
	pluginDir := installScrapers(t)
	installProcessPlugin(t, pluginDir, "indexers", indexersSource, indexersBuild)
	installProcessPlugin(t, pluginDir, "download-clients", downloadClientsSource, downloadClientsBuild)
	srv := start(t, server.Config{
		Database: "sqlite:" + filepath.Join(t.TempDir(), "mavio.db"), FFmpeg: ffmpeg, FFprobe: ffprobe,
		TranscodeDir: t.TempDir(), CacheDir: t.TempDir(), MetadataDir: t.TempDir(),
		PluginDir: pluginDir, PluginDataDir: t.TempDir(), PluginFetch: &http.Client{Transport: fake},
	})
	auth := authv1connect.NewAuthServiceClient(http.DefaultClient, srv.url)
	first, err := auth.CreateFirstUser(ctx, authv1.CreateFirstUserRequest_builder{
		Name: new("admin"), Password: new("secret"), Device: device("e2e"),
	}.Build())
	if err != nil {
		t.Fatalf("CreateFirstUser: %v", err)
	}
	token := withToken(first.GetAccessToken())
	system := systemv1connect.NewSystemServiceClient(http.DefaultClient, srv.url, token)
	tasks := systemv1connect.NewTaskServiceClient(http.DefaultClient, srv.url, token)
	activity := systemv1connect.NewActivityServiceClient(http.DefaultClient, srv.url, token)
	users := userv1connect.NewUserServiceClient(http.DefaultClient, srv.url, token)
	libraries := libraryv1connect.NewLibraryServiceClient(http.DefaultClient, srv.url, token)
	items := libraryv1connect.NewItemServiceClient(http.DefaultClient, srv.url, token)
	playback := playbackv1connect.NewPlaybackServiceClient(http.DefaultClient, srv.url, token)
	profiles := acquisitionv1connect.NewProfileServiceClient(http.DefaultClient, srv.url, token)
	wanted := acquisitionv1connect.NewWantedServiceClient(http.DefaultClient, srv.url, token)
	downloads := acquisitionv1connect.NewDownloadServiceClient(http.DefaultClient, srv.url, token)
	releases := acquisitionv1connect.NewReleaseServiceClient(http.DefaultClient, srv.url, token)
	adminRequests := acquisitionv1connect.NewRequestServiceClient(http.DefaultClient, srv.url, token)
	adminIssues := acquisitionv1connect.NewIssueServiceClient(http.DefaultClient, srv.url, token)

	// 1. The administrator sets acquisition up: libraries of movies and
	// shows, a quality profile of each kind, the indexer and SABnzbd; RSS
	// syncs only when asked.
	createLibrary := func(name string, kind libraryv1.LibraryKind, path string) string {
		t.Helper()
		lib, err := libraries.CreateLibrary(ctx, libraryv1.CreateLibraryRequest_builder{
			Spec: libraryv1.LibrarySpec_builder{Name: new(name), Kind: new(kind), Paths: []string{path}}.Build(),
		}.Build())
		if err != nil {
			t.Fatalf("CreateLibrary %s: %v", name, err)
		}
		return lib.GetLibrary().GetId()
	}
	movieLib := createLibrary("Movies", libraryv1.LibraryKind_LIBRARY_KIND_MOVIES, movies)
	showLib := createLibrary("Shows", libraryv1.LibraryKind_LIBRARY_KIND_SHOWS, shows)
	createProfile := func(kind release.Kind, coreKind core.ItemKind, protoKind acquisitionv1.MediaKind, cutoff release.Quality, qualities []release.Quality) string {
		t.Helper()
		stored, err := acquire.QualityProfileToCore(acquire.NewProfile(kind, "HD", cutoff, qualities, nil), coreKind)
		if err != nil {
			t.Fatal(err)
		}
		profile, err := profiles.CreateQualityProfile(ctx, acquisitionv1.CreateQualityProfileRequest_builder{
			Profile: acquisitionv1.QualityProfile_builder{
				Kind: new(protoKind), Name: new("HD"), UpgradeAllowed: new(true),
				Cutoff: new(int32(stored.Cutoff)), Items: profileItemsToProto(stored.Items),
			}.Build(),
		}.Build())
		if err != nil {
			t.Fatalf("CreateQualityProfile: %v", err)
		}
		return profile.GetProfile().GetId()
	}
	movieProfile := createProfile(release.Movie, core.KindMovie, acquisitionv1.MediaKind_MEDIA_KIND_MOVIE,
		release.WEBDL1080p, []release.Quality{release.HDTV720p, release.WEBDL1080p})
	seriesProfile := createProfile(release.Series, core.KindSeries, acquisitionv1.MediaKind_MEDIA_KIND_SERIES,
		release.WEBDL1080p, []release.Quality{release.HDTV720p, release.WEBDL1080p})

	// Approved requests go to those libraries with those profiles; users
	// may request five movies and ten seasons.
	got, err := system.GetServerSettings(ctx, &systemv1.GetServerSettingsRequest{})
	if err != nil {
		t.Fatalf("GetServerSettings: %v", err)
	}
	settings := got.GetSettings()
	acq := settings.GetAcquisition()
	acq.SetRecycleBin(t.TempDir())
	acq.SetRssSyncInterval(0)
	acq.SetSkipFreeSpaceCheck(true)
	acq.SetRenameMovies(true)
	acq.SetRenameEpisodes(true)
	settings.SetRequests(systemv1.RequestSettings_builder{
		MovieQuotaLimit: new(int32(5)), SeriesQuotaLimit: new(int32(10)),
		MovieLibraryId: new(movieLib), MovieQualityProfileId: new(movieProfile),
		MinimumAvailability: new(acquisitionv1.MinimumAvailability_MINIMUM_AVAILABILITY_RELEASED),
		SeriesLibraryId:     new(showLib), SeriesQualityProfileId: new(seriesProfile), SeasonFolder: new(true),
		MonitorNewItems: new(acquisitionv1.NewItemMonitor_NEW_ITEM_MONITOR_NONE),
	}.Build())
	if _, err := system.UpdateServerSettings(ctx, systemv1.UpdateServerSettingsRequest_builder{Settings: settings}.Build()); err != nil {
		t.Fatalf("UpdateServerSettings: %v", err)
	}
	for id, cfg := range map[string]any{
		"org.mavio.indexers": map[string]any{"indexers": []any{
			map[string]any{"id": "nzb", "definition": "newznab", "settings": map[string]any{"base_url": indexer.URL, "api_key": "nzbkey"}},
		}},
		"org.mavio.download-clients": map[string]any{"clients": []any{
			map[string]any{"id": "sab", "name": "SABnzbd", "definition": "sabnzbd", "settings": map[string]any{
				"host": "127.0.0.1", "port": sab.Listener.Addr().(*net.TCPAddr).Port, "api_key": sabAPIKey, "movie_category": "movies", "tv_category": "tv",
			}},
		}},
	} {
		data, err := json.Marshal(cfg)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := system.SetPluginConfig(ctx, systemv1.SetPluginConfigRequest_builder{PluginId: new(id), ConfigJson: new(string(data))}.Build()); err != nil {
			t.Fatalf("SetPluginConfig %s: %v", id, err)
		}
	}
	waitFor(t, "the indexer and the download client", func() bool {
		ix, err := releases.ListIndexers(ctx, &acquisitionv1.ListIndexersRequest{})
		if err != nil || len(ix.GetIndexers()) != 1 {
			return false
		}
		dc, err := downloads.ListDownloadClients(ctx, &acquisitionv1.ListDownloadClientsRequest{})
		return err == nil && len(dc.GetClients()) == 1
	})
	served := func(path string) int {
		fake.mu.Lock()
		defer fake.mu.Unlock()
		return fake.served[path]
	}
	waitFor(t, "the scene mappings", func() bool {
		before := served("services.sonarr.tv/v1/dailyseries")
		runTask(t, tasks, "acquisition.scene_mappings")
		return served("services.sonarr.tv/v1/dailyseries") > before && served("services.sonarr.tv/v1/scenemapping") > 0
	})

	// 2. A user limited to PG, who may request movies and series but
	// whose requests wait for approval, and may report issues.
	created, err := users.CreateUser(ctx, userv1.CreateUserRequest_builder{
		Name: new("kid"), Password: new("pw"),
		Policy: userv1.UserPolicy_builder{
			AllLibraries: new(true), MaxParentalRating: new(int32(10)), RequestMovies: new(true), RequestSeries: new(true),
			AutoApproveMovies: new(false), AutoApproveSeries: new(false), ReportIssues: new(true),
		}.Build(),
	}.Build())
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	kidID := created.GetUser().GetId()
	signedIn, err := auth.Login(ctx, authv1.LoginRequest_builder{Name: new("kid"), Password: new("pw"), Device: device("tablet")}.Build())
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	kidToken := withToken(signedIn.GetAccessToken())
	discover := acquisitionv1connect.NewDiscoverServiceClient(http.DefaultClient, srv.url, kidToken)
	kidRequests := acquisitionv1connect.NewRequestServiceClient(http.DefaultClient, srv.url, kidToken)
	kidIssues := acquisitionv1connect.NewIssueServiceClient(http.DefaultClient, srv.url, kidToken)
	kidActivity := systemv1connect.NewActivityServiceClient(http.DefaultClient, srv.url, kidToken)
	kidPlayback := playbackv1connect.NewPlaybackServiceClient(http.DefaultClient, srv.url, kidToken)

	// 3. The user finds the films trending and the series popular.
	catalogs, err := discover.ListCatalogs(ctx, &acquisitionv1.ListCatalogsRequest{})
	if err != nil {
		t.Fatalf("ListCatalogs: %v", err)
	}
	var catalogIDs []string
	for _, c := range catalogs.GetCatalogs() {
		catalogIDs = append(catalogIDs, c.GetId())
	}
	const trending, popular = "org.mavio.scraper-tmdb/trending", "org.mavio.scraper-tmdb/popular"
	if !slices.Contains(catalogIDs, trending) || !slices.Contains(catalogIDs, popular) {
		t.Fatalf("catalogs = %q, want = TMDB's trending and popular titles among them", catalogIDs)
	}
	discoverTitles := func(catalog string, kind acquisitionv1.MediaKind) []*acquisitionv1.DiscoveredTitle {
		t.Helper()
		resp, err := discover.Discover(ctx, acquisitionv1.DiscoverRequest_builder{Catalog: new(catalog), MediaKind: new(kind)}.Build())
		if err != nil {
			t.Fatalf("Discover %s: %v", catalog, err)
		}
		return resp.GetTitles()
	}
	describeTitles := func(titles []*acquisitionv1.DiscoveredTitle) []string {
		var out []string
		for _, d := range titles {
			out = append(out, fmt.Sprintf("%s (%d) tmdb=%s %v requestable=%v request=%q %v item=%q", d.GetName(), d.GetYear(),
				d.GetExternalIds()["tmdb"], d.GetStatus(), d.GetRequestable(), d.GetRequestId(), d.GetRequestStatus(), d.GetItemId()))
		}
		return out
	}
	// The trending catalog does not filter by certification: the film TMDB
	// rates R is left out as the metadata providers rate it.
	films := discoverTitles(trending, acquisitionv1.MediaKind_MEDIA_KIND_MOVIE)
	if len(films) != 1 || films[0].GetName() != "The Matrix" || films[0].GetExternalIds()["tmdb"] != "603" ||
		films[0].GetStatus() != acquisitionv1.MediaStatus_MEDIA_STATUS_UNSPECIFIED || !films[0].GetRequestable() || films[0].GetRequestId() != "" {
		t.Errorf("trending films = %q, want = The Matrix alone, neither available nor requested, requestable", describeTitles(films))
	}
	// The popular catalog, for a user who does not block unrated titles,
	// asks TMDB for no certification, which would leave the unrated
	// titles out: the server rates the titles itself.
	series := discoverTitles(popular, acquisitionv1.MediaKind_MEDIA_KIND_SERIES)
	if len(series) != 1 || series[0].GetName() != "Breaking Bad" || series[0].GetExternalIds()["tmdb"] != "1396" ||
		series[0].GetStatus() != acquisitionv1.MediaStatus_MEDIA_STATUS_UNSPECIFIED || !series[0].GetRequestable() {
		t.Errorf("popular series = %q, want = Breaking Bad, neither available nor requested, requestable", describeTitles(series))
	}
	fake.mu.Lock()
	q := fake.queries["api.themoviedb.org/3/discover/tv"]
	fake.mu.Unlock()
	if q.Has("certification") || q.Has("certification_lte") || q.Get("include_adult") != "false" {
		t.Errorf("TMDB's discover query = %v, want = no certification, no adult titles", q)
	}

	// The film rated R cannot be requested.
	request := func(client acquisitionv1connect.RequestServiceClient, kind acquisitionv1.MediaKind, tmdbID int32, seasons ...int32) (*acquisitionv1.Request, error) {
		resp, err := client.CreateRequest(ctx, acquisitionv1.CreateRequestRequest_builder{
			MediaKind: new(kind), TmdbId: new(tmdbID), Seasons: seasons,
		}.Build())
		return resp.GetRequest(), err
	}
	if _, err := request(kidRequests, acquisitionv1.MediaKind_MEDIA_KIND_MOVIE, 604); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("CreateRequest of The Matrix Reloaded, rated R: %v, want = permission denied", err)
	}

	// 4. The user requests the film and the first season of the series,
	// which wait for approval.
	movieReq, err := request(kidRequests, acquisitionv1.MediaKind_MEDIA_KIND_MOVIE, 603)
	if err != nil {
		t.Fatalf("CreateRequest of The Matrix: %v", err)
	}
	if movieReq.GetStatus() != acquisitionv1.RequestStatus_REQUEST_STATUS_PENDING || movieReq.GetTitle() != "The Matrix" ||
		movieReq.GetYear() != 1999 || movieReq.GetRequestedBy() != kidID || movieReq.GetItemId() != "" {
		t.Errorf("request of The Matrix = %v, want = pending, by the user", movieReq)
	}
	seriesReq, err := request(kidRequests, acquisitionv1.MediaKind_MEDIA_KIND_SERIES, 1396, 1)
	if err != nil {
		t.Fatalf("CreateRequest of Breaking Bad: %v", err)
	}
	if seriesReq.GetStatus() != acquisitionv1.RequestStatus_REQUEST_STATUS_PENDING || seriesReq.GetTitle() != "Breaking Bad" ||
		seriesReq.GetTvdbId() != 81189 || len(seriesReq.GetSeasons()) != 1 || seriesReq.GetSeasons()[0].GetSeasonNumber() != 1 {
		t.Errorf("request of Breaking Bad = %v, want = season 1 pending, with its TVDB ID", seriesReq)
	}
	// Requesting them again conflicts.
	if _, err := request(kidRequests, acquisitionv1.MediaKind_MEDIA_KIND_MOVIE, 603); connect.CodeOf(err) != connect.CodeAlreadyExists {
		t.Errorf("CreateRequest of The Matrix again: %v, want = already exists", err)
	}
	quota, err := kidRequests.GetQuota(ctx, &acquisitionv1.GetQuotaRequest{})
	if err != nil {
		t.Fatalf("GetQuota: %v", err)
	}
	if m, s := quota.GetMovie(), quota.GetSeries(); m.GetLimit() != 5 || m.GetUsed() != 1 || m.GetRemaining() != 4 || m.GetRestricted() ||
		s.GetLimit() != 10 || s.GetUsed() != 1 || s.GetRemaining() != 9 || s.GetRestricted() {
		t.Errorf("GetQuota = %v, want = a movie of 5 and a season of 10 used", quota)
	}
	// Discovery marks the titles requested.
	if films := discoverTitles(trending, acquisitionv1.MediaKind_MEDIA_KIND_MOVIE); len(films) != 1 ||
		films[0].GetStatus() != acquisitionv1.MediaStatus_MEDIA_STATUS_REQUESTED || films[0].GetRequestId() != movieReq.GetId() ||
		films[0].GetRequestStatus() != acquisitionv1.RequestStatus_REQUEST_STATUS_PENDING || films[0].GetRequestable() {
		t.Errorf("trending films = %q, want = The Matrix requested, pending", describeTitles(films))
	}
	// The user cannot approve them; the administrator is told of them.
	if _, err := kidRequests.ApproveRequest(ctx, acquisitionv1.ApproveRequestRequest_builder{Id: new(movieReq.GetId())}.Build()); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("ApproveRequest by the user: %v, want = permission denied", err)
	}
	pending, err := adminRequests.ListRequests(ctx, acquisitionv1.ListRequestsRequest_builder{
		Statuses: []acquisitionv1.RequestStatus{acquisitionv1.RequestStatus_REQUEST_STATUS_PENDING}, Ascending: new(true),
	}.Build())
	if err != nil {
		t.Fatalf("ListRequests: %v", err)
	}
	if rs := pending.GetRequests(); len(rs) != 2 || rs[0].GetId() != movieReq.GetId() || rs[1].GetId() != seriesReq.GetId() {
		t.Errorf("pending requests = %v, want = The Matrix and Breaking Bad", rs)
	}
	// activities lists the activities of a type with a request's ID,
	// everyone's or the user's.
	activities := func(mine bool, typ string) map[string]*systemv1.Activity {
		t.Helper()
		var list []*systemv1.Activity
		if mine {
			resp, err := kidActivity.ListMyActivities(ctx, systemv1.ListMyActivitiesRequest_builder{Types: []string{typ}}.Build())
			if err != nil {
				t.Fatalf("ListMyActivities: %v", err)
			}
			list = resp.GetActivities()
		} else {
			resp, err := activity.ListActivities(ctx, systemv1.ListActivitiesRequest_builder{Limit: new(int32(1000))}.Build())
			if err != nil {
				t.Fatalf("ListActivities: %v", err)
			}
			list = resp.GetActivities()
		}
		out := map[string]*systemv1.Activity{}
		for _, a := range list {
			if a.GetType() == typ {
				out[a.GetAttributes()["request_id"]+a.GetAttributes()["issue_id"]] = a
			}
		}
		return out
	}
	describeActivities := func(mine bool, typ string) func() any {
		return func() any {
			var out []string
			for id, a := range activities(mine, typ) {
				out = append(out, fmt.Sprintf("%s %s %q user=%s", id, a.GetType(), a.GetTitle(), a.GetUserId()))
			}
			return out
		}
	}
	waitFor(t, "the pending requests told", func() bool {
		a := activities(false, "request.pending")
		return a[movieReq.GetId()] != nil && a[seriesReq.GetId()] != nil
	}, describeActivities(false, "request.pending"))

	// 5. The administrator approves them: the film and the series' first
	// season become wanted and are searched for, found, downloaded and
	// imported.
	day := time.Now().Add(-48 * time.Hour)
	movieRel := offered{id: "mx01", title: "The.Matrix.1999.1080p.WEB-DL.DD5.1.H.264-GRPB", size: 6 << 30, published: day}
	pack := offered{
		id: "bb01", title: "Breaking.Bad.S01.1080p.WEB-DL.DD5.1.H.264-GRPB", size: 400 << 20, published: day,
		categories: []int{5000, 5040}, attrs: [][2]string{{"tvdbid", "81189"}, {"season", "S01"}},
	}
	indexer.offer(movieRel, pack)
	approve := func(r *acquisitionv1.Request) *acquisitionv1.Request {
		t.Helper()
		resp, err := adminRequests.ApproveRequest(ctx, acquisitionv1.ApproveRequestRequest_builder{Id: new(r.GetId())}.Build())
		if err != nil {
			t.Fatalf("ApproveRequest %s: %v", r.GetTitle(), err)
		}
		got := resp.GetRequest()
		if got.GetStatus() != acquisitionv1.RequestStatus_REQUEST_STATUS_APPROVED || got.GetItemId() == "" || got.GetModifiedBy() != first.GetUser().GetId() {
			t.Fatalf("approved request of %s = %v, want = approved by the administrator, with its item", r.GetTitle(), got)
		}
		return got
	}
	movieReq = approve(movieReq)
	seriesReq = approve(seriesReq)
	movieID, seriesID := movieReq.GetItemId(), seriesReq.GetItemId()

	getItem := func(id string) *libraryv1.Item {
		t.Helper()
		resp, err := items.GetItem(ctx, libraryv1.GetItemRequest_builder{Id: new(id)}.Build())
		if err != nil {
			t.Fatalf("GetItem: %v", err)
		}
		return resp.GetItem()
	}
	if it := getItem(movieID); it.GetName() != "The Matrix" || it.GetLibraryId() != movieLib || !it.GetVirtual() || !it.GetMonitored() {
		t.Errorf("requested movie = %q in %s virtual=%v monitored=%v, want = The Matrix, wanted in Movies", it.GetName(), it.GetLibraryId(), it.GetVirtual(), it.GetMonitored())
	}
	if it := getItem(seriesID); it.GetName() != "Breaking Bad" || it.GetLibraryId() != showLib || !it.GetMonitored() {
		t.Errorf("requested series = %q in %s monitored=%v, want = Breaking Bad, wanted in Shows", it.GetName(), it.GetLibraryId(), it.GetMonitored())
	}
	children := func(kind libraryv1.ItemKind) []*libraryv1.Item {
		t.Helper()
		resp, err := items.ListItems(ctx, libraryv1.ListItemsRequest_builder{
			ParentId: new(seriesID), Recursive: new(true), Kinds: []libraryv1.ItemKind{kind}, IncludeVirtual: new(true),
		}.Build())
		if err != nil {
			t.Fatalf("ListItems: %v", err)
		}
		return resp.GetItems()
	}
	// The first season alone is monitored.
	for _, s := range children(libraryv1.ItemKind_ITEM_KIND_SEASON) {
		if s.GetMonitored() != (s.GetIndexNumber() == 1) {
			t.Errorf("season %d of Breaking Bad monitored=%v, want = only season 1", s.GetIndexNumber(), s.GetMonitored())
		}
	}
	wantedMovies, err := wanted.ListWanted(ctx, acquisitionv1.ListWantedRequest_builder{
		Filter: new(acquisitionv1.WantedFilter_WANTED_FILTER_MISSING), Kind: new(acquisitionv1.WantedKind_WANTED_KIND_MOVIE),
	}.Build())
	if err != nil || len(wantedMovies.GetMovies()) != 1 || wantedMovies.GetMovies()[0].GetItemId() != movieID {
		t.Errorf("missing movies = %v, %v; want = The Matrix", wantedMovies, err)
	}
	// Discovery marks them wanted.
	if films := discoverTitles(trending, acquisitionv1.MediaKind_MEDIA_KIND_MOVIE); len(films) != 1 ||
		films[0].GetStatus() != acquisitionv1.MediaStatus_MEDIA_STATUS_WANTED || films[0].GetItemId() != movieID ||
		films[0].GetRequestStatus() != acquisitionv1.RequestStatus_REQUEST_STATUS_APPROVED || films[0].GetRequestable() {
		t.Errorf("trending films = %q, want = The Matrix wanted, approved", describeTitles(films))
	}

	queue := func() []*acquisitionv1.QueueItem {
		t.Helper()
		resp, err := downloads.ListQueue(ctx, acquisitionv1.ListQueueRequest_builder{IncludeFinished: new(true)}.Build())
		if err != nil {
			t.Fatalf("ListQueue: %v", err)
		}
		return resp.GetItems()
	}
	describeQueue := func() any {
		var out []string
		for _, q := range queue() {
			out = append(out, fmt.Sprintf("%s %s %s %q", q.GetTitle(), q.GetState(), q.GetStatus(), q.GetMessages()))
		}
		return out
	}
	download := func(title string) *acquisitionv1.QueueItem {
		for _, q := range queue() {
			if q.GetTitle() == title {
				return q
			}
		}
		return nil
	}
	// acquired waits for a release to be grabbed for an item, completes
	// its download with files and waits for its import.
	acquired := func(rel offered, itemID, category string, files ...string) {
		t.Helper()
		waitFor(t, rel.title+" grabbed", func() bool {
			q := download(rel.title)
			return q != nil && sab.job(rel.id) != nil
		}, describeQueue)
		j := sab.job(rel.id)
		if q := download(rel.title); q.GetItemId() != itemID || j.category != category {
			t.Errorf("grabbed %s for %s in category %s, want = for %s in %s", rel.title, q.GetItemId(), j.category, itemID, category)
		}
		waitFor(t, rel.title+" tracked downloading", func() bool {
			runTask(t, tasks, "acquisition.queue")
			return download(rel.title).GetStatus() == acquisitionv1.DownloadStatus_DOWNLOAD_STATUS_DOWNLOADING
		}, describeQueue)
		sab.completeWith(t, j, film, files...)
		waitFor(t, rel.title+" imported", func() bool {
			runTask(t, tasks, "acquisition.queue")
			return download(rel.title).GetState() == acquisitionv1.DownloadState_DOWNLOAD_STATE_IMPORTED
		}, describeQueue)
	}
	acquired(movieRel, movieID, "movies", movieRel.title+".mkv")
	acquired(pack, seriesID, "tv", "Breaking.Bad.S01E01.1080p.WEB-DL.DD5.1.H.264-GRPB.mkv", "Breaking.Bad.S01E02.1080p.WEB-DL.DD5.1.H.264-GRPB.mkv")
	moviePath := filepath.Join(movies, "The Matrix (1999)", "The Matrix (1999) WEBDL-1080p.mkv")
	if it := getItem(movieID); it.GetVirtual() || it.GetPath() != filepath.ToSlash(moviePath) {
		t.Errorf("movie after the import: virtual=%v path=%q, want = %q", it.GetVirtual(), it.GetPath(), filepath.ToSlash(moviePath))
	}
	for _, ep := range children(libraryv1.ItemKind_ITEM_KIND_EPISODE) {
		if ep.GetVirtual() != (ep.GetParentIndexNumber() != 1) {
			t.Errorf("episode S%02dE%02d virtual=%v, want = files for season 1 alone", ep.GetParentIndexNumber(), ep.GetIndexNumber(), ep.GetVirtual())
		}
	}

	// 6. The requests complete as their imports make the film and the
	// season available, and the user is told.
	getRequest := func(id string) *acquisitionv1.Request {
		t.Helper()
		resp, err := kidRequests.GetRequest(ctx, acquisitionv1.GetRequestRequest_builder{Id: new(id)}.Build())
		if err != nil {
			t.Fatalf("GetRequest: %v", err)
		}
		return resp.GetRequest()
	}
	for _, r := range []*acquisitionv1.Request{movieReq, seriesReq} {
		waitFor(t, "the request of "+r.GetTitle()+" completed", func() bool {
			return getRequest(r.GetId()).GetStatus() == acquisitionv1.RequestStatus_REQUEST_STATUS_COMPLETED
		}, func() any { return getRequest(r.GetId()) })
	}
	if s := getRequest(seriesReq.GetId()).GetSeasons(); len(s) != 1 || s[0].GetStatus() != acquisitionv1.RequestStatus_REQUEST_STATUS_COMPLETED {
		t.Errorf("seasons of the request of Breaking Bad = %v, want = season 1 completed", s)
	}
	for _, typ := range []string{"request.approved", "request.available"} {
		waitFor(t, "the user told of "+typ, func() bool {
			a := activities(true, typ)
			return a[movieReq.GetId()] != nil && a[seriesReq.GetId()] != nil
		}, describeActivities(true, typ))
		for _, a := range activities(true, typ) {
			if a.GetUserId() != kidID {
				t.Errorf("%s told to %s, want = the user", typ, a.GetUserId())
			}
		}
	}
	if a := activities(true, "request.available")[movieReq.GetId()]; a.GetItemId() != movieID || a.GetTitle() != "The Matrix (1999) is available" {
		t.Errorf("availability of The Matrix told as %q of %s", a.GetTitle(), a.GetItemId())
	}
	// Discovery marks them available, no longer requestable.
	if films := discoverTitles(trending, acquisitionv1.MediaKind_MEDIA_KIND_MOVIE); len(films) != 1 ||
		films[0].GetStatus() != acquisitionv1.MediaStatus_MEDIA_STATUS_AVAILABLE || films[0].GetItemId() != movieID || films[0].GetRequestable() {
		t.Errorf("trending films = %q, want = The Matrix available", describeTitles(films))
	}
	if series := discoverTitles(popular, acquisitionv1.MediaKind_MEDIA_KIND_SERIES); len(series) != 1 ||
		series[0].GetStatus() != acquisitionv1.MediaStatus_MEDIA_STATUS_AVAILABLE || series[0].GetItemId() != seriesID || series[0].GetRequestable() {
		t.Errorf("popular series = %q, want = Breaking Bad available", describeTitles(series))
	}
	if _, err := request(kidRequests, acquisitionv1.MediaKind_MEDIA_KIND_MOVIE, 603); connect.CodeOf(err) != connect.CodeAlreadyExists {
		t.Errorf("CreateRequest of The Matrix once available: %v, want = already exists", err)
	}

	// The user plays the film, whose source the scan read; users are not
	// told the paths of files.
	var sources []*libraryv1.MediaSource
	waitFor(t, "the media source of The Matrix", func() bool {
		resp, err := playback.ListMediaSources(ctx, playbackv1.ListMediaSourcesRequest_builder{ItemId: new(movieID)}.Build())
		if err != nil {
			t.Fatalf("ListMediaSources: %v", err)
		}
		sources = resp.GetMediaSources()
		return len(sources) == 1 && sources[0].GetPath() == filepath.ToSlash(moviePath) && len(sources[0].GetStreams()) == 2
	}, func() any { return sources })
	if resp, err := kidPlayback.ListMediaSources(ctx, playbackv1.ListMediaSourcesRequest_builder{ItemId: new(movieID)}.Build()); err != nil ||
		len(resp.GetMediaSources()) != 1 || resp.GetMediaSources()[0].GetId() != sources[0].GetId() {
		t.Errorf("the user's ListMediaSources = %v, %v; want = the film's source", resp, err)
	}
	resp, err := kidPlayback.StartPlayback(ctx, playbackv1.StartPlaybackRequest_builder{
		ItemId: new(movieID),
		Capabilities: playbackv1.ClientCapabilities_builder{
			Name: new("tablet"),
			DirectPlay: []*playbackv1.DirectPlayProfile{playbackv1.DirectPlayProfile_builder{
				Kind: new(playbackv1.MediaKind_MEDIA_KIND_VIDEO), Container: new("mkv"), VideoCodec: new("h264"), AudioCodec: new("aac"),
			}.Build()},
		}.Build(),
	}.Build())
	if err != nil {
		t.Fatalf("StartPlayback: %v", err)
	}
	if resp.GetMethod() != playbackv1.PlayMethod_PLAY_METHOD_DIRECT_PLAY {
		t.Fatalf("StartPlayback = %v, want = direct play", resp)
	}
	if body := get(t, srv.url+"/"+resp.GetUrl()); !bytes.Equal(body, filmBytes) {
		t.Errorf("played %d bytes, want = the %d of the imported file", len(body), len(filmBytes))
	}
	if _, err := kidPlayback.StopPlayback(ctx, playbackv1.StopPlaybackRequest_builder{PlaybackId: new(resp.GetPlaybackId())}.Build()); err != nil {
		t.Errorf("StopPlayback: %v", err)
	}
	// The episodes of the first season have their sources too.
	for _, ep := range children(libraryv1.ItemKind_ITEM_KIND_EPISODE) {
		if ep.GetParentIndexNumber() != 1 {
			continue
		}
		waitFor(t, "the media source of "+ep.GetName(), func() bool {
			resp, err := playback.ListMediaSources(ctx, playbackv1.ListMediaSourcesRequest_builder{ItemId: new(ep.GetId())}.Build())
			return err == nil && len(resp.GetMediaSources()) == 1
		})
	}

	// 7. The user reports an issue with the film, which the administrator
	// resolves; the user is told.
	issue, err := kidIssues.CreateIssue(ctx, acquisitionv1.CreateIssueRequest_builder{
		ItemId: new(movieID), Type: new(acquisitionv1.IssueType_ISSUE_TYPE_AUDIO), Message: new("The sound is a sine wave."),
	}.Build())
	if err != nil {
		t.Fatalf("CreateIssue: %v", err)
	}
	issueID := issue.GetIssue().GetId()
	waitFor(t, "the issue told", func() bool { return activities(false, "issue.created")[issueID] != nil }, describeActivities(false, "issue.created"))
	if _, err := kidRequests.DeleteRequest(ctx, acquisitionv1.DeleteRequestRequest_builder{Id: new(movieReq.GetId())}.Build()); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("DeleteRequest of a completed request by the user: %v, want = permission denied", err)
	}
	resolved, err := adminIssues.ResolveIssue(ctx, acquisitionv1.ResolveIssueRequest_builder{Id: new(issueID)}.Build())
	if err != nil {
		t.Fatalf("ResolveIssue: %v", err)
	}
	if resolved.GetIssue().GetStatus() != acquisitionv1.IssueStatus_ISSUE_STATUS_RESOLVED || resolved.GetIssue().GetModifiedBy() != first.GetUser().GetId() {
		t.Errorf("resolved issue = %v, want = resolved by the administrator", resolved.GetIssue())
	}
	waitFor(t, "the user told of the issue resolved", func() bool {
		a := activities(true, "issue.resolved")[issueID]
		return a != nil && a.GetUserId() == kidID && a.GetItemId() == movieID
	}, describeActivities(true, "issue.resolved"))
}
