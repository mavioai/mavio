package acquisition

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mavioai/mavio/libs/core"
)

// pendingRelease is a stored pending release with its release.
type pendingRelease struct {
	core.PendingRelease
	release *Release
}

// pending returns the pending releases of an item, or of all for
// NilID. Releases that no longer decode are deleted.
func (r *Runtime) pending(ctx context.Context, itemID core.ID) ([]pendingRelease, error) {
	stored, err := r.Store.PendingReleases().List(ctx, itemID)
	if err != nil {
		return nil, err
	}
	out := make([]pendingRelease, 0, len(stored))
	for _, p := range stored {
		var rel Release
		if err := json.Unmarshal(p.Release, &rel); err != nil {
			r.logger().WarnContext(ctx, "dropping pending release that does not decode", "release", p.Title, "err", err)
			if err := r.Store.PendingReleases().Delete(ctx, p.ID); err != nil {
				return nil, err
			}
			continue
		}
		out = append(out, pendingRelease{PendingRelease: p, release: &rel})
	}
	return out, nil
}

// pendingAdd is a release to keep pending for a movie.
type pendingAdd struct {
	itemID core.ID
	rel    *Release
	reason core.PendingReason
}

// sameRelease reports whether two releases are the same, as Radarr's
// PendingReleaseService.MatchingReleasePredicate: by title, publish date
// and indexer.
func sameRelease(a, b *Release) bool {
	return a.Title == b.Title && a.PublishDate.Equal(b.PublishDate) && a.IndexerRef() == b.IndexerRef()
}

// addPending keeps releases pending, as Radarr's
// PendingReleaseService.AddMany: a release already pending keeps its
// time and takes the new reason, unless it waits for a download client;
// duplicates are removed.
func (r *Runtime) addPending(ctx context.Context, adds []pendingAdd) error {
	byItem := map[core.ID][]pendingRelease{}
	for _, a := range adds {
		existing, ok := byItem[a.itemID]
		if !ok {
			var err error
			if existing, err = r.pending(ctx, a.itemID); err != nil {
				return err
			}
		}
		var matching []int
		for i := range existing {
			if sameRelease(existing[i].release, a.rel) {
				matching = append(matching, i)
			}
		}
		if len(matching) == 0 {
			data, err := json.Marshal(a.rel)
			if err != nil {
				return fmt.Errorf("pending release: %w", err)
			}
			p := pendingRelease{
				PendingRelease: core.PendingRelease{ID: core.NewID(), ItemID: a.itemID, ItemIDs: []core.ID{a.itemID}, Title: a.rel.Title, Added: r.now(), Reason: a.reason, Release: data},
				release:        a.rel,
			}
			if err := r.Store.PendingReleases().Add(ctx, &p.PendingRelease); err != nil {
				return err
			}
			byItem[a.itemID] = append(existing, p)
			continue
		}
		first := &existing[matching[0]]
		if first.Reason != a.reason && first.Reason != core.PendingClientUnavailable {
			// The store keeps no updates of pending releases: the release is
			// added again with its time and the new reason.
			if err := r.Store.PendingReleases().Delete(ctx, first.ID); err != nil {
				return err
			}
			first.ID, first.Reason = core.NewID(), a.reason
			if err := r.Store.PendingReleases().Add(ctx, &first.PendingRelease); err != nil {
				return err
			}
		}
		var dups []core.ID
		for _, i := range matching[1:] {
			dups = append(dups, existing[i].ID)
		}
		if len(dups) > 0 {
			if err := r.Store.PendingReleases().Delete(ctx, dups...); err != nil {
				return err
			}
			for i := len(matching) - 1; i >= 1; i-- {
				existing = append(existing[:matching[i]], existing[matching[i]+1:]...)
			}
		}
		byItem[a.itemID] = existing
	}
	return nil
}

// removePending deletes the pending releases of an item that match
// match, or all when match is nil.
func (r *Runtime) removePending(ctx context.Context, itemID core.ID, match func(*Release) bool) error {
	pending, err := r.pending(ctx, itemID)
	if err != nil {
		return err
	}
	var ids []core.ID
	for _, p := range pending {
		if match == nil || match(p.release) {
			ids = append(ids, p.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	return r.Store.PendingReleases().Delete(ctx, ids...)
}
