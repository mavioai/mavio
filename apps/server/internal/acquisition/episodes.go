package acquisition

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mavioai/mavio/apps/server/internal/releases"
	"github.com/mavioai/mavio/libs/acquire"
	"github.com/mavioai/mavio/libs/acquire/series"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/release"
)

// JobEpisodeSearch searches for episodes of wanted series and grabs the
// best releases; the payload is an EpisodeSearchPayload.
const JobEpisodeSearch = "acquisition.episode_search"

// EpisodeSearchScope is what an episode search is for, as Sonarr's
// search commands.
type EpisodeSearchScope string

// The episode search scopes.
const (
	// ScopeEpisodes searches for EpisodeIDs of SeriesID, one by one, as
	// Sonarr's EpisodeSearchCommand.
	ScopeEpisodes EpisodeSearchScope = "episodes"
	// ScopeSeason searches for SeasonNumber of SeriesID, as Sonarr's
	// SeasonSearchCommand.
	ScopeSeason EpisodeSearchScope = "season"
	// ScopeSeries searches for the monitored seasons of SeriesID, as
	// Sonarr's SeriesSearchCommand.
	ScopeSeries EpisodeSearchScope = "series"
	// ScopeMissing searches for the monitored aired episodes without a
	// file, of SeriesID or of every wanted series, as Sonarr's
	// MissingEpisodeSearchCommand.
	ScopeMissing EpisodeSearchScope = "missing"
	// ScopeCutoffUnmet searches for the monitored episodes whose file has
	// not met the profile's cutoff, as Sonarr's
	// CutoffUnmetEpisodeSearchCommand.
	ScopeCutoffUnmet EpisodeSearchScope = "cutoff_unmet"
)

// EpisodeSearchPayload is the payload of JobEpisodeSearch jobs.
type EpisodeSearchPayload struct {
	Scope EpisodeSearchScope `json:"scope"`
	// SeriesID is the series; zero for missing and cutoff unmet searches
	// of every series.
	SeriesID     core.ID   `json:"series_id,omitzero"`
	SeasonNumber int       `json:"season_number,omitempty"`
	EpisodeIDs   []core.ID `json:"episode_ids,omitempty"`
	// UserInvoked is set for searches a user asked for.
	UserInvoked bool `json:"user_invoked,omitempty"`
}

// EpisodeSearchJob returns a job of an episode search. Searches for the
// same things share a unique key.
func EpisodeSearchJob(p EpisodeSearchPayload, at time.Time) core.Job {
	payload, _ := json.Marshal(p)
	key := JobEpisodeSearch + ":" + string(p.Scope)
	if !p.SeriesID.IsZero() {
		key += ":" + p.SeriesID.String()
	}
	switch p.Scope {
	case ScopeSeason:
		key += ":" + strconv.Itoa(p.SeasonNumber)
	case ScopeEpisodes:
		ids := make([]string, len(p.EpisodeIDs))
		for i, id := range p.EpisodeIDs {
			ids[i] = id.String()
		}
		slices.Sort(ids)
		sum := sha256.Sum256([]byte(strings.Join(ids, ",")))
		key += ":" + hex.EncodeToString(sum[:8])
	}
	return core.Job{ID: core.NewID(), Kind: JobEpisodeSearch, Payload: payload, UniqueKey: key, MaxAttempts: 1, RunAt: at}
}

// QueueEpisodeSearch enqueues an episode search and reports how many
// were queued; a search whose job is pending counts as queued.
func (r *Runtime) QueueEpisodeSearch(ctx context.Context, p EpisodeSearchPayload) (int, error) {
	job := EpisodeSearchJob(p, r.now())
	if _, err := r.Store.Jobs().Enqueue(ctx, &job); err != nil {
		return 0, fmt.Errorf("queue episode search: %w", err)
	}
	return 1, nil
}

