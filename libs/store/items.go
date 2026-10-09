package store

import (
	"context"
	"fmt"
	"iter"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	entsql "entgo.io/ent/dialect/sql"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store/internal/ent"
	"github.com/mavioai/mavio/libs/store/internal/ent/credit"
	"github.com/mavioai/mavio/libs/store/internal/ent/item"
	"github.com/mavioai/mavio/libs/store/internal/ent/itemlink"
	"github.com/mavioai/mavio/libs/store/internal/ent/itemvalue"
	"github.com/mavioai/mavio/libs/store/internal/ent/predicate"
	"github.com/mavioai/mavio/libs/store/internal/ent/userdata"
)

// Kinds of item_values rows.
const (
	valueGenre       = "genre"
	valueTag         = "tag"
	valueStudio      = "studio"
	valueArtist      = "artist"
	valueAlbumArtist = "album_artist"
)

// upsertBatch bounds the rows per INSERT, keeping statements well below
// SQLite's bound-parameter limit.
const upsertBatch = 200

// walkBatch is the page size of Walk.
const walkBatch = 500

type items struct{ s *Store }

func (r items) Get(ctx context.Context, id core.ID) (core.Item, error) {
	it, err := r.s.read.Item.Query().Where(item.ID(id)).WithValues().Only(ctx)
	if err != nil {
		return core.Item{}, mapErr(err, "get item "+id.String())
	}
	return toItem(it), nil
}

func (r items) GetByPath(ctx context.Context, libraryID core.ID, path string) (core.Item, error) {
	it, err := r.s.read.Item.Query().
		Where(item.LibraryID(libraryID), item.Path(path), item.PathNEQ("")).
		WithValues().Only(ctx)
	if err != nil {
		return core.Item{}, mapErr(err, "get item by path "+path)
	}
	return toItem(it), nil
}

func (r items) Upsert(ctx context.Context, list ...core.Item) error {
	for i := range list {
		if err := list[i].Validate(); err != nil {
			return err
		}
	}
	return r.s.writeTx(ctx, func(tx *Store) error {
		for chunk := range slices.Chunk(list, upsertBatch) {
			builders := make([]*ent.ItemCreate, len(chunk))
			ids := make([]core.ID, len(chunk))
			for i, it := range chunk {
				builders[i] = itemCreate(tx.write, it)
				ids[i] = it.ID
			}
			err := tx.write.Item.CreateBulk(builders...).
				OnConflictColumns(item.FieldID).
				UpdateNewValues().
				Exec(ctx)
			if err != nil {
				return mapErr(err, "upsert items")
			}
			if err := clearUnset(ctx, tx.write, chunk); err != nil {
				return err
			}
			if _, err := tx.write.ItemValue.Delete().Where(itemvalue.ItemIDIn(ids...)).Exec(ctx); err != nil {
				return mapErr(err, "replace item values")
			}
			var values []*ent.ItemValueCreate
			for _, it := range chunk {
				values = append(values, itemValueCreates(tx.write, it)...)
			}
			for vs := range slices.Chunk(values, upsertBatch) {
				if err := tx.write.ItemValue.CreateBulk(vs...).Exec(ctx); err != nil {
					return mapErr(err, "insert item values")
				}
			}
		}
		// Recompute the inherited ratings below every item whose parent was
		// not written with it.
		written := make(map[core.ID]bool, len(list))
		for _, it := range list {
			written[it.ID] = true
		}
		var roots []any
		for _, it := range list {
			if !written[it.ParentID] {
				roots = append(roots, it.ID)
			}
		}
		for chunk := range slices.Chunk(roots, upsertBatch) {
			if err := tx.inheritRatings(ctx, "c.id IN ("+tx.placeholders(len(chunk))+")", chunk...); err != nil {
				return err
			}
		}
		return nil
	})
}

