package acquisition

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mavioai/mavio/libs/acquire"
	"github.com/mavioai/mavio/libs/acquire/series"
	"github.com/mavioai/mavio/libs/core"
)

// msgManuallyFailed is the message of downloads marked failed by hand.
const msgManuallyFailed = "Manually marked as failed"

// failTracked handles a tracked download its client reports failed, as
// Radarr's FailedDownloadService.ProcessFailed: without a grab in the
// history it is only warned of.
func (r *Runtime) failTracked(ctx context.Context, d *core.Download, p polled, message string) error {
	grabs, err := r.grabEvents(ctx, d.DownloadID, d.ItemID)
	if errors.Is(err, core.ErrNotFound) {
		return r.warn(ctx, d, "Download has failed wasn't grabbed by Mavio, skipping automatic download handling")
	}
	if err != nil {
		return err
	}
	if err := r.markFailed(ctx, d, grabs, message, false, false); err != nil {
		return err
	}
	if p.client.GetRemoveFailedDownloads() && p.item.GetCanBeRemoved() {
		r.removeFromClient(ctx, d, true)
	}
	return nil
}

// grabEvent returns the latest grab of a download, of an item unless
// itemID is NilID.
func (r *Runtime) grabEvent(ctx context.Context, downloadID string, itemID core.ID) (core.HistoryEvent, error) {
	q := core.HistoryQuery{Types: []core.HistoryEventType{core.HistoryGrabbed}, DownloadID: downloadID, Limit: 1}
	if !itemID.IsZero() {
		q.ItemIDs = []core.ID{itemID}
	}
	page, err := r.Store.History().List(ctx, q)
	if err != nil {
		return core.HistoryEvent{}, err
	}
	if len(page.Items) == 0 {
		return core.HistoryEvent{}, fmt.Errorf("grab of download %s: %w", downloadID, core.ErrNotFound)
	}
	return page.Items[0], nil
}

// grabEvents returns the latest grab of a download, of an item unless
// itemID is NilID, and for episode releases the grabs of the download's
// other episodes after it.
func (r *Runtime) grabEvents(ctx context.Context, downloadID string, itemID core.ID) ([]core.HistoryEvent, error) {
	grab, err := r.grabEvent(ctx, downloadID, itemID)
	if err != nil {
		return nil, err
	}
	if grab.ParentID.IsZero() || downloadID == "" {
		return []core.HistoryEvent{grab}, nil
	}
	page, err := r.Store.History().List(ctx, core.HistoryQuery{
		Types: []core.HistoryEventType{core.HistoryGrabbed}, DownloadID: downloadID, ItemIDs: []core.ID{grab.ParentID}, Limit: core.MaxPageSize,
	})
	if err != nil {
		return nil, err
	}
	grabs := []core.HistoryEvent{grab}
	for _, h := range page.Items {
		if h.ID != grab.ID && h.ParentID == grab.ParentID && !slices.ContainsFunc(grabs, func(o core.HistoryEvent) bool { return o.ItemID == h.ItemID }) {
			grabs = append(grabs, h)
		}
	}
	return grabs, nil
}

// markFailed records that the download of a grab failed, as the handlers
// of Radarr's and Sonarr's DownloadFailedEvent: the tracked download, if
// any, fails; the release is blocklisted; the failure goes in the
// history, once per grabbed item; and the movie or episodes are searched
// for again when search is set, or when redownload is not skipped and the
// settings say so, as Radarr's RedownloadFailedDownloadService: releases
// grabbed from an interactive search only with
// AutoRedownloadFailedFromInteractiveSearch. grabs are the download's grab events, the first
// standing for them; episode grabs have their series as parent.
func (r *Runtime) markFailed(ctx context.Context, d *core.Download, grabs []core.HistoryEvent, message string, skipRedownload, search bool) error {
	now := r.now()
	grab := grabs[0]
	if d != nil {
		d.State, d.Messages = core.DownloadFailed, []string{message}
		if err := r.putDownload(ctx, d); err != nil {
			return err
		}
	}
	entry := blocklistEntry(grab, message, now)
	var episodeIDs []core.ID
	if !grab.ParentID.IsZero() {
		if d != nil && d.ItemID == grab.ParentID && len(d.ItemIDs) > 0 {
			episodeIDs = slices.Clone(d.ItemIDs)
		} else {
			for _, g := range grabs {
				episodeIDs = append(episodeIDs, g.ItemID)
			}
		}
		entry.ItemID, entry.ItemIDs = grab.ParentID, episodeIDs
	}
	if err := r.Store.Blocklist().Add(ctx, &entry); err != nil {
		return fmt.Errorf("blocklist %s: %w", grab.SourceTitle, err)
	}
	data := map[string]string{dataMessage: message}
	for _, k := range []string{dataDownloadClient, dataDownloadClientName, dataIndexer, dataReleaseSource} {
		if v := grab.Data[k]; v != "" {
			data[k] = v
		}
	}
	events := make([]core.HistoryEvent, 0, len(grabs))
	for _, g := range grabs {
		events = append(events, core.HistoryEvent{
			ID: core.NewID(), Type: core.HistoryDownloadFailed, ItemID: g.ItemID, ParentID: g.ParentID, Date: now,
			SourceTitle: g.SourceTitle, Quality: g.Quality, Languages: g.Languages, DownloadID: g.DownloadID, Data: data,
		})
	}
	if err := r.Store.History().Add(ctx, events...); err != nil {
		return err
	}
	r.logger().InfoContext(ctx, "download failed", "release", grab.SourceTitle, "item", grab.ItemID, "message", message)
	if search || (!skipRedownload && r.redownloads(ctx, grab)) {
		if grab.ParentID.IsZero() {
			_, err := r.QueueSearch(ctx, []core.ID{grab.ItemID}, false)
			return err
		}
		return r.redownloadEpisodes(ctx, grab.ParentID, episodeIDs)
	}
	return nil
}

