package series

import (
	"context"
	"log/slog"
	"slices"
	"strings"

	"github.com/mavioai/mavio/libs/acquire"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/release"
)

// SeriesService finds the series releases may be of, as Sonarr's
// ISeriesService. Titles are as parsed; implementations clean them. A
// lookup that matches several series fails.
type SeriesService interface {
	FindByTitle(ctx context.Context, title string) (*Series, error)
	FindByTitleYear(ctx context.Context, title string, year int) (*Series, error)
	// FindByTitleInexact finds the series whose clean title is in a
	// release title's, the leftmost and longest.
	FindByTitleInexact(ctx context.Context, title string) (*Series, error)
	FindByTvdbID(ctx context.Context, id int) (*Series, error)
	FindByTvRageID(ctx context.Context, id int) (*Series, error)
	FindByImdbID(ctx context.Context, id string) (*Series, error)
}

// EpisodeService finds the episodes of series, as Sonarr's
// IEpisodeService.
type EpisodeService interface {
	FindEpisode(ctx context.Context, seriesID core.ID, season, episode int) (*Episode, error)
	FindEpisodeByAbsolute(ctx context.Context, seriesID core.ID, absolute int) (*Episode, error)
	// FindEpisodeByAirDate finds the episode aired on a date
	// ("2006-01-02"), the part-th of the day's when part is set.
	FindEpisodeByAirDate(ctx context.Context, seriesID core.ID, airDate string, part *int) (*Episode, error)
	FindEpisodesBySceneNumbering(ctx context.Context, seriesID core.ID, sceneSeason, sceneEpisode int) ([]*Episode, error)
	FindEpisodesBySceneAbsolute(ctx context.Context, seriesID core.ID, sceneAbsolute int) ([]*Episode, error)
	EpisodesBySeason(ctx context.Context, seriesID core.ID, season int) ([]*Episode, error)
	EpisodesBySceneSeason(ctx context.Context, seriesID core.ID, sceneSeason int) ([]*Episode, error)
	// FindEpisodeByTitle finds the episode of a season whose title a
	// release title holds.
	FindEpisodeByTitle(ctx context.Context, seriesID core.ID, season int, releaseTitle string) (*Episode, error)
}

// Parsing maps parsed releases to their series and episodes, as Sonarr's
// ParsingService.
type Parsing struct {
	Series   SeriesService
	Episodes EpisodeService
	// Scene finds scene mappings; nil for none.
	Scene  SceneMappingService
	Logger *slog.Logger
}

func (p *Parsing) logger() *slog.Logger {
	if p.Logger != nil {
		return p.Logger
	}
	return slog.New(slog.DiscardHandler)
}

func (p *Parsing) findSceneMapping(seriesTitle, releaseTitle string, sceneSeason int) (*SceneMapping, error) {
	if p.Scene == nil {
		return nil, nil
	}
	return p.Scene.FindSceneMapping(seriesTitle, releaseTitle, sceneSeason)
}

func (p *Parsing) findTvdbID(seriesTitle, releaseTitle string, sceneSeason int) (int, bool, error) {
	sm, err := p.findSceneMapping(seriesTitle, releaseTitle, sceneSeason)
	if err != nil || sm == nil {
		return 0, false, err
	}
	return sm.TvdbID, true, nil
}

// GetSeries returns the series a release title is of, as Sonarr's
// ParsingService.GetSeries; nil when none.
func (p *Parsing) GetSeries(ctx context.Context, title string) (*Series, error) {
	parsed := release.ParseSeriesTitle(title)
	if parsed == nil {
		return p.Series.FindByTitle(ctx, title)
	}
	id, ok, err := p.findTvdbID(parsed.SeriesTitle, parsed.ReleaseTitle, parsed.SeasonNumber)
	if err != nil {
		return nil, err
	}
	if ok {
		return p.Series.FindByTvdbID(ctx, id)
	}
	s, err := p.Series.FindByTitle(ctx, parsed.SeriesTitle)
	if err != nil {
		return nil, err
	}
	if s == nil && parsed.SeriesTitleInfo.AllTitles != nil {
		if s, err = p.seriesByAllTitles(ctx, parsed); err != nil {
			return nil, err
		}
	}
	if s == nil {
		return p.Series.FindByTitleYear(ctx, parsed.SeriesTitleInfo.TitleWithoutYear, parsed.SeriesTitleInfo.Year)
	}
	return s, nil
}