// optionalColumns are the item columns that are NULL when unset, with how
// to tell an unset value, find a set column and clear it.
var optionalColumns = []struct {
	unset func(*core.Item) bool
	set   predicate.Item
	clear func(*ent.ItemUpdate) *ent.ItemUpdate
}{
	{func(it *core.Item) bool { return it.IndexNumber == nil }, item.IndexNumberNotNil(), (*ent.ItemUpdate).ClearIndexNumber},
	{func(it *core.Item) bool { return it.ParentIndexNumber == nil }, item.ParentIndexNumberNotNil(), (*ent.ItemUpdate).ClearParentIndexNumber},
	{func(it *core.Item) bool { return it.IndexNumberEnd == nil }, item.IndexNumberEndNotNil(), (*ent.ItemUpdate).ClearIndexNumberEnd},
	{func(it *core.Item) bool { return it.PremiereDate == nil }, item.PremiereDateNotNil(), (*ent.ItemUpdate).ClearPremiereDate},
	{func(it *core.Item) bool { return it.EndDate == nil }, item.EndDateNotNil(), (*ent.ItemUpdate).ClearEndDate},
	{func(it *core.Item) bool { return it.ParentalRating == nil }, item.ParentalRatingNotNil(), (*ent.ItemUpdate).ClearParentalRating},
	{func(it *core.Item) bool { return it.AirsBeforeSeasonNumber == nil }, item.AirsBeforeSeasonNumberNotNil(), (*ent.ItemUpdate).ClearAirsBeforeSeasonNumber},
	{func(it *core.Item) bool { return it.AirsAfterSeasonNumber == nil }, item.AirsAfterSeasonNumberNotNil(), (*ent.ItemUpdate).ClearAirsAfterSeasonNumber},
	{func(it *core.Item) bool { return it.AirsBeforeEpisodeNumber == nil }, item.AirsBeforeEpisodeNumberNotNil(), (*ent.ItemUpdate).ClearAirsBeforeEpisodeNumber},
	{func(it *core.Item) bool { return it.FileModified.IsZero() }, item.FileModifiedNotNil(), (*ent.ItemUpdate).ClearFileModified},
	{func(it *core.Item) bool { return it.MetadataRefreshedAt.IsZero() }, item.MetadataRefreshedAtNotNil(), (*ent.ItemUpdate).ClearMetadataRefreshedAt},
	{func(it *core.Item) bool { return it.MissingSince == nil }, item.MissingSinceNotNil(), (*ent.ItemUpdate).ClearMissingSince},
	{func(it *core.Item) bool { return it.ParentID.IsZero() }, item.ParentIDNotNil(), (*ent.ItemUpdate).ClearParentID},
	{func(it *core.Item) bool { return it.OwnerID.IsZero() }, item.OwnerIDNotNil(), (*ent.ItemUpdate).ClearOwnerID},
	{func(it *core.Item) bool { return it.UserID.IsZero() }, item.UserIDNotNil(), (*ent.ItemUpdate).ClearUserID},
}

// clearUnset sets the optional columns the items leave unset to NULL: an
// upsert writes only the columns some item of its batch sets, so it would
// keep their old values.
func clearUnset(ctx context.Context, c *ent.Client, list []core.Item) error {
	for _, col := range optionalColumns {
		var ids []core.ID
		for i := range list {
			if col.unset(&list[i]) {
				ids = append(ids, list[i].ID)
			}
		}
		if len(ids) == 0 {
			continue
		}
		if _, err := col.clear(c.Item.Update().Where(item.IDIn(ids...), col.set)).Save(ctx); err != nil {
			return mapErr(err, "clear unset item fields")
		}
	}
	return nil
}

// inheritRatings recomputes inherited_rating for the items matching seed,
// a condition on the items table aliased c, and all their descendants: an
// item's own parental rating, or when unrated its parent's inherited one.
func (s *Store) inheritRatings(ctx context.Context, seed string, args ...any) error {
	// Ratings are never negative, so -1 stands for NULL in the comparison.
	query := `WITH RECURSIVE r(id, rating) AS (
	SELECT c.id, COALESCE(c.parental_rating, p.inherited_rating)
	FROM items c LEFT JOIN items p ON p.id = c.parent_id WHERE ` + seed + `
	UNION ALL
	SELECT i.id, COALESCE(i.parental_rating, r.rating)
	FROM items i JOIN r ON i.parent_id = r.id
)
UPDATE items SET inherited_rating = r.rating FROM r
WHERE items.id = r.id AND COALESCE(items.inherited_rating, -1) <> COALESCE(r.rating, -1)`
	_, err := s.tx.ExecContext(ctx, query, args...)
	return mapErr(err, "inherit ratings")
}

// placeholders returns n comma-separated bind parameters of the dialect.
func (s *Store) placeholders(n int) string {
	var b strings.Builder
	for i := range n {
		if i > 0 {
			b.WriteString(", ")
		}
		if s.dialect == DialectPostgres {
			fmt.Fprintf(&b, "$%d", i+1)
		} else {
			b.WriteString("?")
		}
	}
	return b.String()
}