// redownloads reports whether the settings search again for the items
// of a failed grab.
func (r *Runtime) redownloads(ctx context.Context, grab core.HistoryEvent) bool {
	s := r.settings()
	switch {
	case !s.AutoRedownloadFailed:
		r.logger().DebugContext(ctx, "auto redownloading failed releases is disabled")
		return false
	case grab.Data[dataReleaseSource] == releaseSources[acquire.SourceInteractiveSearch] && !s.AutoRedownloadFailedFromInteractiveSearch:
		r.logger().DebugContext(ctx, "auto redownloading failed releases from interactive search is disabled")
		return false
	}
	return true
}

// redownloadEpisodes searches again for the episodes of a failed
// download, as Sonarr's RedownloadFailedDownloadService: one episode by
// itself, a whole season as a season, others one by one.
func (r *Runtime) redownloadEpisodes(ctx context.Context, seriesID core.ID, episodeIDs []core.ID) error {
	if len(episodeIDs) == 0 {
		return nil
	}
	p := EpisodeSearchPayload{Scope: ScopeEpisodes, SeriesID: seriesID, EpisodeIDs: episodeIDs}
	if len(episodeIDs) > 1 {
		rn, err := r.newRun(ctx)
		if err != nil {
			return err
		}
		sh, err := rn.show(ctx, seriesID)
		if errors.Is(err, core.ErrNotFound) || errors.Is(err, core.ErrInvalid) {
			return nil
		}
		if err != nil {
			return err
		}
		if ep, ok := sh.byID[episodeIDs[0]]; ok {
			season := seasonEpisodes(sh, ep.SeasonNumber)
			if len(season) == len(episodeIDs) && !slices.ContainsFunc(season, func(e *series.Episode) bool { return !slices.Contains(episodeIDs, e.ID) }) {
				p = EpisodeSearchPayload{Scope: ScopeSeason, SeriesID: seriesID, SeasonNumber: ep.SeasonNumber}
			}
		}
	}
	_, err := r.QueueEpisodeSearch(ctx, p)
	return err
}

// blocklistEntry returns the blocklist entry of a failed grab, as
// Radarr's BlocklistService.Handle(DownloadFailedEvent).
func blocklistEntry(grab core.HistoryEvent, message string, now time.Time) core.BlocklistEntry {
	b := core.BlocklistEntry{
		ID: core.NewID(), ItemID: grab.ItemID, ItemIDs: []core.ID{grab.ItemID}, SourceTitle: grab.SourceTitle,
		Quality: grab.Quality, Languages: grab.Languages, Date: now, Protocol: core.Protocol(grab.Data[dataProtocol]),
		IndexerID: grab.Data[dataIndexerID], Indexer: grab.Data[dataIndexer], Message: message,
	}
	if t, err := time.Parse(time.RFC3339, grab.Data[dataPublishedDate]); err == nil {
		b.PublishedDate = t
	}
	b.Size, _ = strconv.ParseInt(grab.Data[dataSize], 10, 64)
	if flags := grab.Data[dataIndexerFlags]; flags != "" {
		b.IndexerFlags = strings.Split(flags, ",")
	}
	if !b.Protocol.Valid() {
		b.Protocol = core.ProtocolUsenet
		if grab.Data[dataTorrentInfoHash] != "" {
			b.Protocol = core.ProtocolTorrent
		}
	}
	if b.Protocol == core.ProtocolTorrent {
		b.TorrentInfoHash = grab.Data[dataTorrentInfoHash]
		if b.TorrentInfoHash == "" {
			// A torrent's download ID is its info hash.
			b.TorrentInfoHash = grab.DownloadID
		}
	}
	return b
}

