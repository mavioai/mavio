package acquisition

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mavioai/mavio/apps/server/internal/releases"
	"github.com/mavioai/mavio/libs/acquire"
	"github.com/mavioai/mavio/libs/core"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1/pluginv1connect"
	"github.com/mavioai/mavio/libs/release"
)

// fixtureEpisode is an episode of a fixture series.
type fixtureEpisode struct {
	season, episode, absolute int
	title, aired              string
}

// fixtureSeries is a series fakeIdentifier identifies by TVDB ID.
type fixtureSeries struct {
	name     string
	year     int
	status   core.SeriesStatus
	runtime  time.Duration
	episodes []fixtureEpisode
}

var fixtureShows = map[string]fixtureSeries{
	"81189": {"Breaking Bad", 2008, core.SeriesEnded, 47 * time.Minute, []fixtureEpisode{
		{0, 1, 0, "Good Cop Bad Cop", "2009-02-17"},
		{1, 1, 1, "Pilot", "2008-01-20"},
		{1, 2, 2, "Cat's in the Bag", "2008-01-27"},
		{1, 3, 3, "And the Bag's in the River", "2008-02-10"},
		{2, 1, 4, "Seven Thirty-Seven", "2009-03-08"},
		{2, 2, 5, "Grilled", "2009-03-15"},
	}},
	"71256": {"The Daily Show", 1996, core.SeriesContinuing, 22 * time.Minute, []fixtureEpisode{
		{2026, 1, 0, "Guest One", "2026-05-20"},
		{2026, 2, 0, "Guest Two", "2026-05-21"},
	}},
	"267440": {"Attack on Titan", 2013, core.SeriesEnded, 24 * time.Minute, []fixtureEpisode{
		{1, 1, 1, "To You, in 2000 Years", "2013-04-07"},
		{1, 2, 2, "That Day", "2013-04-14"},
		{1, 3, 3, "A Dim Light Amid Despair", "2013-04-21"},
		{2, 1, 26, "Beast Titan", "2017-04-01"},
	}},
}

// identifySeries identifies a series as its fixture and lists its seasons
// and episodes as virtual items, the seasons but the specials monitored,
// as the library's episode listing does.
func identifySeries(ctx context.Context, st core.Store, it core.Item) (core.Item, error) {
	f, ok := fixtureShows[it.ExternalIDs[core.ProviderTVDB]]
	if !ok {
		return core.Item{}, errors.New("unknown series")
	}
	it.Name, it.ProductionYear, it.SeriesStatus, it.Runtime = f.name, f.year, f.status, f.runtime
	if err := st.Items().Upsert(ctx, it); err != nil {
		return core.Item{}, err
	}
	seasons := map[int]core.Item{}
	for _, ep := range f.episodes {
		season, ok := seasons[ep.season]
		if !ok {
			season = core.Item{
				ID: core.NewID(), LibraryID: it.LibraryID, ParentID: it.ID, Kind: core.KindSeason, Name: "Season " + strconv.Itoa(ep.season),
				IndexNumber: new(ep.season), Virtual: true, Monitored: ep.season != 0, DateAdded: it.DateAdded,
			}
			if err := st.Items().Upsert(ctx, season); err != nil {
				return core.Item{}, err
			}
			seasons[ep.season] = season
		}
		aired, _ := time.Parse(time.DateOnly, ep.aired)
		e := core.Item{
			ID: core.NewID(), LibraryID: it.LibraryID, ParentID: season.ID, Kind: core.KindEpisode, Name: ep.title,
			IndexNumber: new(ep.episode), ParentIndexNumber: new(ep.season), PremiereDate: &aired, Virtual: true,
			Monitored: season.Monitored, DateAdded: it.DateAdded, Runtime: f.runtime,
		}
		if ep.absolute > 0 {
			e.AbsoluteNumber = new(ep.absolute)
		}
		if err := st.Items().Upsert(ctx, e); err != nil {
			return core.Item{}, err
		}
	}
	return it, nil
}

// fakeScene is a scene mapping provider listing mappings, and the scene
// numbering of series by TVDB ID.
type fakeScene struct {
	pluginv1connect.SceneMappingProviderServiceClient
	mappings     []*pluginv1.SceneMapping
	numbering    map[string][]*pluginv1.SceneNumbering
	numberingErr error
}

func (f *fakeScene) ListSceneMappings(context.Context, *pluginv1.ListSceneMappingsRequest) (*pluginv1.ListSceneMappingsResponse, error) {
	return pluginv1.ListSceneMappingsResponse_builder{Mappings: f.mappings}.Build(), nil
}

func (f *fakeScene) ListDailySeries(context.Context, *pluginv1.ListDailySeriesRequest) (*pluginv1.ListDailySeriesResponse, error) {
	return &pluginv1.ListDailySeriesResponse{}, nil
}