func (r items) Delete(ctx context.Context, ids ...core.ID) error {
	if len(ids) == 0 {
		return nil
	}
	// Descendants, extras, values, media sources, images, credits and user
	// data go with the item through ON DELETE CASCADE.
	_, err := r.s.write.Item.Delete().Where(item.IDIn(ids...)).Exec(ctx)
	return mapErr(err, "delete items")
}

func (r items) MarkSeen(ctx context.Context, libraryID core.ID, prefix string, generation int64) error {
	_, err := r.s.write.Item.Update().
		Where(item.LibraryID(libraryID), item.PathNEQ(""), item.MissingSinceIsNil(), underPath(prefix)).
		SetScanGeneration(generation).
		Save(ctx)
	return mapErr(err, "mark items seen")
}

// underPath matches paths equal to prefix or below it. It compares
// exactly: LIKE ignores case in SQLite.
func underPath(prefix string) predicate.Item {
	prefix = strings.TrimSuffix(prefix, "/")
	dir := prefix + "/"
	return item.Or(item.Path(prefix), predicate.Item(func(s *entsql.Selector) {
		s.Where(entsql.P(func(b *entsql.Builder) {
			b.WriteString("substr(").WriteString(s.C(item.FieldPath)).WriteString(", 1, ").
				Arg(utf8.RuneCountInString(dir)).WriteString(") = ").Arg(dir)
		}))
	}))
}

func (r items) Touch(ctx context.Context, libraryID core.ID, generation int64, paths ...string) error {
	for chunk := range slices.Chunk(paths, upsertBatch) {
		_, err := r.s.write.Item.Update().
			Where(item.LibraryID(libraryID), item.PathIn(chunk...)).
			SetScanGeneration(generation).
			ClearMissingSince().
			Save(ctx)
		if err != nil {
			return mapErr(err, "touch items")
		}
	}
	return nil
}

func (r items) MarkMissing(ctx context.Context, libraryID core.ID, generation int64, now time.Time) (int, error) {
	n, err := r.s.write.Item.Update().
		Where(item.LibraryID(libraryID), item.PathNEQ(""), item.ScanGenerationLT(generation), item.MissingSinceIsNil()).
		SetMissingSince(now.UTC()).
		Save(ctx)
	return n, mapErr(err, "mark items missing")
}

func (r items) PurgeMissing(ctx context.Context, libraryID core.ID, before time.Time) ([]core.ID, error) {
	var ids []core.ID
	err := r.s.writeTx(ctx, func(tx *Store) error {
		var err error
		ids, err = tx.write.Item.Query().
			Where(item.LibraryID(libraryID), item.MissingSinceLT(before.UTC())).
			IDs(ctx)
		if err != nil {
			return mapErr(err, "list missing items")
		}
		return items{tx}.Delete(ctx, ids...)
	})
	return ids, err
}

func (r items) Query(ctx context.Context, q core.ItemQuery) (core.Page[core.Item], error) {
	if err := q.Validate(); err != nil {
		return core.Page[core.Item]{}, err
	}
	preds := r.predicates(q)
	total, err := r.s.read.Item.Query().Where(preds...).Count(ctx)
	if err != nil {
		return core.Page[core.Item]{}, mapErr(err, "count items")
	}
	query := r.s.read.Item.Query().Where(preds...).WithValues().
		Limit(q.PageSize()).Offset(q.Offset).
		Modify(func(s *entsql.Selector) { orderItems(s, q) })
	list, err := query.All(ctx)
	if err != nil {
		return core.Page[core.Item]{}, mapErr(err, "query items")
	}
	page := core.Page[core.Item]{Items: make([]core.Item, len(list)), Total: total}
	for i, it := range list {
		page.Items[i] = toItem(it)
	}
	return page, nil
}

func (r items) Walk(ctx context.Context, q core.ItemQuery) iter.Seq2[core.Item, error] {
	return func(yield func(core.Item, error) bool) {
		q.Limit, q.Offset, q.Sort = 0, 0, nil
		if err := q.Validate(); err != nil {
			yield(core.Item{}, err)
			return
		}
		preds := r.predicates(q)
		var after core.ID
		for {
			page := append(slices.Clone(preds), item.IDGT(after))
			list, err := r.s.read.Item.Query().Where(page...).WithValues().
				Order(ent.Asc(item.FieldID)).Limit(walkBatch).All(ctx)
			if err != nil {
				yield(core.Item{}, mapErr(err, "walk items"))
				return
			}
			for _, it := range list {
				if !yield(toItem(it), nil) {
					return
				}
			}
			if len(list) < walkBatch {
				return
			}
			after = list[len(list)-1].ID
		}
	}
}