// SearchEpisodes runs an episode search, as Sonarr's EpisodeSearchService,
// SeasonSearchService and SeriesSearchService, and grabs the best
// releases found.
func (r *Runtime) SearchEpisodes(ctx context.Context, p EpisodeSearchPayload) error {
	rn, err := r.newRun(ctx)
	if err != nil {
		return err
	}
	grabbed := 0
	search := func(sh *show, run func() ([]series.Decision, *batch, error)) error {
		decisions, b, err := run()
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			r.logger().ErrorContext(ctx, "episode search failed", "series", sh.item.Name, "err", err)
			return nil
		}
		out, err := r.processEpisodes(ctx, rn, b, decisions)
		if err != nil {
			return err
		}
		grabbed += len(out.grabbed)
		return r.searched(ctx, rn, sh)
	}
	switch p.Scope {
	case ScopeEpisodes:
		sh, err := rn.show(ctx, p.SeriesID)
		if err != nil {
			return err
		}
		for _, id := range p.EpisodeIDs {
			ep, ok := sh.byID[id]
			if !ok {
				r.logger().WarnContext(ctx, "episode to search for is not of the series", "series", sh.item.Name, "episode", id)
				continue
			}
			if err := search(sh, func() ([]series.Decision, *batch, error) {
				return rn.episodeSearch(ctx, sh, ep, p.UserInvoked, false)
			}); err != nil {
				return err
			}
		}
	case ScopeSeason:
		sh, err := rn.show(ctx, p.SeriesID)
		if err != nil {
			return err
		}
		if err := search(sh, func() ([]series.Decision, *batch, error) {
			return rn.seasonSearch(ctx, sh, seasonEpisodes(sh, p.SeasonNumber), true, p.UserInvoked, false)
		}); err != nil {
			return err
		}
	case ScopeSeries:
		sh, err := rn.show(ctx, p.SeriesID)
		if err != nil {
			return err
		}
		if !slices.ContainsFunc(sh.series.Seasons, func(n int) bool { return sh.seasons[n].Monitored }) {
			// No season is monitored: the monitored aired episodes without a
			// file, one by one.
			now := r.now()
			for _, ep := range sh.episodes {
				if !ep.Monitored || ep.File != nil || ep.AirDateUTC.IsZero() || !ep.AirDateUTC.Before(now) {
					continue
				}
				if err := search(sh, func() ([]series.Decision, *batch, error) {
					return rn.episodeSearch(ctx, sh, ep, p.UserInvoked, false)
				}); err != nil {
					return err
				}
			}
			break
		}
		for _, n := range sh.series.Seasons {
			if !sh.seasons[n].Monitored {
				continue
			}
			if err := search(sh, func() ([]series.Decision, *batch, error) {
				return rn.seasonSearch(ctx, sh, seasonEpisodes(sh, n), true, p.UserInvoked, false)
			}); err != nil {
				return err
			}
		}
	case ScopeMissing, ScopeCutoffUnmet:
		filter := WantedMissing
		if p.Scope == ScopeCutoffUnmet {
			filter = WantedCutoffUnmet
		}
		wanted, err := rn.wantedEpisodes(ctx, filter, p.SeriesID)
		if err != nil {
			return err
		}
		queued, err := r.queuedEpisodes(ctx)
		if err != nil {
			return err
		}
		var episodes []wantedEpisode
		for _, we := range wanted {
			if !queued[we.episode.ID] && (filter == WantedCutoffUnmet || we.aired) {
				episodes = append(episodes, we)
			}
		}
		n, err := r.searchBulk(ctx, rn, episodes, p.UserInvoked)
		if err != nil {
			return err
		}
		grabbed += n
	default:
		return fmt.Errorf("%w: episode search scope %q", core.ErrInvalid, p.Scope)
	}
	r.logger().InfoContext(ctx, "episode search completed", "scope", p.Scope, "series", p.SeriesID, "grabbed", grabbed)
	return nil
}

// searched records that a series was searched for.
func (r *Runtime) searched(ctx context.Context, rn *run, sh *show) error {
	w, err := r.Store.Wanted().Get(ctx, sh.item.ID)
	if err != nil {
		return err
	}
	w.LastSearchedAt = r.now()
	if err := r.Store.Wanted().Put(ctx, &w); err != nil {
		return err
	}
	sh.wanted = w
	return nil
}

// searchedEpisodes records that episodes of a series were searched for
// now, as Sonarr's EpisodeService.UpdateLastSearchTime.
func (rn *run) searchedEpisodes(ctx context.Context, sh *show, episodes []*series.Episode) error {
	if len(episodes) == 0 {
		return nil
	}
	now := rn.rt.now()
	numbers := make([]core.EpisodeNumber, len(episodes))
	for i, ep := range episodes {
		numbers[i] = core.EpisodeNumber{Season: ep.SeasonNumber, Episode: ep.EpisodeNumber}
		sh.searchedAt[ep.ID] = now
	}
	return rn.rt.Store.EpisodeStates().Searched(ctx, sh.item.ID, now, numbers...)
}

// seasonEpisodes returns the episodes of a season of a series.
func seasonEpisodes(sh *show, season int) []*series.Episode {
	var out []*series.Episode
	for _, ep := range sh.episodes {
		if ep.SeasonNumber == season {
			out = append(out, ep)
		}
	}
	return out
}

// queuedEpisodes returns the episodes the tracked downloads hold.
func (r *Runtime) queuedEpisodes(ctx context.Context) (map[core.ID]bool, error) {
	tracked, err := r.unfinished(ctx, core.NilID)
	if err != nil {
		return nil, err
	}
	out := map[core.ID]bool{}
	for _, d := range tracked {
		if isSeriesDownload(d) && d.State != core.DownloadFailedPending {
			for _, id := range d.ItemIDs {
				out[id] = true
			}
		}
	}
	return out, nil
}

