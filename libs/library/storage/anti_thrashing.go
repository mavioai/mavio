package storage

import (
	"context"
	"sync"
	"time"
)

// Candidate represents a pending background I/O request queued for a volume.
type Candidate struct {
	Path string
	Seq  uint64
}

// VolumeLedger manages anti-thrashing serialization lanes for physical storage volumes.
// It directly models Khua's RecentWarmVolumeLedger:
//  1. Single-flight worker per path: avoids duplicate concurrent stat/open calls.
//  2. Single-flight read slot per mechanical or remote volume: prevents concurrent random head thrashing.
//  3. Intent sequence tracking: yields to newer user intents and preserves cooldowns.
type VolumeLedger struct {
	mu sync.Mutex

	inFlightPaths       map[string]struct{}
	volumeWarmedAt      map[string]time.Time
	volumeReadSeqActive map[string]uint64
	pendingByVolume     map[string]Candidate
	volumeKeyByPath     map[string]string
	intentSeqByPath     map[string]uint64
	intentCounter       uint64

	volumeCooldown time.Duration
	entryCooldown  time.Duration
}

// NewVolumeLedger creates a new VolumeLedger with the specified cooldowns.
func NewVolumeLedger(volumeCooldown, entryCooldown time.Duration) *VolumeLedger {
	return &VolumeLedger{
		inFlightPaths:       make(map[string]struct{}),
		volumeWarmedAt:      make(map[string]time.Time),
		volumeReadSeqActive: make(map[string]uint64),
		pendingByVolume:     make(map[string]Candidate),
		volumeKeyByPath:     make(map[string]string),
		intentSeqByPath:     make(map[string]uint64),
		volumeCooldown:      volumeCooldown,
		entryCooldown:       entryCooldown,
	}
}

// ClaimPath attempts to claim an exclusive worker slot for the specified file path.
// Returns false if a worker is already in flight for this path.
func (l *VolumeLedger) ClaimPath(path string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, ok := l.inFlightPaths[path]; ok {
		return false
	}
	l.inFlightPaths[path] = struct{}{}
	return true
}

// FinishPath releases the worker slot for the specified file path.
func (l *VolumeLedger) FinishPath(path string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.inFlightPaths, path)
}

// NoteIntent records a new user or scan intent for path, incrementing its sequence.
func (l *VolumeLedger) NoteIntent(path string) uint64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.intentCounter++
	l.intentSeqByPath[path] = l.intentCounter
	return l.intentCounter
}

// IntentSeq returns the latest intent sequence for path.
func (l *VolumeLedger) IntentSeq(path string) uint64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.intentSeqByPath[path]
}

// NoteVolumeKey associates a path with its resolved volume key.
func (l *VolumeLedger) NoteVolumeKey(volumeKey, path string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.volumeKeyByPath[path] = volumeKey
}

// VolumeKey returns the cached volume key for path.
func (l *VolumeLedger) VolumeKey(path string) (string, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	k, ok := l.volumeKeyByPath[path]
	return k, ok
}

// ClaimVolumeWake attempts to claim a wake/spin-up read for volumeKey within cooldown.
func (l *VolumeLedger) ClaimVolumeWake(volumeKey string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if last, ok := l.volumeWarmedAt[volumeKey]; ok {
		if now.Sub(last) < l.volumeCooldown {
			return false
		}
	}
	l.volumeWarmedAt[volumeKey] = now
	return true
}

// RevokeVolumeWake revokes a wake claim if the operation yielded before reading.
func (l *VolumeLedger) RevokeVolumeWake(volumeKey string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.volumeWarmedAt, volumeKey)
}

// IsVolumeReadInFlight reports whether an active read slot is held for volumeKey.
func (l *VolumeLedger) IsVolumeReadInFlight(volumeKey string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	_, inFlight := l.volumeReadSeqActive[volumeKey]
	return inFlight
}

// ClaimVolumeRead attempts to claim the exclusive read slot for volumeKey with sequence seq.
func (l *VolumeLedger) ClaimVolumeRead(volumeKey string, seq uint64) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, busy := l.volumeReadSeqActive[volumeKey]; busy {
		return false
	}
	l.volumeReadSeqActive[volumeKey] = seq
	return true
}

// NotePendingEntry records path as the next pending candidate for a busy volumeKey.
// Retains only the newest candidate for that volume.
func (l *VolumeLedger) NotePendingEntry(volumeKey, path string, seq uint64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if running, ok := l.volumeReadSeqActive[volumeKey]; ok && running >= seq {
		return
	}
	if current, ok := l.pendingByVolume[volumeKey]; ok && current.Seq >= seq {
		return
	}
	l.pendingByVolume[volumeKey] = Candidate{Path: path, Seq: seq}
}

// HasNewerCandidate reports whether a newer candidate is waiting on volumeKey other than path.
func (l *VolumeLedger) HasNewerCandidate(volumeKey, path string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	pending, ok := l.pendingByVolume[volumeKey]
	if !ok || pending.Path == path {
		return false
	}
	activeSeq := l.volumeReadSeqActive[volumeKey]
	return pending.Seq > activeSeq
}

// ReleaseVolumeRead releases the active read slot on volumeKey, returning the next candidate if any.
func (l *VolumeLedger) ReleaseVolumeRead(volumeKey string) (Candidate, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	activeSeq, hadActive := l.volumeReadSeqActive[volumeKey]
	delete(l.volumeReadSeqActive, volumeKey)

	pending, hasPending := l.pendingByVolume[volumeKey]
	if !hadActive || !hasPending {
		return Candidate{}, false
	}
	if pending.Seq <= activeSeq {
		delete(l.pendingByVolume, volumeKey)
		return Candidate{}, false
	}
	delete(l.pendingByVolume, volumeKey)
	return pending, true
}

// Acquire blocks until an exclusive read slot is claimed for volumeKey or ctx is cancelled.
// It returns a release function that frees the claimed slot.
func (l *VolumeLedger) Acquire(ctx context.Context, volumeKey string) (func(), error) {
	if volumeKey == "" {
		return func() {}, nil
	}
	seq := l.NoteIntent(volumeKey)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		if l.ClaimVolumeRead(volumeKey, seq) {
			var once sync.Once
			return func() {
				once.Do(func() {
					l.ReleaseVolumeRead(volumeKey)
				})
			}, nil
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}