func (r items) Values(ctx context.Context, q core.ValueQuery) ([]core.ValueCount, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}
	matching := r.filter(q.Items)
	limit := q.Limit
	if limit == 0 {
		limit = core.MaxPageSize
	}
	if q.Kind == core.ValueYear {
		return r.years(ctx, matching, limit, q.Offset)
	}
	kinds := []string{string(q.Kind)}
	if q.Kind == core.ValueArtist {
		kinds = append(kinds, valueAlbumArtist)
	}
	ps := []predicate.ItemValue{itemvalue.KindIn(kinds...), itemvalue.ValueKeyNEQ(""), itemvalue.HasItemWith(matching...)}
	srch, searching := parseSearch(q.Search)
	if searching {
		ps = append(ps, predicate.ItemValue(func(s *entsql.Selector) {
			s.Where(srch.match(s.C(itemvalue.FieldValueKey), "", s.C(itemvalue.FieldSortKey)))
		}))
	}
	// Values with the same clean form are one value, shown in the spelling
	// that sorts first by code point, ordered by sort name and counted once
	// per item.
	minValue := "MIN(%s)"
	if r.s.dialect == DialectPostgres {
		minValue = `MIN(%s COLLATE "C")`
	}
	var rows []struct {
		Key   string `sql:"value_key"`
		Value string `sql:"value"`
		Count int    `sql:"item_count"`
	}
	err := r.s.read.ItemValue.Query().Where(ps...).Modify(func(s *entsql.Selector) {
		key := s.C(itemvalue.FieldValueKey)
		s.Select(key,
			entsql.As(fmt.Sprintf(minValue, s.C(itemvalue.FieldValue)), "value"),
			entsql.As("COUNT(DISTINCT "+s.C(itemvalue.FieldItemID)+")", "item_count"),
		).GroupBy(key)
		if searching {
			s.OrderExpr(srch.rank(key))
		}
		s.OrderExpr(entsql.Expr("MIN(" + s.C(itemvalue.FieldSortKey) + ")"))
		s.OrderBy(key).Limit(limit).Offset(q.Offset)
	}).Scan(ctx, &rows)
	if err != nil {
		return nil, mapErr(err, "list item values")
	}
	out := make([]core.ValueCount, len(rows))
	for i, row := range rows {
		out[i] = core.ValueCount{Value: row.Value, Count: row.Count}
	}
	return out, nil
}

func (r items) Links(ctx context.Context, containerID core.ID) ([]core.Link, error) {
	list, err := r.s.read.ItemLink.Query().Where(itemlink.ContainerID(containerID)).
		Order(ent.Asc(itemlink.FieldOrd), ent.Asc(itemlink.FieldID)).All(ctx)
	if err != nil {
		return nil, mapErr(err, "list links")
	}
	out := make([]core.Link, len(list))
	for i, l := range list {
		out[i] = core.Link{ID: l.ID, ContainerID: l.ContainerID, ItemID: l.ItemID}
	}
	return out, nil
}

func (r items) ReplaceLinks(ctx context.Context, containerID core.ID, links []core.Link) error {
	return r.s.writeTx(ctx, func(tx *Store) error {
		if _, err := tx.write.ItemLink.Delete().Where(itemlink.ContainerID(containerID)).Exec(ctx); err != nil {
			return mapErr(err, "replace links")
		}
		for start := 0; start < len(links); start += upsertBatch {
			chunk := links[start:min(start+upsertBatch, len(links))]
			builders := make([]*ent.ItemLinkCreate, len(chunk))
			for i, l := range chunk {
				if l.ID.IsZero() {
					l.ID = core.NewID()
				}
				builders[i] = tx.write.ItemLink.Create().SetID(l.ID).SetContainerID(containerID).
					SetItemID(l.ItemID).SetOrd(start + i)
			}
			if err := tx.write.ItemLink.CreateBulk(builders...).Exec(ctx); err != nil {
				return mapErr(err, "insert links")
			}
		}
		return nil
	})
}

