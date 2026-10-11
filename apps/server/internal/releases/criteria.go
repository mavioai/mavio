package releases

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"github.com/mavioai/mavio/libs/core"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/release"
)

// The Newznab categories searched when none are given: Radarr's movies,
// Sonarr's series and its anime.
const (
	moviesCategory int32 = 2000
	tvCategory     int32 = 5000
	animeCategory  int32 = 5070
)

// criteria are what an item or a text search looks for.
type criteria struct {
	// kind is how releases are parsed: Movie, Series, or zero to guess.
	kind       release.Kind
	categories []int32
	text       *Search
	movie      *movieCriteria
	episode    *episodeCriteria
	// seasonEpisodes are the episodes of a season of anime, or the
	// seasons of a whole series, searched one by one in place of episode.
	seasonEpisodes []*episodeCriteria
	// limits holds the age and size limits of the requests; nil for none.
	limits *Search
	// animeStandardFormat is set for indexers that search anime by season
	// and episode too.
	animeStandardFormat bool
}

// movieCriteria are those of Radarr's MovieSearchCriteria.
type movieCriteria struct {
	titles []string
	year   int
	ids    map[string]string
}

// episodeCriteria are those of Sonarr's single episode, daily episode,
// anime episode and season searches.
type episodeCriteria struct {
	titles     []string
	ids        map[string]string
	seriesType core.SeriesType
	season     int
	// episode is zero for a season search.
	episode  int
	absolute int
	airDate  time.Time
	// query is the episode query the criteria were made of, or nil for
	// those of an item.
	query *EpisodeQuery
}

// The IDs Newznab searches take for movies and for series, in Radarr's
// and Sonarr's order, by the keys of IndexerProviderService.
var (
	movieIDParams = []idParam{{"tmdb", "tmdbid"}, {"imdb", "imdbid"}}
	tvIDParams    = []idParam{{"tvdb", "tvdbid"}, {"imdb", "imdbid"}, {"tvrage", "rid"}, {"tvmaze", "tvmazeid"}, {"tmdb", "tmdbid"}}
)

type idParam struct{ key, param string }

// criteria returns the criteria of a search, reading its item.
func (s *Searcher) criteria(ctx context.Context, q Search) (*criteria, error) {
	if q.Episodes != nil {
		if s.Items == nil {
			return nil, fmt.Errorf("series %s: %w", q.Episodes.SeriesID, core.ErrNotFound)
		}
		series, err := s.Items.Get(ctx, q.Episodes.SeriesID)
		if err != nil {
			return nil, err
		}
		if series.Kind != core.KindSeries {
			return nil, fmt.Errorf("%s %s is not a series: %w", series.Kind, series.ID, core.ErrInvalid)
		}
		return s.episodeQueryCriteria(series, q)
	}
	if q.ItemID == core.NilID {
		c := &criteria{text: &q, categories: q.Categories}
		switch q.Type {
		case pluginv1.IndexerSearchType_INDEXER_SEARCH_TYPE_MOVIE:
			c.kind = release.Movie
		case pluginv1.IndexerSearchType_INDEXER_SEARCH_TYPE_TV:
			c.kind = release.Series
		}
		return c, nil
	}
	if s.Items == nil {
		return nil, fmt.Errorf("item %s: %w", q.ItemID, core.ErrNotFound)
	}
	item, err := s.Items.Get(ctx, q.ItemID)
	if err != nil {
		return nil, err
	}
	switch item.Kind {
	case core.KindMovie:
		return &criteria{
			kind: release.Movie, categories: or(q.Categories, moviesCategory),
			movie: &movieCriteria{titles: titles(item), year: item.ProductionYear, ids: externalIDs(item, movieIDParams)},
		}, nil
	case core.KindSeries:
		return s.seriesCriteria(ctx, item, q)
	case core.KindSeason, core.KindEpisode:
	default:
		return nil, fmt.Errorf("%s %s is not searched for releases, only movies, series, seasons and episodes: %w", item.Kind, item.ID, core.ErrInvalid)
	}
	series, err := s.series(ctx, item)
	if err != nil {
		return nil, err
	}
	e := &episodeCriteria{titles: titles(series), ids: externalIDs(series, tvIDParams), seriesType: series.SeriesType}
	if item.Kind == core.KindSeason {
		if item.IndexNumber == nil {
			return nil, fmt.Errorf("season %s has no number: %w", item.ID, core.ErrInvalid)
		}
		e.season = *item.IndexNumber
		if series.SeriesType == core.SeriesAnime {
			episodes, err := s.animeSeasonEpisodes(ctx, item, series, e)
			if err != nil {
				return nil, err
			}
			return &criteria{kind: release.Series, categories: or(q.Categories, animeCategory), seasonEpisodes: episodes}, nil
		}
	} else {
		if item.IndexNumber == nil || item.ParentIndexNumber == nil {
			return nil, fmt.Errorf("episode %s has no season or episode number: %w", item.ID, core.ErrInvalid)
		}
		e.season, e.episode = *item.ParentIndexNumber, *item.IndexNumber
		if item.AbsoluteNumber != nil {
			e.absolute = *item.AbsoluteNumber
		}
		if item.PremiereDate != nil {
			e.airDate = *item.PremiereDate
		}
		switch {
		case series.SeriesType == core.SeriesDaily && e.airDate.IsZero():
			return nil, fmt.Errorf("episode %s of a daily series has no air date: %w", item.ID, core.ErrInvalid)
		case series.SeriesType == core.SeriesAnime && e.absolute == 0:
			return nil, fmt.Errorf("episode %s of an anime has no absolute number: %w", item.ID, core.ErrInvalid)
		}
	}
	category := tvCategory
	if series.SeriesType == core.SeriesAnime {
		category = animeCategory
	}
	return &criteria{kind: release.Series, categories: or(q.Categories, category), episode: e}, nil
}

