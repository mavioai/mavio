package app

import (
	"context"
	"slices"
	"sync"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	acquisitionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1/acquisitionv1connect"
)

// fakeDownloads serves a fake server's download clients, tracked
// downloads, history and blocklist, and keeps what was done to them.
type fakeDownloads struct {
	acquisitionv1connect.UnimplementedDownloadServiceHandler

	mu        sync.Mutex
	queue     []*acquisitionv1.QueueItem
	history   []*acquisitionv1.HistoryEvent
	blocklist []*acquisitionv1.BlocklistEntry
	removed   []*acquisitionv1.RemoveFromQueueRequest
	failed    []string
	imported  []*acquisitionv1.ManualImportRequest
	grabbed   []string // the releases grabbed, by item and title
	// paths are qBittorrent's folders and where the server reads them.
	paths []*acquisitionv1.DownloadClientPath
}

func newFakeDownloads() *fakeDownloads {
	now := time.Now()
	return &fakeDownloads{
		paths: []*acquisitionv1.DownloadClientPath{
			acquisitionv1.DownloadClientPath_builder{
				RemotePath: new("/downloads/"), LocalPath: new("/mnt/seedbox/"), RootFolder: new(true), Readable: new(true),
				LinkLibraryPaths: []string{"/media/movies"},
			}.Build(),
			acquisitionv1.DownloadClientPath_builder{
				RemotePath: new("/incomplete/"), SuggestedPath: new("/mnt/incomplete/"), Error: new("not found"),
			}.Build(),
		},
		queue: []*acquisitionv1.QueueItem{
			acquisitionv1.QueueItem_builder{
				Id: new("q1"), ItemId: new("dune"), ItemName: new("Dune"), Title: new("Dune.2021.1080p.BluRay.x265-GRP"), IndexerName: new("Nyaa"),
				Quality: new("Bluray-1080p"), Languages: []string{"English"}, Size: new(int64(4_200_000_000)), RemainingSize: new(int64(2_100_000_000)),
				State: new(acquisitionv1.DownloadState_DOWNLOAD_STATE_DOWNLOADING), RemainingTime: durationpb.New(25 * time.Minute),
				GrabTime: timestamppb.New(now.Add(-2 * time.Hour)),
			}.Build(),
			acquisitionv1.QueueItem_builder{
				Id: new("q2"), ItemId: new("arrival"), ItemName: new("Arrival"), Title: new("Arrival.2016.2160p.WEB-DL"), Quality: new("WEBDL-2160p"),
				Size: new(int64(350_000_000)), State: new(acquisitionv1.DownloadState_DOWNLOAD_STATE_IMPORT_BLOCKED),
				Messages: []string{"No files found are eligible for import"},
			}.Build(),
		},
		history: []*acquisitionv1.HistoryEvent{
			acquisitionv1.HistoryEvent_builder{
				Id: new("h1"), Type: new(acquisitionv1.HistoryEventType_HISTORY_EVENT_TYPE_GRABBED), ItemId: new("dune"),
				SourceTitle: new("Dune.2021.1080p.BluRay.x265-GRP"), Quality: new("Bluray-1080p"), Time: timestamppb.New(now.Add(-2 * time.Hour)),
				Data: map[string]string{"indexer": "Nyaa"},
			}.Build(),
			acquisitionv1.HistoryEvent_builder{
				Id: new("h2"), Type: new(acquisitionv1.HistoryEventType_HISTORY_EVENT_TYPE_IMPORTED), SourceTitle: new("Heat.1995.720p"),
				Time: timestamppb.New(now.Add(-48 * time.Hour)),
			}.Build(),
		},
		blocklist: []*acquisitionv1.BlocklistEntry{acquisitionv1.BlocklistEntry_builder{
			Id: new("b1"), ItemId: new("dune"), SourceTitle: new("Dune.2021.CAM"), Quality: new("CAM"), Message: new("Manually marked failed"),
			Time: timestamppb.New(now.Add(-24 * time.Hour)),
		}.Build()},
	}
}

func (*fakeDownloads) ListDownloadClients(context.Context, *acquisitionv1.ListDownloadClientsRequest) (*acquisitionv1.ListDownloadClientsResponse, error) {
	return acquisitionv1.ListDownloadClientsResponse_builder{Clients: []*acquisitionv1.DownloadClient{
		acquisitionv1.DownloadClient_builder{
			PluginId: new("dev.mavio.mail"), Id: new("q"), Name: new("qBittorrent"), Enabled: new(false), Priority: new(int32(1)),
			Protocol: new(acquisitionv1.DownloadProtocol_DOWNLOAD_PROTOCOL_TORRENT),
		}.Build(),
	}}.Build(), nil
}

func (s *fakeDownloads) GetDownloadClientPaths(context.Context, *acquisitionv1.GetDownloadClientPathsRequest) (*acquisitionv1.GetDownloadClientPathsResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return acquisitionv1.GetDownloadClientPathsResponse_builder{Paths: slices.Clone(s.paths)}.Build(), nil
}

