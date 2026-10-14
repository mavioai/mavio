package subtitles

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/language"
	"golang.org/x/text/language/display"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/subtitle"
)

// placeholder is a placeholder of a prompt, as Lingarr's.
var placeholder = regexp.MustCompile(`\{(\w+)\}`)

// fillPrompt replaces a prompt's placeholders with their values, keeping
// those without one.
func fillPrompt(prompt string, values map[string]string) string {
	return placeholder.ReplaceAllStringFunc(prompt, func(m string) string {
		if v, ok := values[m[1:len(m)-1]]; ok {
			return v
		}
		return m
	})
}

// languageName is how prompts name a language: its English name, or its
// code.
func languageName(tag string, codes bool) string {
	if codes {
		return tag
	}
	t, err := language.Parse(tag)
	if err != nil {
		return tag
	}
	if name := display.English.Tags().Name(t); name != "" {
		return name
	}
	return tag
}

// matchTier is how closely a translator's language matches the one asked
// for, closest first, as Lingarr's.
type matchTier int

const (
	matchExact matchTier = iota
	// matchAlias is the same language under another code, such as "iw"
	// for "he".
	matchAlias
	// matchScript is the same language in the script one of them names,
	// such as "zh-Hant" for "zh-TW".
	matchScript
	// matchNeutral is the same language, such as "pt" for "pt-BR".
	matchNeutral
	matchNone
)

// matchLanguage returns the language of a translator's languages closest
// to want, and how close it is; a translator without languages takes any.
func matchLanguage(want string, languages []string) (string, matchTier) {
	if len(languages) == 0 {
		return want, matchExact
	}
	wt, err := language.Parse(want)
	if err != nil {
		for _, l := range languages {
			if strings.EqualFold(l, want) {
				return l, matchExact
			}
		}
		return "", matchNone
	}
	wc := canonical(wt)
	best, tier := "", matchNone
	for _, l := range languages {
		lt, err := language.Parse(l)
		if err != nil {
			continue
		}
		var t matchTier
		switch {
		case strings.EqualFold(l, want):
			return l, matchExact
		case canonical(lt) == wc:
			t = matchAlias
		case base(lt) == base(wt) && script(lt) == script(wt) && (explicitScript(lt) || explicitScript(wt)):
			t = matchScript
		case base(lt) == base(wt):
			t = matchNeutral
		default:
			continue
		}
		if t < tier {
			best, tier = l, t
		}
	}
	return best, tier
}

func canonical(t language.Tag) language.Tag {
	c, err := language.All.Canonicalize(t)
	if err != nil {
		return t
	}
	return c
}

func base(t language.Tag) language.Base {
	b, _ := t.Base()
	return b
}

// explicitScript reports whether a tag names its script, as "zh-Hant"
// does and "zh-TW" does not.
func explicitScript(t language.Tag) bool {
	_, s, _ := t.Raw()
	return s != language.Script{}
}

func script(t language.Tag) language.Script {
	s, _ := t.Script()
	return s
}

// maxLineLength is the length translations are split at into lines, as
// Lingarr's.
const maxLineLength = 42

// subtitleLines splits a cue's translation into lines, as Lingarr's
// ToSubtitleLines: into the cue's lines when line breaks are kept and the
// translation has as many, else at maxLineLength when the cue had lines
// and formatting is stripped.
func subtitleLines(translated string, lines int, preserve, strip bool) []string {
	if preserve && lines > 1 {
		split := strings.Split(translated, "\n")
		for i, l := range split {
			split[i] = strings.TrimRight(l, "\r")
		}
		if len(split) == lines {
			return split
		}
		return []string{strings.Join(split, " ")}
	}
	if lines > 1 && strip {
		return splitIntoLines(translated, maxLineLength)
	}
	return []string{translated}
}

// splitIntoLines wraps text at spaces into lines of at most max
// characters, a longer word making a line of its own.
func splitIntoLines(text string, max int) []string {
	var lines []string
	var line strings.Builder
	n := 0
	for word := range strings.SplitSeq(text, " ") {
		w := utf8.RuneCountInString(word)
		switch {
		case line.Len() == 0:
			line.WriteString(word)
			n = w
		case n+1+w <= max:
			line.WriteString(" " + word)
			n += 1 + w
		default:
			lines = append(lines, line.String())
			line.Reset()
			line.WriteString(word)
			n = w
		}
	}
	return append(lines, line.String())
}

// overlapGap and minOverlapDuration are the gap left between a trimmed cue
// and the next, and the shortest a trimmed cue may last, as Lingarr's.
const (
	overlapGap         = 20 * time.Millisecond
	minOverlapDuration = 500 * time.Millisecond
)

// fixOverlaps trims the end of each cue overlapping the next, unless that
// would leave it too short, as Lingarr's FixOverlappingSubtitles.
func fixOverlaps(cues []subtitle.Cue) {
	for i := 0; i+1 < len(cues); i++ {
		c, next := &cues[i], cues[i+1]
		if c.End+overlapGap > next.Start {
			if end := next.Start - overlapGap; end-c.Start >= minOverlapDuration {
				c.End = end
			}
		}
	}
}

// validateSubtitle refuses a subtitle that looks broken, as Lingarr's
// subtitle validation.
func validateSubtitle(size int, cues []subtitle.Cue, v *core.SubtitleTranslationValidation) error {
	if int64(size) > v.MaxFileSize {
		return fmt.Errorf("%w: the subtitle has %d bytes, more than %d", core.ErrInvalid, size, v.MaxFileSize)
	}
	if len(cues) < 2 {
		return fmt.Errorf("%w: the subtitle has fewer than 2 cues", core.ErrInvalid)
	}
	for i, c := range cues {
		text := subtitle.PlainText(c.Text)
		n := utf8.RuneCountInString(strings.TrimSpace(text))
		switch {
		case n == 0:
			return fmt.Errorf("%w: cue %d is empty", core.ErrInvalid, i+1)
		case n > v.MaxCueLength || n < v.MinCueLength:
			return fmt.Errorf("%w: cue %d has %d characters, outside %d–%d", core.ErrInvalid, i+1, n, v.MinCueLength, v.MaxCueLength)
		case c.Start >= c.End:
			return fmt.Errorf("%w: cue %d ends before it starts", core.ErrInvalid, i+1)
		case c.End-c.Start < v.MinDuration || c.End-c.Start > v.MaxDuration:
			return fmt.Errorf("%w: cue %d lasts %s, outside %s–%s", core.ErrInvalid, i+1, c.End-c.Start, v.MinDuration, v.MaxDuration)
		case i > 0 && c.Start < cues[i-1].End:
			return fmt.Errorf("%w: cue %d overlaps the one before", core.ErrInvalid, i+1)
		case strings.ContainsFunc(text, func(r rune) bool { return unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' }):
			return fmt.Errorf("%w: cue %d has control characters", core.ErrInvalid, i+1)
		}
	}
	return nil
}
