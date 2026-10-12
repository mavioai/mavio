package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"strings"
	"time"
)

// RemoteScheme prefixes library paths on rclone remotes:
// "rclone:<remote>:<path>", the path below the remote's root
// slash-separated and not empty.
const RemoteScheme = "rclone:"

// ErrUnsupported is returned by storage operations the storage cannot do,
// such as hard links on cloud drives.
var ErrUnsupported = errors.New("not supported by the storage")

// Storage holds the files of a library path: a local folder, or a remote
// the server reaches over the network. Names are slash-separated and
// relative to the library path, valid as [fs.ValidPath] says ("." is the
// library path itself). Errors wrap [fs.ErrNotExist] for missing files.
type Storage interface {
	Stat(ctx context.Context, name string) (fs.FileInfo, error)
	ReadDir(ctx context.Context, name string) ([]fs.DirEntry, error)
	Open(ctx context.Context, name string) (File, error)
	// WriteFile writes a small file, replacing what was there; storages
	// with atomic renames write it aside first, so readers see the old
	// file or the new one.
	WriteFile(ctx context.Context, name string, data []byte) error
	// Create creates a new file for writing a large one, failing with
	// [fs.ErrExist] when it exists. The file is complete once closed.
	Create(ctx context.Context, name string) (io.WriteCloser, error)
	MkdirAll(ctx context.Context, name string) error
	// Rename moves a file within the storage, replacing the target.
	Rename(ctx context.Context, from, to string) error
	// Copy copies a file within the storage without passing its bytes
	// through the server; [ErrUnsupported] when the storage cannot.
	Copy(ctx context.Context, from, to string) error
	// Link hard links a file within the storage; [ErrUnsupported] when
	// the storage cannot.
	Link(ctx context.Context, from, to string) error
	Remove(ctx context.Context, name string) error
	Chtimes(ctx context.Context, name string, mtime time.Time) error
	// MediaURL returns the input that ffmpeg and ffprobe read the file
	// from: a file: URL, or an HTTP URL valid while the server runs.
	MediaURL(ctx context.Context, name string) (string, error)
	Capabilities() StorageCapabilities
	// Close releases the storage.
	Close() error
}

// LocalPath returns the OS path of a file of a local storage, false for
// other storages.
func LocalPath(s Storage, name string) (string, bool) {
	l, ok := s.(interface{ LocalPath(name string) string })
	if !ok {
		return "", false
	}
	return l.LocalPath(name), true
}

// File is an open file of a storage. Reads at offsets are ranged
// requests on remote storage.
type File interface {
	io.ReadSeekCloser
	io.ReaderAt
	Stat() (fs.FileInfo, error)
}

// StorageCapabilities say what a storage does beyond reading and writing
// files.
type StorageCapabilities struct {
	// Local storages are folders of the server's file system, read
	// through OS paths: device detection, prefetching and keeping disks
	// awake apply to them.
	Local bool
	// ServerCopy and HardLinks: Copy and Link work.
	ServerCopy, HardLinks bool
	// AtomicRename: a rename replaces its target at once.
	AtomicRename bool
	// FolderModTimes: a folder's modification time changes when entries
	// are added to or removed from it, so unchanged folders need no
	// listing.
	FolderModTimes bool
}

// StorageTransfers is implemented by storages that move and copy files
// from other paths of their backend, such as another folder of the same
// rclone remote, without passing their bytes through the server. src is a
// path as items record it; [ErrUnsupported] when the storage cannot reach
// it so.
type StorageTransfers interface {
	MoveFrom(ctx context.Context, src, to string) error
	CopyFrom(ctx context.Context, src, to string) error
}

// FileIDer is implemented by the [fs.FileInfo.Sys] of storages that know
// a file's identity across renames.
type FileIDer interface {
	FileID() string
}

// IsRemotePath reports whether a library path is on an rclone remote.
func IsRemotePath(p string) bool { return strings.HasPrefix(p, RemoteScheme) }

// SplitRemotePath splits a remote library path into the remote's name and
// the path below its root.
func SplitRemotePath(p string) (remote, rest string, ok bool) {
	s, ok := strings.CutPrefix(p, RemoteScheme)
	if !ok {
		return "", "", false
	}
	remote, rest, ok = strings.Cut(s, ":")
	return remote, rest, ok && remote != ""
}

