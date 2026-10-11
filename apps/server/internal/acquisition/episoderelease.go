package acquisition

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/mavioai/mavio/apps/server/internal/releases"
	"github.com/mavioai/mavio/libs/acquire"
	"github.com/mavioai/mavio/libs/acquire/series"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/release"
)

// isEpisodeKind reports whether items of a kind are of series.
func isEpisodeKind(k core.ItemKind) bool {
	return k == core.KindSeries || k == core.KindSeason || k == core.KindEpisode
}

// episodeScope is an item of a wanted series a user searches for: the
// series, a season or an episode.
type episodeScope struct {
	show *show
	kind core.ItemKind
	// season is the season's number, of seasons and episodes.
	season int
	// episodes are the season's episodes, or the episodes an episode's
	// item holds; nil for the series.
	episodes []*series.Episode
}

// episodeScope resolves an item of a wanted series. Items of no wanted
// series fail with ErrNotWanted.
func (rn *run) episodeScope(ctx context.Context, it core.Item) (*episodeScope, error) {
	seriesID := it.ID
	for range 2 {
		if seriesID.IsZero() {
			break
		}
		p, err := rn.rt.Store.Items().Get(ctx, seriesID)
		if err != nil {
			return nil, err
		}
		if p.Kind == core.KindSeries {
			break
		}
		seriesID = p.ParentID
	}
	sh, err := rn.show(ctx, seriesID)
	if errors.Is(err, core.ErrNotFound) || errors.Is(err, core.ErrInvalid) {
		return nil, ErrNotWanted
	}
	if err != nil {
		return nil, err
	}
	sc := &episodeScope{show: sh, kind: it.Kind}
	switch it.Kind {
	case core.KindSeason:
		if it.IndexNumber == nil {
			return nil, fmt.Errorf("%w: season %s has no number", core.ErrInvalid, it.ID)
		}
		sc.season = *it.IndexNumber
		sc.episodes = seasonEpisodes(sh, sc.season)
	case core.KindEpisode:
		for _, ep := range sh.episodes {
			if sh.items[ep.ID].ID == it.ID {
				sc.episodes = append(sc.episodes, ep)
			}
		}
		if len(sc.episodes) == 0 {
			return nil, fmt.Errorf("%w: episode %s has no number", core.ErrInvalid, it.ID)
		}
		sc.season = sc.episodes[0].SeasonNumber
	}
	return sc, nil
}

// interactiveSearch returns the search an interactive search for a
// scope is, as Sonarr's ReleaseSearchService builds it for an episode or
// a season; a series is searched for as by RSS, nil.
func (rn *run) interactiveSearch(sc *episodeScope) *series.Search {
	sh := sc.show
	switch sc.kind {
	case core.KindEpisode:
		ep := sc.episodes[0]
		kind := series.SingleEpisodeSearch
		switch {
		case sh.series.Type == series.Daily:
			kind = series.DailyEpisodeSearch
		case sh.series.Type == series.Anime && ep.SeasonNumber != 0:
			kind = series.AnimeEpisodeSearch
		case ep.SeasonNumber == 0:
			kind = series.SpecialEpisodeSearch
		}
		s := rn.searchOf(sh, kind, sc.episodes, nil, false, true, true)
		s.SeasonNumber = orInt(ep.SceneSeasonNumber, ep.SeasonNumber)
		s.EpisodeNumber = orInt(ep.SceneEpisodeNumber, ep.EpisodeNumber)
		s.AbsoluteEpisodeNumber = orInt(ep.SceneAbsoluteEpisodeNumber, ep.AbsoluteEpisodeNumber)
		if t, err := time.Parse(time.DateOnly, ep.AirDate); err == nil {
			s.AirDate = t
		}
		return s
	case core.KindSeason:
		kind := series.SeasonSearch
		switch sh.series.Type {
		case series.Daily:
			kind = series.DailySeasonSearch
		case series.Anime:
			kind = series.AnimeSeasonSearch
		}
		if sc.season == 0 {
			kind = series.SpecialEpisodeSearch
		}
		s := rn.searchOf(sh, kind, sc.episodes, nil, false, true, true)
		s.SeasonNumber = sc.season
		return s
	}
	return nil
}

