package store

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store/internal/ent"
	"github.com/mavioai/mavio/libs/store/internal/ent/blocklistentry"
	"github.com/mavioai/mavio/libs/store/internal/ent/download"
	"github.com/mavioai/mavio/libs/store/internal/ent/historyevent"
	"github.com/mavioai/mavio/libs/store/internal/ent/pendingrelease"
	"github.com/mavioai/mavio/libs/store/internal/ent/predicate"
)

// pageSize checks a page's bounds and returns its effective limit; zero
// means core.MaxPageSize.
func pageSize(limit, offset int) (int, error) {
	if limit < 0 || limit > core.MaxPageSize || offset < 0 {
		return 0, fmt.Errorf("%w: page outside 0–%d", core.ErrInvalid, core.MaxPageSize)
	}
	if limit == 0 {
		return core.MaxPageSize, nil
	}
	return limit, nil
}

type downloads struct{ s *Store }

// Downloads returns the tracked downloads.
func (s *Store) Downloads() core.DownloadRepository { return downloads{s} }

func (r downloads) Get(ctx context.Context, id core.ID) (core.Download, error) {
	d, err := r.s.read.Download.Get(ctx, id)
	if err != nil {
		return core.Download{}, mapErr(err, "get download")
	}
	return toDownload(d), nil
}

func (r downloads) GetByDownloadID(ctx context.Context, pluginID, clientID, downloadID string) (core.Download, error) {
	d, err := r.s.read.Download.Query().
		Where(download.PluginID(pluginID), download.ClientID(clientID), download.DownloadID(downloadID)).Only(ctx)
	if err != nil {
		return core.Download{}, mapErr(err, "get download")
	}
	return toDownload(d), nil
}

func (r downloads) List(ctx context.Context, q core.DownloadQuery) (core.Page[core.Download], error) {
	limit, err := pageSize(q.Limit, q.Offset)
	if err != nil {
		return core.Page[core.Download]{}, err
	}
	var where []predicate.Download
	if !q.ItemID.IsZero() {
		where = append(where, download.ItemID(q.ItemID))
	}
	if len(q.States) > 0 {
		states := make([]string, len(q.States))
		for i, s := range q.States {
			states[i] = string(s)
		}
		where = append(where, download.StateIn(states...))
	}
	if q.Unfinished {
		var finished []string
		for _, s := range core.DownloadStates {
			if s.Finished() {
				finished = append(finished, string(s))
			}
		}
		where = append(where, download.StateNotIn(finished...))
	}
	query := r.s.read.Download.Query().Where(where...)
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return core.Page[core.Download]{}, mapErr(err, "count downloads")
	}
	rows, err := query.Order(ent.Desc(download.FieldGrabbedAt), ent.Desc(download.FieldID)).
		Limit(limit).Offset(q.Offset).All(ctx)
	if err != nil {
		return core.Page[core.Download]{}, mapErr(err, "list downloads")
	}
	out := core.Page[core.Download]{Total: total, Items: make([]core.Download, len(rows))}
	for i, d := range rows {
		out.Items[i] = toDownload(d)
	}
	return out, nil
}

// Put stores the download; a zero GrabbedAt or UpdatedAt is now.
func (r downloads) Put(ctx context.Context, d *core.Download) error {
	if d.ID.IsZero() {
		d.ID = core.NewID()
	}
	if err := d.Validate(); err != nil {
		return err
	}
	d.GrabbedAt = orNow(d.GrabbedAt).UTC()
	d.UpdatedAt = orNow(d.UpdatedAt).UTC()
	err := r.s.write.Download.Create().
		SetID(d.ID).
		SetPluginID(d.PluginID).
		SetClientID(d.ClientID).
		SetDownloadID(d.DownloadID).
		SetItemID(d.ItemID).
		SetItemIds(d.ItemIDs).
		SetTitle(d.Title).
		SetIndexerID(d.IndexerID).
		SetIndexer(d.Indexer).
		SetProtocol(string(d.Protocol)).
		SetQuality(d.Quality).
		SetLanguages(d.Languages).
		SetSize(d.Size).
		SetState(string(d.State)).
		SetMessages(d.Messages).
		SetGrabbedAt(d.GrabbedAt).
		SetUpdatedAt(d.UpdatedAt).
		OnConflictColumns(download.FieldID).
		UpdateNewValues().
		Exec(ctx)
	return mapErr(err, "put download")
}

func (r downloads) Delete(ctx context.Context, id core.ID) error {
	return mapErr(r.s.write.Download.DeleteOneID(id).Exec(ctx), "delete download")
}