// seriesByAllTitles returns the series every alternative title of a
// release matches, by title or scene mapping; nil when they match
// different series.
func (p *Parsing) seriesByAllTitles(ctx context.Context, parsed *release.ParsedEpisodeInfo) (*Series, error) {
	var found *Series
	foundID, haveID := 0, false
	for _, title := range parsed.SeriesTitleInfo.AllTitles {
		s, err := p.Series.FindByTitle(ctx, title)
		if err != nil {
			return nil, err
		}
		id, ok := 0, false
		if s != nil {
			id, ok = s.TvdbID, true
		} else if id, ok, err = p.findTvdbID(title, parsed.ReleaseTitle, parsed.SeasonNumber); err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		if haveID && id != foundID {
			return nil, nil
		}
		if found == nil {
			found = s
		}
		foundID, haveID = id, true
	}
	if found == nil && haveID {
		return p.Series.FindByTvdbID(ctx, foundID)
	}
	return found, nil
}

// Map maps a parsed release to its series and episodes, as Sonarr's
// ParsingService.Map: the series is s when known, else found by the
// release's scene mapping, the search, its title or IDs. search is nil
// for RSS syncs and pushed releases.
func (p *Parsing) Map(ctx context.Context, parsed *release.ParsedEpisodeInfo, tvdbID, tvRageID int, imdbID string, s *Series, search *Search) (*Match, error) {
	sm, err := p.findSceneMapping(parsed.SeriesTitle, parsed.ReleaseTitle, parsed.SeasonNumber)
	if err != nil {
		return nil, err
	}
	m := &Match{SceneMapping: sm, MappedSeasonNumber: parsed.SeasonNumber}
	sceneSource := true
	if sm != nil {
		if sm.SeasonNumber != nil && *sm.SeasonNumber >= 0 && sm.SceneSeasonNumber != nil && *sm.SceneSeasonNumber <= parsed.SeasonNumber {
			m.MappedSeasonNumber += *sm.SeasonNumber - *sm.SceneSeasonNumber
		}
		switch {
		case sm.SceneOrigin == "tvdb":
			sceneSource = false
		case sm.Type == "XemService" && sm.SceneSeasonNumber != nil && *sm.SceneSeasonNumber >= 0 &&
			parsed.SeasonNumber == 1 && *sm.SceneSeasonNumber != parsed.SeasonNumber:
			m.MappedSeasonNumber = *sm.SceneSeasonNumber
		}
	}
	if s == nil {
		if s, err = p.findSeries(ctx, parsed, tvdbID, tvRageID, imdbID, sm, search); err != nil {
			return nil, err
		}
	}
	if s != nil {
		m.Series = s
		if validForSeriesType(parsed, s) {
			if m.Episodes, err = p.getEpisodes(ctx, parsed, s, m.MappedSeasonNumber, sceneSource, search); err != nil {
				return nil, err
			}
		} else {
			p.logger().WarnContext(ctx, "found daily-style episode for non-daily series", "series", s.Title)
		}
	} else {
		// Sonarr's DownloadDecisionMaker tells releases of another series'
		// alias apart.
		if id, ok, err := p.findTvdbID(parsed.SeriesTitle, parsed.ReleaseTitle, parsed.SeasonNumber); err != nil {
			return nil, err
		} else if ok {
			m.AliasTvdbID = id
		}
	}
	if search != nil {
		m.EpisodeRequested = slices.ContainsFunc(m.Episodes, func(ep *Episode) bool {
			return slices.ContainsFunc(search.Episodes, func(r *Episode) bool { return r.ID == ep.ID })
		})
	}
	return m, nil
}

// validForSeriesType reports whether a release suits its series' type,
// as Sonarr's ValidateParsedEpisodeInfo.ValidateForSeriesType: daily
// releases do not suit standard series.
func validForSeriesType(parsed *release.ParsedEpisodeInfo, s *Series) bool {
	return !parsed.IsDaily() || s.Type != Standard
}

// GetEpisodes returns the episodes of a series a parsed release holds,
// as Sonarr's ParsingService.GetEpisodes: a scene source is mapped
// through the release's scene mapping first.
func (p *Parsing) GetEpisodes(ctx context.Context, parsed *release.ParsedEpisodeInfo, s *Series, sceneSource bool, search *Search) ([]*Episode, error) {
	if sceneSource {
		m, err := p.Map(ctx, parsed, 0, 0, "", s, search)
		if err != nil {
			return nil, err
		}
		return m.Episodes, nil
	}
	return p.getEpisodes(ctx, parsed, s, parsed.SeasonNumber, sceneSource, search)
}

