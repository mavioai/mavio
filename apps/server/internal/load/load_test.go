package load

import (
	"context"
	"math"
	"runtime"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/mavioai/mavio/libs/library/storage"
)

func TestSince(t *testing.T) {
	t0 := time.Unix(1000, 0)
	r0 := Reading{
		Time:    t0,
		CPU:     &CPUTimes{User: 10, System: 5, Total: 100, Server: 1},
		Disk:    &Counters{In: 1000, Out: 500},
		Network: &Counters{In: 4000, Out: 100},
	}
	r := Reading{
		Time:    t0.Add(2 * time.Second),
		CPU:     &CPUTimes{User: 14, System: 6, Total: 108, Server: 3},
		Memory:  &Memory{Total: 8, Used: 4},
		Disk:    &Counters{In: 3000, Out: 500},
		Network: &Counters{In: 3000, Out: 300},
	}
	s := r.since(r0)
	if got, want := *s.CPU, (CPU{User: 0.5, System: 0.125, Server: 0.25}); got != want {
		t.Errorf("CPU = %+v, want = %+v", got, want)
	}
	if got, want := *s.Disk, (Throughput{In: 1000, Out: 0, InTotal: 3000, OutTotal: 500}); got != want {
		t.Errorf("Disk = %+v, want = %+v", got, want)
	}
	// The received bytes went back, as when an interface went away.
	if got, want := *s.Network, (Throughput{In: 0, Out: 100, InTotal: 3000, OutTotal: 300}); got != want {
		t.Errorf("Network = %+v, want = %+v", got, want)
	}
	if s.Memory != r.Memory {
		t.Errorf("Memory = %v, want = %v", s.Memory, r.Memory)
	}

	// Parts not read are not told.
	if s := (Reading{Time: r.Time}).since(r0); s.CPU != nil || s.Disk != nil || s.Network != nil {
		t.Errorf("since without counters = %+v, want = no parts", s)
	}
}

func TestSamplerSamplesWhileAsked(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var reads atomic.Int64
		s := &Sampler{read: func(context.Context) Reading {
			n := float64(reads.Add(1))
			return Reading{Time: time.Now(), CPU: &CPUTimes{User: n, Total: 2 * n}}
		}}
		if got := s.Samples(); len(got) != 0 {
			t.Fatalf("first Samples = %d samples, want = 0", len(got))
		}
		time.Sleep(3 * Interval)
		synctest.Wait()
		got := s.Samples()
		if len(got) != 3 {
			t.Fatalf("Samples = %d samples, want = 3", len(got))
		}
		if got[0].CPU.User != 0.5 {
			t.Errorf("CPU.User = %v, want = 0.5", got[0].CPU.User)
		}

		// Two minutes' samples are kept.
		for range 2 * Keep {
			time.Sleep(Interval)
			s.Samples()
		}
		if got := len(s.Samples()); got != Keep {
			t.Errorf("Samples = %d samples, want = %d", got, Keep)
		}

		// No one asked: sampling stops, and starts afresh when asked again.
		time.Sleep(Idle + 2*Interval)
		synctest.Wait()
		before := reads.Load()
		time.Sleep(10 * Interval)
		synctest.Wait()
		if reads.Load() != before {
			t.Errorf("reads went on after no one asked")
		}
		if got := s.Samples(); len(got) != 0 {
			t.Errorf("Samples after stopping = %d samples, want = 0", len(got))
		}
		time.Sleep(Idle + 2*Interval)
	})
}

// TestReadThisComputer reads this computer's counters, which every
// supported operating system tells.
func TestReadThisComputer(t *testing.T) {
	switch runtime.GOOS {
	case "linux", "darwin", "windows":
	default:
		t.Skipf("load is not read on %s", runtime.GOOS)
	}
	r := newReader(storage.Volumes)
	ctx := context.Background()
	r0 := r.read(ctx)
	time.Sleep(100 * time.Millisecond)
	s := r.read(ctx).since(r0)
	if s.CPU == nil || math.IsNaN(s.CPU.User) {
		t.Errorf("CPU = %v, want = read", s.CPU)
	}
	if s.Memory == nil || s.Memory.Total == 0 || s.Memory.Used > s.Memory.Total || s.Memory.Server == 0 {
		t.Errorf("Memory = %+v, want = read", s.Memory)
	}
	if s.Space == nil || s.Space.Total == 0 || s.Space.Available > s.Space.Total {
		t.Errorf("Space = %+v, want = read", s.Space)
	}
	if s.Network == nil {
		t.Errorf("Network = nil, want = read")
	}
}

func TestSpaceOf(t *testing.T) {
	got := spaceOf([]storage.Volume{
		{Path: "/", Kind: storage.KindLocalSSD, Total: 1000, Available: 300},
		// Another volume of the same APFS container.
		{Path: "/Volumes/Work", Kind: storage.KindLocalSSD, Total: 1000, Available: 300},
		{Path: "/mnt/usb", Kind: storage.KindLocalHDD, Total: 500, Available: 100},
		{Path: "/mnt/nas", Kind: storage.KindRemoteNAS, Total: 9000, Available: 9000},
		{Path: "/mnt/drive", Kind: storage.KindCloudMount, Total: 9000, Available: 9000},
		{Path: "/mnt/gone", Kind: storage.KindUnknown},
	})
	if want := (Space{Total: 1500, Available: 400}); got == nil || *got != want {
		t.Errorf("spaceOf = %v, want = %+v", got, want)
	}
	if got := spaceOf([]storage.Volume{{Path: "/mnt/nas", Kind: storage.KindRemoteNAS, Total: 1}}); got != nil {
		t.Errorf("spaceOf of shares = %+v, want = nil", got)
	}
}
