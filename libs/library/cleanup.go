package library

import (
	"context"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

// JobCleanup deletes the jobs that finished long ago.
const JobCleanup = "jobs.cleanup"

// Housekeeping intervals.
const (
	// CleanupInterval is the period of scheduled cleanups.
	CleanupInterval = 24 * time.Hour
	// JobRetention is how long finished jobs are kept.
	JobRetention = 7 * 24 * time.Hour
)

// CleanupJob returns a cleanup at the given time. Scheduled and requested
// cleanups have separate keys, as scans do.
func CleanupJob(at time.Time, scheduled bool) core.Job {
	key := JobCleanup
	if scheduled {
		key += ":scheduled"
	}
	return core.Job{ID: core.NewID(), Kind: JobCleanup, UniqueKey: key, MaxAttempts: 3, RunAt: at, Priority: -10}
}

// cleanup purges finished jobs; a scheduled cleanup queues the next.
func (j *Jobs) cleanup(ctx context.Context, job core.Job) ([]core.Job, error) {
	n, err := j.Store.Jobs().Purge(ctx, j.now().Add(-JobRetention))
	if err != nil {
		return nil, err
	}
	if n > 0 {
		j.logger().InfoContext(ctx, "purged finished jobs", "count", n)
	}
	if job.UniqueKey != JobCleanup+":scheduled" {
		return nil, nil
	}
	return []core.Job{CleanupJob(j.now().Add(CleanupInterval), true)}, nil
}

// ScheduleHousekeeping enqueues the scheduled cleanup, unless one is
// pending; each enqueues the next.
func (j *Jobs) ScheduleHousekeeping(ctx context.Context) error {
	job := CleanupJob(j.now().Add(time.Minute), true)
	_, err := j.Store.Jobs().Enqueue(ctx, &job)
	return err
}
