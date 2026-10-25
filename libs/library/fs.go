package library

import (
	"context"
	"errors"
	"io/fs"
	"maps"
	"path"
	"path/filepath"
	"sync"

	"github.com/mavioai/mavio/libs/core"
)

func isNotExist(err error) bool { return errors.Is(err, fs.ErrNotExist) }

// rootFS gives access to one library path through its storage, by the
// absolute slash-separated paths below it that items record.
type rootFS struct {
	ctx  context.Context
	s    core.Storage
	base string
	// listed keeps what a scan listed, on storages whose listings tell
	// what Stat does; nil keeps nothing.
	listed *listings
}

// newRootFS returns the rootFS of a scan of the library path base, which
// keeps what it lists when the storage's listings tell what Stat does.
func newRootFS(ctx context.Context, s core.Storage, base string) rootFS {
	f := rootFS{ctx: ctx, s: s, base: base}
	if s.Capabilities().ListedInfo {
		f.listed = &listings{dirs: map[string][]fs.DirEntry{}, infos: map[string]fs.FileInfo{}}
	}
	return f
}

// listings are what a scan listed of a storage whose listings tell what
// Stat does: so that a folder a resolver listed is not listed again when
// it is walked, and no entry listed is stat'ed. Folders are kept until
// they are walked, infos for the scan.
type listings struct {
	mu    sync.Mutex
	dirs  map[string][]fs.DirEntry
	infos map[string]fs.FileInfo
}

// rel returns the storage name of an absolute slash path.
func (f rootFS) rel(p string) string {
	name, ok := core.StorageName(f.base, p)
	if !ok {
		return "/" // invalid: no such file
	}
	return name
}

// stat returns the info of a path, as the listing of its folder told it
// when the scan listed that.
func (f rootFS) stat(p string) (fs.FileInfo, error) {
	if l := f.listed; l != nil {
		l.mu.Lock()
		info, ok := l.infos[p]
		l.mu.Unlock()
		if ok {
			return info, nil
		}
	}
	return f.s.Stat(f.ctx, f.rel(p))
}

// readDir lists a folder, taking it from what the scan listed when walk
// is set, as a scan's walk lists each folder once. It keeps the infos of
// the entries, of all when the folder is listed for a resolver and of the
// subfolders when it is walked: the walk keeps the files' in the folder's
// state.
func (f rootFS) readDir(dir string, walk bool) ([]fs.DirEntry, error) {
	l := f.listed
	if l != nil {
		l.mu.Lock()
		des, ok := l.dirs[dir]
		if ok && walk {
			delete(l.dirs, dir)
		}
		l.mu.Unlock()
		if ok {
			return des, nil
		}
	}
	des, err := f.s.ReadDir(f.ctx, f.rel(dir))
	if err != nil || l == nil {
		return des, err
	}
	infos := make(map[string]fs.FileInfo, len(des))
	for _, de := range des {
		if !walk || de.IsDir() {
			info, err := de.Info()
			if err != nil {
				return nil, err
			}
			infos[path.Join(dir, de.Name())] = info
		}
	}
	l.mu.Lock()
	if !walk {
		l.dirs[dir] = des
	}
	maps.Copy(l.infos, infos)
	l.mu.Unlock()
	return des, nil
}

// keep records the info of a path the scan stat'ed, such as its root's.
func (f rootFS) keep(p string, info fs.FileInfo) {
	if l := f.listed; l != nil {
		l.mu.Lock()
		l.infos[p] = info
		l.mu.Unlock()
	}
}

// List implements Lister.
func (f rootFS) List(dir string) ([]Entry, error) {
	des, err := f.readDir(dir, false)
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