func (f *fakeScene) GetSceneNumbering(_ context.Context, req *pluginv1.GetSceneNumberingRequest) (*pluginv1.GetSceneNumberingResponse, error) {
	if f.numberingErr != nil {
		return nil, f.numberingErr
	}
	episodes, ok := f.numbering[req.GetSeriesExternalIds()["tvdb"]]
	if !ok {
		return &pluginv1.GetSceneNumberingResponse{}, nil
	}
	return pluginv1.GetSceneNumberingResponse_builder{Found: new(true), Provider: new("tvdb"), Episodes: episodes}.Build(), nil
}

// seriesEnv is an env with a show library and a series quality profile:
// HDTV-720p, WEBDL-720p and WEBDL-1080p, upgraded to the last.
type seriesEnv struct {
	*env
	shows         core.Library
	showsDir      string
	seriesProfile core.ID
	scene         *fakeScene
}

func newSeriesEnv(t *testing.T) *seriesEnv {
	t.Helper()
	e := &seriesEnv{env: newEnv(t), showsDir: t.TempDir(), scene: &fakeScene{}}
	e.rt.SceneProviders = func() []SceneProvider { return []SceneProvider{{ID: "scene", Client: e.scene}} }
	e.shows = core.Library{ID: core.NewID(), Name: "Shows", Kind: core.LibraryShows, Paths: []string{e.showsDir}}
	if err := e.db.Libraries().Create(e.ctx, &e.shows); err != nil {
		t.Fatal(err)
	}
	p := acquire.NewProfile(release.Series, "HD", release.WEBDL1080p, []release.Quality{release.HDTV720p, release.WEBDL720p, release.WEBDL1080p}, nil)
	p.ID, p.UpgradeAllowed = core.NewID(), true
	cp, err := acquire.QualityProfileToCore(p, core.KindSeries)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.db.QualityProfiles().Put(e.ctx, &cp); err != nil {
		t.Fatal(err)
	}
	e.seriesProfile = cp.ID
	return e
}

// addSeries adds a fixture series by TVDB ID, every episode monitored.
func (e *seriesEnv) addSeries(tvdb int, typ core.SeriesType) core.Item {
	e.t.Helper()
	it, _, err := e.rt.AddSeries(e.ctx, NewSeries{
		LibraryID: e.shows.ID, TvdbID: tvdb, QualityProfileID: e.seriesProfile, SeriesType: &typ, SeasonFolder: true,
	})
	if err != nil {
		e.t.Fatal(err)
	}
	return it
}

// episode returns the item of an episode of a series; NilID when it has
// none.
func (e *seriesEnv) episode(seriesID core.ID, season, number int) core.Item {
	e.t.Helper()
	tree, err := e.rt.tree(e.ctx, seriesID)
	if err != nil {
		e.t.Fatal(err)
	}
	for _, it := range tree {
		if it.Kind == core.KindEpisode && *it.ParentIndexNumber == season && *it.IndexNumber == number {
			return it
		}
	}
	return core.Item{}
}

func (e *seriesEnv) season(seriesID core.ID, number int) core.Item {
	e.t.Helper()
	tree, err := e.rt.tree(e.ctx, seriesID)
	if err != nil {
		e.t.Fatal(err)
	}
	for _, it := range tree {
		if it.Kind == core.KindSeason && *it.IndexNumber == number {
			return it
		}
	}
	e.t.Fatalf("no season %d", number)
	return core.Item{}
}

// tvReleases answers TV searches with releases of titles, an hour old,
// of a size each; their hashes are derived from their titles.
func (e *seriesEnv) tvReleases(size int64, titles ...string) {
	var rels []*pluginv1.Release
	for _, t := range titles {
		rels = append(rels, torrent("g-"+t, t, size, hashOf(t), e.clock.Add(-time.Hour)))
	}
	e.indexer.mu.Lock()
	e.indexer.tv = func(*pluginv1.SearchReleasesRequest) []*pluginv1.Release { return rels }
	e.indexer.mu.Unlock()
}

// hashOf returns a torrent info hash for a title.
func hashOf(title string) string {
	h := strings.ToLower(strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, title))
	return (h + strings.Repeat("0", 40))[:40]
}

func (e *seriesEnv) searchEpisodes(seriesID core.ID, ids ...core.ID) {
	e.t.Helper()
	if err := e.rt.SearchEpisodes(e.ctx, EpisodeSearchPayload{Scope: ScopeEpisodes, SeriesID: seriesID, EpisodeIDs: ids, UserInvoked: true}); err != nil {
		e.t.Fatal(err)
	}
}