func toDownload(d *ent.Download) core.Download {
	return core.Download{
		ID: d.ID, PluginID: d.PluginID, ClientID: d.ClientID, DownloadID: d.DownloadID, ItemID: d.ItemID, ItemIDs: d.ItemIds,
		Title: d.Title, IndexerID: d.IndexerID, Indexer: d.Indexer, Protocol: core.Protocol(d.Protocol), Quality: d.Quality,
		Languages: d.Languages, Size: d.Size, State: core.DownloadState(d.State), Messages: d.Messages,
		GrabbedAt: d.GrabbedAt.UTC(), UpdatedAt: d.UpdatedAt.UTC(),
	}
}

type history struct{ s *Store }

// History returns the acquisition history.
func (s *Store) History() core.HistoryRepository { return history{s} }

func (r history) Add(ctx context.Context, events ...core.HistoryEvent) error {
	for i := range events {
		if events[i].ID.IsZero() {
			events[i].ID = core.NewID()
		}
		if err := events[i].Validate(); err != nil {
			return err
		}
	}
	if len(events) == 0 {
		return nil
	}
	creates := make([]*ent.HistoryEventCreate, len(events))
	for i, h := range events {
		c := r.s.write.HistoryEvent.Create().
			SetID(h.ID).
			SetType(string(h.Type)).
			SetItemID(h.ItemID).
			SetDate(h.Date).
			SetSourceTitle(h.SourceTitle).
			SetQuality(h.Quality).
			SetLanguages(h.Languages).
			SetDownloadID(h.DownloadID).
			SetData(h.Data)
		if !h.ParentID.IsZero() {
			c.SetParentID(h.ParentID)
		}
		creates[i] = c
	}
	return mapErr(r.s.write.HistoryEvent.CreateBulk(creates...).Exec(ctx), "add history")
}

func (r history) List(ctx context.Context, q core.HistoryQuery) (core.Page[core.HistoryEvent], error) {
	limit, err := pageSize(q.Limit, q.Offset)
	if err != nil {
		return core.Page[core.HistoryEvent]{}, err
	}
	var where []predicate.HistoryEvent
	if len(q.ItemIDs) > 0 {
		where = append(where, historyevent.Or(historyevent.ItemIDIn(q.ItemIDs...), historyevent.ParentIDIn(q.ItemIDs...)))
	}
	if len(q.Types) > 0 {
		types := make([]string, len(q.Types))
		for i, t := range q.Types {
			types[i] = string(t)
		}
		where = append(where, historyevent.TypeIn(types...))
	}
	if q.DownloadID != "" {
		where = append(where, historyevent.DownloadID(q.DownloadID))
	}
	if !q.Since.IsZero() {
		where = append(where, historyevent.DateGTE(q.Since))
	}
	query := r.s.read.HistoryEvent.Query().Where(where...)
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return core.Page[core.HistoryEvent]{}, mapErr(err, "count history")
	}
	rows, err := query.Order(ent.Desc(historyevent.FieldDate), ent.Desc(historyevent.FieldID)).
		Limit(limit).Offset(q.Offset).All(ctx)
	if err != nil {
		return core.Page[core.HistoryEvent]{}, mapErr(err, "list history")
	}
	out := core.Page[core.HistoryEvent]{Total: total, Items: make([]core.HistoryEvent, len(rows))}
	for i, h := range rows {
		out.Items[i] = core.HistoryEvent{
			ID: h.ID, Type: core.HistoryEventType(h.Type), ItemID: h.ItemID, Date: h.Date.UTC(),
			SourceTitle: h.SourceTitle, Quality: h.Quality, Languages: h.Languages, DownloadID: h.DownloadID, Data: h.Data,
		}
		if h.ParentID != nil {
			out.Items[i].ParentID = *h.ParentID
		}
	}
	return out, nil
}

func (r history) MoveItems(ctx context.Context, to core.ID, from ...core.ID) error {
	from = slices.DeleteFunc(slices.Clone(from), func(id core.ID) bool { return id == to })
	if len(from) == 0 {
		return nil
	}
	err := r.s.write.HistoryEvent.Update().Where(historyevent.ItemIDIn(from...)).SetItemID(to).Exec(ctx)
	return mapErr(err, "move history")
}

func (r history) Purge(ctx context.Context, before time.Time) (int, error) {
	n, err := r.s.write.HistoryEvent.Delete().Where(historyevent.DateLT(before)).Exec(ctx)
	return n, mapErr(err, "purge history")
}

type blocklist struct{ s *Store }

// Blocklist returns the blocklisted releases.
func (s *Store) Blocklist() core.BlocklistRepository { return blocklist{s} }

