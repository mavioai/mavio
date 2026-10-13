package rclone

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/url"
	"path"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

// Remotes opens the storages of library folders on the remotes of an rcd.
type Remotes struct {
	Client *Client
	// MediaURL returns the URL ffmpeg reads an object from, such as
	// Proxy.URL; nil gives no URLs.
	MediaURL func(ctx context.Context, m Media) (string, error)
	// Blocks is the cache the files of folders on cloud storage are read
	// through; nil reads them from the rcd.
	Blocks *BlockCache
	// Class returns the storage class of a library path
	// ("rclone:<remote>:<path>"); nil takes every remote for cloud
	// storage. Only folders on cloud storage are read through Blocks.
	Class func(ctx context.Context, path string) core.StorageClass
	// Jobs is the ledger of transfers, which finds them again after the
	// server restarted; nil keeps none.
	Jobs core.StorageJobRepository
	// Log is where failures of the ledger go; slog.Default when nil.
	Log *slog.Logger
	// now is the clock; time.Now when nil.
	now func() time.Time

	mu    sync.Mutex
	cache map[string]cachedFeatures
}

type cachedFeatures struct {
	features map[string]bool
	at       time.Time
}

// featuresTTL is how long a remote's features are kept.
const featuresTTL = time.Minute

// Open opens the storage of folder dir of remote, which must exist.
func (r *Remotes) Open(ctx context.Context, remote, dir string) (core.Storage, error) {
	f, err := r.features(ctx, remote)
	if err != nil {
		return nil, err
	}
	s := &Storage{r: r, remote: remote, dir: dir, caps: core.StorageCapabilities{ServerCopy: f["Copy"]}}
	if r.Blocks != nil && (r.Class == nil || r.Class(ctx, "rclone:"+remote+":"+dir) == core.StorageCloud) {
		s.blocks, s.seq = r.Blocks, f["SeekReadsFromStart"]
	}
	info, err := s.Stat(ctx, ".")
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, &fs.PathError{Op: "open", Path: s.fsName() + s.dir, Err: errors.New("not a folder")}
	}
	return s, nil
}

// Forget drops what is known of a remote, after its configuration changed.
func (r *Remotes) Forget(remote string) {
	r.mu.Lock()
	delete(r.cache, remote)
	r.mu.Unlock()
}

// OfflineDownloads reports whether a remote's backend downloads links
// itself (operations/addurl).
func (r *Remotes) OfflineDownloads(ctx context.Context, remote string) (bool, error) {
	f, err := r.features(ctx, remote)
	return f["AddURL"], err
}

// features returns the optional features of a remote's backend, as
// operations/fsinfo tells them.
func (r *Remotes) features(ctx context.Context, remote string) (map[string]bool, error) {
	now := time.Now
	if r.now != nil {
		now = r.now
	}
	r.mu.Lock()
	c, ok := r.cache[remote]
	r.mu.Unlock()
	if ok && now().Sub(c.at) < featuresTTL {
		return c.features, nil
	}
	var info struct {
		Features map[string]bool `json:"Features"`
	}
	if err := r.Client.Call(ctx, "operations/fsinfo", map[string]any{"fs": remote + ":"}, &info); err != nil {
		return nil, err
	}
	r.mu.Lock()
	if r.cache == nil {
		r.cache = map[string]cachedFeatures{}
	}
	r.cache[remote] = cachedFeatures{info.Features, now()}
	r.mu.Unlock()
	return info.Features, nil
}

// Storage is a folder of a remote. It implements core.Storage and
// core.StorageTransfers.
type Storage struct {
	r      *Remotes
	remote string
	dir    string
	caps   core.StorageCapabilities
	// blocks is the cache the folder's files are read through, nil when
	// they are read from the rcd; seq tells a remote that reads from
	// the start to reach an offset.
	blocks *BlockCache
	seq    bool
}

// fsName is the rclone file system of the folder: the remote's root, so
// that every folder of a remote shares one file system in the rcd, and
// its caches.
func (s *Storage) fsName() string { return s.remote + ":" }

