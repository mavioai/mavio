package rclone

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
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
	// MediaURL returns the URL ffmpeg reads the object remote of the rclone
	// file system fsName from, such as Proxy.URL; nil gives no URLs.
	MediaURL func(fsName, remote string) (string, error)
	// now is the clock; time.Now when nil.
	now func() time.Time

	mu       sync.Mutex
	features map[string]cachedFeatures
}

type cachedFeatures struct {
	caps core.StorageCapabilities
	at   time.Time
}

// featuresTTL is how long a remote's features are kept.
const featuresTTL = time.Minute

// Open opens the storage of folder dir of remote, which must exist.
func (r *Remotes) Open(ctx context.Context, remote, dir string) (core.Storage, error) {
	caps, err := r.capabilities(ctx, remote)
	if err != nil {
		return nil, err
	}
	s := &Storage{r: r, remote: remote, dir: dir, caps: caps}
	info, err := s.Stat(ctx, ".")
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, &fs.PathError{Op: "open", Path: s.fsName(), Err: errors.New("not a folder")}
	}
	return s, nil
}

// Forget drops what is known of a remote, after its configuration changed.
func (r *Remotes) Forget(remote string) {
	r.mu.Lock()
	delete(r.features, remote)
	r.mu.Unlock()
}

func (r *Remotes) capabilities(ctx context.Context, remote string) (core.StorageCapabilities, error) {
	now := time.Now
	if r.now != nil {
		now = r.now
	}
	r.mu.Lock()
	c, ok := r.features[remote]
	r.mu.Unlock()
	if ok && now().Sub(c.at) < featuresTTL {
		return c.caps, nil
	}
	var info struct {
		Features map[string]bool `json:"Features"`
	}
	if err := r.Client.Call(ctx, "operations/fsinfo", map[string]any{"fs": remote + ":"}, &info); err != nil {
		return core.StorageCapabilities{}, err
	}
	caps := core.StorageCapabilities{ServerCopy: info.Features["Copy"]}
	r.mu.Lock()
	if r.features == nil {
		r.features = map[string]cachedFeatures{}
	}
	r.features[remote] = cachedFeatures{caps, now()}
	r.mu.Unlock()
	return caps, nil
}

// Storage is a folder of a remote. It implements core.Storage and
// core.StorageTransfers.
type Storage struct {
	r      *Remotes
	remote string
	dir    string
	caps   core.StorageCapabilities
}

// fsName is the rclone file system of the folder.
func (s *Storage) fsName() string { return s.remote + ":" + s.dir }

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
	if err := s.r.Client.Call(ctx, "operations/stat", map[string]any{"fs": s.fsName(), "remote": remote, "opt": listOpt}, &out); err != nil {
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
	if err := s.r.Client.Call(ctx, "operations/list", map[string]any{"fs": s.fsName(), "remote": remote, "opt": listOpt}, &out); err != nil {
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
	return &file{ctx: context.WithoutCancel(ctx), c: s.r.Client, url: s.r.Client.objectURL(s.fsName(), name), info: info}, nil
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
	q := url.Values{"fs": {s.fsName()}, "remote": {strings.TrimSuffix(dir, "/")}}
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
	return s.call(ctx, "operations/mkdir", "mkdir", name, map[string]any{"fs": s.fsName(), "remote": remote})
}

func (s *Storage) call(ctx context.Context, method, op, name string, in map[string]any) error {
	if err := s.r.Client.Call(ctx, method, in, nil); err != nil {
		return &fs.PathError{Op: op, Path: name, Err: err}
	}
	return nil
}

// Rename implements core.Storage.
func (s *Storage) Rename(ctx context.Context, from, to string) error {
	return s.transfer(ctx, "operations/movefile", s.fsName(), from, to)
}

// Copy implements core.Storage; core.ErrUnsupported when the remote cannot
// copy files itself.
func (s *Storage) Copy(ctx context.Context, from, to string) error {
	if !s.caps.ServerCopy {
		return core.ErrUnsupported
	}
	return s.transfer(ctx, "operations/copyfile", s.fsName(), from, to)
}

// transfer runs movefile or copyfile from the file from of srcFs to to.
func (s *Storage) transfer(ctx context.Context, method, srcFs, from, to string) error {
	if !fs.ValidPath(from) && srcFs == s.fsName() {
		return &fs.PathError{Op: "transfer", Path: from, Err: fs.ErrInvalid}
	}
	if _, err := rel("transfer", to); err != nil || to == "." {
		return &fs.PathError{Op: "transfer", Path: to, Err: fs.ErrInvalid}
	}
	err := s.r.Client.Run(ctx, method, map[string]any{"srcFs": srcFs, "srcRemote": from, "dstFs": s.fsName(), "dstRemote": to})
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
	return s.call(ctx, method, "remove", name, map[string]any{"fs": s.fsName(), "remote": remote})
}

// Chtimes implements core.Storage: the rc API sets no times.
func (*Storage) Chtimes(context.Context, string, time.Time) error { return core.ErrUnsupported }

// MediaURL implements core.Storage.
func (s *Storage) MediaURL(_ context.Context, name string) (string, error) {
	if _, err := rel("media", name); err != nil {
		return "", err
	}
	if s.r.MediaURL == nil {
		return "", core.ErrUnsupported
	}
	return s.r.MediaURL(s.fsName(), name)
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
type file struct {
	ctx  context.Context
	c    *Client
	url  string
	info fs.FileInfo

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
	resp, err := f.c.http.Do(req)
	if err != nil {
		return nil, err
	}
	switch {
	case resp.StatusCode == http.StatusPartialContent:
	case resp.StatusCode == http.StatusOK && off == 0:
	default:
		_ = resp.Body.Close()
		e := &Error{Method: "serve", Status: resp.StatusCode, Message: resp.Status}
		return nil, &fs.PathError{Op: "read", Path: f.info.Name(), Err: e}
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
