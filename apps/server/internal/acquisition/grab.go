package acquisition

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/mavioai/mavio/libs/acquire"
	"github.com/mavioai/mavio/libs/core"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/release"
)

var (
	// errReleaseUnavailable is returned when the indexer no longer has a
	// release, as Radarr's ReleaseUnavailableException.
	errReleaseUnavailable = errors.New("release unavailable")
	// errClientUnavailable is returned when no download client takes a
	// release now, as Radarr's DownloadClientUnavailableException.
	errClientUnavailable = errors.New("download client unavailable")
)

// The keys of history event data, Radarr's.
const (
	dataIndexer            = "indexer"
	dataIndexerID          = "indexerId"
	dataDownloadClient     = "downloadClient"
	dataDownloadClientName = "downloadClientName"
	dataProtocol           = "protocol"
	dataPublishedDate      = "publishedDate"
	dataSize               = "size"
	dataGUID               = "guid"
	dataDownloadURL        = "downloadUrl"
	dataInfoURL            = "nzbInfoUrl"
	dataReleaseGroup       = "releaseGroup"
	dataTorrentInfoHash    = "torrentInfoHash"
	dataIndexerFlags       = "indexerFlags"
	dataCustomFormatScore  = "customFormatScore"
	dataReleaseSource      = "releaseSource"
	dataTmdbID             = "tmdbId"
	dataImdbID             = "imdbId"
	dataMessage            = "message"
	dataReason             = "reason"
	dataImportedPath       = "importedPath"
	dataDroppedPath        = "droppedPath"
	dataSceneName          = "sceneName"
)

var releaseSources = map[acquire.Source]string{
	acquire.SourceRSS:               "Rss",
	acquire.SourceSearch:            "Search",
	acquire.SourceUserInvokedSearch: "UserInvokedSearch",
	acquire.SourceInteractiveSearch: "InteractiveSearch",
	acquire.SourceReleasePush:       "ReleasePush",
}

// grabRequest is a release to send to a download client for a movie.
type grabRequest struct {
	movie     *movie
	rel       *Release
	candidate *acquire.Candidate
	// pluginID and clientID name the client; empty for the first of the
	// release's protocol.
	pluginID, clientID string
}

// grab sends a release to a download client and tracks the download, as
// Radarr's DownloadService.DownloadReport: it records the grab in the
// history and drops the movie's pending releases.
func (r *Runtime) grab(ctx context.Context, rn *run, g grabRequest) (core.Download, error) {
	rel, c := g.rel, g.candidate
	if rel == nil || c == nil {
		return core.Download{}, fmt.Errorf("%w: nothing to grab", core.ErrInvalid)
	}
	client, err := r.chooseClient(ctx, rel.Protocol, g.pluginID, g.clientID)
	if err != nil {
		return core.Download{}, err
	}
	api := r.clientAPI(client.PluginID)
	if api == nil {
		return core.Download{}, fmt.Errorf("%w: plugin %s is not ready", errClientUnavailable, client.PluginID)
	}
	now := r.now()
	req := pluginv1.AddDownloadRequest_builder{
		ClientId: new(client.GetId()), Title: new(rel.Title), Kind: new(pluginv1.DownloadKind_DOWNLOAD_KIND_MOVIE),
		Recent: new(g.movie.acq.Availability.Recent(now)),
	}.Build()
	if rel.Protocol == core.ProtocolTorrent {
		if rel.InfoHash != "" {
			req.SetInfoHash(rel.InfoHash)
		}
		if rel.MinimumRatio > 0 {
			req.SetSeedRatio(rel.MinimumRatio)
		}
		if rel.MinimumSeedTime > 0 {
			req.SetSeedTime(durationpb.New(rel.MinimumSeedTime))
		}
	}
	var resp *pluginv1.AddDownloadResponse
	if rel.Protocol == core.ProtocolTorrent && rel.MagnetURL != "" && !client.GetPrefersTorrentFile() {
		req.SetMagnetUrl(rel.MagnetURL)
		resp, err = api.AddDownload(ctx, req)
		if connect.CodeOf(err) == connect.CodeUnimplemented && rel.DownloadURL != "" {
			req.ClearMagnetUrl()
			if err = r.releaseFile(ctx, rel, req); err == nil {
				resp, err = api.AddDownload(ctx, req)
			}
		}
	} else {
		if err = r.releaseFile(ctx, rel, req); err == nil {
			resp, err = api.AddDownload(ctx, req)
		}
	}
	if err != nil {
		if errors.Is(err, errReleaseUnavailable) {
			return core.Download{}, err
		}
		switch connect.CodeOf(err) {
		case connect.CodeUnavailable, connect.CodeUnauthenticated, connect.CodePermissionDenied, connect.CodeDeadlineExceeded:
			return core.Download{}, fmt.Errorf("%w: %s: %w", errClientUnavailable, client.GetName(), err)
		}
		return core.Download{}, fmt.Errorf("download client %s: %w", client.GetName(), err)
	}
	if resp.GetDownloadId() == "" {
		return core.Download{}, fmt.Errorf("download client %s returned no download ID", client.GetName())
	}

	d := core.Download{
		ID: core.NewID(), PluginID: client.PluginID, ClientID: client.GetId(), DownloadID: resp.GetDownloadId(),
		ItemID: g.movie.item.ID, ItemIDs: []core.ID{g.movie.item.ID}, Title: rel.Title,
		IndexerID: rel.IndexerRef(), Indexer: rel.IndexerName, Protocol: rel.Protocol,
		Quality: qualityRef(c.Quality), Languages: languageNames(c.Languages), Size: rel.Size,
		State: core.DownloadDownloading, GrabbedAt: now, UpdatedAt: now,
	}
	switch existing, err := r.Store.Downloads().GetByDownloadID(ctx, d.PluginID, d.ClientID, d.DownloadID); {
	case err == nil:
		d.ID = existing.ID
	case !errors.Is(err, core.ErrNotFound):
		return core.Download{}, err
	}
	if err := r.Store.Downloads().Put(ctx, &d); err != nil {
		return core.Download{}, err
	}
	data := map[string]string{
		dataIndexer: rel.IndexerName, dataIndexerID: rel.IndexerRef(),
		dataDownloadClient: client.Ref(), dataDownloadClientName: client.GetName(),
		dataProtocol: string(rel.Protocol), dataSize: strconv.FormatInt(rel.Size, 10), dataGUID: rel.GUID,
		dataDownloadURL: rel.DownloadURL, dataInfoURL: rel.InfoURL, dataReleaseGroup: c.ReleaseGroup,
		dataIndexerFlags: strings.Join(rel.Flags, ","), dataCustomFormatScore: strconv.Itoa(c.FormatScore),
		dataReleaseSource: releaseSources[c.Source],
		dataTmdbID:        g.movie.item.ExternalIDs[core.ProviderTMDB], dataImdbID: g.movie.item.ExternalIDs[core.ProviderIMDb],
	}
	if !rel.PublishDate.IsZero() {
		data[dataPublishedDate] = rel.PublishDate.UTC().Format(time.RFC3339)
	}
	if rel.Protocol == core.ProtocolTorrent {
		data[dataTorrentInfoHash] = rel.InfoHash
	}
	for k, v := range data {
		if v == "" {
			delete(data, k)
		}
	}
	if err := r.Store.History().Add(ctx, core.HistoryEvent{
		ID: core.NewID(), Type: core.HistoryGrabbed, ItemID: d.ItemID, Date: now, SourceTitle: rel.Title,
		Quality: d.Quality, Languages: d.Languages, DownloadID: d.DownloadID, Data: data,
	}); err != nil {
		return d, err
	}
	if err := r.removePending(ctx, d.ItemID, nil); err != nil {
		return d, err
	}
	r.logger().InfoContext(ctx, "grabbed release", "release", rel.Title, "item", d.ItemID, "client", client.GetName(), "indexer", rel.IndexerName)
	return d, nil
}

