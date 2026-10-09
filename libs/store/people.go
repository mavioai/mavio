package store

import (
	"cmp"
	"context"
	"slices"
	"strings"

	entsql "entgo.io/ent/dialect/sql"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store/internal/ent"
	"github.com/mavioai/mavio/libs/store/internal/ent/credit"
	"github.com/mavioai/mavio/libs/store/internal/ent/item"
	"github.com/mavioai/mavio/libs/store/internal/ent/person"
	"github.com/mavioai/mavio/libs/store/internal/ent/predicate"
)

func itemID(id core.ID) predicate.Item     { return item.ID(id) }
func personID(id core.ID) predicate.Person { return person.ID(id) }

type people struct{ s *Store }

func (r people) Get(ctx context.Context, id core.ID) (core.Person, error) {
	p, err := r.s.read.Person.Get(ctx, id)
	if err != nil {
		return core.Person{}, mapErr(err, "get person "+id.String())
	}
	return toPerson(p), nil
}

func (r people) FindByName(ctx context.Context, name string) (core.Person, error) {
	p, err := r.s.read.Person.Query().Where(person.NameKey(strings.ToLower(name))).
		Order(ent.Asc(person.FieldID)).First(ctx)
	if err != nil {
		return core.Person{}, mapErr(err, "find person "+name)
	}
	return toPerson(p), nil
}

func (r people) Upsert(ctx context.Context, list ...core.Person) error {
	for i := range list {
		if err := list[i].Validate(); err != nil {
			return err
		}
	}
	return r.s.writeTx(ctx, func(tx *Store) error {
		for chunk := range slices.Chunk(list, upsertBatch) {
			builders := make([]*ent.PersonCreate, len(chunk))
			for i, p := range chunk {
				builders[i] = tx.write.Person.Create().
					SetID(p.ID).
					SetName(p.Name).
					SetNameKey(strings.ToLower(p.Name)).
					SetSortName(p.SortName).
					SetSearchKey(cleanValue(p.Name)).
					SetSortKey(sortKey(cmp.Or(p.SortName, p.Name))).
					SetOverview(p.Overview).
					SetNillableBirthDate(p.BirthDate).
					SetNillableDeathDate(p.DeathDate).
					SetBirthPlace(p.BirthPlace).
					SetExternalIds(fromProviderMap(p.ExternalIDs))
			}
			err := tx.write.Person.CreateBulk(builders...).OnConflictColumns(person.FieldID).UpdateNewValues().Exec(ctx)
			if err != nil {
				return mapErr(err, "upsert people")
			}
		}
		return nil
	})
}

func (r people) Search(ctx context.Context, q core.PersonQuery) ([]core.PersonCount, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}
	limit := q.Limit
	if limit == 0 {
		limit = core.MaxPageSize
	}
	ps := []predicate.Credit{credit.HasItemWith(items(r).filter(q.Items)...)}
	if len(q.CreditKinds) > 0 {
		kinds := make([]string, len(q.CreditKinds))
		for i, k := range q.CreditKinds {
			kinds[i] = string(k)
		}
		ps = append(ps, credit.KindIn(kinds...))
	}
	srch, searching := parseSearch(q.Search)
	var rows []struct {
		ID    core.ID `sql:"person_id"`
		Count int     `sql:"item_count"`
	}
	err := r.s.read.Credit.Query().Where(ps...).Modify(func(s *entsql.Selector) {
		p := entsql.Table(person.Table).As("p")
		s.Join(p).On(s.C(credit.FieldPersonID), p.C(person.FieldID))
		key := p.C(person.FieldSearchKey)
		if searching {
			s.Where(srch.match(key, "", p.C(person.FieldSortKey)))
		}
		// Grouping by the people's key lets PostgreSQL order by their
		// other columns.
		s.Select(entsql.As(p.C(person.FieldID), "person_id"),
			entsql.As("COUNT(DISTINCT "+s.C(credit.FieldItemID)+")", "item_count"),
		).GroupBy(p.C(person.FieldID))
		if searching {
			s.OrderExpr(srch.rank(key))
		}
		s.OrderBy(p.C(person.FieldSortKey), p.C(person.FieldID)).Limit(limit).Offset(q.Offset)
	}).Scan(ctx, &rows)
	if err != nil {
		return nil, mapErr(err, "search people")
	}
	ids := make([]core.ID, len(rows))
	for i, row := range rows {
		ids[i] = row.ID
	}
	list, err := r.s.read.Person.Query().Where(person.IDIn(ids...)).All(ctx)
	if err != nil {
		return nil, mapErr(err, "get people")
	}
	byID := make(map[core.ID]*ent.Person, len(list))
	for _, p := range list {
		byID[p.ID] = p
	}
	out := make([]core.PersonCount, 0, len(rows))
	for _, row := range rows {
		if p, ok := byID[row.ID]; ok {
			out = append(out, core.PersonCount{Person: toPerson(p), Count: row.Count})
		}
	}
	return out, nil
}

func (r people) CreditsForItem(ctx context.Context, itemID core.ID) ([]core.Credit, error) {
	list, err := r.s.read.Credit.Query().Where(credit.ItemID(itemID)).
		Order(ent.Asc(credit.FieldOrd), ent.Asc(credit.FieldID)).All(ctx)
	if err != nil {
		return nil, mapErr(err, "list credits")
	}
	out := make([]core.Credit, len(list))
	for i, c := range list {
		out[i] = core.Credit{ItemID: c.ItemID, PersonID: c.PersonID, Kind: core.CreditKind(c.Kind), Role: c.Role, Order: c.Ord}
	}
	return out, nil
}

func (r people) ReplaceCredits(ctx context.Context, itemID core.ID, credits []core.Credit) error {
	for i := range credits {
		credits[i].ItemID = itemID
		if err := credits[i].Validate(); err != nil {
			return err
		}
	}
	return r.s.writeTx(ctx, func(tx *Store) error {
		if _, err := tx.write.Credit.Delete().Where(credit.ItemID(itemID)).Exec(ctx); err != nil {
			return mapErr(err, "delete credits")
		}
		for chunk := range slices.Chunk(credits, upsertBatch) {
			builders := make([]*ent.CreditCreate, len(chunk))
			for i, c := range chunk {
				builders[i] = tx.write.Credit.Create().
					SetItemID(itemID).
					SetPersonID(c.PersonID).
					SetKind(string(c.Kind)).
					SetRole(c.Role).
					SetOrd(c.Order)
			}
			if err := tx.write.Credit.CreateBulk(builders...).Exec(ctx); err != nil {
				return mapErr(err, "insert credits")
			}
		}
		return nil
	})
}

func toPerson(p *ent.Person) core.Person {
	return core.Person{
		ID:          p.ID,
		Name:        p.Name,
		SortName:    p.SortName,
		Overview:    p.Overview,
		BirthDate:   utcPtr(p.BirthDate),
		DeathDate:   utcPtr(p.DeathDate),
		BirthPlace:  p.BirthPlace,
		ExternalIDs: toProviderMap(p.ExternalIds),
	}
}
