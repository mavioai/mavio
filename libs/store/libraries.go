package store

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store/internal/ent"
	"github.com/mavioai/mavio/libs/store/internal/ent/library"
)

type libraries struct{ s *Store }

func (r libraries) Get(ctx context.Context, id core.ID) (core.Library, error) {
	l, err := r.s.read.Library.Get(ctx, id)
	if err != nil {
		return core.Library{}, mapErr(err, "get library "+id.String())
	}
	return toLibrary(l), nil
}

func (r libraries) List(ctx context.Context) ([]core.Library, error) {
	ls, err := r.s.read.Library.Query().Order(ent.Asc(library.FieldName), ent.Asc(library.FieldID)).All(ctx)
	if err != nil {
		return nil, mapErr(err, "list libraries")
	}
	out := make([]core.Library, len(ls))
	for i, l := range ls {
		out[i] = toLibrary(l)
	}
	return out, nil
}

func (r libraries) Create(ctx context.Context, lib *core.Library) error {
	if lib.ID.IsZero() {
		lib.ID = core.NewID()
	}
	if err := lib.Validate(); err != nil {
		return err
	}
	return r.s.writeTx(ctx, func(tx *Store) error {
		if err := checkPathsFree(ctx, tx, lib); err != nil {
			return err
		}
		now := time.Now()
		created, err := tx.write.Library.Create().
			SetID(lib.ID).
			SetName(lib.Name).
			SetKind(string(lib.Kind)).
			SetPaths(cleanPaths(lib.Paths)).
			SetScanInterval(lib.ScanInterval).
			SetPreferredLanguage(lib.PreferredLanguage).
			SetMetadataCountry(lib.MetadataCountry).
			SetSaveLocalMetadata(lib.SaveLocalMetadata).
			SetAutoCollections(lib.AutoCollections).
			SetExtractTrickplay(lib.ExtractTrickplay).
			SetExtractChapterImages(lib.ExtractChapterImages).
			SetAnalyzeLoudness(lib.AnalyzeLoudness).
			SetCreatedAt(now).
			SetUpdatedAt(now).
			Save(ctx)
		if err != nil {
			return mapErr(err, "create library")
		}
		*lib = toLibrary(created)
		return nil
	})
}

func (r libraries) Update(ctx context.Context, lib *core.Library) error {
	if err := lib.Validate(); err != nil {
		return err
	}
	return r.s.writeTx(ctx, func(tx *Store) error {
		if err := checkPathsFree(ctx, tx, lib); err != nil {
			return err
		}
		updated, err := tx.write.Library.UpdateOneID(lib.ID).
			SetName(lib.Name).
			SetKind(string(lib.Kind)).
			SetPaths(cleanPaths(lib.Paths)).
			SetScanInterval(lib.ScanInterval).
			SetPreferredLanguage(lib.PreferredLanguage).
			SetMetadataCountry(lib.MetadataCountry).
			SetSaveLocalMetadata(lib.SaveLocalMetadata).
			SetAutoCollections(lib.AutoCollections).
			SetExtractTrickplay(lib.ExtractTrickplay).
			SetExtractChapterImages(lib.ExtractChapterImages).
			SetAnalyzeLoudness(lib.AnalyzeLoudness).
			SetUpdatedAt(time.Now()).
			Save(ctx)
		if err != nil {
			return mapErr(err, "update library "+lib.ID.String())
		}
		*lib = toLibrary(updated)
		return nil
	})
}

func (r libraries) Delete(ctx context.Context, id core.ID) error {
	return mapErr(r.s.write.Library.DeleteOneID(id).Exec(ctx), "delete library "+id.String())
}

// checkPathsFree enforces that a folder belongs to at most one library:
// lib's paths may not equal, contain or lie inside another library's paths.
func checkPathsFree(ctx context.Context, tx *Store, lib *core.Library) error {
	others, err := tx.write.Library.Query().Where(library.IDNEQ(lib.ID)).All(ctx)
	if err != nil {
		return mapErr(err, "list libraries")
	}
	for _, p := range cleanPaths(lib.Paths) {
		for _, o := range others {
			for _, q := range o.Paths {
				if pathOverlaps(p, q) {
					return fmt.Errorf("%w: path %s overlaps library %q (%s)", core.ErrConflict, p, o.Name, q)
				}
			}
		}
	}
	return nil
}

func cleanPaths(paths []string) []string {
	out := make([]string, len(paths))
	for i, p := range paths {
		out[i] = filepath.Clean(p)
	}
	return out
}

func pathOverlaps(a, b string) bool {
	within := func(child, parent string) bool {
		return child == parent || strings.HasPrefix(child, strings.TrimSuffix(parent, string(filepath.Separator))+string(filepath.Separator))
	}
	return within(a, b) || within(b, a)
}

func toLibrary(l *ent.Library) core.Library {
	return core.Library{
		ID:                l.ID,
		Name:              l.Name,
		Kind:              core.LibraryKind(l.Kind),
		Paths:             l.Paths,
		ScanInterval:      l.ScanInterval,
		PreferredLanguage: l.PreferredLanguage,
		MetadataCountry:   l.MetadataCountry,
		SaveLocalMetadata: l.SaveLocalMetadata,
		AutoCollections:   l.AutoCollections,

		ExtractTrickplay:     l.ExtractTrickplay,
		ExtractChapterImages: l.ExtractChapterImages,
		AnalyzeLoudness:      l.AnalyzeLoudness,
		CreatedAt:            l.CreatedAt.UTC(),
		UpdatedAt:            l.UpdatedAt.UTC(),
	}
}
