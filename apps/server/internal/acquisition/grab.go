package acquisition

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/mavioai/mavio/libs/acquire"
	"github.com/mavioai/mavio/libs/acquire/series"
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
	dataTvdbID             = "tvdbId"
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

// grabRequest is a release to send to a download client for a movie, or
// for episodes of a series.
type grabRequest struct {
	movie *movie
	// show and episodes are set for episode releases, in place of movie.
	show      *show
	episodes  []*series.Episode
	rel       *Release
	candidate *acquire.Candidate
	// pluginID and clientID name the client; empty for the first of the
	// release's protocol.
	pluginID, clientID string
	// fullSeason is set for releases of full seasons, which seed for the
	// indexer's season pack seed time.
	fullSeason bool
	// filterBlocked is set for releases pending until a download client
	// is back: they wait while every client has failed recently.
	filterBlocked bool
}

// seedSettings returns how long a torrent release seeds, as Radarr's and
// Sonarr's SeedConfigProvider: the indexer's seed ratio and seed time, the
// season pack seed time for full seasons; else what the release asks.
func (rn *run) seedSettings(ctx context.Context, rel *Release, fullSeason bool) (ratio float64, seedTime time.Duration) {
	ratio, seedTime = rel.MinimumRatio, rel.MinimumSeedTime
	if rn == nil {
		return ratio, seedTime
	}
	ix, ok := rn.indexer(ctx, rel.IndexerRef())
	if !ok {
		return ratio, seedTime
	}
	if ix.HasSeedRatio() {
		ratio = ix.GetSeedRatio()
	}
	switch {
	case fullSeason && ix.HasSeasonPackSeedTime():
		seedTime = ix.GetSeasonPackSeedTime().AsDuration()
	case !fullSeason && ix.HasSeedTime():
		seedTime = ix.GetSeedTime().AsDuration()
	}
	return ratio, seedTime
}

// wantedTags returns the tags of a wanted movie; none for nil.
func (m *movie) wantedTags() []string {
	if m == nil {
		return nil
	}
	return m.wanted.Tags
}

// details returns what a grab tells download clients of its release and
// its movie or series, which Radarr's and Sonarr's Flood tag downloads
// with: the quality, languages, release group and indexer of the
// release, the year and studio of the movie or the network of the
// series, the movie's collection and the series' title as a slug.
func (g grabRequest) details(rel *Release, c *acquire.Candidate) *pluginv1.DownloadDetails {
	d := pluginv1.DownloadDetails_builder{
		Quality: new(c.Quality.Quality.String()), Languages: languageNames(c.Languages),
		ReleaseGroup: new(c.ReleaseGroup), Indexer: new(rel.IndexerName),
	}
	it := g.show.itemOrZero()
	if g.movie != nil {
		it = g.movie.item
		d.Collection = new(it.CollectionName)
	} else {
		d.TitleSlug = new(release.URLSlug(it.Name, true, "-", "-"))
	}
	d.Year = new(int32(it.ProductionYear))
	if len(it.Studios) > 0 {
		d.Studio = new(it.Studios[0])
	}
	return d.Build()
}

// itemOrZero returns a series' item; the zero item for nil.
func (sh *show) itemOrZero() core.Item {
	if sh == nil {
		return core.Item{}
	}
	return sh.item
}

// infoHash returns a download ID that is a torrent's info hash, in upper
// case; "" for other IDs.
func infoHash(id string) string {
	if len(id) != 40 || strings.Trim(strings.ToLower(id), "0123456789abcdef") != "" {
		return ""
	}
	return strings.ToUpper(id)
}

