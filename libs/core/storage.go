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

// ReadIntent tells a storage what a file is read for, so that storages
// read over the network read ahead to suit. It travels in the context
// of [Storage.Open] and [Storage.MediaURL]; [WithReadIntent] sets it.
type ReadIntent struct {
	Kind ReadKind
	// Bitrate is the media's bitrate in bits per second, zero when
	// unknown; playback reads ahead by it.
	Bitrate int64
}

// ReadKind is what a file is read for.
type ReadKind int

const (
	// ReadAny is a read nothing is known of.
	ReadAny ReadKind = iota
	// ReadProbe reads the head and tail of a media file, as ffprobe does.
	ReadProbe
	// ReadPlayback reads a file on, ahead of a player.
	ReadPlayback
	// ReadBackground reads for a background job, which needs no
	// read-ahead.
	ReadBackground
)

type readIntentKey struct{}

// WithReadIntent returns ctx carrying the intent of the reads made with
// it.
func WithReadIntent(ctx context.Context, intent ReadIntent) context.Context {
	return context.WithValue(ctx, readIntentKey{}, intent)
}

// ReadIntentOf returns the intent ctx carries; ReadAny when none.
func ReadIntentOf(ctx context.Context) ReadIntent {
	intent, _ := ctx.Value(readIntentKey{}).(ReadIntent)
	return intent
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

// Prefetcher is a File of storage that keeps the bytes it reads, such as
// the block cache of rclone remotes. Prefetch has the bytes [off, end)
// fetched without waiting for them, and marks off as where the file is
// read, so that what lies behind it is evicted before what lies ahead.
type Prefetcher interface {
	Prefetch(off, end int64)
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

// StorageJobKind is what a storage job is.
type StorageJobKind string

// Storage job kinds.
const (
	// StorageJobTransfer is a move or copy an rcd runs within its
	// remotes, keyed by its source and target.
	StorageJobTransfer StorageJobKind = "transfer"
	// StorageJobOffline is an offline download a cloud drive runs, keyed
	// by its download client and its torrent's info hash.
	StorageJobOffline StorageJobKind = "offline"
)

// StorageJob is a long operation of a storage backend the server follows,
// kept so that the server finds it again after it or the backend
// restarted: an rcd forgets its jobs when it restarts and numbers new ones
// from 1 again, so a transfer is known by its rcd run (ExecuteID) and job
// number (JobID) together.
type StorageJob struct {
	Kind StorageJobKind
	// Key is unique within the kind.
	Key string
	// Remote is the rclone remote the job runs on.
	Remote string
	// ExecuteID and JobID name a transfer's rcd job; empty and 0 for
	// offline downloads.
	ExecuteID string
	JobID     int64
	// Data is what the job's owner keeps of it, such as an offline
	// download's task ID, title and last seen status.
	Data      map[string]string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Validate checks the job's invariants.
func (j *StorageJob) Validate() error {
	switch {
	case j.Kind != StorageJobTransfer && j.Kind != StorageJobOffline:
		return fmt.Errorf("%w: storage job kind %q", ErrInvalid, j.Kind)
	case j.Key == "":
		return fmt.Errorf("%w: storage job requires a key", ErrInvalid)
	case j.Remote == "":
		return fmt.Errorf("%w: storage job %s requires a remote", ErrInvalid, j.Key)
	}
	return nil
}

// StorageJobRepository stores storage jobs.
type StorageJobRepository interface {
	// Get returns ErrNotFound for a job not stored.
	Get(ctx context.Context, kind StorageJobKind, key string) (StorageJob, error)
	// List returns the jobs of a kind, oldest first.
	List(ctx context.Context, kind StorageJobKind) ([]StorageJob, error)
	// Put stores the job, replacing the one of the same kind and key but
	// keeping its CreatedAt; it sets UpdatedAt.
	Put(ctx context.Context, j *StorageJob) error
	// Delete removes a job; none is no error.
	Delete(ctx context.Context, kind StorageJobKind, key string) error
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
// library path root, false when p is not root or below it; '/' and '\'
// both separate names and match each other.
func StorageName(root, p string) (string, bool) {
	rest, ok := cutFolder(p, root)
	if !ok {
		return "", false
	}
	name := strings.ReplaceAll(strings.TrimLeft(rest, `/\`), `\`, "/")
	if name == "" {
		return ".", true
	}
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
