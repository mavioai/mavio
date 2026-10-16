package load

import (
	"context"
	"log/slog"
	stdnet "net"
	"os"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"

	"github.com/mavioai/mavio/libs/library/storage"
)

// reader reads this computer's counters, logging each part it cannot read
// once.
type reader struct {
	volumes func(context.Context) ([]storage.Volume, error)
	self    *process.Process
	failed  sync.Map

	mu       sync.Mutex
	space    *Space
	spaceAt  time.Time
	spaceRun bool
}

// spaceAge is how long the disks' space is kept before it is read again:
// it changes slowly, and a share that does not answer holds its reading
// up for seconds.
const spaceAge = 30 * time.Second

func newReader(volumes func(context.Context) ([]storage.Volume, error)) *reader {
	r := &reader{volumes: volumes}
	if p, err := process.NewProcess(int32(os.Getpid())); err == nil {
		r.self = p
	}
	return r
}

func (r *reader) read(ctx context.Context) Reading {
	out := Reading{Time: time.Now()}
	if ts, err := cpu.TimesWithContext(ctx, false); r.ok(ctx, "cpu", err) && len(ts) > 0 {
		t := ts[0]
		// Linux counts guests' time in User and Nice already.
		busy := CPUTimes{User: t.User + t.Nice, System: t.System + t.Irq + t.Softirq}
		busy.Total = busy.User + busy.System + t.Idle + t.Iowait + t.Steal
		out.CPU = &busy
		if r.self != nil {
			if p, err := r.self.TimesWithContext(ctx); r.ok(ctx, "server cpu", err) {
				out.CPU.Server = p.User + p.System
			}
		}
	}
	if v, err := mem.VirtualMemoryWithContext(ctx); r.ok(ctx, "memory", err) {
		m := &Memory{Total: v.Total, Used: v.Total - min(v.Available, v.Total)}
		if s, err := mem.SwapMemoryWithContext(ctx); r.ok(ctx, "swap", err) {
			m.SwapTotal, m.SwapUsed = s.Total, s.Used
		}
		if r.self != nil {
			if p, err := r.self.MemoryInfoWithContext(ctx); r.ok(ctx, "server memory", err) {
				m.Server = p.RSS
			}
		}
		out.Memory = m
	}
	if ds, err := disk.IOCountersWithContext(ctx); r.ok(ctx, "disks", err) {
		c := &Counters{}
		for name, d := range ds {
			if wholeDisk(name) {
				c.In += d.ReadBytes
				c.Out += d.WriteBytes
			}
		}
		out.Disk = c
	}
	out.Space = r.diskSpace(ctx)
	if ns, err := net.IOCountersWithContext(ctx, true); r.ok(ctx, "network", err) {
		c := &Counters{}
		for _, n := range counted(ns) {
			c.In += n.BytesRecv
			c.Out += n.BytesSent
		}
		out.Network = c
	}
	return out
}

// diskSpace returns the space of the computer's own disks: read at first,
// then kept, and read again in the background once older than spaceAge.
func (r *reader) diskSpace(ctx context.Context) *Space {
	r.mu.Lock()
	first, stale := r.spaceAt.IsZero(), time.Since(r.spaceAt) > spaceAge && !r.spaceRun
	if stale {
		r.spaceRun = true
	}
	r.mu.Unlock()
	refresh := func(ctx context.Context) {
		vs, err := r.volumes(ctx)
		var s *Space
		if r.ok(ctx, "space", err) {
			s = spaceOf(vs)
		}
		r.mu.Lock()
		r.space, r.spaceAt, r.spaceRun = s, time.Now(), false
		r.mu.Unlock()
	}
	switch {
	case first:
		refresh(ctx)
	case stale:
		go refresh(context.WithoutCancel(ctx))
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.space
}

// spaceOf adds up the space of the computer's own disks, those of known
// size; shares and cloud drives are other computers'. Volumes of one
// container, as APFS's, tell its size and free space each, so volumes
// telling the same are counted once.
func spaceOf(vs []storage.Volume) *Space {
	var s *Space
	seen := map[[2]uint64]bool{}
	for _, v := range vs {
		if v.Kind != storage.KindLocalSSD && v.Kind != storage.KindLocalHDD || v.Total == 0 {
			continue
		}
		key := [2]uint64{v.Total, v.Available}
		if seen[key] {
			continue
		}
		seen[key] = true
		if s == nil {
			s = &Space{}
		}
		s.Total += v.Total
		s.Available += v.Available
	}
	return s
}

// ok tells whether err is nil, logging the first error of each part.
func (r *reader) ok(ctx context.Context, part string, err error) bool {
	if err == nil {
		return true
	}
	if _, logged := r.failed.LoadOrStore(part, true); !logged {
		slog.DebugContext(ctx, "load: cannot read", "part", part, "err", err)
	}
	return false
}

// counted are the interfaces whose traffic is counted: those up, neither
// the loopback nor a tunnel, whose traffic other interfaces carry too, and
// on Linux not virtual, as bridges and their containers' ends are, unless
// all are, as in a container.
func counted(ns []net.IOCountersStat) []net.IOCountersStat {
	ifs, err := stdnet.Interfaces()
	if err != nil {
		return nil
	}
	flags := make(map[string]stdnet.Flags, len(ifs))
	for _, i := range ifs {
		flags[i.Name] = i.Flags
	}
	var real, all []net.IOCountersStat
	for _, n := range ns {
		f, ok := flags[n.Name]
		if !ok || f&stdnet.FlagUp == 0 || f&(stdnet.FlagLoopback|stdnet.FlagPointToPoint) != 0 {
			continue
		}
		all = append(all, n)
		if !virtualInterface(n.Name) {
			real = append(real, n)
		}
	}
	if len(real) > 0 {
		return real
	}
	return all
}
