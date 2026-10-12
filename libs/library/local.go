package library

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"slices"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/metadata"
)

// Limits on the files local readers read and savers write.
const (
	// MaxLocalFileSize bounds each file.
	MaxLocalFileSize = 1 << 20
	// MaxLocalFiles bounds the files of an item.
	MaxLocalFiles = 16
)

// LocalFile is a file beside the media.
type LocalFile struct {
	// Name is relative to the item's folder, with "/" separators.
	Name    string
	Content []byte
}

// LocalReader reads metadata from files beside the media in formats the
// library does not know, such as a local metadata plugin. Its metadata
// counts after the providers' and before the NFO file's.
type LocalReader interface {
	Name() string
	// Patterns name the files it reads, as path.Match patterns.
	Patterns() []string
	// ReadLocal returns the metadata of an item from the matching files
	// beside it; nil when they hold none. media is the name of the item's
	// file, or of its folder for items that are folders.
	ReadLocal(ctx context.Context, l Lookup, media string, files []LocalFile) (*metadata.Result, error)
}

// Saver writes metadata beside the media in a format of its own, such as
// a metadata saver plugin, in libraries saving local metadata.
type Saver interface {
	Name() string
	// Save returns the files to write in the item's folder for its
	// metadata. media is as for LocalReader.
	Save(ctx context.Context, media string, res *metadata.Result) ([]LocalFile, error)
}

func (r *Refresher) localReaders() []LocalReader {
	if r.Local == nil {
		return nil
	}
	return r.Local()
}

func (r *Refresher) savers() []Saver {
	if r.Savers == nil {
		return nil
	}
	return r.Savers()
}

// itemFolder opens the storage of the library folder holding an item and
// returns the item's folder and media within it: the item's folder is the
// item itself when it is a folder.
func itemFolder(ctx context.Context, storages *Storages, lib core.Library, it core.Item) (st core.Storage, dir, rel string, err error) {
	root, rel, ok := libraryRoot(lib, it.Path)
	if !ok {
		return nil, "", "", nil
	}
	if st, err = storages.Open(ctx, root); err != nil {
		return nil, "", "", err
	}
	dir = path.Dir(rel)
	if info, err := st.Stat(ctx, rel); err == nil && info.IsDir() {
		dir = rel
	}
	return st, dir, rel, nil
}

// readLocal has the local readers read an item's files, each returning
// its metadata, or nothing; in reader order.
func (r *Refresher) readLocal(ctx context.Context, lib core.Library, it core.Item, l Lookup) []*metadata.Result {
	readers := r.localReaders()
	if len(readers) == 0 {
		return nil
	}
	st, dir, rel, err := itemFolder(ctx, r.Storages, lib, it)
	if err != nil || st == nil {
		if err != nil {
			r.logger().WarnContext(ctx, "reading local metadata failed", "item", it.ID, "err", err)
		}
		return nil
	}
	defer st.Close()
	fsys := core.StorageFS(ctx, st)
	entries, err := st.ReadDir(ctx, dir)
	if err != nil {
		r.logger().WarnContext(ctx, "reading local metadata failed", "item", it.ID, "err", err)
		return nil
	}
	var out []*metadata.Result
	for _, lr := range readers {
		var files []LocalFile
		for _, e := range entries {
			if len(files) == MaxLocalFiles {
				break
			}
			if !e.Type().IsRegular() || !slices.ContainsFunc(lr.Patterns(), func(p string) bool {
				ok, _ := path.Match(p, e.Name())
				return ok
			}) {
				continue
			}
			if info, err := e.Info(); err != nil || info.Size() > MaxLocalFileSize {
				continue
			}
			data, err := fs.ReadFile(fsys, path.Join(dir, e.Name()))
			if err != nil {
				r.logger().WarnContext(ctx, "reading local metadata failed", "item", it.ID, "file", e.Name(), "err", err)
				continue
			}
			files = append(files, LocalFile{Name: e.Name(), Content: data})
		}
		if len(files) == 0 {
			continue
		}
		res, err := lr.ReadLocal(ctx, l, path.Base(rel), files)
		if err != nil {
			r.logger().WarnContext(ctx, "local metadata reader failed", "reader", lr.Name(), "item", it.ID, "err", err)
			continue
		}
		if res != nil {
			out = append(out, res)
		}
	}
	return out
}

// save has the savers write an item's metadata beside it.
func (r *Refresher) save(ctx context.Context, lib core.Library, it core.Item, res *metadata.Result) error {
	savers := r.savers()
	if len(savers) == 0 {
		return nil
	}
	st, dir, rel, err := itemFolder(ctx, r.Storages, lib, it)
	if err != nil || st == nil {
		return err
	}
	defer st.Close()
	for _, s := range savers {
		files, err := s.Save(ctx, path.Base(rel), res)
		if err != nil {
			r.logger().WarnContext(ctx, "metadata saver failed", "saver", s.Name(), "item", it.ID, "err", err)
			continue
		}
		if err := checkSaved(files, dir, rel); err != nil {
			r.logger().WarnContext(ctx, "metadata saver failed", "saver", s.Name(), "item", it.ID, "err", err)
			continue
		}
		for _, f := range files {
			name := path.Join(dir, f.Name)
			if old, err := fs.ReadFile(core.StorageFS(ctx, st), name); err == nil && bytes.Equal(old, f.Content) {
				continue
			}
			if err := st.WriteFile(ctx, name, f.Content); err != nil {
				return fmt.Errorf("save %s of %s: %w", f.Name, it.ID, err)
			}
		}
	}
	return nil
}

// checkSaved checks the files a saver returns: few and small enough, in
// the item's folder dir, and none of them the item's media rel.
func checkSaved(files []LocalFile, dir, rel string) error {
	if len(files) > MaxLocalFiles {
		return fmt.Errorf("%d files; at most %d", len(files), MaxLocalFiles)
	}
	for _, f := range files {
		switch {
		case !filepath.IsLocal(filepath.FromSlash(f.Name)) || path.Clean(f.Name) != f.Name || f.Name == ".":
			return fmt.Errorf("file %q is not a path within the item's folder", f.Name)
		case len(f.Content) > MaxLocalFileSize:
			return fmt.Errorf("file %q has %d bytes; at most %d", f.Name, len(f.Content), MaxLocalFileSize)
		case path.Join(dir, f.Name) == rel:
			return fmt.Errorf("file %q is the item's media", f.Name)
		}
	}
	return nil
}