func (e *seriesEnv) checkQueue() {
	e.t.Helper()
	if err := e.rt.CheckQueue(e.ctx); err != nil {
		e.t.Fatal(err)
	}
}

func ids(items ...core.Item) []core.ID {
	out := make([]core.ID, len(items))
	for i, it := range items {
		out[i] = it.ID
	}
	return out
}

func monitoredEpisodes(t *testing.T, e *seriesEnv, seriesID core.ID) []string {
	t.Helper()
	tree, err := e.rt.tree(e.ctx, seriesID)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, it := range tree {
		if it.Monitored {
			if it.Kind == core.KindSeason {
				out = append(out, "S"+strconv.Itoa(*it.IndexNumber))
			} else {
				out = append(out, slot{*it.ParentIndexNumber, *it.IndexNumber}.String())
			}
		}
	}
	slices.Sort(out)
	return out
}

func TestAddSeries(t *testing.T) {
	e := newSeriesEnv(t)
	it, w, err := e.rt.AddSeries(e.ctx, NewSeries{LibraryID: e.shows.ID, TvdbID: 81189, QualityProfileID: e.seriesProfile, Monitor: MonitorPilot})
	if err != nil {
		t.Fatal(err)
	}
	if it.Name != "Breaking Bad" || !it.Monitored || !it.Virtual || it.SeriesType != core.SeriesStandard {
		t.Errorf("added series = %+v", it)
	}
	if w.MonitorNewItems != core.NewItemsAll || w.QualityProfileID != e.seriesProfile {
		t.Errorf("wanted = %+v, want new items all", w)
	}
	if got, want := monitoredEpisodes(t, e, it.ID), []string{"1x01"}; !slices.Equal(got, want) {
		t.Errorf("monitored with pilot = %v, want = %v", got, want)
	}
	for _, tt := range []struct {
		name string
		n    NewSeries
		want error
	}{
		{"duplicate", NewSeries{LibraryID: e.shows.ID, TvdbID: 81189, QualityProfileID: e.seriesProfile}, core.ErrConflict},
		{"movie library", NewSeries{LibraryID: e.lib.ID, TvdbID: 71256, QualityProfileID: e.seriesProfile}, core.ErrInvalid},
		{"movie profile", NewSeries{LibraryID: e.shows.ID, TvdbID: 71256, QualityProfileID: e.profile}, core.ErrInvalid},
		{"no TVDB ID", NewSeries{LibraryID: e.shows.ID, QualityProfileID: e.seriesProfile}, core.ErrInvalid},
	} {
		if _, _, err := e.rt.AddSeries(e.ctx, tt.n); !errors.Is(err, tt.want) {
			t.Errorf("AddSeries(%s) = %v, want = %v", tt.name, err, tt.want)
		}
	}

	for _, tt := range []struct {
		monitor SeriesMonitor
		want    []string
	}{
		{MonitorAll, []string{"1x01", "1x02", "1x03", "2x01", "2x02", "S1", "S2"}},
		{MonitorLastSeason, []string{"2x01", "2x02", "S2"}},
		{MonitorSpecials, []string{"0x01", "2x01", "2x02", "S0", "S2"}},
		{UnmonitorSpecials, []string{"2x01", "2x02", "S2"}},
		{MonitorFirstSeason, []string{"1x01", "1x02", "1x03", "S1"}},
		{MonitorNone, nil},
	} {
		if err := e.rt.MonitorSeries(e.ctx, []core.ID{it.ID}, tt.monitor, ""); err != nil {
			t.Fatal(err)
		}
		if got := monitoredEpisodes(t, e, it.ID); !slices.Equal(got, tt.want) {
			t.Errorf("monitored with %s = %v, want = %v", tt.monitor, got, tt.want)
		}
	}

	// Seasons take their episodes along.
	if err := e.rt.SetMonitored(e.ctx, []core.ID{e.season(it.ID, 2).ID, e.episode(it.ID, 1, 3).ID}, true); err != nil {
		t.Fatal(err)
	}
	if got, want := monitoredEpisodes(t, e, it.ID), []string{"1x03", "2x01", "2x02", "S2"}; !slices.Equal(got, want) {
		t.Errorf("monitored after SetMonitored = %v, want = %v", got, want)
	}

	// Seasons refreshes add are monitored as the series says.
	if !e.rt.NewSeasonMonitored(e.ctx, it) {
		t.Error("NewSeasonMonitored = false, want = true for new items all")
	}
	if err := e.rt.MonitorSeries(e.ctx, []core.ID{it.ID}, MonitorSkip, core.NewItemsNone); err != nil {
		t.Fatal(err)
	}
	if e.rt.NewSeasonMonitored(e.ctx, it) {
		t.Error("NewSeasonMonitored = true, want = false for new items none")
	}

	// Adding with a search queues a search for the missing episodes.
	daily, _, err := e.rt.AddSeries(e.ctx, NewSeries{LibraryID: e.shows.ID, TvdbID: 71256, QualityProfileID: e.seriesProfile, Search: true})
	if err != nil {
		t.Fatal(err)
	}
	jobs := e.jobs(JobEpisodeSearch)
	if len(jobs) != 1 {
		t.Fatalf("episode search jobs = %d, want = 1", len(jobs))
	}
	var p EpisodeSearchPayload
	if err := json.Unmarshal(jobs[0].Payload, &p); err != nil || p.Scope != ScopeMissing || p.SeriesID != daily.ID {
		t.Errorf("episode search = %+v, %v, want = missing of %s", p, err, daily.ID)
	}
}

