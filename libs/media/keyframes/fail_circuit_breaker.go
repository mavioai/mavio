package keyframes

import (
	"sync"
	"time"
)

// DefaultFailNoteTTL is the default time-to-live for a recorded extraction failure.
const DefaultFailNoteTTL = 10 * time.Second

// DefaultFailNoteCapacity is the default maximum number of failure records retained per breaker.
const DefaultFailNoteCapacity = 16

// DefaultFailNoteHalfSpan is the default half-span around a target timestamp when GOP duration is unknown.
const DefaultFailNoteHalfSpan = 2500 * time.Millisecond

// FailNote records a time window where keyframe or thumbnail extraction failed.
type FailNote struct {
	From       time.Duration
	Until      time.Duration
	RecordedAt time.Time
}

// FailCircuitBreaker tracks failed thumbnail or keyframe extraction windows in-memory,
// suppressing repeat extractions in broken GOPs within a TTL window.
type FailCircuitBreaker struct {
	mu       sync.Mutex
	ttl      time.Duration
	capacity int
	notes    []FailNote
}

// NewFailCircuitBreaker constructs a FailCircuitBreaker with the given TTL and capacity limits.
// If ttl <= 0, DefaultFailNoteTTL is used. If capacity <= 0, DefaultFailNoteCapacity is used.
func NewFailCircuitBreaker(ttl time.Duration, capacity int) *FailCircuitBreaker {
	if ttl <= 0 {
		ttl = DefaultFailNoteTTL
	}
	if capacity <= 0 {
		capacity = DefaultFailNoteCapacity
	}
	return &FailCircuitBreaker{
		ttl:      ttl,
		capacity: capacity,
	}
}

// IsSuppressed reports whether extraction at target is currently suppressed because of
// a recent failure recorded within target's GOP window.
func (cb *FailCircuitBreaker) IsSuppressed(target time.Duration, now time.Time) bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	for _, n := range cb.notes {
		if now.Sub(n.RecordedAt) > cb.ttl {
			continue
		}
		if target >= n.From && target <= n.Until {
			return true
		}
	}
	return false
}

// RecordFailure records an extraction failure at target for a stream with GOP length gopSpan.
// If gopSpan <= 0, DefaultFailNoteHalfSpan (2.5s) is used as the half-window.
func (cb *FailCircuitBreaker) RecordFailure(target, gopSpan time.Duration, now time.Time) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	half := gopSpan
	if half <= 0 {
		half = DefaultFailNoteHalfSpan
	}

	from := target - half
	if from < 0 {
		from = 0
	}
	until := target + half

	if len(cb.notes) >= cb.capacity {
		cb.notes = cb.notes[1:]
	}
	cb.notes = append(cb.notes, FailNote{
		From:       from,
		Until:      until,
		RecordedAt: now,
	})
}

// Clear clears all recorded failure notes.
func (cb *FailCircuitBreaker) Clear() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.notes = nil
}

// KeyedFailCircuitBreaker tracks failures partitioned by arbitrary string keys
// (e.g. media source path or item ID).
type KeyedFailCircuitBreaker struct {
	mu       sync.Mutex
	ttl      time.Duration
	capacity int
	breakers map[string]*FailCircuitBreaker
}

// NewKeyedFailCircuitBreaker constructs a KeyedFailCircuitBreaker.
func NewKeyedFailCircuitBreaker(ttl time.Duration, capacity int) *KeyedFailCircuitBreaker {
	if ttl <= 0 {
		ttl = DefaultFailNoteTTL
	}
	if capacity <= 0 {
		capacity = DefaultFailNoteCapacity
	}
	return &KeyedFailCircuitBreaker{
		ttl:      ttl,
		capacity: capacity,
		breakers: make(map[string]*FailCircuitBreaker),
	}
}

// IsSuppressed reports whether extraction at target for key is currently suppressed.
func (k *KeyedFailCircuitBreaker) IsSuppressed(key string, target time.Duration, now time.Time) bool {
	k.mu.Lock()
	cb, ok := k.breakers[key]
	k.mu.Unlock()
	if !ok {
		return false
	}
	return cb.IsSuppressed(target, now)
}

// RecordFailure records an extraction failure at target for key.
func (k *KeyedFailCircuitBreaker) RecordFailure(key string, target, gopSpan time.Duration, now time.Time) {
	k.mu.Lock()
	cb, ok := k.breakers[key]
	if !ok {
		cb = NewFailCircuitBreaker(k.ttl, k.capacity)
		k.breakers[key] = cb
	}
	k.mu.Unlock()
	cb.RecordFailure(target, gopSpan, now)
}

// Clear removes all tracked records for key.
func (k *KeyedFailCircuitBreaker) Clear(key string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	delete(k.breakers, key)
}