func (p *Parsing) getEpisodes(ctx context.Context, parsed *release.ParsedEpisodeInfo, s *Series, mappedSeason int, sceneSource bool, search *Search) ([]*Episode, error) {
	switch {
	case parsed.FullSeason:
		if s.UseSceneNumbering && sceneSource {
			episodes, err := p.Episodes.EpisodesBySceneSeason(ctx, s.ID, mappedSeason)
			if err != nil || len(episodes) > 0 {
				return episodes, err
			}
		}
		return p.Episodes.EpisodesBySeason(ctx, s.ID, mappedSeason)
	case parsed.IsDaily():
		ep, err := p.dailyEpisode(ctx, s, parsed.AirDate, parsed.DailyPart, search)
		if err != nil || ep == nil {
			return nil, err
		}
		return []*Episode{ep}, nil
	case parsed.IsAbsoluteNumbering():
		return p.animeEpisodes(ctx, s, parsed, sceneSource)
	}
	if parsed.IsPossibleSceneSeasonSpecial() {
		special, err := p.ParseSpecialEpisodeTitleOf(ctx, parsed, parsed.ReleaseTitle, s)
		if err != nil {
			return nil, err
		}
		if special != nil {
			// The numbers found are not scene numbers.
			return p.standardEpisodes(ctx, s, special, special.SeasonNumber, false, search)
		}
	}
	if parsed.Special && mappedSeason != 0 {
		return nil, nil
	}
	return p.standardEpisodes(ctx, s, parsed, mappedSeason, sceneSource, search)
}

// dailyEpisode returns the episode of a daily series aired on a date:
// the search's, else the series'.
func (p *Parsing) dailyEpisode(ctx context.Context, s *Series, airDate string, part *int, search *Search) (*Episode, error) {
	if search != nil {
		var found *Episode
		n := 0
		for _, ep := range search.Episodes {
			if ep.AirDate == airDate {
				found = ep
				n++
			}
		}
		if n == 1 {
			return found, nil
		}
	}
	return p.Episodes.FindEpisodeByAirDate(ctx, s.ID, airDate, part)
}

// animeEpisodes returns the episodes of absolute numbers, by their scene
// numbers when the release is a scene source, as Sonarr's
// ParsingService.GetAnimeEpisodes.
func (p *Parsing) animeEpisodes(ctx context.Context, s *Series, parsed *release.ParsedEpisodeInfo, sceneSource bool) ([]*Episode, error) {
	sceneSeason, haveSceneSeason := 0, false
	if p.Scene != nil {
		var err error
		if sceneSeason, haveSceneSeason, err = p.Scene.GetSceneSeasonNumber(parsed.SeriesTitle, parsed.ReleaseTitle); err != nil {
			return nil, err
		}
	}
	var result []*Episode
	for _, abs := range parsed.AbsoluteEpisodeNumbers {
		var episodes []*Episode
		appendOne := func(ep *Episode, err error) error {
			if ep != nil {
				episodes = append(episodes, ep)
			}
			return err
		}
		var err error
		switch {
		case parsed.Special:
			err = appendOne(p.Episodes.FindEpisode(ctx, s.ID, 0, abs))
		case sceneSource && haveSceneSeason:
			if episodes, err = p.Episodes.FindEpisodesBySceneNumbering(ctx, s.ID, sceneSeason, abs); err == nil && len(episodes) == 0 {
				err = appendOne(p.Episodes.FindEpisode(ctx, s.ID, sceneSeason, abs))
			}
		case sceneSource && parsed.SeasonNumber > 1 && len(parsed.EpisodeNumbers) == 0:
			if episodes, err = p.Episodes.FindEpisodesBySceneNumbering(ctx, s.ID, parsed.SeasonNumber, abs); err == nil && len(episodes) == 0 {
				err = appendOne(p.Episodes.FindEpisode(ctx, s.ID, parsed.SeasonNumber, abs))
			}
		case sceneSource:
			episodes, err = p.Episodes.FindEpisodesBySceneAbsolute(ctx, s.ID, abs)
			// Several episodes need a scene name mapping.
			if len(episodes) > 1 {
				episodes = nil
			}
		}
		if err != nil {
			return nil, err
		}
		if len(episodes) == 0 {
			if err := appendOne(p.Episodes.FindEpisodeByAbsolute(ctx, s.ID, abs)); err != nil {
				return nil, err
			}
		}
		result = append(result, episodes...)
	}
	return result, nil
}