func TestDailyFromSceneProviders(t *testing.T) {
	e := newSeriesEnv(t)
	e.scene.mappings = nil
	daily := &fakeDaily{fakeScene: e.scene, tvdb: "71256"}
	e.rt.SceneProviders = func() []SceneProvider { return []SceneProvider{{ID: "scene", Client: daily}} }
	it, _, err := e.rt.AddSeries(e.ctx, NewSeries{LibraryID: e.shows.ID, TvdbID: 71256, QualityProfileID: e.seriesProfile})
	if err != nil {
		t.Fatal(err)
	}
	if it.SeriesType != core.SeriesDaily {
		t.Errorf("series type = %q, want = daily", it.SeriesType)
	}
}

// fakeDaily lists a series as daily.
type fakeDaily struct {
	*fakeScene
	tvdb string
}

func (f *fakeDaily) ListDailySeries(context.Context, *pluginv1.ListDailySeriesRequest) (*pluginv1.ListDailySeriesResponse, error) {
	s := pluginv1.SeriesExternalIds_builder{ExternalIds: map[string]string{"tvdb": f.tvdb}}.Build()
	return pluginv1.ListDailySeriesResponse_builder{Series: []*pluginv1.SeriesExternalIds{s}}.Build(), nil
}

func TestEpisodeSearchGrabsAndImports(t *testing.T) {
	e := newSeriesEnv(t)
	s := e.addSeries(81189, core.SeriesStandard)
	ep1 := e.episode(s.ID, 1, 1)
	const s01e01 = "Breaking.Bad.S01E01.720p.HDTV.x264-GRP"
	e.tvReleases(gb, s01e01, "Breaking.Bad.S01E02.720p.HDTV.x264-GRP")
	e.searchEpisodes(s.ID, ep1.ID)

	if got := e.client.titles(); !slices.Equal(got, []string{s01e01}) {
		t.Fatalf("grabbed = %v, want = [%s]", got, s01e01)
	}
	e.client.mu.Lock()
	kind := e.client.added[0].GetKind()
	e.client.mu.Unlock()
	if kind != pluginv1.DownloadKind_DOWNLOAD_KIND_SERIES {
		t.Errorf("download kind = %v, want = series", kind)
	}
	req := e.indexer.requests[0]
	if req.GetSeason() != 1 || req.GetEpisode() != "1" || req.GetExternalIds()["tvdb"] != "81189" {
		t.Errorf("search = season %d episode %q ids %v, want = S01E01 by TVDB ID", req.GetSeason(), req.GetEpisode(), req.GetExternalIds())
	}
	d := e.download(hashOf(s01e01))
	if d.ItemID != s.ID || !slices.Equal(d.ItemIDs, []core.ID{ep1.ID}) {
		t.Errorf("download items = %s %v, want = %s [%s]", d.ItemID, d.ItemIDs, s.ID, ep1.ID)
	}
	grabs := e.history(core.HistoryGrabbed)
	if len(grabs) != 1 || grabs[0].ItemID != ep1.ID || grabs[0].ParentID != s.ID || grabs[0].Data[dataTvdbID] != "81189" {
		t.Errorf("grab history = %+v, want = one of S01E01 under the series", grabs)
	}
	if w, err := e.db.Wanted().Get(e.ctx, s.ID); err != nil || !w.LastSearchedAt.Equal(e.clock) {
		t.Errorf("last search = %v, %v, want = %v", w.LastSearchedAt, err, e.clock)
	}

	// The download completes: its file goes to the season folder.
	e.completed(hashOf(s01e01), s01e01, s01e01+".mkv", "sample-"+s01e01+".mkv")
	e.checkQueue()
	if d := e.download(hashOf(s01e01)); d.State != core.DownloadImported {
		t.Fatalf("download = %s %v, want = imported", d.State, d.Messages)
	}
	ep1 = e.item(ep1.ID)
	want := filepath.ToSlash(filepath.Join(e.showsDir, "Breaking Bad", "Season 1", "Breaking Bad - S01E01 - Pilot HDTV-720p.mkv"))
	if ep1.Path != want || ep1.Virtual {
		t.Errorf("episode = %q virtual %t, want = %q", ep1.Path, ep1.Virtual, want)
	}
	if _, err := os.Stat(filepath.FromSlash(want)); err != nil {
		t.Error(err)
	}
	if series := e.item(s.ID); series.Virtual || series.Path != filepath.ToSlash(filepath.Join(e.showsDir, "Breaking Bad")) {
		t.Errorf("series = %q virtual %t, want physical in its folder", series.Path, series.Virtual)
	}
	if season := e.season(s.ID, 1); season.Virtual {
		t.Error("season 1 is still virtual")
	}
	imports := e.history(core.HistoryImported)
	if len(imports) != 1 || imports[0].ItemID != ep1.ID || imports[0].ParentID != s.ID || imports[0].Data[dataImportedPath] != want {
		t.Errorf("import history = %+v", imports)
	}
	if len(e.jobs("library.scan")) == 0 {
		t.Error("no library scan queued")
	}

	// A better release upgrades the file, the old one recycled.
	const better = "Breaking.Bad.S01E01.1080p.WEB-DL.DD5.1.H.264-GRP"
	e.tvReleases(2*gb, better)
	e.clock = e.clock.Add(time.Hour)
	e.searchEpisodes(s.ID, ep1.ID)
	if got := e.client.titles(); !slices.Equal(got, []string{s01e01, better}) {
		t.Fatalf("grabbed = %v, want the upgrade too", got)
	}
	e.completed(hashOf(better), better, better+".mkv")
	e.checkQueue()
	if d := e.download(hashOf(better)); d.State != core.DownloadImported {
		t.Fatalf("upgrade download = %s %v, want = imported", d.State, d.Messages)
	}
	ep1 = e.item(ep1.ID)
	if !strings.HasSuffix(ep1.Path, "Breaking Bad - S01E01 - Pilot WEBDL-1080p.mkv") {
		t.Errorf("upgraded episode = %q", ep1.Path)
	}
	if _, err := os.Stat(filepath.FromSlash(want)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("old file stat = %v, want gone", err)
	}
	if _, err := os.Stat(filepath.Join(e.binDir, "Breaking Bad", filepath.Base(want))); err != nil {
		t.Errorf("old file not in the recycle bin: %v", err)
	}
	if deleted := e.history(core.HistoryFileDeleted); len(deleted) != 1 || deleted[0].ItemID != ep1.ID {
		t.Errorf("file deleted history = %+v", deleted)
	}

	// Searching again finds nothing better: the cutoff is met.
	e.clock = e.clock.Add(time.Hour)
	e.searchEpisodes(s.ID, ep1.ID)
	if got := e.client.titles(); len(got) != 2 {
		t.Errorf("grabbed = %v, want no more", got)
	}
}

