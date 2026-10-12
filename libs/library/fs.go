package library

import (
	"context"
	"errors"
	"io/fs"
	"path"
	"path/filepath"

	"github.com/mavioai/mavio/libs/core"
)

func isNotExist(err error) bool { return errors.Is(err, fs.ErrNotExist) }

// rootFS gives access to one library path through its storage, by the
// absolute slash-separated paths below it that items record.
type rootFS struct {
	ctx  context.Context
	s    core.Storage
	base string
}

// rel returns the storage name of an absolute slash path.
func (f rootFS) rel(p string) string {
	name, ok := core.StorageName(f.base, p)
	if !ok {
		return "/" // invalid: no such file
	}
	return name
}

func (f rootFS) stat(p string) (fs.FileInfo, error) { return f.s.Stat(f.ctx, f.rel(p)) }

// List implements Lister.
func (f rootFS) List(dir string) ([]Entry, error) {
	des, err := f.s.ReadDir(f.ctx, f.rel(dir))
	if err != nil {
		return nil, err
	}
	out := make([]Entry, 0, len(des))
	for _, de := range des {
		out = append(out, Entry{Path: path.Join(dir, de.Name()), IsDir: de.IsDir()})
	}
	return out, nil
}

// cleanRoot returns a library path as items below it record it: local
// paths cleaned and slash-separated.
func cleanRoot(p string) string {
	if core.IsRemotePath(p) {
		return p
	}
	return filepath.ToSlash(filepath.Clean(p))
}

func isNotFound(err error) bool { return errors.Is(err, core.ErrNotFound) }

// fileID identifies a file across renames: by the storage's ID, or by
// device and inode; "" when neither is known.
func fileID(info fs.FileInfo) string {
	if id, ok := info.Sys().(core.FileIDer); ok {
		return id.FileID()
	}
	return osFileID(info)
}
