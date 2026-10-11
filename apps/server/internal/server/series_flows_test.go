package server_test

import (
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"

	acquisitionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1"
	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
	"github.com/mavioai/mavio/libs/release"
)

// sceneTvdbID is the series of the scene numbering test, which releases
// number otherwise than TVDB does.
const sceneTvdbID = 900003

// sabnzbd configures a Newznab indexer and SABnzbd as the indexers and
// the download clients plugins' own.
func (s *acquisitionServer) sabnzbd(indexer *fakeNewznab, sab *fakeSABnzbd) {
	s.t.Helper()
	s.plugins([]any{
		map[string]any{"id": "nzb", "definition": "newznab", "settings": map[string]any{"base_url": indexer.URL, "api_key": "nzbkey"}},
	}, []any{
		map[string]any{"id": "sab", "name": "SABnzbd", "definition": "sabnzbd", "settings": map[string]any{
			"host": "127.0.0.1", "port": sab.Listener.Addr().(*net.TCPAddr).Port, "api_key": sabAPIKey, "movie_category": "movies", "tv_category": "tv",
		}},
	})
}

// sabGrabbed waits for a release to be sent to SABnzbd for an item and
// tracked downloading, and returns its job.
func (s *acquisitionServer) sabGrabbed(sab *fakeSABnzbd, rel offered, itemID, quality string) *sabJob {
	s.t.Helper()
	var q *acquisitionv1.QueueItem
	waitFor(s.t, rel.title+" grabbed", func() bool {
		q = s.download(rel.title)
		return q != nil && sab.job(rel.id) != nil
	}, s.describeQueue)
	if q.GetQuality() != quality || q.GetItemId() != itemID || q.GetClientId() != "sab" || q.GetIndexer().GetIndexerId() != "nzb" {
		s.t.Errorf("queued %s = %v, want = %s in SABnzbd for %s, from the indexer", rel.title, q, quality, itemID)
	}
	s.waitForDownload(rel.title, acquisitionv1.DownloadState_DOWNLOAD_STATE_DOWNLOADING)
	return sab.job(rel.id)
}

// sabImported completes a SABnzbd job with a video under its name and
// waits for its import.
func (s *acquisitionServer) sabImported(sab *fakeSABnzbd, rel offered, j *sabJob, video string) {
	s.t.Helper()
	sab.completeWith(s.t, j, video, j.name+".mkv")
	s.waitForDownload(rel.title, acquisitionv1.DownloadState_DOWNLOAD_STATE_IMPORTED)
	waitFor(s.t, j.name+" removed from SABnzbd", func() bool { return sab.removed(j) })
}

// cutoffUnmet lists the episodes below their profiles' cutoffs.
func (s *acquisitionServer) cutoffUnmet() []string {
	s.t.Helper()
	resp, err := s.wanted.ListWanted(s.t.Context(), acquisitionv1.ListWantedRequest_builder{
		Filter: new(acquisitionv1.WantedFilter_WANTED_FILTER_CUTOFF_UNMET), Kind: new(acquisitionv1.WantedKind_WANTED_KIND_EPISODE),
	}.Build())
	if err != nil {
		s.t.Fatalf("ListWanted: %v", err)
	}
	var out []string
	for _, e := range resp.GetEpisodes() {
		out = append(out, fmt.Sprintf("%s S%02dE%02d", e.GetSeriesName(), e.GetSeasonNumber(), e.GetEpisodeNumber()))
	}
	slices.Sort(out)
	return out
}

