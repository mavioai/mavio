package subtitles

import (
	"context"
	"errors"

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
// library and path, and how many there are; limit 0 returns all from
// offset. Wanted subtitles are found anew from the profiles and the
// streams, as Bazarr's wanted lists after a scan.
func (r *Runtime) Wanted(ctx context.Context, kind core.ItemKind, limit, offset int) ([]Wanted, int, error) {
	s := r.settings()
	var out []Wanted
	total := 0
	err := r.eachWanted(ctx, kind, &s, func(t *target, missing []core.SubtitleLanguage) {
		total++
		if total <= offset || limit > 0 && len(out) >= limit {
			return
		}
		out = append(out, Wanted{Item: t.item, Series: t.series, Path: t.source().Path, Missing: missing})
	})
	return out, total, err
}

// wantedIDs returns the movies or episodes missing subtitles.
func (r *Runtime) wantedIDs(ctx context.Context, kind core.ItemKind) ([]core.ID, error) {
	s := r.settings()
	var out []core.ID
	err := r.eachWanted(ctx, kind, &s, func(t *target, _ []core.SubtitleLanguage) { out = append(out, t.item.ID) })
	return out, err
}

// eachWanted calls f with each movie or episode missing subtitles.
func (r *Runtime) eachWanted(ctx context.Context, kind core.ItemKind, s *core.SubtitleSettings, f func(t *target, missing []core.SubtitleLanguage)) error {
	// The items are listed first, so that loading them does not wait
	// for the walk's connection.
	var ids []core.ID
	for it, err := range r.Store.Items().Walk(ctx, core.ItemQuery{Kinds: []core.ItemKind{kind}}) {
		if err != nil {
			return err
		}
		ids = append(ids, it.ID)
	}
	for _, id := range ids {
		t, err := r.load(ctx, id, s)
		if errors.Is(err, core.ErrNotFound) {
			continue
		}
		if err != nil {
			return err
		}
		if missing := t.missing(s); len(missing) > 0 {
			f(t, missing)
		}
	}
	return nil
}
