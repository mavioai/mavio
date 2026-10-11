package store

import (
	"context"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store/internal/ent"
	"github.com/mavioai/mavio/libs/store/internal/ent/extrafile"
)

type extraFiles struct{ s *Store }

// ExtraFiles returns the extra files imported with movie and episode
// files.
func (s *Store) ExtraFiles() core.ExtraFileRepository { return extraFiles{s} }

func (r extraFiles) List(ctx context.Context, itemID core.ID) ([]core.ExtraFile, error) {
	rows, err := r.s.read.ExtraFile.Query().
		Where(extrafile.ItemID(itemID)).
		Order(ent.Asc(extrafile.FieldPath), ent.Asc(extrafile.FieldID)).
		All(ctx)
	if err != nil {
		return nil, mapErr(err, "list extra files")
	}
	out := make([]core.ExtraFile, len(rows))
	for i, e := range rows {
		out[i] = core.ExtraFile{
			ID: e.ID, ItemID: e.ItemID, Kind: core.ExtraFileKind(e.Kind), Path: e.Path, Extension: e.Extension,
			Language: e.Language, LanguageTags: nilIfEmpty(e.LanguageTags), Added: e.Added.UTC(), LastUpdated: e.LastUpdated.UTC(),
		}
	}
	return out, nil
}

// Put inserts or replaces an extra file; zero times are now.
func (r extraFiles) Put(ctx context.Context, e *core.ExtraFile) error {
	if e.ID.IsZero() {
		e.ID = core.NewID()
	}
	if err := e.Validate(); err != nil {
		return err
	}
	e.Added = orNow(e.Added).UTC()
	e.LastUpdated = orNow(e.LastUpdated).UTC()
	err := r.s.write.ExtraFile.Create().
		SetID(e.ID).
		SetItemID(e.ItemID).
		SetKind(string(e.Kind)).
		SetPath(e.Path).
		SetExtension(e.Extension).
		SetLanguage(e.Language).
		SetLanguageTags(e.LanguageTags).
		SetAdded(e.Added).
		SetLastUpdated(e.LastUpdated).
		OnConflictColumns(extrafile.FieldID).
		UpdateNewValues().
		Exec(ctx)
	return mapErr(err, "put extra file")
}

func (r extraFiles) Delete(ctx context.Context, ids ...core.ID) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := r.s.write.ExtraFile.Delete().Where(extrafile.IDIn(ids...)).Exec(ctx)
	return mapErr(err, "delete extra files")
}

// nilIfEmpty returns nil for an empty slice, as JSON columns read back
// empty arrays.
func nilIfEmpty(s []string) []string {
	if len(s) == 0 {
		return nil
	}
	return s
}
