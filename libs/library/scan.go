package library

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library/storage"
)

// JobProbe is the kind of jobs that probe an item's new or changed media
// files; the payload is a ProbePayload.
const JobProbe = "media.probe"

// ProbePayload is the payload of JobProbe jobs.
type ProbePayload struct {
	ItemID core.ID `json:"item_id"`
}

// Scanner reconciles libraries with their folders.
type Scanner struct {
	Store    core.Store
	Resolver *Resolver
	Logger   *slog.Logger
	// Concurrency bounds the folders scanned at once; default 4.
	Concurrency int
	// NoPruning lists every folder, also those whose modification time and
	// file ID did not change.
	NoPruning bool
	// MissingGrace is how long missing items are kept before they are
	// purged; default 30 days.
	MissingGrace time.Duration
	// Now returns the current time; default time.Now.
	Now func() time.Time

	// VolumeLedger serializes scans on rotational hard drives per physical device.
	VolumeLedger *storage.VolumeLedger
	// QuietGate pauses or throttles background scans during active client streaming.
	QuietGate *storage.QuietGate
	// GrowthPolicy identifies actively downloading or growing files to avoid dirty reads.
	GrowthPolicy *storage.GrowthPolicy
}

// ScanStats summarizes a scan.
type ScanStats struct {
	Generation int64
	// Listed folders were read; Pruned ones were unchanged and taken from
	// the previous scan.
	Listed, Pruned int
	// Unreadable folders and roots kept their items.
	Unreadable int
	// Saved items were added or changed; Probes were queued for new or
	// changed media files.
	Saved, Probes int
	Missing       int
	Purged        int
}

func (s *Scanner) logger() *slog.Logger {
	if s.Logger != nil {
		return s.Logger
	}
	return slog.New(slog.DiscardHandler)
}

func (s *Scanner) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// Scan reconciles lib with its folders: new and changed files are resolved
// into items and queued for probing, items whose files are gone are marked
// missing, and items missing for longer than the grace period are purged.
// A root that cannot be read keeps all its items.
func (s *Scanner) Scan(ctx context.Context, lib core.Library) (ScanStats, error) {
	gen, err := s.Store.Scans().NextGeneration(ctx, lib.ID)
	if err != nil {
		return ScanStats{}, err
	}
	sc := &scan{Scanner: s, lib: lib, gen: gen, stats: ScanStats{Generation: gen}}
	for _, root := range lib.Paths {
		if err := sc.root(ctx, root); err != nil {
			return sc.stats, err
		}
	}
	now := s.now()
	if sc.stats.Missing, err = s.Store.Items().MarkMissing(ctx, lib.ID, gen, now); err != nil {
		return sc.stats, err
	}
	if err := s.Store.Scans().DeleteFolders(ctx, lib.ID, gen); err != nil {
		return sc.stats, err
	}
	grace := s.MissingGrace
	if grace <= 0 {
		grace = 30 * 24 * time.Hour
	}
	purged, err := s.Store.Items().PurgeMissing(ctx, lib.ID, now.Add(-grace))
	sc.stats.Purged = len(purged)
	s.logger().InfoContext(ctx, "library scanned", "library", lib.Name, "generation", gen,
		"listed", sc.stats.Listed, "pruned", sc.stats.Pruned, "saved", sc.stats.Saved,
		"missing", sc.stats.Missing, "purged", sc.stats.Purged)
	return sc.stats, err
}

// scan is one run of Scan.
type scan struct {
	*Scanner
	lib   core.Library
	gen   int64
	mu    sync.Mutex
	stats ScanStats
}

func (sc *scan) count(f func(*ScanStats)) {
	sc.mu.Lock()
	f(&sc.stats)
	sc.mu.Unlock()
}

// task is a folder to scan.
type task struct {
	dir    string
	scope  Scope
	parent core.ID // the item the folder's items belong to
}