// decideEpisodes decides on the releases an interactive search found for
// a series, season or episode, best first, the rejected last.
func (r *Runtime) decideEpisodes(ctx context.Context, rn *run, it core.Item, found []releases.Release) ([]Decided, error) {
	sc, err := rn.episodeScope(ctx, it)
	if err != nil {
		return nil, err
	}
	if !sc.show.item.Monitored {
		return nil, ErrNotWanted
	}
	b := newBatch()
	index := map[*acquire.Release]int{}
	for i, rel := range found {
		rl := b.add(FromSearch(rel))
		index[rl] = i
		b.targets[rl] = sc.show.item.ID
	}
	find, special, err := rn.findEpisodes(ctx, b)
	if err != nil {
		return nil, err
	}
	s := rn.interactiveSearch(sc)
	decisions := rn.senv.Prioritize(rn.sengine.DecideEpisodes(ctx, b.releases, s, false, find, special))
	class := func(d series.Decision) int {
		switch {
		case d.Approved():
			return 0
		case d.TemporarilyRejected():
			return 1
		}
		return 2
	}
	slices.SortStableFunc(decisions, func(x, y series.Decision) int { return cmp.Compare(class(x), class(y)) })
	out := make([]Decided, 0, len(found))
	seen := map[int]bool{}
	for _, d := range decisions {
		i := index[d.Candidate.Release]
		if seen[i] {
			continue
		}
		seen[i] = true
		dd := Decided{Release: found[i], Rejections: d.Rejections, Score: d.Candidate.FormatScore}
		for _, f := range d.Candidate.Formats {
			dd.Formats = append(dd.Formats, f.Name)
		}
		for _, ep := range d.Candidate.Episodes {
			dd.EpisodeIDs = append(dd.EpisodeIDs, ep.ID)
		}
		out = append(out, dd)
	}
	for i, rel := range found {
		if !seen[i] {
			out = append(out, Decided{Release: rel, Rejections: []acquire.Rejection{{Reason: acquire.ReasonUnableToParse, Message: "Unable to parse release"}}})
		}
	}
	return out, nil
}

// grabEpisodeRelease sends a release to a download client for episodes
// of a wanted series, whatever the decisions on it, as Sonarr's
// ReleaseController: the episodes are those its title maps to, else
// those of the season or episode it was found for.
func (r *Runtime) grabEpisodeRelease(ctx context.Context, rn *run, it core.Item, rel *Release, pluginID, clientID string) (core.Download, error) {
	sc, err := rn.episodeScope(ctx, it)
	if err != nil {
		return core.Download{}, err
	}
	sh := sc.show
	b := newBatch()
	rl := b.add(rel)
	b.targets[rl] = sh.item.ID
	find, special, err := rn.findEpisodes(ctx, b)
	if err != nil {
		return core.Download{}, err
	}
	var c *series.Candidate
	if ds := rn.sengine.DecideEpisodes(ctx, b.releases, rn.interactiveSearch(sc), false, find, special); len(ds) > 0 &&
		ds[0].Candidate.Series != nil && ds[0].Candidate.Series.ID == sh.item.ID && len(ds[0].Candidate.Episodes) > 0 {
		c = ds[0].Candidate
	}
	if c == nil {
		if len(sc.episodes) == 0 {
			return core.Download{}, fmt.Errorf("%w: couldn't find the episodes of %s", core.ErrInvalid, rel.Title)
		}
		c = series.NewCandidate(rl, release.ParseSeriesTitle(rel.Title), sh.series, sc.episodes)
		if c.Quality.Quality == release.Unknown {
			c.Quality = acquire.ModelOf(release.ParseQuality(rel.Title, release.Series))
		}
		c.Formats = rn.seriesFormats(rel.Title, c.Quality, c.Languages, rel.Size, rel.Flags)
		c.FormatScore = sh.series.Profile.FormatScore(c.Formats)
	}
	if langs := languagesOf(rel.Languages); len(langs) > 0 {
		c.Languages = langs
	}
	c.Source = acquire.SourceInteractiveSearch
	return r.grab(ctx, rn, grabRequest{show: sh, episodes: c.Episodes, rel: rel, candidate: c.Candidate, pluginID: pluginID, clientID: clientID})
}
