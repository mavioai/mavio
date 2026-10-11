package acquisition

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mavioai/mavio/apps/server/internal/downloads"
	"github.com/mavioai/mavio/apps/server/internal/releases"
	"github.com/mavioai/mavio/libs/acquire"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1/pluginv1connect"
	"github.com/mavioai/mavio/libs/release"
	"github.com/mavioai/mavio/libs/store"
)

// The tests run on an injected clock rather than in a synctest bubble:
// the store's database driver runs goroutines of its own.

const (
	hash1080  = "1080aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	hash720   = "720bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	title1080 = "The.Matrix.1999.1080p.BluRay.x264-GROUP"
	title720  = "The.Matrix.1999.720p.BluRay.x264-OTHER"
	gb        = int64(1) << 30
)

// fakeIndexer is an indexer provider with one indexer, which answers
// searches for an item with search and RSS reads with rss.
type fakeIndexer struct {
	pluginv1connect.IndexerProviderServiceClient

	mu          sync.Mutex
	search, rss []*pluginv1.Release
	// tv answers TV searches when set; requests are those it was asked.
	tv       func(req *pluginv1.SearchReleasesRequest) []*pluginv1.Release
	requests []*pluginv1.SearchReleasesRequest
	// settings, when set, changes the indexer's settings.
	settings func(b *pluginv1.Indexer_builder)
}

func (f *fakeIndexer) set(search, rss []*pluginv1.Release) {
	f.mu.Lock()
	f.search, f.rss = search, rss
	f.mu.Unlock()
}

func (f *fakeIndexer) ListIndexers(context.Context, *pluginv1.ListIndexersRequest) (*pluginv1.ListIndexersResponse, error) {
	caps := pluginv1.IndexerCapabilities_builder{
		SearchParams: []string{"q"}, MovieSearchParams: []string{"q", "imdbid", "tmdbid"}, TvSearchParams: []string{"q", "season", "ep", "tvdbid"},
	}.Build()
	b := pluginv1.Indexer_builder{Id: new("ix"), Name: new("Indexer"), Enabled: new(true), Priority: new(int32(25)), Capabilities: caps}
	if f.settings != nil {
		f.settings(&b)
	}
	ix := b.Build()
	return pluginv1.ListIndexersResponse_builder{Indexers: []*pluginv1.Indexer{ix}}.Build(), nil
}

func (f *fakeIndexer) SearchReleases(_ context.Context, req *pluginv1.SearchReleasesRequest) (*pluginv1.SearchReleasesResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, req)
	found := f.rss
	switch {
	case f.tv != nil && (len(req.GetExternalIds()) > 0 || req.GetQuery() != ""):
		found = f.tv(req)
	case len(req.GetExternalIds()) > 0 || req.GetQuery() != "":
		found = f.search
	}
	return pluginv1.SearchReleasesResponse_builder{Releases: found}.Build(), nil
}

func (f *fakeIndexer) GetReleaseFile(context.Context, *pluginv1.GetReleaseFileRequest) (*pluginv1.GetReleaseFileResponse, error) {
	return pluginv1.GetReleaseFileResponse_builder{Data: []byte("torrent"), FileName: new("release.torrent")}.Build(), nil
}

func torrent(guid, title string, size int64, hash string, published time.Time) *pluginv1.Release {
	return pluginv1.Release_builder{
		Guid: new(guid), Title: new(title), Size: new(size), DownloadUrl: new("https://indexer.test/" + guid),
		MagnetUrl: new("magnet:?xt=urn:btih:" + hash), InfoHash: new(hash), Protocol: new(pluginv1.DownloadProtocol_DOWNLOAD_PROTOCOL_TORRENT),
		PublishDate: timestamppb.New(published), Seeders: new(int32(50)), Peers: new(int32(60)), Categories: []int32{2000},
	}.Build()
}

// fakeClient is a download client provider with one torrent client on
// host "seedbox", which lists downloads and records what it is asked.
type fakeClient struct {
	pluginv1connect.DownloadClientProviderServiceClient

	mu                sync.Mutex
	downloads         []*pluginv1.Download
	added             []*pluginv1.AddDownloadRequest
	removed, imported []string
}

func (f *fakeClient) ListDownloadClients(context.Context, *pluginv1.ListDownloadClientsRequest) (*pluginv1.ListDownloadClientsResponse, error) {
	c := pluginv1.DownloadClient_builder{
		Id: new("qb"), Name: new("qBittorrent"), Protocol: new(pluginv1.DownloadProtocol_DOWNLOAD_PROTOCOL_TORRENT), Enabled: new(true),
		Priority: new(int32(1)), Host: new("seedbox"), RemoveCompletedDownloads: new(true), RemoveFailedDownloads: new(true),
	}.Build()
	return pluginv1.ListDownloadClientsResponse_builder{Clients: []*pluginv1.DownloadClient{c}}.Build(), nil
}

func (f *fakeClient) AddDownload(_ context.Context, req *pluginv1.AddDownloadRequest) (*pluginv1.AddDownloadResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.added = append(f.added, req)
	return pluginv1.AddDownloadResponse_builder{DownloadId: new(req.GetInfoHash())}.Build(), nil
}

func (f *fakeClient) ListDownloads(context.Context, *pluginv1.ListDownloadsRequest) (*pluginv1.ListDownloadsResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return pluginv1.ListDownloadsResponse_builder{Downloads: f.downloads}.Build(), nil
}

func (f *fakeClient) RemoveDownload(_ context.Context, req *pluginv1.RemoveDownloadRequest) (*pluginv1.RemoveDownloadResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.removed = append(f.removed, req.GetDownloadId())
	return &pluginv1.RemoveDownloadResponse{}, nil
}

func (f *fakeClient) MarkDownloadImported(_ context.Context, req *pluginv1.MarkDownloadImportedRequest) (*pluginv1.MarkDownloadImportedResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.imported = append(f.imported, req.GetDownloadId())
	return &pluginv1.MarkDownloadImportedResponse{}, nil
}

