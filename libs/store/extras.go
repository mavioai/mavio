package store

import (
	"context"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store/internal/ent"
	"github.com/mavioai/mavio/libs/store/internal/ent/mediasegment"
	"github.com/mavioai/mavio/libs/store/internal/ent/trickplay"
)

type trickplays struct{ s *Store }

// Trickplay returns the descriptions of items' trickplay sheets.
func (s *Store) Trickplay() core.TrickplayRepository { return trickplays{s} }

func (r trickplays) Put(ctx context.Context, t *core.Trickplay) error {
	if err := t.Validate(); err != nil {
		return err
	}
	err := r.s.write.Trickplay.Create().
		SetItemID(t.ItemID).SetWidth(t.Width).SetHeight(t.Height).SetTileWidth(t.TileWidth).SetTileHeight(t.TileHeight).
		SetThumbnailCount(t.ThumbnailCount).SetInterval(t.Interval).SetBandwidth(t.Bandwidth).
		OnConflictColumns(trickplay.FieldItemID, trickplay.FieldWidth).
		UpdateNewValues().
		Exec(ctx)
	return mapErr(err, "put trickplay")
}

func (r trickplays) List(ctx context.Context, itemID core.ID) ([]core.Trickplay, error) {
	rows, err := r.s.read.Trickplay.Query().Where(trickplay.ItemID(itemID)).Order(ent.Asc(trickplay.FieldWidth)).All(ctx)
	if err != nil {
		return nil, mapErr(err, "list trickplay")
	}
	out := make([]core.Trickplay, len(rows))
	for i, t := range rows {
		out[i] = core.Trickplay{
			ItemID: t.ItemID, Width: t.Width, Height: t.Height, TileWidth: t.TileWidth, TileHeight: t.TileHeight,
			ThumbnailCount: t.ThumbnailCount, Interval: t.Interval, Bandwidth: t.Bandwidth,
		}
	}
	return out, nil
}

func (r trickplays) Delete(ctx context.Context, itemID core.ID) error {
	_, err := r.s.write.Trickplay.Delete().Where(trickplay.ItemID(itemID)).Exec(ctx)
	return mapErr(err, "delete trickplay")
}

type mediaSegments struct{ s *Store }

// MediaSegments returns the items' media segments.
func (s *Store) MediaSegments() core.MediaSegmentRepository { return mediaSegments{s} }

func (r mediaSegments) List(ctx context.Context, itemID core.ID) ([]core.MediaSegment, error) {
	rows, err := r.s.read.MediaSegment.Query().Where(mediasegment.ItemID(itemID)).
		Order(ent.Asc(mediasegment.FieldStart), ent.Asc(mediasegment.FieldID)).All(ctx)
	if err != nil {
		return nil, mapErr(err, "list media segments")
	}
	out := make([]core.MediaSegment, len(rows))
	for i, m := range rows {
		out[i] = core.MediaSegment{ID: m.ID, ItemID: m.ItemID, Kind: core.SegmentKind(m.Kind), Start: m.Start, End: m.End, Provider: m.Provider}
	}
	return out, nil
}

func (r mediaSegments) Replace(ctx context.Context, itemID core.ID, segments []core.MediaSegment) error {
	for i := range segments {
		if segments[i].ID.IsZero() {
			segments[i].ID = core.NewID()
		}
		segments[i].ItemID = itemID
		if err := segments[i].Validate(); err != nil {
			return err
		}
	}
	return r.s.writeTx(ctx, func(tx *Store) error {
		if _, err := tx.write.MediaSegment.Delete().Where(mediasegment.ItemID(itemID)).Exec(ctx); err != nil {
			return mapErr(err, "replace media segments")
		}
		creates := make([]*ent.MediaSegmentCreate, len(segments))
		for i, m := range segments {
			creates[i] = tx.write.MediaSegment.Create().SetID(m.ID).SetItemID(itemID).SetKind(string(m.Kind)).
				SetStart(m.Start).SetEnd(m.End).SetProvider(m.Provider)
		}
		return mapErr(tx.write.MediaSegment.CreateBulk(creates...).Exec(ctx), "replace media segments")
	})
}
