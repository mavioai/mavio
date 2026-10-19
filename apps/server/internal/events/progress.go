package events

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
	sessionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/session/v1"
)

// progressInterval is how often a library's scan progress is sent at most
// while it runs.
const progressInterval = 500 * time.Millisecond

// foldersShare is the share of a scan's progress that reading its folders
// takes; the probes and refreshes it queued take the rest.
const foldersShare = 0.25

// scans follows the libraries' scans and the probes and refreshes they
// queued, in memory: a scan begins when its job is leased, reads its
// folders, and ends once its job and the jobs it queued have ended.
type scans struct {
	mu   sync.Mutex
	libs map[core.ID]*scanProgress
	// folders are the folders each library's last scan read, which the
	// next one expects.
	folders map[core.ID]int
}

// scanProgress is the progress of one library's scan.
type scanProgress struct {
	// reading is set while the scan job runs.
	reading            bool
	read, found, total int
	done, queued       int
	// jobs are the probes and refreshes queued and not yet ended.
	jobs map[core.ID]bool
	// sent is when the progress was last sent; timer sends it later when
	// it changed sooner.
	sent  time.Time
	timer *time.Timer
}

func (p *scanProgress) proto(lib core.ID) *libraryv1.ScanProgress {
	total := max(p.total, p.found)
	phase := libraryv1.ScanPhase_SCAN_PHASE_ITEMS
	fraction := foldersShare
	if p.reading {
		phase = libraryv1.ScanPhase_SCAN_PHASE_FOLDERS
		fraction = 0
		if total > 0 {
			fraction = foldersShare * float64(p.read) / float64(total)
		}
	} else if p.queued > 0 {
		fraction += (1 - foldersShare) * float64(p.done) / float64(p.queued)
	}
	return libraryv1.ScanProgress_builder{
		LibraryId: new(lib.String()), Phase: &phase,
		FoldersDone: new(int32(p.read)), FoldersTotal: new(int32(total)),
		ItemsDone: new(int32(p.done)), ItemsTotal: new(int32(p.queued)),
		Fraction: &fraction,
	}.Build()
}

// ScanProgress returns how far a library's scan has come, nil while it is
// not being scanned.
func (h *Hub) ScanProgress(lib core.ID) *libraryv1.ScanProgress {
	h.scans.mu.Lock()
	defer h.scans.mu.Unlock()
	if p := h.scans.libs[lib]; p != nil {
		return p.proto(lib)
	}
	return nil
}

// ScanFolders records the folders a library's scan has read and found;
// Scanner.OnFolders is it.
func (h *Hub) ScanFolders(lib core.ID, read, found int) {
	h.scanChanged(lib, func(p *scanProgress) { p.read, p.found = read, found })
}

// scanLeased begins the progress of a library's scan, or, while the work
// an earlier scan queued goes on, reads its folders anew.
func (h *Hub) scanLeased(job core.Job) {
	lib := libraryOfJob(job)
	if lib.IsZero() {
		return
	}
	h.scans.mu.Lock()
	if h.scans.libs[lib] == nil {
		h.scans.libs[lib] = &scanProgress{jobs: map[core.ID]bool{}}
	}
	p := h.scans.libs[lib]
	p.reading, p.read, p.found, p.total = true, 0, 0, h.scans.folders[lib]
	h.scans.mu.Unlock()
	h.sendProgress(lib, true)
}

// scanEnded records that a library's scan job ended: its progress goes on
// with the jobs it queued, or ends.
func (h *Hub) scanEnded(job core.Job) {
	lib := libraryOfJob(job)
	h.scans.mu.Lock()
	p := h.scans.libs[lib]
	if p != nil {
		p.reading = false
		h.scans.folders[lib] = p.read
	}
	h.scans.mu.Unlock()
	if p != nil {
		h.settle(lib)
	}
}

// jobEnqueued counts a probe or refresh a scan queued.
func (h *Hub) jobEnqueued(job core.Job) {
	if job.Kind != library.JobProbe && job.Kind != library.JobRefresh {
		return
	}
	var payload struct {
		LibraryID core.ID `json:"library_id"`
	}
	if json.Unmarshal(job.Payload, &payload) != nil || payload.LibraryID.IsZero() {
		return
	}
	h.scanChanged(payload.LibraryID, func(p *scanProgress) {
		p.jobs[job.ID] = true
		p.queued++
	})
}

// jobDone counts a probe or refresh that ended; one that failed counts as
// done, although it may be retried, so that no scan waits for retries.
func (h *Hub) jobDone(id core.ID) {
	h.scans.mu.Lock()
	var lib core.ID
	for l, p := range h.scans.libs {
		if p.jobs[id] {
			delete(p.jobs, id)
			p.done++
			lib = l
			break
		}
	}
	h.scans.mu.Unlock()
	if !lib.IsZero() {
		h.settle(lib)
	}
}

// settle ends a library's progress when its scan job and every job it
// queued ended, and otherwise sends it.
func (h *Hub) settle(lib core.ID) {
	h.scans.mu.Lock()
	p := h.scans.libs[lib]
	ended := p != nil && !p.reading && len(p.jobs) == 0
	if ended {
		if p.timer != nil {
			p.timer.Stop()
		}
		delete(h.scans.libs, lib)
	}
	h.scans.mu.Unlock()
	if ended {
		h.ToAdmins(sessionv1.Event_builder{
			ScanProgressChanged: sessionv1.ScanProgressChanged_builder{LibraryId: new(lib.String())}.Build(),
		}.Build())
		return
	}
	h.sendProgress(lib, false)
}

// scanChanged changes a library's progress while it is being scanned and
// sends it.
func (h *Hub) scanChanged(lib core.ID, change func(p *scanProgress)) {
	h.scans.mu.Lock()
	p := h.scans.libs[lib]
	if p != nil {
		change(p)
	}
	h.scans.mu.Unlock()
	if p != nil {
		h.sendProgress(lib, false)
	}
}

// sendProgress sends a library's progress to administrators: now when
// asked or when it was last sent long enough ago, else once the interval
// has passed.
func (h *Hub) sendProgress(lib core.ID, now bool) {
	h.scans.mu.Lock()
	p := h.scans.libs[lib]
	if p == nil {
		h.scans.mu.Unlock()
		return
	}
	wait := progressInterval - time.Since(p.sent)
	if !now && wait > 0 {
		if p.timer == nil {
			p.timer = time.AfterFunc(wait, func() {
				h.scans.mu.Lock()
				if p := h.scans.libs[lib]; p != nil {
					p.timer = nil
				}
				h.scans.mu.Unlock()
				h.sendProgress(lib, true)
			})
		}
		h.scans.mu.Unlock()
		return
	}
	if p.timer != nil {
		p.timer.Stop()
		p.timer = nil
	}
	p.sent = time.Now()
	progress := p.proto(lib)
	h.scans.mu.Unlock()
	h.ToAdmins(sessionv1.Event_builder{
		ScanProgressChanged: sessionv1.ScanProgressChanged_builder{LibraryId: new(lib.String()), Progress: progress}.Build(),
	}.Build())
}
