package load

import (
	"context"
	"log/slog"
	"slices"
	"sync"
	"time"
)

// Sample is the load over the interval ending at Time. A part is nil
// where the operating system does not tell it.
type Sample struct {
	Time    time.Time
	CPU     *CPU
	Memory  *Memory
	Disk    *Throughput
	Network *Throughput
}

// CPU is the share of all the computer's cores busy, from 0 to 1.
type CPU struct {
	// User is running programs; System, the operating system for them;
	// Server, the server's process, of either.
	User, System, Server float64
}

// Memory is the memory in use, in bytes.
type Memory struct {
	// Used is what is not available to programs.
	Total, Used uint64
	SwapTotal   uint64
	SwapUsed    uint64
	// Server is what the server's process takes.
	Server uint64
}

// Throughput is what disks read and write, or network interfaces receive
// and send, together: In and Out in bytes per second, InTotal and OutTotal
// in bytes since the computer started.
type Throughput struct {
	In, Out           float64
	InTotal, OutTotal uint64
}

// Defaults of a Sampler.
const (
	// Interval is how often the load is sampled.
	Interval = 2 * time.Second
	// Keep is how many samples are kept: two minutes'.
	Keep = 60
	// Idle is how long sampling goes on after the last request.
	Idle = 30 * time.Second
)

// Sampler samples the load every Interval from the first call of Samples
// on, until none came for Idle.
type Sampler struct {
	read func(context.Context) Reading

	mu      sync.Mutex
	samples []Sample
	asked   time.Time
	running bool
}

// New samples the load of this computer.
func New() *Sampler {
	return NewFrom(newReader().read)
}

// NewFrom samples the readings read takes, as tests give.
func NewFrom(read func(context.Context) Reading) *Sampler {
	return &Sampler{read: read}
}

// Samples returns the samples kept, oldest first, and keeps sampling for
// Idle more; the first call starts it, and returns none.
func (s *Sampler) Samples() []Sample {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.asked = time.Now()
	if !s.running {
		s.running = true
		s.samples = nil
		go s.run()
	}
	return slices.Clone(s.samples)
}

// run samples until no one asked for Idle. It outlives the request that
// started it, so it runs on a context of its own.
func (s *Sampler) run() {
	ctx := context.Background()
	last := s.read(ctx)
	t := time.NewTicker(Interval)
	defer t.Stop()
	for range t.C {
		r := s.read(ctx)
		sample := r.since(last)
		last = r
		s.mu.Lock()
		if time.Since(s.asked) > Idle {
			s.running = false
			s.mu.Unlock()
			slog.DebugContext(ctx, "load sampling stopped: no one watches")
			return
		}
		s.samples = append(s.samples, sample)
		if n := len(s.samples); n > Keep {
			s.samples = slices.Delete(s.samples, 0, n-Keep)
		}
		s.mu.Unlock()
	}
}

// Reading is the computer's counters at a time; a part is nil where it was
// not read.
type Reading struct {
	Time time.Time
	// CPU is the seconds all cores spent so: Total is all, idle included.
	CPU *CPUTimes
	// Memory is read as is.
	Memory *Memory
	// Disk and Network are the bytes since the computer started.
	Disk, Network *Counters
}

// CPUTimes are seconds of CPU time, summed over the cores.
type CPUTimes struct {
	User, System, Total, Server float64
}

// Counters are bytes since the computer started.
type Counters struct {
	In, Out uint64
}

// since is the load over the interval from r0 to r.
func (r Reading) since(r0 Reading) Sample {
	s := Sample{Time: r.Time, Memory: r.Memory}
	secs := r.Time.Sub(r0.Time).Seconds()
	if r.CPU != nil && r0.CPU != nil {
		if total := r.CPU.Total - r0.CPU.Total; total > 0 {
			share := func(a, b float64) float64 { return min(1, max(0, (a-b)/total)) }
			s.CPU = &CPU{
				User:   share(r.CPU.User, r0.CPU.User),
				System: share(r.CPU.System, r0.CPU.System),
				Server: share(r.CPU.Server, r0.CPU.Server),
			}
		}
	}
	through := func(c, c0 *Counters) *Throughput {
		if c == nil || c0 == nil || secs <= 0 {
			return nil
		}
		// A counter that went back, as an interface gone, adds nothing.
		rate := func(a, b uint64) float64 {
			if a < b {
				return 0
			}
			return float64(a-b) / secs
		}
		return &Throughput{In: rate(c.In, c0.In), Out: rate(c.Out, c0.Out), InTotal: c.In, OutTotal: c.Out}
	}
	s.Disk = through(r.Disk, r0.Disk)
	s.Network = through(r.Network, r0.Network)
	return s
}