func (r blocklist) Add(ctx context.Context, b *core.BlocklistEntry) error {
	if b.ID.IsZero() {
		b.ID = core.NewID()
	}
	if err := b.Validate(); err != nil {
		return err
	}
	err := r.s.write.BlocklistEntry.Create().
		SetID(b.ID).
		SetItemID(b.ItemID).
		SetItemIds(b.ItemIDs).
		SetSourceTitle(b.SourceTitle).
		SetQuality(b.Quality).
		SetLanguages(b.Languages).
		SetDate(b.Date).
		SetNillablePublishedDate(nonZero(b.PublishedDate)).
		SetSize(b.Size).
		SetProtocol(string(b.Protocol)).
		SetIndexerID(b.IndexerID).
		SetIndexer(b.Indexer).
		SetIndexerFlags(b.IndexerFlags).
		SetMessage(b.Message).
		SetTorrentInfoHash(b.TorrentInfoHash).
		Exec(ctx)
	return mapErr(err, "add blocklist entry")
}

func (r blocklist) List(ctx context.Context, itemID core.ID, limit, offset int) (core.Page[core.BlocklistEntry], error) {
	size, err := pageSize(limit, offset)
	if err != nil {
		return core.Page[core.BlocklistEntry]{}, err
	}
	query := r.s.read.BlocklistEntry.Query()
	if !itemID.IsZero() {
		query.Where(blocklistentry.ItemID(itemID))
	}
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return core.Page[core.BlocklistEntry]{}, mapErr(err, "count blocklist")
	}
	rows, err := query.Order(ent.Desc(blocklistentry.FieldDate), ent.Desc(blocklistentry.FieldID)).
		Limit(size).Offset(offset).All(ctx)
	if err != nil {
		return core.Page[core.BlocklistEntry]{}, mapErr(err, "list blocklist")
	}
	out := core.Page[core.BlocklistEntry]{Total: total, Items: make([]core.BlocklistEntry, len(rows))}
	for i, b := range rows {
		out.Items[i] = core.BlocklistEntry{
			ID: b.ID, ItemID: b.ItemID, ItemIDs: b.ItemIds, SourceTitle: b.SourceTitle, Quality: b.Quality,
			Languages: b.Languages, Date: b.Date.UTC(), PublishedDate: zeroIfNil(b.PublishedDate), Size: b.Size,
			Protocol: core.Protocol(b.Protocol), IndexerID: b.IndexerID, Indexer: b.Indexer, IndexerFlags: b.IndexerFlags,
			Message: b.Message, TorrentInfoHash: b.TorrentInfoHash,
		}
	}
	return out, nil
}

func (r blocklist) Delete(ctx context.Context, ids ...core.ID) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := r.s.write.BlocklistEntry.Delete().Where(blocklistentry.IDIn(ids...)).Exec(ctx)
	return mapErr(err, "delete blocklist entries")
}

type pendingReleases struct{ s *Store }

// PendingReleases returns the releases kept pending.
func (s *Store) PendingReleases() core.PendingReleaseRepository { return pendingReleases{s} }

func (r pendingReleases) List(ctx context.Context, itemID core.ID) ([]core.PendingRelease, error) {
	query := r.s.read.PendingRelease.Query()
	if !itemID.IsZero() {
		query.Where(pendingrelease.ItemID(itemID))
	}
	rows, err := query.Order(ent.Asc(pendingrelease.FieldAdded), ent.Asc(pendingrelease.FieldID)).All(ctx)
	if err != nil {
		return nil, mapErr(err, "list pending releases")
	}
	out := make([]core.PendingRelease, len(rows))
	for i, p := range rows {
		out[i] = core.PendingRelease{
			ID: p.ID, ItemID: p.ItemID, ItemIDs: p.ItemIds, Title: p.Title, Added: p.Added.UTC(),
			Reason: core.PendingReason(p.Reason), Release: json.RawMessage(p.Release),
		}
	}
	return out, nil
}

func (r pendingReleases) Add(ctx context.Context, p *core.PendingRelease) error {
	if p.ID.IsZero() {
		p.ID = core.NewID()
	}
	if err := p.Validate(); err != nil {
		return err
	}
	err := r.s.write.PendingRelease.Create().
		SetID(p.ID).
		SetItemID(p.ItemID).
		SetItemIds(p.ItemIDs).
		SetTitle(p.Title).
		SetAdded(p.Added).
		SetReason(string(p.Reason)).
		SetRelease(string(p.Release)).
		Exec(ctx)
	return mapErr(err, "add pending release")
}

func (r pendingReleases) Delete(ctx context.Context, ids ...core.ID) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := r.s.write.PendingRelease.Delete().Where(pendingrelease.IDIn(ids...)).Exec(ctx)
	return mapErr(err, "delete pending releases")
}