// years lists the production years of the matching items, oldest first.
func (r items) years(ctx context.Context, matching []predicate.Item, limit, offset int) ([]core.ValueCount, error) {
	var rows []struct {
		Year  int `sql:"production_year"`
		Count int `sql:"item_count"`
	}
	err := r.s.read.Item.Query().Where(append(matching, item.ProductionYearGT(0))...).Modify(func(s *entsql.Selector) {
		year := s.C(item.FieldProductionYear)
		s.Select(year, entsql.As(entsql.Count("*"), "item_count")).GroupBy(year).OrderBy(year).Limit(limit).Offset(offset)
	}).Scan(ctx, &rows)
	if err != nil {
		return nil, mapErr(err, "list production years")
	}
	out := make([]core.ValueCount, len(rows))
	for i, row := range rows {
		out[i] = core.ValueCount{Value: strconv.Itoa(row.Year), Count: row.Count}
	}
	return out, nil
}

// filter translates an item filter of value and person lists.
func (r items) filter(f core.ItemFilter) []predicate.Item {
	return r.predicates(core.ItemQuery{LibraryIDs: f.LibraryIDs, Kinds: f.Kinds, MaxRating: f.MaxRating, SkipUnrated: f.SkipUnrated})
}

// predicates translates the filters of q.
func (r items) predicates(q core.ItemQuery) []predicate.Item {
	var ps []predicate.Item
	if len(q.LibraryIDs) > 0 {
		ps = append(ps, item.LibraryIDIn(q.LibraryIDs...))
	}
	switch {
	case !q.ParentID.IsZero() && q.Recursive:
		ps = append(ps, descendantsOf(q.ParentID))
	case !q.ParentID.IsZero():
		ps = append(ps, item.ParentID(q.ParentID))
	case !q.MemberOf.IsZero():
		ps = append(ps, item.HasLinkedInWith(itemlink.ContainerID(q.MemberOf)))
	}
	if len(q.Kinds) > 0 {
		kinds := make([]string, len(q.Kinds))
		for i, k := range q.Kinds {
			kinds[i] = string(k)
		}
		ps = append(ps, item.KindIn(kinds...))
	}
	if !q.IncludeExtras {
		ps = append(ps, item.Extra(""))
	}
	if !q.IncludeMissing {
		ps = append(ps, item.MissingSinceIsNil())
	}
	if srch, ok := parseSearch(q.Search); ok {
		ps = append(ps, predicate.Item(func(s *entsql.Selector) {
			s.Where(srch.match(s.C(item.FieldSearchKey), s.C(item.FieldOriginalKey), s.C(item.FieldSortKey)))
		}))
	}
	for kind, values := range map[string][]string{valueGenre: q.Genres, valueTag: q.Tags, valueStudio: q.Studios} {
		if len(values) > 0 {
			keys := make([]string, len(values))
			for i, v := range values {
				keys[i] = cleanValue(v)
			}
			ps = append(ps, item.HasValuesWith(itemvalue.Kind(kind), itemvalue.ValueKeyIn(keys...)))
		}
	}
	if !q.PersonID.IsZero() {
		ps = append(ps, item.HasCreditsWith(credit.PersonID(q.PersonID)))
	}
	if q.YearFrom != 0 {
		ps = append(ps, item.ProductionYearGTE(q.YearFrom))
	}
	if q.YearTo != 0 {
		ps = append(ps, item.ProductionYearLTE(q.YearTo))
	}
	if q.MaxRating != nil {
		within := item.InheritedRatingLTE(*q.MaxRating)
		if !q.SkipUnrated {
			within = item.Or(within, item.InheritedRatingIsNil())
		}
		ps = append(ps, within)
	}
	if !q.UserID.IsZero() {
		// Other users' playlists are theirs alone.
		ps = append(ps, item.Or(item.KindNEQ(string(core.KindPlaylist)), item.UserID(q.UserID)))
		mine := userdata.UserID(q.UserID)
		if q.Played != nil {
			played := item.HasUserDataWith(mine, userdata.Played(true))
			if !*q.Played {
				played = item.Not(played)
			}
			ps = append(ps, played)
		}
		if q.Favorite != nil {
			fav := item.HasUserDataWith(mine, userdata.Favorite(true))
			if !*q.Favorite {
				fav = item.Not(fav)
			}
			ps = append(ps, fav)
		}
		if q.Resumable {
			ps = append(ps, item.HasUserDataWith(mine, userdata.PositionGT(0)))
		}
	}
	return ps
}