// standardEpisodes returns the episodes of season and episode numbers,
// by the scene's when the series uses scene numbering, as Sonarr's
// ParsingService.GetStandardEpisodes.
func (p *Parsing) standardEpisodes(ctx context.Context, s *Series, parsed *release.ParsedEpisodeInfo, mappedSeason int, sceneSource bool, search *Search) ([]*Episode, error) {
	var result []*Episode
	for _, number := range parsed.EpisodeNumbers {
		if s.UseSceneNumbering && sceneSource {
			var episodes []*Episode
			if search != nil {
				for _, ep := range search.Episodes {
					if ep.SceneSeasonNumber != nil && *ep.SceneSeasonNumber == parsed.SeasonNumber &&
						ep.SceneEpisodeNumber != nil && *ep.SceneEpisodeNumber == number {
						episodes = append(episodes, ep)
					}
				}
			}
			if len(episodes) == 0 {
				var err error
				if episodes, err = p.Episodes.FindEpisodesBySceneNumbering(ctx, s.ID, mappedSeason, number); err != nil {
					return nil, err
				}
			}
			if len(episodes) > 0 {
				result = append(result, episodes...)
				continue
			}
		}
		var found *Episode
		if search != nil {
			n := 0
			for _, ep := range search.Episodes {
				if ep.SeasonNumber == mappedSeason && ep.EpisodeNumber == number {
					found = ep
					n++
				}
			}
			if n != 1 {
				found = nil
			}
		}
		if found == nil {
			var err error
			if found, err = p.Episodes.FindEpisode(ctx, s.ID, mappedSeason, number); err != nil {
				return nil, err
			}
		}
		if found != nil {
			result = append(result, found)
		}
	}
	return result, nil
}

// ParseSpecialEpisodeTitle parses a release as a special of the series
// it is of, as Sonarr's ParsingService.ParseSpecialEpisodeTitle: the
// searched series when the release's IDs are its, else the series found
// by the title or IDs. It returns nil when the release is no special.
func (p *Parsing) ParseSpecialEpisodeTitle(ctx context.Context, parsed *release.ParsedEpisodeInfo, releaseTitle string, tvdbID, tvRageID int, imdbID string, search *Search) (*release.ParsedEpisodeInfo, error) {
	if search != nil && search.Series != nil {
		ss := search.Series
		if (tvdbID != 0 && tvdbID == ss.TvdbID) || (tvRageID != 0 && tvRageID == ss.TvRageID) ||
			(strings.TrimSpace(imdbID) != "" && imdbID == ss.ImdbID) {
			return p.ParseSpecialEpisodeTitleOf(ctx, parsed, releaseTitle, ss)
		}
	}
	s, err := p.GetSeries(ctx, releaseTitle)
	if err != nil {
		return nil, err
	}
	if s == nil {
		if s, err = p.Series.FindByTitleInexact(ctx, releaseTitle); err != nil {
			return nil, err
		}
	}
	if s == nil && tvdbID > 0 {
		if s, err = p.Series.FindByTvdbID(ctx, tvdbID); err != nil {
			return nil, err
		}
	}
	if s == nil && tvRageID > 0 {
		if s, err = p.Series.FindByTvRageID(ctx, tvRageID); err != nil {
			return nil, err
		}
	}
	if s == nil && strings.TrimSpace(imdbID) != "" {
		if s, err = p.Series.FindByImdbID(ctx, imdbID); err != nil {
			return nil, err
		}
	}
	if s == nil {
		return nil, nil
	}
	return p.ParseSpecialEpisodeTitleOf(ctx, parsed, releaseTitle, s)
}

