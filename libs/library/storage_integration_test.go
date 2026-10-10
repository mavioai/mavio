package library

import (
	"context"
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library/storage"
)

func TestScanner_GrowthPolicy(t *testing.T) {
	f := newScan(t, core.LibraryMovies)
	tree(t, f.root, "Movie (2020)/Movie (2020).mkv", "Film (2021)/Film (2021).mkv.part", "Growing (2022)/Growing (2022).mkv")
	old := time.Now().Add(-time.Hour)
	for _, p := range []string{"Movie (2020)/Movie (2020).mkv", "Film (2021)/Film (2021).mkv.part", "Movie (2020)", "Film (2021)", "Growing (2022)"} {
		if err := os.Chtimes(filepath.Join(f.root, p), old, old); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.store.Libraries().Create(t.Context(), &f.lib); err != nil {
		t.Fatal(err)
	}
	clock := time.Now()
	policy := &storage.GrowthPolicy{Now: func() time.Time { return clock }}
	f.sc.GrowthPolicy = policy

	// The download and the file still being written are left out.
	f.scan()
	want := map[string]core.ItemKind{"Movie (2020)/Movie (2020).mkv": core.KindMovie}
	if got := f.items(); !maps.Equal(got, want) {
		t.Fatalf("items = %v, want %v", got, want)
	}
	if got := policy.Pending(); got != 2 {
		t.Fatalf("deferred = %d, want 2", got)
	}
	scans := func() int {
		n := 0
		for _, j := range f.store.jobs {
			if j.Kind == JobScan {
				n++
			}
		}
		f.store.jobs = nil
		return n
	}
	scans()
	// Nothing settled yet.
	f.sc.ReconcileDeferred(t.Context())
	if n := scans(); n != 0 {
		t.Fatalf("scans = %d, want none", n)
	}

	// The file is written no more: once stable its library is scanned
	// again, and the scan lists its folder although the folder's
	// modification time is as before.
	clock = clock.Add(time.Minute)
	f.sc.ReconcileDeferred(t.Context())
	clock = clock.Add(time.Minute)
	f.sc.ReconcileDeferred(t.Context())
	if n := scans(); n != 1 {
		t.Fatalf("scans = %d, want 1", n)
	}
	f.scan()
	want["Growing (2022)/Growing (2022).mkv"] = core.KindMovie
	if got := f.items(); !maps.Equal(got, want) {
		t.Fatalf("items after settling = %v, want %v", got, want)
	}

	// The download finishes: renamed, its library is scanned again.
	part := filepath.Join(f.root, "Film (2021)", "Film (2021).mkv.part")
	if err := os.Rename(part, strings.TrimSuffix(part, ".part")); err != nil {
		t.Fatal(err)
	}
	scans()
	f.sc.ReconcileDeferred(t.Context())
	if n := scans(); n != 1 {
		t.Fatalf("scans after the download = %d, want 1", n)
	}
	f.scan()
	want["Film (2021)/Film (2021).mkv"] = core.KindMovie
	if got := f.items(); !maps.Equal(got, want) {
		t.Fatalf("items after the download = %v, want %v", got, want)
	}
}

// rotational detects every path as on one hard disk.
func rotational(calls *atomic.Int32) *storage.Detector {
	return &storage.Detector{Detect: func(string) (storage.DeviceInfo, error) {
		calls.Add(1)
		return storage.DeviceInfo{ID: "dev:8:16", Kind: storage.KindLocalHDD, Rotational: true}, nil
	}}
}

func TestScanner_SerializedDevice(t *testing.T) {
	var calls atomic.Int32
	f := newScan(t, core.LibraryMovies)
	tree(t, f.root, "Up (2009)/Up (2009).mkv", "Heat (1995)/Heat (1995).mkv", "Alien (1979)/Alien (1979).mkv")
	ledger := storage.NewVolumeLedger()
	f.sc.VolumeLedger, f.sc.Devices = ledger, rotational(&calls)
	if got := f.sc.walkers(storage.DeviceInfo{Kind: storage.KindLocalSSD}); got != 4 {
		t.Errorf("walkers on an SSD = %d, want 4", got)
	}
	if got := f.sc.walkers(storage.DeviceInfo{Rotational: true}); got != 1 {
		t.Errorf("walkers on a hard disk = %d, want 1", got)
	}

	// A scan reads the disk's folders only while it holds the disk.
	hold, err := ledger.Acquire(t.Context(), "dev:8:16")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan ScanStats)
	go func() {
		st, err := f.sc.Scan(t.Context(), f.lib)
		if err != nil {
			t.Error(err)
		}
		done <- st
	}()
	select {
	case <-done:
		t.Fatal("the scan read the disk another reader held")
	case <-time.After(100 * time.Millisecond):
	}
	hold()
	if st := <-done; st.Saved != 3 {
		t.Errorf("saved = %d, want 3", st.Saved)
	}
	// Each folder's device is detected once, across scans.
	before := calls.Load()
	f.scan()
	if got := calls.Load(); got != before {
		t.Errorf("detections after another scan = %d, want %d", got, before)
	}

	// An item with two versions on the disk probes both, one at a time,
	// without waiting for itself.
	tree(t, f.root, "Up (2009)/Up (2009) - 4K.mkv")
	touchLater(t, filepath.Join(f.root, "Up (2009)"), time.Minute)
	f.scan()
	jobs := &Jobs{Store: f.store, Scanner: f.sc, Prober: &fakeProber{}, Now: func() time.Time { return f.clock }}
	up := f.item("Up (2009)/Up (2009).mkv")
	if len(f.store.sources[up.ID]) != 2 {
		t.Fatalf("versions = %+v", f.store.sources[up.ID])
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	payload, _ := json.Marshal(ProbePayload{ItemID: up.ID})
	if _, err := jobs.probe(ctx, core.Job{ID: core.NewID(), Kind: JobProbe, Payload: payload}); err != nil {
		t.Fatalf("probe of two versions on one disk: %v", err)
	}
}
