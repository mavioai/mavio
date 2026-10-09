// Package browse builds the lists of a client's home screen from the
// store: the latest items of the libraries and the next episodes to watch.
package browse

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

// pageSize is the number of items read per query while building a list.
const pageSize = 100

// Scope is what a user may see: their libraries (nil means all) and
// rating limit.
type Scope struct {
	UserID      core.ID
	LibraryIDs  []core.ID
	MaxRating   *int
	SkipUnrated bool
}

// allows reports whether an item is in the scope's libraries and rating.
func (s Scope) allows(it *core.Item) bool {
	policy := core.UserPolicy{Libraries: s.LibraryIDs, MaxParentalRating: s.MaxRating, BlockUnrated: s.SkipUnrated}
	return policy.CanAccess(it)
}

// query returns an item query limited to the scope.
func (s Scope) query() core.ItemQuery {
	return core.ItemQuery{LibraryIDs: s.LibraryIDs, MaxRating: s.MaxRating, SkipUnrated: s.SkipUnrated, UserID: s.UserID}
}

// Browser builds the lists.
type Browser struct {
	store core.Store
}

// New returns a Browser reading store.
func New(store core.Store) *Browser { return &Browser{store: store} }

// LatestKinds are the kinds of items listed as latest by default: those
// played or viewed, not their containers.
var LatestKinds = []core.ItemKind{
	core.KindMovie, core.KindEpisode, core.KindVideo, core.KindTrack, core.KindMusicVideo,
	core.KindAudioBook, core.KindBook, core.KindPhoto,
}

// LatestQuery selects the latest items.
type LatestQuery struct {
	// Kinds defaults to LatestKinds.
	Kinds []core.ItemKind
	// Played filters by the user's played state when set.
	Played *bool
	// Group shows the new episodes of a series as the series or a season,
	// and new tracks as their album.
	Group bool
	Limit int
}

// recentWindow is how long before a series' newest episode other episodes
// count as added with it.
const recentWindow = 24 * time.Hour

// Latest lists the items added last, newest first. Grouped, as in
// Jellyfin, an album stands for its new tracks, and a series for its new
// episodes: the episodes added within a day of its newest one are shown as
// their season when they are all in one season of a series with several,
// as the series when the series has one season or they span several, and
// as the newest episode when it came alone. A container the user may not
// see is replaced by the newest item.
func (b *Browser) Latest(ctx context.Context, scope Scope, q LatestQuery) ([]core.Item, error) {
	iq := scope.query()
	iq.Kinds = cmpOr(q.Kinds, LatestKinds)
	iq.Played = q.Played
	iq.Sort = []core.SortSpec{
		{Field: core.SortDateAdded, Desc: true},
		{Field: core.SortName, Desc: true},
		{Field: core.SortProductionYear, Desc: true},
	}
	iq.Limit = pageSize

	// The newest item of each series, album or other item, newest first.
	var newest []core.Item
	var containers []*core.Item
	seen := map[core.ID]bool{}
	items := newCache(b.store)
	for iq.Offset = 0; len(newest) < q.Limit; iq.Offset += pageSize {
		page, err := b.store.Items().Query(ctx, iq)
		if err != nil {
			return nil, fmt.Errorf("query latest items: %w", err)
		}
		for _, it := range page.Items {
			var container *core.Item
			if q.Group {
				if container, err = items.container(ctx, &it); err != nil {
					return nil, err
				}
			}
			key := it.ID
			if container != nil {
				key = container.ID
			}
			if seen[key] {
				continue
			}
			seen[key] = true
			newest, containers = append(newest, it), append(containers, container)
			if len(newest) >= q.Limit {
				break
			}
		}
		if len(page.Items) < pageSize {
			break
		}
	}

	out := make([]core.Item, len(newest))
	for i, it := range newest {
		out[i] = it
		switch c := containers[i]; {
		case c == nil:
		case c.Kind == core.KindMusicAlbum:
			if scope.allows(c) {
				out[i] = *c
			}
		default:
			shown, err := b.latestOfSeries(ctx, scope, q.Played, c, &it, items)
			if err != nil {
				return nil, err
			}
			out[i] = *shown
		}
	}
	return out, nil
}

