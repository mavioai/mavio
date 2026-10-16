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
	"github.com/mavioai/mavio/libs/release"
)

// The series of the series acquisition test besides Breaking Bad: an
// anime numbered by absolute numbers, which releases name by a scene
// mapping's title, and a daily series Sonarr's services list as daily.
const (
	animeTvdbID = 900001
	dailyTvdbID = 900002
)

// skyHookJSON returns a SkyHook show whose episodes are three minutes
// long, so that a short file is no sample of them.
func skyHookJSON(t *testing.T, tvdbID int, title, firstAired string, extra map[string]any, episodes []map[string]any) string {
	t.Helper()
	var seasons []map[string]any
	for _, ep := range episodes {
		ep["runtime"] = 3
		n := ep["seasonNumber"]
		if !slices.ContainsFunc(seasons, func(s map[string]any) bool { return s["seasonNumber"] == n }) {
			seasons = append(seasons, map[string]any{"seasonNumber": n})
		}
	}
	show := map[string]any{
		"tvdbId": tvdbID, "title": title, "overview": "SkyHook: " + title + ".", "firstAired": firstAired, "status": "Ended",
		"runtime": 3, "seasons": seasons, "episodes": episodes,
	}
	for k, v := range extra {
		show[k] = v
	}
	data, err := json.Marshal(show)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// seriesRoutes are the answers of SkyHook, TMDB, Sonarr's services and
// XEM for the series acquisition test.
func seriesRoutes(t *testing.T) map[string]string {
	breakingBad := skyHookJSON(t, 81189, "Breaking Bad", "2008-01-20", map[string]any{"tmdbId": 1396, "imdbId": "tt0903747"}, []map[string]any{
		{"tvdbId": 349232, "seasonNumber": 1, "episodeNumber": 1, "absoluteEpisodeNumber": 1, "title": "Pilot", "airDate": "2008-01-20"},
		{"tvdbId": 349235, "seasonNumber": 1, "episodeNumber": 2, "absoluteEpisodeNumber": 2, "title": "Cat's in the Bag...", "airDate": "2008-01-27"},
		{"tvdbId": 349236, "seasonNumber": 2, "episodeNumber": 1, "absoluteEpisodeNumber": 3, "title": "Seven Thirty-Seven", "airDate": "2009-03-08"},
	})
	// Twelve episodes in the first season: the thirteenth is S02E01.
	var anime []map[string]any
	for n := 1; n <= 13; n++ {
		season, episode := 1, n
		if n > 12 {
			season, episode = 2, n-12
		}
		anime = append(anime, map[string]any{
			"tvdbId": 910000 + n, "seasonNumber": season, "episodeNumber": episode, "absoluteEpisodeNumber": n,
			"title": fmt.Sprintf("Delivery %d", n), "airDate": time.Date(2020, 1, 4, 0, 0, 0, 0, time.UTC).AddDate(0, 0, 7*(n-1)).Format(time.DateOnly),
		})
	}
	daily := []map[string]any{
		{"tvdbId": 920001, "seasonNumber": 2024, "episodeNumber": 1, "title": "Monday Edition", "airDate": "2024-03-04"},
		{"tvdbId": 920002, "seasonNumber": 2024, "episodeNumber": 2, "title": "Tuesday Edition", "airDate": "2024-03-05"},
	}
	return map[string]string{
		"skyhook.sonarr.tv/v1/tvdb/shows/en/81189": breakingBad,
		"api.themoviedb.org/3/find/349235":         `{"tv_episode_results": [{"show_id": 1396, "season_number": 1, "episode_number": 2}]}`,
		"api.themoviedb.org/3/tv/1396/season/1/episode/2": `{"id": 62086, "name": "Cat's in the Bag...", "overview": "TMDB: Walt and Jesse dispose of a body.",
			"external_ids": {"tvdb_id": 349235}}`,
		fmt.Sprintf("skyhook.sonarr.tv/v1/tvdb/shows/en/%d", animeTvdbID): skyHookJSON(t, animeTvdbID, "Sky Courier", "2020-01-04",
			map[string]any{"originalLanguage": "ja"}, anime),
		fmt.Sprintf("skyhook.sonarr.tv/v1/tvdb/shows/en/%d", dailyTvdbID): skyHookJSON(t, dailyTvdbID, "Nightly News Hour", "2024-03-04", nil, daily),
		// TMDB knows neither the anime nor the daily series.
		fmt.Sprintf("api.themoviedb.org/3/find/%d", animeTvdbID): `{"tv_results": []}`,
		fmt.Sprintf("api.themoviedb.org/3/find/%d", dailyTvdbID): `{"tv_results": []}`,
		"api.themoviedb.org/3/search/tv":                         `{"page": 1, "results": [], "total_pages": 0, "total_results": 0}`,
		// Releases name the anime by its Japanese title.
		"services.sonarr.tv/v1/scenemapping": fmt.Sprintf(`[{"tvdbId": %d, "title": "Sora no Courier", "searchTitle": "Sora no Courier",
			"season": -1, "sceneSeasonNumber": -1, "sceneOrigin": "unknown"}]`, animeTvdbID),
		"services.sonarr.tv/v1/dailyseries": fmt.Sprintf(`[{"tvdbId": %d}]`, dailyTvdbID),
		// XEM has names and numbering of other series alone.
		"thexem.info/map/allNames": `{"result": "success", "data": {}, "message": ""}`,
		"thexem.info/map/havemap":  `{"result": "success", "data": ["79151"], "message": ""}`,
	}
}

// TestSeriesAcquisition runs the assembled server with the indexers and
// the download clients plugins against a Newznab indexer and SABnzbd
// faked on the loopback interface, and the metadata plugins against fakes
// of their services (P19's done criteria): the wanted episodes of a
// series are found as a season pack by a season search and singly by an
// episode search; an anime's episode is matched through its absolute
// number under the title of a scene mapping, and a daily series' through
// its air date; each episode is imported under Sonarr's naming and
// becomes playable.
func TestSeriesAcquisition(t *testing.T) {
	t.Parallel()
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("integration test: ffmpeg not found on PATH")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("integration test: ffprobe not found on PATH")
	}
	ctx := t.Context()

	// The episode the downloads hold: twenty seconds with audio, no sample
	// of an episode of three minutes, of the 1080p the releases name, as
	// the imports aggregate the frame size into their qualities.
	video := filepath.Join(t.TempDir(), "episode.mkv")
	run(t, ffmpeg, "-loglevel", "error", "-f", "lavfi", "-i", "testsrc2=size=1920x1080:rate=1",
		"-f", "lavfi", "-i", "sine=frequency=440:sample_rate=8000", "-t", "20",
		"-c:v", "libx264", "-preset", "ultrafast", "-c:a", "aac", "-b:a", "16k", video)
	settled(t, video)
	videoBytes, err := os.ReadFile(video)
	if err != nil {
		t.Fatal(err)
	}

	shows := t.TempDir()
	indexer := newFakeNewznab(t)
	sab := newFakeSABnzbd(t, t.TempDir())
	fake := &fakeServices{mode: tmdbAcceptsRadarr, routes: seriesRoutes(t)}
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
	first, err := authv1connect.NewAuthServiceClient(http.DefaultClient, srv.url).CreateFirstUser(ctx, authv1.CreateFirstUserRequest_builder{
		Name: new("admin"), Password: new("secret"), Device: device("e2e"),
	}.Build())
	if err != nil {
		t.Fatalf("CreateFirstUser: %v", err)
	}
	token := withToken(first.GetAccessToken())
	system := systemv1connect.NewSystemServiceClient(http.DefaultClient, srv.url, token)
	pluginAdmin := systemv1connect.NewPluginManagementServiceClient(http.DefaultClient, srv.url, token)
	tasks := systemv1connect.NewTaskServiceClient(http.DefaultClient, srv.url, token)
	libraries := libraryv1connect.NewLibraryServiceClient(http.DefaultClient, srv.url, token)
	items := libraryv1connect.NewItemServiceClient(http.DefaultClient, srv.url, token)
	playback := playbackv1connect.NewPlaybackServiceClient(http.DefaultClient, srv.url, token)
	profiles := acquisitionv1connect.NewProfileServiceClient(http.DefaultClient, srv.url, token)
	wanted := acquisitionv1connect.NewWantedServiceClient(http.DefaultClient, srv.url, token)
	downloads := acquisitionv1connect.NewDownloadServiceClient(http.DefaultClient, srv.url, token)
	releases := acquisitionv1connect.NewReleaseServiceClient(http.DefaultClient, srv.url, token)

	// The administrator sets acquisition up: a recycle bin, renaming
	// episodes; RSS syncs only when asked.
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
	if _, err := system.UpdateServerSettings(ctx, systemv1.UpdateServerSettingsRequest_builder{Settings: settings}.Build()); err != nil {
		t.Fatalf("UpdateServerSettings: %v", err)
	}
	lib, err := libraries.CreateLibrary(ctx, libraryv1.CreateLibraryRequest_builder{
		Spec: libraryv1.LibrarySpec_builder{Name: new("Shows"), Kind: new(libraryv1.LibraryKind_LIBRARY_KIND_SHOWS), Paths: []string{shows}}.Build(),
	}.Build())
	if err != nil {
		t.Fatalf("CreateLibrary: %v", err)
	}
	for id, cfg := range map[string]any{
		"dev.mavio.indexers": map[string]any{"indexers": []any{
			map[string]any{"id": "nzb", "definition": "newznab", "settings": map[string]any{"base_url": indexer.URL, "api_key": "nzbkey"}},
		}},
		"dev.mavio.download-clients": map[string]any{"clients": []any{
			map[string]any{"id": "sab", "name": "SABnzbd", "definition": "sabnzbd", "settings": map[string]any{
				"host": "127.0.0.1", "port": sab.Listener.Addr().(*net.TCPAddr).Port, "api_key": sabAPIKey, "movie_category": "movies", "tv_category": "tv",
			}},
		}},
	} {
		data, err := json.Marshal(cfg)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pluginAdmin.SetPluginConfig(ctx, systemv1.SetPluginConfigRequest_builder{PluginId: new(id), ConfigJson: new(string(data))}.Build()); err != nil {
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

	// runTask runs a task and waits for its run to finish, unless a run of
	// it was queued already.
	runTask := func(id string) {
		t.Helper()
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
	served := func(path string) int {
		fake.mu.Lock()
		defer fake.mu.Unlock()
		return fake.served[path]
	}
	// The scene mappings and daily series are read from Sonarr's services
	// once scraper-servarr runs.
	waitFor(t, "the scene mappings", func() bool {
		before := served("services.sonarr.tv/v1/dailyseries")
		runTask("acquisition.scene_mappings")
		return served("services.sonarr.tv/v1/dailyseries") > before && served("services.sonarr.tv/v1/scenemapping") > 0
	})

	// A profile upgrading up to WEB-DL 1080p, from HDTV-720p.
	hd := acquire.NewProfile(release.Series, "HD", release.WEBDL1080p, []release.Quality{release.HDTV720p, release.HDTV1080p, release.WEBDL1080p}, nil)
	stored, err := acquire.QualityProfileToCore(hd, core.KindSeries)
	if err != nil {
		t.Fatal(err)
	}
	profile, err := profiles.CreateQualityProfile(ctx, acquisitionv1.CreateQualityProfileRequest_builder{
		Profile: acquisitionv1.QualityProfile_builder{
			Kind: new(acquisitionv1.MediaKind_MEDIA_KIND_SERIES), Name: new("HD"), UpgradeAllowed: new(true),
			Cutoff: new(int32(stored.Cutoff)), Items: profileItemsToProto(stored.Items),
		}.Build(),
	}.Build())
	if err != nil {
		t.Fatalf("CreateQualityProfile: %v", err)
	}

	// addSeries adds a series monitoring all its episodes, into season
	// folders, and returns its item.
	addSeries := func(tvdbID int, typ libraryv1.SeriesType) *libraryv1.Item {
		t.Helper()
		added, err := wanted.AddSeries(ctx, acquisitionv1.AddSeriesRequest_builder{
			LibraryId: new(lib.GetLibrary().GetId()), TvdbId: new(int32(tvdbID)), QualityProfileId: new(profile.GetProfile().GetId()),
			SeriesType: new(typ), SeasonFolder: new(true), Monitor: new(acquisitionv1.SeriesMonitor_SERIES_MONITOR_ALL),
		}.Build())
		if err != nil {
			t.Fatalf("AddSeries %d: %v", tvdbID, err)
		}
		resp, err := items.GetItem(ctx, libraryv1.GetItemRequest_builder{Id: new(added.GetItemId())}.Build())
		if err != nil {
			t.Fatalf("GetItem: %v", err)
		}
		return resp.GetItem()
	}
	children := func(parent *libraryv1.Item, kind libraryv1.ItemKind) []*libraryv1.Item {
		t.Helper()
		resp, err := items.ListItems(ctx, libraryv1.ListItemsRequest_builder{
			ParentId: new(parent.GetId()), Recursive: new(true), Kinds: []libraryv1.ItemKind{kind}, IncludeVirtual: new(true),
		}.Build())
		if err != nil {
			t.Fatalf("ListItems: %v", err)
		}
		return resp.GetItems()
	}
	// episode returns an episode of a series by its season and number.
	episode := func(s *libraryv1.Item, season, number int32) *libraryv1.Item {
		t.Helper()
		for _, it := range children(s, libraryv1.ItemKind_ITEM_KIND_EPISODE) {
			if it.GetParentIndexNumber() == season && it.GetIndexNumber() == number {
				return it
			}
		}
		t.Fatalf("%s has no episode S%02dE%02d", s.GetName(), season, number)
		return nil
	}
	describeEpisodes := func(s *libraryv1.Item) func() any {
		return func() any {
			var out []string
			for _, it := range children(s, libraryv1.ItemKind_ITEM_KIND_EPISODE) {
				out = append(out, fmt.Sprintf("S%02dE%02d #%d %q %v monitored=%v virtual=%v %q", it.GetParentIndexNumber(), it.GetIndexNumber(),
					it.GetAbsoluteNumber(), it.GetName(), it.GetPremiereDate().AsTime().Format(time.DateOnly), it.GetMonitored(), it.GetVirtual(), it.GetPath()))
			}
			return out
		}
	}

	// What the server tells of the downloads.
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
			out = append(out, fmt.Sprintf("%s %s %s %v %q", q.GetTitle(), q.GetState(), q.GetStatus(), q.GetRemainingSize(), q.GetMessages()))
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
	history := func(it *libraryv1.Item, typ acquisitionv1.HistoryEventType) []*acquisitionv1.HistoryEvent {
		t.Helper()
		resp, err := downloads.ListHistory(ctx, acquisitionv1.ListHistoryRequest_builder{
			ItemId: new(it.GetId()), Types: []acquisitionv1.HistoryEventType{typ},
		}.Build())
		if err != nil {
			t.Fatalf("ListHistory: %v", err)
		}
		return resp.GetEvents()
	}
	// search searches for items of a series and waits for a release to be
	// sent to SABnzbd in the TV category, for the series.
	search := func(s *libraryv1.Item, rel offered, quality string, ids ...string) *sabJob {
		t.Helper()
		indexer.offer(rel)
		resp, err := wanted.SearchWanted(ctx, acquisitionv1.SearchWantedRequest_builder{ItemIds: ids}.Build())
		if err != nil || resp.GetQueued() != 1 {
			t.Fatalf("SearchWanted = %v, %v; want = a search queued", resp, err)
		}
		var q *acquisitionv1.QueueItem
		waitFor(t, rel.title+" grabbed", func() bool {
			q = download(rel.title)
			return q != nil && sab.job(rel.id) != nil
		}, describeQueue)
		j := sab.job(rel.id)
		if q.GetState() != acquisitionv1.DownloadState_DOWNLOAD_STATE_DOWNLOADING || q.GetQuality() != quality || q.GetItemId() != s.GetId() ||
			q.GetClientId() != "sab" || q.GetDownloadId() != j.id || q.GetIndexer().GetIndexerId() != "nzb" {
			t.Errorf("queued %s = %v, want = downloading %s in SABnzbd for %s, from the indexer", rel.title, q, quality, s.GetName())
		}
		if j.category != "tv" {
			t.Errorf("SABnzbd job %s in category %q, want = tv", j.name, j.category)
		}
		return j
	}
	// imported completes a download with files and waits for its import;
	// want are the episodes it holds and the files they are imported to.
	type imported struct {
		episode *libraryv1.Item
		path    string
	}
	importDownload := func(rel offered, j *sabJob, files []string, want []imported) {
		t.Helper()
		waitFor(t, rel.title+" tracked downloading", func() bool {
			runTask("acquisition.queue")
			return download(rel.title).GetStatus() == acquisitionv1.DownloadStatus_DOWNLOAD_STATUS_DOWNLOADING
		}, describeQueue)
		sab.completeWith(t, j, video, files...)
		waitFor(t, rel.title+" imported", func() bool {
			runTask("acquisition.queue")
			return download(rel.title).GetState() == acquisitionv1.DownloadState_DOWNLOAD_STATE_IMPORTED
		}, describeQueue)
		waitFor(t, j.name+" removed from SABnzbd", func() bool { return sab.removed(j) })
		if _, err := os.Stat(filepath.Join(sab.complete, j.category, j.name)); !os.IsNotExist(err) {
			t.Errorf("job folder of %s after the import: %v, want = removed", j.name, err)
		}
		for _, w := range want {
			if _, err := os.Stat(w.path); err != nil {
				t.Errorf("imported file: %v", err)
			}
			resp, err := items.GetItem(ctx, libraryv1.GetItemRequest_builder{Id: new(w.episode.GetId())}.Build())
			if err != nil {
				t.Fatalf("GetItem: %v", err)
			}
			if it := resp.GetItem(); it.GetVirtual() || it.GetPath() != filepath.ToSlash(w.path) {
				t.Errorf("episode %q after the import: virtual=%v path=%q, want = %q", it.GetName(), it.GetVirtual(), it.GetPath(), filepath.ToSlash(w.path))
			}
			grabs, imports := history(w.episode, acquisitionv1.HistoryEventType_HISTORY_EVENT_TYPE_GRABBED), history(w.episode, acquisitionv1.HistoryEventType_HISTORY_EVENT_TYPE_IMPORTED)
			if len(grabs) != 1 || grabs[0].GetSourceTitle() != rel.title || grabs[0].GetDownloadId() != j.id {
				t.Errorf("grabs of %q = %v, want = %s", w.episode.GetName(), grabs, rel.title)
			}
			if len(imports) != 1 || imports[0].GetData()["importedPath"] != filepath.ToSlash(w.path) || imports[0].GetDownloadId() != j.id {
				t.Errorf("imports of %q = %v, want = to %s", w.episode.GetName(), imports, w.path)
			}
			// The scan of the library reads the file as the episode's source.
			var sources []*libraryv1.MediaSource
			waitFor(t, "the media source of "+filepath.Base(w.path), func() bool {
				resp, err := playback.ListMediaSources(ctx, playbackv1.ListMediaSourcesRequest_builder{ItemId: new(w.episode.GetId())}.Build())
				if err != nil {
					t.Fatalf("ListMediaSources: %v", err)
				}
				sources = resp.GetMediaSources()
				return len(sources) == 1 && sources[0].GetPath() == filepath.ToSlash(w.path) && len(sources[0].GetStreams()) == 2
			}, func() any { return sources })
		}
	}
	day := time.Now().Add(-48 * time.Hour)
	tv := []int{5000, 5040}

	// 1. Breaking Bad: a season search finds its first season as a pack,
	// whose files are imported each as its episode.
	bb := addSeries(81189, libraryv1.SeriesType_SERIES_TYPE_STANDARD)
	if bb.GetName() != "Breaking Bad" || bb.GetSeriesType() != libraryv1.SeriesType_SERIES_TYPE_STANDARD {
		t.Fatalf("added series = %q %v, want = Breaking Bad, standard", bb.GetName(), bb.GetSeriesType())
	}
	waitFor(t, "the episodes of Breaking Bad", func() bool { return len(children(bb, libraryv1.ItemKind_ITEM_KIND_EPISODE)) == 3 }, describeEpisodes(bb))
	var season1 *libraryv1.Item
	for _, s := range children(bb, libraryv1.ItemKind_ITEM_KIND_SEASON) {
		if s.GetIndexNumber() == 1 {
			season1 = s
		}
	}
	if season1 == nil {
		t.Fatalf("Breaking Bad has no season 1")
	}
	pilot, second, s2e1 := episode(bb, 1, 1), episode(bb, 1, 2), episode(bb, 2, 1)
	bbFolder := filepath.Join(shows, "Breaking Bad")
	pack := offered{
		id: "bb01", title: "Breaking.Bad.S01.1080p.WEB-DL.DD5.1.H.264-GRPB", size: 400 << 20, published: day,
		categories: tv, attrs: [][2]string{{"tvdbid", "81189"}, {"season", "S01"}},
	}
	job := search(bb, pack, "WEBDL-1080p", season1.GetId())
	pilotPath := filepath.Join(bbFolder, "Season 1", "Breaking Bad - S01E01 - Pilot WEBDL-1080p.mkv")
	importDownload(pack, job, []string{
		"Breaking.Bad.S01E01.1080p.WEB-DL.DD5.1.H.264-GRPB.mkv", "Breaking.Bad.S01E02.1080p.WEB-DL.DD5.1.H.264-GRPB.mkv",
	}, []imported{
		{pilot, pilotPath},
		{second, filepath.Join(bbFolder, "Season 1", "Breaking Bad - S01E02 - Cat's in the Bag WEBDL-1080p.mkv")},
	})

	// The pilot plays.
	resp, err := playback.StartPlayback(ctx, playbackv1.StartPlaybackRequest_builder{
		ItemId: new(pilot.GetId()),
		Capabilities: playbackv1.ClientCapabilities_builder{
			Name: new("e2e"),
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
	if body := get(t, srv.url+"/"+resp.GetUrl()); !bytes.Equal(body, videoBytes) {
		t.Errorf("played %d bytes, want = the %d of the imported file", len(body), len(videoBytes))
	}
	if _, err := playback.StopPlayback(ctx, playbackv1.StopPlaybackRequest_builder{PlaybackId: new(resp.GetPlaybackId())}.Build()); err != nil {
		t.Errorf("StopPlayback: %v", err)
	}

	// 2. An episode search finds the episode of the second season singly.
	single := offered{
		id: "bb02", title: "Breaking.Bad.S02E01.1080p.WEB-DL.DD5.1.H.264-GRPB", size: 200 << 20, published: day,
		categories: tv, attrs: [][2]string{{"tvdbid", "81189"}, {"season", "S02"}, {"episode", "E01"}},
	}
	job = search(bb, single, "WEBDL-1080p", s2e1.GetId())
	importDownload(single, job, []string{single.title + ".mkv"}, []imported{
		{s2e1, filepath.Join(bbFolder, "Season 2", "Breaking Bad - S02E01 - Seven Thirty-Seven WEBDL-1080p.mkv")},
	})

	// 3. An anime's thirteenth episode, S02E01, released under the title
	// of its scene mapping and by its absolute number, without IDs.
	anime := addSeries(animeTvdbID, libraryv1.SeriesType_SERIES_TYPE_ANIME)
	if anime.GetName() != "Sky Courier" || anime.GetSeriesType() != libraryv1.SeriesType_SERIES_TYPE_ANIME {
		t.Fatalf("added series = %q %v, want = Sky Courier, anime", anime.GetName(), anime.GetSeriesType())
	}
	waitFor(t, "the episodes of Sky Courier", func() bool { return len(children(anime, libraryv1.ItemKind_ITEM_KIND_EPISODE)) == 13 }, describeEpisodes(anime))
	thirteenth := episode(anime, 2, 1)
	if thirteenth.GetAbsoluteNumber() != 13 {
		t.Errorf("absolute number of S02E01 = %d, want = 13", thirteenth.GetAbsoluteNumber())
	}
	animeRel := offered{
		id: "sc13", title: "[SubGroup] Sora no Courier - 13 [1080p]", size: 200 << 20, published: day,
		categories: []int{5000, 5070}, attrs: [][2]string{},
	}
	job = search(anime, animeRel, "HDTV-1080p", thirteenth.GetId())
	importDownload(animeRel, job, []string{animeRel.title + ".mkv"}, []imported{
		{thirteenth, filepath.Join(shows, "Sky Courier", "Season 2", "Sky Courier - S02E01 - Delivery 13 HDTV-1080p.mkv")},
	})

	// 4. A daily series, daily as Sonarr's services list it: the episode
	// of an air date.
	news := addSeries(dailyTvdbID, libraryv1.SeriesType_SERIES_TYPE_UNSPECIFIED)
	if news.GetName() != "Nightly News Hour" || news.GetSeriesType() != libraryv1.SeriesType_SERIES_TYPE_DAILY {
		t.Fatalf("added series = %q %v, want = Nightly News Hour, daily", news.GetName(), news.GetSeriesType())
	}
	waitFor(t, "the episodes of Nightly News Hour", func() bool { return len(children(news, libraryv1.ItemKind_ITEM_KIND_EPISODE)) == 2 }, describeEpisodes(news))
	tuesday := episode(news, 2024, 2)
	dailyRel := offered{
		id: "nn05", title: "Nightly.News.Hour.2024.03.05.1080p.WEB-DL.AAC2.0.H.264-GRPN", size: 200 << 20, published: day,
		categories: tv, attrs: [][2]string{{"tvdbid", fmt.Sprint(dailyTvdbID)}},
	}
	job = search(news, dailyRel, "WEBDL-1080p", tuesday.GetId())
	importDownload(dailyRel, job, []string{dailyRel.title + ".mkv"}, []imported{
		{tuesday, filepath.Join(shows, "Nightly News Hour", "Season 2024", "Nightly News Hour - 2024-03-05 - Tuesday Edition WEBDL-1080p.mkv")},
	})

	// The episodes not grabbed are still missing; those imported are not.
	resp2, err := wanted.ListWanted(ctx, acquisitionv1.ListWantedRequest_builder{
		Filter: new(acquisitionv1.WantedFilter_WANTED_FILTER_MISSING), Kind: new(acquisitionv1.WantedKind_WANTED_KIND_EPISODE),
	}.Build())
	if err != nil {
		t.Fatalf("ListWanted: %v", err)
	}
	var missing []string
	for _, e := range resp2.GetEpisodes() {
		missing = append(missing, fmt.Sprintf("%s S%02dE%02d", e.GetSeriesName(), e.GetSeasonNumber(), e.GetEpisodeNumber()))
	}
	for _, imported := range []string{"Breaking Bad S01E01", "Breaking Bad S01E02", "Breaking Bad S02E01", "Sky Courier S02E01", "Nightly News Hour S2024E02"} {
		if slices.Contains(missing, imported) {
			t.Errorf("missing episodes %q list %s, imported", missing, imported)
		}
	}
	if len(missing) != 13 {
		t.Errorf("missing episodes = %q, want = the 12 of Sky Courier's first season and Nightly News Hour's first", missing)
	}
}
