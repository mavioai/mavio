package library

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

// fakeKeyframes finds keyframes every two seconds, none in files named
// "still".
type fakeKeyframes struct{ read []string }

func (k *fakeKeyframes) Keyframes(_ context.Context, path string) ([]time.Duration, error) {
	k.read = append(k.read, path)
	if strings.Contains(path, "still") {
		return nil, nil
	}
	return []time.Duration{0, 2 * time.Second, 4 * time.Second}, nil
}

func TestKeyframesJob(t *testing.T) {
	f := newScan(t, core.LibraryMovies)
	tree(t, f.root, "Up (2009)/Up (2009).mkv", "still (2000)/still (2000).mkv")
	if err := f.store.Libraries().Create(t.Context(), &f.lib); err != nil {
		t.Fatal(err)
	}
	kf := &fakeKeyframes{}
	jobs := &Jobs{
		Store: f.store, Scanner: f.sc, Prober: &fakeProber{}, Keyframes: kf, Now: func() time.Time { return f.clock },
		Refresher: &Refresher{Store: f.store, Now: func() time.Time { return f.clock }},
	}
	w := &Worker{Queue: f.store.Jobs(), Owner: "test", Handlers: jobs.Handlers()}
	if err := jobs.Schedule(t.Context()); err != nil {
		t.Fatal(err)
	}
	// One scan, and per movie a probe, a refresh and a keyframe job.
	if n := drain(t, w); n != 7 {
		t.Errorf("jobs run = %d, want 7", n)
	}
	up, still := f.store.sources[f.item("Up (2009)/Up (2009).mkv").ID], f.store.sources[f.item("still (2000)/still (2000).mkv").ID]
	if len(up) != 1 || len(up[0].Keyframes) != 3 || NeedsKeyframes(&up[0]) {
		t.Errorf("Up sources = %+v", up)
	}
	// A file without keyframes is marked as read.
	if len(still) != 1 || still[0].Keyframes == nil || NeedsKeyframes(&still[0]) {
		t.Errorf("still sources = %+v", still)
	}

	// A keyframe job for an item done reads nothing.
	job := KeyframesJob(f.item("Up (2009)/Up (2009).mkv").ID, f.clock, KeyframesUrgent)
	if _, err := f.store.Jobs().Enqueue(t.Context(), &job); err != nil {
		t.Fatal(err)
	}
	drain(t, w)
	if len(kf.read) != 2 {
		t.Errorf("files read = %v, want each once", kf.read)
	}
}