// MarkFailed marks the download of a grabbed history event as failed, as
// Radarr's FailedDownloadService.MarkAsFailed: other events of the
// download stand for its grab. A tracked download of it fails too.
func (r *Runtime) MarkFailed(ctx context.Context, historyID core.ID) error {
	event, err := r.historyEvent(ctx, historyID)
	if err != nil {
		return err
	}
	grab := event
	if event.Type != core.HistoryGrabbed {
		if event.DownloadID == "" {
			return fmt.Errorf("%w: history event %s has no download", core.ErrInvalid, historyID)
		}
		if grab, err = r.grabEvent(ctx, event.DownloadID, event.ItemID); err != nil {
			return err
		}
	}
	grabs := []core.HistoryEvent{grab}
	if !grab.ParentID.IsZero() {
		if grabs, err = r.grabEvents(ctx, grab.DownloadID, grab.ItemID); err != nil {
			return err
		}
	}
	var d *core.Download
	if plugin, client, ok := strings.Cut(grab.Data[dataDownloadClient], "/"); ok && grab.DownloadID != "" {
		tracked, err := r.Store.Downloads().GetByDownloadID(ctx, plugin, client, grab.DownloadID)
		switch {
		case err == nil:
			d = &tracked
		case !errors.Is(err, core.ErrNotFound):
			return err
		}
	}
	return r.markFailed(ctx, d, grabs, msgManuallyFailed, false, false)
}

// historyEvent returns a history event by ID; the store lists events
// only, so they are paged through.
func (r *Runtime) historyEvent(ctx context.Context, id core.ID) (core.HistoryEvent, error) {
	for offset := 0; ; {
		page, err := r.Store.History().List(ctx, core.HistoryQuery{Limit: core.MaxPageSize, Offset: offset})
		if err != nil {
			return core.HistoryEvent{}, err
		}
		for _, h := range page.Items {
			if h.ID == id {
				return h, nil
			}
		}
		offset += len(page.Items)
		if len(page.Items) == 0 || offset >= page.Total {
			return core.HistoryEvent{}, fmt.Errorf("history event %s: %w", id, core.ErrNotFound)
		}
	}
}

// RemoveFromQueue stops tracking a download, as Radarr's queue removal:
// it is removed from its client with its files when removeFromClient is
// set; with blocklist its release is blocklisted and the download fails,
// searching again when search is set; without, it is ignored.
func (r *Runtime) RemoveFromQueue(ctx context.Context, id core.ID, removeFromClient, blocklist, search bool) error {
	d, err := r.Store.Downloads().Get(ctx, id)
	if err != nil {
		return err
	}
	if removeFromClient {
		r.removeFromClient(ctx, &d, true)
	}
	if blocklist {
		grabs, err := r.grabEvents(ctx, d.DownloadID, d.ItemID)
		switch {
		case err == nil:
			return r.markFailed(ctx, &d, grabs, msgManuallyFailed, true, search)
		case !errors.Is(err, core.ErrNotFound):
			return err
		}
		// Without its grab nothing can be blocklisted.
		d.State, d.Messages = core.DownloadFailed, []string{msgManuallyFailed}
		if err := r.putDownload(ctx, &d); err != nil {
			return err
		}
		switch {
		case !search:
			return nil
		case isSeriesDownload(d):
			return r.redownloadEpisodes(ctx, d.ItemID, d.ItemIDs)
		}
		_, err = r.QueueSearch(ctx, []core.ID{d.ItemID}, false)
		return err
	}
	d.State = core.DownloadIgnored
	if err := r.putDownload(ctx, &d); err != nil {
		return err
	}
	// Episode downloads are ignored for each item grabbed.
	items := []core.HistoryEvent{{ItemID: d.ItemID}}
	if isSeriesDownload(d) {
		grabs, err := r.grabEvents(ctx, d.DownloadID, d.ItemID)
		switch {
		case err == nil:
			items = grabs
		case !errors.Is(err, core.ErrNotFound):
			return err
		}
	}
	events := make([]core.HistoryEvent, 0, len(items))
	for _, it := range items {
		events = append(events, core.HistoryEvent{
			ID: core.NewID(), Type: core.HistoryIgnored, ItemID: it.ItemID, ParentID: it.ParentID, Date: r.now(), SourceTitle: d.Title,
			Quality: d.Quality, Languages: d.Languages, DownloadID: d.DownloadID,
			Data: map[string]string{dataMessage: "Manually ignored", dataDownloadClient: d.PluginID + "/" + d.ClientID},
		})
	}
	return r.Store.History().Add(ctx, events...)
}
