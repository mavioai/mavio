package storage

import (
	"context"
	"testing"
	"testing/synctest"
	"time"
)

func TestQuietGate(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		g := NewQuietGate(time.Second)
		if g.Quiet() {
			t.Error("Quiet before activity = true, want false")
		}
		start := time.Now()
		if err := g.PauseOrCancel(t.Context()); err != nil || time.Since(start) != 0 {
			t.Errorf("PauseOrCancel without activity waited %v: %v", time.Since(start), err)
		}

		// Activity during the window extends it.
		g.NoteActivity()
		go func() {
			time.Sleep(600 * time.Millisecond)
			g.NoteActivity()
		}()
		if !g.Quiet() {
			t.Error("Quiet after activity = false, want true")
		}
		if err := g.PauseOrCancel(t.Context()); err != nil {
			t.Fatal(err)
		}
		if got, want := time.Since(start), 1600*time.Millisecond; got != want {
			t.Errorf("waited %v, want %v", got, want)
		}

		// A canceled wait returns at once.
		g.NoteActivity()
		ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
		defer cancel()
		if err := g.PauseOrCancel(ctx); err == nil {
			t.Error("PauseOrCancel after cancellation = nil, want an error")
		}
	})
}
