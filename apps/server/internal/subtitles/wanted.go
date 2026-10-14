package subtitles

import (
	"context"
	"errors"
	"fmt"

	"github.com/mavioai/mavio/libs/core"
)

// Wanted is a movie or an episode missing subtitles.
type Wanted struct {
	Item core.Item
	// Series is the episode's series.
	Series  *core.Item
	Path    string
	Missing []core.SubtitleLanguage
}

// Wanted returns the movies or the episodes missing subtitles, by
// library and path, and how many there are; limit 0 returns a page of
// core.MaxPageSize from offset. They are read from what is kept as files,
// profiles and settings change (core.SubtitleWanted).
func (r *Runtime) Wanted(ctx context.Context, kind core.ItemKind, limit, offset int) ([]Wanted, int, error) {
	page, err := r.Store.SubtitleWanted().List(ctx, core.SubtitleWantedQuery{Kind: kind, Limit: limit, Offset: offset})
	if err != nil {
		return nil, 0, err
	}
	series := map[core.ID]*core.Item{}
	out := make([]Wanted, 0, len(page.Items))
	for _, w := range page.Items {
		it, err := r.Store.Items().Get(ctx, w.ItemID)
		if errors.Is(err, core.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, 0, err
		}
		e := Wanted{Item: it, Path: w.Path, Missing: w.Missing}
		if !w.ParentID.IsZero() {
			s, ok := series[w.ParentID]
			if !ok {
				p, err := r.Store.Items().Get(ctx, w.ParentID)
				if err == nil {
					s = &p
				} else if !errors.Is(err, core.ErrNotFound) {
					return nil, 0, err
				}
				series[w.ParentID] = s
			}
			e.Series = s
		}
		out = append(out, e)
	}
	return out, page.Total, nil
}

// wantedIDs returns the movies or episodes missing subtitles.
func (r *Runtime) wantedIDs(ctx context.Context, kind core.ItemKind) ([]core.ID, error) {
	var out []core.ID
	for {
		page, err := r.Store.SubtitleWanted().List(ctx, core.SubtitleWantedQuery{Kind: kind, Offset: len(out)})
		if err != nil {
			return nil, err
		}
		for _, w := range page.Items {
			out = append(out, w.ItemID)
		}
		if len(page.Items) == 0 || len(out) >= page.Total {
			return out, nil
		}
	}
}

// keepWanted stores what a target is missing.
func (r *Runtime) keepWanted(ctx context.Context, t *target, missing []core.SubtitleLanguage) error {
	w := core.SubtitleWanted{
		ItemID: t.item.ID, Kind: t.item.Kind, LibraryID: t.lib.ID, Path: t.source().Path, Missing: missing, Updated: r.now(),
	}
	if t.series != nil {
		w.ParentID = t.series.ID
	}
	if err := r.Store.SubtitleWanted().Put(ctx, w); err != nil {
		return fmt.Errorf("keep wanted subtitles of %s: %w", t.item.Name, err)
	}
	return nil
}

// refreshWanted finds anew what a movie or an episode is missing and
// keeps it; items without a video file want nothing.
func (r *Runtime) refreshWanted(ctx context.Context, itemID core.ID, s *core.SubtitleSettings) error {
	t, err := r.load(ctx, itemID, s)
	switch {
	case errors.Is(err, errNoSubtitles):
		return nil
	case errors.Is(err, core.ErrNotFound):
		return r.Store.SubtitleWanted().Delete(ctx, itemID)
	case err != nil:
		return err
	}
	return r.keepWanted(ctx, t, t.missing(s))
}

// updateWanted finds anew what an item is missing after its subtitles
// changed; a failure is logged, as the change itself was made.
func (r *Runtime) updateWanted(ctx context.Context, itemID core.ID) {
	s := r.settings()
	if err := r.refreshWanted(ctx, itemID, &s); err != nil {
		r.logger().WarnContext(ctx, "update wanted subtitles", "item", itemID, "err", err)
	}
}

// RefreshWanted finds anew what every movie and episode is missing, as
// Bazarr lists missing subtitles after a sync with Radarr and Sonarr. It
// runs after profiles, their assignments or the settings change.
func (r *Runtime) RefreshWanted(ctx context.Context) error {
	s := r.settings()
	// The items are listed first, so that loading them does not wait
	// for the walk's connection.
	var ids []core.ID
	for it, err := range r.Store.Items().Walk(ctx, core.ItemQuery{Kinds: []core.ItemKind{core.KindMovie, core.KindEpisode}}) {
		if err != nil {
			return err
		}
		ids = append(ids, it.ID)
	}
	for _, id := range ids {
		if err := r.refreshWanted(ctx, id, &s); err != nil {
			return err
		}
	}
	return nil
}

// Sidecars finds anew what the items a scan found subtitle files of, or
// lost them, are missing, for library.Scanner.OnSidecars.
func (r *Runtime) Sidecars(ctx context.Context, _ core.Library, ids []core.ID) {
	for _, id := range ids {
		r.updateWanted(ctx, id)
	}
}