// searchBulk searches for episodes by series and season, the groups
// searched longest ago first: a season's episodes together, a lone
// episode alone, as Sonarr's EpisodeSearchService.SearchForBulkEpisodes.
// It returns how many releases were grabbed.
func (r *Runtime) searchBulk(ctx context.Context, rn *run, episodes []wantedEpisode, userInvoked bool) (int, error) {
	type group struct {
		sh       *show
		season   int
		episodes []*series.Episode
	}
	var groups []*group
	for _, we := range episodes {
		i := slices.IndexFunc(groups, func(g *group) bool { return g.sh == we.show && g.season == we.episode.SeasonNumber })
		if i < 0 {
			groups = append(groups, &group{sh: we.show, season: we.episode.SeasonNumber})
			i = len(groups) - 1
		}
		groups[i].episodes = append(groups[i].episodes, we.episode)
	}
	// By the episode searched for longest ago, those never searched for
	// first.
	searchedAt := func(g *group) time.Time {
		var first time.Time
		for i, ep := range g.episodes {
			if at := g.sh.searchedAt[ep.ID]; i == 0 || at.Before(first) {
				first = at
			}
		}
		return first
	}
	slices.SortStableFunc(groups, func(a, b *group) int { return searchedAt(a).Compare(searchedAt(b)) })
	grabbed := 0
	for _, g := range groups {
		var decisions []series.Decision
		var b *batch
		var err error
		if len(g.episodes) > 1 {
			decisions, b, err = rn.seasonSearch(ctx, g.sh, g.episodes, true, userInvoked, false)
		} else {
			decisions, b, err = rn.episodeSearch(ctx, g.sh, g.episodes[0], userInvoked, false)
		}
		if err != nil {
			if ctx.Err() != nil {
				return grabbed, ctx.Err()
			}
			r.logger().ErrorContext(ctx, "couldn't search for episodes", "series", g.sh.item.Name, "season", g.season, "err", err)
			continue
		}
		out, err := r.processEpisodes(ctx, rn, b, decisions)
		if err != nil {
			return grabbed, err
		}
		grabbed += len(out.grabbed)
		if err := r.searched(ctx, rn, g.sh); err != nil {
			return grabbed, err
		}
	}
	return grabbed, nil
}

// episodeSearch searches for an episode, as Sonarr's
// ReleaseSearchService.EpisodeSearch: daily episodes by air date, anime
// by absolute number, specials by title, others by scene numbers.
func (rn *run) episodeSearch(ctx context.Context, sh *show, ep *series.Episode, userInvoked, interactive bool) ([]series.Decision, *batch, error) {
	b := newBatch()
	var decisions []series.Decision
	var err error
	switch {
	case sh.series.Type == series.Daily:
		if ep.AirDate == "" {
			return nil, nil, fmt.Errorf("%w: daily episode %s has no air date", core.ErrInvalid, ep.ID)
		}
		decisions, err = rn.searchDaily(ctx, b, sh, ep, false, userInvoked, interactive)
	case sh.series.Type == series.Anime && ep.SeasonNumber == 0 && ep.SceneAbsoluteEpisodeNumber == nil && ep.AbsoluteEpisodeNumber == 0:
		decisions, err = rn.searchSpecial(ctx, b, sh, []*series.Episode{ep}, false, userInvoked, interactive)
	case sh.series.Type == series.Anime:
		decisions, err = rn.searchAnime(ctx, b, sh, ep, false, userInvoked, interactive, false)
	case ep.SeasonNumber == 0:
		decisions, err = rn.searchSpecial(ctx, b, sh, []*series.Episode{ep}, false, userInvoked, interactive)
	default:
		decisions, err = rn.searchSingle(ctx, b, sh, ep, false, userInvoked, interactive)
	}
	return decisions, b, err
}

// seasonSearch searches for episodes of a season, as Sonarr's
// ReleaseSearchService.SeasonSearch: anime and daily series as theirs
// are searched, others by the scene seasons of the episodes, a season
// of one episode as that episode.
func (rn *run) seasonSearch(ctx context.Context, sh *show, episodes []*series.Episode, monitoredOnly, userInvoked, interactive bool) ([]series.Decision, *batch, error) {
	b := newBatch()
	switch sh.series.Type {
	case series.Anime:
		d, err := rn.searchAnimeSeason(ctx, b, sh, episodes, monitoredOnly, userInvoked, interactive)
		return d, b, err
	case series.Daily:
		d, err := rn.searchDailySeason(ctx, b, sh, episodes, monitoredOnly, userInvoked, interactive)
		return d, b, err
	}
	var decisions []series.Decision
	for _, m := range rn.sceneSeasonMappings(sh, episodes) {
		var d []series.Decision
		var err error
		switch {
		case m.season == 0:
			d, err = rn.searchSpecial(ctx, b, sh, m.episodes, monitoredOnly, userInvoked, interactive)
		case len(m.episodes) == 1:
			s := rn.searchOf(sh, series.SingleEpisodeSearch, m.episodes, m.titles, monitoredOnly, userInvoked, interactive)
			s.SeasonNumber, s.EpisodeNumber = m.season, m.episodeNumber
			d, err = rn.dispatch(ctx, b, releases.EpisodeQuery{
				Kind: releases.QuerySingleEpisode, SeriesID: sh.item.ID, Titles: m.titles, Mode: queryMode(m.mode),
				Season: m.season, Episode: m.episodeNumber,
			}, s)
		default:
			s := rn.searchOf(sh, series.SeasonSearch, m.episodes, m.titles, monitoredOnly, userInvoked, interactive)
			s.SeasonNumber = m.season
			d, err = rn.dispatch(ctx, b, releases.EpisodeQuery{
				Kind: releases.QuerySeason, SeriesID: sh.item.ID, Titles: m.titles, Mode: queryMode(m.mode), Season: m.season,
			}, s)
		}
		if err != nil {
			return nil, b, err
		}
		decisions = append(decisions, d...)
	}
	return dedupe(decisions), b, nil
}

