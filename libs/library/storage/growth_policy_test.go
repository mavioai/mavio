package storage

import (
	"slices"
	"testing"
	"time"
)

func TestHasDownloadSuffix(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"/downloads/movie.mkv.part", true},
		{"/downloads/movie.mp4.crdownload", true},
		{"/downloads/movie.download", true},
		{"/downloads/movie.aria2", true},
		{"/downloads/movie.mkv.tmp", true},
		{"/downloads/movie.incomplete", true},
		{"/downloads/movie.!ut", true},
		{"/downloads/movie.mkv.!qB", true},
		{`C:\downloads\movie.mkv.PART`, true},
		{"/downloads/movie.mkv", false},
		{"/downloads/movie.mp4", false},
		{"/downloads/part.mkv", false},
	}
	for _, tc := range cases {
		if got := HasDownloadSuffix(tc.path); got != tc.want {
			t.Errorf("HasDownloadSuffix(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestGrowthPolicy(t *testing.T) {
	clock := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	p := &GrowthPolicy{Now: func() time.Time { return clock }}
	old := clock.Add(-time.Hour)
	files := map[string]FileStamp{}
	stat := func(name string) (FileStamp, bool) { s, ok := files[name]; return s, ok }
	advance := func(d time.Duration) { clock = clock.Add(d) }

	// A file at rest is not growing and is not deferred.
	if p.Growing("lib1", "/m/Old.mkv", FileStamp{Size: 10, ModTime: old}) {
		t.Error("Growing(at rest) = true, want false")
	}
	// One modified within the write window is.
	files["/m/New.mkv"] = FileStamp{Size: 10, ModTime: clock.Add(-2 * time.Second)}
	if !p.Growing("lib1", "/m/New.mkv", files["/m/New.mkv"]) {
		t.Error("Growing(just written) = false, want true")
	}
	// So is a download, whatever its age.
	files["/m/Film.mkv.part"] = FileStamp{Size: 5, ModTime: old}
	if !p.Growing("lib2", "/m/Film.mkv.part", files["/m/Film.mkv.part"]) {
		t.Error("Growing(.part) = false, want true")
	}
	if got := p.Pending(); got != 2 {
		t.Fatalf("Pending = %d, want 2", got)
	}

	// Still being written: nothing settles.
	advance(20 * time.Second)
	files["/m/New.mkv"] = FileStamp{Size: 20, ModTime: clock.Add(-time.Second)}
	if got := p.Reconcile(stat); len(got) != 0 {
		t.Errorf("Reconcile while writing = %v, want none", got)
	}
	// A scan before it settles still defers it, though it is outside the
	// write window by then.
	advance(15 * time.Second)
	if !p.Growing("lib1", "/m/New.mkv", files["/m/New.mkv"]) {
		t.Error("Growing(deferred, not yet stable) = false, want true")
	}
	// Stable for StableFor: its library is scanned again, once.
	advance(16 * time.Second)
	if got := p.Reconcile(stat); !slices.Equal(got, []string{"lib1"}) {
		t.Errorf("Reconcile after settling = %v, want [lib1]", got)
	}
	if got := p.Reconcile(stat); len(got) != 0 {
		t.Errorf("Reconcile again = %v, want none", got)
	}
	if p.Growing("lib1", "/m/New.mkv", files["/m/New.mkv"]) {
		t.Error("Growing(settled) = true, want false")
	}

	// A download settles only by going away, renamed when it finishes.
	advance(time.Hour)
	if got := p.Reconcile(stat); len(got) != 0 {
		t.Errorf("Reconcile of a resting download = %v, want none", got)
	}
	delete(files, "/m/Film.mkv.part")
	if got := p.Reconcile(stat); !slices.Equal(got, []string{"lib2"}) {
		t.Errorf("Reconcile after the download finished = %v, want [lib2]", got)
	}
	if got := p.Pending(); got != 0 {
		t.Errorf("Pending = %d, want 0", got)
	}

	// A download that never finishes is forgotten after a day.
	files["/m/Stuck.mkv.part"] = FileStamp{Size: 1, ModTime: old}
	p.Growing("lib2", "/m/Stuck.mkv.part", files["/m/Stuck.mkv.part"])
	advance(deferredTTL)
	if got := p.Reconcile(stat); len(got) != 0 || p.Pending() != 0 {
		t.Errorf("Reconcile after a day = %v with %d pending, want none", got, p.Pending())
	}
}