// seriesCriteria returns the criteria of an interactive search of a whole
// series: each of its seasons but the specials, searched as Sonarr's
// SeriesSearchService searches a series season by season, and the
// seasons of anime episode by episode as its SearchAnimeSeason.
func (s *Searcher) seriesCriteria(ctx context.Context, series core.Item, q Search) (*criteria, error) {
	base := episodeCriteria{titles: titles(series), ids: externalIDs(series, tvIDParams), seriesType: series.SeriesType}
	var seasons []core.Item
	for it, err := range s.Items.Walk(ctx, core.ItemQuery{
		ParentID: series.ID, Kinds: []core.ItemKind{core.KindSeason}, IncludeVirtual: true, IncludeMissing: true,
	}) {
		if err != nil {
			return nil, err
		}
		if it.IndexNumber != nil && *it.IndexNumber > 0 {
			seasons = append(seasons, it)
		}
	}
	slices.SortFunc(seasons, func(a, b core.Item) int { return *a.IndexNumber - *b.IndexNumber })
	var searches []*episodeCriteria
	for _, season := range seasons {
		e := base
		e.season = *season.IndexNumber
		if series.SeriesType != core.SeriesAnime {
			searches = append(searches, &e)
			continue
		}
		episodes, err := s.animeSeasonEpisodes(ctx, season, series, &e)
		if err != nil {
			return nil, err
		}
		searches = append(searches, episodes...)
	}
	if len(searches) == 0 {
		return nil, fmt.Errorf("series %s has no seasons to search: %w", series.ID, core.ErrInvalid)
	}
	category := tvCategory
	if series.SeriesType == core.SeriesAnime {
		category = animeCategory
	}
	return &criteria{kind: release.Series, categories: or(q.Categories, category), seasonEpisodes: searches}, nil
}

// animeSeasonEpisodes returns the criteria of the episodes of a season of
// anime that have aired and have absolute numbers, as Sonarr's
// SearchAnimeSeason searches them one by one, each as an anime episode;
// season is the season's criteria.
func (s *Searcher) animeSeasonEpisodes(ctx context.Context, item, series core.Item, season *episodeCriteria) ([]*episodeCriteria, error) {
	now := s.now()
	var out []*episodeCriteria
	for ep, err := range s.Items.Walk(ctx, core.ItemQuery{
		ParentID: item.ID, Kinds: []core.ItemKind{core.KindEpisode}, IncludeVirtual: true, IncludeMissing: true,
	}) {
		if err != nil {
			return nil, err
		}
		if ep.PremiereDate == nil || !ep.PremiereDate.Before(now) || ep.AbsoluteNumber == nil || *ep.AbsoluteNumber <= 0 || ep.IndexNumber == nil {
			continue
		}
		e := *season
		e.episode, e.absolute, e.airDate = *ep.IndexNumber, *ep.AbsoluteNumber, *ep.PremiereDate
		out = append(out, &e)
	}
	slices.SortFunc(out, func(a, b *episodeCriteria) int { return a.absolute - b.absolute })
	return out, nil
}

// series returns the series of a season or episode.
func (s *Searcher) series(ctx context.Context, item core.Item) (core.Item, error) {
	for range 3 {
		if item.ParentID == core.NilID {
			break
		}
		parent, err := s.Items.Get(ctx, item.ParentID)
		if err != nil {
			return core.Item{}, err
		}
		if parent.Kind == core.KindSeries {
			return parent, nil
		}
		item = parent
	}
	return core.Item{}, fmt.Errorf("%s %s has no series: %w", item.Kind, item.ID, core.ErrInvalid)
}

