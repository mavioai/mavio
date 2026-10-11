package rpc

import (
	"context"
	"net/http"
	"slices"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mavioai/mavio/apps/server/internal/acquisition"
	"github.com/mavioai/mavio/apps/server/internal/downloads"
	"github.com/mavioai/mavio/apps/server/internal/releases"
	"github.com/mavioai/mavio/libs/acquire"
	"github.com/mavioai/mavio/libs/core"
	acquisitionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1/acquisitionv1connect"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1/pluginv1connect"
	"github.com/mavioai/mavio/libs/release"
	"github.com/mavioai/mavio/libs/store"
)

const (
	matrix1080 = "The.Matrix.1999.1080p.BluRay.x264-GROUP"
	matrix720  = "The.Matrix.1999.720p.BluRay.x264-OTHER"
	hashA      = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	hashB      = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

// stubIndexer is an indexer provider with one indexer finding releases.
type stubIndexer struct {
	pluginv1connect.IndexerProviderServiceClient
	found []*pluginv1.Release
}

func (s *stubIndexer) ListIndexers(context.Context, *pluginv1.ListIndexersRequest) (*pluginv1.ListIndexersResponse, error) {
	caps := pluginv1.IndexerCapabilities_builder{SearchParams: []string{"q"}, MovieSearchParams: []string{"q", "imdbid", "tmdbid"}}.Build()
	return pluginv1.ListIndexersResponse_builder{Indexers: []*pluginv1.Indexer{
		pluginv1.Indexer_builder{Id: new("ix"), Name: new("Indexer"), Enabled: new(true), Priority: new(int32(25)), Capabilities: caps}.Build(),
	}}.Build(), nil
}

func (s *stubIndexer) SearchReleases(context.Context, *pluginv1.SearchReleasesRequest) (*pluginv1.SearchReleasesResponse, error) {
	return pluginv1.SearchReleasesResponse_builder{Releases: s.found}.Build(), nil
}

// stubClient is a download client provider with one torrent client.
type stubClient struct {
	pluginv1connect.DownloadClientProviderServiceClient
	mu      sync.Mutex
	added   []string
	removed []string
}

func (s *stubClient) ListDownloadClients(context.Context, *pluginv1.ListDownloadClientsRequest) (*pluginv1.ListDownloadClientsResponse, error) {
	return pluginv1.ListDownloadClientsResponse_builder{Clients: []*pluginv1.DownloadClient{pluginv1.DownloadClient_builder{
		Id: new("qb"), Name: new("qBittorrent"), Protocol: new(pluginv1.DownloadProtocol_DOWNLOAD_PROTOCOL_TORRENT), Enabled: new(true), Priority: new(int32(1)),
	}.Build()}}.Build(), nil
}

func (s *stubClient) AddDownload(_ context.Context, req *pluginv1.AddDownloadRequest) (*pluginv1.AddDownloadResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.added = append(s.added, req.GetTitle())
	return pluginv1.AddDownloadResponse_builder{DownloadId: new(req.GetInfoHash())}.Build(), nil
}

func (s *stubClient) ListDownloads(context.Context, *pluginv1.ListDownloadsRequest) (*pluginv1.ListDownloadsResponse, error) {
	return &pluginv1.ListDownloadsResponse{}, nil
}

func (s *stubClient) RemoveDownload(_ context.Context, req *pluginv1.RemoveDownloadRequest) (*pluginv1.RemoveDownloadResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.removed = append(s.removed, req.GetDownloadId())
	return &pluginv1.RemoveDownloadResponse{}, nil
}

func stubRelease(guid, title string, size int64, hash string, published time.Time) *pluginv1.Release {
	return pluginv1.Release_builder{
		Guid: new(guid), Title: new(title), Size: new(size), DownloadUrl: new("https://indexer.test/" + guid),
		MagnetUrl: new("magnet:?xt=urn:btih:" + hash), InfoHash: new(hash), Protocol: new(pluginv1.DownloadProtocol_DOWNLOAD_PROTOCOL_TORRENT),
		PublishDate: timestamppb.New(published), Seeders: new(int32(50)), Peers: new(int32(60)), Categories: []int32{2000},
	}.Build()
}

// acquisitionFixture is a store with a movie library and an HD profile,
// and a runtime over a stub indexer and download client.
type acquisitionFixture struct {
	db      *store.Store
	rt      *acquisition.Runtime
	indexer *stubIndexer
	client  *stubClient
	lib     core.Library
	profile core.ID
	now     time.Time
}

func newAcquisitionFixture(t *testing.T) *acquisitionFixture {
	t.Helper()
	ctx := t.Context()
	f := &acquisitionFixture{db: openStore(t), indexer: &stubIndexer{}, client: &stubClient{}, now: time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)}
	f.rt = &acquisition.Runtime{
		Store:    f.db,
		Indexers: func() []releases.Provider { return []releases.Provider{{ID: "idx", Client: f.indexer}} },
		Clients:  func() []downloads.Provider { return []downloads.Provider{{ID: "dl", Client: f.client}} },
		Now:      func() time.Time { return f.now },
	}
	f.lib = core.Library{ID: core.NewID(), Name: "Movies", Kind: core.LibraryMovies, Paths: []string{t.TempDir()}}
	if err := f.db.Libraries().Create(ctx, &f.lib); err != nil {
		t.Fatal(err)
	}
	p := acquire.NewProfile(release.Movie, "HD", release.Bluray1080p, []release.Quality{release.Bluray720p, release.Bluray1080p}, nil)
	p.ID, p.UpgradeAllowed = core.NewID(), true
	cp, err := acquire.QualityProfileToCore(p, core.KindMovie)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.db.QualityProfiles().Put(ctx, &cp); err != nil {
		t.Fatal(err)
	}
	f.profile = cp.ID
	f.indexer.found = []*pluginv1.Release{
		stubRelease("g720", matrix720, 4<<30, hashB, f.now.Add(-time.Hour)),
		stubRelease("g1080", matrix1080, 8<<30, hashA, f.now.Add(-time.Hour)),
	}
	return f
}