func (f *fakeClient) setDownloads(d ...*pluginv1.Download) {
	f.mu.Lock()
	f.downloads = d
	f.mu.Unlock()
}

func (f *fakeClient) titles() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, a := range f.added {
		out = append(out, a.GetTitle())
	}
	return out
}

// fakeProber reads files as two hours long with audio; samples last 30
// seconds.
type fakeProber struct{}

func (fakeProber) Probe(_ context.Context, path string, _ bool) (library.ProbeResult, error) {
	d := 2 * time.Hour
	if strings.Contains(strings.ToLower(filepath.Base(path)), "sample") {
		d = 30 * time.Second
	}
	return library.ProbeResult{Source: core.MediaSource{Duration: d, Streams: []core.MediaStream{{Kind: core.StreamVideo}, {Kind: core.StreamAudio}}}}, nil
}

// fakeIdentifier identifies every movie as The Matrix.
type fakeIdentifier struct{ st core.Store }

func (f fakeIdentifier) Identify(ctx context.Context, _ core.Library, id core.ID, ids map[core.Provider]string) (core.Item, error) {
	it, err := f.st.Items().Get(ctx, id)
	if err != nil {
		return core.Item{}, err
	}
	if it.Kind == core.KindSeries {
		return identifySeries(ctx, f.st, it)
	}
	matrix(&it)
	return it, f.st.Items().Upsert(ctx, it)
}

func matrix(it *core.Item) {
	premiere := time.Date(1999, 3, 31, 0, 0, 0, 0, time.UTC)
	it.Name, it.ProductionYear, it.PremiereDate, it.Runtime = "The Matrix", 1999, &premiere, 136*time.Minute
	it.ExternalIDs = map[core.Provider]string{core.ProviderTMDB: "603", core.ProviderIMDb: "tt0133093"}
}

// env is a runtime over a store, an indexer and a download client, with
// a library, a download folder and a recycle bin in temporary folders.
type env struct {
	t        *testing.T
	ctx      context.Context
	db       *store.Store
	rt       *Runtime
	clock    time.Time
	settings core.AcquisitionSettings
	indexer  *fakeIndexer
	client   *fakeClient
	lib      core.Library
	profile  core.ID
	libDir   string
	dlDir    string
	binDir   string
}

func newEnv(t *testing.T) *env {
	t.Helper()
	ctx := t.Context()
	db, err := store.Open(ctx, "sqlite:"+filepath.Join(t.TempDir(), "mavio.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	e := &env{
		t: t, ctx: ctx, db: db, clock: time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC),
		indexer: &fakeIndexer{}, client: &fakeClient{},
		libDir: t.TempDir(), dlDir: t.TempDir(), binDir: filepath.Join(t.TempDir(), "bin"),
	}
	e.settings = core.DefaultAcquisitionSettings()
	e.settings.RecycleBin = e.binDir
	e.settings.Naming.RenameMovies = true
	e.settings.ImportExtraFiles = true
	e.rt = &Runtime{
		Store:      db,
		Settings:   func() core.AcquisitionSettings { return e.settings },
		Indexers:   func() []releases.Provider { return []releases.Provider{{ID: "idx", Client: e.indexer}} },
		Clients:    func() []downloads.Provider { return []downloads.Provider{{ID: "dl", Client: e.client}} },
		Identifier: fakeIdentifier{db},
		Prober:     fakeProber{},
		Now:        func() time.Time { return e.clock },
	}
	e.lib = core.Library{ID: core.NewID(), Name: "Movies", Kind: core.LibraryMovies, Paths: []string{e.libDir}}
	if err := db.Libraries().Create(ctx, &e.lib); err != nil {
		t.Fatal(err)
	}
	p := acquire.NewProfile(release.Movie, "HD", release.Bluray1080p, []release.Quality{release.Bluray720p, release.Bluray1080p}, nil)
	p.ID, p.UpgradeAllowed = core.NewID(), true
	cp, err := acquire.QualityProfileToCore(p, core.KindMovie)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.QualityProfiles().Put(ctx, &cp); err != nil {
		t.Fatal(err)
	}
	e.profile = cp.ID
	return e
}

// addMovie adds The Matrix, wanted without searching.
func (e *env) addMovie() core.Item {
	e.t.Helper()
	it, _, err := e.rt.AddMovie(e.ctx, NewMovie{LibraryID: e.lib.ID, TmdbID: 603, QualityProfileID: e.profile})
	if err != nil {
		e.t.Fatal(err)
	}
	return it
}

func (e *env) item(id core.ID) core.Item {
	e.t.Helper()
	it, err := e.db.Items().Get(e.ctx, id)
	if err != nil {
		e.t.Fatal(err)
	}
	return it
}

func (e *env) jobs(kind string) []core.Job {
	e.t.Helper()
	page, err := e.db.Jobs().List(e.ctx, core.JobQuery{Kinds: []string{kind}})
	if err != nil {
		e.t.Fatal(err)
	}
	return page.Items
}

func (e *env) history(types ...core.HistoryEventType) []core.HistoryEvent {
	e.t.Helper()
	page, err := e.db.History().List(e.ctx, core.HistoryQuery{Types: types})
	if err != nil {
		e.t.Fatal(err)
	}
	return page.Items
}

func (e *env) downloads() []core.Download {
	e.t.Helper()
	page, err := e.db.Downloads().List(e.ctx, core.DownloadQuery{})
	if err != nil {
		e.t.Fatal(err)
	}
	return page.Items
}

func (e *env) download(downloadID string) core.Download {
	e.t.Helper()
	d, err := e.db.Downloads().GetByDownloadID(e.ctx, "dl", "qb", downloadID)
	if err != nil {
		e.t.Fatal(err)
	}
	return d
}

func (e *env) blocklist(itemID core.ID) []core.BlocklistEntry {
	e.t.Helper()
	page, err := e.db.Blocklist().List(e.ctx, itemID, core.MaxPageSize, 0)
	if err != nil {
		e.t.Fatal(err)
	}
	return page.Items
}

