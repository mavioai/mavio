package series

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/mavioai/mavio/libs/acquire"
	"github.com/mavioai/mavio/libs/release"
)

// Match is what a release was mapped to, as the parts of Sonarr's
// RemoteEpisode that ParsingService.Map fills.
type Match struct {
	// Series is nil when the release matched none.
	Series   *Series
	Episodes []*Episode
	// SceneMapping is the mapping the series was found by, or nil.
	SceneMapping       *SceneMapping
	MappedSeasonNumber int
	EpisodeRequested   bool
	// AliasTvdbID is, when Series is nil, the TVDB ID of a series the
	// release's series title is an alias of; 0 for none.
	AliasTvdbID int
}

// FindEpisodes maps a parsed release to its series and episodes, as
// Sonarr's ParsingService.Map. s is nil for RSS syncs and pushed
// releases.
type FindEpisodes func(ctx context.Context, parsed *release.ParsedEpisodeInfo, r *acquire.Release, s *Search) (*Match, error)

// ParseSpecial parses a release that did not parse, or may be a special,
// as a special of a known series, as Sonarr's
// ParsingService.ParseSpecialEpisodeTitle. parsed is nil when the title
// did not parse. It returns nil when the release is no special.
type ParseSpecial func(ctx context.Context, parsed *release.ParsedEpisodeInfo, r *acquire.Release, s *Search) (*release.ParsedEpisodeInfo, error)

// DecideEpisodes makes decisions on episode releases, as Sonarr's
// DownloadDecisionMaker: it parses each release, maps it to its series
// and episodes, works out its languages and custom formats, and
// evaluates the candidate. Releases whose titles do not parse are left
// out, unless they were searched for. s is nil for RSS syncs and pushed
// releases; special may be nil.
func (e *Engine) DecideEpisodes(ctx context.Context, releases []*acquire.Release, s *Search, pushed bool, find FindEpisodes, special ParseSpecial) []Decision {
	var decisions []Decision
	for _, r := range releases {
		d, ok := e.decideEpisode(ctx, r, s, find, special)
		if !ok {
			continue
		}
		d.Candidate.Source = sourceOf(s, pushed)
		decisions = append(decisions, d)
	}
	return decisions
}

// sourceOf returns the source of a release found by s, or pushed.
func sourceOf(s *Search, pushed bool) acquire.Source {
	switch {
	case s == nil && pushed:
		return acquire.SourceReleasePush
	case s == nil:
		return acquire.SourceRSS
	case s.Interactive:
		return acquire.SourceInteractiveSearch
	case s.UserInvoked:
		return acquire.SourceUserInvokedSearch
	default:
		return acquire.SourceSearch
	}
}

// failed returns the decision on a release that could not be processed.
func failed(ctx context.Context, r *acquire.Release, err error) Decision {
	slog.ErrorContext(ctx, "couldn't process release", "release", r.Title, "error", err)
	return Decision{
		Candidate:  NewCandidate(r, nil, nil, nil),
		Rejections: []acquire.Rejection{{Reason: acquire.ReasonError, Message: "Unexpected error processing release"}},
	}
}

func (e *Engine) decideEpisode(ctx context.Context, r *acquire.Release, s *Search, find FindEpisodes, special ParseSpecial) (Decision, bool) {
	parsed := release.ParseSeriesTitle(r.Title)
	if (parsed == nil || parsed.IsPossibleSpecialEpisode()) && special != nil {
		sp, err := special(ctx, parsed, r, s)
		if err != nil {
			return failed(ctx, r, err), true
		}
		if sp != nil {
			parsed = sp
		}
	}
	var d Decision
	ok := false
	if parsed != nil && strings.TrimSpace(parsed.SeriesTitle) != "" {
		m, err := find(ctx, parsed, r, s)
		if err != nil {
			return failed(ctx, r, err), true
		}
		if m == nil {
			m = &Match{}
		}
		c := NewCandidate(r, parsed, m.Series, m.Episodes)
		c.SceneMapping = m.SceneMapping
		c.MappedSeasonNumber = m.MappedSeasonNumber
		c.EpisodeRequested = m.EpisodeRequested
		ok = true
		switch {
		case m.Series == nil && m.AliasTvdbID != 0:
			d = Decision{Candidate: c, Rejections: []acquire.Rejection{{
				Reason:  ReasonMatchesAnotherSeries,
				Message: fmt.Sprintf("%s matches an alias for series with TVDB ID: %d", parsed.SeriesTitle, m.AliasTvdbID),
			}}}
		case m.Series == nil:
			d = Decision{Candidate: c, Rejections: []acquire.Rejection{{Reason: acquire.ReasonUnknownTarget, Message: "Unknown Series"}}}
		case len(m.Episodes) == 0:
			d = Decision{Candidate: c, Rejections: []acquire.Rejection{{Reason: ReasonUnknownEpisode, Message: "Unable to identify correct episode(s) using release name and scene mappings"}}}
		default:
			if err := e.augment(ctx, c); err != nil {
				return failed(ctx, r, err), true
			}
			d = e.Evaluate(ctx, c, s)
		}
	}
	if s != nil {
		if parsed == nil {
			parsed = &release.ParsedEpisodeInfo{
				Languages: release.ParseLanguages(r.Title, release.Series),
				Quality:   release.ParseQuality(r.Title, release.Series),
			}
		}
		if strings.TrimSpace(parsed.SeriesTitle) == "" {
			c := NewCandidate(r, parsed, nil, nil)
			return Decision{Candidate: c, Rejections: []acquire.Rejection{{Reason: acquire.ReasonUnableToParse, Message: "Unable to parse release"}}}, true
		}
	}
	return d, ok
}