// TestSeriesRSSUpgradesAndFailures runs the assembled server against a
// Newznab indexer and SABnzbd faked on the loopback interface (P19's
// done criteria for RSS syncs, multi-episode files, upgrades and failed
// downloads): an RSS sync grabs a release of two episodes, imported as
// one file of both under Sonarr's naming; another RSS sync upgrades both
// episodes, the replaced file going to the recycle bin; a download of an
// episode SABnzbd fails is blocklisted and replaced by a new search,
// which grabs another release, as Sonarr's FailedDownloadService and
// RedownloadFailedDownloadService.
func TestSeriesRSSUpgradesAndFailures(t *testing.T) {
	s := startAcquisition(t, seriesRoutes(t), true)
	ctx := t.Context()
	// Two minutes, no sample of one or two episodes of three minutes (Sonarr's
	// DetectSample sums the runtimes of a file's episodes), of the frame sizes
	// the releases name.
	v720, v1080 := s.video("1280x720", 120), s.video("1920x1080", 120)

	shows := t.TempDir()
	indexer := newFakeNewznab(t)
	sab := newFakeSABnzbd(t, t.TempDir())
	s.sabnzbd(indexer, sab)
	s.sceneMappings()
	lib := s.library("Shows", libraryv1.LibraryKind_LIBRARY_KIND_SHOWS, shows)
	profile := s.profile(release.Series, release.WEBDL1080p, release.HDTV720p, release.HDTV1080p, release.WEBDL1080p)
	bb := s.addSeries(lib, profile, 81189, libraryv1.SeriesType_SERIES_TYPE_STANDARD, 3)
	pilot, second, s2e1 := s.episode(bb, 1, 1), s.episode(bb, 1, 2), s.episode(bb, 2, 1)
	season1 := filepath.Join(shows, "Breaking Bad", "Season 1")
	day := time.Now().Add(-48 * time.Hour)
	tv := []int{5000, 5040}
	bbAttrs := [][2]string{{"tvdbid", "81189"}}

	// path returns the file of an episode, empty when it has none.
	path := func(ep *libraryv1.Item) string {
		t.Helper()
		if it := s.item(ep.GetId()); !it.GetVirtual() {
			return it.GetPath()
		}
		return ""
	}

	// 1. An RSS sync grabs a release of the first two episodes, imported
	// as one file of both.
	multi := offered{id: "mm01", title: "Breaking.Bad.S01E01E02.720p.HDTV.x264-GRPA", size: 300 << 20, published: day, categories: tv, attrs: bbAttrs}
	indexer.offer(multi)
	if _, err := s.wanted.SyncRSS(ctx, &acquisitionv1.SyncRSSRequest{}); err != nil {
		t.Fatalf("SyncRSS: %v", err)
	}
	job := s.sabGrabbed(sab, multi, bb, "HDTV-720p")
	s.sabImported(sab, multi, job, v720)
	first := filepath.Join(season1, "Breaking Bad - S01E01-E02 - Pilot + Cat's in the Bag HDTV-720p.mkv")
	// holds checks that the pilot's item holds a file of both episodes,
	// the second's item gone, as the library keeps a multi-episode file.
	holds := func(want string) {
		t.Helper()
		if got := path(pilot); got != filepath.ToSlash(want) {
			t.Errorf("file of S01E01 = %q, want = %q", got, filepath.ToSlash(want))
		}
		if it := s.item(pilot.GetId()); it.GetIndexNumberEnd() != 2 || it.GetName() != "Pilot / Cat's in the Bag..." {
			t.Errorf("S01E01's item = %q to episode %d, want = \"Pilot / Cat's in the Bag...\" to episode 2", it.GetName(), it.GetIndexNumberEnd())
		}
		if _, err := s.items.GetItem(ctx, libraryv1.GetItemRequest_builder{Id: new(second.GetId())}.Build()); connect.CodeOf(err) != connect.CodeNotFound {
			t.Errorf("GetItem S01E02 = %v, want = not found, held by S01E01's file", err)
		}
	}
	holds(first)
	if imports := s.history(pilot.GetId(), acquisitionv1.HistoryEventType_HISTORY_EVENT_TYPE_IMPORTED); len(imports) != 1 ||
		imports[0].GetSourceTitle() != multi.title || imports[0].GetData()["importedPath"] != filepath.ToSlash(first) {
		t.Errorf("imports of S01E01 = %v, want = %s to %s", imports, multi.title, first)
	}
	if got, want := s.cutoffUnmet(), []string{"Breaking Bad S01E01", "Breaking Bad S01E02"}; !slices.Equal(got, want) {
		t.Errorf("cutoff unmet = %q, want = %q", got, want)
	}

	// 2. Another RSS sync finds both episodes as a WEB-DL, which upgrades
	// them; the HDTV file goes to the recycle bin.
	better := offered{id: "mm02", title: "Breaking.Bad.S01E01E02.1080p.WEB-DL.DD5.1.H.264-GRPB", size: 600 << 20, published: day, categories: tv, attrs: bbAttrs}
	indexer.offer(better)
	if _, err := s.wanted.SyncRSS(ctx, &acquisitionv1.SyncRSSRequest{}); err != nil {
		t.Fatalf("SyncRSS: %v", err)
	}
	job = s.sabGrabbed(sab, better, bb, "WEBDL-1080p")
	s.sabImported(sab, better, job, v1080)
	upgraded := filepath.Join(season1, "Breaking Bad - S01E01-E02 - Pilot + Cat's in the Bag WEBDL-1080p.mkv")
	holds(upgraded)
	if _, err := os.Stat(first); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("replaced file: %v, want = gone", err)
	}
	if _, err := os.Stat(filepath.Join(s.recycleBin, "Breaking Bad", filepath.Base(first))); err != nil {
		t.Errorf("replaced file in the recycle bin: %v", err)
	}
	if deleted := s.history(pilot.GetId(), acquisitionv1.HistoryEventType_HISTORY_EVENT_TYPE_FILE_DELETED); len(deleted) != 1 || deleted[0].GetData()["reason"] != "Upgrade" {
		t.Errorf("deleted files of S01E01 = %v, want = the HDTV file, upgraded", deleted)
	}
	if got := s.cutoffUnmet(); len(got) != 0 {
		t.Errorf("cutoff unmet = %q, want = none", got)
	}
	// A third RSS sync grabs nothing: the episodes are at their cutoff.
	if _, err := s.wanted.SyncRSS(ctx, &acquisitionv1.SyncRSSRequest{}); err != nil {
		t.Fatalf("SyncRSS: %v", err)
	}

	// 3. A search grabs the second season's episode, which SABnzbd fails:
	// it is blocklisted, and the search that follows grabs another.
	failing := offered{
		id: "ff01", title: "Breaking.Bad.S02E01.1080p.WEB-DL.DD5.1.H.264-GRPC", size: 200 << 20, published: day,
		categories: tv, attrs: [][2]string{{"tvdbid", "81189"}, {"season", "S02"}, {"episode", "E01"}},
	}
	replacement := offered{
		id: "ff02", title: "Breaking.Bad.S02E01.1080p.WEB-DL.DD5.1.H.264-GRPD", size: 200 << 20, published: day.Add(-time.Hour),
		categories: tv, attrs: [][2]string{{"tvdbid", "81189"}, {"season", "S02"}, {"episode", "E01"}},
	}
	indexer.offer(failing)
	if _, err := s.wanted.SearchWanted(ctx, acquisitionv1.SearchWantedRequest_builder{ItemIds: []string{s2e1.GetId()}}.Build()); err != nil {
		t.Fatalf("SearchWanted: %v", err)
	}
	job = s.sabGrabbed(sab, failing, bb, "WEBDL-1080p")
	indexer.offer(replacement)
	sab.finish(t, job, "", "Repair failed, not enough repair blocks (100 short)")
	s.waitForDownload(failing.title, acquisitionv1.DownloadState_DOWNLOAD_STATE_FAILED)
	if failed := s.history(s2e1.GetId(), acquisitionv1.HistoryEventType_HISTORY_EVENT_TYPE_DOWNLOAD_FAILED); len(failed) != 1 ||
		failed[0].GetSourceTitle() != failing.title || !strings.Contains(failed[0].GetData()["message"], "Repair failed") {
		t.Errorf("failed downloads of S02E01 = %v, want = %s with SABnzbd's message", failed, failing.title)
	}
	block, err := s.downloads.ListBlocklist(ctx, acquisitionv1.ListBlocklistRequest_builder{ItemId: new(bb)}.Build())
	if err != nil || len(block.GetEntries()) != 1 || block.GetEntries()[0].GetSourceTitle() != failing.title ||
		!strings.Contains(block.GetEntries()[0].GetMessage(), "Repair failed") {
		t.Errorf("ListBlocklist = %v, %v; want = %s with SABnzbd's message", block, err, failing.title)
	}
	waitFor(t, "the failed job removed from SABnzbd", func() bool { return sab.removed(job) })
	job = s.sabGrabbed(sab, replacement, bb, "WEBDL-1080p")
	s.sabImported(sab, replacement, job, v1080)
	if got, want := path(s2e1), filepath.ToSlash(filepath.Join(shows, "Breaking Bad", "Season 2", "Breaking Bad - S02E01 - Seven Thirty-Seven WEBDL-1080p.mkv")); got != want {
		t.Errorf("file of S02E01 = %q, want = %q", got, want)
	}
	// The failed release stays rejected as blocklisted.
	found, err := s.releases.SearchReleases(ctx, acquisitionv1.SearchReleasesRequest_builder{ItemId: new(s2e1.GetId())}.Build())
	if err != nil {
		t.Fatalf("SearchReleases: %v", err)
	}
	i := slices.IndexFunc(found.GetReleases(), func(r *acquisitionv1.Release) bool { return r.GetTitle() == failing.title })
	if i < 0 {
		t.Errorf("SearchReleases found no %s", failing.title)
	} else if r := found.GetReleases()[i]; !slices.ContainsFunc(r.GetRejections(), func(rj *acquisitionv1.Rejection) bool { return rj.GetReason() == "blocklisted" }) {
		t.Errorf("rejections of the failed release = %v, want = blocklisted", r.GetRejections())
	}
}

