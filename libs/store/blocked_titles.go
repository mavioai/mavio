package store

import (
	"context"
	"fmt"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store/internal/ent"
	"github.com/mavioai/mavio/libs/store/internal/ent/blockedtitle"
)

type blockedTitles struct{ s *Store }

// BlockedTitles returns the blocklist of titles.
func (s *Store) BlockedTitles() core.BlockedTitleRepository { return blockedTitles{s} }

func (r blockedTitles) Get(ctx context.Context, kind core.ItemKind, tmdbID int) (core.BlockedTitle, error) {
	row, err := r.s.read.BlockedTitle.Query().
		Where(blockedtitle.Kind(string(kind)), blockedtitle.TmdbID(tmdbID)).
		Only(ctx)
	if err != nil {
		return core.BlockedTitle{}, mapErr(err, "get blocked title")
	}
	return toBlockedTitle(row), nil
}

func (r blockedTitles) List(ctx context.Context, q core.BlockedTitleQuery) (core.Page[core.BlockedTitle], error) {
	if err := q.Validate(); err != nil {
		return core.Page[core.BlockedTitle]{}, err
	}
	limit, err := pageSize(q.Limit, q.Offset)
	if err != nil {
		return core.Page[core.BlockedTitle]{}, err
	}
	query := r.s.read.BlockedTitle.Query()
	if q.Search != "" {
		query = query.Where(blockedtitle.TitleContainsFold(q.Search))
	}
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return core.Page[core.BlockedTitle]{}, mapErr(err, "count blocked titles")
	}
	rows, err := query.Order(ent.Desc(blockedtitle.FieldCreatedAt), ent.Desc(blockedtitle.FieldID)).
		Limit(limit).Offset(q.Offset).All(ctx)
	if err != nil {
		return core.Page[core.BlockedTitle]{}, mapErr(err, "list blocked titles")
	}
	out := core.Page[core.BlockedTitle]{Total: total, Items: make([]core.BlockedTitle, len(rows))}
	for i, row := range rows {
		out.Items[i] = toBlockedTitle(row)
	}
	return out, nil
}

// Add stores a blocked title; a zero time is now.
func (r blockedTitles) Add(ctx context.Context, b *core.BlockedTitle) error {
	if err := b.Validate(); err != nil {
		return err
	}
	b.CreatedAt = orNow(b.CreatedAt).UTC()
	err := r.s.write.BlockedTitle.Create().
		SetKind(string(b.Kind)).
		SetTmdbID(b.TmdbID).
		SetTitle(b.Title).
		SetNillableUserID(nonZeroID(b.UserID)).
		SetCreatedAt(b.CreatedAt).
		Exec(ctx)
	return mapErr(err, "add blocked title")
}

func (r blockedTitles) Delete(ctx context.Context, kind core.ItemKind, tmdbID int) error {
	n, err := r.s.write.BlockedTitle.Delete().
		Where(blockedtitle.Kind(string(kind)), blockedtitle.TmdbID(tmdbID)).
		Exec(ctx)
	if err != nil {
		return mapErr(err, "delete blocked title")
	}
	if n == 0 {
		return fmt.Errorf("delete blocked title: %w", core.ErrNotFound)
	}
	return nil
}

func toBlockedTitle(b *ent.BlockedTitle) core.BlockedTitle {
	out := core.BlockedTitle{Kind: core.ItemKind(b.Kind), TmdbID: b.TmdbID, Title: b.Title, CreatedAt: b.CreatedAt.UTC()}
	if b.UserID != nil {
		out.UserID = *b.UserID
	}
	return out
}
