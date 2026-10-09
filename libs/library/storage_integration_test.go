package library

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library/storage"
)

func TestScanner_GrowthPolicyAndTempFileFilter(t *testing.T) {
	dir := t.TempDir()
	moviesDir := filepath.Join(dir, "Movies", "Movie (2020)")
	if err := os.MkdirAll(moviesDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// 1. Stable media file (mtime in the past)
	stablePath := filepath.Join(moviesDir, "Movie (2020).mkv")
	if err := os.WriteFile(stablePath, []byte("fake video content"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-1 * time.Hour)
	_ = os.Chtimes(stablePath, oldTime, oldTime)

	// 2. Temporary download file
	tempPath := filepath.Join(moviesDir, "Movie (2020).mkv.part")
	if err := os.WriteFile(tempPath, []byte("partial content"), 0o644); err != nil {
		t.Fatal(err)
	}

	// 3. Actively growing file (mtime is now)
	growingPath := filepath.Join(moviesDir, "Downloading (2020).mkv")
	if err := os.WriteFile(growingPath, []byte("growing content"), 0o644); err != nil {
		t.Fatal(err)
	}

	store := newMemStore()
	lib := core.Library{ID: core.NewID(), Name: "Movies", Kind: core.LibraryMovies, Paths: []string{filepath.Join(dir, "Movies")}}
	if err := store.Libraries().Create(context.Background(), &lib); err != nil {
		t.Fatal(err)
	}

	growthPolicy := storage.NewGrowthPolicy(30 * time.Second)
	// Prime growingPath with previous size so it is detected as actively growing
	growthPolicy.IsGrowing(growingPath, 5, oldTime)
	scanner := &Scanner{
		Store:        store,
		Resolver:     NewResolver(),
		GrowthPolicy: growthPolicy,
	}

	stats, err := scanner.Scan(context.Background(), lib)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	// The temporary file and growing file should not be saved as items.
	// Only the stable movie file should be listed and saved.
	if stats.Saved != 1 {
		t.Errorf("saved = %d, want 1 (only stable movie)", stats.Saved)
	}
}

func TestScanner_QuietGateInterlock(t *testing.T) {
	gate := storage.NewQuietGate(20 * time.Millisecond)
	releasePlayback := gate.AcquirePlayback("session-123")

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// While playback is active, PauseOrCancel should wait until timeout
	err := gate.PauseOrCancel(ctx)
	if err == nil {
		t.Errorf("expected PauseOrCancel to block while playback is active, got nil")
	}

	// Now release playback
	releasePlayback()

	// Note activity with 0 to allow immediate pass in test
	gate.NoteActivity(1 * time.Millisecond)
	time.Sleep(5 * time.Millisecond)

	ctx2, cancel2 := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel2()
	if err := gate.PauseOrCancel(ctx2); err != nil {
		t.Errorf("expected PauseOrCancel to succeed after playback released, got: %v", err)
	}
}

func TestScanner_VolumeLedgerSerialization(t *testing.T) {
	ledger := storage.NewVolumeLedger(time.Millisecond, time.Millisecond)
	ctx := context.Background()

	rel1, err := ledger.Acquire(ctx, "vol:1")
	if err != nil {
		t.Fatalf("failed to acquire vol:1: %v", err)
	}

	acquiredVol2 := make(chan struct{})
	go func() {
		rel2, err2 := ledger.Acquire(ctx, "vol:1")
		if err2 != nil {
			return
		}
		close(acquiredVol2)
		rel2()
	}()

	select {
	case <-acquiredVol2:
		t.Fatalf("vol:1 acquired concurrently without release!")
	case <-time.After(20 * time.Millisecond):
		// Expected: blocked waiting for rel1
	}

	rel1()

	select {
	case <-acquiredVol2:
		// Successfully acquired after release
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("timed out waiting for vol:1 to be acquired after release")
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
	ledger := storage.NewVolumeLedger(0, 0)
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