// abs returns the path from the remote's root of remote, a path of the
// folder.
func (s *Storage) abs(remote string) string { return path.Join(s.dir, remote) }

// rel returns the rclone path of name, "" for the folder itself.
func rel(op, name string) (string, error) {
	if !fs.ValidPath(name) {
		return "", &fs.PathError{Op: op, Path: name, Err: fs.ErrInvalid}
	}
	if name == "." {
		return "", nil
	}
	return name, nil
}

// item is an entry of operations/list and operations/stat.
type item struct {
	Name    string    `json:"Name"`
	Size    int64     `json:"Size"`
	ModTime time.Time `json:"ModTime"`
	IsDir   bool      `json:"IsDir"`
	ID      string    `json:"ID"`
	OrigID  string    `json:"OrigID"`
}

var listOpt = map[string]any{"showOrigIDs": true, "noMimeType": true}

// Stat implements core.Storage.
func (s *Storage) Stat(ctx context.Context, name string) (fs.FileInfo, error) {
	remote, err := rel("stat", name)
	if err != nil {
		return nil, err
	}
	var out struct {
		Item *item `json:"item"`
	}
	if err := s.r.Client.Call(ctx, "operations/stat", map[string]any{"fs": s.fsName(), "remote": s.abs(remote), "opt": listOpt}, &out); err != nil {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: err}
	}
	if out.Item == nil {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrNotExist}
	}
	if name == "." {
		out.Item.Name = "."
	}
	return fileInfo{*out.Item}, nil
}

// ReadDir implements core.Storage.
func (s *Storage) ReadDir(ctx context.Context, name string) ([]fs.DirEntry, error) {
	remote, err := rel("readdir", name)
	if err != nil {
		return nil, err
	}
	var out struct {
		List []item `json:"list"`
	}
	if err := s.r.Client.Call(ctx, "operations/list", map[string]any{"fs": s.fsName(), "remote": s.abs(remote), "opt": listOpt}, &out); err != nil {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: err}
	}
	entries := make([]fs.DirEntry, 0, len(out.List))
	for _, it := range out.List {
		entries = append(entries, fs.FileInfoToDirEntry(fileInfo{it}))
	}
	slices.SortFunc(entries, func(a, b fs.DirEntry) int { return strings.Compare(a.Name(), b.Name()) })
	return entries, nil
}

// Open implements core.Storage.
func (s *Storage) Open(ctx context.Context, name string) (core.File, error) {
	info, err := s.Stat(ctx, name)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, &fs.PathError{Op: "open", Path: name, Err: errors.New("is a directory")}
	}
	u := s.r.Client.objectURL(s.fsName(), s.abs(name))
	if s.blocks != nil {
		it, err := s.blocks.open(itemKey(s.fsName(), s.abs(name), info.(fileInfo)), u, info.Size(), s.seq)
		if err == nil {
			ahead := aheadOf(core.ReadIntentOf(ctx))
			return &cachedFile{ctx: context.WithoutCancel(ctx), it: it, info: info, ahead: ahead}, nil
		}
		s.blocks.log.WarnContext(ctx, "read past the block cache", "remote", s.remote, "file", name, "err", err)
	}
	return &file{ctx: context.WithoutCancel(ctx), c: s.r.Client, url: u, info: info}, nil
}

// WriteFile implements core.Storage. The remote replaces the file as it
// does: on most cloud drives the old file stays until the upload ends.
func (s *Storage) WriteFile(ctx context.Context, name string, data []byte) error {
	return s.upload(ctx, name, bytes.NewReader(data))
}

