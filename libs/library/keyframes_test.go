package library

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

// fakeKeyframes finds keyframes every two seconds in the index of files
// named "indexed" and by reading other files whole, none in files named
// "still".
type fakeKeyframes struct{ read, whole []string }

func (k *fakeKeyframes) Keyframes(_ context.Context, f MediaFile, whole bool) ([]time.Duration, bool, error) {
	path := f.Path
	if !strings.Contains(path, "indexed") {
		if !whole {
			return nil, false, nil
		}
		k.whole = append(k.whole, path)
	}
	k.read = append(k.read, path)
	if strings.Contains(path, "still") {
		return nil, true, nil
	}
	return []time.Duration{0, 2 * time.Second, 4 * time.Second}, true, nil
}

func TestKeyframesJob(t *testing.T) {
	for _, tt := range []struct {
		name  string
		class core.StorageClass
		// whole lists the files read whole, still those left without
		// keyframes.
		whole, still []string
		jobs         int
	}{
		// One scan, per movie a probe and a refresh, and a keyframe job
		// for each movie without an index.
		{"local", core.StorageLocal, []string{"Up (2009)/Up (2009).mkv", "still (2000)/still (2000).mkv"}, nil, 9},
		// On cloud storage, files without an index are not read whole.
		{"cloud", core.StorageCloud, nil, []string{"Up (2009)/Up (2009).mkv", "still (2000)/still (2000).mkv"}, 9},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newScan(t, core.LibraryMovies)
			tree(t, f.root, "Up (2009)/Up (2009).mkv", "still (2000)/still (2000).mkv", "indexed (2001)/indexed (2001).mkv")
			f.lib.StorageClasses = map[string]core.StorageClass{f.root: tt.class}
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
			if n := drain(t, w); n != tt.jobs {
				t.Errorf("jobs run = %d, want = %d", n, tt.jobs)
			}
			var whole []string
			for _, p := range kf.whole {
				whole = append(whole, strings.TrimPrefix(p, f.root+"/"))
			}
			slices.Sort(whole)
			if !slices.Equal(whole, tt.whole) {
				t.Errorf("read whole = %v, want = %v", whole, tt.whole)
			}
			indexed := f.store.sources[f.item("indexed (2001)/indexed (2001).mkv").ID]
			if len(indexed) != 1 || len(indexed[0].Keyframes) != 3 {
				t.Errorf("indexed sources = %+v", indexed)
			}
			for _, name := range []string{"Up (2009)/Up (2009).mkv", "still (2000)/still (2000).mkv"} {
				src := f.store.sources[f.item(name).ID]
				// A file read without keyframes is marked as read.
				if want := slices.Contains(tt.still, name); len(src) != 1 || NeedsKeyframes(&src[0]) != want {
					t.Errorf("%s sources = %+v, want needing keyframes %v", name, src, want)
				}
			}

			// A keyframe job for an item done reads nothing.
			read := len(kf.read)
			job := KeyframesJob(f.item("indexed (2001)/indexed (2001).mkv").ID, f.clock, KeyframesUrgent)
			if _, err := f.store.Jobs().Enqueue(t.Context(), &job); err != nil {
				t.Fatal(err)
			}
			drain(t, w)
			if len(kf.read) != read {
				t.Errorf("files read = %v, want each once", kf.read)
			}
		})
	}
}
