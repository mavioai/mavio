package rclone

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Sizes and times of the block cache.
const (
	// minFetch is the least a read that misses has fetched.
	minFetch = 256 << 10
	// blockWrite is how much a downloader writes to disk at a time.
	blockWrite = 1 << 20
	// chunkMin and chunkMax bound the ranges a downloader requests: each
	// request doubles the last, so that an abandoned read wastes little.
	chunkMin = 16 << 20
	chunkMax = 256 << 20
	// reuseWindow is how far behind a missing range a downloader may be
	// and still be extended to it, rather than a new one started.
	reuseWindow = 16 << 20
	// maxDownloaders is how many downloaders a file has at most.
	maxDownloaders = 2
	// idleStop is how long a downloader that reached its target waits to
	// be extended.
	idleStop = 5 * time.Second
	// maxErrors is how many failed requests in a row fail the reads
	// waiting for them.
	maxErrors = 10
	// saveEvery is how often an item's present ranges are saved while it
	// downloads.
	saveEvery = 5 * time.Second
	// minFreeShare is the share of the disk the cache leaves free,
	// checked as every diskCheckEvery bytes arrive.
	minFreeShare   = 0.05
	diskCheckEvery = 64 << 20
	// keepBehind is how much of an open item behind its readers eviction
	// keeps, for short rewinds; keepHead how much of its head, which
	// every opening of the file reads. Eviction drops evictUnit pieces,
	// and frees evictSlack bytes past what it must, so that it does not
	// run for every write.
	keepBehind = 16 << 20
	keepHead   = 1 << 20
	evictUnit  = 16 << 20
	evictSlack = 64 << 20
	// DefaultBlockCacheAge is how long items not read are kept.
	DefaultBlockCacheAge = 7 * 24 * time.Hour
)

// BlockCache keeps the bytes read from files of rclone remotes on local
// disk, so that a range is downloaded once however many readers ask for
// it, and read again from disk.
//
// Each file is an item: a sparse file at the file's full size, and the
// ranges of it present, saved beside it so that the cache survives
// restarts. Missing ranges are fetched by downloaders, at most
// maxDownloaders per file, each reading on through ranged requests of
// --rc-serve.
type BlockCache struct {
	c      *Client
	dir    string
	max    int64
	maxAge time.Duration
	log    *slog.Logger
	now    func() time.Time
	// diskSpace returns the free and total bytes of the cache's disk.
	diskSpace func(dir string) (free, total int64, ok bool)
	// backoff is how long a downloader waits after its n-th failed
	// request in a row.
	backoff func(err error, n int) time.Duration

	mu    sync.Mutex
	items map[string]*blockItem
	used  int64 // bytes present in all items
	// unchecked counts the bytes added since the free disk space was
	// last checked.
	unchecked int64
	swept     time.Time
	// noPunch: the disk frees no ranges of files, so open items are
	// kept whole.
	noPunch bool

	// wg counts the running downloaders; closed stops new ones.
	wg     sync.WaitGroup
	closed atomic.Bool
}

// BlockCacheOptions configure a block cache.
type BlockCacheOptions struct {
	// Dir holds the cache's files.
	Dir string
	// MaxSize is how many bytes the cache holds at most.
	MaxSize int64
	// MaxAge is how long items not read are kept; DefaultBlockCacheAge
	// when zero.
	MaxAge time.Duration
	// Logger is where failures go; slog.Default when nil.
	Logger *slog.Logger
}

