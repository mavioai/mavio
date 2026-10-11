package acquisition

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

// SeriesMonitor is which episodes of a series to monitor, as Sonarr's
// MonitorTypes.
type SeriesMonitor string

// The series monitoring options.
const (
	// MonitorAll monitors every episode but the specials.
	MonitorAll SeriesMonitor = "all"
	// MonitorFuture monitors the episodes not aired yet.
	MonitorFuture SeriesMonitor = "future"
	// MonitorMissing monitors the episodes without a file.
	MonitorMissing SeriesMonitor = "missing"
	// MonitorExisting monitors the episodes with a file.
	MonitorExisting SeriesMonitor = "existing"
	// MonitorFirstSeason monitors the episodes of the first season.
	MonitorFirstSeason SeriesMonitor = "first_season"
	// MonitorLastSeason monitors the episodes of the last season.
	MonitorLastSeason SeriesMonitor = "last_season"
	// MonitorPilot monitors the first episode of the first season.
	MonitorPilot SeriesMonitor = "pilot"
	// MonitorRecent monitors the episodes aired in the last 90 days and
	// those to come.
	MonitorRecent SeriesMonitor = "recent"
	// MonitorSpecials monitors the specials, leaving the other episodes.
	MonitorSpecials SeriesMonitor = "monitor_specials"
	// UnmonitorSpecials unmonitors the specials, leaving the other
	// episodes.
	UnmonitorSpecials SeriesMonitor = "unmonitor_specials"
	// MonitorNone monitors no episode.
	MonitorNone SeriesMonitor = "none"
	// MonitorSkip leaves the episodes as their seasons' monitoring made
	// them.
	MonitorSkip SeriesMonitor = "skip"
)

// NewSeries is a series to add.
type NewSeries struct {
	LibraryID        core.ID
	TvdbID           int
	QualityProfileID core.ID
	// SeriesType is the series' type; nil for daily when the scene
	// mapping providers list the series as daily, else standard.
	SeriesType      *core.SeriesType
	SeasonFolder    bool
	Monitor         SeriesMonitor
	MonitorNewItems core.NewItemMonitor
	Tags            []string
	// Search searches for the monitored episodes without a file once
	// added.
	Search bool
}

