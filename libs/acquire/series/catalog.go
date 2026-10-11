package series

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/release"
)

// ErrMultipleSeries is returned when a lookup matches several series, as
// Sonarr's MultipleSeriesFoundException.
var ErrMultipleSeries = errors.New("several series match")

// ErrMultipleEpisodes is returned when several regular episodes aired on
// a date and the release does not tell which, as Sonarr's
// EpisodeService.FindOneByAirDate.
var ErrMultipleEpisodes = errors.New("several episodes aired on the same date")

// Catalog is the series releases may be of with their episodes, kept in
// memory, as Sonarr's SeriesService and EpisodeService over their
// repositories. Episodes are loaded per series on first use.
type Catalog struct {
	List []*Series
	// Load returns the episodes of a series.
	Load func(ctx context.Context, seriesID core.ID) ([]*Episode, error)

	episodes map[core.ID][]*Episode
}

var (
	_ SeriesService  = (*Catalog)(nil)
	_ EpisodeService = (*Catalog)(nil)
)

// single returns the one series matching, nil for none.
func single(list []*Series, what string) (*Series, error) {
	switch len(list) {
	case 0:
		return nil, nil
	case 1:
		return list[0], nil
	}
	names := make([]string, len(list))
	for i, s := range list {
		names[i] = s.Title
	}
	return nil, fmt.Errorf("%w: %s: %s", ErrMultipleSeries, what, strings.Join(names, ", "))
}

func (c *Catalog) where(pred func(s *Series) bool) []*Series {
	var out []*Series
	for _, s := range c.List {
		if pred(s) {
			out = append(out, s)
		}
	}
	return out
}

// FindByTitle implements SeriesService.
func (c *Catalog) FindByTitle(_ context.Context, title string) (*Series, error) {
	clean := strings.ToLower(release.CleanSeriesTitle(title))
	return single(c.where(func(s *Series) bool { return s.cleanTitle() == clean }), title)
}

// FindByTitleYear implements SeriesService.
func (c *Catalog) FindByTitleYear(_ context.Context, title string, year int) (*Series, error) {
	clean := strings.ToLower(release.CleanSeriesTitle(title))
	return single(c.where(func(s *Series) bool { return s.cleanTitle() == clean && s.Year == year }), title)
}

// FindByTitleInexact implements SeriesService.
func (c *Catalog) FindByTitleInexact(_ context.Context, title string) (*Series, error) {
	clean := release.CleanSeriesTitle(title)
	list := c.where(func(s *Series) bool { return s.cleanTitle() != "" && strings.Contains(clean, s.cleanTitle()) })
	if len(list) <= 1 {
		return single(list, title)
	}
	// The leftmost series, the longest of those: series titles come first
	// in releases.
	slices.SortStableFunc(list, func(a, b *Series) int {
		if r := cmp.Compare(strings.Index(clean, a.cleanTitle()), strings.Index(clean, b.cleanTitle())); r != 0 {
			return r
		}
		return cmp.Compare(len(b.cleanTitle()), len(a.cleanTitle()))
	})
	return list[0], nil
}

// FindByTvdbID implements SeriesService.
func (c *Catalog) FindByTvdbID(_ context.Context, id int) (*Series, error) {
	return single(c.where(func(s *Series) bool { return id != 0 && s.TvdbID == id }), fmt.Sprint("TVDB ", id))
}

// FindByTvRageID implements SeriesService.
func (c *Catalog) FindByTvRageID(_ context.Context, id int) (*Series, error) {
	return single(c.where(func(s *Series) bool { return id != 0 && s.TvRageID == id }), fmt.Sprint("TVRage ", id))
}

// FindByImdbID implements SeriesService.
func (c *Catalog) FindByImdbID(_ context.Context, id string) (*Series, error) {
	id = release.NormalizeImdbID(id)
	return single(c.where(func(s *Series) bool { return id != "" && release.NormalizeImdbID(s.ImdbID) == id }), "IMDb "+id)
}

// Episodes returns every episode of a series.
func (c *Catalog) Episodes(ctx context.Context, seriesID core.ID) ([]*Episode, error) {
	if eps, ok := c.episodes[seriesID]; ok {
		return eps, nil
	}
	if c.Load == nil {
		return nil, nil
	}
	eps, err := c.Load(ctx, seriesID)
	if err != nil {
		return nil, err
	}
	if c.episodes == nil {
		c.episodes = map[core.ID][]*Episode{}
	}
	c.episodes[seriesID] = eps
	return eps, nil
}

// Forget drops the loaded episodes of a series, for them to be loaded
// again.
func (c *Catalog) Forget(seriesID core.ID) { delete(c.episodes, seriesID) }

func (c *Catalog) episodesWhere(ctx context.Context, seriesID core.ID, pred func(ep *Episode) bool) ([]*Episode, error) {
	eps, err := c.Episodes(ctx, seriesID)
	if err != nil {
		return nil, err
	}
	var out []*Episode
	for _, ep := range eps {
		if pred(ep) {
			out = append(out, ep)
		}
	}
	return out, nil
}

func first(eps []*Episode, err error) (*Episode, error) {
	if err != nil || len(eps) == 0 {
		return nil, err
	}
	return eps[0], nil
}