// descendantsOf matches all items below parent, using a recursive CTE that
// both SQLite and PostgreSQL support.
func descendantsOf(parent core.ID) predicate.Item {
	return predicate.Item(func(s *entsql.Selector) {
		s.Where(entsql.P(func(b *entsql.Builder) {
			b.Ident(s.C(item.FieldID)).WriteString(" IN (WITH RECURSIVE d(id) AS (SELECT id FROM items WHERE parent_id = ").
				Arg(parent).
				WriteString(" UNION ALL SELECT i.id FROM items i JOIN d ON i.parent_id = d.id) SELECT id FROM d)")
		}))
	})
}

// orderItems applies q.Sort, always tie-breaking by ID for stable paging.
// Search results are ordered by relevance first and by sort name after
// q.Sort.
func orderItems(s *entsql.Selector, q core.ItemQuery) {
	srch, searching := parseSearch(q.Search)
	if searching {
		s.OrderExpr(srch.rank(s.C(item.FieldSearchKey)))
	}
	var ud *entsql.SelectTable
	joinUserData := func() *entsql.SelectTable {
		if ud == nil {
			ud = entsql.Table(userdata.Table).As("ud")
			s.LeftJoin(ud).OnP(entsql.And(
				entsql.ColumnsEQ(s.C(item.FieldID), ud.C(userdata.FieldItemID)),
				entsql.EQ(ud.C(userdata.FieldUserID), q.UserID),
			))
		}
		return ud
	}
	dir := func(expr string, desc bool) string {
		if desc {
			return expr + " DESC"
		}
		return expr + " ASC"
	}
	for _, spec := range q.Sort {
		switch spec.Field {
		case core.SortName:
			s.OrderExpr(entsql.Expr(dir(s.C(item.FieldSortKey), spec.Desc)))
		case core.SortDateAdded:
			s.OrderExpr(entsql.Expr(dir(s.C(item.FieldDateAdded), spec.Desc)))
		case core.SortPremiereDate:
			// Undated items sort last in both directions.
			s.OrderExpr(entsql.Expr("CASE WHEN " + s.C(item.FieldPremiereDate) + " IS NULL THEN 1 ELSE 0 END"))
			s.OrderExpr(entsql.Expr(dir(s.C(item.FieldPremiereDate), spec.Desc)))
		case core.SortProductionYear:
			s.OrderExpr(entsql.Expr(dir(s.C(item.FieldProductionYear), spec.Desc)))
		case core.SortCommunityRating:
			s.OrderExpr(entsql.Expr(dir(s.C(item.FieldCommunityRating), spec.Desc)))
		case core.SortRuntime:
			s.OrderExpr(entsql.Expr(dir(s.C(item.FieldRuntime), spec.Desc)))
		case core.SortIndex:
			s.OrderExpr(entsql.Expr(dir("COALESCE("+s.C(item.FieldParentIndexNumber)+", -1)", spec.Desc)))
			s.OrderExpr(entsql.Expr(dir("COALESCE("+s.C(item.FieldIndexNumber)+", -1)", spec.Desc)))
		case core.SortRandom:
			s.OrderExpr(entsql.Expr("RANDOM()"))
		case core.SortLastPlayed:
			t := joinUserData()
			s.OrderExpr(entsql.Expr("CASE WHEN " + t.C(userdata.FieldLastPlayedAt) + " IS NULL THEN 1 ELSE 0 END"))
			s.OrderExpr(entsql.Expr(dir(t.C(userdata.FieldLastPlayedAt), spec.Desc)))
		case core.SortPlayCount:
			t := joinUserData()
			s.OrderExpr(entsql.Expr(dir("COALESCE("+t.C(userdata.FieldPlayCount)+", 0)", spec.Desc)))
		case core.SortListOrder:
			// An item listed twice sorts by its first entry. The ID is
			// written as a literal: ent drops the arguments of ORDER BY
			// expressions, and its canonical form needs no escaping.
			s.OrderExpr(entsql.Expr(dir("(SELECT MIN(ord) FROM item_links WHERE container_id = '"+q.MemberOf.String()+
				"' AND item_id = "+s.C(item.FieldID)+")", spec.Desc)))
		}
	}
	if searching {
		s.OrderExpr(entsql.Expr(s.C(item.FieldSortKey) + " ASC"))
	}
	s.OrderExpr(entsql.Expr(s.C(item.FieldID) + " ASC"))
}