func (sc *scan) root(ctx context.Context, base string) error {
	base = filepath.ToSlash(filepath.Clean(base))
	if sc.VolumeLedger != nil {
		dev, _ := storage.DetectDevice(filepath.FromSlash(base))
		if dev.ID != "" {
			rel, err := sc.VolumeLedger.Acquire(ctx, dev.ID)
			if err != nil {
				return err
			}
			defer rel()
		}
	}
	r, err := os.OpenRoot(filepath.FromSlash(base))
	if err != nil {
		// An unmounted or unreadable root keeps its items until it is back.
		sc.logger().WarnContext(ctx, "library folder unavailable", "path", base, "err", err)
		sc.count(func(s *ScanStats) { s.Unreadable++ })
		return sc.Store.Items().MarkSeen(ctx, sc.lib.ID, base, sc.gen)
	}
	defer r.Close()
	rfs := rootFS{root: r, base: base}
	ignores := &IgnoreFiles{FS: r.FS()}

	g, gctx := errgroup.WithContext(ctx)
	limit := sc.Concurrency
	if limit <= 0 {
		limit = 4
	}
	g.SetLimit(limit)
	var visit func(t task)
	visit = func(t task) {
		run := func() error {
			subs, err := sc.folder(gctx, rfs, ignores, t)
			if err != nil {
				return err
			}
			for _, sub := range subs {
				visit(sub)
			}
			return nil
		}
		// A full pool runs the folder in the caller, so that waiting for a
		// slot can never block every worker.
		if !g.TryGo(run) {
			if err := run(); err != nil {
				g.Go(func() error { return err })
			}
		}
	}
	visit(task{dir: base, scope: Scope{Kind: sc.lib.Kind, Root: base}})
	return g.Wait()
}

// folder scans one folder and returns its subfolders.
func (sc *scan) folder(ctx context.Context, rfs rootFS, ignores *IgnoreFiles, t task) ([]task, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if sc.QuietGate != nil {
		if err := sc.QuietGate.PauseOrCancel(ctx); err != nil {
			return nil, err
		}
	}
	state, changed, err := sc.state(ctx, rfs, ignores, t.dir)
	if err != nil {
		// An unreadable folder keeps its items.
		sc.logger().WarnContext(ctx, "folder unreadable", "path", t.dir, "err", err)
		sc.count(func(s *ScanStats) { s.Unreadable++ })
		return nil, sc.Store.Items().MarkSeen(ctx, sc.lib.ID, t.dir, sc.gen)
	}
	entries := make([]Entry, len(state.Entries))
	stats := map[string]core.FolderEntry{}
	for i, e := range state.Entries {
		p := path.Join(t.dir, e.Name)
		entries[i] = Entry{Path: p, IsDir: e.IsDir}
		stats[p] = e
	}
	res, err := sc.Resolver.Resolve(t.scope, t.dir, entries, filtered{rfs, ignores})
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", t.dir, err)
	}
	var parent core.ID
	if changed {
		if parent, err = sc.save(ctx, rfs, res, t.parent, stats); err != nil {
			return nil, err
		}
	} else {
		if parent, err = sc.touch(ctx, res, t.parent); err != nil {
			return nil, err
		}
	}
	if err := sc.Store.Scans().PutFolders(ctx, state); err != nil {
		return nil, err
	}
	subs := make([]task, len(res.Subfolders))
	for i, sub := range res.Subfolders {
		subs[i] = task{dir: sub.Path, scope: sub.Scope, parent: parent}
	}
	return subs, nil
}

// state returns the folder's state for this scan: its entries, taken from
// the previous scan when the folder's modification time and file ID did
// not change, and whether anything in it changed.
func (sc *scan) state(ctx context.Context, rfs rootFS, ignores *IgnoreFiles, dir string) (core.FolderState, bool, error) {
	info, err := rfs.stat(dir)
	if err != nil {
		return core.FolderState{}, false, err
	}
	st := core.FolderState{LibraryID: sc.lib.ID, Path: dir, ModTime: modTime(info), FileID: fileID(info), Generation: sc.gen}
	prev, err := sc.Store.Scans().Folder(ctx, sc.lib.ID, dir)
	switch {
	case errors.Is(err, core.ErrNotFound):
	case err != nil:
		return st, false, err
	case !sc.NoPruning && prev.ModTime.Equal(st.ModTime) && prev.FileID == st.FileID:
		// Entries are as before; files may still have new contents.
		changed := false
		st.Entries = make([]core.FolderEntry, 0, len(prev.Entries))
		for _, e := range prev.Entries {
			// Subfolders count too: extras folders are resolved with the
			// folder, not on their own.
			fi, err := rfs.stat(path.Join(dir, e.Name))
			if err != nil {
				return sc.list(rfs, ignores, st)
			}
			size := fi.Size()
			if e.IsDir {
				size = 0
			}
			if size != e.Size || !modTime(fi).Equal(e.ModTime) {
				changed = true
				e.Size, e.ModTime = size, modTime(fi)
			}
			st.Entries = append(st.Entries, e)
		}
		sc.count(func(s *ScanStats) { s.Pruned++ })
		return st, changed, nil
	}
	return sc.list(rfs, ignores, st)
}

