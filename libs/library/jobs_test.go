package library

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/metadata"
)

type fakeProber struct{ probed []string }

func (p *fakeProber) Probe(_ context.Context, path string, audio bool) (ProbeResult, error) {
	p.probed = append(p.probed, path)
	return ProbeResult{Source: core.MediaSource{
		Container: "mkv", Duration: 90 * time.Minute,
		Streams: []core.MediaStream{{Index: 0, Kind: core.StreamVideo, Codec: "h264"}},
	}}, nil
}

type fakeProvider struct{ lookups []Lookup }

func (p *fakeProvider) Name() string { return "fake" }

func (p *fakeProvider) Metadata(_ context.Context, l Lookup) (*metadata.Result, error) {
	p.lookups = append(p.lookups, l)
	if l.Kind != core.KindMovie {
		return nil, nil
	}
	return &metadata.Result{
		Item: core.Item{
			Name: "Remote " + l.Name, Overview: "From the provider.", Genres: []string{"Animation"},
			ExternalIDs: map[core.Provider]string{core.ProviderTMDB: "14160"},
		},
		People:       []metadata.Person{{Name: "Pete Docter", Kind: core.CreditDirector}},
		RemoteImages: []metadata.RemoteImage{{Kind: core.ImagePrimary, URL: "https://image.example/up.jpg"}},
	}, nil
}

const upNFO = `<?xml version="1.0" encoding="UTF-8"?>
<movie><title>Up (Local)</title><lockedfields>Genres</lockedfields></movie>`

// drain runs the worker until no job is due.
func drain(t *testing.T, w *Worker) int {
	t.Helper()
	n := 0
	for {
		ran, err := w.RunOne(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if !ran {
			return n
		}
		n++
	}
}

func TestJobs(t *testing.T) {
	f := newScan(t, core.LibraryMovies)
	f.lib.ScanInterval = 6 * time.Hour
	tree(t, f.root, "Up (2009)/Up (2009).mkv", "Brazil (1985)/Brazil (1985).mkv")
	write(t, filepath.Join(f.root, "Up (2009)", "movie.nfo"), upNFO)
	if err := f.store.Libraries().Create(t.Context(), &f.lib); err != nil {
		t.Fatal(err)
	}
	f.store.clock = func() time.Time { return f.clock }
	prober, provider := &fakeProber{}, &fakeProvider{}
	jobs := &Jobs{
		Store: f.store, Scanner: f.sc, Prober: prober, Now: func() time.Time { return f.clock },
		Refresher: &Refresher{Store: f.store, Providers: []Provider{provider}, Now: func() time.Time { return f.clock }},
	}
	w := &Worker{Queue: f.store.Jobs(), Owner: "test", Handlers: jobs.Handlers()}
	if err := jobs.Schedule(t.Context()); err != nil {
		t.Fatal(err)
	}
	// One scan, two probes, two refreshes; the scheduled scan waits.
	if n := drain(t, w); n != 5 {
		t.Errorf("jobs run = %d, want 5", n)
	}
	up := f.item("Up (2009)/Up (2009).mkv")
	// The NFO's title wins over the provider's, the provider fills the
	// rest, and the genres the NFO locked are kept.
	if up.Name != "Up (Local)" || up.Overview != "From the provider." || len(up.Genres) != 0 ||
		up.ExternalIDs[core.ProviderTMDB] != "14160" || up.Runtime != 90*time.Minute || up.MetadataRefreshedAt.IsZero() {
		t.Errorf("Up = %+v", up)
	}
	if src := f.store.sources[up.ID]; len(src) != 1 || src[0].ProbedAt.IsZero() || src[0].Container != "mkv" {
		t.Errorf("sources = %+v", src)
	}
	if c := f.store.credits[up.ID]; len(c) != 1 || f.store.people[c[0].PersonID].Name != "Pete Docter" {
		t.Errorf("credits = %+v", c)
	}
	if img := f.store.images[up.ID]; len(img) != 1 || img[0].RemoteURL == "" {
		t.Errorf("images = %+v", img)
	}
	if len(prober.probed) != 2 {
		t.Errorf("probed = %v", prober.probed)
	}

	// The scheduled scan runs after the interval, finds nothing new and
	// schedules the next one.
	f.clock = f.clock.Add(6 * time.Hour)
	if n := drain(t, w); n != 1 {
		t.Errorf("scheduled run = %d jobs", n)
	}
	pending := 0
	for _, j := range f.store.jobs {
		if j.State == "" || j.State == core.JobPending {
			pending++
			if !j.RunAt.Equal(f.clock.Add(6*time.Hour)) || j.UniqueKey != ScanJob(f.lib, time.Time{}, true).UniqueKey {
				t.Errorf("next scan = %+v", j)
			}
		}
	}
	if pending != 1 {
		t.Errorf("pending jobs = %d", pending)
	}
}

func TestLibraryManagerScanCases(t *testing.T) {
	portedCases(t, "library_manager_scan.json", ported{
		run: map[string]func(t *testing.T, a args){
			// Requesting a scan while one is queued or running queues
			// nothing more.
			"StartScanInBackground_QueuesOnlyWhenIdle": func(t *testing.T, a args) {
				m := newMemStore()
				lib := core.Library{ID: core.NewID()}
				if a.boolean(t, "scanRunning") {
					j := ScanJob(lib, m.clock(), false)
					if _, err := m.Jobs().Enqueue(t.Context(), &j); err != nil {
						t.Fatal(err)
					}
				}
				j := ScanJob(lib, m.clock(), false)
				added, err := m.Jobs().Enqueue(t.Context(), &j)
				if err != nil || added == a.boolean(t, "scanRunning") {
					t.Errorf("added = %v, %v", added, err)
				}
			},
		},
		facts: map[string]string{"ValidateMediaLibrary_RestartsScheduledScan": "TestRequestedScanBesideScheduled"},
	})
}

// TestRequestedScanBesideScheduled: a requested scan is queued even while a
// scheduled one is pending, rather than waiting for it.
func TestRequestedScanBesideScheduled(t *testing.T) {
	m := newMemStore()
	lib := core.Library{ID: core.NewID()}
	scheduled := ScanJob(lib, m.clock().Add(time.Hour), true)
	requested := ScanJob(lib, m.clock(), false)
	for _, j := range []*core.Job{&scheduled, &requested} {
		if added, err := m.Jobs().Enqueue(t.Context(), j); err != nil || !added {
			t.Errorf("%s: added = %v, %v", j.UniqueKey, added, err)
		}
	}
}
