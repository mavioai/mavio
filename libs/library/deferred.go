package library

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library/storage"
)

// DefaultDeferredInterval is how often RunDeferred checks deferred files.
const DefaultDeferredInterval = 10 * time.Second

// RunDeferred checks the files the GrowthPolicy deferred every interval
// (DefaultDeferredInterval when not positive) until ctx is canceled, and
// queues a scan of each library whose deferred files settled or went
// away. It returns at once without a GrowthPolicy.
func (s *Scanner) RunDeferred(ctx context.Context, interval time.Duration) error {
	if s.GrowthPolicy == nil {
		return nil
	}
	if interval <= 0 {
		interval = DefaultDeferredInterval
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return context.Cause(ctx)
		case <-t.C:
			s.ReconcileDeferred(ctx)
		}
	}
}

// ReconcileDeferred queues a scan of each library whose deferred files
// settled or went away.
func (s *Scanner) ReconcileDeferred(ctx context.Context) {
	if s.GrowthPolicy == nil || s.GrowthPolicy.Pending() == 0 {
		return
	}
	owners := s.GrowthPolicy.Reconcile(func(name string) (storage.FileStamp, bool) {
		fi, err := os.Stat(filepath.FromSlash(name))
		if err != nil {
			return storage.FileStamp{}, false
		}
		return storage.FileStamp{Size: fi.Size(), ModTime: modTime(fi)}, true
	})
	for _, owner := range owners {
		id, err := core.ParseID(owner)
		if err != nil {
			continue
		}
		lib, err := s.Store.Libraries().Get(ctx, id)
		if errors.Is(err, core.ErrNotFound) {
			continue
		}
		if err != nil {
			s.logger().WarnContext(ctx, "scan settled files", "library", owner, "err", err)
			continue
		}
		job := ScanJob(lib, s.now(), false)
		if _, err := s.Store.Jobs().Enqueue(ctx, &job); err != nil {
			s.logger().WarnContext(ctx, "scan settled files", "library", lib.Name, "err", err)
			continue
		}
		s.logger().InfoContext(ctx, "files finished writing; scanning", "library", lib.Name)
	}
}