func itemCreate(c *ent.Client, it core.Item) *ent.ItemCreate {
	sortName := it.SortName
	if sortName == "" {
		sortName = it.Name
	}
	create := c.Item.Create().
		SetID(it.ID).
		SetLibraryID(it.LibraryID).
		SetKind(string(it.Kind)).
		SetName(it.Name).
		SetSortName(sortName).
		SetSortKey(sortKey(sortName)).
		SetSearchKey(cleanValue(it.Name)).
		SetOriginalKey(lowerFold(it.OriginalTitle)).
		SetOriginalTitle(it.OriginalTitle).
		SetOverview(it.Overview).
		SetTagline(it.Tagline).
		SetPath(it.Path).
		SetNillableIndexNumber(it.IndexNumber).
		SetNillableParentIndexNumber(it.ParentIndexNumber).
		SetNillableIndexNumberEnd(it.IndexNumberEnd).
		SetProductionYear(it.ProductionYear).
		SetNillablePremiereDate(it.PremiereDate).
		SetNillableEndDate(it.EndDate).
		SetRuntime(it.Runtime).
		SetOfficialRating(it.OfficialRating).
		SetCustomRating(it.CustomRating).
		SetNillableParentalRating(it.ParentalRating).
		// Upsert then derives it from the ancestors when unrated.
		SetNillableInheritedRating(it.ParentalRating).
		SetCommunityRating(it.CommunityRating).
		SetCriticRating(it.CriticRating).
		SetExternalIds(fromProviderMap(it.ExternalIDs)).
		SetProductionLocations(it.ProductionLocations).
		SetRemoteTrailers(it.RemoteTrailers).
		SetCollectionName(it.CollectionName).
		SetAspectRatio(it.AspectRatio).
		SetVideo3dFormat(string(it.Video3DFormat)).
		SetAlbum(it.Album).
		SetSeriesStatus(string(it.SeriesStatus)).
		SetAirDays(fromWeekdays(it.AirDays)).
		SetAirTime(it.AirTime).
		SetDisplayOrder(it.DisplayOrder).
		SetNillableAirsBeforeSeasonNumber(it.AirsBeforeSeasonNumber).
		SetNillableAirsAfterSeasonNumber(it.AirsAfterSeasonNumber).
		SetNillableAirsBeforeEpisodeNumber(it.AirsBeforeEpisodeNumber).
		SetMetadataLanguage(it.MetadataLanguage).
		SetMetadataCountry(it.MetadataCountry).
		SetLocked(it.Locked).
		SetLockedFields(fromFields(it.LockedFields)).
		SetExtra(string(it.Extra)).
		SetDateAdded(orNow(it.DateAdded)).
		SetNillableFileModified(nonZero(it.FileModified)).
		SetNillableMetadataRefreshedAt(nonZero(it.MetadataRefreshedAt)).
		SetScanGeneration(it.ScanGeneration).
		SetNillableMissingSince(utcPtr(it.MissingSince))
	if !it.ParentID.IsZero() {
		create.SetParentID(it.ParentID)
	}
	if !it.OwnerID.IsZero() {
		create.SetOwnerID(it.OwnerID)
	}
	if !it.UserID.IsZero() {
		create.SetUserID(it.UserID)
	}
	return create
}

func itemValueCreates(c *ent.Client, it core.Item) []*ent.ItemValueCreate {
	var out []*ent.ItemValueCreate
	add := func(kind string, values []string) {
		for i, v := range values {
			out = append(out, c.ItemValue.Create().SetItemID(it.ID).SetKind(kind).SetValue(v).
				SetValueKey(cleanValue(v)).SetSortKey(sortKey(v)).SetOrd(i))
		}
	}
	add(valueGenre, it.Genres)
	add(valueTag, it.Tags)
	add(valueStudio, it.Studios)
	add(valueArtist, it.Artists)
	add(valueAlbumArtist, it.AlbumArtists)
	return out
}