// temporaryDownloadExtensions are suffixes of partially downloaded files that should never be indexed.
var temporaryDownloadExtensions = []string{
	".part", ".crdownload", ".download", ".aria2", ".!qb", ".tmp",
}

// list reads a folder's entries, without those .ignore files exclude.
func (sc *scan) list(rfs rootFS, ignores *IgnoreFiles, st core.FolderState) (core.FolderState, bool, error) {
	entries, err := rfs.List(st.Path)
	if err != nil {
		return st, false, err
	}
	for _, e := range entries {
		ext := strings.ToLower(path.Ext(e.Path))
		if slices.Contains(temporaryDownloadExtensions, ext) {
			continue
		}
		if ignored, err := ignores.Ignored(slashRel(rfs, e.Path), e.IsDir); err != nil || ignored {
			continue
		}
		fe := core.FolderEntry{Name: path.Base(e.Path), IsDir: e.IsDir}
		fi, err := rfs.stat(e.Path)
		if err != nil {
			continue // gone since listed
		}
		fe.ModTime = modTime(fi)
		if !e.IsDir {
			fe.Size = fi.Size()
			if sc.GrowthPolicy != nil && sc.GrowthPolicy.IsGrowing(e.Path, fe.Size, fe.ModTime) {
				continue
			}
		}
		st.Entries = append(st.Entries, fe)
	}
	sc.count(func(s *ScanStats) { s.Listed++ })
	return st, true, nil
}

// modTime returns a file's modification time as the store keeps it: in UTC
// and to the microsecond, so that an unchanged file compares equal to what
// the previous scan recorded.
func modTime(fi fs.FileInfo) time.Time { return fi.ModTime().UTC().Truncate(time.Microsecond) }

func slashRel(rfs rootFS, p string) string { return filepath.ToSlash(rfs.rel(p)) }

// filtered lists folders for the resolver without what .ignore files
// exclude.
type filtered struct {
	rfs     rootFS
	ignores *IgnoreFiles
}

func (f filtered) List(dir string) ([]Entry, error) {
	entries, err := f.rfs.List(dir)
	if err != nil {
		return nil, err
	}
	out := entries[:0]
	for _, e := range entries {
		if ignored, err := f.ignores.Ignored(slashRel(f.rfs, e.Path), e.IsDir); err == nil && !ignored {
			out = append(out, e)
		}
	}
	return out, nil
}

// touch marks the items of an unchanged folder seen and returns the item
// its subfolders belong to.
func (sc *scan) touch(ctx context.Context, res Result, parent core.ID) (core.ID, error) {
	var paths []string
	add := func(n Node) {
		paths = append(paths, n.Path)
		for _, e := range n.Extras {
			paths = append(paths, e.Path)
		}
	}
	if res.Item != nil {
		add(*res.Item)
	}
	for _, n := range res.Items {
		add(n)
	}
	if err := sc.Store.Items().Touch(ctx, sc.lib.ID, sc.gen, paths...); err != nil {
		return core.NilID, err
	}
	if res.Item == nil {
		return parent, nil
	}
	it, err := sc.Store.Items().GetByPath(ctx, sc.lib.ID, res.Item.Path)
	if errors.Is(err, core.ErrNotFound) {
		// Recorded before its item was saved; save it now.
		return sc.save(ctx, rootFS{}, Result{Item: res.Item}, parent, nil)
	}
	return it.ID, err
}

