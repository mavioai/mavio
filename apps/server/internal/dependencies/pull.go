package dependencies

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

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
	waiters []func(containers.PullProgress)
	done    chan struct{}
	err     error
}

// pullImage pulls an image as a JobPull job and waits for it, telling
// progress of it; without a job queue it pulls it at once.
func (s *Service) pullImage(engine containers.Engine) func(ctx context.Context, ref string, progress func(containers.PullProgress)) error {
	if s.cfg.Jobs == nil {
		return engine.PullImage
	}
	return func(ctx context.Context, ref string, progress func(containers.PullProgress)) error {
		s.mu.Lock()
		p := s.pulls[ref]
		if p == nil {
			p = &pull{done: make(chan struct{})}
			s.pulls[ref] = p
		}
		p.waiters = append(p.waiters, progress)
		s.mu.Unlock()
		payload, _ := json.Marshal(PullPayload{Image: ref})
		job := core.Job{ID: core.NewID(), Kind: JobPull, Payload: payload, UniqueKey: JobPull + ":" + ref, MaxAttempts: 1, RunAt: s.cfg.Now()}
		if _, err := s.cfg.Jobs.Enqueue(ctx, &job); err != nil {
			return fmt.Errorf("queue the pull of %s: %w", ref, err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-p.done:
			return p.err
		}
	}
}

// RunPull is the handler of JobPull jobs.
func (s *Service) RunPull(ctx context.Context, job core.Job) ([]core.Job, error) {
	var pl PullPayload
	if err := json.Unmarshal(job.Payload, &pl); err != nil {
		return nil, fmt.Errorf("pull payload: %w", err)
	}
	rt, err := s.Runtime(ctx, false)
	if err == nil {
		var frac containers.PullFraction
		err = s.cfg.Engine(rt).PullImage(ctx, pl.Image, func(pp containers.PullProgress) {
			f, moved := frac.Add(pp)
			s.mu.Lock()
			if moved {
				s.progress[job.ID] = f
			}
			var waiters []func(containers.PullProgress)
			if p := s.pulls[pl.Image]; p != nil {
				waiters = slices.Clone(p.waiters)
			}
			s.mu.Unlock()
			for _, w := range waiters {
				w(pp)
			}
		})
	}
	s.mu.Lock()
	p := s.pulls[pl.Image]
	delete(s.pulls, pl.Image)
	delete(s.progress, job.ID)
	s.mu.Unlock()
	if p != nil {
		p.err = err
		close(p.done)
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