// both returns the 1080p and 720p releases, published an hour ago.
func (e *env) both() []*pluginv1.Release {
	return []*pluginv1.Release{
		torrent("g720", title720, 4*gb, hash720, e.clock.Add(-time.Hour)),
		torrent("g1080", title1080, 8*gb, hash1080, e.clock.Add(-time.Hour)),
	}
}

// completed writes a download's files to the download folder, as the
// client's host "seedbox" sees them in /downloads, and lists it as
// completed; a remote path mapping maps the folder.
func (e *env) completed(hash, title string, files ...string) string {
	e.t.Helper()
	dir := filepath.Join(e.dlDir, title)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		e.t.Fatal(err)
	}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("content of "+f), 0o644); err != nil {
			e.t.Fatal(err)
		}
	}
	m := core.RemotePathMapping{ID: core.NewID(), Host: "seedbox", RemotePath: "/downloads/", LocalPath: e.dlDir + string(filepath.Separator)}
	if err := e.db.RemotePathMappings().Put(e.ctx, &m); err != nil && !errors.Is(err, core.ErrConflict) {
		e.t.Fatal(err)
	}
	e.client.setDownloads(pluginv1.Download_builder{
		ClientId: new("qb"), Id: new(hash), Title: new(title), Status: new(pluginv1.DownloadStatus_DOWNLOAD_STATUS_COMPLETED),
		OutputPath: new("/downloads/" + title), CanBeRemoved: new(true),
	}.Build())
	return dir
}

func TestAddMovie(t *testing.T) {
	e := newEnv(t)
	it, w, err := e.rt.AddMovie(e.ctx, NewMovie{LibraryID: e.lib.ID, TmdbID: 603, QualityProfileID: e.profile, Search: true})
	if err != nil {
		t.Fatal(err)
	}
	got := e.item(it.ID)
	if got.Name != "The Matrix" || !got.Virtual || !got.Monitored || got.ExternalIDs[core.ProviderTMDB] != "603" {
		t.Errorf("item = %q virtual %v monitored %v ids %v, want = identified virtual monitored The Matrix", got.Name, got.Virtual, got.Monitored, got.ExternalIDs)
	}
	if w.MinimumAvailability != core.AvailabilityReleased || w.QualityProfileID != e.profile || !w.AddedAt.Equal(e.clock) {
		t.Errorf("wanted = %+v, want = released with the profile, added now", w)
	}
	if jobs := e.jobs(JobSearch); len(jobs) != 1 || !strings.Contains(string(jobs[0].Payload), it.ID.String()) {
		t.Errorf("search jobs = %v, want = one for the movie", jobs)
	}

	shows := core.Library{ID: core.NewID(), Name: "Shows", Kind: core.LibraryShows, Paths: []string{t.TempDir()}}
	if err := e.db.Libraries().Create(e.ctx, &shows); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		n    NewMovie
		want error
	}{
		{"duplicate", NewMovie{LibraryID: e.lib.ID, TmdbID: 603, QualityProfileID: e.profile}, core.ErrConflict},
		{"show library", NewMovie{LibraryID: shows.ID, TmdbID: 604, QualityProfileID: e.profile}, core.ErrInvalid},
		{"no profile", NewMovie{LibraryID: e.lib.ID, TmdbID: 604, QualityProfileID: core.NewID()}, core.ErrInvalid},
		{"bad TMDB ID", NewMovie{LibraryID: e.lib.ID, QualityProfileID: e.profile}, core.ErrInvalid},
		{"no library", NewMovie{LibraryID: core.NewID(), TmdbID: 604, QualityProfileID: e.profile}, core.ErrNotFound},
	} {
		if _, _, err := e.rt.AddMovie(e.ctx, tc.n); !errors.Is(err, tc.want) {
			t.Errorf("%s: AddMovie() err = %v, want = %v", tc.name, err, tc.want)
		}
	}

	// Wanted lists: the movie is missing and available.
	list, total, err := e.rt.ListWanted(e.ctx, WantedMissing, 0, 0)
	if err != nil || total != 1 || len(list) != 1 || list[0].Item.ID != it.ID || !list[0].Available {
		t.Errorf("ListWanted(missing) = %+v, %d, %v, want = the movie, available", list, total, err)
	}
	if list, total, _ := e.rt.ListWanted(e.ctx, WantedCutoffUnmet, 0, 0); total != 0 || len(list) != 0 {
		t.Errorf("ListWanted(cutoff unmet) = %d movies, want = none", total)
	}
	u, err := e.rt.UpdateWanted(e.ctx, WantedItem{Wanted: core.Wanted{ItemID: it.ID, QualityProfileID: e.profile, Tags: []string{"4k"}}})
	if err != nil || u.Monitored || !slices.Equal(u.Tags, []string{"4k"}) || e.item(it.ID).Monitored {
		t.Errorf("UpdateWanted(unmonitored) = %+v, %v, want = unmonitored with tags", u, err)
	}
	if n, err := e.rt.SearchWanted(e.ctx, WantedKindMovie, nil, WantedMissing); err != nil || n != 0 {
		t.Errorf("SearchWanted(unmonitored) = %d, %v, want = 0", n, err)
	}
}

