package series

import (
	"slices"
	"time"

	"github.com/mavioai/mavio/libs/acquire"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/release"
)

// Type is how a series numbers its episodes, as Sonarr's SeriesTypes.
type Type int

// The series types.
const (
	// Standard series number episodes by season and episode.
	Standard Type = iota
	// Daily series name episodes by air date.
	Daily
	// Anime series number episodes absolutely.
	Anime
)

// Series is a series releases are for. Its Target's Kind is
// release.Series.
type Series struct {
	acquire.Target
	Type Type
	// Runtime is the usual runtime of an episode in minutes; 0 when
	// unknown.
	Runtime int
	// Seasons are the numbers of the series' seasons, 0 for specials.
	Seasons []int
	// TvdbID, TvRageID and ImdbID are the series' external IDs; zero or
	// empty when unknown.
	TvdbID   int
	TvRageID int
	ImdbID   string
	// CleanTitle is release.CleanSeriesTitle of the title; empty to work
	// it out from Title.
	CleanTitle string
	// UseSceneNumbering is set when the scene numbers the series' episodes
	// apart from TVDB, as XEM tells.
	UseSceneNumbering bool
}

// cleanTitle returns the series' clean title.
func (s *Series) cleanTitle() string {
	if s.CleanTitle != "" {
		return s.CleanTitle
	}
	return release.CleanSeriesTitle(s.Title)
}

// Episode is an episode of a series. Its Item's Runtime is the episode's
// own runtime, 0 when unknown.
type Episode struct {
	acquire.Item
	SeasonNumber          int
	EpisodeNumber         int
	AbsoluteEpisodeNumber int
	Title                 string
	// AirDateUTC is zero when the air date is unknown.
	AirDateUTC time.Time
	// AirDate is the local air date as "2006-01-02"; empty when unknown.
	AirDate string
	// SceneSeasonNumber, SceneEpisodeNumber and SceneAbsoluteEpisodeNumber
	// are how the scene numbers the episode, nil when as TVDB does.
	SceneSeasonNumber          *int
	SceneEpisodeNumber         *int
	SceneAbsoluteEpisodeNumber *int
	// UnverifiedSceneNumbering is set when the scene numbering of the
	// episode is extrapolated from that of others, not confirmed.
	UnverifiedSceneNumbering bool
}

// SceneMapping is a title the scene gives a series, as Sonarr's
// SceneMapping.
type SceneMapping struct {
	Title  string
	TvdbID int
	// SearchTerm is the title to search by; ParseTerm is
	// release.CleanSeriesTitle of Title, worked out when empty.
	SearchTerm, ParseTerm string
	// SeasonNumber is the TVDB season the title is of, and
	// SceneSeasonNumber the scene's; nil (or -1) for any.
	SeasonNumber, SceneSeasonNumber *int
	// SceneOrigin says how the scene numbers the series: "tvdb" as TVDB
	// does, "mixed" when its numbering is ambiguous, "unknown" or
	// "unknown:<type>" when it is not known, empty when the mapping says
	// nothing.
	SceneOrigin string
	// SearchMode is how indexers are searched with the title.
	SearchMode SearchMode
	Comment    string
	// FilterRegex, a .NET regular expression, limits the mapping to
	// release titles it matches.
	FilterRegex string
	// Type is the provider the mapping came from: "XemService" for XEM,
	// as Sonarr names its providers.
	Type string
}

// SearchMode is how indexers are searched for a series by a scene
// title, as Sonarr's SearchMode flags.
type SearchMode int

// The search modes.
const (
	// SearchModeDefault searches by the series' IDs, then by title.
	SearchModeDefault SearchMode = iota
	// SearchModeSearchID searches by IDs only.
	SearchModeSearchID
	// SearchModeSearchTitle searches by title only.
	SearchModeSearchTitle
	// SearchModeSearchIDAndTitle searches by IDs and title at once.
	SearchModeSearchIDAndTitle = SearchModeSearchID | SearchModeSearchTitle
)

// Candidate is an episode release mapped to its series and episodes, as
// Sonarr's RemoteEpisode. The embedded Candidate's Target is the
// series' and its Items the episodes'.
type Candidate struct {
	*acquire.Candidate
	Parsed *release.ParsedEpisodeInfo
	// Series is nil when the release matched none.
	Series   *Series
	Episodes []*Episode
	// SceneMapping is the mapping the series was found by, or nil.
	SceneMapping       *SceneMapping
	MappedSeasonNumber int
	// EpisodeRequested is set when the release has an episode a search
	// asked for.
	EpisodeRequested bool
	// DownloadAllowed is set when the release may be grabbed.
	DownloadAllowed bool
	// SeasonSearchMaximumSingleEpisodeAge is the setting of the
	// release's indexer, in days; 0 for none.
	SeasonSearchMaximumSingleEpisodeAge int
}

// NewCandidate returns the candidate of a release of a series, or of no
// series when s is nil, with what was parsed from its title. parsed may
// be nil.
func NewCandidate(r *acquire.Release, parsed *release.ParsedEpisodeInfo, s *Series, episodes []*Episode) *Candidate {
	if parsed == nil {
		parsed = &release.ParsedEpisodeInfo{}
	}
	c := &Candidate{
		Candidate: &acquire.Candidate{
			Release:      r,
			Quality:      acquire.ModelOf(parsed.Quality),
			Languages:    parsed.Languages,
			ReleaseGroup: parsed.ReleaseGroup,
			ReleaseType:  parsed.ReleaseType(),
		},
		Parsed:   parsed,
		Series:   s,
		Episodes: episodes,
	}
	if s != nil {
		c.Target = &s.Target
	}
	for _, ep := range episodes {
		c.Items = append(c.Items, &ep.Item)
	}
	return c
}

// files returns the files of the candidate's episodes, an episode's
// file once for each episode.
func (c *Candidate) files() []*acquire.File {
	var files []*acquire.File
	for _, ep := range c.Episodes {
		if ep.File != nil {
			files = append(files, ep.File)
		}
	}
	return files
}

// distinctFiles returns the files of the candidate's episodes, each
// once.
func (c *Candidate) distinctFiles() []*acquire.File {
	var files []*acquire.File
	for _, f := range c.files() {
		if !slices.ContainsFunc(files, func(g *acquire.File) bool { return g.ID == f.ID }) {
			files = append(files, f)
		}
	}
	return files
}

// episodeIDs returns the IDs of episodes.
func episodeIDs(episodes []*Episode) []core.ID {
	ids := make([]core.ID, len(episodes))
	for i, ep := range episodes {
		ids[i] = ep.ID
	}
	return ids
}