// augment works out the languages, custom formats and indexer settings
// of a candidate mapped to its series and episodes.
func (e *Engine) augment(ctx context.Context, c *Candidate) error {
	r := c.Release
	ix, err := acquire.IndexerOf(ctx, e.Env.Ports, r)
	if err != nil {
		return err
	}
	var multi []release.Language
	if ix != nil {
		multi = ix.MultiLanguages
		c.SeasonSearchMaximumSingleEpisodeAge = ix.SeasonSearchMaximumSingleEpisodeAge
	}
	c.Languages = episodeLanguages(c.Parsed, r, c.Series, c.Episodes, multi)
	c.Formats = acquire.MatchFormats(e.Env.Formats, formatInput(c))
	if c.Series.Profile != nil {
		c.FormatScore = c.Series.Profile.FormatScore(c.Formats)
	}
	c.DownloadAllowed = len(c.Episodes) > 0
	return nil
}

// formatInput returns the input custom formats are matched against, as
// Sonarr's CustomFormatInput of a RemoteEpisode.
func formatInput(c *Candidate) *acquire.Input {
	in := &acquire.Input{
		Title:        c.Parsed.ReleaseTitle,
		Quality:      c.Parsed.Quality,
		Languages:    c.Languages,
		ReleaseGroup: c.Parsed.ReleaseGroup,
		ReleaseType:  c.ReleaseType,
	}
	if c.Release != nil {
		in.Size = c.Release.Size
		in.Flags = c.Release.Flags
	}
	if c.Series != nil {
		in.OriginalLanguage = c.Series.OriginalLanguage
	}
	return in
}

// episodeLanguages works out the languages of an episode release, as
// Sonarr's AggregateLanguages: those the indexer gave, or those parsed
// less any that only came from an episode's title; the indexer's
// languages for MULTi releases; the series' original language when none
// are known.
func episodeLanguages(parsed *release.ParsedEpisodeInfo, r *acquire.Release, s *Series, episodes []*Episode, multi []release.Language) []release.Language {
	langs := slices.Clone(parsed.Languages)
	if s == nil {
		if r != nil && len(r.Languages) > 0 {
			return r.Languages
		}
		return langs
	}
	if r != nil && len(r.Languages) > 0 {
		langs = slices.Clone(r.Languages)
	} else {
		tokens := parsed.ReleaseTokens
		if tokens == "" {
			tokens = parsed.ReleaseTitle
		}
		normalizedTokens := strings.ToLower(release.NormalizeEpisodeTitle(tokens))
		var remove []release.Language
		for _, ep := range episodes {
			titleLangs := release.ParseLanguages(ep.Title, release.Series)
			if slices.Contains(titleLangs, release.LanguageUnknown) {
				continue
			}
			normalizedTitle := release.NormalizeEpisodeTitle(ep.Title)
			if i := strings.Index(normalizedTokens, strings.ToLower(normalizedTitle)); i >= 0 && i+len(normalizedTitle) <= len(tokens) {
				tokens = tokens[:i] + tokens[i+len(normalizedTitle):]
				remove = append(remove, titleLangs...)
			}
		}
		remove = except(remove, release.ParseLanguages(tokens, release.Series))
		langs = except(langs, remove)
	}
	if r != nil && len(multi) > 0 && release.HasMultipleLanguages(r.Title) {
		if len(langs) == 0 || (len(langs) == 1 && langs[0] == release.LanguageUnknown) {
			langs = slices.Clone(multi)
		} else {
			langs = append(langs, except(multi, langs)...)
		}
	}
	if len(langs) == 0 || (len(langs) == 1 && langs[0] == release.LanguageUnknown) {
		langs = []release.Language{s.OriginalLanguage}
	}
	if i := slices.Index(langs, release.Original); i >= 0 {
		langs = slices.Delete(langs, i, i+1)
		if !slices.Contains(langs, s.OriginalLanguage) {
			langs = append(langs, s.OriginalLanguage)
		} else {
			langs = append(langs, release.LanguageUnknown)
		}
	}
	return langs
}

// except returns the distinct elements of a not in b, as LINQ's Except.
func except[T comparable](a, b []T) []T {
	var out []T
	for _, v := range a {
		if !slices.Contains(b, v) && !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	return out
}