func or(categories []int32, category ...int32) []int32 {
	if len(categories) > 0 {
		return categories
	}
	return category
}

// titles returns an item's titles, the original after the name.
func titles(item core.Item) []string {
	var out []string
	for _, t := range []string{item.Name, item.OriginalTitle} {
		if t = strings.TrimSpace(t); t != "" && !slices.Contains(out, t) {
			out = append(out, t)
		}
	}
	return out
}

func externalIDs(item core.Item, params []idParam) map[string]string {
	ids := map[string]string{}
	for _, p := range params {
		if v := strings.TrimSpace(item.ExternalIDs[core.Provider(p.key)]); v != "" {
			ids[p.key] = v
		}
	}
	return ids
}

// searches returns the searches of an indexer with caps, each tiers of
// requests: one search, or one by episode of a season of anime, or by
// season of a whole series.
func (c *criteria) searches(caps *pluginv1.IndexerCapabilities) [][][]*pluginv1.SearchReleasesRequest {
	if c.seasonEpisodes == nil {
		return [][][]*pluginv1.SearchReleasesRequest{c.tiers(caps)}
	}
	var out [][][]*pluginv1.SearchReleasesRequest
	for _, e := range c.seasonEpisodes {
		ec := *c
		ec.seasonEpisodes, ec.episode = nil, e
		if tiers := ec.tiers(caps); len(tiers) > 0 {
			out = append(out, tiers)
		}
	}
	return out
}

// tiers returns the requests an indexer with caps is sent, tier by tier;
// a tier is tried only when those before it found nothing.
func (c *criteria) tiers(caps *pluginv1.IndexerCapabilities) [][]*pluginv1.SearchReleasesRequest {
	switch {
	case c.movie != nil:
		return c.movieTiers(caps)
	case c.episode != nil && c.episode.query != nil:
		return c.queryTiers(caps)
	case c.episode != nil:
		return c.episodeTiers(caps)
	}
	t := c.text
	req := c.request(t.Type, t.Query, nil)
	if t.Season != nil {
		req.SetSeason(int32(*t.Season))
	}
	req.SetEpisode(t.Episode)
	return [][]*pluginv1.SearchReleasesRequest{{req}}
}

// movieTiers are Radarr's: the IDs the indexer takes, then a basic search
// of each title with the year.
func (c *criteria) movieTiers(caps *pluginv1.IndexerCapabilities) [][]*pluginv1.SearchReleasesRequest {
	m := c.movie
	var ids, titles []*pluginv1.SearchReleasesRequest
	if found := supported(m.ids, movieIDParams, caps.GetMovieSearchParams()); len(found) > 0 {
		ids = append(ids, c.request(pluginv1.IndexerSearchType_INDEXER_SEARCH_TYPE_MOVIE, "", found))
	}
	if len(caps.GetSearchParams()) > 0 && m.year > 0 {
		for _, t := range cleanTitles(m.titles) {
			titles = append(titles, c.request(pluginv1.IndexerSearchType_INDEXER_SEARCH_TYPE_UNSPECIFIED, fmt.Sprintf("%s %d", t, m.year), nil))
		}
	}
	return [][]*pluginv1.SearchReleasesRequest{ids, titles}
}

// episodeTiers are Sonarr's: by the series' IDs the indexer takes, then by
// its titles, with the season and episode, or the air date of a daily
// series; anime by absolute number, by IDs and titles at once.
func (c *criteria) episodeTiers(caps *pluginv1.IndexerCapabilities) [][]*pluginv1.SearchReleasesRequest {
	e := c.episode
	tv := pluginv1.IndexerSearchType_INDEXER_SEARCH_TYPE_TV
	params := caps.GetTvSearchParams()
	ids := supported(e.ids, tvIDParams, params)
	if e.seriesType == core.SeriesAnime {
		if len(caps.GetSearchParams()) == 0 {
			return nil
		}
		number := fmt.Sprintf("%02d", e.absolute)
		var tier []*pluginv1.SearchReleasesRequest
		if len(ids) > 0 {
			tier = append(tier, c.request(tv, number, ids))
		}
		for _, t := range cleanTitles(e.titles) {
			tier = append(tier, c.request(pluginv1.IndexerSearchType_INDEXER_SEARCH_TYPE_UNSPECIFIED, t+" "+number, nil))
		}
		return [][]*pluginv1.SearchReleasesRequest{tier}
	}

	if !slices.Contains(params, "season") || e.episode > 0 && !slices.Contains(params, "ep") {
		return nil
	}
	if !textSearch(params) && len(ids) == 0 {
		return nil
	}
	numbered := func(req *pluginv1.SearchReleasesRequest) *pluginv1.SearchReleasesRequest {
		switch {
		case e.seriesType == core.SeriesDaily && e.episode > 0:
			req.SetSeason(int32(e.airDate.Year()))
			req.SetEpisode(e.airDate.Format("01/02"))
		default:
			req.SetSeason(int32(e.season))
			if e.episode > 0 {
				req.SetEpisode(fmt.Sprint(e.episode))
			}
		}
		return req
	}
	var byID, byTitle []*pluginv1.SearchReleasesRequest
	if len(ids) > 0 {
		byID = append(byID, numbered(c.request(tv, "", ids)))
	}
	byTitle = c.titleRequests(params, e.titles, numbered)
	return [][]*pluginv1.SearchReleasesRequest{byID, byTitle}
}

