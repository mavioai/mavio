package library

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

// tree creates files below dir; names ending in "/" are folders.
func tree(t *testing.T, dir string, files ...string) {
	t.Helper()
	for _, f := range files {
		p := filepath.Join(dir, filepath.FromSlash(f))
		if f[len(f)-1] == '/' {
			if err := os.MkdirAll(p, 0o755); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

type scanFixture struct {
	t     *testing.T
	root  string
	lib   core.Library
	store *memStore
	sc    *Scanner
	clock time.Time
}

func newScan(t *testing.T, kind core.LibraryKind) *scanFixture {
	root := filepath.ToSlash(t.TempDir())
	f := &scanFixture{t: t, root: root, store: newMemStore(), clock: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)}
	f.lib = core.Library{ID: core.NewID(), Name: "Test", Kind: kind, Paths: []string{root}}
	f.sc = &Scanner{Store: f.store, Resolver: NewResolver(), Now: func() time.Time { return f.clock }}
	return f
}

func (f *scanFixture) scan() ScanStats {
	f.t.Helper()
	st, err := f.sc.Scan(f.t.Context(), f.lib)
	if err != nil {
		f.t.Fatal(err)
	}
	return st
}

// items returns the present items as path (relative to the root) → kind.
func (f *scanFixture) items() map[string]core.ItemKind {
	out := map[string]core.ItemKind{}
	for p, it := range f.store.present() {
		rel, _ := filepath.Rel(f.root, p)
		out[filepath.ToSlash(rel)] = it.Kind
	}
	return out
}

func (f *scanFixture) item(rel string) core.Item {
	f.t.Helper()
	it, ok := f.store.present()[f.root+"/"+rel]
	if !ok {
		f.t.Fatalf("no item %s", rel)
	}
	return it
}

// touchLater moves a path's modification time forward, as a later write
// would.
func touchLater(t *testing.T, p string, d time.Duration) {
	t.Helper()
	at := time.Now().Add(d)
	if err := os.Chtimes(p, at, at); err != nil {
		t.Fatal(err)
	}
}

func TestScanMovies(t *testing.T) {
	f := newScan(t, core.LibraryMovies)
	tree(t, f.root, "Up (2009)/Up (2009).mkv", "Up (2009)/Up (2009)-trailer.mkv", "Up (2009)/trailers/Teaser.mkv",
		"Brazil (1985)/Brazil (1985).mkv", "Brazil (1985)/Brazil (1985) - 1080p.mkv", "loose.mp4", "notes.txt")
	st := f.scan()
	want := map[string]core.ItemKind{
		"Up (2009)/Up (2009).mkv":         core.KindMovie,
		"Up (2009)/Up (2009)-trailer.mkv": core.KindVideo,
		"Up (2009)/trailers/Teaser.mkv":   core.KindVideo,
		"Brazil (1985)/Brazil (1985).mkv": core.KindMovie,
		"loose.mp4":                       core.KindMovie,
	}
	if got := f.items(); !maps.Equal(got, want) {
		t.Fatalf("items = %v, want %v", got, want)
	}
	up := f.item("Up (2009)/Up (2009).mkv")
	if up.Name != "Up" || up.ProductionYear != 2009 {
		t.Errorf("Up = %q %d", up.Name, up.ProductionYear)
	}
	trailer := f.item("Up (2009)/Up (2009)-trailer.mkv")
	if trailer.Extra != core.ExtraTrailer || trailer.OwnerID != up.ID || trailer.Name != "Trailer" {
		t.Errorf("trailer = %+v", trailer)
	}
	brazil := f.item("Brazil (1985)/Brazil (1985).mkv")
	if srcs := f.store.sources[brazil.ID]; len(srcs) != 2 || srcs[1].Name != "1080p" {
		t.Errorf("Brazil sources = %+v", srcs)
	}
	if st.Probes != 5 || len(f.store.jobs) != 5 {
		t.Errorf("probes = %d, jobs = %d", st.Probes, len(f.store.jobs))
	}

	// Unchanged: everything is pruned and nothing is saved.
	f.clock = f.clock.Add(time.Hour)
	st = f.scan()
	if st.Listed != 0 || st.Saved != 0 || st.Missing != 0 || len(f.items()) != 5 {
		t.Errorf("rescan = %+v, items %v", st, f.items())
	}

	// A new trailer in the extras folder is found although the movie
	// folder itself did not change.
	tree(t, f.root, "Up (2009)/trailers/Final.mkv")
	touchLater(t, f.root+"/Up (2009)/trailers", time.Second)
	f.scan()
	if f.items()["Up (2009)/trailers/Final.mkv"] != core.KindVideo {
		t.Errorf("new extra = %v", f.items())
	}

	// Rename, delete and content change.
	if err := os.Rename(f.root+"/Brazil (1985)", f.root+"/Brazil (1986)"); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(f.root+"/Brazil (1986)/Brazil (1985).mkv", f.root+"/Brazil (1986)/Brazil (1986).mkv"); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(f.root + "/loose.mp4"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f.root+"/Up (2009)/Up (2009).mkv", []byte("longer"), 0o644); err != nil {
		t.Fatal(err)
	}
	jobs := len(f.store.jobs)
	st = f.scan()
	got := f.items()
	if _, ok := got["Brazil (1985)/Brazil (1985).mkv"]; ok {
		t.Error("renamed folder still present")
	}
	if got["Brazil (1986)/Brazil (1986).mkv"] != core.KindMovie {
		t.Errorf("renamed movie missing: %v", got)
	}
	if _, ok := got["loose.mp4"]; ok {
		t.Error("deleted file still present")
	}
	if st.Missing != 2 {
		t.Errorf("missing = %d, want 2 (old Brazil, loose.mp4)", st.Missing)
	}
	// The changed movie is probed again, the renamed one for the first time.
	if added := len(f.store.jobs) - jobs; added != 2 {
		t.Errorf("new probe jobs = %d, want 2", added)
	}
	if srcs := f.store.sources[f.item("Up (2009)/Up (2009).mkv").ID]; srcs[0].Size != 6 {
		t.Errorf("changed source = %+v", srcs)
	}

	// Missing items are purged after the grace period.
	f.clock = f.clock.Add(31 * 24 * time.Hour)
	if st = f.scan(); st.Purged != 2 {
		t.Errorf("purged = %d", st.Purged)
	}
}

func TestScanUnavailableRoot(t *testing.T) {
	f := newScan(t, core.LibraryMovies)
	tree(t, f.root, "A (2000)/A (2000).mkv", "B (2001)/B (2001).mkv")
	f.scan()
	// The mount point disappears.
	moved := f.root + ".away"
	if err := os.Rename(f.root, moved); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(moved) })
	st := f.scan()
	if st.Unreadable != 1 || st.Missing != 0 || len(f.items()) != 2 {
		t.Errorf("unavailable root: stats %+v, items %v", st, f.items())
	}
	// It comes back.
	if err := os.Rename(moved, f.root); err != nil {
		t.Fatal(err)
	}
	if st = f.scan(); st.Missing != 0 || len(f.items()) != 2 {
		t.Errorf("after return: stats %+v, items %v", st, f.items())
	}
}