// save writes the items of a folder, queues probes of new or changed
// media files, which refresh their items' metadata once probed, and
// refreshes of new items without media, such as series and albums. It
// returns the item the folder's subfolders belong to.
func (sc *scan) save(ctx context.Context, rfs rootFS, res Result, parent core.ID, stats map[string]core.FolderEntry) (core.ID, error) {
	folderParent := parent
	var probes, refreshes []core.ID
	err := sc.Store.InTx(ctx, func(tx core.Store) error {
		probes, refreshes = nil, nil
		p := saver{scan: sc, tx: tx, rfs: rfs, stats: stats, probes: &probes, refreshes: &refreshes}
		if res.Item != nil {
			id, err := p.node(ctx, *res.Item, parent, core.NilID)
			if err != nil {
				return err
			}
			folderParent = id
			if err := p.extras(ctx, res.Item.Extras, id); err != nil {
				return err
			}
		}
		for _, n := range res.Items {
			id, err := p.node(ctx, n, folderParent, core.NilID)
			if err != nil {
				return err
			}
			if err := p.extras(ctx, n.Extras, id); err != nil {
				return err
			}
		}
		sc.count(func(s *ScanStats) { s.Saved += p.saved })
		return nil
	})
	if err != nil {
		return core.NilID, err
	}
	for _, id := range probes {
		payload, err := json.Marshal(ProbePayload{ItemID: id})
		if err != nil {
			return core.NilID, err
		}
		added, err := sc.Store.Jobs().Enqueue(ctx, &core.Job{
			ID: core.NewID(), Kind: JobProbe, Payload: payload, UniqueKey: JobProbe + ":" + id.String(),
			MaxAttempts: 3, RunAt: sc.now(),
		})
		if err != nil {
			return core.NilID, err
		}
		if added {
			sc.count(func(s *ScanStats) { s.Probes++ })
		}
	}
	for _, id := range refreshes {
		job := RefreshJob(id, sc.now())
		if _, err := sc.Store.Jobs().Enqueue(ctx, &job); err != nil {
			return core.NilID, err
		}
	}
	return folderParent, nil
}

// saver writes the items of one folder within a transaction.
type saver struct {
	scan  *scan
	tx    core.Store
	rfs   rootFS
	stats map[string]core.FolderEntry
	saved int
	// probes and refreshes collect the items to probe and to refresh.
	probes, refreshes *[]core.ID
}

func (p *saver) extras(ctx context.Context, extras []Node, owner core.ID) error {
	for _, e := range extras {
		if _, err := p.node(ctx, e, core.NilID, owner); err != nil {
			return err
		}
	}
	return nil
}

// stat returns the size and modification time of a file, from the folder
// listing or the file system.
func (p *saver) stat(path string) (core.FolderEntry, bool) {
	if e, ok := p.stats[path]; ok {
		return e, true
	}
	if p.rfs.root == nil {
		return core.FolderEntry{}, false
	}
	fi, err := p.rfs.stat(path)
	if err != nil {
		return core.FolderEntry{}, false
	}
	return core.FolderEntry{Size: fi.Size(), ModTime: modTime(fi)}, true
}

// node writes one item, to be probed when its media are new or changed,
// or refreshed when it is new and has no media. New items take the
// resolved names and numbers; known items keep their metadata, which
// refreshes and users own, and only take the structure.
func (p *saver) node(ctx context.Context, n Node, parent, owner core.ID) (core.ID, error) {
	sc := p.scan
	it, err := p.tx.Items().GetByPath(ctx, sc.lib.ID, n.Path)
	isNew := errors.Is(err, core.ErrNotFound)
	if err != nil && !isNew {
		return core.NilID, err
	}
	before := it
	if isNew {
		it = core.Item{ID: core.NewID(), LibraryID: sc.lib.ID, DateAdded: sc.now(), Name: n.Name, Path: n.Path}
		if n.Year != nil {
			it.ProductionYear = *n.Year
		}
		it.IndexNumber, it.ParentIndexNumber, it.IndexNumberEnd = n.Index, n.ParentIndex, n.IndexEnd
		it.PremiereDate = n.Aired
		it.Video3DFormat = n.Format3D
	} else if n.Extra != "" {
		it.Name = RenewExtraName(it.Name, it.Locked || slices.Contains(it.LockedFields, core.FieldName), n)
	}
	if it.Name == "" {
		it.Name = fileNameWithoutExt(n.Path)
	}
	it.Kind, it.Extra, it.ParentID, it.OwnerID = n.Kind, n.Extra, parent, owner
	for provider, id := range n.ExternalIDs {
		if it.ExternalIDs == nil {
			it.ExternalIDs = map[core.Provider]string{}
		}
		if _, ok := it.ExternalIDs[provider]; !ok {
			it.ExternalIDs[provider] = id
		}
	}
	it.ScanGeneration, it.MissingSince = sc.gen, nil

	sources, probe, err := p.sources(ctx, n, it.ID, isNew)
	if err != nil {
		return core.NilID, err
	}
	if len(sources) > 0 {
		it.FileModified = sources[0].Modified
	}
	if isNew || probe || changedStructure(before, it) {
		if err := p.tx.Items().Upsert(ctx, it); err != nil {
			return core.NilID, fmt.Errorf("save %s: %w", n.Path, err)
		}
		p.saved++
	} else if err := p.tx.Items().Touch(ctx, sc.lib.ID, sc.gen, n.Path); err != nil {
		return core.NilID, err
	}
	if sources != nil {
		if err := p.tx.MediaSources().Replace(ctx, it.ID, sources); err != nil {
			return core.NilID, err
		}
	}
	switch {
	case probe:
		*p.probes = append(*p.probes, it.ID)
	case isNew && !playable[n.Kind]:
		*p.refreshes = append(*p.refreshes, it.ID)
	}
	return it.ID, nil
}