// ParseSpecialEpisodeTitleOf parses a release as a special of a series:
// the special of season 0 whose title the release title holds. An
// episode 0 of a season the scene numbers stays as parsed.
func (p *Parsing) ParseSpecialEpisodeTitleOf(ctx context.Context, parsed *release.ParsedEpisodeInfo, releaseTitle string, s *Series) (*release.ParsedEpisodeInfo, error) {
	if parsed != nil && parsed.IsPossibleSceneSeasonSpecial() && s.UseSceneNumbering {
		episodes, err := p.Episodes.FindEpisodesBySceneNumbering(ctx, s.ID, parsed.SeasonNumber, 0)
		if err != nil {
			return nil, err
		}
		if len(episodes) > 0 {
			return parsed, nil
		}
	}
	ep, err := p.Episodes.FindEpisodeByTitle(ctx, s.ID, 0, releaseTitle)
	if err != nil || ep == nil {
		return nil, err
	}
	return &release.ParsedEpisodeInfo{
		ReleaseTitle:    releaseTitle,
		SeriesTitle:     s.Title,
		SeriesTitleInfo: release.SeriesTitleInfo{Title: s.Title},
		SeasonNumber:    ep.SeasonNumber,
		EpisodeNumbers:  []int{ep.EpisodeNumber},
		Quality:         release.ParseQuality(releaseTitle, release.Series),
		ReleaseGroup:    release.ParseReleaseGroup(releaseTitle, release.Series),
		Languages:       release.ParseLanguages(releaseTitle, release.Series),
		Special:         true,
	}, nil
}

// findSeries returns the series of a release, as Sonarr's
// ParsingService.FindSeries: that of its scene mapping, else the
// searched series when the title or IDs match it, else the series of the
// title, its alternatives, the title and year, or the IDs.
func (p *Parsing) findSeries(ctx context.Context, parsed *release.ParsedEpisodeInfo, tvdbID, tvRageID int, imdbID string, sm *SceneMapping, search *Search) (*Series, error) {
	var ss *Series
	if search != nil {
		ss = search.Series
	}
	if sm != nil {
		if ss != nil && ss.TvdbID == sm.TvdbID {
			return ss, nil
		}
		return p.Series.FindByTvdbID(ctx, sm.TvdbID)
	}
	if ss != nil {
		switch {
		case ss.cleanTitle() == release.CleanSeriesTitle(parsed.SeriesTitle):
			return ss, nil
		case tvdbID > 0 && tvdbID == ss.TvdbID,
			tvRageID > 0 && tvRageID == ss.TvRageID && tvdbID <= 0,
			strings.TrimSpace(imdbID) != "" && imdbID == ss.ImdbID && tvdbID <= 0:
			p.logger().DebugContext(ctx, "found matching series by ID, an alias may be needed", "title", parsed.SeriesTitle)
			return ss, nil
		}
	}
	s, err := p.Series.FindByTitle(ctx, parsed.SeriesTitle)
	if err != nil {
		return nil, err
	}
	if s == nil && parsed.SeriesTitleInfo.AllTitles != nil {
		if s, err = p.seriesByAllTitles(ctx, parsed); err != nil {
			return nil, err
		}
	}
	if s == nil && parsed.SeriesTitleInfo.Year > 0 {
		if s, err = p.Series.FindByTitleYear(ctx, parsed.SeriesTitleInfo.TitleWithoutYear, parsed.SeriesTitleInfo.Year); err != nil {
			return nil, err
		}
	}
	if s == nil && tvdbID > 0 {
		if s, err = p.Series.FindByTvdbID(ctx, tvdbID); err != nil {
			return nil, err
		}
	}
	if s == nil && tvRageID > 0 && tvdbID <= 0 {
		if s, err = p.Series.FindByTvRageID(ctx, tvRageID); err != nil {
			return nil, err
		}
	}
	if s == nil && strings.TrimSpace(imdbID) != "" && tvdbID <= 0 {
		if s, err = p.Series.FindByImdbID(ctx, imdbID); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// FindEpisodes returns the FindEpisodes of DecideEpisodes: Map with the
// release's IDs. known, when set, gives the series of releases whose
// series is known, such as pending ones; it returns nil for others.
func (p *Parsing) FindEpisodes(known func(r *acquire.Release) *Series) FindEpisodes {
	return func(ctx context.Context, parsed *release.ParsedEpisodeInfo, r *acquire.Release, s *Search) (*Match, error) {
		var series *Series
		if known != nil {
			series = known(r)
		}
		return p.Map(ctx, parsed, r.TvdbID, r.TvRageID, r.ImdbID, series, s)
	}
}

// ParseSpecial returns the ParseSpecial of DecideEpisodes:
// ParseSpecialEpisodeTitle with the release's IDs.
func (p *Parsing) ParseSpecial() ParseSpecial {
	return func(ctx context.Context, parsed *release.ParsedEpisodeInfo, r *acquire.Release, s *Search) (*release.ParsedEpisodeInfo, error) {
		return p.ParseSpecialEpisodeTitle(ctx, parsed, r.Title, r.TvdbID, r.TvRageID, r.ImdbID, s)
	}
}
