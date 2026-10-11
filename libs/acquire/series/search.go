package series

import (
	"time"

	"github.com/mavioai/mavio/libs/acquire"
)

// SearchKind is what a search asked indexers for, as the subclasses of
// Sonarr's SearchCriteriaBase.
type SearchKind int

// The search kinds.
const (
	// SingleEpisodeSearch is for an episode by season and episode
	// number: SeasonNumber and EpisodeNumber.
	SingleEpisodeSearch SearchKind = iota
	// SeasonSearch is for a season: SeasonNumber.
	SeasonSearch
	// DailyEpisodeSearch is for an episode of a daily series: AirDate.
	DailyEpisodeSearch
	// DailySeasonSearch is for a year of a daily series: Year.
	DailySeasonSearch
	// AnimeEpisodeSearch is for an anime episode: AbsoluteEpisodeNumber,
	// SeasonNumber, EpisodeNumber and IsSeasonSearch.
	AnimeEpisodeSearch
	// AnimeSeasonSearch is for a season of an anime: SeasonNumber.
	AnimeSeasonSearch
	// SpecialEpisodeSearch is for specials by title:
	// EpisodeQueryTitles.
	SpecialEpisodeSearch
)

// Search is what a search for episodes was for. A nil *Search means an
// RSS sync or a pushed release.
type Search struct {
	// Search's TargetID and ItemIDs may be left empty; they are taken
	// from Series and Episodes.
	acquire.Search
	Kind   SearchKind
	Series *Series
	// Episodes are the episodes searched for.
	Episodes []*Episode
	// SceneTitles are the titles the series was searched by.
	SceneTitles []string
	// MonitoredEpisodesOnly is set when only monitored episodes are
	// wanted.
	MonitoredEpisodesOnly bool

	SeasonNumber          int
	EpisodeNumber         int
	AbsoluteEpisodeNumber int
	// IsSeasonSearch is set on anime episode searches made for a whole
	// season.
	IsSeasonSearch bool
	AirDate        time.Time
	Year           int
	// EpisodeQueryTitles are the titles specials are searched by.
	EpisodeQueryTitles []string
}

// root returns the search as the root package sees it, or nil.
func (s *Search) root() *acquire.Search {
	if s == nil {
		return nil
	}
	r := s.Search
	if r.TargetID.IsZero() && s.Series != nil {
		r.TargetID = s.Series.ID
	}
	if len(r.ItemIDs) == 0 {
		r.ItemIDs = episodeIDs(s.Episodes)
	}
	return &r
}
