package rclone

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

// startDaemon starts an rcd of the rclone on PATH with a memory remote
// "mem" and a remote "disk" of a temporary folder, skipping when
// rclone is missing.
func startDaemon(t *testing.T) (*Daemon, string) {
	t.Helper()
	bin, err := exec.LookPath("rclone")
	if err != nil {
		t.Skip("rclone is not installed")
	}
	dir := t.TempDir()
	conf := filepath.Join(dir, "rclone", "rclone.conf")
	d, err := StartDaemon(t.Context(), DaemonOptions{Path: bin, Config: conf, ConfigPass: "secret", CacheDir: filepath.Join(dir, "cache")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	if data, err := os.ReadFile(conf); err != nil || !strings.HasPrefix(string(data), "# Encrypted rclone configuration File") {
		t.Errorf("configuration file = %.40q, %v, want it encrypted", data, err)
	}
	ctx := t.Context()
	if _, err := d.Client().CreateRemote(ctx, "mem", "memory", nil); err != nil {
		t.Fatal(err)
	}
	disk := filepath.Join(dir, "disk")
	if err := os.MkdirAll(disk, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Client().CreateRemote(ctx, "disk", "alias", map[string]string{"remote": disk}); err != nil {
		t.Fatal(err)
	}
	return d, disk
}

func TestStorage(t *testing.T) {
	d, _ := startDaemon(t)
	ctx := t.Context()
	c := d.Client()
	proxy := NewProxy(c, nil)
	if err := proxy.Listen(); err != nil {
		t.Fatal(err)
	}
	defer proxy.Close()
	r := &Remotes{Client: c, MediaURL: proxy.URL}

	if err := c.Call(ctx, "operations/mkdir", map[string]any{"fs": "mem:", "remote": "Movies"}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Open(ctx, "mem", "Missing"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Open(missing) error = %v, want = %v", err, fs.ErrNotExist)
	}
	st, err := r.Open(ctx, "mem", "Movies")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	data := bytes.Repeat([]byte("0123456789"), 100_000)
	w, err := st.Create(ctx, "Film (2020)/film.mkv")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Create(ctx, "Film (2020)/film.mkv"); !errors.Is(err, fs.ErrExist) {
		t.Errorf("Create(existing) error = %v, want = %v", err, fs.ErrExist)
	}
	if err := st.WriteFile(ctx, "Film (2020)/film.nfo", []byte("<movie/>")); err != nil {
		t.Fatal(err)
	}
	if got, err := fs.ReadFile(core.StorageFS(ctx, st), "Film (2020)/film.nfo"); err != nil || string(got) != "<movie/>" {
		t.Errorf("ReadFile() = %q, %v, want = <movie/>", got, err)
	}

	entries, err := st.ReadDir(ctx, "Film (2020)")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if got := strings.Join(names, ","); got != "film.mkv,film.nfo" {
		t.Errorf("ReadDir() = %s, want = film.mkv,film.nfo", got)
	}
	if info, err := st.Stat(ctx, "Film (2020)"); err != nil || !info.IsDir() {
		t.Errorf("Stat(folder) = %v, %v, want a folder", info, err)
	}
	if _, err := st.Stat(ctx, "Film (2020)/none.mkv"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Stat(missing) error = %v, want = %v", err, fs.ErrNotExist)
	}

	f, err := st.Open(ctx, "Film (2020)/film.mkv")
	if err != nil {
		t.Fatal(err)
	}
	b := make([]byte, 5)
	if n, err := f.ReadAt(b, 123_456); err != nil || string(b[:n]) != "67890" {
		t.Errorf("ReadAt() = %q, %v, want = 67890", b[:n], err)
	}
	if _, err := f.ReadAt(b, int64(len(data))-2); !errors.Is(err, io.EOF) {
		t.Errorf("ReadAt(end) error = %v, want = %v", err, io.EOF)
	}
	if _, err := f.Seek(500_000, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	rest, err := io.ReadAll(f)
	if err != nil || !bytes.Equal(rest, data[500_000:]) {
		t.Errorf("ReadAll() after Seek = %d bytes, %v, want = %d", len(rest), err, len(data)-500_000)
	}
	_ = f.Close()

	// ffmpeg reads ranges through the proxy, without the credentials.
	u, err := st.MediaURL(ctx, "Film (2020)/film.mkv")
	if err != nil || !strings.HasSuffix(u, "/film.mkv") {
		t.Fatalf("MediaURL() = %q, %v", u, err)
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	req.Header.Set("Range", "bytes=10-14")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent || string(got) != "01234" {
		t.Errorf("GET %s = %d %q, want = 206 01234", u, resp.StatusCode, got)
	}
	if resp, err := http.Get(proxy.base + "/unknown/film.mkv"); err != nil || resp.StatusCode != http.StatusNotFound {
		t.Errorf("GET unknown token = %v, %v, want = 404", resp, err)
	}

	if err := st.Rename(ctx, "Film (2020)/film.nfo", "Film (2020)/movie.nfo"); err != nil {
		t.Fatal(err)
	}
	if err := st.Copy(ctx, "Film (2020)/movie.nfo", "Film (2020)/copy.nfo"); err != nil {
		t.Fatal(err)
	}
	if err := st.Link(ctx, "Film (2020)/movie.nfo", "x"); !errors.Is(err, core.ErrUnsupported) {
		t.Errorf("Link() error = %v, want = %v", err, core.ErrUnsupported)
	}
	if err := st.Remove(ctx, "Film (2020)/copy.nfo"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Stat(ctx, "Film (2020)/copy.nfo"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Stat(removed) error = %v, want = %v", err, fs.ErrNotExist)
	}

	// Files move in from other folders of the remote.
	if err := c.Call(ctx, "operations/mkdir", map[string]any{"fs": "mem:", "remote": "downloads"}, nil); err != nil {
		t.Fatal(err)
	}
	dl, err := r.Open(ctx, "mem", "downloads")
	if err != nil {
		t.Fatal(err)
	}
	if err := dl.WriteFile(ctx, "new.mkv", []byte("new")); err != nil {
		t.Fatal(err)
	}
	tr := st.(core.StorageTransfers)
	if err := tr.MoveFrom(ctx, "rclone:mem:downloads/new.mkv", "New/new.mkv"); err != nil {
		t.Fatal(err)
	}
	if _, err := dl.Stat(ctx, "new.mkv"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("source after MoveFrom error = %v, want = %v", err, fs.ErrNotExist)
	}
	if info, err := st.Stat(ctx, "New/new.mkv"); err != nil || info.Size() != 3 {
		t.Errorf("Stat(moved) = %v, %v", info, err)
	}
	if err := tr.MoveFrom(ctx, "/local/file.mkv", "x.mkv"); !errors.Is(err, core.ErrUnsupported) {
		t.Errorf("MoveFrom(local) error = %v, want = %v", err, core.ErrUnsupported)
	}
}

func TestStorageWithoutProxy(t *testing.T) {
	d, disk := startDaemon(t)
	ctx := t.Context()
	if err := os.MkdirAll(filepath.Join(disk, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(disk, "sub", "a.mkv"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := &Remotes{Client: d.Client()}
	st, err := r.Open(ctx, "disk", "sub")
	if err != nil {
		t.Fatal(err)
	}
	info, err := st.Stat(ctx, "a.mkv")
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != 1 || info.ModTime().IsZero() {
		t.Errorf("Stat() = %d bytes at %v", info.Size(), info.ModTime())
	}
	if _, err := st.MediaURL(ctx, "a.mkv"); !errors.Is(err, core.ErrUnsupported) {
		t.Errorf("MediaURL() without a proxy error = %v, want = %v", err, core.ErrUnsupported)
	}
}

func TestFileVersions(t *testing.T) {
	d, disk := startDaemon(t)
	ctx := t.Context()
	name := filepath.Join(disk, "a.mkv")
	if err := os.WriteFile(name, []byte("0123456789"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := &Remotes{Client: d.Client()}
	st, err := r.Open(ctx, "disk", "")
	if err != nil {
		t.Fatal(err)
	}
	f, err := st.Open(ctx, "a.mkv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	b := make([]byte, 3)
	if n, err := f.ReadAt(b, 2); err != nil || string(b[:n]) != "234" {
		t.Fatalf("ReadAt() = %q, %v, want = 234", b[:n], err)
	}

	// The file is replaced, and a scan sees the new version.
	if err := os.WriteFile(name, []byte("abcdefghijklmnop"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(name, time.Now(), time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Stat(ctx, "a.mkv"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.ReadAt(b, 4); !errors.Is(err, ErrChanged) {
		t.Errorf("ReadAt() of a replaced file error = %v, want = %v", err, ErrChanged)
	}

	// A file opened since reads the new version.
	g, err := st.Open(ctx, "a.mkv")
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	if n, err := g.ReadAt(b, 4); err != nil || string(b[:n]) != "efg" {
		t.Errorf("ReadAt() = %q, %v, want = efg", b[:n], err)
	}
}

func TestDaemonRestarts(t *testing.T) {
	d, _ := startDaemon(t)
	ctx := t.Context()
	d.mu.Lock()
	old := d.cmd
	_ = old.Process.Kill()
	d.mu.Unlock()
	deadline := time.Now().Add(20 * time.Second)
	for {
		d.mu.Lock()
		restarted := d.cmd != old
		d.mu.Unlock()
		if restarted && d.Client().Noop(ctx) == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("rclone was not restarted")
		}
		time.Sleep(100 * time.Millisecond)
	}
	// The configuration survives the restart.
	remotes, err := d.Client().Remotes(ctx)
	if err != nil || len(remotes) != 2 || remotes[0].Name != "disk" || remotes[1].Type != "memory" {
		t.Errorf("Remotes() = %+v, %v", remotes, err)
	}
}

func TestClientErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if u, p, ok := r.BasicAuth(); !ok || u != "u" || p != "p" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/operations/deletefile":
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"error":"object not found","status":404}`)
		case "/job/status":
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = io.WriteString(w, `{"error":"job not found","status":500}`)
		default:
			_, _ = io.WriteString(w, `{"jobid":7,"executeId":"run1"}`)
		}
	}))
	defer srv.Close()
	c, err := NewClient(Endpoint{URL: srv.URL, User: "u", Password: "p"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx := t.Context()
	if err := c.Call(ctx, "operations/deletefile", map[string]any{}, nil); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Call() error = %v, want = %v", err, fs.ErrNotExist)
	}
	if err := c.Run(ctx, "operations/movefile", nil); !errors.Is(err, ErrJobLost) {
		t.Errorf("Run() error = %v, want = %v", err, ErrJobLost)
	}
	bad, _ := NewClient(Endpoint{URL: srv.URL}, nil)
	var e *Error
	if err := bad.Call(ctx, "rc/noopauth", nil, nil); !errors.As(err, &e) || e.Status != http.StatusUnauthorized {
		t.Errorf("Call() without credentials error = %v, want status 401", err)
	}
	if _, err := NewClient(Endpoint{URL: "ftp://x"}, nil); err == nil {
		t.Error("NewClient(ftp) error = nil")
	}
}

func TestClientKeepsIdleConnections(t *testing.T) {
	for _, u := range []string{"https://rclone.example:5572", "unix:///tmp/rc.sock"} {
		c, err := NewClient(Endpoint{URL: u}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if got := c.http.Transport.(*http.Transport).MaxIdleConnsPerHost; got != maxIdlePerHost {
			t.Errorf("%s: MaxIdleConnsPerHost = %d, want = %d", u, got, maxIdlePerHost)
		}
	}
	own := &http.Transport{}
	c, _ := NewClient(Endpoint{URL: "http://rclone.example"}, own)
	if c.http.Transport != own {
		t.Error("NewClient() replaced the transport it was given")
	}
}

func TestStorageBlockCache(t *testing.T) {
	d, disk := startDaemon(t)
	ctx := t.Context()
	data := make([]byte, 3<<20)
	for i := range data {
		data[i] = byte(i % 251)
	}
	name := filepath.Join(disk, "a.mkv")
	if err := os.WriteFile(name, data, 0o644); err != nil {
		t.Fatal(err)
	}
	bc, err := NewBlockCache(d.Client(), BlockCacheOptions{Dir: t.TempDir(), MaxSize: 1 << 30})
	if err != nil {
		t.Fatal(err)
	}
	defer bc.Close()
	proxy := NewProxy(d.Client(), nil)
	if err := proxy.Listen(); err != nil {
		t.Fatal(err)
	}
	defer proxy.Close()
	class := core.StorageCloud
	r := &Remotes{Client: d.Client(), MediaURL: proxy.URL, Blocks: bc, Class: func(_ context.Context, p string) core.StorageClass {
		if p != "rclone:disk:" {
			t.Errorf("Class(%q), want = rclone:disk:", p)
		}
		return class
	}}
	st, err := r.Open(ctx, "disk", "")
	if err != nil {
		t.Fatal(err)
	}

	f, err := st.Open(core.WithReadIntent(ctx, core.ReadIntent{Kind: core.ReadPlayback}), "a.mkv")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := f.(*cachedFile); !ok {
		t.Errorf("Open() = %T, want = a file read through the block cache", f)
	}
	got, err := io.ReadAll(f)
	if err != nil || !bytes.Equal(got, data) {
		t.Errorf("ReadAll() = %d bytes, %v, want = %d", len(got), err, len(data))
	}
	_ = f.Close()

	// ffprobe reads through the proxy from the cache.
	u, err := st.MediaURL(core.WithReadIntent(ctx, core.ReadIntent{Kind: core.ReadProbe}), "a.mkv")
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	req.Header.Set("Range", "bytes=1000-1009")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent || !bytes.Equal(body, data[1000:1010]) {
		t.Errorf("GET %s = %d %v, want = 206 and bytes 1000-1009", u, resp.StatusCode, body)
	}

	// A file replaced while it is read fails, rather than mixing versions.
	g, err := st.Open(ctx, "a.mkv")
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	if err := os.WriteFile(name, bytes.Repeat([]byte("x"), len(data)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(name, time.Now(), time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Stat(ctx, "a.mkv"); err != nil {
		t.Fatal(err)
	}
	// Forget what g's item holds, so that it fetches again.
	it := g.(*cachedFile).it
	it.mu.Lock()
	it.have = nil
	it.mu.Unlock()
	if _, err := g.ReadAt(make([]byte, 10), 2<<20); !errors.Is(err, ErrChanged) {
		t.Errorf("ReadAt() of a replaced file error = %v, want = %v", err, ErrChanged)
	}

	// Folders on local storage are read from the rcd.
	class = core.StorageLocal
	st, err = r.Open(ctx, "disk", "")
	if err != nil {
		t.Fatal(err)
	}
	h, err := st.Open(ctx, "a.mkv")
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	if _, ok := h.(*file); !ok {
		t.Errorf("Open() on local storage = %T, want = a file read from the rcd", h)
	}
}
