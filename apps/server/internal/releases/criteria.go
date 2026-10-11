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
	case core.KindSeason, core.KindEpisode:
	default:
		return nil, fmt.Errorf("%s %s is not searched for releases, only movies, seasons and episodes: %w", item.Kind, item.ID, core.ErrInvalid)
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
			return nil, fmt.Errorf("seasons of anime are searched by episode: %w", core.ErrInvalid)
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
	query := slices.Contains(params, "q")
	if !query && len(ids) == 0 {
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
	if query {
		for _, t := range cleanTitles(e.titles) {
			byTitle = append(byTitle, numbered(c.request(tv, t, nil)))
		}
	}
	return [][]*pluginv1.SearchReleasesRequest{byID, byTitle}
}

func (c *criteria) request(t pluginv1.IndexerSearchType, query string, ids map[string]string) *pluginv1.SearchReleasesRequest {
	return pluginv1.SearchReleasesRequest_builder{
		Type: &t, Query: &query, Categories: c.categories, ExternalIds: ids,
	}.Build()
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
