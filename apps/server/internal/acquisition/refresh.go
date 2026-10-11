package acquisition

import (
	"context"
	"errors"
	"fmt"

	"github.com/mavioai/mavio/libs/acquire"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
)

// JobRefreshMovies refreshes the metadata of the wanted movies due for it,
// daily, as Radarr's RefreshMovieCommand.
const JobRefreshMovies = "acquisition.refresh_movies"

// RefreshMovies queues refreshes of the wanted movies whose metadata is
// due for one, as Radarr's ShouldRefreshMovie decides: movies never
// refreshed, such as those whose identification failed when they were
// added and are still named by their TMDB ID, those refreshed more than
// 180 days ago, and those not yet released or released in the last 30
// days.
func (r *Runtime) RefreshMovies(ctx context.Context) error {
	wanted, err := r.Store.Wanted().List(ctx)
	if err != nil {
		return err
	}
	now := r.now()
	queued := 0
	for _, w := range wanted {
		it, err := r.Store.Items().Get(ctx, w.ItemID)
		if errors.Is(err, core.ErrNotFound) {
			continue
		}
		if err != nil {
			return err
		}
		if it.Kind != core.KindMovie {
			continue
		}
		if !acquire.ShouldRefreshMovie(it.MetadataRefreshedAt, availability(it, w), now) {
			continue
		}
		job := library.RefreshJob(it.ID, now)
		if _, err := r.Store.Jobs().Enqueue(ctx, &job); err != nil {
			return fmt.Errorf("queue refresh of movie %s: %w", it.ID, err)
		}
		queued++
	}
	if queued > 0 {
		r.logger().InfoContext(ctx, "queued refreshes of wanted movies", "movies", queued)
	}
	return nil
}