func toItem(e *ent.Item) core.Item {
	it := core.Item{
		ID:                      e.ID,
		LibraryID:               e.LibraryID,
		Kind:                    core.ItemKind(e.Kind),
		Name:                    e.Name,
		SortName:                e.SortName,
		OriginalTitle:           e.OriginalTitle,
		Overview:                e.Overview,
		Tagline:                 e.Tagline,
		Path:                    e.Path,
		IndexNumber:             e.IndexNumber,
		ParentIndexNumber:       e.ParentIndexNumber,
		IndexNumberEnd:          e.IndexNumberEnd,
		ProductionYear:          e.ProductionYear,
		PremiereDate:            utcPtr(e.PremiereDate),
		EndDate:                 utcPtr(e.EndDate),
		Runtime:                 e.Runtime,
		OfficialRating:          e.OfficialRating,
		CustomRating:            e.CustomRating,
		ParentalRating:          e.ParentalRating,
		InheritedRating:         e.InheritedRating,
		CommunityRating:         e.CommunityRating,
		CriticRating:            e.CriticRating,
		ExternalIDs:             toProviderMap(e.ExternalIds),
		ProductionLocations:     e.ProductionLocations,
		RemoteTrailers:          e.RemoteTrailers,
		CollectionName:          e.CollectionName,
		AspectRatio:             e.AspectRatio,
		Video3DFormat:           core.Video3DFormat(e.Video3dFormat),
		Album:                   e.Album,
		SeriesStatus:            core.SeriesStatus(e.SeriesStatus),
		AirDays:                 toWeekdays(e.AirDays),
		AirTime:                 e.AirTime,
		DisplayOrder:            e.DisplayOrder,
		AirsBeforeSeasonNumber:  e.AirsBeforeSeasonNumber,
		AirsAfterSeasonNumber:   e.AirsAfterSeasonNumber,
		AirsBeforeEpisodeNumber: e.AirsBeforeEpisodeNumber,
		MetadataLanguage:        e.MetadataLanguage,
		MetadataCountry:         e.MetadataCountry,
		Locked:                  e.Locked,
		LockedFields:            toFields(e.LockedFields),
		Extra:                   core.ExtraKind(e.Extra),
		DateAdded:               e.DateAdded.UTC(),
		FileModified:            zeroIfNil(e.FileModified),
		MetadataRefreshedAt:     zeroIfNil(e.MetadataRefreshedAt),
		ScanGeneration:          e.ScanGeneration,
		MissingSince:            utcPtr(e.MissingSince),
	}
	if e.ParentID != nil {
		it.ParentID = *e.ParentID
	}
	if e.OwnerID != nil {
		it.OwnerID = *e.OwnerID
	}
	if e.UserID != nil {
		it.UserID = *e.UserID
	}
	values := slices.Clone(e.Edges.Values)
	slices.SortFunc(values, func(a, b *ent.ItemValue) int { return a.Ord - b.Ord })
	for _, v := range values {
		switch v.Kind {
		case valueGenre:
			it.Genres = append(it.Genres, v.Value)
		case valueTag:
			it.Tags = append(it.Tags, v.Value)
		case valueStudio:
			it.Studios = append(it.Studios, v.Value)
		case valueArtist:
			it.Artists = append(it.Artists, v.Value)
		case valueAlbumArtist:
			it.AlbumArtists = append(it.AlbumArtists, v.Value)
		}
	}
	return it
}

func fromProviderMap(m map[core.Provider]string) map[string]string {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[string(k)] = v
	}
	return out
}

func toProviderMap(m map[string]string) map[core.Provider]string {
	if len(m) == 0 {
		return nil
	}
	out := make(map[core.Provider]string, len(m))
	for k, v := range m {
		out[core.Provider(k)] = v
	}
	return out
}

func fromWeekdays(days []time.Weekday) []int {
	if len(days) == 0 {
		return nil
	}
	out := make([]int, len(days))
	for i, d := range days {
		out[i] = int(d)
	}
	return out
}

func toWeekdays(days []int) []time.Weekday {
	if len(days) == 0 {
		return nil
	}
	out := make([]time.Weekday, len(days))
	for i, d := range days {
		out[i] = time.Weekday(d)
	}
	return out
}

func fromFields(fields []core.MetadataField) []string {
	if len(fields) == 0 {
		return nil
	}
	out := make([]string, len(fields))
	for i, f := range fields {
		out[i] = string(f)
	}
	return out
}

func toFields(fields []string) []core.MetadataField {
	if len(fields) == 0 {
		return nil
	}
	out := make([]core.MetadataField, len(fields))
	for i, f := range fields {
		out[i] = core.MetadataField(f)
	}
	return out
}
