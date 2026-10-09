package smoke_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/mavioai/mavio/apps/server/internal/providers"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
	"github.com/mavioai/mavio/libs/store"
)

// EnvPostgresDSN points TestScanReconciles at an existing PostgreSQL
// server in which it may create databases. Without it, the test starts a
// container when Docker is available and skips PostgreSQL otherwise.
const EnvPostgresDSN = "MAVIO_TEST_POSTGRES_DSN"

// countingProber stands in for ffprobe, whose adapter has its own test,
// and counts the probes of each file.
type countingProber struct {
	mu     sync.Mutex
	probes map[string]int
}

func (p *countingProber) Probe(_ context.Context, file string, _ bool) (library.ProbeResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.probes[file]++
	return library.ProbeResult{Source: core.MediaSource{
		Container: "mkv", Duration: 100 * time.Minute,
		Streams: []core.MediaStream{{Index: 0, Kind: core.StreamVideo, Codec: "h264", Width: 1920, Height: 1080}},
	}}, nil
}

// scanEnv is one server instance: a store, two libraries in a folder of
// their own, and the library's jobs with the smoke plugin as metadata
// provider.
type scanEnv struct {
	t      *testing.T
	store  *store.Store
	root   string
	libs   []core.Library
	jobs   *library.Jobs
	worker *library.Worker
	prober *countingProber
	clock  time.Time
}

