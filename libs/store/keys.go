package store

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store/internal/ent"
	"github.com/mavioai/mavio/libs/store/internal/ent/item"
	"github.com/mavioai/mavio/libs/store/internal/ent/itemvalue"
	"github.com/mavioai/mavio/libs/store/internal/ent/person"
)

// keysVersion identifies how the stored search and sort keys are derived
// (cleanValue, lowerFold, sortKey). Bump it whenever their output changes: Open
// then recomputes the keys of every existing row once, recording the
// version in schema_migrations.
const keysVersion = "keys-1"

// ratingsVersion identifies how inherited ratings are derived
// (inheritRatings). Bump it whenever the derivation changes: Open then
// recomputes them for every item once.
const ratingsVersion = "ratings-1"

// backfillBatch is the number of rows read per backfill query.
const backfillBatch = 500

// backfillKeys recomputes the derived keys and inherited ratings of all
// rows unless their current versions have been applied already.
func (s *Store) backfillKeys(ctx context.Context) error {
	err := s.backfill(ctx, keysVersion, func(tx *Store) error {
		if err := backfillItems(ctx, tx.write); err != nil {
			return err
		}
		if err := backfillValues(ctx, tx.write); err != nil {
			return err
		}
		return backfillPeople(ctx, tx.write)
	})
	if err != nil {
		return err
	}
	return s.backfill(ctx, ratingsVersion, func(tx *Store) error {
		return tx.inheritRatings(ctx, "c.parent_id IS NULL")
	})
}

// backfill runs fn in a transaction that records version in
// schema_migrations, unless it is recorded already.
func (s *Store) backfill(ctx context.Context, version string, fn func(tx *Store) error) error {
	query := `SELECT version FROM schema_migrations WHERE version = ?`
	insert := `INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`
	if s.dialect == DialectPostgres {
		query = `SELECT version FROM schema_migrations WHERE version = $1`
		insert = `INSERT INTO schema_migrations (version, applied_at) VALUES ($1, $2)`
	}
	var v string
	err := s.dbs[0].QueryRowContext(ctx, query, version).Scan(&v)
	switch {
	case err == nil:
		return nil
	case !errors.Is(err, sql.ErrNoRows):
		return fmt.Errorf("check %s: %w", version, err)
	}
	err = s.writeTx(ctx, func(tx *Store) error {
		if err := fn(tx); err != nil {
			return err
		}
		_, err := tx.tx.ExecContext(ctx, insert, version, time.Now().Unix())
		return err
	})
	if err != nil {
		return fmt.Errorf("backfill %s: %w", version, err)
	}
	return nil
}

func backfillItems(ctx context.Context, c *ent.Client) error {
	var after core.ID
	for {
		list, err := c.Item.Query().Where(item.IDGT(after)).Order(ent.Asc(item.FieldID)).
			Limit(backfillBatch).Select(item.FieldName, item.FieldSortName, item.FieldOriginalTitle).All(ctx)
		if err != nil {
			return err
		}
		for _, it := range list {
			err := c.Item.UpdateOneID(it.ID).
				SetSortKey(sortKey(cmp.Or(it.SortName, it.Name))).
				SetSearchKey(cleanValue(it.Name)).
				SetOriginalKey(lowerFold(it.OriginalTitle)).
				Exec(ctx)
			if err != nil {
				return err
			}
		}
		if len(list) < backfillBatch {
			return nil
		}
		after = list[len(list)-1].ID
	}
}

func backfillValues(ctx context.Context, c *ent.Client) error {
	after := 0
	for {
		list, err := c.ItemValue.Query().Where(itemvalue.IDGT(after)).Order(ent.Asc(itemvalue.FieldID)).
			Limit(backfillBatch).Select(itemvalue.FieldValue).All(ctx)
		if err != nil {
			return err
		}
		for _, v := range list {
			if err := c.ItemValue.UpdateOneID(v.ID).SetValueKey(cleanValue(v.Value)).SetSortKey(sortKey(v.Value)).Exec(ctx); err != nil {
				return err
			}
		}
		if len(list) < backfillBatch {
			return nil
		}
		after = list[len(list)-1].ID
	}
}

func backfillPeople(ctx context.Context, c *ent.Client) error {
	var after core.ID
	for {
		list, err := c.Person.Query().Where(person.IDGT(after)).Order(ent.Asc(person.FieldID)).
			Limit(backfillBatch).Select(person.FieldName, person.FieldSortName).All(ctx)
		if err != nil {
			return err
		}
		for _, p := range list {
			err := c.Person.UpdateOneID(p.ID).
				SetSearchKey(cleanValue(p.Name)).
				SetSortKey(sortKey(cmp.Or(p.SortName, p.Name))).
				Exec(ctx)
			if err != nil {
				return err
			}
		}
		if len(list) < backfillBatch {
			return nil
		}
		after = list[len(list)-1].ID
	}
}