func TestSearchGrabsBest(t *testing.T) {
	e := newEnv(t)
	it := e.addMovie()
	e.indexer.set(e.both(), nil)
	if err := e.rt.SearchMovie(e.ctx, it.ID, false); err != nil {
		t.Fatal(err)
	}
	if got := e.client.titles(); !slices.Equal(got, []string{title1080}) {
		t.Fatalf("grabbed = %v, want = [%s]", got, title1080)
	}
	if add := e.client.added[0]; add.GetMagnetUrl() == "" || add.GetInfoHash() != hash1080 || add.GetKind() != pluginv1.DownloadKind_DOWNLOAD_KIND_MOVIE {
		t.Errorf("AddDownload = %v, want = the magnet of the movie release", add)
	}
	d := e.download(hash1080)
	if d.State != core.DownloadDownloading || d.ItemID != it.ID || d.Protocol != core.ProtocolTorrent || d.Indexer != "Indexer" ||
		d.Quality.Quality != acquire.QualityID(release.Bluray1080p) || !d.GrabbedAt.Equal(e.clock) {
		t.Errorf("download = %+v, want = downloading Bluray-1080p torrent of the movie", d)
	}
	grabs := e.history(core.HistoryGrabbed)
	if len(grabs) != 1 {
		t.Fatalf("grab events = %d, want = 1", len(grabs))
	}
	for k, want := range map[string]string{
		dataDownloadClient: "dl/qb", dataDownloadClientName: "qBittorrent", dataIndexer: "Indexer", dataIndexerID: "idx/ix",
		dataProtocol: "torrent", dataTorrentInfoHash: hash1080, dataReleaseGroup: "GROUP", dataReleaseSource: "Search", dataTmdbID: "603",
	} {
		if got := grabs[0].Data[k]; got != want {
			t.Errorf("grab data %s = %q, want = %q", k, got, want)
		}
	}
	w, err := e.db.Wanted().Get(e.ctx, it.ID)
	if err != nil || !w.LastSearchedAt.Equal(e.clock) {
		t.Errorf("LastSearchedAt = %v, %v, want = %v", w.LastSearchedAt, err, e.clock)
	}

	// The download is in the queue: a second search grabs nothing.
	if err := e.rt.SearchMovie(e.ctx, it.ID, false); err != nil {
		t.Fatal(err)
	}
	if got := len(e.client.titles()); got != 1 {
		t.Errorf("grabs after a second search = %d, want = 1", got)
	}

	// Interactive decisions list the queued release as rejected.
	found, err := e.rt.searcher().Search(e.ctx, releases.Search{ItemID: it.ID})
	if err != nil {
		t.Fatal(err)
	}
	decided, err := e.rt.Decide(e.ctx, it.ID, found.Releases)
	if err != nil || len(decided) != 2 {
		t.Fatalf("Decide() = %d, %v, want = 2", len(decided), err)
	}
	for _, d := range decided {
		if len(d.Rejections) == 0 {
			t.Errorf("Decide(%s) approved, want = rejected while one is queued", d.Release.GetTitle())
		}
	}
	if _, err := e.rt.Decide(e.ctx, core.NewID(), found.Releases); !errors.Is(err, ErrNotWanted) {
		t.Errorf("Decide(unknown) err = %v, want = ErrNotWanted", err)
	}
}

// TestGrabSeedSettings checks grabs seed as the indexer's settings say,
// as Radarr's and Sonarr's SeedConfigProvider, and the decisions read the
// indexer's settings.
func TestGrabSeedSettings(t *testing.T) {
	e := newEnv(t)
	it := e.addMovie()
	e.indexer.settings = func(b *pluginv1.Indexer_builder) {
		b.SeedRatio = new(2.5)
		b.SeedTime = durationpb.New(90 * time.Minute)
		b.SeasonPackSeedTime = durationpb.New(3 * time.Hour)
	}
	e.indexer.set(e.both(), nil)
	if err := e.rt.SearchMovie(e.ctx, it.ID, false); err != nil {
		t.Fatal(err)
	}
	if len(e.client.added) != 1 {
		t.Fatalf("grabs = %d, want = 1", len(e.client.added))
	}
	if add := e.client.added[0]; add.GetSeedRatio() != 2.5 || add.GetSeedTime().AsDuration() != 90*time.Minute {
		t.Errorf("seed ratio, time = %v, %v, want = 2.5, 1h30m0s", add.GetSeedRatio(), add.GetSeedTime().AsDuration())
	}

	rn := &run{rt: e.rt}
	ratio, seedTime := rn.seedSettings(e.ctx, &Release{PluginID: "idx", IndexerID: "ix", MinimumRatio: 1}, true)
	if ratio != 2.5 || seedTime != 3*time.Hour {
		t.Errorf("seedSettings(full season) = %v, %v, want = 2.5, 3h0m0s", ratio, seedTime)
	}
	ratio, seedTime = rn.seedSettings(e.ctx, &Release{PluginID: "idx", IndexerID: "gone", MinimumRatio: 1, MinimumSeedTime: time.Hour}, false)
	if ratio != 1 || seedTime != time.Hour {
		t.Errorf("seedSettings(unknown indexer) = %v, %v, want = the release's 1, 1h0m0s", ratio, seedTime)
	}
}

// TestIndexerSettings checks the decisions read the indexer's settings:
// releases without its required flags or minimum seeders are not grabbed.
func TestIndexerSettings(t *testing.T) {
	e := newEnv(t)
	it := e.addMovie()
	e.indexer.settings = func(b *pluginv1.Indexer_builder) {
		b.Tags = []string{"hd"}
		b.RequiredFlags = []string{"freeleech"}
		b.MinimumSeeders = new(int32(3))
		b.MultiLanguages = []string{"French", "Klingon"}
	}
	e.indexer.set(e.both(), nil)
	if err := e.rt.SearchMovie(e.ctx, it.ID, false); err != nil {
		t.Fatal(err)
	}
	if got := e.client.titles(); len(got) != 0 {
		t.Errorf("grabbed = %v, want = none without the required flags", got)
	}

	p := ports{rn: &run{rt: e.rt}}
	info, err := p.Indexer(e.ctx, "idx/ix")
	if err != nil || info == nil {
		t.Fatalf("Indexer() = %v, %v, want = the indexer", info, err)
	}
	if info.Name != "Indexer" || !slices.Equal(info.Tags, []string{"hd"}) || !slices.Equal(info.RequiredFlags, []string{"freeleech"}) ||
		info.MinimumSeeders != 3 || !slices.Equal(info.MultiLanguages, []release.Language{release.French}) {
		t.Errorf("Indexer() = %+v, want = the indexer's settings, without unknown languages", info)
	}
	if byName, err := p.IndexerByName(e.ctx, "Indexer"); err != nil || byName == nil || byName.ID != "idx/ix" {
		t.Errorf("IndexerByName(Indexer) = %+v, %v, want = idx/ix", byName, err)
	}
	if none, err := p.IndexerByName(e.ctx, "Other"); err != nil || none != nil {
		t.Errorf("IndexerByName(Other) = %+v, %v, want = nil", none, err)
	}
}

