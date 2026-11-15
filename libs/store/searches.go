package store

import (
	"context"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store/internal/ent"
	"github.com/mavioai/mavio/libs/store/internal/ent/searchrecord"
)

type searches struct{ s *Store }

// Searches returns the searches that rank popular searches.
func (s *Store) Searches() core.SearchRepository { return searches{s} }

// Record stores a search, replacing the voter's record of its key.
func (r searches) Record(ctx context.Context, rec *core.SearchRecord) error {
	if err := rec.Validate(); err != nil {
		return err
	}
	err := r.s.write.SearchRecord.Create().
		SetQuery(rec.Key).
		SetText(rec.Text).
		SetVoter(rec.Voter).
		SetSearchedAt(rec.SearchedAt.UTC()).
		OnConflictColumns(searchrecord.FieldQuery, searchrecord.FieldVoter).
		UpdateText().
		UpdateSearchedAt().
		Exec(ctx)
	return mapErr(err, "record search")
}

func (r searches) List(ctx context.Context, since time.Time, limit int) ([]core.SearchRecord, error) {
	if limit <= 0 || limit > core.MaxSearchRecords {
		limit = core.MaxSearchRecords
	}
	rows, err := r.s.read.SearchRecord.Query().
		Where(searchrecord.SearchedAtGTE(since.UTC())).
		Order(ent.Desc(searchrecord.FieldSearchedAt), ent.Desc(searchrecord.FieldID)).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, mapErr(err, "list searches")
	}
	out := make([]core.SearchRecord, len(rows))
	for i, row := range rows {
		out[i] = core.SearchRecord{Key: row.Query, Text: row.Text, Voter: row.Voter, SearchedAt: row.SearchedAt.UTC()}
	}
	return out, nil
}

func (r searches) Prune(ctx context.Context, before time.Time, keep int) (int, error) {
	n, err := r.s.write.SearchRecord.Delete().Where(searchrecord.SearchedAtLT(before.UTC())).Exec(ctx)
	if err != nil {
		return 0, mapErr(err, "prune searches")
	}
	// The newest record past keep, and every older one, go too.
	last, err := r.s.write.SearchRecord.Query().
		Order(ent.Desc(searchrecord.FieldSearchedAt), ent.Desc(searchrecord.FieldID)).
		Offset(max(keep, 0)).
		First(ctx)
	if ent.IsNotFound(err) {
		return n, nil
	}
	if err != nil {
		return n, mapErr(err, "prune searches")
	}
	m, err := r.s.write.SearchRecord.Delete().Where(searchrecord.Or(
		searchrecord.SearchedAtLT(last.SearchedAt),
		searchrecord.And(searchrecord.SearchedAt(last.SearchedAt), searchrecord.IDLTE(last.ID)),
	)).Exec(ctx)
	if err != nil {
		return n, mapErr(err, "prune searches")
	}
	return n + m, nil
}