// UpdateDownloadClientPaths keeps the paths given, which the server reads
// from then on, and the root folder.
func (s *fakeDownloads) UpdateDownloadClientPaths(_ context.Context, r *acquisitionv1.UpdateDownloadClientPathsRequest) (*acquisitionv1.UpdateDownloadClientPathsResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var next []*acquisitionv1.DownloadClientPath
	for _, p := range s.paths {
		if p.GetRootFolder() && !slices.ContainsFunc(r.GetPaths(), func(x *acquisitionv1.DownloadClientPath) bool { return x.GetRemotePath() == p.GetRemotePath() }) {
			next = append(next, acquisitionv1.DownloadClientPath_builder{RemotePath: new(p.GetRemotePath()), RootFolder: new(true)}.Build())
		}
	}
	for _, p := range r.GetPaths() {
		root := slices.ContainsFunc(s.paths, func(x *acquisitionv1.DownloadClientPath) bool {
			return x.GetRootFolder() && x.GetRemotePath() == p.GetRemotePath()
		})
		next = append(next, acquisitionv1.DownloadClientPath_builder{
			RemotePath: new(p.GetRemotePath()), LocalPath: new(p.GetLocalPath()), RootFolder: new(root), Readable: new(true),
		}.Build())
	}
	s.paths = next
	return acquisitionv1.UpdateDownloadClientPathsResponse_builder{Paths: slices.Clone(next)}.Build(), nil
}

func (s *fakeDownloads) ListQueue(_ context.Context, r *acquisitionv1.ListQueueRequest) (*acquisitionv1.ListQueueResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := slices.DeleteFunc(slices.Clone(s.queue), func(q *acquisitionv1.QueueItem) bool {
		return r.GetItemId() != "" && q.GetItemId() != r.GetItemId()
	})
	return acquisitionv1.ListQueueResponse_builder{Items: items, Total: new(int32(len(items)))}.Build(), nil
}

func (s *fakeDownloads) GrabRelease(_ context.Context, r *acquisitionv1.GrabReleaseRequest) (*acquisitionv1.GrabReleaseResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.grabbed = append(s.grabbed, r.GetItemId()+" "+r.GetRelease().GetTitle())
	return acquisitionv1.GrabReleaseResponse_builder{ClientName: new("qBittorrent")}.Build(), nil
}

func (s *fakeDownloads) RemoveFromQueue(_ context.Context, r *acquisitionv1.RemoveFromQueueRequest) (*acquisitionv1.RemoveFromQueueResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.removed = append(s.removed, r)
	s.queue = slices.DeleteFunc(s.queue, func(q *acquisitionv1.QueueItem) bool { return q.GetId() == r.GetId() })
	return &acquisitionv1.RemoveFromQueueResponse{}, nil
}

func (s *fakeDownloads) ListHistory(_ context.Context, r *acquisitionv1.ListHistoryRequest) (*acquisitionv1.ListHistoryResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	events := slices.DeleteFunc(slices.Clone(s.history), func(e *acquisitionv1.HistoryEvent) bool {
		return len(r.GetTypes()) > 0 && !slices.Contains(r.GetTypes(), e.GetType()) || r.GetItemId() != "" && e.GetItemId() != r.GetItemId()
	})
	return acquisitionv1.ListHistoryResponse_builder{Events: events, Total: new(int32(len(events)))}.Build(), nil
}

func (s *fakeDownloads) MarkFailed(_ context.Context, r *acquisitionv1.MarkFailedRequest) (*acquisitionv1.MarkFailedResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failed = append(s.failed, r.GetHistoryEventId())
	return &acquisitionv1.MarkFailedResponse{}, nil
}

func (s *fakeDownloads) ListBlocklist(context.Context, *acquisitionv1.ListBlocklistRequest) (*acquisitionv1.ListBlocklistResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return acquisitionv1.ListBlocklistResponse_builder{Entries: slices.Clone(s.blocklist), Total: new(int32(len(s.blocklist)))}.Build(), nil
}

func (s *fakeDownloads) DeleteBlocklist(_ context.Context, r *acquisitionv1.DeleteBlocklistRequest) (*acquisitionv1.DeleteBlocklistResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.blocklist = slices.DeleteFunc(s.blocklist, func(e *acquisitionv1.BlocklistEntry) bool { return slices.Contains(r.GetIds(), e.GetId()) })
	return &acquisitionv1.DeleteBlocklistResponse{}, nil
}

func (s *fakeDownloads) ListManualImport(_ context.Context, r *acquisitionv1.ListManualImportRequest) (*acquisitionv1.ListManualImportResponse, error) {
	if r.GetFolder() != "" {
		return acquisitionv1.ListManualImportResponse_builder{Files: []*acquisitionv1.ManualImportFile{
			acquisitionv1.ManualImportFile_builder{
				Path: new(r.GetFolder() + "/Unknown.mkv"), RelativePath: new("Unknown.mkv"), Size: new(int64(900_000_000)), Rejections: []string{"Unknown Movie"},
			}.Build(),
		}}.Build(), nil
	}
	return acquisitionv1.ListManualImportResponse_builder{Files: []*acquisitionv1.ManualImportFile{
		acquisitionv1.ManualImportFile_builder{
			Path: new("/downloads/Arrival/Arrival.mkv"), RelativePath: new("Arrival.mkv"), Size: new(int64(2_500_000_000)), Quality: new("WEBDL-2160p"),
			MovieId: new("arrival"), ReleaseGroup: new("GRP"),
		}.Build(),
	}}.Build(), nil
}

func (s *fakeDownloads) ManualImport(_ context.Context, r *acquisitionv1.ManualImportRequest) (*acquisitionv1.ManualImportResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.imported = append(s.imported, r)
	return &acquisitionv1.ManualImportResponse{}, nil
}
