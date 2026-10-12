package library

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

// Storages opens the storage of library paths: local folders, and through
// Remote the folders of rclone remotes. A nil Storages opens local folders
// only.
type Storages struct {
	// Remote opens a folder of an rclone remote, given its name and the
	// path below its root; nil leaves remote paths unavailable.
	Remote func(ctx context.Context, remote, dir string) (core.Storage, error)
}

// Open opens the storage of a library path; the caller closes it.
func (s *Storages) Open(ctx context.Context, root string) (core.Storage, error) {
	if !core.IsRemotePath(root) {
		return OpenLocalStorage(root)
	}
	remote, dir, ok := core.SplitRemotePath(root)
	if !ok {
		return nil, fmt.Errorf("%w: library path %q", core.ErrInvalid, root)
	}
	if s == nil || s.Remote == nil {
		return nil, fmt.Errorf("library path %q: %w: rclone is not set up", root, core.ErrUnsupported)
	}
	return s.Remote(ctx, remote, dir)
}

// OpenFile opens the file name within the storage of the library path
// root; closing the file releases the storage.
func (s *Storages) OpenFile(ctx context.Context, root, name string) (core.File, error) {
	st, err := s.Open(ctx, root)
	if err != nil {
		return nil, err
	}
	f, err := st.Open(ctx, name)
	if err != nil {
		_ = st.Close()
		return nil, err
	}
	return storageFile{f, st}, nil
}

// storageFile is a file whose storage closes with it.
type storageFile struct {
	core.File
	st core.Storage
}

func (f storageFile) Close() error {
	return errors.Join(f.File.Close(), f.st.Close())
}

// RootOf returns which of the library paths roots holds p, a path items
// record, and p's name within its storage; false when none does.
func RootOf(roots []string, p string) (root, name string, ok bool) {
	for _, r := range roots {
		r = cleanRoot(r)
		if name, ok := core.StorageName(r, p); ok && name != "." {
			return r, name, true
		}
	}
	return "", "", false
}

// localStorage is a folder of the server's file system, confined to it
// with os.Root.
type localStorage struct {
	root *os.Root
	dir  string // OS path
}

// OpenLocalStorage opens a folder of the server's file system as a
// storage. Symbolic links are followed while they stay within it.
func OpenLocalStorage(dir string) (core.Storage, error) {
	dir = filepath.Clean(filepath.FromSlash(dir))
	r, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	return &localStorage{root: r, dir: dir}, nil
}

func osName(name string) string { return filepath.FromSlash(name) }

func (l *localStorage) Close() error { return l.root.Close() }

func (l *localStorage) Stat(_ context.Context, name string) (fs.FileInfo, error) {
	return l.root.Stat(osName(name))
}

// ReadDir lists a folder, with symbolic links as what they point to;
// links leaving the folder, or to nothing, are left out.
func (l *localStorage) ReadDir(_ context.Context, name string) ([]fs.DirEntry, error) {
	des, err := fs.ReadDir(l.root.FS(), name)
	if err != nil {
		return nil, err
	}
	out := des[:0]
	for _, de := range des {
		if de.Type()&fs.ModeSymlink != 0 {
			info, err := l.root.Stat(osName(path.Join(name, de.Name())))
			if err != nil {
				continue
			}
			de = linkEntry{name: de.Name(), info: info}
		}
		out = append(out, de)
	}
	return out, nil
}

// linkEntry is a symbolic link listed as its target, under its own name.
type linkEntry struct {
	name string
	info fs.FileInfo
}

func (e linkEntry) Name() string               { return e.name }
func (e linkEntry) IsDir() bool                { return e.info.IsDir() }
func (e linkEntry) Type() fs.FileMode          { return e.info.Mode().Type() }
func (e linkEntry) Info() (fs.FileInfo, error) { return e.info, nil }

func (l *localStorage) Open(_ context.Context, name string) (core.File, error) {
	return l.root.Open(osName(name))
}

// WriteFile writes the file aside and renames it into place.
func (l *localStorage) WriteFile(_ context.Context, name string, data []byte) error {
	if err := l.root.MkdirAll(osName(path.Dir(name)), 0o755); err != nil {
		return err
	}
	tmp := path.Join(path.Dir(name), "."+path.Base(name)+".tmp")
	if err := l.root.WriteFile(osName(tmp), data, 0o644); err != nil {
		return err
	}
	if err := l.root.Rename(osName(tmp), osName(name)); err != nil {
		_ = l.root.Remove(osName(tmp))
		return err
	}
	return nil
}

func (l *localStorage) Create(_ context.Context, name string) (io.WriteCloser, error) {
	return l.root.OpenFile(osName(name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
}

func (l *localStorage) MkdirAll(_ context.Context, name string) error {
	return l.root.MkdirAll(osName(name), 0o755)
}

func (l *localStorage) Rename(_ context.Context, from, to string) error {
	return l.root.Rename(osName(from), osName(to))
}

func (l *localStorage) Copy(context.Context, string, string) error { return core.ErrUnsupported }

func (l *localStorage) Link(_ context.Context, from, to string) error {
	return l.root.Link(osName(from), osName(to))
}

func (l *localStorage) Remove(_ context.Context, name string) error {
	return l.root.Remove(osName(name))
}

func (l *localStorage) Chtimes(_ context.Context, name string, mtime time.Time) error {
	return l.root.Chtimes(osName(name), mtime, mtime)
}

// MediaURL returns a file: URL, so that names with colons are not taken
// for protocols.
func (l *localStorage) MediaURL(_ context.Context, name string) (string, error) {
	return "file:" + l.LocalPath(name), nil
}

// LocalPath returns the OS path of a file of the storage.
func (l *localStorage) LocalPath(name string) string {
	return filepath.Join(l.dir, osName(name))
}

func (l *localStorage) Capabilities() core.StorageCapabilities {
	return core.StorageCapabilities{Local: true, HardLinks: true, AtomicRename: true, FolderModTimes: true}
}