// AddSeries adds a series to a show library by its TVDB ID, as Sonarr's
// AddSeriesService and its handlers: a virtual monitored series wanted
// with a quality profile, identified by the library's metadata
// providers, which list its episodes, then monitored as the monitor
// option says. A library that has the series fails with
// core.ErrConflict. A series the providers fail to identify is kept,
// named by its ID, for a later refresh.
func (r *Runtime) AddSeries(ctx context.Context, n NewSeries) (core.Item, core.Wanted, error) {
	lib, err := r.Store.Libraries().Get(ctx, n.LibraryID)
	if err != nil {
		return core.Item{}, core.Wanted{}, err
	}
	if lib.Kind != core.LibraryShows {
		return core.Item{}, core.Wanted{}, fmt.Errorf("%w: library %s is not a show library", core.ErrInvalid, lib.Name)
	}
	if n.TvdbID <= 0 {
		return core.Item{}, core.Wanted{}, fmt.Errorf("%w: TVDB ID %d", core.ErrInvalid, n.TvdbID)
	}
	if n.MonitorNewItems != "" && !n.MonitorNewItems.Valid() {
		return core.Item{}, core.Wanted{}, fmt.Errorf("%w: new item monitoring %q", core.ErrInvalid, n.MonitorNewItems)
	}
	if err := r.checkProfile(ctx, n.QualityProfileID, core.KindSeries); err != nil {
		return core.Item{}, core.Wanted{}, err
	}
	tvdb := strconv.Itoa(n.TvdbID)
	for it, err := range r.Store.Items().Walk(ctx, core.ItemQuery{
		LibraryIDs: []core.ID{lib.ID}, Kinds: []core.ItemKind{core.KindSeries}, IncludeVirtual: true, IncludeMissing: true,
	}) {
		if err != nil {
			return core.Item{}, core.Wanted{}, err
		}
		if it.ExternalIDs[core.ProviderTVDB] == tvdb {
			return core.Item{}, core.Wanted{}, fmt.Errorf("%w: library %s has the series with TVDB ID %d", core.ErrConflict, lib.Name, n.TvdbID)
		}
	}
	typ := core.SeriesStandard
	switch {
	case n.SeriesType != nil:
		typ = *n.SeriesType
	case r.isDaily(ctx, n.TvdbID):
		typ = core.SeriesDaily
	}
	now := r.now()
	it := core.Item{
		ID: core.NewID(), LibraryID: lib.ID, Kind: core.KindSeries, Name: "TVDB " + tvdb, DateAdded: now, SeriesType: typ,
		Virtual: true, Monitored: true, ExternalIDs: map[core.Provider]string{core.ProviderTVDB: tvdb},
	}
	if err := r.Store.Items().Upsert(ctx, it); err != nil {
		return core.Item{}, core.Wanted{}, err
	}
	// The wanted state goes first, for the refresh listing the episodes to
	// monitor new seasons as it says.
	w := core.Wanted{
		ItemID: it.ID, QualityProfileID: n.QualityProfileID, MonitorNewItems: n.MonitorNewItems, SeasonFolder: n.SeasonFolder,
		Tags: n.Tags, AddedAt: now,
	}
	if w.MonitorNewItems == "" {
		w.MonitorNewItems = core.NewItemsAll
	}
	if err := r.Store.Wanted().Put(ctx, &w); err != nil {
		return core.Item{}, core.Wanted{}, err
	}
	if r.Identifier != nil {
		identified, err := r.Identifier.Identify(ctx, lib, it.ID, map[core.Provider]string{core.ProviderTVDB: tvdb})
		if err != nil {
			if ctx.Err() != nil {
				return core.Item{}, core.Wanted{}, ctx.Err()
			}
			r.logger().WarnContext(ctx, "couldn't identify added series", "item", it.ID, "tvdb", tvdb, "err", err)
		} else {
			it = identified
		}
	}
	if it.SeriesType != typ && (n.SeriesType != nil || typ == core.SeriesDaily) {
		it.SeriesType = typ
		if err := r.Store.Items().Upsert(ctx, it); err != nil {
			return core.Item{}, core.Wanted{}, err
		}
	}
	if err := r.monitorEpisodes(ctx, it, cmpMonitor(n.Monitor)); err != nil {
		return core.Item{}, core.Wanted{}, err
	}
	if err := r.updateSceneNumbering(ctx, it); err != nil {
		if ctx.Err() != nil {
			return core.Item{}, core.Wanted{}, ctx.Err()
		}
		r.logger().WarnContext(ctx, "couldn't update scene numbering", "item", it.ID, "err", err)
	}
	r.logger().InfoContext(ctx, "added series", "item", it.ID, "name", it.Name, "tvdb", tvdb)
	if n.Search {
		if _, err := r.QueueEpisodeSearch(ctx, EpisodeSearchPayload{Scope: ScopeMissing, SeriesID: it.ID}); err != nil {
			return it, w, err
		}
	}
	return it, w, nil
}

func cmpMonitor(m SeriesMonitor) SeriesMonitor {
	if m == "" {
		return MonitorAll
	}
	return m
}

// MonitorSeries applies a monitoring option to the seasons and episodes
// of wanted series, and changes their monitoring of new items unless
// newItems is empty, as Sonarr's season pass.
func (r *Runtime) MonitorSeries(ctx context.Context, seriesIDs []core.ID, monitor SeriesMonitor, newItems core.NewItemMonitor) error {
	if newItems != "" && !newItems.Valid() {
		return fmt.Errorf("%w: new item monitoring %q", core.ErrInvalid, newItems)
	}
	for _, id := range seriesIDs {
		w, err := r.Store.Wanted().Get(ctx, id)
		if err != nil {
			return fmt.Errorf("wanted series %s: %w", id, err)
		}
		it, err := r.Store.Items().Get(ctx, id)
		if err != nil {
			return err
		}
		if it.Kind != core.KindSeries {
			return fmt.Errorf("%w: item %s is not a series", core.ErrInvalid, id)
		}
		if newItems != "" && newItems != w.MonitorNewItems {
			w.MonitorNewItems = newItems
			if err := r.Store.Wanted().Put(ctx, &w); err != nil {
				return err
			}
		}
		if err := r.monitorEpisodes(ctx, it, monitor); err != nil {
			return err
		}
	}
	return nil
}

