package rutracker

import (
	"slices"
	"strings"
	"unicode"

	"github.com/dlclark/regexp2"

	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
)

// TitleOptions are the settings that change how ParseTitle normalises a
// title.
type TitleOptions struct {
	// StripCyrillic removes Cyrillic letters.
	StripCyrillic bool
	// MoveFirstTags moves the bracketed tags a title starts with to its
	// end.
	MoveFirstTags bool
	// MoveAllTags moves all bracketed tags to the end; it wins over
	// MoveFirstTags.
	MoveAllTags bool
	// AddRussianToTitle appends "RUS" to the titles of movies and TV.
	AddRussianToTitle bool
}

// cyrillic is the Unicode block .NET's \p{IsCyrillic} stands for, which
// regexp2 lacks.
const cyrillic = `Ѐ-ӿ`

func re(expr string, ignoreCase bool) *regexp2.Regexp {
	var opts regexp2.RegexOptions
	if ignoreCase {
		opts = regexp2.IgnoreCase
	}
	return regexp2.MustCompile(expr, opts)
}

var (
	// findTags match balanced parenthesised and bracketed tags.
	findTags = []*regexp2.Regexp{
		re(`\((?>\((?<c>)|[^()]+|\)(?<-c>))*(?(c)(?!))\)`, false),
		re(`\[(?>\[(?<c>)|[^\[\]]+|\](?<-c>))*(?(c)(?!))\]`, false),
	}

	stripCyrillic = re(`(\([`+cyrillic+`\W]+\))|(^[`+cyrillic+`\W\d]+\/ )|([`+cyrillic+` \-]+,+)|([`+cyrillic+`]+)`, true)

	tvComma     = re(`\s(\d+),(\d+)`, false)
	tvCyrillicX = re(`([\s-])Х+([\s\)\]])`, true)

	tvSeasonEpisodeOf = re(`Сезон\s*[:]*\s+(\d+).+(?:Серии|Эпизод|Выпуски)+\s*[:]*\s+(\d+(?:-\d+)?)\s*из\s*([\w?])`, true)
	tvSeasonEpisode   = re(`Сезон\s*[:]*\s+(\d+).+(?:Серии|Эпизод|Выпуски)+\s*[:]*\s+(\d+(?:-\d+)?)`, true)
	tvSeason          = re(`Сезон\s*[:]*\s+(\d+(?:-\d+)?)`, true)
	tvEpisodeOf       = re(`(?:Серии|Эпизод|Выпуски)+\s*[:]*\s+(\d+(?:-\d+)?)\s*из\s*([\w?])`, true)
	tvEpisode         = re(`(?:Серии|Эпизод|Выпуски)+\s*[:]*\s+(\d+(?:-\d+)?)`, true)
	tvSeasonAnime     = re(`ТВ[-]*(?:(\d+))`, true)
	tvEpisodeAnimeOf  = re(`\[(\d+(\+\d+)?)\s+из\s+(\d+(\+\d+)?)\]`, true)

	dashes    = re(`\p{Pd}`, true)
	double4K  = re(`\b(2160p), 4K\b`, true)
	bluRay    = re(`\bBlu-ray Disc\b`, true)
	director  = re(`(\([`+cyrillic+`\W]+)(?:\s/\s(.+?))?\)`, true)
	voiceOver = re(`((?:\dx\s)?(?:[A-Z])?VO\s\(.+?\))`, false)
	r5        = re(`(.*)(.R5.)(.*)`, false)
	subs      = re(`(\bSub\b[^+]*\b|\b[\+]*Sub[\+]*\b)`, false)

	cleanups = []struct {
		re   *regexp2.Regexp
		with string
	}{
		{re(`\b-Rip\b`, true), "Rip"},
		{re(`\bHDTVRip\b`, true), "HDTV"},
		{re(`\bWEB-DLRip\b`, true), "WEB-DL"},
		{re(`\bWEBDLRip\b`, true), "WEB-DL"},
		{re(`\bWEBDL\b`, true), "WEB-DL"},
		{re(`\bКураж-Бамбей\b`, true), "kurazh"},
		{re(`\(\s*\/\s*`, false), "("},
		{re(`\s*\/\s*\)`, false), ")"},
		{re(`[\[\(]\s*[\)\]]`, false), ""},
		{re(`\s+\+(?:\s+\+)+\s+`, false), " + "},
	}

	spaces = re(`\s+`, false)
)