// queryMode returns the releases' search mode of a scene mapping's.
func queryMode(m series.SearchMode) releases.SearchMode {
	switch m {
	case series.SearchModeSearchID:
		return releases.SearchByID
	case series.SearchModeSearchTitle:
		return releases.SearchByTitle
	case series.SearchModeSearchIDAndTitle:
		return releases.SearchByIDAndTitle
	}
	return releases.SearchDefault
}

// episodeMapping is how releases number an episode under some of the
// series' titles, as Sonarr's SceneEpisodeMapping; episodes are those a
// season mapping groups.
type episodeMapping struct {
	episode       *series.Episode
	episodes      []*series.Episode
	titles        []string
	mode          series.SearchMode
	season        int
	episodeNumber int
	absolute      int
}

func sceneSeason(n *int) (int, bool) {
	if n == nil || *n < 0 {
		return 0, false
	}
	return *n, true
}

func orInt(n *int, fallback int) int {
	if n == nil {
		return fallback
	}
	return *n
}

// sceneEpisodeMappings returns how releases number an episode, by the
// scene mappings of its series and its own title, as Sonarr's
// ReleaseSearchService.GetSceneEpisodeMappings; mappings numbering it
// alike are merged.
func (rn *run) sceneEpisodeMappings(sh *show, ep *series.Episode, mappings []*series.SceneMapping) []episodeMapping {
	var out []episodeMapping
	add := func(m episodeMapping) {
		for i := range out {
			if out[i].season == m.season && out[i].episodeNumber == m.episodeNumber && out[i].mode == m.mode {
				for _, t := range m.titles {
					if !slices.ContainsFunc(out[i].titles, func(o string) bool { return strings.EqualFold(o, t) }) {
						out[i].titles = append(out[i].titles, t)
					}
				}
				return
			}
		}
		out = append(out, m)
	}
	s := sh.series
	includeGlobal := true
	for _, sm := range mappings {
		ignoreScene := sm.SceneOrigin == "tvdb" || sm.SceneOrigin == "unknown:tvdb"
		mappingSceneSeason, hasSceneSeason := sceneSeason(sm.SceneSeasonNumber)
		mappingSeason, hasSeason := sceneSeason(sm.SeasonNumber)
		mappedSeason := ep.SeasonNumber
		if !ignoreScene {
			mappedSeason = orInt(ep.SceneSeasonNumber, ep.SeasonNumber)
		}
		releaseSeason := mappedSeason
		if hasSceneSeason {
			releaseSeason = mappingSceneSeason
			alt := mappingSceneSeason
			if hasSeason {
				alt = mappingSeason
			}
			if alt != mappedSeason {
				continue
			}
		} else if hasSeason && mappingSeason != ep.SeasonNumber {
			continue
		}
		if sm.SearchTerm == s.Title && strings.TrimSpace(sm.FilterRegex) == "" {
			includeGlobal = false
		}
		mode := sm.SearchMode
		if mode == series.SearchModeDefault && hasSceneSeason &&
			release.CleanSeriesTitle(s.Title) != release.CleanSeriesTitle(sm.SearchTerm) {
			mode = series.SearchModeSearchTitle
		}
		m := episodeMapping{episode: ep, mode: mode, titles: []string{sm.SearchTerm}, season: releaseSeason}
		if ignoreScene {
			m.episodeNumber, m.absolute = ep.EpisodeNumber, ep.AbsoluteEpisodeNumber
		} else {
			m.episodeNumber = orInt(ep.SceneEpisodeNumber, ep.EpisodeNumber)
			m.absolute = orInt(ep.SceneAbsoluteEpisodeNumber, ep.AbsoluteEpisodeNumber)
		}
		add(m)
	}
	if includeGlobal {
		add(episodeMapping{
			episode: ep, mode: series.SearchModeDefault, titles: []string{s.Title},
			season: orInt(ep.SceneSeasonNumber, ep.SeasonNumber), episodeNumber: orInt(ep.SceneEpisodeNumber, ep.EpisodeNumber),
			absolute: orInt(ep.SceneAbsoluteEpisodeNumber, ep.AbsoluteEpisodeNumber),
		})
	}
	return out
}

// sceneSeasonMappings groups episodes by the seasons releases number
// them in, as Sonarr's ReleaseSearchService.GetSceneSeasonMappings.
func (rn *run) sceneSeasonMappings(sh *show, episodes []*series.Episode) []episodeMapping {
	mappings := rn.sceneMappings.FindByTvdbID(sh.series.TvdbID)
	type key struct{ scene, season int }
	var order []key
	groups := map[key][]*series.Episode{}
	for _, ep := range episodes {
		k := key{orInt(ep.SceneSeasonNumber, ep.SeasonNumber), ep.SeasonNumber}
		if _, ok := groups[k]; !ok {
			order = append(order, k)
		}
		groups[k] = append(groups[k], ep)
	}
	var out []episodeMapping
	for _, k := range order {
		group := groups[k]
		for _, em := range rn.sceneEpisodeMappings(sh, group[0], mappings) {
			i := slices.IndexFunc(out, func(o episodeMapping) bool { return o.season == em.season && o.mode == em.mode })
			if i < 0 {
				em.episodes = slices.Clone(group)
				out = append(out, em)
				continue
			}
			for _, ep := range group {
				if !slices.Contains(out[i].episodes, ep) {
					out[i].episodes = append(out[i].episodes, ep)
				}
			}
			for _, t := range em.titles {
				if !slices.ContainsFunc(out[i].titles, func(o string) bool { return strings.EqualFold(o, t) }) {
					out[i].titles = append(out[i].titles, t)
				}
			}
		}
	}
	return out
}

