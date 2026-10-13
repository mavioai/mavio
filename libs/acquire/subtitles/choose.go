package subtitles

import (
	"cmp"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/mavioai/mavio/libs/core"
)

// HIMode is how a search treats hearing-impaired subtitles.
type HIMode int

const (
	// HIAllowed takes subtitles of either kind, preferring neither.
	HIAllowed HIMode = iota
	// HIForce takes only hearing-impaired subtitles.
	HIForce
	// HIForceNon takes only subtitles that are not hearing-impaired.
	HIForceNon
)

// HIModeFor returns the mode a profile asks for in searching for l: that
// of the item with l's language and forced flag.
func HIModeFor(p *core.SubtitleProfile, l core.SubtitleLanguage) HIMode {
	for _, it := range p.Items {
		if it.Language == l.Language && it.Forced == l.Forced {
			switch it.HI {
			case core.SubtitleHIRequired:
				return HIForce
			case core.SubtitleHIExcluded:
				return HIForceNon
			}
			return HIAllowed
		}
	}
	return HIAllowed
}

// Candidate is a subtitle a provider listed.
type Candidate struct {
	Provider string
	ID       string
	Language core.SubtitleLanguage
	// ReleaseInfo is the release names of the subtitle, one per line.
	ReleaseInfo string
	// HashVerifiable is whether a hash match can be checked against the
	// other properties; HearingImpairedVerifiable whether the provider
	// knows if the subtitle is hearing-impaired.
	HashVerifiable, HearingImpairedVerifiable bool
	// Matches are the properties the subtitle matches.
	Matches Matches
}

// CandidateMatches returns the properties a provider knows a subtitle
// matches together with those its release names share with the video.
func CandidateMatches(v *Video, known Matches, releaseNames []string, partial bool) Matches {
	m := known.Clone()
	for _, name := range releaseNames {
		if strings.TrimSpace(name) == "" {
			continue
		}
		m.Add(GuessMatches(v, ParseGuess(name, v.Episode), partial))
	}
	return m
}

// BanList holds the patterns a profile requires and forbids in release
// names.
type BanList struct {
	mustContain, mustNotContain []*regexp.Regexp
}

// NewBanList compiles a profile's patterns, matched case-insensitively.
func NewBanList(p *core.SubtitleProfile) (*BanList, error) {
	compile := func(ps []string) ([]*regexp.Regexp, error) {
		var out []*regexp.Regexp
		for _, s := range ps {
			re, err := regexp.Compile("(?i)" + s)
			if err != nil {
				return nil, fmt.Errorf("subtitle profile %q pattern %q: %w", p.Name, s, err)
			}
			out = append(out, re)
		}
		return out, nil
	}
	var b BanList
	var err error
	if b.mustContain, err = compile(p.MustContain); err != nil {
		return nil, err
	}
	if b.mustNotContain, err = compile(p.MustNotContain); err != nil {
		return nil, err
	}
	return &b, nil
}

// Allows reports whether a subtitle's release names contain none of the
// forbidden patterns and all of the required ones. A subtitle without
// release names is allowed.
func (b *BanList) Allows(releaseInfo string) bool {
	if b == nil || releaseInfo == "" {
		return true
	}
	for _, re := range b.mustNotContain {
		if re.MatchString(releaseInfo) {
			return false
		}
	}
	for _, re := range b.mustContain {
		if !re.MatchString(releaseInfo) {
			return false
		}
	}
	return true
}

// Filter prepares a provider's subtitles as Bazarr's pool does: it maps
// their languages through equals, and drops blocklisted subtitles, those
// the ban list rejects and repeated IDs.
func Filter(cands []Candidate, equals LanguageEquals, ban *BanList, blocklist []core.SubtitleBlocklistEntry) []Candidate {
	var out []Candidate
	seen := map[[2]string]bool{}
	for _, c := range cands {
		c.Language = equals.Apply(c.Language)
		key := [2]string{c.Provider, c.ID}
		if seen[key] || !ban.Allows(c.ReleaseInfo) || slices.ContainsFunc(blocklist, func(b core.SubtitleBlocklistEntry) bool {
			return b.Provider == c.Provider && b.SubtitleID == c.ID
		}) {
			continue
		}
		seen[key] = true
		out = append(out, c)
	}
	return out
}

// Ranked is a scored candidate.
type Ranked struct {
	Candidate
	Score, ScoreWithoutHash int
	// Final are the matches the score counts.
	Final Matches
}

// Choose returns the candidates in langs worth downloading for a video,
// best first, as Bazarr's download_best_subtitles: by score, then score
// without the hash, down to minScore, without those of the wrong
// hearing-impaired kind when the mode forces one, and, for an episode,
// without those not matching its series, season and episode unless an
// unverifiable hash vouches for them. The caller tries them in order until
// one downloads.
func Choose(cands []Candidate, v *Video, langs []core.SubtitleLanguage, mode HIMode, minScore int) []Ranked {
	useHI := mode == HIForce
	var ranked []Ranked
	for _, c := range cands {
		if !slices.ContainsFunc(langs, func(l core.SubtitleLanguage) bool { return l.Language == c.Language.Language }) {
			continue
		}
		score, without, final := Score(c.Matches, ScoreSubject{HashVerifiable: c.HashVerifiable, HearingImpaired: c.Language.HearingImpaired}, v, &useHI)
		ranked = append(ranked, Ranked{Candidate: c, Score: score, ScoreWithoutHash: without, Final: final})
	}
	slices.SortStableFunc(ranked, func(a, b Ranked) int {
		return cmp.Or(cmp.Compare(b.Score, a.Score), cmp.Compare(b.ScoreWithoutHash, a.ScoreWithoutHash))
	})
	var out []Ranked
	for _, r := range ranked {
		if r.Score < minScore {
			break
		}
		if r.HearingImpairedVerifiable && !r.Final[MatchHearingImpaired] && mode != HIAllowed {
			continue
		}
		if v.Episode {
			verifiable := r.HashVerifiable || !r.Final[MatchHash]
			series := r.Matches.Has(MatchSeason, MatchEpisode) && (r.Matches[MatchSeries] || r.Matches[MatchIMDbID])
			if verifiable && !series {
				continue
			}
		}
		out = append(out, r)
	}
	return out
}
