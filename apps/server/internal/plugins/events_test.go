package plugins

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"strconv"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

// recorder consumes batches, failing the first fails calls.
type recorder struct {
	mu      sync.Mutex
	fails   int
	calls   int
	batches [][]string
}

func (r *recorder) consume(_ context.Context, events []core.Activity) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	if r.fails > 0 {
		r.fails--
		return errors.New("busy")
	}
	var types []string
	for _, e := range events {
		types = append(types, e.Type)
	}
	r.batches = append(r.batches, types)
	return nil
}

// result returns the calls and the delivered batches so far.
func (r *recorder) result() (int, [][]string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls, r.batches
}

func events(n int) []core.Activity {
	out := make([]core.Activity, n)
	for i := range out {
		out[i] = core.Activity{Type: "item." + strconv.Itoa(i)}
	}
	return out
}

func TestEventQueue(t *testing.T) {
	tests := []struct {
		name  string
		push  int
		fails int
		// wantBatches are the sizes of the delivered batches.
		wantBatches []int
		wantCalls   int
	}{
		{"one batch", 3, 0, []int{3}, 1},
		{"split at the batch size", 150, 0, []int{100, 50}, 2},
		{"retried", 2, 2, []int{2}, 3},
		{"dropped after the retries", 2, eventRetries + 1, nil, eventRetries + 1},
		{"bounded queue", eventQueueSize + 5, 0, nil, eventQueueSize / eventBatchSize},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				r := &recorder{fails: tt.fails}
				q := newEventQueue("p", slog.New(slog.DiscardHandler), r.consume)
				ctx, cancel := context.WithCancel(t.Context())
				go q.run(ctx)
				for _, e := range events(tt.push) {
					q.push(e)
				}
				// Nothing is sent before the batch delay.
				time.Sleep(eventBatchDelay - time.Millisecond)
				synctest.Wait()
				if calls, _ := r.result(); calls != 0 {
					t.Fatalf("calls before the batch delay = %d, want 0", calls)
				}
				time.Sleep(time.Minute)
				synctest.Wait()
				calls, batches := r.result()
				if calls != tt.wantCalls {
					t.Errorf("calls = %d, want %d", calls, tt.wantCalls)
				}
				if tt.wantBatches != nil {
					var sizes []int
					for _, b := range batches {
						sizes = append(sizes, len(b))
					}
					if !slices.Equal(sizes, tt.wantBatches) {
						t.Errorf("batch sizes = %v, want %v", sizes, tt.wantBatches)
					}
				}
				// Events arrive in order.
				i := 0
				for _, b := range batches {
					for _, typ := range b {
						if want := "item." + strconv.Itoa(i); typ != want {
							t.Fatalf("event %d = %s, want %s", i, typ, want)
						}
						i++
					}
				}
				cancel()
			})
		})
	}
}

func TestEventQueueStop(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := &recorder{}
		q := newEventQueue("p", slog.New(slog.DiscardHandler), r.consume)
		go q.run(t.Context())
		q.push(core.Activity{Type: "item.added"})
		q.stop()
		time.Sleep(time.Minute)
		synctest.Wait()
		if calls, _ := r.result(); calls != 0 {
			t.Errorf("calls after stop = %d, want 0", calls)
		}
	})
}
