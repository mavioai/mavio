package dependencies

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/mavioai/mavio/libs/containers"
	"github.com/mavioai/mavio/libs/core"
)

// JobPull pulls the image of a managed dependency; the payload is a
// PullPayload.
const JobPull = "plugin.container_pull"

// PullPayload is the payload of JobPull jobs.
type PullPayload struct {
	Image string `json:"image"`
}

// pull is an image being pulled and those waiting for it.
type pull struct {
	waiters map[*waiter]bool
	// running reports that its job runs, and cancel ends it.
	running bool
	cancel  context.CancelFunc
	done    chan struct{}
	err     error
}

// waiter is told of a pull's progress.
type waiter struct{ progress func(containers.PullProgress) }

// errAbandoned ends a pull nothing waits for any longer.
var errAbandoned = errors.New("nothing waits for the pull")

// abandonAfter is how long a running pull nothing waits for goes on
// before it is cancelled.
const abandonAfter = 2 * time.Second

// pullImage pulls an image as a JobPull job and waits for it, telling
// progress of it, and containers.PullQueued while the job waits for
// others; without a job queue it pulls it at once. A pull nothing waits
// for any longer is cancelled after abandonAfter, or left out when its
// job starts.
func (s *Service) pullImage(engine containers.Engine) func(ctx context.Context, ref string, progress func(containers.PullProgress)) error {
	if s.cfg.Jobs == nil {
		return engine.PullImage
	}
	return func(ctx context.Context, ref string, progress func(containers.PullProgress)) error {
		w := &waiter{progress: progress}
		s.mu.Lock()
		p := s.pulls[ref]
		if p == nil {
			p = &pull{waiters: map[*waiter]bool{}, done: make(chan struct{})}
			s.pulls[ref] = p
		}
		p.waiters[w] = true
		running := p.running
		s.mu.Unlock()
		defer func() {
			s.mu.Lock()
			delete(p.waiters, w)
			if len(p.waiters) == 0 && p.cancel != nil {
				// A dependency started again waits for the same image.
				time.AfterFunc(abandonAfter, func() {
					s.mu.Lock()
					defer s.mu.Unlock()
					if len(p.waiters) == 0 {
						p.cancel()
					}
				})
			}
			s.mu.Unlock()
		}()
		payload, _ := json.Marshal(PullPayload{Image: ref})
		job := core.Job{ID: core.NewID(), Kind: JobPull, Payload: payload, UniqueKey: JobPull + ":" + ref, MaxAttempts: 1, RunAt: s.cfg.Now()}
		if _, err := s.cfg.Jobs.Enqueue(ctx, &job); err != nil {
			return fmt.Errorf("queue the pull of %s: %w", ref, err)
		}
		if !running {
			progress(containers.PullProgress{Status: containers.PullQueued})
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-p.done:
			return p.err
		}
	}
}

// RunPull is the handler of JobPull jobs. It leaves out a pull that
// waiters left, and ends one when the last of them leaves.
func (s *Service) RunPull(ctx context.Context, job core.Job) ([]core.Job, error) {
	var pl PullPayload
	if err := json.Unmarshal(job.Payload, &pl); err != nil {
		return nil, fmt.Errorf("pull payload: %w", err)
	}
	pctx, cancel := context.WithCancel(ctx)
	defer cancel()
	s.mu.Lock()
	p := s.pulls[pl.Image]
	if p != nil {
		if len(p.waiters) == 0 {
			delete(s.pulls, pl.Image)
			s.mu.Unlock()
			p.err = errAbandoned
			close(p.done)
			s.cfg.Logger.InfoContext(ctx, "left out a pull nothing waits for", "image", pl.Image)
			return nil, nil
		}
		p.running, p.cancel = true, cancel
	}
	s.mu.Unlock()
	rt, err := s.Runtime(ctx, false)
	if err == nil {
		var frac containers.PullFraction
		err = s.cfg.Engine(rt).PullImage(pctx, pl.Image, func(pp containers.PullProgress) {
			f, moved := frac.Add(pp)
			s.mu.Lock()
			if moved {
				s.progress[job.ID] = f
			}
			var waiters []*waiter
			if p := s.pulls[pl.Image]; p != nil {
				waiters = slices.Collect(maps.Keys(p.waiters))
			}
			s.mu.Unlock()
			for _, w := range waiters {
				w.progress(pp)
			}
		})
	}
	abandoned := err != nil && pctx.Err() != nil && ctx.Err() == nil
	if abandoned {
		err = errAbandoned
	}
	s.mu.Lock()
	p = s.pulls[pl.Image]
	delete(s.pulls, pl.Image)
	delete(s.progress, job.ID)
	s.mu.Unlock()
	if p != nil {
		p.err = err
		close(p.done)
	}
	if abandoned {
		s.cfg.Logger.InfoContext(ctx, "cancelled a pull nothing waits for", "image", pl.Image)
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("pull %s: %w", pl.Image, err)
	}
	return nil, nil
}

// JobProgress returns the fraction done of a running JobPull job.
func (s *Service) JobProgress(id core.ID) (float64, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, ok := s.progress[id]
	return f, ok
}