// blocklistedHashes returns the info hashes blocklisted for the movie or
// series of a torrent grab whose hash the release did not give, for the
// client to refuse a torrent among them, as Radarr's
// TorrentClientBase.EnsureReleaseIsNotBlocklisted: only for releases not
// from an interactive search, of indexers set to reject them.
func (rn *run) blocklistedHashes(ctx context.Context, rel *Release, c *acquire.Candidate, itemID core.ID) ([]string, error) {
	if rn == nil || c.Source == acquire.SourceInteractiveSearch {
		return nil, nil
	}
	if ix, ok := rn.indexer(ctx, rel.IndexerRef()); !ok || !ix.GetRejectBlocklistedTorrentHashes() {
		return nil, nil
	}
	var hashes []string
	for offset := 0; ; {
		page, err := rn.rt.Store.Blocklist().List(ctx, itemID, core.MaxPageSize, offset)
		if err != nil {
			return nil, fmt.Errorf("blocklisted hashes: %w", err)
		}
		for _, b := range page.Items {
			if b.TorrentInfoHash != "" && !slices.ContainsFunc(hashes, func(h string) bool { return strings.EqualFold(h, b.TorrentInfoHash) }) {
				hashes = append(hashes, b.TorrentInfoHash)
			}
		}
		offset += len(page.Items)
		if len(page.Items) == 0 || offset >= page.Total {
			return hashes, nil
		}
	}
}

// recentEpisodes reports whether any of episodes aired in the last two
// weeks or is to air, as Sonarr's RemoteEpisode.IsRecentEpisode.
func recentEpisodes(episodes []*series.Episode, now time.Time) bool {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	return slices.ContainsFunc(episodes, func(e *series.Episode) bool {
		return !e.AirDateUTC.IsZero() && !e.AirDateUTC.Before(today.AddDate(0, 0, -14))
	})
}