// searchOf returns the search of episodes, as Sonarr's
// ReleaseSearchService.Get; titles are the scene titles searched by, nil
// for the series' scene names of the episodes' seasons and its title.
func (rn *run) searchOf(sh *show, kind series.SearchKind, episodes []*series.Episode, titles []string, monitoredOnly, userInvoked, interactive bool) *series.Search {
	s := sh.series
	if titles == nil {
		var seasons, sceneSeasons []int
		for _, ep := range episodes {
			seasons = append(seasons, ep.SeasonNumber)
			sceneSeasons = append(sceneSeasons, orInt(ep.SceneSeasonNumber, ep.SeasonNumber))
		}
		titles = rn.sceneMappings.GetSceneNames(s.TvdbID, seasonsOf(seasons), seasonsOf(sceneSeasons))
		if !slices.ContainsFunc(titles, func(t string) bool { return strings.EqualFold(t, s.Title) }) {
			titles = append(titles, s.Title)
		}
	}
	return &series.Search{
		Search: acquire.Search{TargetID: s.ID, UserInvoked: userInvoked, Interactive: interactive},
		Kind:   kind, Series: s, Episodes: episodes, SceneTitles: titles, MonitoredEpisodesOnly: monitoredOnly,
	}
}

// searchSingle searches for an episode by the seasons and numbers
// releases give it, as Sonarr's ReleaseSearchService.SearchSingle.
func (rn *run) searchSingle(ctx context.Context, b *batch, sh *show, ep *series.Episode, monitoredOnly, userInvoked, interactive bool) ([]series.Decision, error) {
	var decisions []series.Decision
	for _, m := range rn.sceneEpisodeMappings(sh, ep, rn.sceneMappings.FindByTvdbID(sh.series.TvdbID)) {
		s := rn.searchOf(sh, series.SingleEpisodeSearch, []*series.Episode{ep}, m.titles, monitoredOnly, userInvoked, interactive)
		s.SeasonNumber, s.EpisodeNumber = m.season, m.episodeNumber
		d, err := rn.dispatch(ctx, b, releases.EpisodeQuery{
			Kind: releases.QuerySingleEpisode, SeriesID: sh.item.ID, Titles: m.titles, Mode: queryMode(m.mode),
			Season: m.season, Episode: m.episodeNumber,
		}, s)
		if err != nil {
			return nil, err
		}
		decisions = append(decisions, d...)
	}
	return dedupe(decisions), nil
}

// searchDaily searches for an episode of a daily series by its air date,
// as Sonarr's ReleaseSearchService.SearchDaily.
func (rn *run) searchDaily(ctx context.Context, b *batch, sh *show, ep *series.Episode, monitoredOnly, userInvoked, interactive bool) ([]series.Decision, error) {
	airDate, err := time.Parse(time.DateOnly, ep.AirDate)
	if err != nil {
		return nil, fmt.Errorf("air date of %s: %w", ep.ID, err)
	}
	s := rn.searchOf(sh, series.DailyEpisodeSearch, []*series.Episode{ep}, nil, monitoredOnly, userInvoked, interactive)
	s.AirDate = airDate
	d, err := rn.dispatch(ctx, b, releases.EpisodeQuery{
		Kind: releases.QueryDailyEpisode, SeriesID: sh.item.ID, Titles: s.SceneTitles, AirDate: airDate,
	}, s)
	return dedupe(d), err
}

// searchAnime searches for an anime episode by its absolute number, as
// Sonarr's ReleaseSearchService.SearchAnime.
func (rn *run) searchAnime(ctx context.Context, b *batch, sh *show, ep *series.Episode, monitoredOnly, userInvoked, interactive, seasonSearch bool) ([]series.Decision, error) {
	s := rn.searchOf(sh, series.AnimeEpisodeSearch, []*series.Episode{ep}, nil, monitoredOnly, userInvoked, interactive)
	s.IsSeasonSearch = seasonSearch
	s.SeasonNumber = orInt(ep.SceneSeasonNumber, ep.SeasonNumber)
	s.EpisodeNumber = orInt(ep.SceneEpisodeNumber, ep.EpisodeNumber)
	s.AbsoluteEpisodeNumber = orInt(ep.SceneAbsoluteEpisodeNumber, ep.AbsoluteEpisodeNumber)
	d, err := rn.dispatch(ctx, b, releases.EpisodeQuery{
		Kind: releases.QueryAnimeEpisode, SeriesID: sh.item.ID, Titles: s.SceneTitles,
		Season: s.SeasonNumber, Episode: s.EpisodeNumber, Absolute: s.AbsoluteEpisodeNumber,
	}, s)
	return dedupe(d), err
}

