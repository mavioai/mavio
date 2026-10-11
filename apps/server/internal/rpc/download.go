package rpc

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/mavioai/mavio/apps/server/internal/acquisition"
	"github.com/mavioai/mavio/libs/acquire"
	"github.com/mavioai/mavio/libs/core"
	acquisitionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1/acquisitionv1connect"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

// DownloadService implements mavio.acquisition.v1.DownloadService.
type DownloadService struct {
	store core.Store
	rt    *acquisition.Runtime
}

var _ acquisitionv1connect.DownloadServiceHandler = (*DownloadService)(nil)

// NewDownloadService returns a DownloadService tracking downloads in
// store with rt.
func NewDownloadService(store core.Store, rt *acquisition.Runtime) *DownloadService {
	return &DownloadService{store: store, rt: rt}
}

// ListDownloadClients lists the clients of every ready provider.
func (s *DownloadService) ListDownloadClients(ctx context.Context, _ *acquisitionv1.ListDownloadClientsRequest) (*acquisitionv1.ListDownloadClientsResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	clients, failures := s.rt.ListClients(ctx)
	out := make([]*acquisitionv1.DownloadClient, len(clients))
	for i, c := range clients {
		out[i] = acquisitionv1.DownloadClient_builder{
			PluginId: new(c.PluginID), Id: new(c.GetId()), Name: new(c.GetName()), DefinitionId: new(c.GetDefinitionId()),
			Protocol: new(acquisitionv1.DownloadProtocol(c.GetProtocol())), Enabled: new(c.GetEnabled()),
			Priority: new(c.GetPriority()), DisabledUntil: c.GetDisabledUntil(), Failure: new(c.GetFailure()),
		}.Build()
	}
	return acquisitionv1.ListDownloadClientsResponse_builder{Clients: out, Failures: failures}.Build(), nil
}

// GrabRelease sends a release to a download client for a movie or
// episodes.
func (s *DownloadService) GrabRelease(ctx context.Context, req *acquisitionv1.GrabReleaseRequest) (*acquisitionv1.GrabReleaseResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	id, err := core.ParseID(req.GetItemId())
	if err != nil {
		return nil, connectError(ctx, err)
	}
	d, err := s.rt.GrabRelease(ctx, id, releaseFromProto(req.GetRelease()), req.GetClientPluginId(), req.GetClientId())
	if err != nil {
		return nil, failedPrecondition(ctx, err)
	}
	item, err := s.queueItem(ctx, d, map[core.ID]string{})
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return acquisitionv1.GrabReleaseResponse_builder{Item: item}.Build(), nil
}

// releaseFromProto returns the release a client passed back.
func releaseFromProto(r *acquisitionv1.Release) *acquisition.Release {
	out := &acquisition.Release{
		PluginID: r.GetPluginId(), IndexerID: r.GetIndexerId(), IndexerName: r.GetIndexerName(),
		GUID: r.GetGuid(), Title: r.GetTitle(), Size: r.GetSize(),
		DownloadURL: r.GetDownloadUrl(), MagnetURL: r.GetMagnetUrl(), InfoHash: r.GetInfoHash(), InfoURL: r.GetInfoUrl(),
		Protocol: acquisition.ProtocolFromPlugin(pluginv1.DownloadProtocol(r.GetProtocol())), Flags: r.GetFlags(),
		ImdbID: r.GetExternalIds()["imdb"],
	}
	if r.HasPublishDate() {
		out.PublishDate = r.GetPublishDate().AsTime()
	}
	if id, err := strconv.Atoi(r.GetExternalIds()["tmdb"]); err == nil {
		out.TmdbID = id
	}
	if id, err := strconv.Atoi(r.GetExternalIds()["tvdb"]); err == nil {
		out.TvdbID = id
	}
	if id, err := strconv.Atoi(r.GetExternalIds()["tvrage"]); err == nil {
		out.TvRageID = id
	}
	if r.HasSeeders() {
		out.Seeders = new(int(r.GetSeeders()))
	}
	if r.HasPeers() {
		out.Peers = new(int(r.GetPeers()))
	}
	if p := r.GetParsed(); p != nil {
		out.Languages = p.GetLanguages()
	}
	return out
}

var downloadStates = map[core.DownloadState]acquisitionv1.DownloadState{
	core.DownloadDownloading:   acquisitionv1.DownloadState_DOWNLOAD_STATE_DOWNLOADING,
	core.DownloadImportBlocked: acquisitionv1.DownloadState_DOWNLOAD_STATE_IMPORT_BLOCKED,
	core.DownloadImportPending: acquisitionv1.DownloadState_DOWNLOAD_STATE_IMPORT_PENDING,
	core.DownloadImporting:     acquisitionv1.DownloadState_DOWNLOAD_STATE_IMPORTING,
	core.DownloadImported:      acquisitionv1.DownloadState_DOWNLOAD_STATE_IMPORTED,
	core.DownloadFailedPending: acquisitionv1.DownloadState_DOWNLOAD_STATE_FAILED_PENDING,
	core.DownloadFailed:        acquisitionv1.DownloadState_DOWNLOAD_STATE_FAILED,
	core.DownloadIgnored:       acquisitionv1.DownloadState_DOWNLOAD_STATE_IGNORED,
}

