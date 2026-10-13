package rclone

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

// origin is an object --rc-serve serves: ranges, its ETag and If-Match.
type origin struct {
	mu     sync.Mutex
	data   []byte
	etag   string
	fail   int // status of every response, when set
	reqs   int
	served int64
}

func (o *origin) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	o.mu.Lock()
	data, etag, fail := o.data, o.etag, o.fail
	o.reqs++
	o.mu.Unlock()
	if fail != 0 {
		w.Header().Set("Retry-After", "1")
		http.Error(w, http.StatusText(fail), fail)
		return
	}
	w.Header().Set("ETag", etag)
	cw := &countWriter{ResponseWriter: w}
	http.ServeContent(cw, r, "", time.Time{}, bytes.NewReader(data))
	o.mu.Lock()
	o.served += cw.n
	o.mu.Unlock()
}

func (o *origin) stats() (reqs int, served int64) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.reqs, o.served
}

type countWriter struct {
	http.ResponseWriter
	n int64
}

func (w *countWriter) Write(p []byte) (int, error) {
	n, err := w.ResponseWriter.Write(p)
	w.n += int64(n)
	return n, err
}

// newOrigin serves size random bytes; the cache returned reads them.
func newOrigin(t *testing.T, size int, dir string, max int64) (*origin, *BlockCache, string) {
	t.Helper()
	data := make([]byte, size)
	r := rand.NewChaCha8([32]byte{1})
	_, _ = r.Read(data)
	o := &origin{data: data, etag: `"v1"`}
	srv := httptest.NewServer(o)
	t.Cleanup(srv.Close)
	return o, newTestCache(t, srv.URL, dir, max), srv.URL + "/obj"
}

func newTestCache(t *testing.T, base, dir string, max int64) *BlockCache {
	t.Helper()
	c, err := NewClient(Endpoint{URL: base}, nil)
	if err != nil {
		t.Fatal(err)
	}
	bc, err := NewBlockCache(c, BlockCacheOptions{Dir: dir, MaxSize: max})
	if err != nil {
		t.Fatalf("NewBlockCache() error = %v", err)
	}
	bc.diskSpace = func(string) (int64, int64, bool) { return 0, 0, false }
	bc.backoff = func(error, int) time.Duration { return time.Millisecond }
	t.Cleanup(func() { _ = bc.Close() })
	return bc
}

