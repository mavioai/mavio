package storage

import (
	"context"
	"sync"
	"time"
)

// DefaultQuietPeriod is the standard duration background tasks pause following foreground activity.
const DefaultQuietPeriod = 3 * time.Second

// QuietGate synchronizes foreground user operations (streaming, seeking) with background I/O tasks.
// Directly implements Khua's SPBackgroundStorageGate & SPForegroundStorageQuietState:
// When foreground activity occurs, the quiet deadline is extended (never shortened),
// coalescing bursts of user activity into a single quiet period without timer storms.
type QuietGate struct {
	mu             sync.Mutex
	quietUntil     time.Time
	activeSessions int
	period         time.Duration
}

// NewQuietGate creates a new initialized QuietGate.
func NewQuietGate(periods ...time.Duration) *QuietGate {
	p := DefaultQuietPeriod
	if len(periods) > 0 && periods[0] > 0 {
		p = periods[0]
	}
	return &QuietGate{period: p}
}

// NoteActivity registers foreground activity, extending the quiet deadline to at least now + period.
func (g *QuietGate) NoteActivity(period time.Duration) {
	if period <= 0 {
		period = g.period
		if period <= 0 {
			period = DefaultQuietPeriod
		}
	}
	g.mu.Lock()
	target := time.Now().Add(period)
	if target.After(g.quietUntil) {
		g.quietUntil = target
	}
	g.mu.Unlock()
}

// AcquirePlayback notes an active streaming session. While any playback session
// is active, background tasks will be blocked. When the returned release function
// is called, a cooldown is observed before background tasks resume.
func (g *QuietGate) AcquirePlayback(sessionID string) func() {
	g.mu.Lock()
	g.activeSessions++
	g.mu.Unlock()

	var once sync.Once
	return func() {
		once.Do(func() {
			g.mu.Lock()
			if g.activeSessions > 0 {
				g.activeSessions--
			}
			p := g.period
			if p <= 0 {
				p = DefaultQuietPeriod
			}
			target := time.Now().Add(p)
			if target.After(g.quietUntil) {
				g.quietUntil = target
			}
			g.mu.Unlock()
		})
	}
}

// IsActive reports whether the gate is currently active (i.e. background I/O should pause).
func (g *QuietGate) IsActive() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.activeSessions > 0 || time.Now().Before(g.quietUntil)
}

// QuietUntil returns the current quiet deadline timestamp.
func (g *QuietGate) QuietUntil() time.Time {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.quietUntil
}

// Wait blocks until the current quiet window and active streaming sessions have ended or ctx is cancelled.
// If no quiet window is active, it returns nil immediately.
func (g *QuietGate) Wait(ctx context.Context) error {
	for {
		g.mu.Lock()
		active := g.activeSessions > 0
		remaining := time.Until(g.quietUntil)
		g.mu.Unlock()

		if !active && remaining <= 0 {
			return nil
		}

		sleepDur := remaining
		if active || sleepDur <= 0 || sleepDur > 500*time.Millisecond {
			sleepDur = 500 * time.Millisecond
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(sleepDur):
			// Loop back to recheck active sessions and updated deadline.
		}
	}
}

// PauseOrCancel is an alias for Wait(ctx), checking if background tasks need to yield.
func (g *QuietGate) PauseOrCancel(ctx context.Context) error {
	return g.Wait(ctx)
}
