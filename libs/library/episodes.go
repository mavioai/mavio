package library

import (
	"cmp"
	"context"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/metadata"
)

// EpisodeLister lists the seasons and episodes of series from an external
// source, such as an episode list plugin.
type EpisodeLister interface {
	Name() string
	// Episodes lists a series' seasons and episodes, numbered in the
	// lookup's display order; nil when the source does not know the
	// series.
	Episodes(ctx context.Context, l Lookup) (*EpisodeList, error)
}

// EpisodeList is what a source knows of a series' seasons and episodes.
// Seasons carry their number as IndexNumber, episodes theirs as
// ParentIndexNumber, IndexNumber and AbsoluteNumber.
type EpisodeList struct {
	// Provider is the external ID that identifies the listed episodes.
	Provider core.Provider
	Seasons  []metadata.Result
	Episodes []metadata.Result
}

// episodeListers returns the episode listers a library uses, in its
// order.
func (r *Refresher) episodeListers(lib core.Library) []EpisodeLister {
	if r.Episodes == nil {
		return nil
	}
	return ordered(r.Episodes(), lib.Providers.Episodes)
}

// slot is an episode's place in its series.
type slot struct{ season, episode int }

// listEpisodes ports Jellyfin's TmdbMissingEpisodeProvider: it gives a
// series virtual episodes for the episodes it lacks, as the library's
// options ask, or every episode when the series is monitored. Virtual
// episodes follow what the first lister knowing the series lists: they
// are updated, moved into the season of their number, and deleted once a
// file holds them, they age out, or they are no longer wanted. Seasons
// without a folder become virtual seasons, deleted once empty.
func (r *Refresher) listEpisodes(ctx context.Context, lib core.Library, series core.Item, l Lookup) error {
	opts := lib.VirtualEpisodes
	importMissing, importUnaired := opts.Missing || series.Monitored, opts.Unaired || series.Monitored
	tree, err := r.seriesTree(ctx, series.ID)
	if err != nil {
		return err
	}
	if !importMissing && !importUnaired {
		var stale []core.ID
		for _, it := range tree {
			if it.Virtual && it.Kind == core.KindEpisode {
				stale = append(stale, it.ID)
			}
		}
		return r.pruneVirtual(ctx, tree, stale)
	}
	var list *EpisodeList
	for _, p := range r.episodeListers(lib) {
		if list, err = p.Episodes(ctx, l); err != nil {
			r.logger().WarnContext(ctx, "episode lister failed", "lister", p.Name(), "series", series.ID, "err", err)
			continue
		}
		if list != nil {
			break
		}
	}
	if list == nil {
		return nil
	}

	now := r.now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	rec := reconciler{r: r, lib: lib, series: series, provider: list.Provider, now: now, seasons: map[int]core.Item{}, monitorNew: series.Monitored}
	if r.NewSeasonMonitored != nil {
		rec.monitorNew = r.NewSeasonMonitored(ctx, series)
	}
	// Every slot that has an episode, physical or virtual, so that none is
	// listed twice; the provider's IDs of the physical episodes, which
	// are the episodes whatever numbers their files have; and the virtual
	// episodes the lister made, to keep in step with it.
	taken := map[slot]bool{}
	physical := map[slot]*core.Item{}
	physicalIDs := map[string]bool{}
	type virtualEpisode struct {
		slot slot
		item core.Item
	}
	var ours []virtualEpisode
	var stale []core.ID
	for i := range tree {
		it := &tree[i]
		switch it.Kind {
		case core.KindSeason:
			if it.IndexNumber != nil {
				if have, ok := rec.seasons[*it.IndexNumber]; !ok || (have.Virtual && !it.Virtual) {
					rec.seasons[*it.IndexNumber] = *it
				}
			}
			continue
		case core.KindEpisode:
		default:
			continue
		}
		isOurs := it.Virtual && it.ExternalIDs[list.Provider] != ""
		if shouldPrune(*it, list.Provider, !importMissing, today, opts.GraceDays, opts.Specials) {
			stale = append(stale, it.ID)
			continue
		}
		if !it.Virtual {
			if id := it.ExternalIDs[list.Provider]; id != "" {
				physicalIDs[id] = true
			}
		}
		if it.ParentIndexNumber == nil || it.IndexNumber == nil {
			continue
		}
		last := *it.IndexNumber
		if !it.Virtual && it.IndexNumberEnd != nil && *it.IndexNumberEnd > last {
			last = *it.IndexNumberEnd
		}
		for n := *it.IndexNumber; n <= last; n++ {
			s := slot{*it.ParentIndexNumber, n}
			taken[s] = true
			if !it.Virtual {
				physical[s] = it
			}
		}
		if isOurs {
			ours = append(ours, virtualEpisode{slot{*it.ParentIndexNumber, *it.IndexNumber}, *it})
		}
	}
	// A virtual episode whose file has come, under its number or another,
	// goes.
	updatable := map[slot]core.Item{}
	for _, v := range ours {
		switch {
		case physical[v.slot] != nil, physicalIDs[v.item.ExternalIDs[list.Provider]]:
			stale = append(stale, v.item.ID)
		default:
			updatable[v.slot] = v.item
		}
	}

	seasonNames := map[int]metadata.Result{}
	for _, s := range list.Seasons {
		if s.Item.IndexNumber != nil {
			seasonNames[*s.Item.IndexNumber] = s
		}
	}
	listed := slices.Clone(list.Episodes)
	slices.SortStableFunc(listed, func(a, b metadata.Result) int {
		return cmp.Or(cmp.Compare(deref(a.Item.ParentIndexNumber), deref(b.Item.ParentIndexNumber)),
			cmp.Compare(deref(a.Item.IndexNumber), deref(b.Item.IndexNumber)))
	})
	for _, ep := range listed {
		if err := ctx.Err(); err != nil {
			return err
		}
		if ep.Item.ParentIndexNumber == nil || ep.Item.IndexNumber == nil {
			continue
		}
		s := slot{*ep.Item.ParentIndexNumber, *ep.Item.IndexNumber}
		// Physical episodes learn their absolute numbers from the list.
		if p := physical[s]; p != nil && ep.Item.AbsoluteNumber != nil && *p.IndexNumber == s.episode &&
			(p.AbsoluteNumber == nil || *p.AbsoluteNumber != *ep.Item.AbsoluteNumber) {
			p.AbsoluteNumber = ep.Item.AbsoluteNumber
			rec.save(*p)
		}
		if !shouldImportEpisode(ep.Item.PremiereDate, today, importUnaired, importMissing, s.season == 0, opts.Specials) {
			continue
		}
		if existing, ok := updatable[s]; ok {
			season, err := rec.season(ctx, s.season, seasonNames[s.season])
			if err != nil {
				return err
			}
			changed := updateVirtualEpisode(&existing, ep.Item)
			if existing.ParentID != season.ID {
				existing.ParentID, changed = season.ID, true
			}
			if a := ep.Item.AbsoluteNumber; a != nil && (existing.AbsoluteNumber == nil || *existing.AbsoluteNumber != *a) {
				existing.AbsoluteNumber, changed = a, true
			}
			if changed {
				rec.save(existing)
			}
			if err := rec.still(ctx, existing.ID, ep); err != nil {
				return err
			}
			continue
		}
		if taken[s] {
			continue
		}
		taken[s] = true
		// The slot is free, but the episode may not be: files in another
		// order hold it under another number.
		if id := ep.Item.ExternalIDs[list.Provider]; id != "" && physicalIDs[id] {
			continue
		}
		season, err := rec.season(ctx, s.season, seasonNames[s.season])
		if err != nil {
			return err
		}
		it := rec.virtualEpisode(ctx, season, ep.Item)
		rec.save(it)
		rec.stills = append(rec.stills, pendingStill{it.ID, ep})
	}
	rec.alignSeasonSortNames()
	if err := rec.flush(ctx); err != nil {
		return err
	}
	if len(stale) > 0 || rec.saved > 0 {
		if tree, err = r.seriesTree(ctx, series.ID); err != nil {
			return err
		}
	}
	return r.pruneVirtual(ctx, tree, stale)
}