// validLibraryPath checks a library path: absolute, or a remote path below
// the remote's root.
func validLibraryPath(p string) error {
	if !IsRemotePath(p) {
		if !strings.HasPrefix(p, "/") && !isWindowsAbs(p) {
			return fmt.Errorf("%w: library path %q is not absolute", ErrInvalid, p)
		}
		return nil
	}
	remote, rest, ok := SplitRemotePath(p)
	switch {
	case !ok || strings.ContainsAny(remote, `/\`):
		return fmt.Errorf("%w: library path %q names no remote", ErrInvalid, p)
	case rest == "" || rest != path.Clean(rest) || strings.HasPrefix(rest, "/") || strings.HasPrefix(rest, "../") || rest == "..":
		return fmt.Errorf("%w: library path %q needs a clean folder below the remote's root", ErrInvalid, p)
	}
	return nil
}

// isWindowsAbs reports whether p is a Windows path with a drive letter or
// a UNC path.
func isWindowsAbs(p string) bool {
	if len(p) >= 3 && p[1] == ':' && (p[2] == '\\' || p[2] == '/') {
		c := p[0] | 0x20
		return c >= 'a' && c <= 'z'
	}
	return strings.HasPrefix(p, `\\`) || strings.HasPrefix(p, "//")
}

// StorageName returns the name of the file at p within the storage of the
// library path root, false when p is not root or below it.
func StorageName(root, p string) (string, bool) {
	root = strings.TrimRight(root, `/\`)
	if p == root || strings.TrimRight(p, `/\`) == root {
		return ".", true
	}
	rest, ok := strings.CutPrefix(p, root)
	if !ok || rest == "" || (rest[0] != '/' && rest[0] != '\\') {
		return "", false
	}
	name := strings.ReplaceAll(strings.TrimLeft(rest, `/\`), `\`, "/")
	if !fs.ValidPath(name) {
		return "", false
	}
	return name, true
}

// StoragePath returns the path of the file name within the storage of the
// library path root, as items and media sources record it.
func StoragePath(root, name string) string {
	root = strings.TrimRight(root, `/\`)
	if name == "." || name == "" {
		return root
	}
	return root + "/" + name
}

// StorageFS returns the storage as an [fs.FS] whose calls use ctx.
func StorageFS(ctx context.Context, s Storage) fs.FS { return storageFS{ctx, s} }

type storageFS struct {
	ctx context.Context
	s   Storage
}

func (f storageFS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	info, err := f.s.Stat(f.ctx, name)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return &storageDir{fs: f, name: name, info: info}, nil
	}
	return f.s.Open(f.ctx, name)
}

func (f storageFS) Stat(name string) (fs.FileInfo, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrInvalid}
	}
	return f.s.Stat(f.ctx, name)
}

func (f storageFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrInvalid}
	}
	return f.s.ReadDir(f.ctx, name)
}

func (f storageFS) ReadFile(name string) ([]byte, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "read", Path: name, Err: fs.ErrInvalid}
	}
	file, err := f.s.Open(f.ctx, name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(file)
}

// storageDir is a folder opened through StorageFS.
type storageDir struct {
	fs      storageFS
	name    string
	info    fs.FileInfo
	entries []fs.DirEntry
	read    bool
}

func (d *storageDir) Stat() (fs.FileInfo, error) { return d.info, nil }
func (d *storageDir) Close() error               { return nil }

func (d *storageDir) Read([]byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: d.name, Err: errors.New("is a directory")}
}

func (d *storageDir) ReadDir(n int) ([]fs.DirEntry, error) {
	if !d.read {
		entries, err := d.fs.s.ReadDir(d.fs.ctx, d.name)
		if err != nil {
			return nil, err
		}
		d.entries, d.read = entries, true
	}
	if n <= 0 {
		out := d.entries
		d.entries = nil
		return out, nil
	}
	if len(d.entries) == 0 {
		return nil, io.EOF
	}
	n = min(n, len(d.entries))
	out := d.entries[:n]
	d.entries = d.entries[n:]
	return out, nil
}
