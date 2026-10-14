package auth

import (
	"sync"
	"time"
)

// Throttle limits.
const (
	// FreeFailures is how many failures a key may make before it waits.
	FreeFailures = 3
	// MaxLockout bounds the wait after many failures.
	MaxLockout = 15 * time.Minute
	// FailureMemory is how long after its last failure a key is forgotten.
	FailureMemory = time.Hour
)

// Throttle slows down failed attempts progressively, per key such as a
// user name or an address: after FreeFailures, each failure locks the key
// out twice as long as the one before, from a second up to MaxLockout. A
// key is forgotten FailureMemory after its last failure. It never
// disables anything.
type Throttle struct {
	now func() time.Time

	mu   sync.Mutex
	keys map[string]*failures
}

type failures struct {
	n     int
	last  time.Time
	until time.Time
}

// NewThrottle returns an empty throttle.
func NewThrottle() *Throttle {
	return &Throttle{now: time.Now, keys: map[string]*failures{}}
}

// Wait returns how long the most locked out of keys still waits; zero
// when none is.
func (t *Throttle) Wait(keys ...string) time.Duration {
	now := t.now()
	t.mu.Lock()
	defer t.mu.Unlock()
	var wait time.Duration
	for _, k := range keys {
		if f, ok := t.keys[k]; ok {
			wait = max(wait, f.until.Sub(now))
		}
	}
	return wait
}

// Fail records a failure of each key.
func (t *Throttle) Fail(keys ...string) {
	now := t.now()
	t.mu.Lock()
	defer t.mu.Unlock()
	for k, f := range t.keys {
		if now.Sub(f.last) >= FailureMemory {
			delete(t.keys, k)
		}
	}
	for _, k := range keys {
		f, ok := t.keys[k]
		if !ok {
			f = &failures{}
			t.keys[k] = f
		}
		f.n++
		f.last = now
		if over := f.n - FreeFailures; over > 0 {
			lock := MaxLockout
			if over <= 20 {
				lock = min(time.Second<<(over-1), MaxLockout)
			}
			f.until = now.Add(lock)
		}
	}
}

// Succeed forgets the failures of keys.
func (t *Throttle) Succeed(keys ...string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, k := range keys {
		delete(t.keys, k)
	}
}