func TestSeasonSearchPrefersSeasonPack(t *testing.T) {
	e := newSeriesEnv(t)
	s := e.addSeries(81189, core.SeriesStandard)
	const pack = "Breaking.Bad.S01.720p.HDTV.x264-GRP"
	e.tvReleases(3*gb, pack, "Breaking.Bad.S01E02.720p.HDTV.x264-GRP")
	if err := e.rt.SearchEpisodes(e.ctx, EpisodeSearchPayload{Scope: ScopeSeason, SeriesID: s.ID, SeasonNumber: 1}); err != nil {
		t.Fatal(err)
	}
	if got := e.client.titles(); !slices.Equal(got, []string{pack}) {
		t.Fatalf("grabbed = %v, want = [%s]", got, pack)
	}
	if add := e.client.added[0]; !add.GetFullSeason() || add.GetDetails().GetTitleSlug() != "breaking-bad" || add.GetDetails().GetCollection() != "" {
		t.Errorf("full season, details = %v, %v, want = a full season of breaking-bad", add.GetFullSeason(), add.GetDetails())
	}
	eps := []core.Item{e.episode(s.ID, 1, 1), e.episode(s.ID, 1, 2), e.episode(s.ID, 1, 3)}
	d := e.download(hashOf(pack))
	if got := slices.Sorted(slices.Values(idStrings(d.ItemIDs))); !slices.Equal(got, slices.Sorted(slices.Values(idStrings(ids(eps...))))) {
		t.Errorf("download episodes = %v, want season 1's", d.ItemIDs)
	}
	if req := e.indexer.requests[0]; req.GetSeason() != 1 || req.GetEpisode() != "" {
		t.Errorf("search = season %d episode %q, want = season 1", req.GetSeason(), req.GetEpisode())
	}

	// The pack's files go to their episodes.
	e.completed(hashOf(pack), pack,
		"Breaking.Bad.S01E01.720p.HDTV.x264-GRP.mkv", "Breaking.Bad.S01E02.720p.HDTV.x264-GRP.mkv", "Breaking.Bad.S01E03.720p.HDTV.x264-GRP.mkv")
	e.checkQueue()
	if d := e.download(hashOf(pack)); d.State != core.DownloadImported {
		t.Fatalf("download = %s %v, want = imported", d.State, d.Messages)
	}
	for _, ep := range eps {
		if it := e.item(ep.ID); it.Virtual || !strings.Contains(it.Path, "S01E0"+strconv.Itoa(*it.IndexNumber)) {
			t.Errorf("episode %d = %q virtual %t", *it.IndexNumber, it.Path, it.Virtual)
		}
	}
	if n := len(e.history(core.HistoryImported)); n != 3 {
		t.Errorf("imports = %d, want = 3", n)
	}
}

