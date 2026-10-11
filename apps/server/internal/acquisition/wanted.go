package acquisition

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/mavioai/mavio/libs/acquire"
	"github.com/mavioai/mavio/libs/core"
)

// NewMovie is a movie to add.
type NewMovie struct {
	LibraryID           core.ID
	TmdbID              int
	QualityProfileID    core.ID
	MinimumAvailability core.MovieAvailability
	Tags                []string
	// Search searches for the movie once added.
	Search bool
}

// AddMovie adds a movie to a movie library by its TMDB ID, as Radarr's
// AddMovieService: a virtual monitored movie identified by the library's
// metadata providers and wanted with a quality profile. A library that
// has the movie fails with core.ErrConflict. A movie the providers fail
// to identify is kept, named by its ID, for a later refresh.
func (r *Runtime) AddMovie(ctx context.Context, n NewMovie) (core.Item, core.Wanted, error) {
	lib, err := r.Store.Libraries().Get(ctx, n.LibraryID)
	if err != nil {
		return core.Item{}, core.Wanted{}, err
	}
	if lib.Kind != core.LibraryMovies {
		return core.Item{}, core.Wanted{}, fmt.Errorf("%w: library %s is not a movie library", core.ErrInvalid, lib.Name)
	}
	if n.TmdbID <= 0 {
		return core.Item{}, core.Wanted{}, fmt.Errorf("%w: TMDB ID %d", core.ErrInvalid, n.TmdbID)
	}
	if err := r.checkProfile(ctx, n.QualityProfileID); err != nil {
		return core.Item{}, core.Wanted{}, err
	}
	tmdb := strconv.Itoa(n.TmdbID)
	for it, err := range r.Store.Items().Walk(ctx, core.ItemQuery{
		LibraryIDs: []core.ID{lib.ID}, Kinds: []core.ItemKind{core.KindMovie}, IncludeVirtual: true, IncludeMissing: true,
	}) {
		if err != nil {
			return core.Item{}, core.Wanted{}, err
		}
		if it.ExternalIDs[core.ProviderTMDB] == tmdb {
			return core.Item{}, core.Wanted{}, fmt.Errorf("%w: library %s has the movie with TMDB ID %d", core.ErrConflict, lib.Name, n.TmdbID)
		}
	}
	now := r.now()
	it := core.Item{
		ID: core.NewID(), LibraryID: lib.ID, Kind: core.KindMovie, Name: "TMDB " + tmdb, DateAdded: now,
		Virtual: true, Monitored: true, ExternalIDs: map[core.Provider]string{core.ProviderTMDB: tmdb},
	}
	if err := r.Store.Items().Upsert(ctx, it); err != nil {
		return core.Item{}, core.Wanted{}, err
	}
	if r.Identifier != nil {
		identified, err := r.Identifier.Identify(ctx, lib, it.ID, map[core.Provider]string{core.ProviderTMDB: tmdb})
		if err != nil {
			if ctx.Err() != nil {
				return core.Item{}, core.Wanted{}, ctx.Err()
			}
			r.logger().WarnContext(ctx, "couldn't identify added movie", "item", it.ID, "tmdb", tmdb, "err", err)
		} else {
			it = identified
		}
	}
	w := core.Wanted{
		ItemID: it.ID, QualityProfileID: n.QualityProfileID, MinimumAvailability: n.MinimumAvailability,
		Tags: n.Tags, AddedAt: now,
	}
	if w.MinimumAvailability == "" {
		w.MinimumAvailability = core.AvailabilityReleased
	}
	if err := r.Store.Wanted().Put(ctx, &w); err != nil {
		return core.Item{}, core.Wanted{}, err
	}
	r.logger().InfoContext(ctx, "added movie", "item", it.ID, "name", it.Name, "tmdb", tmdb)
	if n.Search {
		if _, err := r.QueueSearch(ctx, []core.ID{it.ID}, false); err != nil {
			return it, w, err
		}
	}
	return it, w, nil
}

// checkProfile fails with core.ErrInvalid for a profile that is not a
// movie profile.
func (r *Runtime) checkProfile(ctx context.Context, id core.ID) error {
	p, err := r.Store.QualityProfiles().Get(ctx, id)
	if errors.Is(err, core.ErrNotFound) {
		return fmt.Errorf("%w: quality profile %s does not exist", core.ErrInvalid, id)
	}
	if err != nil {
		return err
	}
	if p.Kind != core.KindMovie {
		return fmt.Errorf("%w: quality profile %s is not for movies", core.ErrInvalid, p.Name)
	}
	return nil
}

// WantedItem is a wanted movie with its item's monitored flag.
type WantedItem struct {
	core.Wanted
	Monitored bool
}

// GetWanted returns how a movie is acquired.
func (r *Runtime) GetWanted(ctx context.Context, itemID core.ID) (WantedItem, error) {
	w, err := r.Store.Wanted().Get(ctx, itemID)
	if err != nil {
		return WantedItem{}, err
	}
	it, err := r.Store.Items().Get(ctx, itemID)
	if err != nil {
		return WantedItem{}, err
	}
	return WantedItem{Wanted: w, Monitored: it.Monitored}, nil
}