// textSearch reports whether an indexer taking the TV search parameters
// searches series by text, as Sonarr's SupportsTvTextSearches.
func textSearch(params []string) bool {
	return slices.Contains(params, "q") || slices.Contains(params, "title")
}

// titleRequests are the TV searches of a series by its titles, as
// Sonarr's AddTitlePageableRequests makes them: by Newznab's title, of
// the titles as they are, for indexers taking it, else by query, of the
// clean titles.
func (c *criteria) titleRequests(params, titles []string, numbered func(*pluginv1.SearchReleasesRequest) *pluginv1.SearchReleasesRequest) []*pluginv1.SearchReleasesRequest {
	tv := pluginv1.IndexerSearchType_INDEXER_SEARCH_TYPE_TV
	var out []*pluginv1.SearchReleasesRequest
	switch {
	case slices.Contains(params, "title"):
		var seen []string
		for _, t := range titles {
			if t = strings.TrimSpace(t); t == "" || slices.Contains(seen, t) {
				continue
			}
			seen = append(seen, t)
			req := c.request(tv, "", nil)
			req.SetTitle(t)
			out = append(out, numbered(req))
		}
	case slices.Contains(params, "q"):
		for _, t := range cleanTitles(titles) {
			out = append(out, numbered(c.request(tv, t, nil)))
		}
	}
	return out
}

func (c *criteria) request(t pluginv1.IndexerSearchType, query string, ids map[string]string) *pluginv1.SearchReleasesRequest {
	b := pluginv1.SearchReleasesRequest_builder{
		Type: &t, Query: &query, Categories: c.categories, ExternalIds: ids,
	}
	if l := c.limits; l != nil {
		b.MinAge, b.MaxAge, b.MinSize, b.MaxSize = l.MinAge, l.MaxAge, l.MinSize, l.MaxSize
	}
	return b.Build()
}

// supported returns the IDs whose Newznab parameters an indexer takes.
func supported(ids map[string]string, params []idParam, takes []string) map[string]string {
	out := map[string]string{}
	for _, p := range params {
		if v := ids[p.key]; v != "" && slices.Contains(takes, p.param) {
			out[p.key] = v
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// cleanTitles returns the titles as Radarr's and Sonarr's
// CleanSceneTitles make them, with spaces between words.
func cleanTitles(titles []string) []string {
	var out []string
	for _, t := range titles {
		if t = cleanTitle(t); t != "" && !slices.Contains(out, t) {
			out = append(out, t)
		}
	}
	return out
}

// CleanSceneTitle returns a title as indexers are searched by it, as
// Sonarr's SearchCriteriaBase.GetCleanSceneTitle and NewsnabifyTitle make
// it.
func CleanSceneTitle(title string) string { return cleanTitle(title) }

// cleanTitle is GetCleanSceneTitle followed by NewsnabifyTitle: a leading
// "The" dropped, "&" spelled out, apostrophes and periods dropped and other
// characters but letters and digits made separators, accents removed.
func cleanTitle(title string) string {
	if len(title) > 4 && strings.EqualFold(title[:3], "the") && unicode.IsSpace(rune(title[3])) {
		title = title[4:]
	}
	title = strings.ReplaceAll(title, "&", "and")
	var b strings.Builder
	sep := false
	for _, r := range norm.NFD.String(title) {
		switch {
		case strings.ContainsRune("'.`´‘’", r) || unicode.Is(unicode.Mn, r):
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_':
			if sep && b.Len() > 0 {
				b.WriteByte(' ')
			}
			sep = false
			b.WriteRune(r)
		default:
			sep = true
		}
	}
	return b.String()
}