func protocolToProto(p core.Protocol) acquisitionv1.DownloadProtocol {
	for k, v := range downloadProtocols {
		if v == p {
			return k
		}
	}
	return acquisitionv1.DownloadProtocol_DOWNLOAD_PROTOCOL_UNSPECIFIED
}

// qualityName returns Radarr's name of a stored quality.
func qualityName(q core.QualityRef) string {
	rq, ok := acquire.QualityByID(q.Quality)
	if !ok {
		return ""
	}
	return rq.String()
}

// indexerRef returns the reference of an indexer named "<plugin>/<indexer>".
func indexerRef(id string) *acquisitionv1.IndexerRef {
	if id == "" {
		return nil
	}
	plugin, indexer, _ := strings.Cut(id, "/")
	return acquisitionv1.IndexerRef_builder{PluginId: new(plugin), IndexerId: new(indexer)}.Build()
}

// queueItem returns a download with its movie's name and what its client
// last told; names caches the names read.
func (s *DownloadService) queueItem(ctx context.Context, d core.Download, names map[core.ID]string) (*acquisitionv1.QueueItem, error) {
	name, ok := names[d.ItemID]
	if !ok {
		it, err := s.store.Items().Get(ctx, d.ItemID)
		if err != nil && !errors.Is(err, core.ErrNotFound) {
			return nil, err
		}
		name = it.Name
		names[d.ItemID] = name
	}
	b := acquisitionv1.QueueItem_builder{
		Id: new(d.ID.String()), ItemId: new(d.ItemID.String()), ItemName: new(name), Title: new(d.Title),
		ClientPluginId: new(d.PluginID), ClientId: new(d.ClientID), DownloadId: new(d.DownloadID),
		Indexer: indexerRef(d.IndexerID), IndexerName: new(d.Indexer), Protocol: new(protocolToProto(d.Protocol)),
		Quality: new(qualityName(d.Quality)), Languages: d.Languages, Size: new(d.Size),
		State: new(downloadStates[d.State]), Messages: d.Messages, GrabTime: timestamp(d.GrabbedAt),
	}
	if st, ok := s.rt.Status(d.ID); ok {
		b.Status = new(acquisitionv1.DownloadStatus(st.Status))
		b.RemainingSize = new(st.RemainingSize)
		if st.HasRemainingTime {
			b.RemainingTime = durationpb.New(st.RemainingTime)
		}
	}
	return b.Build(), nil
}

// ListQueue lists the tracked downloads.
func (s *DownloadService) ListQueue(ctx context.Context, req *acquisitionv1.ListQueueRequest) (*acquisitionv1.ListQueueResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	q := core.DownloadQuery{Unfinished: !req.GetIncludeFinished(), Limit: int(req.GetLimit()), Offset: int(req.GetOffset())}
	if req.HasItemId() {
		id, err := core.ParseID(req.GetItemId())
		if err != nil {
			return nil, connectError(ctx, err)
		}
		q.ItemID = id
	}
	page, err := s.store.Downloads().List(ctx, q)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	names := map[core.ID]string{}
	out := make([]*acquisitionv1.QueueItem, len(page.Items))
	for i, d := range page.Items {
		if out[i], err = s.queueItem(ctx, d, names); err != nil {
			return nil, connectError(ctx, err)
		}
	}
	return acquisitionv1.ListQueueResponse_builder{Items: out, Total: new(int32(page.Total))}.Build(), nil
}

// RemoveFromQueue stops tracking a download.
func (s *DownloadService) RemoveFromQueue(ctx context.Context, req *acquisitionv1.RemoveFromQueueRequest) (*acquisitionv1.RemoveFromQueueResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	id, err := core.ParseID(req.GetId())
	if err != nil {
		return nil, connectError(ctx, err)
	}
	if err := s.rt.RemoveFromQueue(ctx, id, req.GetRemoveFromClient(), req.GetBlocklist(), req.GetSearch()); err != nil {
		return nil, connectError(ctx, err)
	}
	return &acquisitionv1.RemoveFromQueueResponse{}, nil
}