func TestSeasonPackMissingEpisodeBlocksImport(t *testing.T) {
	e := newSeriesEnv(t)
	s := e.addSeries(81189, core.SeriesStandard)
	const pack = "Breaking.Bad.S01.720p.HDTV.x264-GRP"
	e.tvReleases(3*gb, pack)
	if err := e.rt.SearchEpisodes(e.ctx, EpisodeSearchPayload{Scope: ScopeSeason, SeriesID: s.ID, SeasonNumber: 1}); err != nil {
		t.Fatal(err)
	}
	e.completed(hashOf(pack), pack, "Breaking.Bad.S01E01.720p.HDTV.x264-GRP.mkv", "Breaking.Bad.S01E02.720p.HDTV.x264-GRP.mkv")
	e.checkQueue()
	d := e.download(hashOf(pack))
	if d.State != core.DownloadImportBlocked || len(d.Messages) == 0 || d.Messages[0] != msgNotAllEpisodesImported {
		t.Errorf("download = %s %v, want = blocked as not all imported", d.State, d.Messages)
	}
	if it := e.episode(s.ID, 1, 2); it.Virtual {
		t.Error("S01E02 was not imported")
	}
}

func idStrings(list []core.ID) []string {
	out := make([]string, len(list))
	for i, id := range list {
		out[i] = id.String()
	}
	return out
}

