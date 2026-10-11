package releases

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/mavioai/mavio/libs/core"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/release"
)

// EpisodeQueryKind is what an episode query looks for, as the subclasses
// of Sonarr's SearchCriteriaBase.
type EpisodeQueryKind int

// The episode query kinds.
const (
	// QuerySingleEpisode looks for an episode by Season and Episode.
	QuerySingleEpisode EpisodeQueryKind = iota
	// QuerySeason looks for a season pack by Season.
	QuerySeason
	// QueryDailyEpisode looks for an episode of a daily series by AirDate.
	QueryDailyEpisode
	// QueryDailySeason looks for the episodes of a daily series aired in
	// Year.
	QueryDailySeason
	// QueryAnimeEpisode looks for an anime episode by Absolute.
	QueryAnimeEpisode
	// QueryAnimeSeason looks for a season pack of an anime by Season.
	QueryAnimeSeason
	// QuerySpecial looks for specials by SpecialTitles.
	QuerySpecial
)

// SearchMode is how a query searches by the series' IDs and titles, as
// Sonarr's SearchMode flags.
type SearchMode int

// The search modes.
const (
	// SearchDefault searches by IDs, then by titles when the IDs found
	// nothing.
	SearchDefault SearchMode = 0
	// SearchByID searches by IDs.
	SearchByID SearchMode = 1
	// SearchByTitle searches by titles.
	SearchByTitle SearchMode = 2
)

// EpisodeQuery is a search for episodes of a series, as Sonarr's search
// criteria make it: with the scene titles and numbers releases use.
type EpisodeQuery struct {
	Kind EpisodeQueryKind
	// SeriesID is the series, searched by its external IDs.
	SeriesID core.ID
	// Titles are the titles to search by, scene titles among them; empty
	// for the series' own.
	Titles []string
	Mode   SearchMode
	// Season and Episode are as releases number them.
	Season, Episode int
	Absolute        int
	AirDate         time.Time
	Year            int
	// SpecialTitles are the queries of specials, "<series> <episode
	// title>".
	SpecialTitles []string
}

// episodeQueryCriteria returns the criteria of an episode query.
func (s *Searcher) episodeQueryCriteria(series core.Item, q Search) (*criteria, error) {
	eq := q.Episodes
	queryTitles := eq.Titles
	if len(queryTitles) == 0 {
		queryTitles = titles(series)
	}
	e := &episodeCriteria{
		titles: queryTitles, ids: externalIDs(series, tvIDParams), seriesType: series.SeriesType, query: eq,
		season: eq.Season, episode: eq.Episode, absolute: eq.Absolute, airDate: eq.AirDate,
	}
	categories := []int32{tvCategory}
	switch eq.Kind {
	case QueryAnimeEpisode, QueryAnimeSeason:
		categories = []int32{animeCategory}
	case QuerySpecial:
		categories = []int32{tvCategory, animeCategory}
	case QueryDailyEpisode:
		if eq.AirDate.IsZero() {
			return nil, fmt.Errorf("daily episode query without an air date: %w", core.ErrInvalid)
		}
	}
	return &criteria{kind: release.Series, categories: or(q.Categories, categories...), episode: e}, nil
}

// queryTiers are the requests of an episode query, as Sonarr's
// NewznabRequestGenerator.GetSearchRequests of its criteria: by the IDs
// the indexer takes, and by titles as the search mode says; anime by
// absolute number; specials by their titles.
func (c *criteria) queryTiers(caps *pluginv1.IndexerCapabilities) [][]*pluginv1.SearchReleasesRequest {
	e := c.episode
	q := e.query
	tv := pluginv1.IndexerSearchType_INDEXER_SEARCH_TYPE_TV
	basic := pluginv1.IndexerSearchType_INDEXER_SEARCH_TYPE_UNSPECIFIED
	params := caps.GetTvSearchParams()
	ids := supported(e.ids, tvIDParams, params)
	searchable := len(caps.GetSearchParams()) > 0
	switch q.Kind {
	case QueryAnimeEpisode:
		if !searchable {
			return nil
		}
		number := fmt.Sprintf("%02d", q.Absolute)
		var tier []*pluginv1.SearchReleasesRequest
		if len(ids) > 0 {
			tier = append(tier, c.request(tv, number, ids))
		}
		for _, t := range cleanTitles(e.titles) {
			tier = append(tier, c.request(basic, t+" "+number, nil))
		}
		return [][]*pluginv1.SearchReleasesRequest{tier}
	case QueryAnimeSeason:
		// Sonarr searches season packs of anime only for indexers set to
		// search anime in the standard format, which the plugins do not
		// tell.
		return nil
	case QuerySpecial:
		if !searchable {
			return nil
		}
		var tier []*pluginv1.SearchReleasesRequest
		for _, t := range q.SpecialTitles {
			tier = append(tier, c.request(basic, strings.ReplaceAll(t, "+", " "), nil))
		}
		return [][]*pluginv1.SearchReleasesRequest{tier}
	}
	needsEpisode := q.Kind == QuerySingleEpisode || q.Kind == QueryDailyEpisode
	if !slices.Contains(params, "season") || needsEpisode && !slices.Contains(params, "ep") {
		return nil
	}
	if !textSearch(params) && len(ids) == 0 {
		return nil
	}
	numbered := func(req *pluginv1.SearchReleasesRequest) *pluginv1.SearchReleasesRequest {
		switch q.Kind {
		case QueryDailyEpisode:
			req.SetSeason(int32(q.AirDate.Year()))
			req.SetEpisode(q.AirDate.Format("01/02"))
		case QueryDailySeason:
			req.SetSeason(int32(q.Year))
		case QuerySeason:
			req.SetSeason(int32(q.Season))
		default:
			req.SetSeason(int32(q.Season))
			req.SetEpisode(fmt.Sprint(q.Episode))
		}
		return req
	}
	byTitles := func() []*pluginv1.SearchReleasesRequest {
		return c.titleRequests(params, e.titles, numbered)
	}
	var first, second []*pluginv1.SearchReleasesRequest
	if (q.Mode&SearchByID != 0 || q.Mode == SearchDefault) && len(ids) > 0 {
		first = append(first, numbered(c.request(tv, "", ids)))
	}
	if q.Mode&SearchByTitle != 0 {
		first = append(first, byTitles()...)
	}
	if q.Mode == SearchDefault {
		second = byTitles()
	}
	return [][]*pluginv1.SearchReleasesRequest{first, second}
}