// idle waits for the downloaders of it to end.
func idle(t *testing.T, it *blockItem) {
	t.Helper()
	for range 5000 {
		it.mu.Lock()
		refs := it.refs
		it.mu.Unlock()
		if refs == 0 {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("item %s still in use", it.name)
}

func TestBlockCacheReads(t *testing.T) {
	const size = 40<<20 + 12345
	dir := t.TempDir()
	o, bc, u := newOrigin(t, size, dir, 1<<30)
	ctx := t.Context()

	it, err := bc.open("k", u, size, false)
	if err != nil {
		t.Fatal(err)
	}
	f := &cachedFile{ctx: ctx, it: it, info: fileInfo{item{Name: "a.mkv", Size: size}}, ahead: anyAhead}
	got, err := io.ReadAll(f)
	if err != nil || !bytes.Equal(got, o.data) {
		t.Fatalf("ReadAll() = %d bytes, %v; want = the %d bytes served", len(got), err, size)
	}
	if reqs, served := o.stats(); served != size {
		t.Errorf("served = %d bytes in %d requests, want = %d", served, reqs, size)
	}
	_ = f.Close()
	idle(t, it)
	_ = bc.Close()

	// The cache survives a restart: nothing is read again.
	bc2 := newTestCache(t, strings.TrimSuffix(u, "/obj"), dir, 1<<30)
	it, err = bc2.open("k", u, size, false)
	if err != nil {
		t.Fatal(err)
	}
	reqs, _ := o.stats()
	p := make([]byte, 1000)
	if n, err := it.ReadAt(ctx, nil, p, size-500, 0); n != 500 || !errors.Is(err, io.EOF) || !bytes.Equal(p[:n], o.data[size-500:]) {
		t.Errorf("ReadAt(tail) = %d, %v; want = 500, EOF and the tail", n, err)
	}
	if n, err := it.ReadAt(ctx, nil, p, 7<<20, 0); n != len(p) || err != nil || !bytes.Equal(p, o.data[7<<20:7<<20+1000]) {
		t.Errorf("ReadAt() = %d, %v; want = %d, nil", n, err, len(p))
	}
	if again, _ := o.stats(); again != reqs {
		t.Errorf("requests after the restart = %d, want = 0", again-reqs)
	}
	it.release()
}

func TestBlockCacheSmallReads(t *testing.T) {
	const size = 64 << 20
	o, bc, u := newOrigin(t, size, t.TempDir(), 1<<30)
	it, err := bc.open("k", u, size, false)
	if err != nil {
		t.Fatal(err)
	}
	defer it.release()
	p := make([]byte, 4096)
	for _, off := range []int64{size - 4096, 30 << 20, 30<<20 + 100<<10} {
		if n, err := it.ReadAt(t.Context(), nil, p, off, 0); n != len(p) || err != nil || !bytes.Equal(p, o.data[off:off+4096]) {
			t.Errorf("ReadAt(%d) = %d, %v; want = %d, nil", off, n, err, len(p))
		}
	}
	// Reads fetch their bytes rounded up, and the read after the second
	// finds them: two requests of a megabyte at most.
	if reqs, served := o.stats(); reqs != 2 || served > 2<<20 {
		t.Errorf("served = %d bytes in %d requests, want = at most 2 MiB in 2", served, reqs)
	}
}

func TestBlockCacheChanged(t *testing.T) {
	const size = 8 << 20
	o, bc, u := newOrigin(t, size, t.TempDir(), 1<<30)
	it, err := bc.open("k", u, size, false)
	if err != nil {
		t.Fatal(err)
	}
	defer it.release()
	p := make([]byte, 1000)
	if _, err := it.ReadAt(t.Context(), nil, p, 0, 0); err != nil {
		t.Fatalf("ReadAt() error = %v", err)
	}
	o.mu.Lock()
	o.etag = `"v2"`
	o.mu.Unlock()
	if _, err := it.ReadAt(t.Context(), nil, p, 4<<20, 0); !errors.Is(err, ErrChanged) {
		t.Errorf("ReadAt() after a change error = %v, want = %v", err, ErrChanged)
	}
	if _, err := it.ReadAt(t.Context(), nil, p, 0, 0); !errors.Is(err, ErrChanged) {
		t.Errorf("ReadAt() of present bytes after a change error = %v, want = %v", err, ErrChanged)
	}
}

func TestBlockCacheFailures(t *testing.T) {
	const size = 1 << 20
	o, bc, u := newOrigin(t, size, t.TempDir(), 1<<30)
	o.fail = http.StatusServiceUnavailable
	it, err := bc.open("k", u, size, false)
	if err != nil {
		t.Fatal(err)
	}
	defer it.release()
	var waits []time.Duration
	var mu sync.Mutex
	bc.backoff = func(err error, n int) time.Duration {
		mu.Lock()
		waits = append(waits, backoff(err, n))
		mu.Unlock()
		return time.Millisecond
	}
	var e *Error
	if _, err := it.ReadAt(t.Context(), nil, make([]byte, 10), 0, 0); !errors.As(err, &e) || e.Status != http.StatusServiceUnavailable {
		t.Fatalf("ReadAt() error = %v, want = the 503", err)
	}
	if reqs, _ := o.stats(); reqs != maxErrors {
		t.Errorf("requests = %d, want = %d", reqs, maxErrors)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(waits) == 0 || waits[0] != time.Second {
		t.Errorf("waits = %v, want = the Retry-After of 1s", waits)
	}
}

func TestBlockCacheEviction(t *testing.T) {
	const size = 2 << 20
	_, bc, u := newOrigin(t, size, t.TempDir(), 3<<20)
	read := func(key string) *blockItem {
		t.Helper()
		it, err := bc.open(key, u, size, false)
		if err != nil {
			t.Fatal(err)
		}
		p := make([]byte, size)
		if n, err := it.ReadAt(t.Context(), nil, p, 0, 0); n != size || err != nil {
			t.Fatalf("ReadAt() = %d, %v", n, err)
		}
		return it
	}
	a := read("a")
	a.release()
	idle(t, a)
	b := read("b")
	// a, not in use, made room for b; b, in use, stays while c is read.
	bc.mu.Lock()
	_, hasA := bc.items[a.name]
	bc.mu.Unlock()
	if hasA {
		t.Errorf("item a kept, want it evicted for b")
	}
	c := read("c")
	bc.mu.Lock()
	_, hasB := bc.items[b.name]
	bc.mu.Unlock()
	if !hasB {
		t.Errorf("item b evicted while in use")
	}
	b.release()
	c.release()
	idle(t, b)
	idle(t, c)
	bc.mu.Lock()
	used := bc.used
	bc.mu.Unlock()
	if used > 3<<20 {
		t.Errorf("used = %d, want <= %d", used, 3<<20)
	}
}

func TestBlockCacheEvictsBehindReaders(t *testing.T) {
	const (
		size = 96 << 20
		max  = 48 << 20
	)
	o, bc, u := newOrigin(t, size, t.TempDir(), max)
	it, err := bc.open("k", u, size, false)
	if err != nil {
		t.Fatal(err)
	}
	f := &cachedFile{ctx: t.Context(), it: it, cur: it.attach(0), info: fileInfo{item{Name: "a.mkv", Size: size}}}
	defer f.Close()
	got, err := io.ReadAll(f)
	if err != nil || !bytes.Equal(got, o.data) {
		t.Fatalf("ReadAll() = %d bytes, %v; want = the %d bytes served", len(got), err, size)
	}
	bc.mu.Lock()
	used, noPunch := bc.used, bc.noPunch
	bc.mu.Unlock()
	if noPunch {
		t.Skip("the disk of the test's folder frees no ranges of files")
	}
	if used > max {
		t.Errorf("used = %d, want <= %d", used, max)
	}
	it.mu.Lock()
	head := it.have.present(0, keepHead)
	behind := it.have.present(size-keepBehind, size)
	evicted := !it.have.present(keepHead, keepHead+evictUnit)
	durable := slices.Clone(it.durable)
	it.mu.Unlock()
	if !head || !behind || !evicted {
		t.Errorf("present: head %v, the last %d bytes %v, evicted after the head %v; want = true, true, true", head, keepBehind, behind, evicted)
	}
	// The saved ranges record no evicted data.
	if durable.present(keepHead, keepHead+evictUnit) {
		t.Errorf("saved ranges = %v, want = without the evicted bytes", durable)
	}
	if n, ok := diskUsage(it.dataPath()); ok && n > max+evictUnit {
		t.Errorf("disk usage = %d, want <= %d", n, max+evictUnit)
	}
	// Evicted bytes are fetched again.
	p := make([]byte, 4096)
	if n, err := f.ReadAt(p, 5<<20); n != len(p) || err != nil || !bytes.Equal(p, o.data[5<<20:5<<20+4096]) {
		t.Errorf("ReadAt(evicted) = %d, %v; want = %d, nil and the bytes served", n, err, len(p))
	}
}

func TestCachedFilePrefetch(t *testing.T) {
	const size = 8 << 20
	o, bc, u := newOrigin(t, size, t.TempDir(), 1<<30)
	it, err := bc.open("k", u, size, false)
	if err != nil {
		t.Fatal(err)
	}
	f := &cachedFile{ctx: t.Context(), it: it, cur: it.attach(0), info: fileInfo{item{Name: "a.mkv", Size: size}}}
	defer f.Close()
	f.Prefetch(2<<20, 4<<20)
	present := false
	for range 5000 {
		it.mu.Lock()
		present = it.have.present(2<<20, 4<<20)
		pos := f.cur.pos
		it.mu.Unlock()
		if pos != 2<<20 {
			t.Fatalf("reader at %d, want = %d", pos, 2<<20)
		}
		if present {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if !present {
		t.Fatal("prefetched bytes never arrived")
	}
	if _, served := o.stats(); served > 2<<20 {
		t.Errorf("served = %d bytes, want <= %d", served, 2<<20)
	}
}

func TestRangesRemove(t *testing.T) {
	r := ranges{{0, 10}, {20, 30}, {40, 50}}
	if n := r.remove(5, 45); n != 5+10+5 {
		t.Errorf("remove() = %d, want = %d", n, 20)
	}
	if want := (ranges{{0, 5}, {45, 50}}); !slices.Equal(r, want) {
		t.Errorf("ranges = %v, want = %v", r, want)
	}
}

func TestBlockCacheDownloaders(t *testing.T) {
	const size = 128 << 20
	_, bc, u := newOrigin(t, size, t.TempDir(), 1<<30)
	it, err := bc.open("k", u, size, false)
	if err != nil {
		t.Fatal(err)
	}
	defer it.release()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	p := make([]byte, 10)
	for _, off := range []int64{0, 50 << 20, 100 << 20} {
		if _, err := it.ReadAt(ctx, nil, p, off, 64<<20); err != nil {
			t.Fatalf("ReadAt(%d) error = %v", off, err)
		}
		it.mu.Lock()
		n := len(it.dls)
		it.mu.Unlock()
		if n > maxDownloaders {
			t.Errorf("downloaders = %d, want <= %d", n, maxDownloaders)
		}
	}
}

func TestAheadOf(t *testing.T) {
	for _, tt := range []struct {
		name   string
		intent core.ReadIntent
		want   int64
	}{
		{"any", core.ReadIntent{}, anyAhead},
		{"probe", core.ReadIntent{Kind: core.ReadProbe}, 0},
		{"background", core.ReadIntent{Kind: core.ReadBackground}, 0},
		{"playback", core.ReadIntent{Kind: core.ReadPlayback, Bitrate: 8_000_000}, 30_000_000},
		{"playback, high bitrate", core.ReadIntent{Kind: core.ReadPlayback, Bitrate: 200_000_000}, maxPlaybackAhead},
		{"playback, bitrate unknown", core.ReadIntent{Kind: core.ReadPlayback}, unknownPlaybackAhead},
	} {
		if got := aheadOf(tt.intent); got != tt.want {
			t.Errorf("%s: aheadOf() = %d, want = %d", tt.name, got, tt.want)
		}
	}
}
