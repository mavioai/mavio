package library

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

// JobCleanup deletes the jobs that finished long ago and the folders in
// the metadata folder of items that no longer exist.
const JobCleanup = "jobs.cleanup"

// Housekeeping intervals.
const (
	// CleanupInterval is the period of scheduled cleanups.
	CleanupInterval = 24 * time.Hour
	// JobRetention is how long finished jobs are kept.
	JobRetention = 7 * 24 * time.Hour
	// MetadataGrace is how long the folder of an item that does not exist
	// is kept in the metadata folder, leaving alone one made while its item
	// is being saved.
	MetadataGrace = 24 * time.Hour
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

// cleanup purges finished jobs and the metadata folders of deleted items;
// a scheduled cleanup queues the next.
func (j *Jobs) cleanup(ctx context.Context, job core.Job) ([]core.Job, error) {
	n, err := j.Store.Jobs().Purge(ctx, j.now().Add(-JobRetention))
	if err != nil {
		return nil, err
	}
	if n > 0 {
		j.logger().InfoContext(ctx, "purged finished jobs", "count", n)
	}
	n, err = j.sweepMetadata(ctx)
	if n > 0 {
		j.logger().InfoContext(ctx, "removed the metadata folders of deleted items", "count", n)
	}
	if err != nil {
		return nil, fmt.Errorf("sweep metadata folder: %w", err)
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

// sweepMetadata removes the folders "<id[:2]>/<id>" in MetadataDir, with
// the artwork, subtitles, trickplay sheets and chapter images within, of
// items that no longer exist, such as those purged by scans or deleted
// with their library, unless they changed within MetadataGrace. It
// reports how many it removed.
func (j *Jobs) sweepMetadata(ctx context.Context) (int, error) {
	if j.MetadataDir == "" {
		return 0, nil
	}
	root, err := os.OpenRoot(filepath.FromSlash(j.MetadataDir))
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	defer root.Close()
	before := j.now().Add(-MetadataGrace)
	prefixes, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		return 0, err
	}
	n := 0
	for _, prefix := range prefixes {
		if !prefix.IsDir() || len(prefix.Name()) != 2 {
			continue
		}
		folders, err := fs.ReadDir(root.FS(), prefix.Name())
		if err != nil {
			return n, err
		}
		for _, folder := range folders {
			if err := ctx.Err(); err != nil {
				return n, err
			}
			id, err := core.ParseID(folder.Name())
			if err != nil || !folder.IsDir() || id.String() != folder.Name() || id.String()[:2] != prefix.Name() {
				continue
			}
			info, err := folder.Info()
			if err != nil {
				return n, err
			}
			if !info.ModTime().Before(before) {
				continue
			}
			if _, err := j.Store.Items().Get(ctx, id); !errors.Is(err, core.ErrNotFound) {
				if err != nil {
					return n, err
				}
				continue
			}
			if err := root.RemoveAll(path.Join(prefix.Name(), folder.Name())); err != nil {
				return n, err
			}
			n++
		}
	}
	return n, nil
}
