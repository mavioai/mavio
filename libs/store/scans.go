package store

import (
	"context"
	"slices"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store/internal/ent"
	"github.com/mavioai/mavio/libs/store/internal/ent/folderstate"
	"github.com/mavioai/mavio/libs/store/internal/ent/library"
)

type scans struct{ s *Store }

func (r scans) NextGeneration(ctx context.Context, libraryID core.ID) (int64, error) {
	var gen int64
	err := r.s.writeTx(ctx, func(tx *Store) error {
		n, err := tx.write.Library.Update().Where(library.ID(libraryID)).AddScanGeneration(1).Save(ctx)
		if err != nil {
			return mapErr(err, "count scan")
		}
		if n == 0 {
			return mapErr(&ent.NotFoundError{}, "library "+libraryID.String())
		}
		lib, err := tx.write.Library.Get(ctx, libraryID)
		if err != nil {
			return mapErr(err, "get library")
		}
		gen = lib.ScanGeneration
		return nil
	})
	return gen, err
}

func (r scans) Folder(ctx context.Context, libraryID core.ID, path string) (core.FolderState, error) {
	f, err := r.s.read.FolderState.Query().Where(folderstate.LibraryID(libraryID), folderstate.Path(path)).Only(ctx)
	if err != nil {
		return core.FolderState{}, mapErr(err, "get folder state "+path)
	}
	return core.FolderState{
		LibraryID: f.LibraryID, Path: f.Path, ModTime: f.ModTime.UTC(), FileID: f.FileID,
		Entries: f.Entries, Generation: f.Generation,
	}, nil
}

func (r scans) PutFolders(ctx context.Context, folders ...core.FolderState) error {
	return r.s.writeTx(ctx, func(tx *Store) error {
		for chunk := range slices.Chunk(folders, upsertBatch) {
			builders := make([]*ent.FolderStateCreate, len(chunk))
			for i, f := range chunk {
				entries := make([]core.FolderEntry, len(f.Entries))
				for j, e := range f.Entries {
					e.ModTime = e.ModTime.UTC()
					entries[j] = e
				}
				builders[i] = tx.write.FolderState.Create().
					SetLibraryID(f.LibraryID).
					SetPath(f.Path).
					SetModTime(f.ModTime.UTC()).
					SetFileID(f.FileID).
					SetEntries(entries).
					SetGeneration(f.Generation)
			}
			err := tx.write.FolderState.CreateBulk(builders...).
				OnConflictColumns(folderstate.FieldLibraryID, folderstate.FieldPath).
				UpdateModTime().UpdateFileID().UpdateEntries().UpdateGeneration().
				Exec(ctx)
			if err != nil {
				return mapErr(err, "put folder states")
			}
		}
		return nil
	})
}

func (r scans) DeleteFolders(ctx context.Context, libraryID core.ID, before int64) error {
	_, err := r.s.write.FolderState.Delete().
		Where(folderstate.LibraryID(libraryID), folderstate.GenerationLT(before)).
		Exec(ctx)
	return mapErr(err, "delete folder states")
}