func TestScanShows(t *testing.T) {
	f := newScan(t, core.LibraryShows)
	tree(t, f.root, "Show (2010)/Season 01/Show S01E01.mkv", "Show (2010)/Season 01/Show S01E02.mkv",
		"Show (2010)/Specials/Show S00E01.mkv", "Show (2010)/trailer.mkv")
	f.scan()
	series := f.item("Show (2010)")
	season := f.item("Show (2010)/Season 01")
	ep := f.item("Show (2010)/Season 01/Show S01E02.mkv")
	special := f.item("Show (2010)/Specials/Show S00E01.mkv")
	if series.Kind != core.KindSeries || season.ParentID != series.ID || ep.ParentID != season.ID ||
		*ep.ParentIndexNumber != 1 || *ep.IndexNumber != 2 || *special.ParentIndexNumber != 0 {
		t.Errorf("series %+v\nseason %+v\nepisode %+v", series, season, ep)
	}
	if tr := f.item("Show (2010)/trailer.mkv"); tr.OwnerID != series.ID || tr.Extra != core.ExtraTrailer {
		t.Errorf("series trailer = %+v", tr)
	}
	// A new episode in the season folder.
	tree(t, f.root, "Show (2010)/Season 01/Show S01E03.mkv")
	touchLater(t, f.root+"/Show (2010)/Season 01", time.Second)
	f.scan()
	if e3 := f.item("Show (2010)/Season 01/Show S01E03.mkv"); e3.ParentID != season.ID {
		t.Errorf("new episode = %+v", e3)
	}
}

func TestScanIgnoreFiles(t *testing.T) {
	f := newScan(t, core.LibraryMovies)
	tree(t, f.root, "A (2000)/A (2000).mkv", "Private/B (2001)/B (2001).mkv", "C/C.sample.mkv",
		"Some/D (2002)/D (2002).mkv", "Some/E (2003)/E (2003).mkv")
	// An empty .ignore ignores its folder; rules ignore what they match.
	write(t, filepath.Join(f.root, "Private", ".ignore"), "")
	write(t, filepath.Join(f.root, "Some", ".ignore"), "E (2003)/\n")
	f.scan()
	got := slices.Collect(maps.Keys(f.items()))
	slices.Sort(got)
	if !slices.Equal(got, []string{"A (2000)/A (2000).mkv", "Some/D (2002)/D (2002).mkv"}) {
		t.Errorf("items = %v", got)
	}
}

func TestScanFullReconciliation(t *testing.T) {
	f := newScan(t, core.LibraryMovies)
	tree(t, f.root, "A (2000)/A (2000).mkv")
	f.scan()
	f.sc.NoPruning = true
	if st := f.scan(); st.Pruned != 0 || st.Listed != 2 {
		t.Errorf("no pruning: %+v", st)
	}
}