// FindEpisode implements EpisodeService.
func (c *Catalog) FindEpisode(ctx context.Context, seriesID core.ID, season, episode int) (*Episode, error) {
	return first(c.episodesWhere(ctx, seriesID, func(ep *Episode) bool { return ep.SeasonNumber == season && ep.EpisodeNumber == episode }))
}

// FindEpisodeByAbsolute implements EpisodeService.
func (c *Catalog) FindEpisodeByAbsolute(ctx context.Context, seriesID core.ID, absolute int) (*Episode, error) {
	eps, err := c.episodesWhere(ctx, seriesID, func(ep *Episode) bool { return ep.AbsoluteEpisodeNumber != 0 && ep.AbsoluteEpisodeNumber == absolute })
	if len(eps) > 1 {
		// Sonarr's SingleOrDefault fails; no episode is better here.
		return nil, err
	}
	return first(eps, err)
}

// FindEpisodeByAirDate implements EpisodeService, as Sonarr's
// FindOneByAirDate: of several episodes aired that day the regular
// one, or the part-th of the regular ones.
func (c *Catalog) FindEpisodeByAirDate(ctx context.Context, seriesID core.ID, airDate string, part *int) (*Episode, error) {
	eps, err := c.episodesWhere(ctx, seriesID, func(ep *Episode) bool { return ep.AirDate != "" && ep.AirDate == airDate })
	if err != nil || len(eps) == 0 {
		return nil, err
	}
	if len(eps) == 1 {
		return eps[0], nil
	}
	var regular []*Episode
	for _, ep := range eps {
		if ep.SeasonNumber > 0 {
			regular = append(regular, ep)
		}
	}
	switch {
	case len(regular) == 1 && part == nil:
		return regular[0], nil
	case part != nil && *part >= 1 && *part <= len(regular):
		slices.SortStableFunc(regular, func(a, b *Episode) int {
			if r := cmp.Compare(a.SeasonNumber, b.SeasonNumber); r != 0 {
				return r
			}
			return cmp.Compare(a.EpisodeNumber, b.EpisodeNumber)
		})
		return regular[*part-1], nil
	}
	return nil, fmt.Errorf("%w: %s", ErrMultipleEpisodes, airDate)
}

// FindEpisodesBySceneNumbering implements EpisodeService.
func (c *Catalog) FindEpisodesBySceneNumbering(ctx context.Context, seriesID core.ID, sceneSeason, sceneEpisode int) ([]*Episode, error) {
	return c.episodesWhere(ctx, seriesID, func(ep *Episode) bool {
		return ep.SceneSeasonNumber != nil && *ep.SceneSeasonNumber == sceneSeason && ep.SceneEpisodeNumber != nil && *ep.SceneEpisodeNumber == sceneEpisode
	})
}

// FindEpisodesBySceneAbsolute implements EpisodeService.
func (c *Catalog) FindEpisodesBySceneAbsolute(ctx context.Context, seriesID core.ID, sceneAbsolute int) ([]*Episode, error) {
	return c.episodesWhere(ctx, seriesID, func(ep *Episode) bool {
		return ep.SceneAbsoluteEpisodeNumber != nil && *ep.SceneAbsoluteEpisodeNumber == sceneAbsolute
	})
}

// EpisodesBySeason implements EpisodeService and EpisodePorts.
func (c *Catalog) EpisodesBySeason(ctx context.Context, seriesID core.ID, season int) ([]*Episode, error) {
	return c.episodesWhere(ctx, seriesID, func(ep *Episode) bool { return ep.SeasonNumber == season })
}

// EpisodesBySceneSeason implements EpisodeService.
func (c *Catalog) EpisodesBySceneSeason(ctx context.Context, seriesID core.ID, sceneSeason int) ([]*Episode, error) {
	return c.episodesWhere(ctx, seriesID, func(ep *Episode) bool { return ep.SceneSeasonNumber != nil && *ep.SceneSeasonNumber == sceneSeason })
}

// FindEpisodeByTitle implements EpisodeService, as Sonarr's
// EpisodeService.FindEpisodeByTitle: the episode whose normalized title
// comes first in the release's, the longest of those.
func (c *Catalog) FindEpisodeByTitle(ctx context.Context, seriesID core.ID, season int, releaseTitle string) (*Episode, error) {
	eps, err := c.EpisodesBySeason(ctx, seriesID, season)
	if err != nil {
		return nil, err
	}
	normalized := release.NormalizeEpisodeTitle(releaseTitle)
	cleanNormalized := release.CleanSeriesTitle(normalized)
	type hit struct {
		pos, length int
		ep          *Episode
	}
	var hits []hit
	for _, ep := range eps {
		if ep.Title == "" {
			continue
		}
		title := release.NormalizeEpisodeTitle(ep.Title)
		if i := strings.Index(strings.ToLower(normalized), strings.ToLower(title)); i >= 0 {
			hits = append(hits, hit{i, len(title), ep})
		}
		if i := strings.Index(strings.ToLower(cleanNormalized), strings.ToLower(release.CleanSeriesTitle(title))); i >= 0 {
			hits = append(hits, hit{i, len(title), ep})
		}
	}
	if len(hits) == 0 {
		return nil, nil
	}
	slices.SortStableFunc(hits, func(a, b hit) int {
		if r := cmp.Compare(a.pos, b.pos); r != 0 {
			return r
		}
		return cmp.Compare(b.length, a.length)
	})
	return hits[0].ep, nil
}
