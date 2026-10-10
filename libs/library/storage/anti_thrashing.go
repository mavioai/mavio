package storage

import (
	"context"
	"sync"
)

// VolumeLedger serializes background reads per volume: a hard disk reads
// for one reader at a time, so that its heads do not thrash between
// files, and a network volume shares its bandwidth no further. Readers of
// a volume wait in the order they came.
type VolumeLedger struct {
	mu    sync.Mutex
	lanes map[string]*lane
}

// lane is the read slot of a volume.
type lane struct {
	slot chan struct{}
	// users hold or wait for the slot; the lane is dropped without any.
	users int
}

// NewVolumeLedger returns an empty ledger.
func NewVolumeLedger() *VolumeLedger {
	return &VolumeLedger{lanes: map[string]*lane{}}
}

// Acquire waits until the read slot of volume is free, or ctx is
// canceled, and takes it; release frees it. An empty volume is not
// serialized.
func (l *VolumeLedger) Acquire(ctx context.Context, volume string) (release func(), err error) {
	if volume == "" {
		return func() {}, nil
	}
	l.mu.Lock()
	ln := l.lanes[volume]
	if ln == nil {
		ln = &lane{slot: make(chan struct{}, 1)}
		l.lanes[volume] = ln
	}
	ln.users++
	l.mu.Unlock()
	leave := func() {
		l.mu.Lock()
		if ln.users--; ln.users == 0 {
			delete(l.lanes, volume)
		}
		l.mu.Unlock()
	}
	select {
	case ln.slot <- struct{}{}:
	case <-ctx.Done():
		leave()
		return nil, context.Cause(ctx)
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			<-ln.slot
			leave()
		})
	}, nil
}