// NewBlockCache opens the block cache in opts.Dir, reading the items it
// kept, for the rcd c serves.
func NewBlockCache(c *Client, opts BlockCacheOptions) (*BlockCache, error) {
	if opts.MaxAge <= 0 {
		opts.MaxAge = DefaultBlockCacheAge
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if err := os.MkdirAll(opts.Dir, 0o750); err != nil {
		return nil, fmt.Errorf("block cache: %w", err)
	}
	bc := &BlockCache{
		c: c, dir: opts.Dir, max: opts.MaxSize, maxAge: opts.MaxAge, log: opts.Logger,
		now: time.Now, diskSpace: diskSpace, backoff: backoff, items: map[string]*blockItem{},
	}
	if err := bc.load(); err != nil {
		return nil, err
	}
	bc.mu.Lock()
	bc.trim()
	bc.mu.Unlock()
	return bc, nil
}

// Close stops the downloaders and saves what they fetched. Reads that
// miss fail after it.
func (bc *BlockCache) Close() error {
	bc.closed.Store(true)
	bc.mu.Lock()
	for _, it := range bc.items {
		it.mu.Lock()
		for _, d := range it.dls {
			d.stop()
		}
		it.err = errClosed
		it.broadcast()
		it.mu.Unlock()
	}
	bc.mu.Unlock()
	bc.wg.Wait()
	// Items still read are saved now; their last release saves them
	// again.
	bc.mu.Lock()
	items := slices.Collect(maps.Values(bc.items))
	bc.mu.Unlock()
	for _, it := range items {
		it.mu.Lock()
		f := it.f
		it.mu.Unlock()
		if f != nil {
			it.save(f)
		}
	}
	return nil
}

// errClosed fails the reads of a closed cache.
var errClosed = errors.New("block cache closed")

// itemMeta is what an item keeps beside its data.
type itemMeta struct {
	Key    string    `json:"key"`
	Size   int64     `json:"size"`
	ETag   string    `json:"etag,omitempty"`
	Ranges ranges    `json:"ranges"`
	Used   time.Time `json:"used"`
}

// load reads the items kept in the cache's folder, removing those whose
// data is gone.
func (bc *BlockCache) load() error {
	entries, err := os.ReadDir(bc.dir)
	if err != nil {
		return fmt.Errorf("block cache: %w", err)
	}
	for _, e := range entries {
		name, ok := strings.CutSuffix(e.Name(), ".json")
		if !ok {
			continue
		}
		it := &blockItem{bc: bc, name: name}
		var m itemMeta
		data, err := os.ReadFile(it.metaPath())
		if err == nil {
			err = json.Unmarshal(data, &m)
		}
		if err == nil {
			var st os.FileInfo
			if st, err = os.Stat(it.dataPath()); err == nil && st.Size() != m.Size {
				err = errors.New("data has another size")
			}
		}
		if err != nil {
			bc.log.Warn("drop block cache item", "item", name, "err", err)
			it.removeFiles()
			continue
		}
		it.key, it.size, it.etag, it.have, it.durable, it.used = m.Key, m.Size, m.ETag, m.Ranges, slices.Clone(m.Ranges), m.Used
		it.changed = make(chan struct{})
		bc.items[name] = it
		bc.used += it.have.size()
	}
	// Data without its metadata is not known.
	for _, e := range entries {
		if name, ok := strings.CutSuffix(e.Name(), ".data"); ok && bc.items[name] == nil {
			_ = os.Remove(filepath.Join(bc.dir, e.Name()))
		}
	}
	return nil
}

// blockItem is a file of a remote in the cache.
type blockItem struct {
	bc   *BlockCache
	name string // of its files in the cache's folder
	key  string
	url  string
	size int64
	// seq: the remote reads from the start to reach an offset, so the
	// item has one downloader, which reads on rather than jumping.
	seq bool

	mu sync.Mutex
	// refs counts the users of the item, readers and downloaders.
	refs int
	f    *os.File
	have ranges
	// durable is what the saved metadata records present.
	durable ranges
	etag    string
	used    time.Time
	dirty   bool
	saved   time.Time
	changed chan struct{} // closed when the item changes
	dls     []*downloader
	waits   map[*int64]struct{} // offsets reads wait for
	// readers are where the item's readers read; busy the spans read
	// from or written to its data now, which eviction leaves.
	readers map[*cursor]struct{}
	busy    map[*span]struct{}
	// err fails every read: the file changed on the remote.
	err error
	// errs counts failed requests in a row; failGen counts the times
	// they reached maxErrors, failErr is the last of them.
	errs    int
	failGen int
	failErr error
	removed bool

	saveMu sync.Mutex
}

func (it *blockItem) dataPath() string { return filepath.Join(it.bc.dir, it.name+".data") }
func (it *blockItem) metaPath() string { return filepath.Join(it.bc.dir, it.name+".json") }

func (it *blockItem) removeFiles() {
	_ = os.Remove(it.dataPath())
	_ = os.Remove(it.metaPath())
}

// itemKey identifies a version of a file: its remote, path, identity,
// size and modification time.
func itemKey(fsName, remote string, info fileInfo) string {
	return fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%d\x00%d", fsName, remote, info.it.OrigID, info.it.ID, info.it.Size, info.it.ModTime.UnixNano())
}

// open returns the item of the version key of the object at url, of size
// bytes, taking a reference the caller releases.
func (bc *BlockCache) open(key, url string, size int64, seq bool) (*blockItem, error) {
	sum := sha256.Sum256([]byte(key))
	name := hex.EncodeToString(sum[:16])
	bc.mu.Lock()
	defer bc.mu.Unlock()
	bc.sweep()
	it := bc.items[name]
	if it != nil && (it.key != key || it.size != size) {
		bc.remove(it)
		it = nil
	}
	if it == nil {
		it = &blockItem{bc: bc, name: name, key: key, size: size, changed: make(chan struct{})}
		bc.items[name] = it
	}
	it.mu.Lock()
	defer it.mu.Unlock()
	it.url, it.seq = url, seq
	it.used = bc.now()
	if it.f == nil {
		f, err := os.OpenFile(it.dataPath(), os.O_RDWR|os.O_CREATE, 0o640)
		if err == nil {
			err = markSparse(f)
		}
		if err == nil {
			err = f.Truncate(size)
		}
		if err != nil {
			if f != nil {
				_ = f.Close()
			}
			delete(bc.items, name)
			return nil, fmt.Errorf("block cache: %w", err)
		}
		it.f = f
	}
	it.refs++
	return it, nil
}

// release drops a reference; the last one saves the item and closes its
// data, or removes the item if it was dropped.
func (it *blockItem) release() {
	bc := it.bc
	bc.mu.Lock()
	it.mu.Lock()
	it.refs--
	last := it.refs == 0
	removed := it.removed
	var f *os.File
	if last {
		f, it.f = it.f, nil
	}
	it.mu.Unlock()
	if last {
		if !removed {
			it.save(f)
		}
		if f != nil {
			_ = f.Close()
		}
		bc.trim()
	}
	bc.mu.Unlock()
}

// remove drops it from the cache with its files; its users read on
// from the data they hold open, and an item opened for the same version
// starts afresh. Call with bc.mu held.
func (bc *BlockCache) remove(it *blockItem) {
	if bc.items[it.name] == it {
		delete(bc.items, it.name)
	}
	// No save is under way, and none starts.
	it.saveMu.Lock()
	defer it.saveMu.Unlock()
	it.mu.Lock()
	if it.removed {
		it.mu.Unlock()
		return
	}
	bc.used -= it.have.size()
	it.removed = true
	for _, d := range it.dls {
		d.stop()
	}
	it.mu.Unlock()
	it.removeFiles()
}

// sweep removes the items not read for maxAge, at most hourly. Call with
// bc.mu held.
func (bc *BlockCache) sweep() {
	now := bc.now()
	if now.Sub(bc.swept) < time.Hour {
		return
	}
	bc.swept = now
	for _, it := range bc.items {
		it.mu.Lock()
		old := it.refs == 0 && now.Sub(it.used) > bc.maxAge
		it.mu.Unlock()
		if old {
			bc.remove(it)
		}
	}
}

// trim removes the items no one uses, least recently read first, while
// the cache holds more than its size or leaves too little of its disk
// free; then the ranges of open items furthest behind their readers.
// Call with bc.mu held.
func (bc *BlockCache) trim() {
	over := func() bool { return bc.deficit() > 0 }
	if !over() {
		return
	}
	type idleItem struct {
		it   *blockItem
		used time.Time
	}
	idle := make([]idleItem, 0, len(bc.items))
	for _, it := range bc.items {
		it.mu.Lock()
		if it.refs == 0 {
			idle = append(idle, idleItem{it, it.used})
		}
		it.mu.Unlock()
	}
	slices.SortFunc(idle, func(a, b idleItem) int { return a.used.Compare(b.used) })
	for _, i := range idle {
		if !over() {
			return
		}
		bc.remove(i.it)
	}
	if need := bc.deficit(); need > 0 && !bc.noPunch {
		bc.trimOpen(need + evictSlack)
	}
}

// deficit returns how many bytes the cache holds past its size, or past
// the disk space it must leave free, whichever is more; 0 when neither.
// Call with bc.mu held.
func (bc *BlockCache) deficit() int64 {
	need := bc.used - bc.max
	if free, total, ok := bc.diskSpace(bc.dir); ok {
		need = max(need, int64(minFreeShare*float64(total))-free)
	}
	return max(need, 0)
}

// trimOpen drops about need bytes of open items: their ranges behind
// every reader, by keepBehind and past keepHead, those furthest behind
// first. Call with bc.mu held.
func (bc *BlockCache) trimOpen(need int64) {
	type piece struct {
		it     *blockItem
		s      span
		behind int64
	}
	var pieces []piece
	for _, it := range bc.items {
		it.mu.Lock()
		if limit, ok := it.evictLimit(); ok {
			for _, r := range it.have {
				start := max(r.Start, keepHead)
				end := min(r.End, limit)
				// Pieces are whole MiBs, which every file system's
				// blocks divide.
				start = (start + blockWrite - 1) / blockWrite * blockWrite
				end = end / blockWrite * blockWrite
				for start < end {
					e := min((start/evictUnit+1)*evictUnit, end)
					pieces = append(pieces, piece{it, span{start, e}, limit - e})
					start = e
				}
			}
		}
		it.mu.Unlock()
	}
	slices.SortFunc(pieces, func(a, b piece) int { return cmp.Compare(b.behind, a.behind) })
	byItem := map[*blockItem][]span{}
	var items []*blockItem
	for _, p := range pieces {
		if need <= 0 {
			break
		}
		if byItem[p.it] == nil {
			items = append(items, p.it)
		}
		byItem[p.it] = append(byItem[p.it], p.s)
		need -= p.s.End - p.s.Start
	}
	for _, it := range items {
		n, err := it.evict(byItem[it])
		bc.used -= n
		if errors.Is(err, errors.ErrUnsupported) {
			bc.noPunch = true
			bc.log.Info("block cache keeps open items whole: the disk frees no ranges of files", "dir", bc.dir)
			return
		}
		if err != nil {
			bc.log.Warn("evict from block cache item", "item", it.name, "err", err)
		}
	}
}

// evictLimit returns the offset before which an open item's ranges may be
// evicted: keepBehind before its readers; false when it has none. Call
// with it.mu held.
func (it *blockItem) evictLimit() (int64, bool) {
	if it.refs == 0 || it.removed || it.f == nil || len(it.readers) == 0 {
		return 0, false
	}
	lo := it.size
	for c := range it.readers {
		lo = min(lo, c.pos)
	}
	return lo - keepBehind, lo-keepBehind > keepHead
}

// evict drops the spans of the item's data, those still behind its
// readers and not read or written now, and returns the bytes dropped.
// The metadata is saved without them before their data goes, so that it
// never records data that is gone.
func (it *blockItem) evict(spans []span) (int64, error) {
	it.saveMu.Lock()
	defer it.saveMu.Unlock()
	it.mu.Lock()
	defer it.mu.Unlock()
	limit, ok := it.evictLimit()
	if !ok {
		return 0, nil
	}
	spans = slices.DeleteFunc(spans, func(s span) bool {
		if s.End > limit {
			return true
		}
		for b := range it.busy {
			if overlap(*b, s.Start, s.End) > 0 {
				return true
			}
		}
		return false
	})
	if len(spans) == 0 {
		return 0, nil
	}
	durable := slices.Clone(it.durable)
	for _, s := range spans {
		durable.remove(s.Start, s.End)
	}
	if err := it.writeMeta(itemMeta{Key: it.key, Size: it.size, ETag: it.etag, Ranges: durable, Used: it.used}, true); err != nil {
		return 0, err
	}
	it.durable = durable
	var n int64
	for _, s := range spans {
		if err := punchHole(it.f, s.Start, s.End-s.Start); err != nil {
			return n, err
		}
		n += it.have.remove(s.Start, s.End)
	}
	it.dirty = true
	return n, nil
}

// account adds n bytes newly present, trimming the cache when it is full.
func (bc *BlockCache) account(n int64) {
	bc.mu.Lock()
	bc.used += n
	bc.unchecked += n
	if bc.used > bc.max || bc.unchecked >= diskCheckEvery {
		bc.unchecked = 0
		bc.trim()
	}
	bc.mu.Unlock()
}

// save writes the item's metadata, after syncing the data it records.
// f is the item's data, nil when it was not opened.
func (it *blockItem) save(f *os.File) {
	it.saveMu.Lock()
	defer it.saveMu.Unlock()
	it.mu.Lock()
	if !it.dirty || it.removed {
		it.mu.Unlock()
		return
	}
	m := itemMeta{Key: it.key, Size: it.size, ETag: it.etag, Ranges: slices.Clone(it.have), Used: it.used}
	it.dirty = false
	it.saved = it.bc.now()
	it.mu.Unlock()
	// The metadata records no more than the data holds: the data is
	// synced first.
	if f != nil {
		if err := f.Sync(); err != nil {
			it.bc.log.Warn("sync block cache item", "item", it.name, "err", err)
			return
		}
	}
	if err := it.writeMeta(m, false); err != nil {
		it.bc.log.Warn("save block cache item", "item", it.name, "err", err)
		return
	}
	it.mu.Lock()
	it.durable = m.Ranges
	it.mu.Unlock()
}

// writeMeta writes the item's metadata m, synced to disk when sync is set.
func (it *blockItem) writeMeta(m itemMeta, sync bool) error {
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	tmp := it.metaPath() + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o640)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if err == nil && sync {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	return os.Rename(tmp, it.metaPath())
}

// broadcast wakes the reads waiting for the item. Call with it.mu held.
func (it *blockItem) broadcast() {
	close(it.changed)
	it.changed = make(chan struct{})
}

// cursor is where a reader of an item reads, guarded by the item's mu.
type cursor struct {
	pos int64
}

// attach adds a reader of the item reading at pos.
func (it *blockItem) attach(pos int64) *cursor {
	it.mu.Lock()
	defer it.mu.Unlock()
	c := &cursor{pos: pos}
	if it.readers == nil {
		it.readers = map[*cursor]struct{}{}
	}
	it.readers[c] = struct{}{}
	return c
}

// detach removes the reader c.
func (it *blockItem) detach(c *cursor) {
	it.mu.Lock()
	defer it.mu.Unlock()
	delete(it.readers, c)
}

// ReadAt reads len(p) bytes at off, waiting for the downloaders to fetch
// what is missing, and has ahead bytes past them fetched without
// waiting. cur, when not nil, is the reader's, which moves to off.
func (it *blockItem) ReadAt(ctx context.Context, cur *cursor, p []byte, off, ahead int64) (int, error) {
	if cur != nil {
		it.mu.Lock()
		cur.pos = off
		it.mu.Unlock()
	}
	if off >= it.size {
		return 0, io.EOF
	}
	end := min(off+int64(len(p)), it.size)
	// Small reads fetch at least minFetch, so that the reads near them
	// find their bytes.
	aheadEnd := min(max(end+max(ahead, 0), off+minFetch), it.size)
	wait := new(int64)
	it.mu.Lock()
	gen := it.failGen
	for {
		if it.err != nil {
			err := it.err
			it.mu.Unlock()
			return 0, err
		}
		if it.failGen != gen {
			err := it.failErr
			it.mu.Unlock()
			return 0, err
		}
		gap, missing := it.have.missing(off, end)
		if !missing {
			break
		}
		it.fetch(gap.Start, aheadEnd)
		*wait = gap.Start
		if it.waits == nil {
			it.waits = map[*int64]struct{}{}
		}
		it.waits[wait] = struct{}{}
		changed := it.changed
		it.mu.Unlock()
		select {
		case <-changed:
		case <-ctx.Done():
			it.mu.Lock()
			delete(it.waits, wait)
			it.mu.Unlock()
			return 0, ctx.Err()
		}
		it.mu.Lock()
		delete(it.waits, wait)
	}
	if gap, ok := it.have.missing(end, aheadEnd); ok {
		it.fetch(gap.Start, aheadEnd)
	}
	it.used = it.bc.now()
	f := it.f
	// Eviction leaves the span while it is read.
	reading := &span{off, end}
	it.addBusy(reading)
	it.mu.Unlock()
	n, err := f.ReadAt(p[:end-off], off)
	it.mu.Lock()
	delete(it.busy, reading)
	it.mu.Unlock()
	if err == nil && n < len(p) {
		err = io.EOF
	}
	return n, err
}

// addBusy marks s as read or written now. Call with it.mu held.
func (it *blockItem) addBusy(s *span) {
	if it.busy == nil {
		it.busy = map[*span]struct{}{}
	}
	it.busy[s] = struct{}{}
}

// prefetch has the bytes [start, end) fetched without waiting for them;
// cur, when not nil, is the reader's, which moves to start.
func (it *blockItem) prefetch(cur *cursor, start, end int64) {
	it.mu.Lock()
	defer it.mu.Unlock()
	if cur != nil {
		cur.pos = start
	}
	end = min(end, it.size)
	if gap, ok := it.have.missing(max(start, 0), end); ok && it.err == nil {
		it.fetch(gap.Start, end)
	}
}

// fetch has a downloader fetch from start, a missing offset, through end:
// one already reading towards start is extended, else one is started.
// Call with it.mu held.
func (it *blockItem) fetch(start, end int64) {
	if it.bc.closed.Load() {
		return
	}
	for _, d := range it.dls {
		if d.pos <= start && (start-d.pos <= reuseWindow || it.seq) {
			d.extend(end)
			return
		}
	}
	switch {
	case it.seq:
		// The remote reads from the start anyway: read on from the
		// downloader furthest along.
		for _, d := range it.dls {
			d.stop()
		}
		it.dls = nil
	case len(it.dls) >= maxDownloaders:
		victim := slices.MaxFunc(it.dls, func(a, b *downloader) int {
			return cmp.Compare(it.distance(a.pos), it.distance(b.pos))
		})
		victim.stop()
		it.dls = slices.DeleteFunc(it.dls, func(d *downloader) bool { return d == victim })
	}
	d := &downloader{it: it, pos: start, target: end, kick: make(chan struct{}, 1), done: make(chan struct{})}
	it.dls = append(it.dls, d)
	it.refs++ // the downloader's, released when it ends
	it.bc.wg.Add(1)
	go d.run()
}

// distance is how far pos is from the nearest offset a read waits for.
// Call with it.mu held.
func (it *blockItem) distance(pos int64) int64 {
	best := int64(-1)
	for w := range it.waits {
		d := *w - pos
		if d < 0 {
			d = -d
		}
		if best < 0 || d < best {
			best = d
		}
	}
	if best < 0 {
		return 1 << 62
	}
	return best
}

// invalidate fails every read of the item and drops it: the file
// changed on the remote. Call with it.mu held.
func (it *blockItem) invalidate() {
	it.err = ErrChanged
	it.broadcast()
	go func() {
		it.bc.mu.Lock()
		it.bc.remove(it)
		it.bc.mu.Unlock()
	}()
}

// downloader fetches a file's bytes from pos on, through target.
type downloader struct {
	it *blockItem
	// pos and target are guarded by it.mu.
	pos, target int64
	stopped     bool
	kick        chan struct{}
	done        chan struct{}
	once        sync.Once
}

// extend moves the downloader's target to end. Call with it.mu held.
func (d *downloader) extend(end int64) {
	if end > d.target {
		d.target = end
		select {
		case d.kick <- struct{}{}:
		default:
		}
	}
}

// stop ends the downloader. Call with it.mu held.
func (d *downloader) stop() {
	d.stopped = true
	d.once.Do(func() { close(d.done) })
}

// finish removes the downloader from its item. Call with it.mu held.
func (d *downloader) finish() {
	it := d.it
	d.stop()
	it.dls = slices.DeleteFunc(it.dls, func(x *downloader) bool { return x == d })
	it.broadcast()
}

func (d *downloader) run() {
	it := d.it
	defer it.bc.wg.Done()
	defer it.release()
	chunk := int64(chunkMin)
	for {
		it.mu.Lock()
		if d.stopped || it.err != nil {
			d.finish()
			it.mu.Unlock()
			return
		}
		if d.pos >= it.size {
			d.finish()
			it.mu.Unlock()
			return
		}
		if d.pos >= d.target {
			it.mu.Unlock()
			select {
			case <-d.kick:
				continue
			case <-d.done:
				it.mu.Lock()
				d.finish()
				it.mu.Unlock()
				return
			case <-time.After(idleStop):
				it.mu.Lock()
				if d.pos >= d.target || d.pos >= it.size {
					d.finish()
					it.mu.Unlock()
					return
				}
				it.mu.Unlock()
				continue
			}
		}
		// A downloader that runs into present data ends.
		if it.have.present(d.pos, min(d.pos+blockWrite, it.size)) {
			d.finish()
			it.mu.Unlock()
			return
		}
		// A request asks for no more than the target, rounded up, so
		// that short reads fetch little; reading on, requests grow.
		pos, etag, url := d.pos, it.etag, it.url
		n := min(chunk, it.size-pos, (d.target-pos+blockWrite-1)/blockWrite*blockWrite)
		// Eviction leaves the span while it is written.
		writing := &span{pos, pos + n}
		it.addBusy(writing)
		it.mu.Unlock()

		err := d.download(pos, n, etag, url)
		it.mu.Lock()
		delete(it.busy, writing)
		it.mu.Unlock()
		if n == chunk {
			chunk = min(chunk*2, chunkMax)
		}
		if err == nil {
			continue
		}
		it.mu.Lock()
		if errors.Is(err, ErrChanged) {
			if it.err == nil {
				it.invalidate()
			}
			d.finish()
			it.mu.Unlock()
			return
		}
		if d.stopped {
			d.finish()
			it.mu.Unlock()
			return
		}
		it.errs++
		it.failErr = err
		failed := it.errs >= maxErrors
		if failed {
			it.errs = 0
			it.failGen++
			d.finish()
			it.mu.Unlock()
			it.bc.log.Warn("block cache download failed", "item", it.name, "err", err)
			return
		}
		it.mu.Unlock()
		select {
		case <-time.After(it.bc.backoff(err, it.errsSnapshot())):
		case <-d.done:
		}
	}
}

// errsSnapshot returns the failed requests in a row.
func (it *blockItem) errsSnapshot() int {
	it.mu.Lock()
	defer it.mu.Unlock()
	return it.errs
}

// backoff is how long to wait after the n-th failed request in a row:
// what the rcd asks, else doubling from 200 ms to 5 s.
func backoff(err error, n int) time.Duration {
	var e *Error
	if errors.As(err, &e) && e.RetryAfter > 0 {
		return e.RetryAfter
	}
	return min(200*time.Millisecond<<min(n, 5), 5*time.Second)
}

// download requests n bytes at pos and writes them to the item.
func (d *downloader) download(pos, n int64, etag, url string) error {
	it := d.it
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		select {
		case <-d.done:
			cancel()
		case <-ctx.Done():
		}
	}()
	req, err := it.bc.c.request(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", pos, pos+n-1))
	if etag != "" {
		req.Header.Set("If-Match", etag)
	}
	resp, err := it.bc.c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusPartialContent:
	case resp.StatusCode == http.StatusOK && pos == 0 && n == it.size:
	default:
		e := serveError(resp)
		if e.Status == http.StatusPreconditionFailed {
			return ErrChanged
		}
		return e
	}
	if total, ok := totalSize(resp); ok && total != it.size {
		return ErrChanged
	}
	it.mu.Lock()
	if it.etag == "" {
		it.etag = resp.Header.Get("ETag")
	}
	f := it.f
	it.mu.Unlock()

	buf := make([]byte, blockWrite)
	for n > 0 {
		k, err := io.ReadFull(resp.Body, buf[:min(int64(len(buf)), n)])
		if k > 0 {
			if _, werr := f.WriteAt(buf[:k], pos); werr != nil {
				return werr
			}
			it.mu.Lock()
			added := it.have.add(pos, pos+int64(k))
			if it.removed {
				added = 0
			}
			pos += int64(k)
			n -= int64(k)
			d.pos = pos
			it.errs = 0
			it.dirty = true
			save := it.bc.now().Sub(it.saved) > saveEvery
			stop := d.stopped || (n > 0 && it.have.present(pos, min(pos+blockWrite, it.size)))
			it.broadcast()
			it.mu.Unlock()
			it.bc.account(added)
			if save {
				it.save(f)
			}
			if stop {
				return nil
			}
		}
		if err != nil {
			if n == 0 {
				return nil
			}
			return fmt.Errorf("read %s: %w", it.name, err)
		}
	}
	return nil
}
