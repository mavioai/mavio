package library

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

// Handler runs one kind of job. Jobs it returns are enqueued once the job
// has completed, such as the next scheduled run, which could not be
// enqueued while the job itself still holds its unique key.
type Handler func(ctx context.Context, job core.Job) ([]core.Job, error)

// Worker leases jobs from the queue and runs their handlers, extending the
// lease while a job runs. A crashed worker's jobs return to the queue when
// their leases expire.
type Worker struct {
	Queue    core.JobQueue
	Owner    string
	Handlers map[string]Handler
	// LeaseTTL is how long a lease lasts without being extended; default
	// one minute. Leases are extended at a third of it.
	LeaseTTL time.Duration
	// Idle is how long the worker waits when no job is due; default one
	// second.
	Idle   time.Duration
	Logger *slog.Logger
}

func (w *Worker) logger() *slog.Logger {
	if w.Logger != nil {
		return w.Logger
	}
	return slog.New(slog.DiscardHandler)
}

// Run works until ctx is canceled.
func (w *Worker) Run(ctx context.Context) error {
	idle := w.Idle
	if idle <= 0 {
		idle = time.Second
	}
	for {
		ran, err := w.RunOne(ctx)
		if ctx.Err() != nil {
			return context.Cause(ctx)
		}
		if err != nil {
			w.logger().ErrorContext(ctx, "job queue", "err", err)
		}
		if ran {
			continue
		}
		t := time.NewTimer(idle)
		select {
		case <-ctx.Done():
			t.Stop()
			return context.Cause(ctx)
		case <-t.C:
		}
	}
}

func (w *Worker) ttl() time.Duration {
	if w.LeaseTTL > 0 {
		return w.LeaseTTL
	}
	return time.Minute
}

// RunOne leases one due job and runs it; it reports whether there was one.
func (w *Worker) RunOne(ctx context.Context) (bool, error) {
	kinds := slices.Sorted(maps.Keys(w.Handlers))
	job, err := w.Queue.Lease(ctx, w.Owner, kinds, w.ttl())
	if errors.Is(err, core.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	log := w.logger().With("job", job.ID, "kind", job.Kind, "attempt", job.Attempts)

	jobCtx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	done := make(chan struct{})
	go func() {
		// Keep the lease while the job runs; losing it means another worker
		// may take the job, so this one stops.
		t := time.NewTicker(w.ttl() / 3)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-t.C:
				if err := w.Queue.Extend(ctx, job.ID, w.Owner, w.ttl()); err != nil {
					cancel(fmt.Errorf("lease lost: %w", err))
					return
				}
			}
		}
	}()
	next, runErr := w.run(jobCtx, job)
	close(done)

	if ctx.Err() != nil {
		// Shutting down: the lease expires and the job runs again.
		return true, nil
	}
	if runErr != nil {
		log.WarnContext(ctx, "job failed", "err", runErr)
		return true, w.Queue.Fail(ctx, job.ID, w.Owner, runErr)
	}
	if err := w.Queue.Complete(ctx, job.ID, w.Owner); err != nil {
		return true, err
	}
	for i := range next {
		if _, err := w.Queue.Enqueue(ctx, &next[i]); err != nil {
			return true, err
		}
	}
	log.DebugContext(ctx, "job done")
	return true, nil
}

func (w *Worker) run(ctx context.Context, job core.Job) (next []core.Job, err error) {
	h, ok := w.Handlers[job.Kind]
	if !ok {
		return nil, fmt.Errorf("no handler for job kind %q", job.Kind)
	}
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("job panicked: %v", r)
		}
	}()
	return h(ctx, job)
}