// TestSceneNumbering runs the assembled server with scraper-servarr
// against a fake of XEM (P19's done criteria for scene numbering): XEM
// numbers a series' second season's first episode as the first season's
// third; a search for it asks the indexer for the scene's numbers, and
// the release named by them is imported as TVDB numbers the episode, as
// Sonarr's XemService and SceneMappingService.
func TestSceneNumbering(t *testing.T) {
	routes := seriesRoutes(t)
	routes[fmt.Sprintf("skyhook.sonarr.tv/v1/tvdb/shows/en/%d", sceneTvdbID)] = skyHookJSON(t, sceneTvdbID, "Scene Shuffle", "2021-02-01", nil, []map[string]any{
		{"tvdbId": 930001, "seasonNumber": 1, "episodeNumber": 1, "absoluteEpisodeNumber": 1, "title": "Opening", "airDate": "2021-02-01"},
		{"tvdbId": 930002, "seasonNumber": 1, "episodeNumber": 2, "absoluteEpisodeNumber": 2, "title": "Middle", "airDate": "2021-02-08"},
		{"tvdbId": 930003, "seasonNumber": 2, "episodeNumber": 1, "absoluteEpisodeNumber": 3, "title": "Finale", "airDate": "2021-02-15"},
	})
	routes[fmt.Sprintf("api.themoviedb.org/3/find/%d", sceneTvdbID)] = `{"tv_results": []}`
	routes["thexem.info/map/havemap"] = fmt.Sprintf(`{"result": "success", "data": ["%d"], "message": ""}`, sceneTvdbID)
	routes["thexem.info/map/all"] = `{"result": "success", "message": "", "data": [
		{"scene": {"season": 1, "episode": 1, "absolute": 1}, "tvdb": {"season": 1, "episode": 1, "absolute": 1}},
		{"scene": {"season": 1, "episode": 2, "absolute": 2}, "tvdb": {"season": 1, "episode": 2, "absolute": 2}},
		{"scene": {"season": 1, "episode": 3, "absolute": 3}, "tvdb": {"season": 2, "episode": 1, "absolute": 3}}]}`
	s := startAcquisition(t, routes, true)
	ctx := t.Context()
	video := s.video("1920x1080", 20)

	shows := t.TempDir()
	indexer := newFakeNewznab(t)
	sab := newFakeSABnzbd(t, t.TempDir())
	s.sabnzbd(indexer, sab)
	lib := s.library("Shows", libraryv1.LibraryKind_LIBRARY_KIND_SHOWS, shows)
	profile := s.profile(release.Series, release.WEBDL1080p, release.HDTV720p, release.WEBDL1080p)
	series := s.addSeries(lib, profile, sceneTvdbID, libraryv1.SeriesType_SERIES_TYPE_STANDARD, 3)
	// The scene mappings task reads XEM's numbering of the wanted series.
	s.sceneMappings()
	if s.served("thexem.info/map/all") == 0 {
		t.Fatalf("XEM's numbering of the series never asked for")
	}
	finale := s.episode(series, 2, 1)

	rel := offered{
		id: "sc03", title: "Scene.Shuffle.S01E03.1080p.WEB-DL.DD5.1.H.264-GRPS", size: 200 << 20, published: time.Now().Add(-48 * time.Hour),
		categories: []int{5000, 5040}, attrs: [][2]string{{"tvdbid", fmt.Sprint(sceneTvdbID)}, {"season", "S01"}, {"episode", "E03"}},
	}
	indexer.offer(rel)
	if _, err := s.wanted.SearchWanted(ctx, acquisitionv1.SearchWantedRequest_builder{ItemIds: []string{finale.GetId()}}.Build()); err != nil {
		t.Fatalf("SearchWanted: %v", err)
	}
	job := s.sabGrabbed(sab, rel, series, "WEBDL-1080p")
	// The indexer was asked for the scene's season and episode.
	if !slices.ContainsFunc(indexer.searched(), func(q url.Values) bool {
		return q.Get("t") == "tvsearch" && q.Get("season") == "1" && q.Get("ep") == "3"
	}) {
		t.Errorf("indexer searches = %v, want = one for season 1, episode 3", indexer.searched())
	}
	s.sabImported(sab, rel, job, video)
	want := filepath.Join(shows, "Scene Shuffle", "Season 2", "Scene Shuffle - S02E01 - Finale WEBDL-1080p.mkv")
	if it := s.item(finale.GetId()); it.GetVirtual() || it.GetPath() != filepath.ToSlash(want) {
		t.Errorf("S02E01 after the import: virtual=%v path=%q, want = %q", it.GetVirtual(), it.GetPath(), filepath.ToSlash(want))
	}
	// TVDB's S01E03 does not exist: no other episode got the file.
	for _, ep := range s.children(series, libraryv1.ItemKind_ITEM_KIND_EPISODE) {
		if ep.GetId() != finale.GetId() && !ep.GetVirtual() {
			t.Errorf("S%02dE%02d has file %q, want = none", ep.GetParentIndexNumber(), ep.GetIndexNumber(), ep.GetPath())
		}
	}
}
