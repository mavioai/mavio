package images

import (
	"container/list"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/egoist/mygo/ui"
)

// maxBytes caps an image's download.
const maxBytes = 32 << 20

// Loader loads images by URL.
type Loader struct {
	ctx    context.Context
	client *http.Client
	dir    string
	update func(func())
	slots  chan struct{}
	limit  int

	// Owned by the main thread.
	entries map[string]*list.Element
	order   *list.List
}

type entry struct {
	key    string
	bitmap *ui.Bitmap
	failed bool
}

// New returns a Loader that caches images in dir, keeps up to limit of them
// in memory and gives what it loaded to the view through update, which runs
// a function on the main thread and draws the window again. Loads stop when
// ctx ends.
func New(ctx context.Context, client *http.Client, dir string, limit int, update func(func())) *Loader {
	return &Loader{
		ctx:     ctx,
		client:  client,
		dir:     dir,
		update:  update,
		slots:   make(chan struct{}, 6),
		limit:   limit,
		entries: map[string]*list.Element{},
		order:   list.New(),
	}
}

// Get returns the image at url sized for a box width DIPs wide on a display
// of scale, starting to load it when it is not yet loaded; it returns nil
// until it loads, and for an image that failed. Call it on the main thread.
func (l *Loader) Get(url string, width, scale float32) *ui.Bitmap {
	if url == "" {
		return nil
	}
	url = Sized(url, width, scale)
	if e, ok := l.entries[url]; ok {
		l.order.MoveToFront(e)
		return e.Value.(*entry).bitmap
	}
	e := &entry{key: url}
	l.entries[url] = l.order.PushFront(e)
	for l.order.Len() > l.limit {
		last := l.order.Back()
		l.order.Remove(last)
		delete(l.entries, last.Value.(*entry).key)
	}
	go l.load(e)
	return nil
}

// Failed reports whether the image at url, sized as Get sizes it, failed
// to load.
func (l *Loader) Failed(url string, width, scale float32) bool {
	e, ok := l.entries[Sized(url, width, scale)]
	return ok && e.Value.(*entry).failed
}

// Sized returns url asking for an image width DIPs wide on a display of
// scale, in steps so that close widths share an image.
func Sized(url string, width, scale float32) string {
	if width <= 0 {
		return url
	}
	px := int(width * max(scale, 1))
	step := 120
	px = (px + step - 1) / step * step
	sep := "?"
	if strings.Contains(url, "?") {
		sep = "&"
	}
	return url + sep + "maxWidth=" + strconv.Itoa(px)
}

func (l *Loader) load(e *entry) {
	select {
	case l.slots <- struct{}{}:
	case <-l.ctx.Done():
		return
	}
	defer func() { <-l.slots }()
	b, err := l.read(e.key)
	var bitmap *ui.Bitmap
	if err == nil {
		bitmap, err = ui.DecodeBitmap(b)
	}
	if err != nil {
		if l.ctx.Err() == nil {
			slog.WarnContext(l.ctx, "load an image", "url", e.key, "error", err)
		}
		l.update(func() { e.failed = true })
		return
	}
	l.update(func() { e.bitmap = bitmap })
}

// read returns the image at url from the disk cache, else downloads it into
// the cache.
func (l *Loader) read(url string) ([]byte, error) {
	sum := sha256.Sum256([]byte(url))
	name := hex.EncodeToString(sum[:])
	path := filepath.Join(l.dir, name[:2], name)
	if b, err := os.ReadFile(path); err == nil {
		return b, nil
	}
	req, err := http.NewRequestWithContext(l.ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	res, err := l.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("the server answered %s", res.Status)
	}
	b, err := io.ReadAll(io.LimitReader(res.Body, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(b) > maxBytes {
		return nil, errors.New("the image is too large")
	}
	if err := write(path, b); err != nil {
		slog.WarnContext(l.ctx, "cache an image", "error", err)
	}
	return b, nil
}

var writeMu sync.Mutex

func write(path string, b []byte) error {
	writeMu.Lock()
	defer writeMu.Unlock()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
