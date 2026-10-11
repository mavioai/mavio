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
}

// SceneMapping is the scene mapping a release's series title was found
// by, as Sonarr's SceneMapping.
type SceneMapping struct {
	Title  string
	TvdbID int
	// SceneOrigin says how the scene numbers the series: "mixed" when
	// its numbering is ambiguous, "unknown" or "unknown:<type>" when it
	// is not known, empty when the mapping says nothing.
	SceneOrigin string
	Comment     string
}

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
