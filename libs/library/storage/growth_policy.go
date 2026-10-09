package storage

import (
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// GrowthMode represents the lifecycle mode of a file being read.
type GrowthMode uint8

const (
	// ModeStatic represents a completed, fixed-size file.
	ModeStatic GrowthMode = 0

	// ModeProbing represents an ambiguous file that had a recent timestamp and is being monitored for growth.
	ModeProbing GrowthMode = 1

	// ModeGrowing represents an actively writing file (e.g. download or live recording).
	ModeGrowing GrowthMode = 2

	// ModeFinal represents a formerly growing file that has completed or permanently stalled.
	ModeFinal GrowthMode = 3
)

// GrowthAction represents the demuxer action upon reaching the current end of file.
type GrowthAction uint8

const (
	// ActionEndOfFile indicates genuine EOF; terminate playback or reading.
	ActionEndOfFile GrowthAction = 0

	// ActionRetry indicates new bytes are already available; retry reading immediately.
	ActionRetry GrowthAction = 1

	// ActionWait indicates the file is still growing; pause and wait for subsequent writes.
	ActionWait GrowthAction = 2
)

// Growth constants matching Khua's SPSourceGrowthPolicy.
const (
	RecentMtimeNs        int64 = 5 * 1000 * 1000 * 1000       // 5 seconds
	ProbeGraceUs         int64 = 3 * 1000 * 1000              // 3 seconds
	IdleFinalUs          int64 = 30 * 1000 * 1000             // 30 seconds
	StalledHintUs        int64 = 10 * 1000 * 1000             // 10 seconds
	IdleFinalAfterHintUs int64 = 1 * 1000 * 1000              // 1 second
	PollUs               int64 = 100 * 1000                   // 100 ms
	WatchRecentNs        int64 = 10 * 60 * 1000 * 1000 * 1000 // 10 minutes
)

// FileStamp holds size and nanosecond modification time.
type FileStamp struct {
	Size    int64
	MtimeNs int64
}

// GrowthInputs encapsulates the current state of a reading stream and its backing file.
type GrowthInputs struct {
	Mode            GrowthMode
	AtOpen          FileStamp
	Now             FileStamp
	ReadPos         int64
	DownloadHint    bool
	HadDownloadHint bool
	WallNowNs       int64
	MonoNowUs       int64
	LastGrowthUs    int64
	ProbeStartUs    int64
	WriterOpen      int // -1 = unknown, 0 = closed, 1 = open writer detected
	PendingZero     bool
	InPlaceFill     bool
	OpenWallNs      int64
}

// GrowthDecision describes the updated mode and action for the reader.
type GrowthDecision struct {
	Mode   GrowthMode
	Action GrowthAction
}

func changedSinceOpen(in GrowthInputs) bool {
	return in.Now.Size >= 0 && (in.Now.Size != in.AtOpen.Size || in.Now.MtimeNs != in.AtOpen.MtimeNs)
}

// DecideGrowth calculates the next mode and action according to Khua's SPSourceGrowthPolicy.
func DecideGrowth(in GrowthInputs) GrowthDecision {
	d := GrowthDecision{Mode: in.Mode, Action: ActionEndOfFile}
	if in.Now.Size < 0 {
		return d
	}

	bytesAhead := !in.PendingZero && in.Now.Size > in.ReadPos
	inPlaceEnd := !in.PendingZero && in.InPlaceFill && in.Now.Size == in.AtOpen.Size && !bytesAhead

	switch in.Mode {
	case ModeStatic:
		if changedSinceOpen(in) {
			d.Mode = ModeGrowing
			if bytesAhead {
				d.Action = ActionRetry
			} else if inPlaceEnd {
				d.Action = ActionEndOfFile
			} else {
				d.Action = ActionWait
			}
			return d
		}

		recent := in.WallNowNs-in.Now.MtimeNs < RecentMtimeNs
		if inPlaceEnd {
			return d
		}

		pausedAtOpen := in.OpenWallNs > 0 && in.OpenWallNs-in.AtOpen.MtimeNs < RecentMtimeNs &&
			in.WriterOpen == 1 && in.MonoNowUs-in.LastGrowthUs < IdleFinalUs

		if (recent && in.WriterOpen != 0) || (in.PendingZero && in.WriterOpen == 1) || pausedAtOpen {
			if in.WriterOpen == 1 || in.DownloadHint {
				d.Mode = ModeGrowing
			} else {
				d.Mode = ModeProbing
			}
			d.Action = ActionWait
		}
		return d

	case ModeProbing:
		if changedSinceOpen(in) {
			d.Mode = ModeGrowing
			if bytesAhead {
				d.Action = ActionRetry
			} else if inPlaceEnd {
				d.Action = ActionEndOfFile
			} else {
				d.Action = ActionWait
			}
			return d
		}
		if in.MonoNowUs-in.ProbeStartUs >= ProbeGraceUs {
			d.Mode = ModeStatic
			return d
		}
		d.Action = ActionWait
		return d

	case ModeGrowing, ModeFinal:
		if bytesAhead {
			d.Mode = ModeGrowing
			d.Action = ActionRetry
			return d
		}
		if in.Mode == ModeFinal {
			return d
		}
		if inPlaceEnd {
			d.Mode = ModeGrowing
			return d
		}
		if in.WriterOpen == 1 || (in.DownloadHint && in.WriterOpen == -1) {
			d.Action = ActionWait
			return d
		}

		idle := in.MonoNowUs - in.LastGrowthUs
		limit := IdleFinalUs
		if in.HadDownloadHint || in.WriterOpen == 0 {
			limit = IdleFinalAfterHintUs
		}

		if idle >= limit {
			d.Mode = ModeFinal
			return d
		}
		d.Action = ActionWait
		return d
	}

	return d
}

var downloadExtensions = []string{
	".part",
	".crdownload",
	".download",
	".aria2",
	".tmp",
	".incomplete",
	".!ut",
}

// HasDownloadSuffix reports whether path ends in a known temporary download extension.
func HasDownloadSuffix(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	for _, suffix := range downloadExtensions {
		if ext == suffix {
			return true
		}
	}
	return false
}

// GrowthPolicy tracks actively growing or recently modified files to prevent premature indexing.
type GrowthPolicy struct {
	mu        sync.Mutex
	lastSeen  map[string]FileStamp
	threshold time.Duration
}

// NewGrowthPolicy creates a GrowthPolicy with a stability threshold.
func NewGrowthPolicy(threshold time.Duration) *GrowthPolicy {
	if threshold <= 0 {
		threshold = 10 * time.Second
	}
	return &GrowthPolicy{
		lastSeen:  make(map[string]FileStamp),
		threshold: threshold,
	}
}

// IsGrowing reports whether the file at path has a temporary download suffix,
// or has changed size/mtime across consecutive observations.
func (p *GrowthPolicy) IsGrowing(path string, size int64, mtime time.Time) bool {
	if HasDownloadSuffix(path) {
		return true
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	prev, ok := p.lastSeen[path]
	p.lastSeen[path] = FileStamp{Size: size, MtimeNs: mtime.UnixNano()}
	if ok && (prev.Size != size || prev.MtimeNs != mtime.UnixNano()) {
		return true
	}
	return false
}