// tree returns the seasons and episodes of a series.
func (r *Runtime) tree(ctx context.Context, seriesID core.ID) ([]core.Item, error) {
	var out []core.Item
	for it, err := range r.Store.Items().Walk(ctx, core.ItemQuery{
		ParentID: seriesID, Recursive: true, Kinds: []core.ItemKind{core.KindSeason, core.KindEpisode},
		IncludeVirtual: true, IncludeMissing: true,
	}) {
		if err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, nil
}

// monitorEpisodes monitors the episodes of a series as a monitoring
// option says, and its seasons by their episodes, as Sonarr's
// EpisodeMonitoredService.SetEpisodeMonitoredStatus. A multi-episode
// file counts as its first episode.
func (r *Runtime) monitorEpisodes(ctx context.Context, s core.Item, monitor SeriesMonitor) error {
	if monitor == MonitorSkip {
		return nil
	}
	tree, err := r.tree(ctx, s.ID)
	if err != nil {
		return err
	}
	var seasonNumbers []int
	for _, it := range tree {
		if it.Kind == core.KindSeason && it.IndexNumber != nil {
			seasonNumbers = append(seasonNumbers, *it.IndexNumber)
		}
	}
	firstSeason, lastSeason := 0, 0
	for _, n := range seasonNumbers {
		if n > 0 && (firstSeason == 0 || n < firstSeason) {
			firstSeason = n
		}
		lastSeason = max(lastSeason, n)
	}
	now := r.now()
	aired := func(it core.Item) bool { return it.PremiereDate != nil && it.PremiereDate.Before(now) }
	var predicate func(season, episode int, it core.Item) bool
	only := -1 // the season whose episodes alone change, -1 for all
	switch monitor {
	case MonitorAll:
		predicate = func(season, _ int, _ core.Item) bool { return season > 0 }
	case MonitorFuture:
		predicate = func(season, _ int, it core.Item) bool {
			return season > 0 && (it.PremiereDate == nil || !it.PremiereDate.Before(now))
		}
	case MonitorMissing:
		predicate = func(season, _ int, it core.Item) bool { return season > 0 && !hasFile(it) }
	case MonitorExisting:
		predicate = func(season, _ int, it core.Item) bool { return season > 0 && hasFile(it) }
	case MonitorPilot:
		predicate = func(season, episode int, _ core.Item) bool {
			return season > 0 && season == firstSeason && episode == 1
		}
	case MonitorFirstSeason:
		predicate = func(season, _ int, _ core.Item) bool { return season > 0 && season == firstSeason }
	case MonitorLastSeason:
		predicate = func(season, _ int, _ core.Item) bool { return season > 0 && season == lastSeason }
	case MonitorRecent:
		predicate = func(season, _ int, it core.Item) bool {
			return season > 0 && (it.PremiereDate == nil || (aired(it) && now.Sub(*it.PremiereDate) <= 90*24*time.Hour) || it.PremiereDate.After(now))
		}
	case MonitorSpecials:
		only, predicate = 0, func(int, int, core.Item) bool { return true }
	case UnmonitorSpecials:
		only, predicate = 0, func(int, int, core.Item) bool { return false }
	case MonitorNone:
		predicate = func(int, int, core.Item) bool { return false }
	default:
		return fmt.Errorf("%w: series monitoring %q", core.ErrInvalid, monitor)
	}
	var changed []core.Item
	monitoredSeasons := map[int]bool{}
	for i := range tree {
		it := &tree[i]
		if it.Kind != core.KindEpisode || it.ParentIndexNumber == nil || it.IndexNumber == nil {
			continue
		}
		season := *it.ParentIndexNumber
		monitored := it.Monitored
		if only < 0 || season == only {
			monitored = predicate(season, *it.IndexNumber, *it)
		}
		if monitored != it.Monitored {
			it.Monitored = monitored
			changed = append(changed, *it)
		}
		if monitored {
			monitoredSeasons[season] = true
		}
	}
	continuing := s.SeriesStatus == core.SeriesContinuing || s.SeriesStatus == core.SeriesUnreleased
	for i := range tree {
		it := &tree[i]
		if it.Kind != core.KindSeason || it.IndexNumber == nil {
			continue
		}
		n := *it.IndexNumber
		var monitored bool
		switch {
		case n > 0 && n == lastSeason && (monitor == MonitorAll || (monitor == MonitorFuture && continuing)):
			monitored = true
		case n == firstSeason && monitor == MonitorPilot:
			monitored = false
		default:
			monitored = monitoredSeasons[n]
		}
		if monitored != it.Monitored {
			it.Monitored = monitored
			changed = append(changed, *it)
		}
	}
	for _, it := range changed {
		if err := r.Store.Items().Upsert(ctx, it); err != nil {
			return err
		}
	}
	r.logger().DebugContext(ctx, "monitored episodes", "series", s.Name, "monitor", monitor, "changed", len(changed))
	return nil
}

// SetMonitored monitors or unmonitors seasons and episodes; a season
// takes its episodes along, as Sonarr's SeriesService does when a
// season's monitoring changes.
func (r *Runtime) SetMonitored(ctx context.Context, itemIDs []core.ID, monitored bool) error {
	for _, id := range itemIDs {
		it, err := r.Store.Items().Get(ctx, id)
		if err != nil {
			return err
		}
		items := []core.Item{it}
		switch it.Kind {
		case core.KindEpisode:
		case core.KindSeason:
			for ep, err := range r.Store.Items().Walk(ctx, core.ItemQuery{
				ParentID: id, Kinds: []core.ItemKind{core.KindEpisode}, IncludeVirtual: true, IncludeMissing: true,
			}) {
				if err != nil {
					return err
				}
				items = append(items, ep)
			}
		default:
			return fmt.Errorf("%w: item %s is not a season or an episode", core.ErrInvalid, id)
		}
		for _, it := range items {
			if it.Monitored == monitored {
				continue
			}
			it.Monitored = monitored
			if err := r.Store.Items().Upsert(ctx, it); err != nil {
				return err
			}
		}
	}
	return nil
}

// NewSeasonMonitored tells whether the seasons a refresh or a scan adds
// to a series are monitored, for library.Refresher and library.Scanner: as a wanted series' monitoring of
// new items says, as Sonarr's RefreshSeriesService; for other series,
// when the series is monitored.
func (r *Runtime) NewSeasonMonitored(ctx context.Context, s core.Item) bool {
	w, err := r.Store.Wanted().Get(ctx, s.ID)
	if err != nil {
		if !errors.Is(err, core.ErrNotFound) {
			r.logger().WarnContext(ctx, "couldn't read wanted series", "item", s.ID, "err", err)
		}
		return s.Monitored
	}
	return w.MonitorNewItems != core.NewItemsNone
}

// UnmonitorMissing unmonitors the episodes of wanted series among the
// items a scan of lib marked missing, when the settings say to, as
// Sonarr's EpisodeService does with AutoUnmonitorPreviouslyDownloadedEpisodes
// for episode files missing from disk; for library.Scanner.OnMissing.
func (r *Runtime) UnmonitorMissing(ctx context.Context, lib core.Library, ids []core.ID) {
	if lib.Kind != core.LibraryShows || !r.settings().AutoUnmonitorPreviouslyDownloadedEpisodes {
		return
	}
	if err := r.unmonitorMissing(ctx, ids); err != nil {
		r.logger().WarnContext(ctx, "couldn't unmonitor missing episodes", "library", lib.Name, "err", err)
	}
}

func (r *Runtime) unmonitorMissing(ctx context.Context, ids []core.ID) error {
	wanted := map[core.ID]bool{}
	isWanted := func(id core.ID) (bool, error) {
		if w, ok := wanted[id]; ok {
			return w, nil
		}
		_, err := r.Store.Wanted().Get(ctx, id)
		switch {
		case errors.Is(err, core.ErrNotFound):
			wanted[id] = false
		case err != nil:
			return false, err
		default:
			wanted[id] = true
		}
		return wanted[id], nil
	}
	for _, id := range ids {
		it, err := r.Store.Items().Get(ctx, id)
		if errors.Is(err, core.ErrNotFound) {
			continue
		}
		if err != nil {
			return err
		}
		if it.Kind != core.KindEpisode || !it.Monitored {
			continue
		}
		seriesID := it.ParentID
		if parent, err := r.Store.Items().Get(ctx, seriesID); err != nil && !errors.Is(err, core.ErrNotFound) {
			return err
		} else if err == nil && parent.Kind == core.KindSeason {
			seriesID = parent.ParentID
		}
		ok, err := isWanted(seriesID)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		it.Monitored = false
		if err := r.Store.Items().Upsert(ctx, it); err != nil {
			return err
		}
		r.logger().InfoContext(ctx, "unmonitored episode whose file went missing", "item", it.ID, "path", it.Path)
	}
	return nil
}

// seasonsOf returns the distinct season numbers of episodes.
func seasonsOf(numbers []int) []int {
	out := slices.Clone(numbers)
	slices.Sort(out)
	return slices.Compact(out)
}