// upload uploads r as name, replacing what is there.
func (s *Storage) upload(ctx context.Context, name string, r io.Reader) error {
	if _, err := rel("write", name); err != nil || name == "." {
		return &fs.PathError{Op: "write", Path: name, Err: fs.ErrInvalid}
	}
	dir, base := path.Split(name)
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	go func() {
		part, err := mw.CreateFormFile("file", base)
		if err == nil {
			_, err = io.Copy(part, r)
		}
		if err == nil {
			err = mw.Close()
		}
		_ = pw.CloseWithError(err)
	}()
	q := url.Values{"fs": {s.fsName()}, "remote": {s.abs(dir)}}
	req, err := s.r.Client.request(ctx, http.MethodPost, s.r.Client.methodURL("operations/uploadfile", q), pr)
	if err != nil {
		_ = pr.CloseWithError(err)
		return err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	err = s.r.Client.do(req, "operations/uploadfile", nil)
	_ = pr.CloseWithError(errors.New("upload ended"))
	if err != nil {
		return &fs.PathError{Op: "write", Path: name, Err: err}
	}
	return nil
}

// Create implements core.Storage: what is written streams to the remote,
// and Close returns once the upload ends.
func (s *Storage) Create(ctx context.Context, name string) (io.WriteCloser, error) {
	if _, err := s.Stat(ctx, name); err == nil {
		return nil, &fs.PathError{Op: "create", Path: name, Err: fs.ErrExist}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	pr, pw := io.Pipe()
	w := &uploadWriter{pw: pw, done: make(chan error, 1)}
	go func() {
		err := s.upload(ctx, name, pr)
		_ = pr.CloseWithError(err)
		w.done <- err
	}()
	return w, nil
}

// uploadWriter writes a file Create uploads.
type uploadWriter struct {
	pw   *io.PipeWriter
	done chan error
}

func (w *uploadWriter) Write(p []byte) (int, error) { return w.pw.Write(p) }

func (w *uploadWriter) Close() error {
	_ = w.pw.Close()
	return <-w.done
}

// MkdirAll implements core.Storage.
func (s *Storage) MkdirAll(ctx context.Context, name string) error {
	remote, err := rel("mkdir", name)
	if err != nil {
		return err
	}
	return s.call(ctx, "operations/mkdir", "mkdir", name, map[string]any{"fs": s.fsName(), "remote": s.abs(remote)})
}

func (s *Storage) call(ctx context.Context, method, op, name string, in map[string]any) error {
	if err := s.r.Client.Call(ctx, method, in, nil); err != nil {
		return &fs.PathError{Op: op, Path: name, Err: err}
	}
	return nil
}

// Rename implements core.Storage.
func (s *Storage) Rename(ctx context.Context, from, to string) error {
	return s.within(ctx, "operations/movefile", from, to)
}

// Copy implements core.Storage; core.ErrUnsupported when the remote cannot
// copy files itself.
func (s *Storage) Copy(ctx context.Context, from, to string) error {
	if !s.caps.ServerCopy {
		return core.ErrUnsupported
	}
	return s.within(ctx, "operations/copyfile", from, to)
}

// within runs movefile or copyfile from the file from of the folder to to.
func (s *Storage) within(ctx context.Context, method, from, to string) error {
	if !fs.ValidPath(from) || from == "." {
		return &fs.PathError{Op: "transfer", Path: from, Err: fs.ErrInvalid}
	}
	return s.transfer(ctx, method, s.fsName(), s.abs(from), to)
}

// transfer runs movefile or copyfile from the file from of srcFs to to.
func (s *Storage) transfer(ctx context.Context, method, srcFs, from, to string) error {
	if _, err := rel("transfer", to); err != nil || to == "." {
		return &fs.PathError{Op: "transfer", Path: to, Err: fs.ErrInvalid}
	}
	err := s.r.run(ctx, s.remote, method, map[string]any{"srcFs": srcFs, "srcRemote": from, "dstFs": s.fsName(), "dstRemote": s.abs(to)})
	if err != nil {
		return &fs.PathError{Op: "transfer", Path: to, Err: err}
	}
	return nil
}

// MoveFrom implements core.StorageTransfers for files of remotes of the
// rcd, which moves them itself: within one remote, most cloud drives move
// files without transferring them.
func (s *Storage) MoveFrom(ctx context.Context, src, to string) error {
	return s.from(ctx, "operations/movefile", src, to)
}

// CopyFrom implements core.StorageTransfers like MoveFrom.
func (s *Storage) CopyFrom(ctx context.Context, src, to string) error {
	return s.from(ctx, "operations/copyfile", src, to)
}

func (s *Storage) from(ctx context.Context, method, src, to string) error {
	remote, rest, ok := core.SplitRemotePath(src)
	if !ok {
		return core.ErrUnsupported
	}
	return s.transfer(ctx, method, remote+":", rest, to)
}

// Link implements core.Storage: remotes have no hard links.
func (*Storage) Link(context.Context, string, string) error { return core.ErrUnsupported }

// Remove implements core.Storage, removing files and empty folders.
func (s *Storage) Remove(ctx context.Context, name string) error {
	info, err := s.Stat(ctx, name)
	if err != nil {
		return err
	}
	remote, _ := rel("remove", name)
	method := "operations/deletefile"
	if info.IsDir() {
		method = "operations/rmdir"
	}
	return s.call(ctx, method, "remove", name, map[string]any{"fs": s.fsName(), "remote": s.abs(remote)})
}

// RemoveAll removes a file, or a folder and everything in it; nothing is
// no error.
func (s *Storage) RemoveAll(ctx context.Context, name string) error {
	info, err := s.Stat(ctx, name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	remote, _ := rel("remove", name)
	method := "operations/deletefile"
	if info.IsDir() {
		method = "operations/purge"
	}
	return s.call(ctx, method, "remove", name, map[string]any{"fs": s.fsName(), "remote": s.abs(remote)})
}

// Chtimes implements core.Storage: the rc API sets no times.
func (*Storage) Chtimes(context.Context, string, time.Time) error { return core.ErrUnsupported }

// MediaURL implements core.Storage. The read intent of ctx tells how
// the block cache, if the folder has it, reads ahead of ffmpeg.
func (s *Storage) MediaURL(ctx context.Context, name string) (string, error) {
	if _, err := rel("media", name); err != nil {
		return "", err
	}
	if s.r.MediaURL == nil {
		return "", core.ErrUnsupported
	}
	m := Media{FsName: s.fsName(), Remote: s.abs(name), Intent: core.ReadIntentOf(ctx)}
	if s.blocks != nil {
		info, err := s.Stat(ctx, name)
		if err != nil {
			return "", err
		}
		if !info.IsDir() {
			m.blocks, m.key, m.size, m.seq = s.blocks, itemKey(m.FsName, m.Remote, info.(fileInfo)), info.Size(), s.seq
		}
	}
	return s.r.MediaURL(ctx, m)
}

// Media is an object of a remote that ffmpeg reads.
type Media struct {
	// FsName is the rclone file system, Remote the object's path in it.
	FsName, Remote string
	// Intent is what the object is read for.
	Intent core.ReadIntent

	// blocks is the cache the object is read through, nil when it is
	// read from the rcd; key, size and seq are its item's.
	blocks *BlockCache
	key    string
	size   int64
	seq    bool
}

// Capabilities implements core.Storage.
func (s *Storage) Capabilities() core.StorageCapabilities { return s.caps }

// Close implements core.Storage.
func (*Storage) Close() error { return nil }

// fileInfo is an item as fs.FileInfo.
type fileInfo struct{ it item }

func (f fileInfo) Name() string       { return f.it.Name }
func (f fileInfo) Size() int64        { return f.it.Size }
func (f fileInfo) ModTime() time.Time { return f.it.ModTime }
func (f fileInfo) IsDir() bool        { return f.it.IsDir }

func (f fileInfo) Mode() fs.FileMode {
	if f.it.IsDir {
		return fs.ModeDir | 0o755
	}
	return 0o644
}

// Sys returns the file's identity, a core.FileIDer, where the remote has
// one.
func (f fileInfo) Sys() any {
	id := f.it.OrigID
	if id == "" {
		id = f.it.ID
	}
	if id == "" {
		return nil
	}
	return fileID(id)
}

type fileID string

func (f fileID) FileID() string { return string(f) }

// objectURL is the URL of the rcd's --rc-serve object server for remote of
// fsName.
func (c *Client) objectURL(fsName, remote string) string {
	u := *c.base
	u.Path += "/[" + fsName + "]/" + remote
	return u.String()
}

// file is an object of a remote, read through --rc-serve with ranged
// requests.
//
// The first response names the object's version (its ETag), and every
// later request asks for that version (If-Match): a file changed while
// it is read fails with ErrChanged instead of mixing two versions.
type file struct {
	ctx  context.Context
	c    *Client
	url  string
	info fs.FileInfo
	ver  version

	mu sync.Mutex
	// off is where Read reads next; body reads from bodyOff on.
	off, bodyOff int64
	body         io.ReadCloser
	closed       bool
}

func (f *file) Stat() (fs.FileInfo, error) { return f.info, nil }

// get requests the bytes from off on, n of them unless n is 0.
func (f *file) get(off, n int64) (io.ReadCloser, error) {
	req, err := f.c.request(f.ctx, http.MethodGet, f.url, nil)
	if err != nil {
		return nil, err
	}
	if n > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", off, off+n-1))
	} else if off > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", off))
	}
	f.ver.ask(req)
	resp, err := f.c.http.Do(req)
	if err != nil {
		return nil, err
	}
	switch {
	case resp.StatusCode == http.StatusPartialContent:
	case resp.StatusCode == http.StatusOK && off == 0:
	default:
		_ = resp.Body.Close()
		return nil, &fs.PathError{Op: "read", Path: f.info.Name(), Err: serveError(resp)}
	}
	if err := f.ver.check(resp, f.info.Size()); err != nil {
		_ = resp.Body.Close()
		return nil, &fs.PathError{Op: "read", Path: f.info.Name(), Err: err}
	}
	return resp.Body, nil
}

func (f *file) ReadAt(p []byte, off int64) (int, error) {
	if off < 0 {
		return 0, &fs.PathError{Op: "readat", Path: f.info.Name(), Err: fs.ErrInvalid}
	}
	size := f.info.Size()
	if off >= size {
		return 0, io.EOF
	}
	want := min(int64(len(p)), size-off)
	body, err := f.get(off, want)
	if err != nil {
		return 0, err
	}
	defer body.Close()
	n, err := io.ReadFull(body, p[:want])
	if err != nil {
		return n, fmt.Errorf("read %s: %w", f.info.Name(), err)
	}
	if int64(n) < int64(len(p)) {
		return n, io.EOF
	}
	return n, nil
}

func (f *file) Read(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return 0, fs.ErrClosed
	}
	if f.off >= f.info.Size() {
		return 0, io.EOF
	}
	if f.body != nil && f.bodyOff != f.off {
		_ = f.body.Close()
		f.body = nil
	}
	if f.body == nil {
		body, err := f.get(f.off, 0)
		if err != nil {
			return 0, err
		}
		f.body, f.bodyOff = body, f.off
	}
	n, err := f.body.Read(p)
	f.off += int64(n)
	f.bodyOff = f.off
	if errors.Is(err, io.EOF) && f.off < f.info.Size() {
		_ = f.body.Close()
		f.body = nil
		if n == 0 {
			return 0, io.ErrUnexpectedEOF
		}
		err = nil
	}
	return n, err
}

func (f *file) Seek(offset int64, whence int) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch whence {
	case io.SeekStart:
	case io.SeekCurrent:
		offset += f.off
	case io.SeekEnd:
		offset += f.info.Size()
	default:
		return 0, fs.ErrInvalid
	}
	if offset < 0 {
		return 0, &fs.PathError{Op: "seek", Path: f.info.Name(), Err: fs.ErrInvalid}
	}
	f.off = offset
	return offset, nil
}

func (f *file) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.body != nil {
		_ = f.body.Close()
		f.body = nil
	}
	f.closed = true
	return nil
}

var (
	_ core.Storage          = (*Storage)(nil)
	_ core.StorageTransfers = (*Storage)(nil)
	_ core.File             = (*file)(nil)
)
