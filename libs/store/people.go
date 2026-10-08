package store

import (
	"context"
	"slices"
	"strings"

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