// releaseFile fetches a release's torrent file or NZB from its indexer
// into req, or the magnet link it leads to. A release its indexer no
// longer has fails with errReleaseUnavailable.
func (r *Runtime) releaseFile(ctx context.Context, rel *Release, req *pluginv1.AddDownloadRequest) error {
	if rel.DownloadURL == "" {
		return fmt.Errorf("%w: %s has no download link", errReleaseUnavailable, rel.Title)
	}
	api := r.indexerAPI(rel.PluginID)
	if api == nil {
		return fmt.Errorf("%w: indexer plugin %s is not ready", errReleaseUnavailable, rel.PluginID)
	}
	resp, err := api.GetReleaseFile(ctx, pluginv1.GetReleaseFileRequest_builder{
		IndexerId: new(rel.IndexerID), DownloadUrl: new(rel.DownloadURL),
	}.Build())
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			return fmt.Errorf("%w: %w", errReleaseUnavailable, err)
		}
		return fmt.Errorf("fetch %s: %w", rel.Title, err)
	}
	switch {
	case len(resp.GetData()) > 0:
		req.SetData(resp.GetData())
		req.SetFileName(resp.GetFileName())
	case resp.GetMagnetUrl() != "":
		req.SetMagnetUrl(resp.GetMagnetUrl())
	default:
		return fmt.Errorf("%w: %s led to nothing", errReleaseUnavailable, rel.Title)
	}
	return nil
}

// GrabRelease sends a release a search found to a download client for a
// wanted movie, whatever the decisions on it, and tracks it, as Radarr's
// ReleaseController does. pluginID and clientID name the client; empty
// for the first of the release's protocol.
func (r *Runtime) GrabRelease(ctx context.Context, itemID core.ID, rel *Release, pluginID, clientID string) (core.Download, error) {
	rn, err := r.newRun(ctx)
	if err != nil {
		return core.Download{}, err
	}
	m, err := rn.movie(ctx, itemID)
	if err != nil {
		return core.Download{}, err
	}
	if !rel.Protocol.Valid() {
		return core.Download{}, fmt.Errorf("%w: release has no protocol", core.ErrInvalid)
	}
	c := &acquire.Candidate{Release: rel.decisionRelease(), Source: acquire.SourceInteractiveSearch, Target: m.target, Items: []*acquire.Item{m.acq}}
	if parsed := release.ParseMovieTitle(rel.Title, false); parsed != nil {
		c.Quality, c.Languages, c.ReleaseGroup, c.Edition, c.Year = acquire.ModelOf(parsed.Quality), parsed.Languages, parsed.ReleaseGroup, parsed.Edition, parsed.Year
	} else {
		c.Quality, c.Languages = acquire.ModelOf(release.ParseQuality(rel.Title, release.Movie)), release.ParseLanguages(rel.Title, release.Movie)
	}
	if langs := languagesOf(rel.Languages); len(langs) > 0 {
		c.Languages = langs
	}
	c.Formats = rn.formats(rel.Title, c.Quality, c.Languages, rel.Size, rel.Flags)
	c.FormatScore = m.target.Profile.FormatScore(c.Formats)
	d, err := r.grab(ctx, rn, grabRequest{movie: m, rel: rel, candidate: c, pluginID: pluginID, clientID: clientID})
	if errors.Is(err, errReleaseUnavailable) || errors.Is(err, errClientUnavailable) {
		return d, fmt.Errorf("%w: %w", core.ErrConflict, err)
	}
	return d, err
}
