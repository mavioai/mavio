package storage

import (
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestKeeper(t *testing.T) {
	devices := map[string]DeviceInfo{
		"/ssd":   {ID: "dev:1", Kind: KindLocalSSD},
		"/hdd":   {ID: "dev:2", Kind: KindLocalHDD, Rotational: true},
		"/nas":   {ID: "fsid:3", Kind: KindRemoteNAS, Remote: true},
		"/cloud": {ID: "fsid:4", Kind: KindCloudMount, Remote: true},
		"/what":  {Kind: KindUnknown},
	}
	clock := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var mu sync.Mutex
	var reads, prefetches []string
	release := make(chan struct{})
	k := &Keeper{
		Devices: &Detector{Detect: func(p string) (DeviceInfo, error) {
			return devices["/"+strings.Split(p, "/")[1]], nil
		}},
		Interval: time.Second, WakeCooldown: 30 * time.Second,
		Now: func() time.Time { return clock },
		Read: func(p string) error {
			if strings.HasSuffix(p, "hung.mkv") {
				<-release
			}
			mu.Lock()
			reads = append(reads, p)
			mu.Unlock()
			return nil
		},
		Prefetch: func(p string) error {
			mu.Lock()
			prefetches = append(prefetches, p)
			mu.Unlock()
			return nil
		},
	}
	got := func() []string {
		k.Wait()
		mu.Lock()
		defer mu.Unlock()
		out := slices.Sorted(slices.Values(reads))
		reads = nil
		return out
	}

	// One read per sleeping volume, none for the others.
	if n := k.Beat("/ssd/a.mkv", "/hdd/a.mkv", "/hdd/b.mkv", "/nas/a.mkv", "/cloud/a.mkv", "/what/a.mkv"); n != 2 {
		t.Errorf("Beat started %d reads, want 2", n)
	}
	if r := got(); !slices.Equal(r, []string{"/hdd/a.mkv", "/nas/a.mkv"}) {
		t.Errorf("reads = %v", r)
	}
	// Within the interval nothing is read; after it, again.
	clock = clock.Add(500 * time.Millisecond)
	if n := k.Beat("/hdd/a.mkv"); n != 0 {
		t.Errorf("Beat within the interval started %d reads, want 0", n)
	}
	clock = clock.Add(time.Second)
	if n := k.Beat("/hdd/a.mkv"); n != 1 {
		t.Errorf("Beat after the interval started %d reads, want 1", n)
	}
	got()

	// A hung read holds its volume: no reads pile up behind it.
	clock = clock.Add(time.Minute)
	k.Beat("/nas/hung.mkv")
	clock = clock.Add(time.Minute)
	if n := k.Beat("/nas/a.mkv"); n != 0 {
		t.Errorf("Beat behind a hung read started %d reads, want 0", n)
	}
	close(release)
	got()

	// A wake reads once per cooldown and warms the file.
	clock = clock.Add(time.Minute)
	if !k.Wake("/hdd/movie.mkv") {
		t.Error("Wake = false, want true")
	}
	if k.Wake("/hdd/other.mkv") {
		t.Error("Wake within the cooldown = true, want false")
	}
	if k.Wake("/ssd/movie.mkv") {
		t.Error("Wake of an SSD = true, want false")
	}
	if r := got(); !slices.Equal(r, []string{"/hdd/movie.mkv"}) || !slices.Equal(prefetches, []string{"/hdd/movie.mkv"}) {
		t.Errorf("reads = %v, prefetches = %v", r, prefetches)
	}
	clock = clock.Add(31 * time.Second)
	if !k.Wake("/hdd/other.mkv") {
		t.Error("Wake after the cooldown = false, want true")
	}
	got()
}
