package storage

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"
)

func TestVolumeLedger(t *testing.T) {
	l := NewVolumeLedger()
	ctx := t.Context()
	hold, err := l.Acquire(ctx, "dev:1")
	if err != nil {
		t.Fatal(err)
	}
	// Another volume and the empty one are not held up.
	other, err := l.Acquire(ctx, "dev:2")
	if err != nil {
		t.Fatal(err)
	}
	other()
	none, _ := l.Acquire(ctx, "")
	none()

	// Readers of the held volume wait, in order, one at a time.
	var mu sync.Mutex
	var order []int
	var wg sync.WaitGroup
	for i := range 3 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			release, err := l.Acquire(ctx, "dev:1")
			if err != nil {
				t.Error(err)
				return
			}
			mu.Lock()
			order = append(order, i)
			mu.Unlock()
			release()
		}()
		time.Sleep(20 * time.Millisecond) // queue them in order
	}
	mu.Lock()
	if len(order) != 0 {
		t.Fatalf("readers ran while the volume was held: %v", order)
	}
	mu.Unlock()
	hold()
	hold() // releasing twice is harmless
	wg.Wait()
	if !slices.Equal(order, []int{0, 1, 2}) {
		t.Errorf("order = %v, want [0 1 2]", order)
	}

	// A canceled wait gives up and leaves the lane free.
	hold, _ = l.Acquire(ctx, "dev:1")
	cctx, cancel := context.WithTimeout(ctx, 20*time.Millisecond)
	defer cancel()
	if _, err := l.Acquire(cctx, "dev:1"); err == nil {
		t.Error("Acquire after cancellation = nil error, want one")
	}
	hold()
	l.mu.Lock()
	n := len(l.lanes)
	l.mu.Unlock()
	if n != 0 {
		t.Errorf("lanes = %d, want 0", n)
	}
}
