package storage

import (
	"path"
	"slices"
	"strings"
	"sync"
	"time"
)

// Defaults of a GrowthPolicy.
const (
	// DefaultWriteWindow is how recently a file may have been modified
	// for it to count as still being written.
	DefaultWriteWindow = 10 * time.Second
	// DefaultStableFor is how long a deferred file's size and
	// modification time must stay the same before it is scanned.
	DefaultStableFor = 30 * time.Second
	// deferredCap bounds the files a GrowthPolicy defers; deferredTTL is
	// how long a file that never settles, such as a paused download, is
	// kept.
	deferredCap = 4096
	deferredTTL = 24 * time.Hour
)

// downloadExtensions are the suffixes download clients give the files they
// are still writing.
var downloadExtensions = []string{
	".part", ".crdownload", ".download", ".aria2", ".tmp", ".incomplete", ".!ut", ".!qb",
}

// HasDownloadSuffix reports whether name ends in a temporary download
// extension.
func HasDownloadSuffix(name string) bool {
	return slices.Contains(downloadExtensions, strings.ToLower(path.Ext(strings.ReplaceAll(name, `\`, "/"))))
}

// FileStamp is a file's size and modification time.
type FileStamp struct {
	Size    int64
	ModTime time.Time
}

// FileStat returns the stamp of the file at path; ok is false when it is
// gone.
type FileStat func(path string) (stamp FileStamp, ok bool)

// GrowthPolicy keeps files that are still being written, such as
// downloads, out of scans and defers them: a file whose name has a
// download suffix, that was modified within the write window, or that
// changed since it was deferred is growing. Reconcile reports the owners
// (libraries) of deferred files that settled or went away, so that they
// are scanned again.
type GrowthPolicy struct {
	// WriteWindow and StableFor default to DefaultWriteWindow and
	// DefaultStableFor.
	WriteWindow, StableFor time.Duration
	// Now returns the current time; default time.Now.
	Now func() time.Time

	mu       sync.Mutex
	deferred map[string]*deferral
}

// deferral is a deferred file.
type deferral struct {
	owner string
	stamp FileStamp
	// since is when the stamp last changed, added when it was deferred.
	since, added time.Time
}

// NewGrowthPolicy returns a policy with the default windows.
func NewGrowthPolicy() *GrowthPolicy {
	return &GrowthPolicy{}
}

func (p *GrowthPolicy) now() time.Time {
	if p.Now != nil {
		return p.Now()
	}
	return time.Now()
}

func (p *GrowthPolicy) writeWindow() time.Duration {
	if p.WriteWindow > 0 {
		return p.WriteWindow
	}
	return DefaultWriteWindow
}

func (p *GrowthPolicy) stableFor() time.Duration {
	if p.StableFor > 0 {
		return p.StableFor
	}
	return DefaultStableFor
}

func normalize(name string) string { return path.Clean(strings.ReplaceAll(name, `\`, "/")) }

// Growing reports whether the file at name, with stamp, is still being
// written, deferring it for owner when it is.
func (p *GrowthPolicy) Growing(owner, name string, stamp FileStamp) bool {
	now := p.now()
	key := normalize(name)
	p.mu.Lock()
	defer p.mu.Unlock()
	d := p.deferred[key]
	if d != nil && d.stamp != stamp {
		d.stamp, d.since = stamp, now
	}
	growing := HasDownloadSuffix(name) || now.Sub(stamp.ModTime) < p.writeWindow() ||
		(d != nil && now.Sub(d.since) < p.stableFor())
	switch {
	case !growing:
		delete(p.deferred, key)
	case d == nil:
		if p.deferred == nil {
			p.deferred = map[string]*deferral{}
		}
		if len(p.deferred) >= deferredCap {
			p.evictOldest()
		}
		p.deferred[key] = &deferral{owner: owner, stamp: stamp, since: now, added: now}
	default:
		d.owner = owner
	}
	return growing
}

// evictOldest forgets the file deferred first.
func (p *GrowthPolicy) evictOldest() {
	var oldest string
	var at time.Time
	for k, d := range p.deferred {
		if oldest == "" || d.added.Before(at) {
			oldest, at = k, d.added
		}
	}
	delete(p.deferred, oldest)
}

// Pending returns how many files are deferred.
func (p *GrowthPolicy) Pending() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.deferred)
}

// Reconcile checks the deferred files with stat and returns, once each,
// the owners of those that went away (renamed when their download
// finished, or removed) or whose stamp stayed the same for StableFor
// after the write window. A file with a download suffix settles only by
// going away; one that never does is forgotten after a day.
func (p *GrowthPolicy) Reconcile(stat FileStat) []string {
	p.mu.Lock()
	names := make([]string, 0, len(p.deferred))
	for k := range p.deferred {
		names = append(names, k)
	}
	p.mu.Unlock()
	// Stat without the lock: remote volumes may be slow to answer.
	stamps := make(map[string]FileStamp, len(names))
	gone := map[string]bool{}
	for _, n := range names {
		if s, ok := stat(n); ok {
			stamps[n] = s
		} else {
			gone[n] = true
		}
	}
	now := p.now()
	p.mu.Lock()
	defer p.mu.Unlock()
	var owners []string
	for _, n := range names {
		d := p.deferred[n]
		if d == nil {
			continue
		}
		settled := gone[n]
		if !settled {
			if s := stamps[n]; s != d.stamp {
				d.stamp, d.since = s, now
			} else {
				settled = !HasDownloadSuffix(n) && now.Sub(d.since) >= p.stableFor() &&
					now.Sub(s.ModTime) >= p.writeWindow()
			}
		}
		switch {
		case settled:
			// The scan that follows defers the file anew should it grow
			// again.
			delete(p.deferred, n)
			if !slices.Contains(owners, d.owner) {
				owners = append(owners, d.owner)
			}
		case now.Sub(d.added) >= deferredTTL:
			delete(p.deferred, n)
		}
	}
	slices.Sort(owners)
	return owners
}