// identify names an added movie as its metadata providers would.
func (f *acquisitionFixture) identify(t *testing.T, id string) {
	t.Helper()
	itemID, err := core.ParseID(id)
	if err != nil {
		t.Fatal(err)
	}
	it, err := f.db.Items().Get(t.Context(), itemID)
	if err != nil {
		t.Fatal(err)
	}
	it.Name, it.ProductionYear, it.Runtime = "The Matrix", 1999, 136*time.Minute
	premiere := time.Date(1999, 3, 31, 0, 0, 0, 0, time.UTC)
	it.PremiereDate = &premiere
	it.ExternalIDs[core.ProviderIMDb] = "tt0133093"
	if err := f.db.Items().Upsert(t.Context(), it); err != nil {
		t.Fatal(err)
	}
}

func (f *acquisitionFixture) searchJobs(t *testing.T) int {
	t.Helper()
	page, err := f.db.Jobs().List(t.Context(), core.JobQuery{Kinds: []string{acquisition.JobSearch}})
	if err != nil {
		t.Fatal(err)
	}
	return len(page.Items)
}

// TestWantedService adds, reads, changes, lists and searches wanted
// movies.
func TestWantedService(t *testing.T) {
	ctx := t.Context()
	f := newAcquisitionFixture(t)
	url := testHandlers(t, func(opts connect.HandlerOption) (string, http.Handler) {
		return acquisitionv1connect.NewWantedServiceHandler(NewWantedService(f.rt), opts)
	})
	admin := acquisitionv1connect.NewWantedServiceClient(http.DefaultClient, url, as("admin"))
	user := acquisitionv1connect.NewWantedServiceClient(http.DefaultClient, url, as("user"))

	add := acquisitionv1.AddMovieRequest_builder{
		LibraryId: new(f.lib.ID.String()), TmdbId: new(int32(603)), QualityProfileId: new(f.profile.String()),
		MinimumAvailability: new(acquisitionv1.MinimumAvailability_MINIMUM_AVAILABILITY_IN_CINEMAS), Tags: []string{"hd"},
	}.Build()
	_, err := user.AddMovie(ctx, add)
	wantCode(t, "AddMovie as user", err, connect.CodePermissionDenied)
	added, err := admin.AddMovie(ctx, add)
	if err != nil {
		t.Fatalf("AddMovie: %v", err)
	}
	w := added.GetWanted()
	if !w.GetMonitored() || w.GetQualityProfileId() != f.profile.String() || !slices.Equal(w.GetTags(), []string{"hd"}) ||
		w.GetMinimumAvailability() != acquisitionv1.MinimumAvailability_MINIMUM_AVAILABILITY_IN_CINEMAS || !w.GetAddTime().AsTime().Equal(f.now) || w.HasLastSearchTime() {
		t.Errorf("AddMovie wanted = %v", w)
	}
	_, err = admin.AddMovie(ctx, add)
	wantCode(t, "AddMovie twice", err, connect.CodeAlreadyExists)
	badProfile := acquisitionv1.AddMovieRequest_builder{LibraryId: new(f.lib.ID.String()), TmdbId: new(int32(604)), QualityProfileId: new(core.NewID().String())}.Build()
	_, err = admin.AddMovie(ctx, badProfile)
	wantCode(t, "AddMovie with an unknown profile", err, connect.CodeInvalidArgument)
	_, err = admin.AddMovie(ctx, acquisitionv1.AddMovieRequest_builder{LibraryId: new(f.lib.ID.String()), QualityProfileId: new(f.profile.String())}.Build())
	wantCode(t, "AddMovie without a TMDB ID", err, connect.CodeInvalidArgument)
	f.identify(t, added.GetItemId())

	got, err := admin.GetWanted(ctx, acquisitionv1.GetWantedRequest_builder{ItemId: new(added.GetItemId())}.Build())
	if err != nil || got.GetWanted().GetItemId() != added.GetItemId() {
		t.Fatalf("GetWanted = %v, %v", got, err)
	}
	_, err = admin.GetWanted(ctx, acquisitionv1.GetWantedRequest_builder{ItemId: new(core.NewID().String())}.Build())
	wantCode(t, "GetWanted of an unknown movie", err, connect.CodeNotFound)
	_, err = user.GetWanted(ctx, acquisitionv1.GetWantedRequest_builder{ItemId: new(added.GetItemId())}.Build())
	wantCode(t, "GetWanted as user", err, connect.CodePermissionDenied)

	list := func(filter acquisitionv1.WantedFilter) *acquisitionv1.ListWantedResponse {
		t.Helper()
		res, err := admin.ListWanted(ctx, acquisitionv1.ListWantedRequest_builder{Filter: new(filter)}.Build())
		if err != nil {
			t.Fatalf("ListWanted: %v", err)
		}
		return res
	}
	missing := list(acquisitionv1.WantedFilter_WANTED_FILTER_MISSING)
	if missing.GetTotal() != 1 || len(missing.GetMovies()) != 1 {
		t.Fatalf("missing = %v, want = the movie", missing)
	}
	if m := missing.GetMovies()[0]; m.GetName() != "The Matrix" || m.GetYear() != 1999 || !m.GetAvailable() || m.GetQuality() != "" {
		t.Errorf("missing movie = %v", m)
	}
	if res := list(acquisitionv1.WantedFilter_WANTED_FILTER_CUTOFF_UNMET); res.GetTotal() != 0 {
		t.Errorf("cutoff unmet = %v, want = none", res)
	}
	_, err = admin.ListWanted(ctx, acquisitionv1.ListWantedRequest_builder{Filter: new(acquisitionv1.WantedFilter_WANTED_FILTER_UNSPECIFIED)}.Build())
	wantCode(t, "ListWanted with an unspecified filter", err, connect.CodeInvalidArgument)

	// Searches are queued as jobs.
	searched, err := admin.SearchWanted(ctx, acquisitionv1.SearchWantedRequest_builder{Filter: new(acquisitionv1.WantedFilter_WANTED_FILTER_MISSING)}.Build())
	if err != nil || searched.GetQueued() != 1 || f.searchJobs(t) != 1 {
		t.Errorf("SearchWanted(missing) = %v, %v, jobs %d, want = 1 queued", searched, err, f.searchJobs(t))
	}
	_, err = admin.SearchWanted(ctx, acquisitionv1.SearchWantedRequest_builder{ItemIds: []string{core.NewID().String()}}.Build())
	wantCode(t, "SearchWanted of an unknown movie", err, connect.CodeNotFound)
	if _, err := admin.SyncRSS(ctx, &acquisitionv1.SyncRSSRequest{}); err != nil {
		t.Errorf("SyncRSS: %v", err)
	}
	if page, _ := f.db.Jobs().List(ctx, core.JobQuery{Kinds: []string{acquisition.JobRSS}}); len(page.Items) != 1 {
		t.Errorf("RSS jobs = %d, want = 1", len(page.Items))
	}
	_, err = user.SyncRSS(ctx, &acquisitionv1.SyncRSSRequest{})
	wantCode(t, "SyncRSS as user", err, connect.CodePermissionDenied)

	upd := acquisitionv1.Wanted_builder{
		ItemId: new(added.GetItemId()), Monitored: new(false), QualityProfileId: new(f.profile.String()),
		MinimumAvailability: new(acquisitionv1.MinimumAvailability_MINIMUM_AVAILABILITY_RELEASED),
	}.Build()
	updated, err := admin.UpdateWanted(ctx, acquisitionv1.UpdateWantedRequest_builder{Wanted: upd}.Build())
	if err != nil {
		t.Fatalf("UpdateWanted: %v", err)
	}
	if uw := updated.GetWanted(); uw.GetMonitored() || len(uw.GetTags()) != 0 || !uw.GetAddTime().AsTime().Equal(f.now) ||
		uw.GetMinimumAvailability() != acquisitionv1.MinimumAvailability_MINIMUM_AVAILABILITY_RELEASED {
		t.Errorf("UpdateWanted = %v", uw)
	}
	if res := list(acquisitionv1.WantedFilter_WANTED_FILTER_MISSING); res.GetTotal() != 0 {
		t.Errorf("missing after unmonitoring = %d, want = 0", res.GetTotal())
	}
	upd.SetQualityProfileId(core.NewID().String())
	_, err = admin.UpdateWanted(ctx, acquisitionv1.UpdateWantedRequest_builder{Wanted: upd}.Build())
	wantCode(t, "UpdateWanted with an unknown profile", err, connect.CodeInvalidArgument)
}

