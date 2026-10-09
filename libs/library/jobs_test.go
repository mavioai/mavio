package library

import (
	"context"
	"errors"
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
<movie><title>Up (Local)</title><mpaa>Rated PG</mpaa><lockedfields>Genres</lockedfields></movie>`

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
	// The collections library has no folders to scan.
	if err := f.store.Libraries().Create(t.Context(), &core.Library{Name: "Collections", Kind: core.LibraryCollections}); err != nil {
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
	// The rating's score in the US system, as the library has no country.
	if up.OfficialRating != "Rated PG" || up.ParentalRating == nil || *up.ParentalRating != 10 {
		t.Errorf("Up rated %q, score %v; want 10", up.OfficialRating, up.ParentalRating)
	}
	if brazil := f.item("Brazil (1985)/Brazil (1985).mkv"); brazil.ParentalRating != nil {
		t.Errorf("unrated Brazil has score %d", *brazil.ParentalRating)
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

func TestJobsRefreshNewSeries(t *testing.T) {
	f := newScan(t, core.LibraryShows)
	tree(t, f.root, "Lost/Season 1/Lost S01E01.mkv")
	write(t, filepath.Join(f.root, "Lost", "tvshow.nfo"), `<tvshow><title>Lost</title><mpaa>TV-14</mpaa><genre>Drama</genre></tvshow>`)
	if err := f.store.Libraries().Create(t.Context(), &f.lib); err != nil {
		t.Fatal(err)
	}
	jobs := &Jobs{
		Store: f.store, Scanner: f.sc, Prober: &fakeProber{}, Now: func() time.Time { return f.clock },
		Refresher: &Refresher{Store: f.store, Now: func() time.Time { return f.clock }},
	}
	w := &Worker{Queue: f.store.Jobs(), Owner: "test", Handlers: jobs.Handlers()}
	if err := jobs.Schedule(t.Context()); err != nil {
		t.Fatal(err)
	}
	// The scan, the series' and the season's refreshes, the episode's
	// probe and refresh.
	if n := drain(t, w); n != 5 {
		t.Errorf("jobs run = %d, want 5", n)
	}
	series := f.item("Lost")
	if series.OfficialRating != "TV-14" || len(series.Genres) != 1 || series.MetadataRefreshedAt.IsZero() ||
		series.ParentalRating == nil || *series.ParentalRating != 14 {
		t.Errorf("series = %+v", series)
	}
}

type fakeBorders struct{ calls int }

func (b *fakeBorders) Borders(_ context.Context, _ string, duration time.Duration, width, height int) (core.Crop, error) {
	b.calls++
	if duration <= 0 || width == 0 || height == 0 {
		return core.Crop{}, errors.New("no size")
	}
	return core.Crop{Top: 140, Bottom: 140}, nil
}

func TestJobsBorders(t *testing.T) {
	f := newScan(t, core.LibraryMovies)
	tree(t, f.root, "Up (2009)/Up (2009).mkv")
	if err := f.store.Libraries().Create(t.Context(), &f.lib); err != nil {
		t.Fatal(err)
	}
	borders := &fakeBorders{}
	prober := &sizedProber{}
	jobs := &Jobs{
		Store: f.store, Scanner: f.sc, Prober: prober, Borders: borders, Now: func() time.Time { return f.clock },
		Refresher: &Refresher{Store: f.store, Now: func() time.Time { return f.clock }},
	}
	w := &Worker{Queue: f.store.Jobs(), Owner: "test", Handlers: jobs.Handlers()}
	if err := jobs.Schedule(t.Context()); err != nil {
		t.Fatal(err)
	}
	// The scan, the probe, the refresh and the borders.
	if n := drain(t, w); n != 4 {
		t.Errorf("jobs run = %d, want 4", n)
	}
	up := f.item("Up (2009)/Up (2009).mkv")
	video := f.store.sources[up.ID][0].Streams[0]
	if video.Crop == nil || *video.Crop != (core.Crop{Top: 140, Bottom: 140}) || borders.calls != 1 {
		t.Errorf("crop = %v after %d detections", video.Crop, borders.calls)
	}
	// Looked for once: another run does nothing.
	if _, err := jobs.borders(t.Context(), BordersJob(up.ID, f.clock)); err != nil || borders.calls != 1 {
		t.Errorf("second run: %v, %d detections", err, borders.calls)
	}
}

// sizedProber probes a 1080p video.
type sizedProber struct{}

func (sizedProber) Probe(context.Context, string, bool) (ProbeResult, error) {
	return ProbeResult{Source: core.MediaSource{
		Container: "mkv", Duration: 90 * time.Minute,
		Streams: []core.MediaStream{{Index: 0, Kind: core.StreamVideo, Codec: "h264", Width: 1920, Height: 1080}},
	}}, nil
}

func TestCleanup(t *testing.T) {
	f := newScan(t, core.LibraryMovies)
	jobs := &Jobs{Store: f.store, Now: func() time.Time { return f.clock }}
	f.store.clock = func() time.Time { return f.clock }
	old := f.clock.Add(-8 * 24 * time.Hour)
	f.store.jobs = append(f.store.jobs,
		core.Job{ID: core.NewID(), Kind: JobScan, State: core.JobSucceeded, FinishedAt: &old},
		core.Job{ID: core.NewID(), Kind: JobScan, State: core.JobFailed, FinishedAt: &f.clock},
	)
	if err := jobs.ScheduleHousekeeping(t.Context()); err != nil {
		t.Fatal(err)
	}
	f.clock = f.clock.Add(time.Minute)
	w := &Worker{Queue: f.store.Jobs(), Owner: "test", Handlers: jobs.Handlers()}
	if n := drain(t, w); n != 1 {
		t.Errorf("jobs run = %d, want 1", n)
	}
	var kinds []string
	for _, j := range f.store.jobs {
		kinds = append(kinds, j.Kind+":"+string(j.State))
	}
	// The old job is gone, the recent one stays, and the next cleanup waits.
	if len(kinds) != 3 || kinds[0] != "library.scan:failed" || kinds[1] != "jobs.cleanup:succeeded" || kinds[2] != "jobs.cleanup:" {
		t.Errorf("jobs = %v", kinds)
	}
}