// latestOfSeries picks what shows the new episodes of a series whose
// newest is newest: the series, a season or that episode.
func (b *Browser) latestOfSeries(ctx context.Context, scope Scope, played *bool, series, newest *core.Item, items *cache) (*core.Item, error) {
	iq := scope.query()
	iq.ParentID, iq.Recursive, iq.Kinds, iq.Played = series.ID, true, []core.ItemKind{core.KindEpisode}, played
	cutoff := newest.DateAdded.Add(-recentWindow)
	recent := 0
	seasons := map[core.ID]bool{}
	for ep, err := range b.store.Items().Walk(ctx, iq) {
		if err != nil {
			return nil, fmt.Errorf("list episodes: %w", err)
		}
		if ep.DateAdded.Before(cutoff) {
			continue
		}
		recent++
		if ep.ParentID != series.ID {
			seasons[ep.ParentID] = true
		}
	}

	var shown *core.Item
	switch len(seasons) {
	case 1:
		var season core.ID
		for id := range seasons {
			season = id
		}
		episodes, err := b.count(ctx, season, core.KindEpisode)
		if err != nil {
			return nil, err
		}
		allSeasons, err := b.count(ctx, series.ID, core.KindSeason)
		if err != nil {
			return nil, err
		}
		if recent > 1 || recent == episodes {
			shown = series
			if allSeasons > 1 {
				if shown, err = items.get(ctx, season); err != nil {
					return nil, err
				}
			}
		}
	default:
		if len(seasons) > 1 {
			shown = series
		}
	}
	if shown == nil || !scope.allows(shown) {
		return newest, nil
	}
	return shown, nil
}

// count returns the number of present children of a kind.
func (b *Browser) count(ctx context.Context, parent core.ID, kind core.ItemKind) (int, error) {
	page, err := b.store.Items().Query(ctx, core.ItemQuery{ParentID: parent, Kinds: []core.ItemKind{kind}, Limit: 1})
	if err != nil {
		return 0, fmt.Errorf("count %ss: %w", kind, err)
	}
	return page.Total, nil
}

// NextUpQuery selects the next episodes.
type NextUpQuery struct {
	// SeriesID restricts to one series.
	SeriesID core.ID
	// Since leaves out series last played before it.
	Since time.Time
	// IncludeResumable also offers episodes with a resume position, which
	// otherwise belong to continue watching.
	IncludeResumable bool
	Limit, Offset    int
}

// NextUp lists the next episode to watch of each series the user has
// played, the series played most recently first (core.NextEpisode).
func (b *Browser) NextUp(ctx context.Context, scope Scope, q NextUpQuery) ([]core.Item, error) {
	want := q.Offset + q.Limit
	var out []core.Item
	add := func(series core.ID) error {
		next, err := b.nextEpisode(ctx, scope, series, q.IncludeResumable)
		if next != nil {
			out = append(out, *next)
		}
		return err
	}
	if !q.SeriesID.IsZero() {
		if err := add(q.SeriesID); err != nil {
			return nil, err
		}
		return window(out, q.Offset, q.Limit), nil
	}

	// The series in the order their episodes were last played.
	iq := scope.query()
	iq.Kinds = []core.ItemKind{core.KindEpisode}
	iq.Sort = []core.SortSpec{{Field: core.SortLastPlayed, Desc: true}}
	iq.Limit = pageSize
	seen := map[core.ID]bool{}
	items := newCache(b.store)
	for iq.Offset = 0; len(out) < want; iq.Offset += pageSize {
		page, err := b.store.Items().Query(ctx, iq)
		if err != nil {
			return nil, fmt.Errorf("query played episodes: %w", err)
		}
		data, err := b.store.UserData().GetMany(ctx, scope.UserID, ids(page.Items))
		if err != nil {
			return nil, fmt.Errorf("get user data: %w", err)
		}
		for _, ep := range page.Items {
			last := data[ep.ID].LastPlayedAt
			if last == nil || last.Before(q.Since) {
				return window(out, q.Offset, q.Limit), nil
			}
			series, err := items.series(ctx, &ep)
			if err != nil {
				return nil, err
			}
			if series == nil || seen[series.ID] {
				continue
			}
			seen[series.ID] = true
			if err := add(series.ID); err != nil {
				return nil, err
			}
			if len(out) >= want {
				break
			}
		}
		if len(page.Items) < pageSize {
			break
		}
	}
	return window(out, q.Offset, q.Limit), nil
}