// TestDownloadService grabs a release a search found, lists the queue
// and the history, marks the download failed, and removes the
// blocklisted release.
func TestDownloadService(t *testing.T) {
	ctx := t.Context()
	f := newAcquisitionFixture(t)
	url := testHandlers(t, func(opts connect.HandlerOption) (string, http.Handler) {
		mux := http.NewServeMux()
		mux.Handle(acquisitionv1connect.NewDownloadServiceHandler(NewDownloadService(f.db, f.rt), opts))
		mux.Handle(acquisitionv1connect.NewReleaseServiceHandler(NewReleaseService(f.db, f.rt.Indexers, f.rt), opts))
		mux.Handle(acquisitionv1connect.NewWantedServiceHandler(NewWantedService(f.rt), opts))
		return "/", mux
	})
	admin := acquisitionv1connect.NewDownloadServiceClient(http.DefaultClient, url, as("admin"))
	user := acquisitionv1connect.NewDownloadServiceClient(http.DefaultClient, url, as("user"))
	search := acquisitionv1connect.NewReleaseServiceClient(http.DefaultClient, url, as("admin"))
	wanted := acquisitionv1connect.NewWantedServiceClient(http.DefaultClient, url, as("admin"))

	clients, err := admin.ListDownloadClients(ctx, &acquisitionv1.ListDownloadClientsRequest{})
	if err != nil || len(clients.GetClients()) != 1 {
		t.Fatalf("ListDownloadClients = %v, %v", clients, err)
	}
	if c := clients.GetClients()[0]; c.GetPluginId() != "dl" || c.GetId() != "qb" || c.GetProtocol() != acquisitionv1.DownloadProtocol_DOWNLOAD_PROTOCOL_TORRENT {
		t.Errorf("client = %v", c)
	}
	_, err = user.ListDownloadClients(ctx, &acquisitionv1.ListDownloadClientsRequest{})
	wantCode(t, "ListDownloadClients as user", err, connect.CodePermissionDenied)

	added, err := wanted.AddMovie(ctx, acquisitionv1.AddMovieRequest_builder{
		LibraryId: new(f.lib.ID.String()), TmdbId: new(int32(603)), QualityProfileId: new(f.profile.String()),
	}.Build())
	if err != nil {
		t.Fatalf("AddMovie: %v", err)
	}
	f.identify(t, added.GetItemId())

	// Searches of wanted movies come with the decisions, best first.
	found, err := search.SearchReleases(ctx, acquisitionv1.SearchReleasesRequest_builder{ItemId: new(added.GetItemId())}.Build())
	if err != nil {
		t.Fatalf("SearchReleases: %v", err)
	}
	rels := found.GetReleases()
	if len(rels) != 2 || rels[0].GetTitle() != matrix1080 || !rels[0].GetEvaluated() || len(rels[0].GetRejections()) != 0 {
		t.Fatalf("SearchReleases = %v, want = the 1080p release first, approved", rels)
	}

	grabbed, err := admin.GrabRelease(ctx, acquisitionv1.GrabReleaseRequest_builder{ItemId: new(added.GetItemId()), Release: rels[1]}.Build())
	if err != nil {
		t.Fatalf("GrabRelease: %v", err)
	}
	item := grabbed.GetItem()
	if item.GetTitle() != matrix720 || item.GetDownloadId() != hashB || item.GetItemName() != "The Matrix" || item.GetQuality() != "Bluray-720p" ||
		item.GetState() != acquisitionv1.DownloadState_DOWNLOAD_STATE_DOWNLOADING || item.GetIndexer().GetIndexerId() != "ix" {
		t.Errorf("GrabRelease item = %v", item)
	}
	_, err = admin.GrabRelease(ctx, acquisitionv1.GrabReleaseRequest_builder{ItemId: new(added.GetItemId()), Release: rels[0], ClientPluginId: new("dl"), ClientId: new("nope")}.Build())
	wantCode(t, "GrabRelease to an unknown client", err, connect.CodeNotFound)
	usenet := acquisitionv1.Release_builder{
		PluginId: new("idx"), IndexerId: new("ix"), Guid: new("nzb"), Title: new(matrix1080), DownloadUrl: new("https://indexer.test/nzb"),
		Protocol: new(acquisitionv1.DownloadProtocol_DOWNLOAD_PROTOCOL_USENET),
	}.Build()
	_, err = admin.GrabRelease(ctx, acquisitionv1.GrabReleaseRequest_builder{ItemId: new(added.GetItemId()), Release: usenet}.Build())
	wantCode(t, "GrabRelease without a usenet client", err, connect.CodeFailedPrecondition)
	_, err = user.GrabRelease(ctx, acquisitionv1.GrabReleaseRequest_builder{ItemId: new(added.GetItemId()), Release: rels[0]}.Build())
	wantCode(t, "GrabRelease as user", err, connect.CodePermissionDenied)

	queue, err := admin.ListQueue(ctx, &acquisitionv1.ListQueueRequest{})
	if err != nil || queue.GetTotal() != 1 || queue.GetItems()[0].GetId() != item.GetId() {
		t.Fatalf("ListQueue = %v, %v, want = the grab", queue, err)
	}

	history, err := admin.ListHistory(ctx, acquisitionv1.ListHistoryRequest_builder{
		ItemId: new(added.GetItemId()), Types: []acquisitionv1.HistoryEventType{acquisitionv1.HistoryEventType_HISTORY_EVENT_TYPE_GRABBED},
	}.Build())
	if err != nil || history.GetTotal() != 1 {
		t.Fatalf("ListHistory = %v, %v, want = the grab", history, err)
	}
	ev := history.GetEvents()[0]
	if ev.GetSourceTitle() != matrix720 || ev.GetDownloadId() != hashB || ev.GetData()["downloadClient"] != "dl/qb" || ev.GetQuality() != "Bluray-720p" {
		t.Errorf("grab event = %v", ev)
	}

	if _, err := admin.MarkFailed(ctx, acquisitionv1.MarkFailedRequest_builder{HistoryEventId: new(ev.GetId())}.Build()); err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}
	_, err = admin.MarkFailed(ctx, acquisitionv1.MarkFailedRequest_builder{HistoryEventId: new(core.NewID().String())}.Build())
	wantCode(t, "MarkFailed of an unknown event", err, connect.CodeNotFound)
	if f.searchJobs(t) != 1 {
		t.Errorf("search jobs after MarkFailed = %d, want = 1", f.searchJobs(t))
	}
	queue, err = admin.ListQueue(ctx, &acquisitionv1.ListQueueRequest{})
	if err != nil || queue.GetTotal() != 0 {
		t.Errorf("ListQueue after the failure = %v, %v, want = empty", queue, err)
	}
	all, err := admin.ListQueue(ctx, acquisitionv1.ListQueueRequest_builder{IncludeFinished: new(true)}.Build())
	if err != nil || all.GetTotal() != 1 || all.GetItems()[0].GetState() != acquisitionv1.DownloadState_DOWNLOAD_STATE_FAILED {
		t.Errorf("ListQueue(finished) = %v, %v, want = the failed download", all, err)
	}

	bl, err := admin.ListBlocklist(ctx, acquisitionv1.ListBlocklistRequest_builder{ItemId: new(added.GetItemId())}.Build())
	if err != nil || bl.GetTotal() != 1 {
		t.Fatalf("ListBlocklist = %v, %v, want = the failed release", bl, err)
	}
	if e := bl.GetEntries()[0]; e.GetSourceTitle() != matrix720 || e.GetProtocol() != acquisitionv1.DownloadProtocol_DOWNLOAD_PROTOCOL_TORRENT || e.GetIndexerName() != "Indexer" {
		t.Errorf("blocklist entry = %v", e)
	}
	// Decisions reject the blocklisted release now.
	found, err = search.SearchReleases(ctx, acquisitionv1.SearchReleasesRequest_builder{ItemId: new(added.GetItemId())}.Build())
	if err != nil {
		t.Fatalf("SearchReleases: %v", err)
	}
	for _, r := range found.GetReleases() {
		if r.GetTitle() == matrix720 && !slices.ContainsFunc(r.GetRejections(), func(x *acquisitionv1.Rejection) bool { return x.GetReason() == "blocklisted" }) {
			t.Errorf("rejections of the blocklisted release = %v, want = blocklisted", r.GetRejections())
		}
	}
	if _, err := admin.DeleteBlocklist(ctx, acquisitionv1.DeleteBlocklistRequest_builder{Ids: []string{bl.GetEntries()[0].GetId()}}.Build()); err != nil {
		t.Fatalf("DeleteBlocklist: %v", err)
	}
	if bl, _ := admin.ListBlocklist(ctx, &acquisitionv1.ListBlocklistRequest{}); bl.GetTotal() != 0 {
		t.Errorf("blocklist after delete = %d, want = 0", bl.GetTotal())
	}
	_, err = user.DeleteBlocklist(ctx, acquisitionv1.DeleteBlocklistRequest_builder{Ids: []string{core.NewID().String()}}.Build())
	wantCode(t, "DeleteBlocklist as user", err, connect.CodePermissionDenied)

	// Removing from the queue ignores the download.
	grabbed, err = admin.GrabRelease(ctx, acquisitionv1.GrabReleaseRequest_builder{ItemId: new(added.GetItemId()), Release: rels[0]}.Build())
	if err != nil {
		t.Fatalf("GrabRelease: %v", err)
	}
	if _, err := admin.RemoveFromQueue(ctx, acquisitionv1.RemoveFromQueueRequest_builder{Id: new(grabbed.GetItem().GetId()), RemoveFromClient: new(true)}.Build()); err != nil {
		t.Fatalf("RemoveFromQueue: %v", err)
	}
	f.client.mu.Lock()
	removed := slices.Clone(f.client.removed)
	f.client.mu.Unlock()
	if !slices.Equal(removed, []string{hashA}) {
		t.Errorf("removed from the client = %v, want = [%s]", removed, hashA)
	}
	ignored, err := admin.ListHistory(ctx, acquisitionv1.ListHistoryRequest_builder{
		Types: []acquisitionv1.HistoryEventType{acquisitionv1.HistoryEventType_HISTORY_EVENT_TYPE_IGNORED},
	}.Build())
	if err != nil || ignored.GetTotal() != 1 {
		t.Errorf("ignored events = %v, %v, want = 1", ignored, err)
	}
	_, err = admin.RemoveFromQueue(ctx, acquisitionv1.RemoveFromQueueRequest_builder{Id: new(core.NewID().String())}.Build())
	wantCode(t, "RemoveFromQueue of an unknown download", err, connect.CodeNotFound)
	_, err = user.ListQueue(ctx, &acquisitionv1.ListQueueRequest{})
	wantCode(t, "ListQueue as user", err, connect.CodePermissionDenied)
}