// grab sends a release to a download client and tracks the download, as
// Radarr's and Sonarr's DownloadService.DownloadReport: it records the
// grab in the history, once per episode for episode releases, and drops
// the pending releases it stands for.
func (r *Runtime) grab(ctx context.Context, rn *run, g grabRequest) (core.Download, error) {
	rel, c := g.rel, g.candidate
	if rel == nil || c == nil || (g.movie == nil && (g.show == nil || len(g.episodes) == 0)) {
		return core.Download{}, fmt.Errorf("%w: nothing to grab", core.ErrInvalid)
	}
	tags := g.movie.wantedTags()
	if g.show != nil {
		tags = g.show.wanted.Tags
	}
	ch := clientChoice{
		protocol: rel.Protocol, tags: tags, pluginID: g.pluginID, clientID: g.clientID,
		indexer: rel.IndexerName, filterBlocked: g.filterBlocked,
	}
	if rn != nil {
		if ix, ok := rn.indexer(ctx, rel.IndexerRef()); ok {
			ch.indexerClient = ix.GetDownloadClientId()
		}
	}
	client, err := r.chooseClient(ctx, ch)
	if err != nil {
		return core.Download{}, err
	}
	api := r.clientAPI(client.PluginID)
	if api == nil {
		return core.Download{}, fmt.Errorf("%w: plugin %s is not ready", errClientUnavailable, client.PluginID)
	}
	now := r.now()
	kind, recent := pluginv1.DownloadKind_DOWNLOAD_KIND_MOVIE, false
	var itemID core.ID
	var itemIDs []core.ID
	if g.show != nil {
		kind, recent, itemID = pluginv1.DownloadKind_DOWNLOAD_KIND_SERIES, recentEpisodes(g.episodes, now), g.show.item.ID
		for _, ep := range g.episodes {
			itemIDs = append(itemIDs, ep.ID)
		}
	} else {
		recent, itemID, itemIDs = g.movie.acq.Availability.Recent(now), g.movie.item.ID, []core.ID{g.movie.item.ID}
	}
	req := pluginv1.AddDownloadRequest_builder{
		ClientId: new(client.GetId()), Title: new(rel.Title), Kind: new(kind), Recent: new(recent),
		FullSeason: new(g.fullSeason), Details: g.details(rel, c),
	}.Build()
	if rel.Protocol == core.ProtocolTorrent {
		if rel.InfoHash != "" {
			req.SetInfoHash(rel.InfoHash)
		} else if hashes, err := rn.blocklistedHashes(ctx, rel, c, itemID); err != nil {
			return core.Download{}, err
		} else {
			req.SetBlocklistedInfoHashes(hashes)
		}
		ratio, seedTime := rn.seedSettings(ctx, rel, g.fullSeason)
		if ratio > 0 {
			req.SetSeedRatio(ratio)
		}
		if seedTime > 0 {
			req.SetSeedTime(durationpb.New(seedTime))
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
		case connect.CodeFailedPrecondition:
			// The client refused the release, as Radarr's
			// ReleaseBlockedException and ReleaseDownloadException: the
			// release fails, not the client.
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
		ItemID: itemID, ItemIDs: itemIDs, Title: rel.Title,
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
	}
	if g.show != nil {
		data[dataTvdbID], data[dataImdbID] = g.show.item.ExternalIDs[core.ProviderTVDB], g.show.item.ExternalIDs[core.ProviderIMDb]
	} else {
		data[dataTmdbID], data[dataImdbID] = g.movie.item.ExternalIDs[core.ProviderTMDB], g.movie.item.ExternalIDs[core.ProviderIMDb]
	}
	if !rel.PublishDate.IsZero() {
		data[dataPublishedDate] = rel.PublishDate.UTC().Format(time.RFC3339)
	}
	if rel.Protocol == core.ProtocolTorrent {
		// A hash the release did not give is the client's, as Radarr
		// learns it from the torrent while grabbing.
		data[dataTorrentInfoHash] = cmp.Or(rel.InfoHash, infoHash(d.DownloadID))
	}
	for k, v := range data {
		if v == "" {
			delete(data, k)
		}
	}
	var events []core.HistoryEvent
	if g.show != nil {
		// History events are of items: the episodes a multi-episode file
		// covers are recorded as the file's item.
		seen := map[core.ID]bool{}
		for _, ep := range g.episodes {
			if h := rn.holderOf(ep.ID); !seen[h] {
				seen[h] = true
				events = append(events, core.HistoryEvent{
					ID: core.NewID(), Type: core.HistoryGrabbed, ItemID: h, ParentID: itemID, Date: now, SourceTitle: rel.Title,
					Quality: d.Quality, Languages: d.Languages, DownloadID: d.DownloadID, Data: data,
				})
			}
		}
	} else {
		events = []core.HistoryEvent{{
			ID: core.NewID(), Type: core.HistoryGrabbed, ItemID: d.ItemID, Date: now, SourceTitle: rel.Title,
			Quality: d.Quality, Languages: d.Languages, DownloadID: d.DownloadID, Data: data,
		}}
	}
	if err := r.Store.History().Add(ctx, events...); err != nil {
		return d, err
	}
	if g.show != nil {
		if err := r.removeGrabbedPending(ctx, g.show, itemIDs, c.Quality); err != nil {
			return d, err
		}
	} else if err := r.removePending(ctx, d.ItemID, nil); err != nil {
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
// wanted movie, or for episodes of a wanted series given it, a season or
// an episode, whatever the decisions on it, and tracks it, as Radarr's
// and Sonarr's ReleaseController do. pluginID and clientID name the
// client; empty for the first of the release's protocol.
func (r *Runtime) GrabRelease(ctx context.Context, itemID core.ID, rel *Release, pluginID, clientID string) (core.Download, error) {
	rn, err := r.newRun(ctx)
	if err != nil {
		return core.Download{}, err
	}
	if !rel.Protocol.Valid() {
		return core.Download{}, fmt.Errorf("%w: release has no protocol", core.ErrInvalid)
	}
	it, err := r.Store.Items().Get(ctx, itemID)
	if err != nil {
		return core.Download{}, err
	}
	if isEpisodeKind(it.Kind) {
		d, err := r.grabEpisodeRelease(ctx, rn, it, rel, pluginID, clientID)
		if errors.Is(err, errReleaseUnavailable) || errors.Is(err, errClientUnavailable) {
			return d, fmt.Errorf("%w: %w", core.ErrConflict, err)
		}
		return d, err
	}
	m, err := rn.movie(ctx, itemID)
	if err != nil {
		return core.Download{}, err
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
