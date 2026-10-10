package storage

import (
	"context"
	"sync"
	"time"
)

// DefaultQuietPeriod is how long background I/O waits after foreground
// activity.
const DefaultQuietPeriod = 3 * time.Second

// QuietGate makes background I/O yield to foreground streaming, as
// khuaplayer's foreground storage gate does: foreground activity extends
// a quiet deadline, never shortening it, so that a burst of requests
// makes one quiet window without a timer per request, and background
// tasks wait the window out before reading.
type QuietGate struct {
	period time.Duration

	mu         sync.Mutex
	quietUntil time.Time
}

// NewQuietGate returns a gate whose activity quiets background I/O for
// period, DefaultQuietPeriod when not positive.
func NewQuietGate(period time.Duration) *QuietGate {
	if period <= 0 {
		period = DefaultQuietPeriod
	}
	return &QuietGate{period: period}
}

// NoteActivity notes foreground activity: background I/O waits until a
// period from now at least.
func (g *QuietGate) NoteActivity() {
	g.mu.Lock()
	if until := time.Now().Add(g.period); until.After(g.quietUntil) {
		g.quietUntil = until
	}
	g.mu.Unlock()
}

// Quiet reports whether background I/O should wait.
func (g *QuietGate) Quiet() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return time.Now().Before(g.quietUntil)
}

// PauseOrCancel waits until the quiet window, extended by any activity
// meanwhile, has passed, or ctx is canceled.
func (g *QuietGate) PauseOrCancel(ctx context.Context) error {
	for {
		g.mu.Lock()
		wait := time.Until(g.quietUntil)
		g.mu.Unlock()
		if wait <= 0 {
			return nil
		}
		t := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			t.Stop()
			return context.Cause(ctx)
		case <-t.C:
		}
	}
}
