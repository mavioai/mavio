package store

import (
	"context"
	"fmt"
	"path"
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
			SetStorageClasses(cleanClasses(lib.StorageClasses)).
			SetScanInterval(lib.ScanInterval).
			SetPreferredLanguage(lib.PreferredLanguage).
			SetMetadataCountry(lib.MetadataCountry).
			SetSaveLocalMetadata(lib.SaveLocalMetadata).
			SetAutoCollections(lib.AutoCollections).
			SetExtractTrickplay(lib.ExtractTrickplay).
			SetExtractChapterImages(lib.ExtractChapterImages).
			SetAnalyzeLoudness(lib.AnalyzeLoudness).
			SetDownloadLyrics(lib.DownloadLyrics).
			SetProviderOrder(lib.Providers).
			SetVirtualEpisodes(lib.VirtualEpisodes).
			SetHubs(lib.Hubs).
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
			SetStorageClasses(cleanClasses(lib.StorageClasses)).
			SetScanInterval(lib.ScanInterval).
			SetPreferredLanguage(lib.PreferredLanguage).
			SetMetadataCountry(lib.MetadataCountry).
			SetSaveLocalMetadata(lib.SaveLocalMetadata).
			SetAutoCollections(lib.AutoCollections).
			SetExtractTrickplay(lib.ExtractTrickplay).
			SetExtractChapterImages(lib.ExtractChapterImages).
			SetAnalyzeLoudness(lib.AnalyzeLoudness).
			SetDownloadLyrics(lib.DownloadLyrics).
			SetProviderOrder(lib.Providers).
			SetVirtualEpisodes(lib.VirtualEpisodes).
			SetHubs(lib.Hubs).
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
		out[i] = cleanPath(p)
	}
	return out
}

func cleanClasses(classes map[string]core.StorageClass) map[string]core.StorageClass {
	if len(classes) == 0 {
		return nil
	}
	out := make(map[string]core.StorageClass, len(classes))
	for p, c := range classes {
		out[cleanPath(p)] = c
	}
	return out
}

// cleanPath cleans a library path the way it was written, so it stays
// valid: remote paths are clean already, slash-rooted paths keep their
// slashes, and only other paths, of Windows, are cleaned by the OS rules.
func cleanPath(p string) string {
	switch {
	case core.IsRemotePath(p):
		return p
	case strings.HasPrefix(p, "/") && !strings.HasPrefix(p, "//"):
		return path.Clean(p)
	}
	return filepath.Clean(p)
}

func pathOverlaps(a, b string) bool { return core.InFolder(a, b) || core.InFolder(b, a) }

func toLibrary(l *ent.Library) core.Library {
	return core.Library{
		ID:                l.ID,
		Name:              l.Name,
		Kind:              core.LibraryKind(l.Kind),
		Paths:             l.Paths,
		StorageClasses:    l.StorageClasses,
		ScanInterval:      l.ScanInterval,
		PreferredLanguage: l.PreferredLanguage,
		MetadataCountry:   l.MetadataCountry,
		SaveLocalMetadata: l.SaveLocalMetadata,
		AutoCollections:   l.AutoCollections,

		ExtractTrickplay:     l.ExtractTrickplay,
		ExtractChapterImages: l.ExtractChapterImages,
		AnalyzeLoudness:      l.AnalyzeLoudness,
		DownloadLyrics:       l.DownloadLyrics,
		VirtualEpisodes:      l.VirtualEpisodes,
		Hubs:                 l.Hubs,
		Providers:            l.ProviderOrder,
		CreatedAt:            l.CreatedAt.UTC(),
		UpdatedAt:            l.UpdatedAt.UTC(),
	}
}
