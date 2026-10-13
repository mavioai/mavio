package subtitles

import (
	"regexp"
	"slices"
	"strings"
)

// The properties a subtitle can match, as subliminal names them.
const (
	MatchHash             = "hash"
	MatchTitle            = "title"
	MatchYear             = "year"
	MatchSeries           = "series"
	MatchSeason           = "season"
	MatchEpisode          = "episode"
	MatchReleaseGroup     = "release_group"
	MatchSource           = "source"
	MatchAudioCodec       = "audio_codec"
	MatchResolution       = "resolution"
	MatchVideoCodec       = "video_codec"
	MatchHearingImpaired  = "hearing_impaired"
	MatchStreamingService = "streaming_service"
	MatchEdition          = "edition"
	MatchIMDbID           = "imdb_id"
	MatchSeriesIMDbID     = "series_imdb_id"
	MatchTVDBID           = "tvdb_id"
	MatchSeriesTVDBID     = "series_tvdb_id"
)

// Matches is a set of matched properties.
type Matches map[string]bool

// NewMatches returns the set of the properties.
func NewMatches(props ...string) Matches {
	m := Matches{}
	for _, p := range props {
		m[p] = true
	}
	return m
}

// Clone returns a copy of m.
func (m Matches) Clone() Matches {
	c := make(Matches, len(m))
	for k, v := range m {
		if v {
			c[k] = true
		}
	}
	return c
}

// Has reports whether m has every property.
func (m Matches) Has(props ...string) bool {
	for _, p := range props {
		if !m[p] {
			return false
		}
	}
	return true
}

// Add adds the properties of o to m.
func (m Matches) Add(o Matches) {
	for k, v := range o {
		if v {
			m[k] = true
		}
	}
}

// Sorted returns the properties in order.
func (m Matches) Sorted() []string {
	var out []string
	for k, v := range m {
		if v {
			out = append(out, k)
		}
	}
	slices.Sort(out)
	return out
}

// GuessMatches returns the properties a guess shares with a video, as
// Bazarr's guess_matches. A partial guess does not count the absence of a
// year as matching a series that needs none.
func GuessMatches(v *Video, g Guess, partial bool) Matches {
	m := Matches{}
	if v.Episode {
		if v.Series != "" && slices.ContainsFunc(g.Titles, func(t string) bool {
			return slices.ContainsFunc(append([]string{v.Series}, v.AlternativeSeries...), func(n string) bool {
				return sanitize(t) == sanitize(n)
			})
		}) {
			m[MatchSeries] = true
		}
		if v.Title != "" && g.EpisodeTitle != "" && sanitize(g.EpisodeTitle) == sanitize(v.Title) {
			m[MatchTitle] = true
		}
		if v.Season != 0 && g.Season == v.Season && g.Season != 0 {
			m[MatchSeason] = true
		}
		// Of several episodes, subtitles are for the lowest.
		if v.EpisodeNumber != 0 && len(g.Episodes) > 0 && slices.Min(g.Episodes) == v.EpisodeNumber {
			m[MatchEpisode] = true
		}
		if v.Year != 0 && g.Year == v.Year {
			m[MatchYear] = true
		}
		if !partial && v.OriginalSeries && g.Year == 0 {
			m[MatchYear] = true
		}
	} else {
		if v.Year != 0 && g.Year == v.Year {
			m[MatchYear] = true
		}
		if v.Title != "" && len(g.Titles) > 0 && slices.ContainsFunc(append([]string{v.Title}, v.AlternativeTitles...), func(n string) bool {
			return sanitize(g.Titles[0]) == sanitize(n)
		}) {
			m[MatchTitle] = true
		}
	}
	if g.ReleaseGroup != "" && v.ReleaseGroup != "" && slices.Contains(equivalentReleaseGroups(sanitizeReleaseGroup(v.ReleaseGroup)), sanitizeReleaseGroup(g.ReleaseGroup)) {
		m[MatchReleaseGroup] = true
	}
	if g.Source != "" {
		if v.Source != "" && g.Source == v.Source {
			m[MatchSource] = true
		}
		// A release group is not trusted when the source differs.
		if m[MatchReleaseGroup] && !m[MatchSource] {
			delete(m, MatchReleaseGroup)
		}
	}
	has := func(video, guess string) bool { return video != "" && guess != "" && video == guess }
	if has(v.VideoCodec, g.VideoCodec) {
		m[MatchVideoCodec] = true
	}
	if v.AudioCodec != "" && slices.Contains(g.AudioCodecs, v.AudioCodec) {
		m[MatchAudioCodec] = true
	}
	if has(v.Resolution, g.Resolution) {
		m[MatchResolution] = true
	}
	// Streaming service and edition match when both have the same one, or
	// neither has one.
	optional := func(video, guess string) bool { return video == guess }
	if optional(v.StreamingService, g.StreamingService) {
		m[MatchStreamingService] = true
	}
	if optional(normalizeEdition(v.Edition), normalizeEdition(g.Edition)) {
		m[MatchEdition] = true
	}
	return m
}

var (
	sanitizeSpaces  = regexp.MustCompile(`[-:().]`)
	sanitizeRemoved = regexp.MustCompile(`'`)
	sanitizeRuns    = regexp.MustCompile(`\s+`)
	bracketed       = regexp.MustCompile(`\[[\w|.]+\]`)
)

// sanitize normalizes a title for comparison, as subliminal's sanitize.
func sanitize(s string) string {
	s = sanitizeSpaces.ReplaceAllString(s, " ")
	s = sanitizeRemoved.ReplaceAllString(s, "")
	s = sanitizeRuns.ReplaceAllString(s, " ")
	return strings.ToLower(strings.TrimSpace(s))
}

// sanitizeReleaseGroup drops bracketed text and upper-cases a release
// group, as subliminal's sanitize_release_group.
func sanitizeReleaseGroup(s string) string {
	return strings.ToUpper(strings.TrimSpace(bracketed.ReplaceAllString(s, "")))
}

// releaseGroupEquivalents are the release groups subliminal counts as one.
var releaseGroupEquivalents = [][]string{
	{"FRAMESTOR", "W4NK3R", "BHDSTUDIO"},
	{"LOL", "DIMENSION"},
	{"ASAP", "IMMERSE", "FLEET"},
	{"AVS", "SVA"},
}

// equivalentReleaseGroups returns the groups equivalent to a sanitized
// release group, itself included.
func equivalentReleaseGroups(group string) []string {
	for _, eq := range releaseGroupEquivalents {
		if slices.Contains(eq, group) {
			return eq
		}
	}
	return []string{group}
}

// normalizeEdition makes editions parsed from different names comparable:
// letters and digits only, lower case.
func normalizeEdition(e string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			return r
		case r >= 'A' && r <= 'Z':
			return r + 'a' - 'A'
		}
		return -1
	}, e)
}