func TestDelayKeepsPendingThenGrabs(t *testing.T) {
	e := newEnv(t)
	delay := core.DelayProfile{
		ID: core.NewID(), Default: true, Order: 1, EnableUsenet: true, EnableTorrent: true,
		PreferredProtocol: core.ProtocolUsenet, TorrentDelay: time.Hour,
	}
	if err := e.db.DelayProfiles().Put(e.ctx, &delay); err != nil {
		t.Fatal(err)
	}
	it := e.addMovie()
	e.indexer.set([]*pluginv1.Release{torrent("g1080", title1080, 8*gb, hash1080, e.clock)}, nil)
	if err := e.rt.SearchMovie(e.ctx, it.ID, false); err != nil {
		t.Fatal(err)
	}
	if got := e.client.titles(); len(got) != 0 {
		t.Fatalf("grabbed = %v during the delay, want = none", got)
	}
	pending, err := e.db.PendingReleases().List(e.ctx, it.ID)
	if err != nil || len(pending) != 1 || pending[0].Reason != core.PendingDelay || pending[0].Title != title1080 {
		t.Fatalf("pending = %+v, %v, want = the release, delayed", pending, err)
	}
	// Still delayed half an hour on.
	e.indexer.set(nil, nil)
	e.clock = e.clock.Add(30 * time.Minute)
	if err := e.rt.SyncRSS(e.ctx); err != nil {
		t.Fatal(err)
	}
	if got := e.client.titles(); len(got) != 0 {
		t.Fatalf("grabbed = %v during the delay, want = none", got)
	}
	e.clock = e.clock.Add(time.Hour)
	if err := e.rt.SyncRSS(e.ctx); err != nil {
		t.Fatal(err)
	}
	if got := e.client.titles(); !slices.Equal(got, []string{title1080}) {
		t.Fatalf("grabbed = %v after the delay, want = [%s]", got, title1080)
	}
	if pending, _ := e.db.PendingReleases().List(e.ctx, it.ID); len(pending) != 0 {
		t.Errorf("pending after the grab = %d, want = 0", len(pending))
	}
	if g := e.history(core.HistoryGrabbed); len(g) != 1 || g[0].Data[dataReleaseSource] != "Rss" && g[0].Data[dataReleaseSource] != "Search" {
		t.Errorf("grab events = %v, want = one", g)
	}
}

func TestRSSMatchesByTitle(t *testing.T) {
	e := newEnv(t)
	it := e.addMovie()
	e.indexer.set(nil, []*pluginv1.Release{
		torrent("other", "Inception.2010.1080p.BluRay.x264-GROUP", 8*gb, "cccccccccccccccccccccccccccccccccccccccc", e.clock.Add(-time.Hour)),
		torrent("g1080", title1080, 8*gb, hash1080, e.clock.Add(-time.Hour)),
	})
	if err := e.rt.SyncRSS(e.ctx); err != nil {
		t.Fatal(err)
	}
	if got := e.client.titles(); !slices.Equal(got, []string{title1080}) {
		t.Fatalf("grabbed = %v, want = [%s]", got, title1080)
	}
	g := e.history(core.HistoryGrabbed)
	if len(g) != 1 || g[0].ItemID != it.ID || g[0].Data[dataReleaseSource] != "Rss" {
		t.Errorf("grab events = %+v, want = one RSS grab of the movie", g)
	}
}

func TestImport(t *testing.T) {
	for _, tc := range []struct {
		name     string
		hardlink bool
	}{
		{"hardlink", true},
		{"copy", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			e.settings.CopyUsingHardlinks = tc.hardlink
			it := e.addMovie()
			e.indexer.set(e.both(), nil)
			if err := e.rt.SearchMovie(e.ctx, it.ID, false); err != nil {
				t.Fatal(err)
			}
			video := title1080 + ".mkv"
			dir := e.completed(hash1080, title1080, video, title1080+".en.srt", "readme.nfo")
			e.clock = e.clock.Add(time.Hour)
			if err := e.rt.CheckQueue(e.ctx); err != nil {
				t.Fatal(err)
			}

			d := e.download(hash1080)
			if d.State != core.DownloadImported || len(d.Messages) != 0 {
				t.Fatalf("download = %s %v, want = imported", d.State, d.Messages)
			}
			dest := filepath.Join(e.libDir, "The Matrix (1999)", "The Matrix (1999) Bluray-1080p.mkv")
			destInfo, err := os.Stat(dest)
			if err != nil {
				t.Fatalf("imported file: %v", err)
			}
			srcInfo, err := os.Stat(filepath.Join(dir, video))
			if err != nil {
				t.Fatalf("download file: %v", err)
			}
			if got := os.SameFile(srcInfo, destInfo); got != tc.hardlink {
				t.Errorf("imported file is the download's = %v, want = %v", got, tc.hardlink)
			}
			if b, _ := os.ReadFile(dest); string(b) != "content of "+video {
				t.Errorf("imported content = %q", b)
			}
			if _, err := os.Stat(filepath.Join(e.libDir, "The Matrix (1999)", "The Matrix (1999) Bluray-1080p.en.srt")); err != nil {
				t.Errorf("imported subtitle: %v", err)
			}
			if _, err := os.Stat(filepath.Join(e.libDir, "The Matrix (1999)", "readme.nfo")); err == nil {
				t.Errorf("readme.nfo imported, want = only wanted extras")
			}

			got := e.item(it.ID)
			if got.Path != filepath.ToSlash(dest) || got.Virtual {
				t.Errorf("item path = %q virtual %v, want = %q, not virtual", got.Path, got.Virtual, filepath.ToSlash(dest))
			}
			if scans := e.jobs(library.JobScan); len(scans) != 1 || !strings.Contains(string(scans[0].Payload), e.lib.ID.String()) {
				t.Errorf("scan jobs = %v, want = one of the library", scans)
			}
			imported := e.history(core.HistoryImported)
			if len(imported) != 1 || imported[0].DownloadID != hash1080 || imported[0].Data[dataImportedPath] != filepath.ToSlash(dest) ||
				imported[0].Data[dataDroppedPath] != filepath.ToSlash(filepath.Join(dir, video)) || imported[0].Data[dataDownloadClient] != "dl/qb" {
				t.Errorf("imported events = %+v, want = one of the file", imported)
			}
			e.client.mu.Lock()
			marked, removed := slices.Clone(e.client.imported), slices.Clone(e.client.removed)
			e.client.mu.Unlock()
			if !slices.Equal(marked, []string{hash1080}) || !slices.Equal(removed, []string{hash1080}) {
				t.Errorf("marked imported %v, removed %v, want = the download once each", marked, removed)
			}

			// The movie has its file: the cutoff is met, no longer missing.
			if list, _, _ := e.rt.ListWanted(e.ctx, WantedMissing, 0, 0); len(list) != 0 {
				t.Errorf("missing = %d, want = 0", len(list))
			}
			// Polling again changes nothing.
			if err := e.rt.CheckQueue(e.ctx); err != nil {
				t.Fatal(err)
			}
			if n := len(e.history(core.HistoryImported)); n != 1 {
				t.Errorf("imported events after another poll = %d, want = 1", n)
			}
		})
	}
}