// replace replaces all of the expression's matches, panicking on regexp2
// timeouts, which are not set.
func replace(r *regexp2.Regexp, s, with string) string {
	out, err := r.Replace(s, with, -1, -1)
	if err != nil {
		panic(err)
	}
	return out
}

// ParseTitle normalises a RuTracker title for the categories of its forum
// so that Sonarr and Radarr can parse it, as Prowlarr's
// RuTrackerTitleParser does: Russian seasons and episodes become S01E02,
// directors, voice-overs and subtitle languages are removed, and, by the
// options, Cyrillic letters are stripped and tags moved.
func ParseTitle(title string, categories []int, o TitleOptions) string {
	tv, movie := isAny(categories, indexer.TV), isAny(categories, indexer.Movies)

	title = replace(dashes, title, "-")
	// 2160p is often followed by 4K, saying it twice.
	title = replace(double4K, title, "$1")

	if tv {
		title = replace(tvComma, title, " $1-$2")
		title = replace(tvCyrillicX, title, "$1XX$2")
		title = replace(tvSeasonEpisodeOf, title, "S$1E$2 of $3")
		title = replace(tvSeasonEpisode, title, "S$1E$2")
		title = replace(tvSeason, title, "S$1")
		title = replace(tvEpisodeOf, title, "E$1 of $2")
		title = replace(tvEpisode, title, "E$1")
		title = replace(tvSeasonAnime, title, "S$1")
		title = replace(tvEpisodeAnimeOf, title, "E$1 of $3")
	} else if movie {
		// Radarr takes Blu-ray Disc for Bluray-1080p rather than BR-DISK.
		title = replace(bluRay, title, "BR-DISK")
	}

	if tv || movie {
		// Titles read "Russian name / English name (Russian director /
		// English director) other stuff": the directors go.
		title = replace(director, title, "")
		title = replace(voiceOver, title, "")
		title = replace(r5, title, "$1")
		title = replace(subs, title, "")
		// All of RuTracker's releases have a Russian track. Prowlarr's check
		// for RUS already being there never matches, its pattern having
		// backspaces for \b, so RUS is always added.
		if o.AddRussianToTitle {
			title += " RUS"
		}
	}

	if o.StripCyrillic {
		title = strings.Trim(replace(stripCyrillic, title, ""), " -")
	}

	switch {
	case o.MoveAllTags:
		title = moveAllTags(title)
	case o.MoveFirstTags:
		title = moveFirstTags(title)
	}

	for _, c := range cleanups {
		title = replace(c.re, title, c.with)
	}
	title = strings.Trim(title, ` &,.!?+-_|/\:`)
	title = replace(spaces, title, " ")
	return strings.TrimFunc(title, unicode.IsSpace)
}

// isAny reports whether the categories have the parent category or one of
// its subcategories.
func isAny(categories []int, parent *indexer.Category) bool {
	return slices.ContainsFunc(categories, func(id int) bool {
		return id == parent.ID || slices.ContainsFunc(parent.Sub, func(c *indexer.Category) bool { return c.ID == id })
	})
}

// matches returns the texts of the expression's matches in s.
func matches(r *regexp2.Regexp, s string) []matchAt {
	var out []matchAt
	m, err := r.FindStringMatch(s)
	for err == nil && m != nil {
		out = append(out, matchAt{text: m.String(), index: m.Index})
		m, err = r.FindNextMatch(m)
	}
	if err != nil {
		panic(err)
	}
	return out
}

// matchAt is a match and its index in runes.
type matchAt struct {
	text  string
	index int
}

// moveAllTags moves every tag of the input to the end.
func moveAllTags(input string) string {
	output := input
	for _, r := range findTags {
		for _, m := range matches(r, input) {
			output = strings.TrimSpace(strings.ReplaceAll(output, m.text, "") + " " + m.text)
		}
	}
	return strings.TrimSpace(output)
}

// moveFirstTags moves the tags the input starts with, separated by
// whitespace only, to the end. Like Prowlarr, it reads the indexes of the
// matches as they were before the tags of the same kind were moved.
func moveFirstTags(input string) string {
	output := input
	for _, r := range findTags {
		expected := 0
		for _, m := range matches(r, output) {
			if m.index > expected {
				runes := []rune(output)
				// Prowlarr fails on an index past the end; stop moving tags.
				if m.index > len(runes) {
					break
				}
				if strings.TrimSpace(string(runes[expected:m.index])) != "" {
					break
				}
				expected = m.index
			}
			output = strings.TrimSpace(strings.Replace(output, m.text, "", 1) + " " + m.text)
			expected += len([]rune(m.text))
		}
	}
	return strings.TrimSpace(output)
}