// seriesTree returns the seasons and episodes of a series, virtual and
// missing ones too.
func (r *Refresher) seriesTree(ctx context.Context, seriesID core.ID) ([]core.Item, error) {
	var out []core.Item
	q := core.ItemQuery{
		ParentID: seriesID, Recursive: true, Kinds: []core.ItemKind{core.KindSeason, core.KindEpisode},
		IncludeVirtual: true, IncludeMissing: true,
	}
	for it, err := range r.Store.Items().Walk(ctx, q) {
		if err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, nil
}

// pruneVirtual deletes the stale virtual episodes of a series' tree, then
// its virtual seasons left without episodes.
func (r *Refresher) pruneVirtual(ctx context.Context, tree []core.Item, stale []core.ID) error {
	gone := map[core.ID]bool{}
	for _, id := range stale {
		gone[id] = true
	}
	occupied := map[core.ID]bool{}
	for _, it := range tree {
		if it.Kind == core.KindEpisode && !gone[it.ID] {
			occupied[it.ParentID] = true
		}
	}
	for _, it := range tree {
		if it.Kind == core.KindSeason && it.Virtual && !occupied[it.ID] && !gone[it.ID] {
			stale = append(stale, it.ID)
		}
	}
	if len(stale) == 0 {
		return nil
	}
	r.logger().InfoContext(ctx, "removing virtual items", "count", len(stale))
	return r.Store.Items().Delete(ctx, stale...)
}

// reconciler collects the items and images a series' episode listing
// writes.
type reconciler struct {
	r        *Refresher
	lib      core.Library
	series   core.Item
	provider core.Provider
	now      time.Time
	// seasons are the series' seasons by number, physical before virtual.
	seasons map[int]core.Item
	// monitorNew is whether new seasons but the specials are monitored.
	monitorNew bool
	pending    []core.Item
	stills     []pendingStill
	saved      int
}

type pendingStill struct {
	id core.ID
	ep metadata.Result
}

func (rc *reconciler) save(it core.Item) {
	if i := slices.IndexFunc(rc.pending, func(p core.Item) bool { return p.ID == it.ID }); i >= 0 {
		rc.pending[i] = it
		return
	}
	rc.pending = append(rc.pending, it)
}

// season returns the series' season with a number, making a virtual one
// when the series has none.
func (rc *reconciler) season(ctx context.Context, number int, listed metadata.Result) (core.Item, error) {
	if s, ok := rc.seasons[number]; ok {
		return s, nil
	}
	s := core.Item{
		ID: core.NewID(), LibraryID: rc.series.LibraryID, ParentID: rc.series.ID, Kind: core.KindSeason,
		Name: listed.Item.Name, Overview: listed.Item.Overview, PremiereDate: listed.Item.PremiereDate,
		ProductionYear: listed.Item.ProductionYear, ExternalIDs: listed.Item.ExternalIDs,
		IndexNumber: &number, Virtual: true, Monitored: rc.monitorNew && number != 0, DateAdded: rc.now,
	}
	if s.Name == "" {
		s.Name = seasonName(number)
	}
	if err := s.Validate(); err != nil {
		return core.Item{}, fmt.Errorf("virtual season %d of %s: %w", number, rc.series.ID, err)
	}
	rc.r.logger().InfoContext(ctx, "creating virtual season", "series", rc.series.Name, "season", number)
	rc.seasons[number] = s
	rc.save(s)
	return s, nil
}

// seasonName names a season without a name as scans name season folders.
func seasonName(number int) string {
	if number == 0 {
		return "Specials"
	}
	return "Season " + strconv.Itoa(number)
}

// virtualEpisode makes a virtual episode of what a lister knows of it,
// monitored with its season, as Sonarr's
// RefreshEpisodeService.GetMonitoredStatus: an episode 0 outside season 1
// is not.
func (rc *reconciler) virtualEpisode(ctx context.Context, season core.Item, ep core.Item) core.Item {
	monitored := season.Monitored && (*ep.IndexNumber != 0 || *ep.ParentIndexNumber == 1)
	it := core.Item{
		ID: core.NewID(), LibraryID: rc.series.LibraryID, ParentID: season.ID, Kind: core.KindEpisode,
		Name: ep.Name, Overview: ep.Overview, PremiereDate: ep.PremiereDate, ProductionYear: ep.ProductionYear,
		Runtime: ep.Runtime, ExternalIDs: ep.ExternalIDs,
		ParentIndexNumber: ep.ParentIndexNumber, IndexNumber: ep.IndexNumber, AbsoluteNumber: ep.AbsoluteNumber,
		Virtual: true, Monitored: monitored, DateAdded: rc.now,
	}
	if it.Name == "" {
		it.Name = "Episode " + strconv.Itoa(*ep.IndexNumber)
	}
	if it.ProductionYear == 0 && it.PremiereDate != nil {
		it.ProductionYear = it.PremiereDate.Year()
	}
	rc.r.logger().InfoContext(ctx, "creating virtual episode", "series", rc.series.Name, "season", *ep.ParentIndexNumber, "episode", *ep.IndexNumber)
	return it
}

// still records the primary image a lister gives a virtual episode that
// has none.
func (rc *reconciler) still(ctx context.Context, id core.ID, ep metadata.Result) error {
	images, err := rc.r.Store.Images().ListForOwner(ctx, id)
	if err != nil || slices.ContainsFunc(images, func(img core.Image) bool { return img.Kind == core.ImagePrimary }) {
		return err
	}
	rc.stills = append(rc.stills, pendingStill{id, ep})
	return nil
}

// flush writes the collected items, then the stills of new and imageless
// virtual episodes.
func (rc *reconciler) flush(ctx context.Context) error {
	if len(rc.pending) == 0 && len(rc.stills) == 0 {
		return nil
	}
	// Seasons go before the episodes in them.
	slices.SortStableFunc(rc.pending, func(a, b core.Item) int {
		return cmp.Compare(kindRank(a.Kind), kindRank(b.Kind))
	})
	return rc.r.Store.InTx(ctx, func(tx core.Store) error {
		if len(rc.pending) > 0 {
			if err := tx.Items().Upsert(ctx, rc.pending...); err != nil {
				return err
			}
			rc.saved = len(rc.pending)
		}
		for _, s := range rc.stills {
			for _, img := range s.ep.RemoteImages {
				if img.Kind == core.ImagePrimary && img.URL != "" {
					still := core.Image{ID: core.NewID(), OwnerID: s.id, Kind: core.ImagePrimary, RemoteURL: img.URL, Width: img.Width, Height: img.Height}
					if err := tx.Images().Replace(ctx, s.id, []core.Image{still}); err != nil {
						return err
					}
					break
				}
			}
		}
		return nil
	})
}

func kindRank(k core.ItemKind) int {
	if k == core.KindSeason {
		return 0
	}
	return 1
}

// alignSeasonSortNames gives virtual seasons the sort names of the
// physical seasons' convention, so that they sort among them by number.
func (rc *reconciler) alignSeasonSortNames() {
	seasons := make([]core.Item, 0, len(rc.seasons))
	for _, n := range slices.Sorted(maps.Keys(rc.seasons)) {
		seasons = append(seasons, rc.seasons[n])
	}
	template := seasonSortNameTemplate(seasons)
	if template == nil {
		return
	}
	for _, s := range seasons {
		if !s.Virtual || s.IndexNumber == nil {
			continue
		}
		if want := template(*s.IndexNumber); s.SortName != want {
			s.SortName = want
			rc.seasons[*s.IndexNumber] = s
			rc.save(s)
		}
	}
}

func deref(p *int) int {
	if p == nil {
		return -1
	}
	return *p
}

// shouldImportEpisode ports ShouldImportEpisode: whether a listed episode
// becomes a virtual episode. Episodes without an air date never do;
// those airing today or later need importUnaired, aired ones
// importMissing, and specials importSpecials.
func shouldImportEpisode(premiere *time.Time, today time.Time, importUnaired, importMissing, isSpecial, importSpecials bool) bool {
	if premiere == nil {
		return false
	}
	if isSpecial && !importSpecials {
		return false
	}
	if !dateOf(*premiere).Before(today) {
		return importUnaired
	}
	return importMissing
}

// shouldPrune ports ShouldPrune: whether a virtual episode listed by the
// provider's lister goes. Specials go unless importSpecials; others, when
// pruneAgedOut, once they aired more than graceDays ago.
func shouldPrune(ep core.Item, provider core.Provider, pruneAgedOut bool, today time.Time, graceDays int, importSpecials bool) bool {
	if !ep.Virtual || ep.ExternalIDs[provider] == "" {
		return false
	}
	if ep.ParentIndexNumber != nil && *ep.ParentIndexNumber == 0 && !importSpecials {
		return true
	}
	return pruneAgedOut && ep.PremiereDate != nil && dateOf(*ep.PremiereDate).Before(today.AddDate(0, 0, -graceDays))
}

// dateOf returns the UTC calendar day of t.
func dateOf(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// updateVirtualEpisode ports UpdateVirtualEpisode: a virtual episode takes
// the name, overview and air date the lister gives, when given, and
// reports whether it changed.
func updateVirtualEpisode(it *core.Item, listed core.Item) bool {
	changed := false
	if listed.Name != "" && it.Name != listed.Name {
		it.Name, changed = listed.Name, true
	}
	if listed.Overview != "" && it.Overview != listed.Overview {
		it.Overview, changed = listed.Overview, true
	}
	if listed.PremiereDate != nil && (it.PremiereDate == nil || !it.PremiereDate.Equal(*listed.PremiereDate)) {
		it.PremiereDate, it.ProductionYear, changed = listed.PremiereDate, listed.PremiereDate.Year(), true
	}
	return changed
}

// seasonSortNameTemplate ports BuildSeasonSortNameTemplate: when a
// physical season has a sort name of its own, such as "Season 01" from
// its NFO file, it returns the sort name of a season number in the same
// convention, swapping the last run of digits; nil otherwise.
func seasonSortNameTemplate(seasons []core.Item) func(int) string {
	i := slices.IndexFunc(seasons, func(s core.Item) bool {
		return !s.Virtual && s.IndexNumber != nil && s.SortName != "" && s.SortName != s.Name
	})
	if i < 0 {
		return nil
	}
	forced := []rune(seasons[i].SortName)
	start, end := -1, -1
	for j := len(forced) - 1; j >= 0; j-- {
		if unicode.IsDigit(forced[j]) {
			if end < 0 {
				end = j
			}
			start = j
		} else if end >= 0 {
			break
		}
	}
	if end < 0 {
		return nil
	}
	prefix, suffix, width := string(forced[:start]), string(forced[end+1:]), end-start+1
	return func(n int) string {
		digits := strconv.Itoa(n)
		return prefix + strings.Repeat("0", max(0, width-len(digits))) + digits + suffix
	}
}