// nextEpisode returns the next episode of a series in the scope, or nil.
func (b *Browser) nextEpisode(ctx context.Context, scope Scope, series core.ID, includeResumable bool) (*core.Item, error) {
	iq := scope.query()
	iq.UserID = core.NilID
	iq.ParentID, iq.Recursive, iq.Kinds = series, true, []core.ItemKind{core.KindEpisode}
	var episodes []core.Item
	for ep, err := range b.store.Items().Walk(ctx, iq) {
		if err != nil {
			return nil, fmt.Errorf("list episodes: %w", err)
		}
		episodes = append(episodes, ep)
	}
	data, err := b.store.UserData().GetMany(ctx, scope.UserID, ids(episodes))
	if err != nil {
		return nil, fmt.Errorf("get user data: %w", err)
	}
	return core.NextEpisode(episodes, data, includeResumable), nil
}

// cache looks up the ancestors of items once per list.
type cache struct {
	store core.Store
	items map[core.ID]*core.Item
}

func newCache(store core.Store) *cache { return &cache{store: store, items: map[core.ID]*core.Item{}} }

func (c *cache) get(ctx context.Context, id core.ID) (*core.Item, error) {
	if it, ok := c.items[id]; ok {
		return it, nil
	}
	it, err := c.store.Items().Get(ctx, id)
	switch {
	case errors.Is(err, core.ErrNotFound):
		c.items[id] = nil
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf("get item: %w", err)
	}
	c.items[id] = &it
	return &it, nil
}

// series returns the series an episode or season belongs to, or nil.
func (c *cache) series(ctx context.Context, it *core.Item) (*core.Item, error) {
	for id := it.ParentID; !id.IsZero(); {
		parent, err := c.get(ctx, id)
		if err != nil || parent == nil {
			return nil, err
		}
		if parent.Kind == core.KindSeries {
			return parent, nil
		}
		id = parent.ParentID
	}
	return nil, nil
}

// container returns what the latest items group an item into: the series
// of an episode or the album of a track; nil for other items.
func (c *cache) container(ctx context.Context, it *core.Item) (*core.Item, error) {
	switch it.Kind {
	case core.KindEpisode:
		return c.series(ctx, it)
	case core.KindTrack:
		if it.ParentID.IsZero() {
			return nil, nil
		}
		parent, err := c.get(ctx, it.ParentID)
		if err != nil || parent == nil || parent.Kind != core.KindMusicAlbum {
			return nil, err
		}
		return parent, nil
	}
	return nil, nil
}

func ids(items []core.Item) []core.ID {
	out := make([]core.ID, len(items))
	for i := range items {
		out[i] = items[i].ID
	}
	return out
}

// window returns the page of list at offset.
func window[T any](list []T, offset, limit int) []T {
	if offset >= len(list) {
		return nil
	}
	return slices.Clip(list[offset:min(len(list), offset+limit)])
}

func cmpOr[T any](list, def []T) []T {
	if len(list) == 0 {
		return def
	}
	return list
}
