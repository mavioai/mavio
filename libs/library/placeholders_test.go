package library

import (
	"context"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

type fakeAnalyzer struct{ analyzed []core.ID }

func (a *fakeAnalyzer) Analyze(_ context.Context, img core.Image) (ImageFacts, error) {
	a.analyzed = append(a.analyzed, img.ID)
	return ImageFacts{Width: 400, Height: 600, Blurhash: "LEHV6nWB2yk8", Thumbhash: []byte{1}}, nil
}

func TestPlaceholdersJob(t *testing.T) {
	f := newScan(t, core.LibraryMovies)
	tree(t, f.root, "Up (2009)/Up (2009).mkv")
	if err := f.store.Libraries().Create(t.Context(), &f.lib); err != nil {
		t.Fatal(err)
	}
	analyzer := &fakeAnalyzer{}
	now := func() time.Time { return f.clock }
	jobs := &Jobs{
		Store: f.store, Scanner: f.sc, Prober: &fakeProber{}, Images: analyzer, Now: now,
		Refresher: &Refresher{Store: f.store, Providers: []Provider{&fakeProvider{}}, Now: now},
	}
	w := &Worker{Queue: f.store.Jobs(), Owner: "test", Handlers: jobs.Handlers()}
	if err := jobs.Schedule(t.Context()); err != nil {
		t.Fatal(err)
	}
	// A scan, a probe, a refresh and the placeholders of its images.
	if n := drain(t, w); n != 4 {
		t.Errorf("jobs run = %d, want 4", n)
	}
	up := f.item("Up (2009)/Up (2009).mkv")
	images := f.store.images[up.ID]
	if len(images) != 1 || images[0].Blurhash != "LEHV6nWB2yk8" || images[0].Width != 400 || len(analyzer.analyzed) != 1 {
		t.Fatalf("images = %+v, analyzed %v", images, analyzer.analyzed)
	}

	// Refreshing again keeps the image and its placeholders; nothing is
	// analyzed again.
	job := RefreshJob(up.ID, f.clock)
	if _, err := f.store.Jobs().Enqueue(t.Context(), &job); err != nil {
		t.Fatal(err)
	}
	drain(t, w)
	again := f.store.images[up.ID]
	if len(again) != 1 || again[0].ID != images[0].ID || again[0].Blurhash == "" || len(analyzer.analyzed) != 1 {
		t.Errorf("after refresh = %+v, analyzed %v", again, analyzer.analyzed)
	}
}