var historyTypes = map[core.HistoryEventType]acquisitionv1.HistoryEventType{
	core.HistoryGrabbed:        acquisitionv1.HistoryEventType_HISTORY_EVENT_TYPE_GRABBED,
	core.HistoryImported:       acquisitionv1.HistoryEventType_HISTORY_EVENT_TYPE_IMPORTED,
	core.HistoryDownloadFailed: acquisitionv1.HistoryEventType_HISTORY_EVENT_TYPE_DOWNLOAD_FAILED,
	core.HistoryFileDeleted:    acquisitionv1.HistoryEventType_HISTORY_EVENT_TYPE_FILE_DELETED,
	core.HistoryFileRenamed:    acquisitionv1.HistoryEventType_HISTORY_EVENT_TYPE_FILE_RENAMED,
	core.HistoryIgnored:        acquisitionv1.HistoryEventType_HISTORY_EVENT_TYPE_IGNORED,
}

// ListHistory lists the acquisition history, newest first.
func (s *DownloadService) ListHistory(ctx context.Context, req *acquisitionv1.ListHistoryRequest) (*acquisitionv1.ListHistoryResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	q := core.HistoryQuery{Limit: int(req.GetLimit()), Offset: int(req.GetOffset())}
	if req.HasItemId() {
		id, err := core.ParseID(req.GetItemId())
		if err != nil {
			return nil, connectError(ctx, err)
		}
		q.ItemIDs = []core.ID{id}
	}
	for _, t := range req.GetTypes() {
		for k, v := range historyTypes {
			if v == t {
				q.Types = append(q.Types, k)
			}
		}
	}
	page, err := s.store.History().List(ctx, q)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	out := make([]*acquisitionv1.HistoryEvent, len(page.Items))
	for i, h := range page.Items {
		out[i] = acquisitionv1.HistoryEvent_builder{
			Id: new(h.ID.String()), Type: new(historyTypes[h.Type]), ItemId: new(h.ItemID.String()), Time: timestamp(h.Date),
			SourceTitle: new(h.SourceTitle), Quality: new(qualityName(h.Quality)), Languages: h.Languages,
			DownloadId: new(h.DownloadID), Data: h.Data,
		}.Build()
	}
	return acquisitionv1.ListHistoryResponse_builder{Events: out, Total: new(int32(page.Total))}.Build(), nil
}

// MarkFailed marks the download of a grab as failed.
func (s *DownloadService) MarkFailed(ctx context.Context, req *acquisitionv1.MarkFailedRequest) (*acquisitionv1.MarkFailedResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	id, err := core.ParseID(req.GetHistoryEventId())
	if err != nil {
		return nil, connectError(ctx, err)
	}
	if err := s.rt.MarkFailed(ctx, id); err != nil {
		return nil, connectError(ctx, err)
	}
	return &acquisitionv1.MarkFailedResponse{}, nil
}

// ListBlocklist lists the blocklisted releases, newest first.
func (s *DownloadService) ListBlocklist(ctx context.Context, req *acquisitionv1.ListBlocklistRequest) (*acquisitionv1.ListBlocklistResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	var itemID core.ID
	if req.HasItemId() {
		id, err := core.ParseID(req.GetItemId())
		if err != nil {
			return nil, connectError(ctx, err)
		}
		itemID = id
	}
	page, err := s.store.Blocklist().List(ctx, itemID, int(req.GetLimit()), int(req.GetOffset()))
	if err != nil {
		return nil, connectError(ctx, err)
	}
	out := make([]*acquisitionv1.BlocklistEntry, len(page.Items))
	for i, b := range page.Items {
		e := acquisitionv1.BlocklistEntry_builder{
			Id: new(b.ID.String()), ItemId: new(b.ItemID.String()), SourceTitle: new(b.SourceTitle), Quality: new(qualityName(b.Quality)),
			Languages: b.Languages, Time: timestamp(b.Date), Size: new(b.Size), Protocol: new(protocolToProto(b.Protocol)),
			Indexer: indexerRef(b.IndexerID), IndexerName: new(b.Indexer), Message: new(b.Message),
		}
		if !b.PublishedDate.IsZero() {
			e.PublishTime = timestamp(b.PublishedDate)
		}
		out[i] = e.Build()
	}
	return acquisitionv1.ListBlocklistResponse_builder{Entries: out, Total: new(int32(page.Total))}.Build(), nil
}

// DeleteBlocklist deletes blocklist entries.
func (s *DownloadService) DeleteBlocklist(ctx context.Context, req *acquisitionv1.DeleteBlocklistRequest) (*acquisitionv1.DeleteBlocklistResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	ids, err := parseIDs(ctx, req.GetIds())
	if err != nil {
		return nil, err
	}
	if err := s.store.Blocklist().Delete(ctx, ids...); err != nil {
		return nil, connectError(ctx, err)
	}
	return &acquisitionv1.DeleteBlocklistResponse{}, nil
}