// searchSpecial searches for specials by "<series> <episode title>" and
// by number, as Sonarr's ReleaseSearchService.SearchSpecial.
func (rn *run) searchSpecial(ctx context.Context, b *batch, sh *show, episodes []*series.Episode, monitoredOnly, userInvoked, interactive bool) ([]series.Decision, error) {
	s := rn.searchOf(sh, series.SpecialEpisodeSearch, episodes, nil, monitoredOnly, userInvoked, interactive)
	var clean []string
	for _, t := range s.SceneTitles {
		if c := releases.CleanSceneTitle(t); c != "" && !slices.Contains(clean, c) {
			clean = append(clean, c)
		}
	}
	for _, ep := range episodes {
		if strings.TrimSpace(ep.Title) == "" || (!interactive && monitoredOnly && !ep.Monitored) {
			continue
		}
		for _, t := range clean {
			q := t + " " + releases.CleanSceneTitle(ep.Title)
			if !slices.ContainsFunc(s.EpisodeQueryTitles, func(o string) bool { return strings.EqualFold(o, q) }) {
				s.EpisodeQueryTitles = append(s.EpisodeQueryTitles, q)
			}
		}
	}
	decisions, err := rn.dispatch(ctx, b, releases.EpisodeQuery{
		Kind: releases.QuerySpecial, SeriesID: sh.item.ID, Titles: s.SceneTitles, SpecialTitles: s.EpisodeQueryTitles,
	}, s)
	if err != nil {
		return nil, err
	}
	for _, ep := range episodes {
		if !interactive && monitoredOnly && !ep.Monitored {
			continue
		}
		d, err := rn.searchSingle(ctx, b, sh, ep, monitoredOnly, userInvoked, interactive)
		if err != nil {
			return nil, err
		}
		decisions = append(decisions, d...)
	}
	return dedupe(decisions), nil
}

// searchAnimeSeason searches for a season of an anime, as Sonarr's
// ReleaseSearchService.SearchAnimeSeason: by season where indexers allow
// it, then the aired episodes one by one.
func (rn *run) searchAnimeSeason(ctx context.Context, b *batch, sh *show, episodes []*series.Episode, monitoredOnly, userInvoked, interactive bool) ([]series.Decision, error) {
	now := rn.rt.now()
	var toSearch []*series.Episode
	for _, ep := range episodes {
		if (interactive || !monitoredOnly || ep.Monitored) && !ep.AirDateUTC.IsZero() && ep.AirDateUTC.Before(now) {
			toSearch = append(toSearch, ep)
		}
	}
	var decisions []series.Decision
	s := rn.searchOf(sh, series.AnimeSeasonSearch, episodes, nil, monitoredOnly, userInvoked, interactive)
	var seasons []int
	for _, m := range rn.sceneSeasonMappings(sh, toSearch) {
		if slices.Contains(seasons, m.season) {
			continue
		}
		seasons = append(seasons, m.season)
		s.SeasonNumber = m.season
		d, err := rn.dispatch(ctx, b, releases.EpisodeQuery{
			Kind: releases.QueryAnimeSeason, SeriesID: sh.item.ID, Titles: s.SceneTitles, Season: m.season,
		}, s)
		if err != nil {
			return nil, err
		}
		decisions = append(decisions, d...)
	}
	for _, ep := range toSearch {
		d, err := rn.searchAnime(ctx, b, sh, ep, monitoredOnly, userInvoked, interactive, true)
		if err != nil {
			return nil, err
		}
		decisions = append(decisions, d...)
	}
	return dedupe(decisions), nil
}

// searchDailySeason searches for episodes of a daily series by the years
// they aired, as Sonarr's ReleaseSearchService.SearchDailySeason: a year
// of one episode as that episode.
func (rn *run) searchDailySeason(ctx context.Context, b *batch, sh *show, episodes []*series.Episode, monitoredOnly, userInvoked, interactive bool) ([]series.Decision, error) {
	var years []int
	byYear := map[int][]*series.Episode{}
	for _, ep := range episodes {
		if !interactive && monitoredOnly && !ep.Monitored || ep.AirDate == "" {
			continue
		}
		t, err := time.Parse(time.DateOnly, ep.AirDate)
		if err != nil {
			continue
		}
		if _, ok := byYear[t.Year()]; !ok {
			years = append(years, t.Year())
		}
		byYear[t.Year()] = append(byYear[t.Year()], ep)
	}
	var decisions []series.Decision
	for _, y := range years {
		eps := byYear[y]
		var d []series.Decision
		var err error
		if len(eps) > 1 {
			s := rn.searchOf(sh, series.DailySeasonSearch, eps, nil, monitoredOnly, userInvoked, interactive)
			s.Year = y
			d, err = rn.dispatch(ctx, b, releases.EpisodeQuery{Kind: releases.QueryDailySeason, SeriesID: sh.item.ID, Titles: s.SceneTitles, Year: y}, s)
		} else {
			d, err = rn.searchDaily(ctx, b, sh, eps[0], monitoredOnly, userInvoked, interactive)
		}
		if err != nil {
			return nil, err
		}
		decisions = append(decisions, d...)
	}
	return dedupe(decisions), nil
}

