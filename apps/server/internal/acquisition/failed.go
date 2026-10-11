package acquisition

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

// msgManuallyFailed is the message of downloads marked failed by hand.
const msgManuallyFailed = "Manually marked as failed"

// failTracked handles a tracked download its client reports failed, as
// Radarr's FailedDownloadService.ProcessFailed: without a grab in the
// history it is only warned of.
func (r *Runtime) failTracked(ctx context.Context, d *core.Download, p polled, message string) error {
	grab, err := r.grabEvent(ctx, d.DownloadID, d.ItemID)
	if errors.Is(err, core.ErrNotFound) {
		return r.warn(ctx, d, "Download has failed wasn't grabbed by Mavio, skipping automatic download handling")
	}
	if err != nil {
		return err
	}
	if err := r.markFailed(ctx, d, grab, message, false, false); err != nil {
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

// markFailed records that the download of a grab failed, as the handlers
// of Radarr's DownloadFailedEvent: the tracked download, if any, fails;
// the release is blocklisted; the failure goes in the history; and the
// movie is searched for again when the settings say so and redownload is
// not skipped, or when search is set.
func (r *Runtime) markFailed(ctx context.Context, d *core.Download, grab core.HistoryEvent, message string, skipRedownload, search bool) error {
	now := r.now()
	if d != nil {
		d.State, d.Messages = core.DownloadFailed, []string{message}
		if err := r.putDownload(ctx, d); err != nil {
			return err
		}
	}
	entry := blocklistEntry(grab, message, now)
	if err := r.Store.Blocklist().Add(ctx, &entry); err != nil {
		return fmt.Errorf("blocklist %s: %w", grab.SourceTitle, err)
	}
	data := map[string]string{dataMessage: message}
	for _, k := range []string{dataDownloadClient, dataDownloadClientName, dataIndexer, dataReleaseSource} {
		if v := grab.Data[k]; v != "" {
			data[k] = v
		}
	}
	if err := r.Store.History().Add(ctx, core.HistoryEvent{
		ID: core.NewID(), Type: core.HistoryDownloadFailed, ItemID: grab.ItemID, ParentID: grab.ParentID, Date: now,
		SourceTitle: grab.SourceTitle, Quality: grab.Quality, Languages: grab.Languages, DownloadID: grab.DownloadID, Data: data,
	}); err != nil {
		return err
	}
	r.logger().InfoContext(ctx, "download failed", "release", grab.SourceTitle, "item", grab.ItemID, "message", message)
	if search || (!skipRedownload && r.settings().AutoRedownloadFailed) {
		if _, err := r.QueueSearch(ctx, []core.ID{grab.ItemID}, false); err != nil {
			return err
		}
	}
	return nil
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
	return r.markFailed(ctx, d, grab, msgManuallyFailed, false, false)
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
		grab, err := r.grabEvent(ctx, d.DownloadID, d.ItemID)
		switch {
		case err == nil:
			return r.markFailed(ctx, &d, grab, msgManuallyFailed, true, search)
		case !errors.Is(err, core.ErrNotFound):
			return err
		}
		// Without its grab nothing can be blocklisted.
		d.State, d.Messages = core.DownloadFailed, []string{msgManuallyFailed}
		if err := r.putDownload(ctx, &d); err != nil {
			return err
		}
		if search {
			_, err := r.QueueSearch(ctx, []core.ID{d.ItemID}, false)
			return err
		}
		return nil
	}
	d.State = core.DownloadIgnored
	if err := r.putDownload(ctx, &d); err != nil {
		return err
	}
	return r.Store.History().Add(ctx, core.HistoryEvent{
		ID: core.NewID(), Type: core.HistoryIgnored, ItemID: d.ItemID, Date: r.now(), SourceTitle: d.Title,
		Quality: d.Quality, Languages: d.Languages, DownloadID: d.DownloadID,
		Data: map[string]string{dataMessage: "Manually ignored", dataDownloadClient: d.PluginID + "/" + d.ClientID},
	})
}
