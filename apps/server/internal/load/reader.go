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
)

// reader reads this computer's counters, logging each part it cannot read
// once.
type reader struct {
	self   *process.Process
	failed sync.Map
}

func newReader() *reader {
	r := &reader{}
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