// dispatch searches the indexers with a query and decides on what they
// found, as Sonarr's ReleaseSearchService.Dispatch.
func (rn *run) dispatch(ctx context.Context, b *batch, q releases.EpisodeQuery, s *series.Search) ([]series.Decision, error) {
	res, err := rn.rt.searcher().Search(ctx, releases.Search{Episodes: &q})
	if err != nil {
		return nil, err
	}
	for _, f := range res.Failures {
		rn.rt.logger().WarnContext(ctx, "indexer failed", "plugin", f.Provider, "indexer", f.Indexer, "err", f.Message)
	}
	// The episodes were searched for if an indexer was.
	if sh := rn.shows[q.SeriesID]; sh != nil && res.Searched > 0 {
		if err := rn.searchedEpisodes(ctx, sh, s.Episodes); err != nil {
			return nil, err
		}
	}
	rels := make([]*acquire.Release, 0, len(res.Releases))
	for _, rel := range res.Releases {
		rels = append(rels, b.add(FromSearch(rel)))
	}
	find, special, err := rn.findEpisodes(ctx, b)
	if err != nil {
		return nil, err
	}
	return rn.sengine.DecideEpisodes(ctx, rels, s, false, find, special), nil
}

// dedupe keeps one decision per release GUID, that with the fewest
// rejections, then of the indexer of highest priority, as Sonarr's
// ReleaseSearchService.DeDupeDecisions.
func dedupe(decisions []series.Decision) []series.Decision {
	var out []series.Decision
	index := map[string]int{}
	for _, d := range decisions {
		guid := d.Candidate.Release.GUID
		i, ok := index[guid]
		if !ok {
			index[guid] = len(out)
			out = append(out, d)
			continue
		}
		o := out[i]
		if c := cmp.Or(cmp.Compare(len(d.Rejections), len(o.Rejections)),
			cmp.Compare(d.Candidate.Release.IndexerPriority, o.Candidate.Release.IndexerPriority)); c < 0 {
			out[i] = d
		}
	}
	return out
}

// processEpisodes grabs the best approved releases of episodes, skipping
// those whose episodes were grabbed already, and keeps the delayed ones
// pending, as Sonarr's ProcessDownloadDecisions: when a protocol's
// download client fails, that protocol's later releases are kept pending
// too.
func (r *Runtime) processEpisodes(ctx context.Context, rn *run, b *batch, decisions []series.Decision) (processedEpisodes, error) {
	var out processedEpisodes
	var qualified []series.Decision
	for _, d := range decisions {
		if d.Rejected() {
			out.rejected = append(out.rejected, d)
		}
		if (d.Approved() || d.TemporarilyRejected()) && d.Candidate.Series != nil && len(d.Candidate.Episodes) > 0 {
			qualified = append(qualified, d)
		}
	}
	processedIn := func(list []series.Decision, d series.Decision) bool {
		return slices.ContainsFunc(list, func(o series.Decision) bool {
			return slices.ContainsFunc(o.Candidate.Episodes, func(e *series.Episode) bool {
				return slices.ContainsFunc(d.Candidate.Episodes, func(f *series.Episode) bool { return e.ID == f.ID })
			})
		})
	}
	failed := map[acquire.Protocol]bool{}
	var adds []pendingAdd
	keep := func(d series.Decision, reason core.PendingReason) {
		if processedIn(out.grabbed, d) || processedIn(out.pending, d) {
			reason = core.PendingFallback
		}
		ids := make([]core.ID, len(d.Candidate.Episodes))
		for i, ep := range d.Candidate.Episodes {
			ids[i] = ep.ID
		}
		adds = append(adds, pendingAdd{itemID: d.Candidate.Series.ID, itemIDs: ids, rel: b.of[d.Candidate.Release], reason: reason})
		out.pending = append(out.pending, d)
	}
	for _, d := range rn.senv.Prioritize(qualified) {
		if processedIn(out.grabbed, d) {
			continue
		}
		if d.TemporarilyRejected() {
			keep(d, core.PendingDelay)
			continue
		}
		protocol := d.Candidate.Release.Protocol
		if failed[protocol] {
			keep(d, core.PendingClientUnavailable)
			continue
		}
		switch r.grabEpisodeDecision(ctx, rn, b, d) {
		case grabGrabbed:
			out.grabbed = append(out.grabbed, d)
		case grabRejected:
			out.rejected = append(out.rejected, d)
		case grabFailed:
			keep(d, core.PendingClientUnavailable)
			failed[protocol] = true
		}
	}
	if len(adds) > 0 {
		if err := r.addPending(ctx, adds); err != nil {
			return out, err
		}
	}
	return out, nil
}

// processedEpisodes is what processing episode decisions did.
type processedEpisodes struct {
	grabbed, pending, rejected []series.Decision
}