// UpdateWanted changes how a movie is acquired: its profile, minimum
// availability and tags, and its item's monitored flag. The times are
// the server's.
func (r *Runtime) UpdateWanted(ctx context.Context, u WantedItem) (WantedItem, error) {
	w, err := r.Store.Wanted().Get(ctx, u.ItemID)
	if err != nil {
		return WantedItem{}, err
	}
	if u.QualityProfileID != w.QualityProfileID {
		if err := r.checkProfile(ctx, u.QualityProfileID); err != nil {
			return WantedItem{}, err
		}
	}
	w.QualityProfileID, w.Tags = u.QualityProfileID, u.Tags
	w.MinimumAvailability = cmp.Or(u.MinimumAvailability, core.AvailabilityReleased)
	if err := r.Store.Wanted().Put(ctx, &w); err != nil {
		return WantedItem{}, err
	}
	it, err := r.Store.Items().Get(ctx, u.ItemID)
	if err != nil {
		return WantedItem{}, err
	}
	if it.Monitored != u.Monitored {
		it.Monitored = u.Monitored
		if err := r.Store.Items().Upsert(ctx, it); err != nil {
			return WantedItem{}, err
		}
	}
	return WantedItem{Wanted: w, Monitored: it.Monitored}, nil
}

// WantedFilter selects monitored movies.
type WantedFilter int

// The wanted filters.
const (
	// WantedMissing selects the movies without a file.
	WantedMissing WantedFilter = iota
	// WantedCutoffUnmet selects those whose file has not met their
	// profile's cutoff.
	WantedCutoffUnmet
)

// WantedMovie is a monitored movie a filter selects.
type WantedMovie struct {
	Item   core.Item
	Wanted core.Wanted
	// Quality is the title of its file's quality; empty without a file.
	Quality string
	// Available is whether it is available by its minimum availability.
	Available bool
}

// ListWanted returns the monitored movies a filter selects, by name, a
// page of them and how many there are, as Radarr's missing and cutoff
// unmet lists.
func (r *Runtime) ListWanted(ctx context.Context, filter WantedFilter, limit, offset int) ([]WantedMovie, int, error) {
	all, err := r.wantedMovies(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	if limit <= 0 || limit > core.MaxPageSize {
		limit = core.MaxPageSize
	}
	total := len(all)
	if offset >= total {
		return nil, total, nil
	}
	return all[offset:min(offset+limit, total)], total, nil
}

// wantedMovies returns every monitored movie a filter selects, by name.
func (r *Runtime) wantedMovies(ctx context.Context, filter WantedFilter) ([]WantedMovie, error) {
	rn, err := r.newRun(ctx)
	if err != nil {
		return nil, err
	}
	wanted, err := r.Store.Wanted().List(ctx)
	if err != nil {
		return nil, err
	}
	upgrades := acquire.Upgrades{Propers: rn.env.Config.Propers}
	now := r.now()
	var out []WantedMovie
	for _, w := range wanted {
		m, err := rn.movie(ctx, w.ItemID)
		if errors.Is(err, core.ErrNotFound) || errors.Is(err, core.ErrInvalid) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if !m.item.Monitored {
			continue
		}
		f := m.acq.File
		switch filter {
		case WantedMissing:
			if f != nil {
				continue
			}
		case WantedCutoffUnmet:
			if f == nil || !upgrades.CutoffNotMet(m.target.Profile, f.Quality, f.Formats, nil) {
				continue
			}
		}
		wm := WantedMovie{Item: m.item, Wanted: w, Available: m.acq.Availability.Available(now, rn.settings.AvailabilityDelay)}
		if f != nil {
			wm.Quality = rn.qualityTitle(f.Quality.Quality)
		}
		out = append(out, wm)
	}
	slices.SortStableFunc(out, func(a, b WantedMovie) int {
		return cmp.Or(cmp.Compare(strings.ToLower(sortName(a.Item)), strings.ToLower(sortName(b.Item))), cmp.Compare(a.Item.ID.String(), b.Item.ID.String()))
	})
	return out, nil
}

func sortName(it core.Item) string { return cmp.Or(it.SortName, it.Name) }

// SearchWanted queues searches for movies, or for every monitored movie
// a filter selects when none are given, and reports how many were
// queued; searches of given movies are a user's, which search
// unmonitored movies too.
func (r *Runtime) SearchWanted(ctx context.Context, itemIDs []core.ID, filter WantedFilter) (int, error) {
	if len(itemIDs) > 0 {
		for _, id := range itemIDs {
			if _, err := r.Store.Wanted().Get(ctx, id); err != nil {
				return 0, fmt.Errorf("wanted movie %s: %w", id, err)
			}
		}
		return r.QueueSearch(ctx, itemIDs, true)
	}
	movies, err := r.wantedMovies(ctx, filter)
	if err != nil {
		return 0, err
	}
	ids := make([]core.ID, 0, len(movies))
	for _, m := range movies {
		if m.Available {
			ids = append(ids, m.Item.ID)
		}
	}
	return r.QueueSearch(ctx, ids, false)
}
