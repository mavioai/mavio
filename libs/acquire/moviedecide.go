package acquire

import (
	"context"
	"slices"
	"strings"

	"github.com/mavioai/mavio/libs/release"
)

// FindMovie finds the movie a release is for: the movie and its item, or
// nil when it matches none.
type FindMovie func(ctx context.Context, parsed *release.ParsedMovieInfo, r *Release, s *Search) (*Target, *Item, error)

// DecideMovies makes decisions on movie releases, as Radarr's
// DownloadDecisionMaker: it parses each release, finds its movie, works
// out its languages and custom formats, and evaluates the candidate.
// Releases whose titles do not parse are left out, unless they were
// searched for. s is nil for RSS syncs and pushed releases.
func (e *Engine) DecideMovies(ctx context.Context, releases []*Release, s *Search, pushed bool, find FindMovie) []Decision {
	var decisions []Decision
	for _, r := range releases {
		if d, ok := e.decideMovie(ctx, r, s, pushed, find); ok {
			decisions = append(decisions, d)
		}
	}
	return decisions
}

func (e *Engine) decideMovie(ctx context.Context, r *Release, s *Search, pushed bool, find FindMovie) (Decision, bool) {
	source := sourceOf(s, pushed)
	parsed := release.ParseMovieTitle(r.Title, false)
	if parsed == nil || parsed.PrimaryMovieTitle() == "" {
		if s == nil {
			return Decision{}, false
		}
		c := &Candidate{Release: r, Source: source}
		if parsed != nil {
			c.Languages = parsed.Languages
			c.Quality = ModelOf(parsed.Quality)
		} else {
			c.Languages = release.ParseLanguages(r.Title, release.Movie)
			c.Quality = ModelOf(release.ParseQuality(r.Title, release.Movie))
		}
		return Decision{Candidate: c, Rejections: []Rejection{{Reason: ReasonUnableToParse, Message: "Unable to parse release"}}}, true
	}
	c := &Candidate{
		Release:       r,
		Source:        source,
		Quality:       ModelOf(parsed.Quality),
		Languages:     parsed.Languages,
		ReleaseGroup:  parsed.ReleaseGroup,
		Edition:       parsed.Edition,
		HardcodedSubs: parsed.HardcodedSubs,
		Year:          parsed.Year,
	}
	target, item, err := find(ctx, parsed, r, s)
	if err != nil {
		e.logger().ErrorContext(ctx, "couldn't process release", "release", r.Title, "error", err)
		return Decision{Candidate: c, Rejections: []Rejection{{Reason: ReasonError, Message: "Unexpected error processing release"}}}, true
	}
	if target == nil {
		msg := "Unknown Movie. Unable to match to correct movie using release title."
		if pushed {
			msg = "Unknown Movie. Unable to match to existing movie in Library using release title."
		}
		return Decision{Candidate: c, Rejections: []Rejection{{Reason: ReasonUnknownTarget, Message: msg}}}, true
	}
	c.Target = target
	if item != nil {
		c.Items = []*Item{item}
	}
	var multi []release.Language
	ix, err := IndexerOf(ctx, e.Env.Ports, r)
	if err != nil {
		e.logger().ErrorContext(ctx, "couldn't process release", "release", r.Title, "error", err)
		return Decision{Candidate: c, Rejections: []Rejection{{Reason: ReasonError, Message: "Unexpected error processing release"}}}, true
	}
	if ix != nil {
		multi = ix.MultiLanguages
	}
	c.Languages = movieLanguages(parsed, r, target, multi)
	c.Formats = MatchFormats(e.Env.Formats, c.formatInput(parsed.SimpleReleaseTitle))
	if target.Profile != nil {
		c.FormatScore = target.Profile.FormatScore(c.Formats)
	}
	return e.Evaluate(ctx, c, s), true
}

// movieLanguages works out the languages of a movie release, as Radarr's
// AggregateLanguages: those the indexer gave, or those parsed less any
// that only came from the movie's title; the indexer's languages for
// MULTi releases; the movie's original language when none are known.
func movieLanguages(parsed *release.ParsedMovieInfo, r *Release, t *Target, multi []release.Language) []release.Language {
	langs := slices.Clone(parsed.Languages)
	if len(r.Languages) > 0 {
		langs = slices.Clone(r.Languages)
	} else {
		tokens := parsed.SimpleReleaseTitle
		if tokens == "" {
			tokens = parsed.ReleaseTitle
		}
		normalizedTokens := release.NormalizeMovieTitle(tokens)
		var remove []release.Language
		titleLangs := release.ParseLanguages(t.Title, release.Movie)
		if !slices.Contains(titleLangs, release.LanguageUnknown) {
			normalizedTitle := release.NormalizeMovieTitle(t.Title)
			if i := strings.Index(strings.ToLower(normalizedTokens), strings.ToLower(normalizedTitle)); i >= 0 && i+len(normalizedTitle) <= len(tokens) {
				tokens = tokens[:i] + tokens[i+len(normalizedTitle):]
				remove = append(remove, titleLangs...)
			}
		}
		remove = except(remove, release.ParseLanguages(tokens, release.Movie))
		langs = except(langs, remove)
	}
	if len(multi) > 0 && release.HasMultipleLanguages(r.Title) {
		if len(langs) == 0 || (len(langs) == 1 && langs[0] == release.LanguageUnknown) {
			langs = slices.Clone(multi)
		} else {
			langs = append(langs, except(multi, langs)...)
		}
	}
	if len(langs) == 0 || (len(langs) == 1 && langs[0] == release.LanguageUnknown) {
		langs = []release.Language{t.OriginalLanguage}
	}
	if i := slices.Index(langs, release.Original); i >= 0 {
		langs = slices.Delete(langs, i, i+1)
		if !slices.Contains(langs, t.OriginalLanguage) {
			langs = append(langs, t.OriginalLanguage)
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