// changedStructure reports whether a scan changed what it owns of an item.
func changedStructure(a, b core.Item) bool {
	return a.Kind != b.Kind || a.Name != b.Name || a.Extra != b.Extra || a.ParentID != b.ParentID ||
		a.OwnerID != b.OwnerID || a.MissingSince != nil || !a.FileModified.Equal(b.FileModified) ||
		len(a.ExternalIDs) != len(b.ExternalIDs)
}

// playable kinds have media sources.
var playable = map[core.ItemKind]bool{
	core.KindMovie: true, core.KindEpisode: true, core.KindVideo: true, core.KindMusicVideo: true,
	core.KindTrack: true, core.KindAudioBook: true,
}

// sources returns the item's media sources when they changed, nil when
// they did not, and whether any needs probing. Sources whose file kept its
// size and modification time keep their probe results.
func (p *saver) sources(ctx context.Context, n Node, itemID core.ID, isNew bool) ([]core.MediaSource, bool, error) {
	if !playable[n.Kind] {
		return nil, false, nil
	}
	var known []core.MediaSource
	if !isNew {
		var err error
		if known, err = p.tx.MediaSources().ListForItem(ctx, itemID); err != nil {
			return nil, false, err
		}
	}
	versions := append([]Version{{Path: n.Path, Parts: n.Parts}}, n.Versions...)
	sources := make([]core.MediaSource, 0, len(versions))
	probe, same := false, len(known) == len(versions)
	for i, v := range versions {
		src := core.MediaSource{ID: core.NewID(), ItemID: itemID, Path: v.Path, Parts: v.Parts, Disc: core.DiscKind(n.Disc)}
		if len(versions) > 1 {
			src.Name = versionName(n.Path, v.Path)
		}
		if st, ok := p.stat(v.Path); ok {
			src.Size, src.Modified = st.Size, st.ModTime
		}
		reused := false
		for _, k := range known {
			if k.Path == v.Path && k.Size == src.Size && k.Modified.Equal(src.Modified) && !k.ProbedAt.IsZero() {
				k.Parts, k.Disc, k.Name = src.Parts, src.Disc, src.Name
				src, reused = k, true
				break
			}
		}
		if !reused {
			probe = true
		}
		same = same && reused && known[i].Path == v.Path
		sources = append(sources, src)
	}
	if same && !isNew {
		return nil, false, nil
	}
	return sources, probe, nil
}

// versionName labels an alternate version by what its file name adds to
// the primary's, e.g. "1080p" for "Movie - 1080p.mkv".
func versionName(primary, file string) string {
	base := fileNameWithoutExt(file)
	if file == primary {
		return base
	}
	common := fileNameWithoutExt(primary)
	for !strings.HasPrefix(base, common) && common != "" {
		common = common[:len(common)-1]
	}
	label := strings.Trim(strings.TrimPrefix(base, common), " -_.[]")
	if label == "" {
		return base
	}
	return label
}