// grabEpisodeDecision grabs an approved episode release and tells how it
// went.
func (r *Runtime) grabEpisodeDecision(ctx context.Context, rn *run, b *batch, d series.Decision) grabResult {
	c := d.Candidate
	sh, err := rn.show(ctx, c.Series.ID)
	if err != nil {
		r.logger().WarnContext(ctx, "couldn't grab release", "release", c.Release.Title, "err", err)
		return grabSkipped
	}
	_, err = r.grab(ctx, rn, grabRequest{
		show: sh, episodes: c.Episodes, rel: b.of[c.Release], candidate: c.Candidate,
		fullSeason: c.Parsed != nil && c.Parsed.FullSeason, filterBlocked: b.waitsForClient(c.Release),
	})
	switch {
	case err == nil:
		return grabGrabbed
	case errors.Is(err, errReleaseUnavailable):
		r.logger().WarnContext(ctx, "release not available", "release", c.Release.Title, "indexer", c.Release.Indexer, "err", err)
		return grabRejected
	case errors.Is(err, errClientUnavailable):
		r.logger().DebugContext(ctx, "download client unavailable; keeping release pending", "release", c.Release.Title, "err", err)
		return grabFailed
	}
	r.logger().WarnContext(ctx, "couldn't add release to the download queue", "release", c.Release.Title, "indexer", c.Release.Indexer, "err", err)
	return grabSkipped
}

// wantedEpisode is a monitored episode of a monitored wanted series a
// wanted filter selects.
type wantedEpisode struct {
	show    *show
	episode *series.Episode
	// aired is whether it has aired.
	aired bool
}

// wantedEpisodes returns the monitored episodes of monitored wanted
// series a filter selects, of one series or of all when seriesID is
// zero, as Sonarr's EpisodeService.EpisodesWithoutFiles (aired only) and
// EpisodeCutoffService.EpisodesWhereCutoffUnmet.
func (rn *run) wantedEpisodes(ctx context.Context, filter WantedFilter, seriesID core.ID) ([]wantedEpisode, error) {
	var shows []*show
	if seriesID.IsZero() {
		all, err := rn.allShows(ctx)
		if err != nil {
			return nil, err
		}
		shows = all
	} else {
		sh, err := rn.show(ctx, seriesID)
		if err != nil {
			return nil, err
		}
		shows = []*show{sh}
	}
	upgrades := acquire.Upgrades{Propers: rn.env.Config.Propers}
	now := rn.rt.now()
	var out []wantedEpisode
	for _, sh := range shows {
		if !sh.item.Monitored {
			continue
		}
		for _, ep := range sh.episodes {
			if !ep.Monitored {
				continue
			}
			aired := !ep.AirDateUTC.IsZero() && !ep.AirDateUTC.After(now)
			switch filter {
			case WantedMissing:
				if ep.File != nil || !aired {
					continue
				}
			case WantedCutoffUnmet:
				if ep.File == nil || !upgrades.CutoffNotMet(sh.series.Profile, ep.File.Quality, ep.File.Formats, nil) {
					continue
				}
			}
			out = append(out, wantedEpisode{show: sh, episode: ep, aired: aired})
		}
	}
	return out, nil
}

// WantedEpisode is a monitored episode of a wanted series a filter
// selects.
type WantedEpisode struct {
	// ID is the episode's: its item's, or of an episode a multi-episode
	// file covers beyond its first.
	ID                        core.ID
	Series                    core.Item
	Wanted                    core.Wanted
	Season, Episode, Absolute int
	Name                      string
	// AirTime is zero when unknown.
	AirTime time.Time
	// Quality is the title of its file's quality; empty without a file.
	Quality string
}

// ListWantedEpisodes returns the monitored episodes a filter selects,
// most recently aired first, a page of them and how many there are, as
// Sonarr's missing and cutoff unmet lists.
func (r *Runtime) ListWantedEpisodes(ctx context.Context, filter WantedFilter, limit, offset int) ([]WantedEpisode, int, error) {
	rn, err := r.newRun(ctx)
	if err != nil {
		return nil, 0, err
	}
	wanted, err := rn.wantedEpisodes(ctx, filter, core.NilID)
	if err != nil {
		return nil, 0, err
	}
	all := make([]WantedEpisode, 0, len(wanted))
	for _, we := range wanted {
		ep := we.episode
		e := WantedEpisode{
			ID: ep.ID, Series: we.show.item, Wanted: we.show.wanted, Season: ep.SeasonNumber, Episode: ep.EpisodeNumber,
			Absolute: ep.AbsoluteEpisodeNumber, Name: ep.Title, AirTime: ep.AirDateUTC,
		}
		if ep.File != nil {
			e.Quality = rn.qualityTitle(ep.File.Quality.Quality)
		}
		all = append(all, e)
	}
	slices.SortStableFunc(all, func(a, b WantedEpisode) int {
		return cmp.Or(b.AirTime.Compare(a.AirTime), cmp.Compare(a.ID.String(), b.ID.String()))
	})
	if limit <= 0 || limit > core.MaxPageSize {
		limit = core.MaxPageSize
	}
	total := len(all)
	if offset >= total {
		return nil, total, nil
	}
	return all[offset:min(offset+limit, total)], total, nil
}