func TestMultiEpisodeFile(t *testing.T) {
	e := newSeriesEnv(t)
	s := e.addSeries(81189, core.SeriesStandard)
	ep2, ep3 := e.episode(s.ID, 1, 2), e.episode(s.ID, 1, 3)
	const title = "Breaking.Bad.S01E02E03.720p.HDTV.x264-GRP"
	rel := FromSearch(releasesRelease(torrent("g-multi", title, 2*gb, hashOf(title), e.clock.Add(-time.Hour))))
	d, err := e.rt.GrabRelease(e.ctx, ep2.ID, rel, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(d.ItemIDs, []core.ID{ep2.ID, ep3.ID}) {
		t.Fatalf("download episodes = %v, want = [%s %s]", d.ItemIDs, ep2.ID, ep3.ID)
	}
	e.completed(hashOf(title), title, title+".mkv")
	e.checkQueue()
	if d := e.download(hashOf(title)); d.State != core.DownloadImported {
		t.Fatalf("download = %s %v, want = imported", d.State, d.Messages)
	}
	it := e.item(ep2.ID)
	if it.Virtual || it.IndexNumberEnd == nil || *it.IndexNumberEnd != 3 || !strings.HasSuffix(it.Path, "Breaking Bad - S01E02-E03 - Cat's in the Bag + And the Bag's in the River HDTV-720p.mkv") {
		t.Errorf("episode item = %q end %v virtual %t", it.Path, it.IndexNumberEnd, it.Virtual)
	}
	if left := e.episode(s.ID, 1, 3); !left.ID.IsZero() {
		t.Errorf("S01E03 still has its own item %+v", left)
	}
	// The file holds both episodes: S01E03 is no longer missing.
	missing, _, err := e.rt.ListWantedEpisodes(e.ctx, WantedMissing, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, m := range missing {
		got = append(got, slot{m.Season, m.Episode}.String())
	}
	if want := []string{"2x02", "2x01", "1x01"}; !slices.Equal(got, want) {
		t.Errorf("missing = %v, want = %v", got, want)
	}
}

func TestAnimeAbsoluteNumbers(t *testing.T) {
	e := newSeriesEnv(t)
	s := e.addSeries(267440, core.SeriesAnime)
	ep2 := e.episode(s.ID, 1, 2)
	const title = "[SubsPlease] Attack on Titan - 02 (720p) [ABCD1234]"
	e.tvReleases(gb/2, "[SubsPlease] Attack on Titan - 03 (720p) [ABCD1235]", title)
	e.searchEpisodes(s.ID, ep2.ID)
	if got := e.client.titles(); !slices.Equal(got, []string{title}) {
		t.Fatalf("grabbed = %v, want = [%s]", got, title)
	}
	if d := e.download(hashOf(title)); !slices.Equal(d.ItemIDs, []core.ID{ep2.ID}) {
		t.Errorf("download episodes = %v, want = [%s]", d.ItemIDs, ep2.ID)
	}
	if q := e.indexer.requests[0].GetQuery(); q != "02" {
		t.Errorf("search query = %q, want = the absolute number", q)
	}
}

func TestDailyAirDates(t *testing.T) {
	e := newSeriesEnv(t)
	s := e.addSeries(71256, core.SeriesDaily)
	ep2 := e.episode(s.ID, 2026, 2)
	const title = "The.Daily.Show.2026.05.21.Guest.Two.720p.WEB.h264-GRP"
	e.tvReleases(gb/2, "The.Daily.Show.2026.05.20.Guest.One.720p.WEB.h264-GRP", title)
	e.searchEpisodes(s.ID, ep2.ID)
	if got := e.client.titles(); !slices.Equal(got, []string{title}) {
		t.Fatalf("grabbed = %v, want = [%s]", got, title)
	}
	if d := e.download(hashOf(title)); !slices.Equal(d.ItemIDs, []core.ID{ep2.ID}) {
		t.Errorf("download episodes = %v, want = [%s]", d.ItemIDs, ep2.ID)
	}
	if req := e.indexer.requests[0]; req.GetSeason() != 2026 || req.GetEpisode() != "05/21" {
		t.Errorf("search = season %d episode %q, want = 2026 05/21", req.GetSeason(), req.GetEpisode())
	}
}

func TestRSSMatchesSceneTitles(t *testing.T) {
	e := newSeriesEnv(t)
	e.scene.mappings = []*pluginv1.SceneMapping{pluginv1.SceneMapping_builder{
		Source: new("services"), SeriesExternalIds: map[string]string{"tvdb": "267440"}, Title: new("Shingeki no Kyojin"),
	}.Build()}
	s := e.addSeries(267440, core.SeriesAnime)
	if err := e.rt.UpdateSceneMappings(e.ctx); err != nil {
		t.Fatal(err)
	}
	const title = "[Group] Shingeki no Kyojin - 03 [720p]"
	e.indexer.set(nil, []*pluginv1.Release{
		torrent("g-rss", title, gb/2, hashOf(title), e.clock.Add(-time.Hour)),
		torrent("g-other", "Unknown.Show.S01E01.720p.HDTV.x264-GRP", gb, hashOf("other"), e.clock.Add(-time.Hour)),
	})
	if err := e.rt.SyncRSS(e.ctx); err != nil {
		t.Fatal(err)
	}
	if got := e.client.titles(); !slices.Equal(got, []string{title}) {
		t.Fatalf("grabbed = %v, want = [%s]", got, title)
	}
	if d := e.download(hashOf(title)); !slices.Equal(d.ItemIDs, []core.ID{e.episode(s.ID, 1, 3).ID}) {
		t.Errorf("download episodes = %v, want S01E03", d.ItemIDs)
	}
}

func TestFailedEpisodeDownloadBlocklistsAndSearches(t *testing.T) {
	e := newSeriesEnv(t)
	s := e.addSeries(81189, core.SeriesStandard)
	ep1 := e.episode(s.ID, 1, 1)
	const title = "Breaking.Bad.S01E01.720p.HDTV.x264-GRP"
	e.tvReleases(gb, title)
	e.searchEpisodes(s.ID, ep1.ID)
	e.client.setDownloads(pluginv1.Download_builder{
		ClientId: new("qb"), Id: new(hashOf(title)), Title: new(title), Status: new(pluginv1.DownloadStatus_DOWNLOAD_STATUS_FAILED),
		Message: new("unpack failed"), CanBeRemoved: new(true),
	}.Build())
	e.checkQueue()
	if d := e.download(hashOf(title)); d.State != core.DownloadFailed {
		t.Errorf("download = %s, want = failed", d.State)
	}
	entries := e.blocklist(s.ID)
	if len(entries) != 1 || entries[0].ItemID != s.ID || !slices.Equal(entries[0].ItemIDs, []core.ID{ep1.ID}) || entries[0].SourceTitle != title {
		t.Fatalf("blocklist = %+v, want the release for S01E01", entries)
	}
	if failed := e.history(core.HistoryDownloadFailed); len(failed) != 1 || failed[0].ItemID != ep1.ID || failed[0].ParentID != s.ID {
		t.Errorf("failed history = %+v", failed)
	}
	jobs := e.jobs(JobEpisodeSearch)
	if len(jobs) != 1 {
		t.Fatalf("episode search jobs = %d, want = 1", len(jobs))
	}
	var p EpisodeSearchPayload
	if err := json.Unmarshal(jobs[0].Payload, &p); err != nil || p.Scope != ScopeEpisodes || !slices.Equal(p.EpisodeIDs, []core.ID{ep1.ID}) {
		t.Errorf("re-search = %+v, %v, want S01E01", p, err)
	}
	// The blocklisted release is not grabbed again.
	e.clock = e.clock.Add(time.Hour)
	if err := e.rt.SearchEpisodes(e.ctx, p); err != nil {
		t.Fatal(err)
	}
	if got := e.client.titles(); len(got) != 1 {
		t.Errorf("grabbed = %v, want the blocklisted release once", got)
	}
}

func TestWantedEpisodes(t *testing.T) {
	e := newSeriesEnv(t)
	s := e.addSeries(81189, core.SeriesStandard)
	missing, total, err := e.rt.ListWantedEpisodes(e.ctx, WantedMissing, 2, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 5 || len(missing) != 2 || missing[0].Season != 2 || missing[0].Episode != 2 || missing[0].Series.ID != s.ID {
		t.Errorf("missing = %+v (%d), want 5, most recent first", missing, total)
	}
	n, err := e.rt.SearchWanted(e.ctx, WantedKindEpisode, nil, WantedMissing)
	if err != nil || n != 1 {
		t.Errorf("SearchWanted(episodes) = %d, %v, want = 1", n, err)
	}
	n, err = e.rt.SearchWanted(e.ctx, WantedKindEpisode, []core.ID{s.ID, e.season(s.ID, 2).ID, e.episode(s.ID, 1, 1).ID, e.episode(s.ID, 1, 2).ID}, WantedMissing)
	if err != nil || n != 3 {
		t.Errorf("SearchWanted(items) = %d, %v, want = 3", n, err)
	}
	var scopes []string
	for _, j := range e.jobs(JobEpisodeSearch) {
		var p EpisodeSearchPayload
		if err := json.Unmarshal(j.Payload, &p); err != nil {
			t.Fatal(err)
		}
		scopes = append(scopes, string(p.Scope)+":"+strconv.Itoa(len(p.EpisodeIDs)))
	}
	slices.Sort(scopes)
	if want := []string{"episodes:2", "missing:0", "season:0", "series:0"}; !slices.Equal(scopes, want) {
		t.Errorf("searches = %v, want = %v", scopes, want)
	}

	// Searching for the missing episodes searches by season.
	e.tvReleases(gb)
	if err := e.rt.SearchEpisodes(e.ctx, EpisodeSearchPayload{Scope: ScopeMissing}); err != nil {
		t.Fatal(err)
	}
	var seasons []int32
	for _, r := range e.indexer.requests {
		if len(r.GetExternalIds()) > 0 && r.GetEpisode() == "" {
			seasons = append(seasons, r.GetSeason())
		}
	}
	slices.Sort(seasons)
	if !slices.Equal(seasons, []int32{1, 2}) {
		t.Errorf("season searches = %v, want = [1 2]", seasons)
	}
}

func TestDecideEpisodes(t *testing.T) {
	e := newSeriesEnv(t)
	s := e.addSeries(81189, core.SeriesStandard)
	ep1 := e.episode(s.ID, 1, 1)
	found := []*pluginv1.Release{
		torrent("a", "Breaking.Bad.S01E01.720p.HDTV.x264-GRP", gb, hashOf("a"), e.clock.Add(-time.Hour)),
		torrent("b", "Breaking.Bad.S01E02.720p.HDTV.x264-GRP", gb, hashOf("b"), e.clock.Add(-time.Hour)),
	}
	var rels []releases.Release
	for _, r := range found {
		rels = append(rels, releasesRelease(r))
	}
	decided, err := e.rt.Decide(e.ctx, ep1.ID, rels)
	if err != nil {
		t.Fatal(err)
	}
	if len(decided) != 2 || len(decided[0].Rejections) != 0 || !slices.Equal(decided[0].EpisodeIDs, []core.ID{ep1.ID}) || len(decided[1].Rejections) == 0 {
		t.Errorf("decided = %+v, want S01E01 approved first, S01E02 rejected", decided)
	}
}

// releasesRelease returns a release as the indexer's search found it.
func releasesRelease(r *pluginv1.Release) releases.Release {
	r.SetIndexerId("ix")
	return releases.Release{Provider: "idx", IndexerName: "Indexer", Priority: 25, Release: r}
}
