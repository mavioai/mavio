package store

import (
	"context"
	"fmt"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store/internal/ent"
	"github.com/mavioai/mavio/libs/store/internal/ent/image"
	"github.com/mavioai/mavio/libs/store/internal/ent/mediasource"
)

type mediaSources struct{ s *Store }

func (r mediaSources) ListForItem(ctx context.Context, itemID core.ID) ([]core.MediaSource, error) {
	list, err := r.s.read.MediaSource.Query().Where(mediasource.ItemID(itemID)).
		Order(ent.Asc(mediasource.FieldOrd)).All(ctx)
	if err != nil {
		return nil, mapErr(err, "list media sources")
	}
	out := make([]core.MediaSource, len(list))
	for i, m := range list {
		out[i] = toMediaSource(m)
	}
	return out, nil
}

func (r mediaSources) Replace(ctx context.Context, itemID core.ID, sources []core.MediaSource) error {
	for i := range sources {
		if sources[i].ID.IsZero() {
			sources[i].ID = core.NewID()
		}
		sources[i].ItemID = itemID
		if err := sources[i].Validate(); err != nil {
			return err
		}
	}
	return r.s.writeTx(ctx, func(tx *Store) error {
		if _, err := tx.write.MediaSource.Delete().Where(mediasource.ItemID(itemID)).Exec(ctx); err != nil {
			return mapErr(err, "delete media sources")
		}
		builders := make([]*ent.MediaSourceCreate, len(sources))
		for i, m := range sources {
			builders[i] = tx.write.MediaSource.Create().
				SetID(m.ID).
				SetItemID(itemID).
				SetOrd(i).
				SetPath(m.Path).
				SetParts(m.Parts).
				SetDisc(string(m.Disc)).
				SetNillableModified(nonZero(m.Modified)).
				SetName(m.Name).
				SetContainer(m.Container).
				SetSize(m.Size).
				SetDuration(m.Duration).
				SetBitrate(m.Bitrate).
				SetStreams(m.Streams).
				SetChapters(m.Chapters).
				SetKeyframes(m.Keyframes).
				SetNillableProbedAt(nonZero(m.ProbedAt))
		}
		return mapErr(tx.write.MediaSource.CreateBulk(builders...).Exec(ctx), "insert media sources")
	})
}

func toMediaSource(m *ent.MediaSource) core.MediaSource {
	return core.MediaSource{
		ID:        m.ID,
		ItemID:    m.ItemID,
		Path:      m.Path,
		Parts:     m.Parts,
		Disc:      core.DiscKind(m.Disc),
		Modified:  zeroIfNil(m.Modified),
		Name:      m.Name,
		Container: m.Container,
		Size:      m.Size,
		Duration:  m.Duration,
		Bitrate:   m.Bitrate,
		Streams:   m.Streams,
		Chapters:  m.Chapters,
		Keyframes: m.Keyframes,
		ProbedAt:  zeroIfNil(m.ProbedAt),
	}
}

type images struct{ s *Store }

func (r images) ListForOwner(ctx context.Context, ownerID core.ID) ([]core.Image, error) {
	list, err := r.s.read.Image.Query().
		Where(image.Or(image.ItemID(ownerID), image.PersonID(ownerID))).
		Order(ent.Asc(image.FieldKind), ent.Asc(image.FieldIndex)).All(ctx)
	if err != nil {
		return nil, mapErr(err, "list images")
	}
	out := make([]core.Image, len(list))
	for i, img := range list {
		out[i] = toImage(img)
	}
	return out, nil
}

func (r images) Get(ctx context.Context, id core.ID) (core.Image, error) {
	img, err := r.s.read.Image.Get(ctx, id)
	if err != nil {
		return core.Image{}, mapErr(err, "get image "+id.String())
	}
	return toImage(img), nil
}

func (r images) ListForOwners(ctx context.Context, ownerIDs []core.ID) (map[core.ID][]core.Image, error) {
	out := map[core.ID][]core.Image{}
	if len(ownerIDs) == 0 {
		return out, nil
	}
	list, err := r.s.read.Image.Query().
		Where(image.Or(image.ItemIDIn(ownerIDs...), image.PersonIDIn(ownerIDs...))).
		Order(ent.Asc(image.FieldKind), ent.Asc(image.FieldIndex)).All(ctx)
	if err != nil {
		return nil, mapErr(err, "list images")
	}
	for _, e := range list {
		img := toImage(e)
		out[img.OwnerID] = append(out[img.OwnerID], img)
	}
	return out, nil
}

func (r images) Replace(ctx context.Context, ownerID core.ID, list []core.Image) error {
	for i := range list {
		if list[i].ID.IsZero() {
			list[i].ID = core.NewID()
		}
		list[i].OwnerID = ownerID
		if err := list[i].Validate(); err != nil {
			return err
		}
	}
	return r.s.writeTx(ctx, func(tx *Store) error {
		isItem, err := tx.write.Item.Query().Where(itemID(ownerID)).Exist(ctx)
		if err != nil {
			return mapErr(err, "find image owner")
		}
		if !isItem {
			isPerson, err := tx.write.Person.Query().Where(personID(ownerID)).Exist(ctx)
			if err != nil {
				return mapErr(err, "find image owner")
			}
			if !isPerson {
				return fmt.Errorf("image owner %s: %w", ownerID, core.ErrNotFound)
			}
		}
		if _, err := tx.write.Image.Delete().Where(image.Or(image.ItemID(ownerID), image.PersonID(ownerID))).Exec(ctx); err != nil {
			return mapErr(err, "delete images")
		}
		builders := make([]*ent.ImageCreate, len(list))
		for i, img := range list {
			b := tx.write.Image.Create().
				SetID(img.ID).
				SetKind(string(img.Kind)).
				SetIndex(img.Index).
				SetPath(img.Path).
				SetRemoteURL(img.RemoteURL).
				SetWidth(img.Width).
				SetHeight(img.Height).
				SetBlurhash(img.Blurhash).
				SetThumbhash(img.Thumbhash)
			if isItem {
				b.SetItemID(ownerID)
			} else {
				b.SetPersonID(ownerID)
			}
			builders[i] = b
		}
		return mapErr(tx.write.Image.CreateBulk(builders...).Exec(ctx), "insert images")
	})
}

func toImage(img *ent.Image) core.Image {
	out := core.Image{
		ID:        img.ID,
		Kind:      core.ImageKind(img.Kind),
		Index:     img.Index,
		Path:      img.Path,
		RemoteURL: img.RemoteURL,
		Width:     img.Width,
		Height:    img.Height,
		Blurhash:  img.Blurhash,
		Thumbhash: img.Thumbhash,
	}
	switch {
	case img.ItemID != nil:
		out.OwnerID = *img.ItemID
	case img.PersonID != nil:
		out.OwnerID = *img.PersonID
	}
	return out
}