func TestImportBlocked(t *testing.T) {
	e := newEnv(t)
	it := e.addMovie()
	e.indexer.set(e.both(), nil)
	if err := e.rt.SearchMovie(e.ctx, it.ID, false); err != nil {
		t.Fatal(err)
	}
	// Only a sample: blocked, and retried on later polls.
	e.completed(hash1080, title1080, "sample.mkv")
	if err := e.rt.CheckQueue(e.ctx); err != nil {
		t.Fatal(err)
	}
	d := e.download(hash1080)
	if d.State != core.DownloadImportBlocked && d.State != core.DownloadImportPending {
		t.Errorf("download state = %s %v, want = blocked or pending", d.State, d.Messages)
	}
	if e.item(it.ID).Virtual != true {
		t.Errorf("item has a file, want = none")
	}
	// A path the client's host gives that is not one of the server's
	// needs a remote path mapping: a Windows path on other systems, a Unix
	// one on Windows, neither of them mapped.
	remote := `D:\Downloads\` + title1080
	if runtime.GOOS == "windows" {
		remote = "/elsewhere/" + title1080
	}
	e.client.setDownloads(pluginv1.Download_builder{
		ClientId: new("qb"), Id: new(hash1080), Title: new(title1080), Status: new(pluginv1.DownloadStatus_DOWNLOAD_STATUS_COMPLETED),
		OutputPath: new(remote), CanBeRemoved: new(true),
	}.Build())
	if err := e.rt.CheckQueue(e.ctx); err != nil {
		t.Fatal(err)
	}
	if d := e.download(hash1080); len(d.Messages) != 1 || !strings.Contains(d.Messages[0], "Remote Path Mapping") {
		t.Errorf("messages = %v, want = a remote path mapping warning", d.Messages)
	}
}

func TestUpgradeRecyclesOldFile(t *testing.T) {
	e := newEnv(t)
	it := e.addMovie()
	folder := filepath.Join(e.libDir, "The Matrix (1999)")
	oldName := "The Matrix (1999) Bluray-720p.mkv"
	old := filepath.Join(folder, oldName)
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(old, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	cur := e.item(it.ID)
	cur.Path, cur.Virtual = filepath.ToSlash(old), false
	if err := e.db.Items().Upsert(e.ctx, cur); err != nil {
		t.Fatal(err)
	}
	// The bin has a file of the name already, long since.
	binFolder := filepath.Join(e.binDir, "The Matrix (1999)")
	if err := os.MkdirAll(binFolder, 0o755); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(binFolder, oldName)
	if err := os.WriteFile(stale, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	long := e.clock.AddDate(0, 0, -10)
	if err := os.Chtimes(stale, long, long); err != nil {
		t.Fatal(err)
	}

	// The 720p file is an upgrade of nothing; the 1080p release is.
	if list, _, _ := e.rt.ListWanted(e.ctx, WantedCutoffUnmet, 0, 0); len(list) != 1 || list[0].Quality != "Bluray-720p" {
		t.Errorf("cutoff unmet = %+v, want = the movie at Bluray-720p", list)
	}
	e.indexer.set(e.both(), nil)
	if err := e.rt.SearchMovie(e.ctx, it.ID, false); err != nil {
		t.Fatal(err)
	}
	if got := e.client.titles(); !slices.Equal(got, []string{title1080}) {
		t.Fatalf("grabbed = %v, want = [%s]", got, title1080)
	}
	e.completed(hash1080, title1080, title1080+".mkv")
	if err := e.rt.CheckQueue(e.ctx); err != nil {
		t.Fatal(err)
	}
	if d := e.download(hash1080); d.State != core.DownloadImported {
		t.Fatalf("download = %s %v, want = imported", d.State, d.Messages)
	}
	if _, err := os.Stat(old); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("old file: err = %v, want = gone", err)
	}
	recycled := filepath.Join(binFolder, "The Matrix (1999) Bluray-720p_2.mkv")
	if b, err := os.ReadFile(recycled); err != nil || string(b) != "old" {
		t.Errorf("recycled file = %q, %v, want = the old file", b, err)
	}
	if fi, err := os.Stat(recycled); err == nil && !fi.ModTime().Equal(e.clock) {
		t.Errorf("recycled mtime = %v, want = %v", fi.ModTime(), e.clock)
	}
	deleted := e.history(core.HistoryFileDeleted)
	if len(deleted) != 1 || deleted[0].Data[dataReason] != "Upgrade" || deleted[0].SourceTitle != filepath.ToSlash(old) {
		t.Errorf("file deleted events = %+v, want = one upgrade", deleted)
	}
	if got := e.item(it.ID).Path; !strings.HasSuffix(got, "The Matrix (1999) Bluray-1080p.mkv") {
		t.Errorf("item path = %q, want = the 1080p file", got)
	}

	// Cleaning deletes only what has been in the bin longer than the days.
	if err := e.rt.CleanRecycleBin(e.ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stale); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("stale file: err = %v, want = deleted", err)
	}
	if _, err := os.Stat(recycled); err != nil {
		t.Errorf("recycled file: %v, want = kept", err)
	}
	e.clock = e.clock.AddDate(0, 0, 8)
	if err := e.rt.CleanRecycleBin(e.ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(binFolder); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("bin folder: err = %v, want = deleted when emptied", err)
	}
}

func TestFailedBlocklistsAndSearchesAgain(t *testing.T) {
	e := newEnv(t)
	it := e.addMovie()
	e.indexer.set(e.both(), nil)
	if err := e.rt.SearchMovie(e.ctx, it.ID, false); err != nil {
		t.Fatal(err)
	}
	e.client.setDownloads(pluginv1.Download_builder{
		ClientId: new("qb"), Id: new(hash1080), Title: new(title1080), Status: new(pluginv1.DownloadStatus_DOWNLOAD_STATUS_FAILED),
		Message: new("Unpack failed"), CanBeRemoved: new(true),
	}.Build())
	if err := e.rt.CheckQueue(e.ctx); err != nil {
		t.Fatal(err)
	}
	if d := e.download(hash1080); d.State != core.DownloadFailed || !slices.Equal(d.Messages, []string{"Unpack failed"}) {
		t.Errorf("download = %s %v, want = failed: Unpack failed", d.State, d.Messages)
	}
	bl := e.blocklist(it.ID)
	if len(bl) != 1 || bl[0].TorrentInfoHash != hash1080 || bl[0].Protocol != core.ProtocolTorrent || bl[0].SourceTitle != title1080 ||
		bl[0].Indexer != "Indexer" || bl[0].Size != 8*gb {
		t.Errorf("blocklist = %+v, want = the 1080p release", bl)
	}
	if f := e.history(core.HistoryDownloadFailed); len(f) != 1 || f[0].Data[dataMessage] != "Unpack failed" {
		t.Errorf("download failed events = %+v, want = one", f)
	}
	if n := len(e.jobs(JobSearch)); n != 1 {
		t.Errorf("search jobs = %d, want = 1", n)
	}
	e.client.mu.Lock()
	removed := slices.Clone(e.client.removed)
	e.client.mu.Unlock()
	if !slices.Equal(removed, []string{hash1080}) {
		t.Errorf("removed = %v, want = the failed download", removed)
	}

	// Searching again grabs the next best release.
	if err := e.rt.SearchMovie(e.ctx, it.ID, false); err != nil {
		t.Fatal(err)
	}
	if got := e.client.titles(); !slices.Equal(got, []string{title1080, title720}) {
		t.Errorf("grabbed = %v, want = the 720p release after the blocklisted one", got)
	}
}

func TestMarkFailed(t *testing.T) {
	e := newEnv(t)
	e.settings.AutoRedownloadFailed = false
	it := e.addMovie()
	e.indexer.set(e.both(), nil)
	if err := e.rt.SearchMovie(e.ctx, it.ID, false); err != nil {
		t.Fatal(err)
	}
	grab := e.history(core.HistoryGrabbed)[0]
	if err := e.rt.MarkFailed(e.ctx, grab.ID); err != nil {
		t.Fatal(err)
	}
	if d := e.download(hash1080); d.State != core.DownloadFailed || !slices.Equal(d.Messages, []string{msgManuallyFailed}) {
		t.Errorf("download = %s %v, want = manually failed", d.State, d.Messages)
	}
	if bl := e.blocklist(it.ID); len(bl) != 1 || bl[0].Message != msgManuallyFailed {
		t.Errorf("blocklist = %+v, want = one", bl)
	}
	if n := len(e.jobs(JobSearch)); n != 0 {
		t.Errorf("search jobs = %d, want = 0 without redownload", n)
	}
	if err := e.rt.MarkFailed(e.ctx, core.NewID()); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("MarkFailed(unknown) err = %v, want = ErrNotFound", err)
	}
}

func TestRemoveFromQueue(t *testing.T) {
	for _, tc := range []struct {
		name              string
		blocklist, search bool
		state             core.DownloadState
		blocked, searches int
	}{
		{"ignore", false, false, core.DownloadIgnored, 0, 0},
		{"blocklist and search", true, true, core.DownloadFailed, 1, 1},
		{"blocklist", true, false, core.DownloadFailed, 1, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			it := e.addMovie()
			e.indexer.set(e.both(), nil)
			if err := e.rt.SearchMovie(e.ctx, it.ID, false); err != nil {
				t.Fatal(err)
			}
			d := e.download(hash1080)
			if err := e.rt.RemoveFromQueue(e.ctx, d.ID, true, tc.blocklist, tc.search); err != nil {
				t.Fatal(err)
			}
			if got := e.download(hash1080).State; got != tc.state {
				t.Errorf("state = %s, want = %s", got, tc.state)
			}
			if got := len(e.blocklist(it.ID)); got != tc.blocked {
				t.Errorf("blocklisted = %d, want = %d", got, tc.blocked)
			}
			if got := len(e.jobs(JobSearch)); got != tc.searches {
				t.Errorf("search jobs = %d, want = %d", got, tc.searches)
			}
			e.client.mu.Lock()
			removed := slices.Clone(e.client.removed)
			e.client.mu.Unlock()
			if !slices.Equal(removed, []string{hash1080}) {
				t.Errorf("removed = %v, want = the download", removed)
			}
			if !tc.blocklist {
				if ig := e.history(core.HistoryIgnored); len(ig) != 1 {
					t.Errorf("ignored events = %d, want = 1", len(ig))
				}
			}
		})
	}
}

func TestQueueStatusAndMissing(t *testing.T) {
	e := newEnv(t)
	it := e.addMovie()
	e.indexer.set(e.both(), nil)
	if err := e.rt.SearchMovie(e.ctx, it.ID, false); err != nil {
		t.Fatal(err)
	}
	e.client.setDownloads(
		pluginv1.Download_builder{
			ClientId: new("qb"), Id: new(hash1080), Title: new(title1080), Status: new(pluginv1.DownloadStatus_DOWNLOAD_STATUS_WARNING),
			Message: new("Stalled"), RemainingSize: new(gb),
		}.Build(),
		// Not tracked: left alone.
		pluginv1.Download_builder{ClientId: new("qb"), Id: new("other"), Title: new("Other"), Status: new(pluginv1.DownloadStatus_DOWNLOAD_STATUS_COMPLETED)}.Build(),
	)
	if err := e.rt.CheckQueue(e.ctx); err != nil {
		t.Fatal(err)
	}
	d := e.download(hash1080)
	if d.State != core.DownloadDownloading || !slices.Equal(d.Messages, []string{"Stalled"}) {
		t.Errorf("download = %s %v, want = downloading with the warning", d.State, d.Messages)
	}
	if s, ok := e.rt.Status(d.ID); !ok || s.RemainingSize != gb || s.Status != pluginv1.DownloadStatus_DOWNLOAD_STATUS_WARNING {
		t.Errorf("Status() = %+v, %v, want = what the client told", s, ok)
	}
	if n := len(e.downloads()); n != 1 {
		t.Errorf("tracked downloads = %d, want = 1", n)
	}

	e.client.setDownloads()
	if err := e.rt.CheckQueue(e.ctx); err != nil {
		t.Fatal(err)
	}
	if d := e.download(hash1080); !slices.Contains(d.Messages, msgNotInClient) {
		t.Errorf("messages = %v, want = %q", d.Messages, msgNotInClient)
	}
	if _, ok := e.rt.Status(d.ID); ok {
		t.Errorf("Status() kept for a download gone from its client")
	}
}

func TestGrabRelease(t *testing.T) {
	e := newEnv(t)
	it := e.addMovie()
	rel := &Release{
		PluginID: "idx", IndexerID: "ix", IndexerName: "Indexer", GUID: "g720", Title: title720, Size: 4 * gb,
		DownloadURL: "https://indexer.test/g720", Protocol: core.ProtocolTorrent, InfoHash: hash720, PublishDate: e.clock,
	}
	d, err := e.rt.GrabRelease(e.ctx, it.ID, rel, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if d.DownloadID != hash720 || e.client.added[0].GetData() == nil {
		t.Errorf("download = %+v, want = the 720p release sent as its torrent file", d)
	}
	if g := e.history(core.HistoryGrabbed); len(g) != 1 || g[0].Data[dataReleaseSource] != "InteractiveSearch" {
		t.Errorf("grab events = %+v, want = one interactive grab", g)
	}
	if _, err := e.rt.GrabRelease(e.ctx, it.ID, rel, "dl", "nope"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("GrabRelease(unknown client) err = %v, want = ErrNotFound", err)
	}
	usenet := *rel
	usenet.Protocol = core.ProtocolUsenet
	if _, err := e.rt.GrabRelease(e.ctx, it.ID, &usenet, "", ""); !errors.Is(err, core.ErrConflict) {
		t.Errorf("GrabRelease(no usenet client) err = %v, want = ErrConflict", err)
	}
}

func TestScheduleAndHandlers(t *testing.T) {
	e := newEnv(t)
	if err := e.rt.Schedule(e.ctx, e.settings); err != nil {
		t.Fatal(err)
	}
	// Scheduling again keeps the pending jobs.
	if err := e.rt.Schedule(e.ctx, e.settings); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{JobQueue, JobRecycleBin, JobRSS} {
		if n := len(e.jobs(kind)); n != 1 {
			t.Errorf("%s jobs = %d, want = 1", kind, n)
		}
	}
	h := e.rt.Handlers()
	next, err := h[JobQueue](e.ctx, scheduledJob(JobQueue, e.clock, true))
	if err != nil || len(next) != 1 || !next[0].RunAt.Equal(e.clock.Add(time.Minute)) {
		t.Errorf("queue handler = %v, %v, want = the next run in a minute", next, err)
	}
	if next, err := h[JobRSS](e.ctx, scheduledJob(JobRSS, e.clock, false)); err != nil || len(next) != 0 {
		t.Errorf("requested RSS handler = %v, %v, want = no next run", next, err)
	}
	e.settings.RSSSyncInterval = 0
	if next, err := h[JobRSS](e.ctx, scheduledJob(JobRSS, e.clock, true)); err != nil || len(next) != 0 {
		t.Errorf("disabled RSS handler = %v, %v, want = no next run", next, err)
	}
	it := e.addMovie()
	e.indexer.set(e.both(), nil)
	if n, err := e.rt.QueueSearch(e.ctx, []core.ID{it.ID}, true); err != nil || n != 1 {
		t.Fatalf("QueueSearch() = %d, %v", n, err)
	}
	jobs := e.jobs(JobSearch)
	if len(jobs) != 1 {
		t.Fatalf("search jobs = %d, want = 1", len(jobs))
	}
	if _, err := h[JobSearch](e.ctx, jobs[0]); err != nil {
		t.Fatal(err)
	}
	if got := e.client.titles(); !slices.Equal(got, []string{title1080}) {
		t.Errorf("grabbed = %v, want = [%s]", got, title1080)
	}
}
