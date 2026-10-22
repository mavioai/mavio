package store

import (
	"context"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store/internal/ent"
	"github.com/mavioai/mavio/libs/store/internal/ent/itemlocalization"
)

type localizations struct{ s *Store }

// Localizations returns items' localizations.
func (s *Store) Localizations() core.LocalizationRepository { return localizations{s} }

func (r localizations) Get(ctx context.Context, itemID core.ID, language string) (core.ItemLocalization, error) {
	row, err := r.s.read.ItemLocalization.Query().
		Where(itemlocalization.ItemID(itemID), itemlocalization.Language(language)).Only(ctx)
	if err != nil {
		return core.ItemLocalization{}, mapErr(err, "get localization")
	}
	return localizationFromEnt(row), nil
}

func (r localizations) GetMany(ctx context.Context, itemIDs []core.ID, language string) (map[core.ID]core.ItemLocalization, error) {
	out := map[core.ID]core.ItemLocalization{}
	if len(itemIDs) == 0 {
		return out, nil
	}
	rows, err := r.s.read.ItemLocalization.Query().
		Where(itemlocalization.ItemIDIn(itemIDs...), itemlocalization.Language(language)).All(ctx)
	if err != nil {
		return nil, mapErr(err, "get localizations")
	}
	for _, row := range rows {
		out[row.ItemID] = localizationFromEnt(row)
	}
	return out, nil
}

func (r localizations) Languages(ctx context.Context, itemID core.ID) ([]string, error) {
	langs, err := r.s.read.ItemLocalization.Query().Where(itemlocalization.ItemID(itemID)).
		Order(ent.Asc(itemlocalization.FieldLanguage)).Select(itemlocalization.FieldLanguage).Strings(ctx)
	return langs, mapErr(err, "list localization languages")
}

func (r localizations) Put(ctx context.Context, l *core.ItemLocalization) error {
	if err := l.Validate(); err != nil {
		return err
	}
	err := r.s.write.ItemLocalization.Create().
		SetItemID(l.ItemID).SetLanguage(l.Language).
		SetName(l.Name).SetSearchKey(cleanValue(l.Name)).SetSortKey(sortKey(l.Name)).SetOverview(l.Overview).SetTagline(l.Tagline).SetFetchedAt(l.FetchedAt.UTC()).
		OnConflictColumns(itemlocalization.FieldItemID, itemlocalization.FieldLanguage).
		Update(func(u *ent.ItemLocalizationUpsert) {
			u.UpdateName().UpdateSearchKey().UpdateSortKey().UpdateOverview().UpdateTagline().UpdateFetchedAt()
		}).
		Exec(ctx)
	return mapErr(err, "put localization")
}

func localizationFromEnt(e *ent.ItemLocalization) core.ItemLocalization {
	return core.ItemLocalization{
		ItemID: e.ItemID, Language: e.Language, Name: e.Name, Overview: e.Overview, Tagline: e.Tagline, FetchedAt: e.FetchedAt,
	}
}