func newScanEnv(t *testing.T, s *store.Store, provider library.Provider) *scanEnv {
	t.Helper()
	e := &scanEnv{t: t, store: s, root: filepath.ToSlash(t.TempDir()), prober: &countingProber{probes: map[string]int{}}, clock: time.Now()}
	for _, l := range []struct {
		name string
		kind core.LibraryKind
	}{{"films", core.LibraryMovies}, {"shows", core.LibraryShows}} {
		lib := core.Library{Name: l.name, Kind: l.kind, Paths: []string{e.root + "/" + l.name}, PreferredLanguage: "zh", MetadataCountry: "CN"}
		if err := os.MkdirAll(filepath.FromSlash(lib.Paths[0]), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := s.Libraries().Create(t.Context(), &lib); err != nil {
			t.Fatal(err)
		}
		e.libs = append(e.libs, lib)
	}
	now := func() time.Time { return e.clock }
	e.jobs = &library.Jobs{
		Store:     s,
		Scanner:   &library.Scanner{Store: s, Resolver: library.NewResolver(), Now: now},
		Prober:    e.prober,
		Refresher: &library.Refresher{Store: s, Providers: []library.Provider{provider}},
	}
	// Failed jobs are retried later; the test fails at once.
	handlers := e.jobs.Handlers()
	for kind, h := range handlers {
		handlers[kind] = func(ctx context.Context, job core.Job) ([]core.Job, error) {
			next, err := h(ctx, job)
			if err != nil {
				e.t.Errorf("job %s failed: %v", kind, err)
			}
			return next, err
		}
	}
	e.worker = &library.Worker{Queue: s.Jobs(), Owner: "smoke", Handlers: handlers}
	return e
}

// write creates or overwrites files with the given contents.
func (e *scanEnv) write(files map[string]string) {
	e.t.Helper()
	for rel, content := range files {
		p := filepath.Join(filepath.FromSlash(e.root), filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			e.t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			e.t.Fatal(err)
		}
	}
}

func (e *scanEnv) rename(from, to string) {
	e.t.Helper()
	abs := func(rel string) string { return filepath.Join(filepath.FromSlash(e.root), filepath.FromSlash(rel)) }
	if err := os.Rename(abs(from), abs(to)); err != nil {
		e.t.Fatal(err)
	}
}

func (e *scanEnv) remove(rel string) {
	e.t.Helper()
	if err := os.RemoveAll(filepath.Join(filepath.FromSlash(e.root), filepath.FromSlash(rel))); err != nil {
		e.t.Fatal(err)
	}
}

// scan scans every library as a requested scan would and runs the jobs
// that follow: probes of new and changed files and metadata refreshes.
func (e *scanEnv) scan() {
	e.t.Helper()
	ctx := e.t.Context()
	for _, lib := range e.libs {
		job := library.ScanJob(lib, time.Now(), false)
		if _, err := e.store.Jobs().Enqueue(ctx, &job); err != nil {
			e.t.Fatal(err)
		}
	}
	for {
		ran, err := e.worker.RunOne(ctx)
		if err != nil {
			e.t.Fatal(err)
		}
		if !ran {
			break
		}
	}
}

// state describes the stored libraries, independent of IDs and times: one
// line per item, by path relative to the libraries' folder.
func (e *scanEnv) state() []string {
	e.t.Helper()
	ctx := e.t.Context()
	var ids []core.ID
	for _, lib := range e.libs {
		ids = append(ids, lib.ID)
	}
	page, err := e.store.Items().Query(ctx, core.ItemQuery{LibraryIDs: ids, IncludeExtras: true, IncludeMissing: true})
	if err != nil {
		e.t.Fatal(err)
	}
	paths := map[core.ID]string{}
	for _, it := range page.Items {
		paths[it.ID] = strings.TrimPrefix(it.Path, e.root+"/")
	}
	var lines []string
	for _, it := range page.Items {
		line := fmt.Sprintf("%s %s parent=%q name=%q year=%d", it.Kind, paths[it.ID], paths[it.ParentID], it.Name, it.ProductionYear)
		if it.Extra != "" {
			line += fmt.Sprintf(" extra=%s owner=%q", it.Extra, paths[it.OwnerID])
		}
		if it.MissingSince != nil {
			line += " missing"
		}
		if id := it.ExternalIDs[core.ProviderTMDB]; id != "" {
			line += fmt.Sprintf(" tmdb=%s genres=%v", id, it.Genres)
		}
		sources, err := e.store.MediaSources().ListForItem(ctx, it.ID)
		if err != nil {
			e.t.Fatal(err)
		}
		for _, src := range sources {
			line += fmt.Sprintf(" source=%s:%d:probed=%v", path.Base(src.Path), src.Size, !src.ProbedAt.IsZero())
		}
		lines = append(lines, line)
	}
	slices.Sort(lines)
	return lines
}

// lines returns the state lines about the item at rel.
func lines(state []string, rel string) []string {
	var out []string
	for _, l := range state {
		if strings.Contains(l, " "+rel+" ") {
			out = append(out, l)
		}
	}
	return out
}

// TestScanReconciles runs P4 end to end on SQLite and PostgreSQL: a film
// and a shows library are scanned as jobs, new files probed and their
// metadata fetched from the smoke plugin through the provider adapter.
// The libraries then change — files are added, deleted, renamed and
// rewritten, and the folder holding them goes away for a while — and after
// every change the scans converge to the same state on both databases.
func TestScanReconciles(t *testing.T) {
	ctx := t.Context()
	plugin := openPlugin(t, "wasm")
	provider := &providers.Plugin{ID: "org.mavio.smoke", Client: plugin.Metadata()}

	stores := map[string]*store.Store{}
	openStore := func(name, dsn string) {
		s, err := store.Open(ctx, dsn)
		if err != nil {
			t.Fatalf("open %s: %v", name, err)
		}
		t.Cleanup(func() { _ = s.Close() })
		stores[name] = s
	}
	openStore("sqlite", "sqlite:"+filepath.Join(t.TempDir(), "mavio.db"))
	if dsn, skip := postgresDSN(t); dsn != "" {
		openStore("postgres", dsn)
	} else {
		t.Logf("PostgreSQL not tested: %s", skip)
	}

	envs := map[string]*scanEnv{}
	for name, s := range stores {
		envs[name] = newScanEnv(t, s, provider)
	}
	// step applies a change to every instance, scans, checks the state
	// and that all instances agree.
	step := func(name string, change func(e *scanEnv), check func(t *testing.T, state []string, e *scanEnv)) {
		t.Run(name, func(t *testing.T) {
			states := map[string][]string{}
			for db, e := range envs {
				e.t = t
				change(e)
				e.scan()
				states[db] = e.state()
				check(t, states[db], e)
			}
			if pg, ok := states["postgres"]; ok && !slices.Equal(states["sqlite"], pg) {
				t.Errorf("states differ:\nsqlite:\n%s\npostgres:\n%s", strings.Join(states["sqlite"], "\n"), strings.Join(pg, "\n"))
			}
		})
	}
	want := func(t *testing.T, state []string, rel string, parts ...string) {
		t.Helper()
		got := lines(state, rel)
		if len(got) != 1 {
			t.Errorf("%s: got = %q, want = one item", rel, got)
			return
		}
		for _, p := range parts {
			if strings.HasPrefix(p, "!") {
				if strings.Contains(got[0], p[1:]) {
					t.Errorf("%s: got = %s, want = without %q", rel, got[0], p[1:])
				}
			} else if !strings.Contains(got[0], p) {
				t.Errorf("%s: got = %s, want = with %q", rel, got[0], p)
			}
		}
	}

	const (
		concubine  = "films/Farewell My Concubine (1993)/Farewell My Concubine (1993).mkv"
		alien      = "films/Alien (1979)/Alien (1979).mkv"
		teaser     = "films/Alien (1979)/trailers/Teaser.mkv"
		brazil     = "films/Brazil (1985)/Brazil (1985).mkv"
		show       = "shows/Farewell Show (2020)"
		ep1        = show + "/Season 01/Farewell Show S01E01.mkv"
		ep2        = show + "/Season 01/Farewell Show S01E02.mkv"
		ep3        = show + "/Season 01/Farewell Show S01E03.mkv"
		ep3Renamed = show + "/Season 01/Farewell Show - S01E03 - The Third.mkv"
	)

	step("initial", func(e *scanEnv) {
		e.write(map[string]string{concubine: "concubine", alien: "alien", teaser: "teaser", ep1: "ep1", ep2: "ep2"})
	}, func(t *testing.T, state []string, e *scanEnv) {
		if len(state) != 7 {
			t.Errorf("state = %d items, want = 7:\n%s", len(state), strings.Join(state, "\n"))
		}
		// The plugin knows the film; its metadata and the probe are stored.
		want(t, state, concubine, "movie ", `year=1993`, "tmdb=10997 genres=[Drama Romance]", "source=Farewell My Concubine (1993).mkv:9:probed=true", "!missing")
		want(t, state, alien, `name="Alien"`, "!tmdb=", "probed=true")
		want(t, state, teaser, `extra=trailer owner="`+alien+`"`)
		want(t, state, show, "series ")
		want(t, state, show+"/Season 01", "season ", `parent="`+show+`"`)
		want(t, state, ep2, "episode ", `parent="`+show+`/Season 01"`, "probed=true")
	})

	step("unchanged", func(*scanEnv) {}, func(t *testing.T, state []string, e *scanEnv) {
		if n := e.prober.probes[e.root+"/"+concubine]; n != 1 {
			t.Errorf("probes of an unchanged file = %d, want = 1", n)
		}
	})

	step("added", func(e *scanEnv) {
		e.write(map[string]string{brazil: "brazil", ep3: "ep3"})
	}, func(t *testing.T, state []string, e *scanEnv) {
		want(t, state, brazil, "movie ", "year=1985", "probed=true")
		want(t, state, ep3, "episode ", "probed=true")
	})

	step("deleted", func(e *scanEnv) {
		e.remove("films/Alien (1979)")
	}, func(t *testing.T, state []string, e *scanEnv) {
		// Missing items are kept, with their metadata, until purged.
		want(t, state, alien, "missing")
		want(t, state, teaser, "missing")
		want(t, state, concubine, "!missing")
	})

	step("renamed", func(e *scanEnv) {
		e.rename(ep3, ep3Renamed)
	}, func(t *testing.T, state []string, e *scanEnv) {
		want(t, state, ep3, "missing")
		want(t, state, ep3Renamed, "episode ", "!missing", "probed=true")
	})

	step("rewritten", func(e *scanEnv) {
		e.write(map[string]string{concubine: "a longer cut of the film"})
		future := time.Now().Add(time.Hour)
		if err := os.Chtimes(filepath.FromSlash(e.root+"/"+concubine), future, future); err != nil {
			t.Fatal(err)
		}
	}, func(t *testing.T, state []string, e *scanEnv) {
		want(t, state, concubine, "source=Farewell My Concubine (1993).mkv:24:probed=true", "tmdb=10997")
		if n := e.prober.probes[e.root+"/"+concubine]; n != 2 {
			t.Errorf("probes of the rewritten file = %d, want = 2", n)
		}
	})

	var before map[string][]string
	step("unavailable", func(e *scanEnv) {
		if before == nil {
			before = map[string][]string{}
		}
		before[e.store.Dialect()] = e.state()
		e.rename("films", "films.offline")
	}, func(t *testing.T, state []string, e *scanEnv) {
		// An unreadable library folder keeps its items as they were.
		if !slices.Equal(state, before[e.store.Dialect()]) {
			t.Errorf("state while unavailable:\n%s\nwant:\n%s", strings.Join(state, "\n"), strings.Join(before[e.store.Dialect()], "\n"))
		}
	})

	step("available again", func(e *scanEnv) {
		e.rename("films.offline", "films")
	}, func(t *testing.T, state []string, e *scanEnv) {
		if !slices.Equal(state, before[e.store.Dialect()]) {
			t.Errorf("state after return:\n%s\nwant:\n%s", strings.Join(state, "\n"), strings.Join(before[e.store.Dialect()], "\n"))
		}
	})

	step("purged", func(e *scanEnv) {
		e.clock = e.clock.Add(31 * 24 * time.Hour)
	}, func(t *testing.T, state []string, e *scanEnv) {
		for _, rel := range []string{alien, teaser, ep3} {
			if got := lines(state, rel); len(got) != 0 {
				t.Errorf("%s: got = %q, want = purged", rel, got)
			}
		}
		if len(state) != 7 {
			t.Errorf("state = %d items, want = 7:\n%s", len(state), strings.Join(state, "\n"))
		}
	})
}

// postgresDSN returns the DSN of a fresh PostgreSQL database, or why there
// is none.
func postgresDSN(t *testing.T) (dsn, skip string) {
	t.Helper()
	admin := os.Getenv(EnvPostgresDSN)
	if admin == "" {
		if testing.Short() {
			return "", "skipped in -short mode"
		}
		ctx := context.Background()
		container, err := tcpostgres.Run(ctx, "postgres:17-alpine", tcpostgres.BasicWaitStrategies())
		if err != nil {
			return "", fmt.Sprintf("container unavailable: %v", err)
		}
		t.Cleanup(func() { _ = testcontainers.TerminateContainer(container) })
		if admin, err = container.ConnectionString(ctx, "sslmode=disable"); err != nil {
			return "", err.Error()
		}
	}
	name := fmt.Sprintf("mavio_smoke_%d", os.Getpid())
	db, err := sql.Open("pgx", admin)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.ExecContext(t.Context(), "CREATE DATABASE "+name); err != nil {
		t.Fatalf("create database: %v", err)
	}
	t.Cleanup(func() {
		if db, err := sql.Open("pgx", admin); err == nil {
			_, _ = db.Exec("DROP DATABASE IF EXISTS " + name + " WITH (FORCE)")
			_ = db.Close()
		}
	})
	rest, query, _ := strings.Cut(admin, "?")
	dsn = rest[:strings.LastIndex(rest, "/")+1] + name
	if query != "" {
		dsn += "?" + query
	}
	return dsn, ""
}
