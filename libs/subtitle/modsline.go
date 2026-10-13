package subtitle

import (
	"errors"
	"slices"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/dlclark/regexp2"
)

// The line mods of Subzero (mods/common.py, mods/hearing_impaired.py,
// mods/ocr_fixes.py) and the loop of main.py that applies them. Their
// regular expressions are Subzero's, in the .NET syntax of regexp2: flags
// Python needs and .NET does not ((?u), (?x) on patterns without spaces)
// are dropped and literal braces escaped.

// errEmptyEntry is Subzero's EmptyEntryError: a processor found the whole
// cue to be removed.
var errEmptyEntry = errors.New("empty entry")

// lineContext is what a processor sees besides the line.
type lineContext struct {
	// entry is the cue's text, stripped, with \N line breaks.
	entry string
	mod   *modContext
}

// processor is one step of a line mod.
type processor struct {
	name  string
	apply func(content string, ctx *lineContext) (string, error)
	// supported, when set, limits the processor to some subtitles.
	supported func(ctx *modContext) bool
}

// reProcessor is Subzero's NReProcessor: a substitution on the line.
func reProcessor(name, pattern string, repl func(m regexp2.Match) string) processor {
	re := pyRegex(pattern)
	return processor{name: name, apply: func(content string, _ *lineContext) (string, error) {
		return pySub(re, content, repl), nil
	}}
}

// literal returns a replacement function for a replacement without group
// references.
func literal(s string) func(regexp2.Match) string { return func(regexp2.Match) string { return s } }

// template returns a replacement function for Python's "\1"-style
// templates: parts are literal strings and group numbers.
func template(parts ...any) func(regexp2.Match) string {
	return func(m regexp2.Match) string {
		var b strings.Builder
		for _, p := range parts {
			switch v := p.(type) {
			case string:
				b.WriteString(v)
			case int:
				b.WriteString(group(m, v))
			}
		}
		return b.String()
	}
}

// subzeroTag is Subzero's TAG: optional {\i1}-style tags.
const subzeroTag = `(?:\s*\{\\[iusb][0-1]\}\s*)*`

func withTags(pattern string) string { return strings.ReplaceAll(pattern, "%(t)s", subzeroTag) }

// emptyTagProcessor is Subzero's EMPTY_TAG_PROCESSOR.
var emptyTagProcessor = sync.OnceValue(func() processor {
	return reProcessor("empty_tag", `(\{\\\w1\})[\s.,-_!?]*(\{\\\w0\})`, literal(""))
})

func emptyLinePostProcessors() []processor {
	return []processor{
		emptyTagProcessor(),
		reProcessor("empty_line", `^[\s-]+$`, literal("")),
	}
}

// isLanguage reports whether a subtitle's language is babelfish's
// Language(iso3): the same language, without a country.
func (c *modContext) isLanguage(iso3 string) bool { return c.lang == iso3 && c.region == "" }

// lineMod is a mod applied line by line.
type lineMod struct {
	pre, procs, post, last []processor
}

// hearingImpairedCues are the words of HI_all_caps.
var hearingImpairedCues = []string{
	"LAUGH", "APPLAU", "CHEER", "MUSIC", "GASP", "SIGHS", "GROAN", "COUGH", "SCREAM", "SHOUT", "WHISPER",
	"PHONE", "DOOR", "KNOCK", "FOOTSTEP", "THUNDER", "EXPLOSION", "GUNSHOT", "SIREN",
}

// hearingImpairedProcessors are the processors of remove_HI.
var hearingImpairedProcessors = sync.OnceValue(func() []processor {
	fullBracket := pyRegex(withTags(`(?s)^-?%(t)s[([].+(?=[^)\]]{3,}).+[)\]]%(t)s$`))
	notUppercase := func(c *modContext) bool { return !c.mostlyUppercase }
	return []processor{
		// FullBracketEntryProcessor: the whole cue in brackets.
		{name: "HI_brackets_full", apply: func(content string, ctx *lineContext) (string, error) {
			if ctx.entry != "" && pyStrip(pySubString(fullBracket, ctx.entry, "")) == "" {
				return "", errEmptyEntry
			}
			return content, nil
		}},
		reProcessor("HI_before_colon_caps",
			`(?:(?<=^)|(?<=[.\-!?"'])\s)([\s\->~]*(?=[A-ZÀ-Ž&+]\s*[A-ZÀ-Ž&+]\s*[A-ZÀ-Ž&+])`+
				`[A-zÀ-ž-_0-9\s"'&+()\[\],:]+:(?!["'’ʼ❜‘‛”“‟„])(?:\s+|$))(?![0-9])`, literal("")),
		reProcessor("HI_before_colon_noncaps",
			`(?:(?<=^)|(?<=[.\-!?"]))([\s\->~]*((?=[A-zÀ-ž&+]\s*[A-zÀ-ž&+]\s*[A-zÀ-ž&+])`+
				`[A-zÀ-ž-_0-9\s"'&+()\[\]]+:)(?!["’ʼ❜‘‛”“‟„])\s*)(?![0-9]|//)`,
			func(m regexp2.Match) string {
				g1 := group(m, 1)
				switch {
				case strings.Contains(group(m, 2), " ") || strings.Contains(g1, "-"):
					return g1
				case strings.HasPrefix(g1, " "):
					return " "
				default:
					return ""
				}
			}),
		reProcessor("HI_brackets",
			withTags(`(?s)-?%(t)s["']*\[(?=[^\[\]]{3,})[A-Za-zÀ-ž0-9\s'".:-_&+]+[)\]]["']*[\s:]*%(t)s`), literal("")),
		withSupport(reProcessor("HI_all_caps", `(^(?=.*[A-ZÀ-Ž&+]{4,})[A-ZÀ-Ž-_\s&+]+$)`,
			func(m regexp2.Match) string {
				g1 := group(m, 1)
				for _, cue := range hearingImpairedCues {
					if strings.Contains(g1, cue) {
						return ""
					}
				}
				return g1
			}), notUppercase),
		reProcessor("HI_remove_man", `(?si)(\b(?:WO)MAN:\s*)`, literal("")),
		withSupport(reProcessor("HI_starting_upper_then_sentence",
			`^(?=[A-ZÀ-Ž]{4,})[A-ZÀ-Ž-_\s]+\s([A-ZÀ-Ž][a-zà-ž].+)`, template(1)), notUppercase),
		withSupport(reProcessor("JP_parentheses", `（.+）|\(.+\)`, literal("")),
			func(c *modContext) bool { return c.isLanguage("jpn") }),
	}
})

func withSupport(p processor, supported func(*modContext) bool) processor {
	p.supported = supported
	return p
}

// hearingImpairedMod is remove_HI.
var hearingImpairedMod = sync.OnceValue(func() lineMod {
	music := pyRegex(`(?ms)(^[-\s>~]*[*#¶♫♪]+\s*.+|.+\s*[*#¶♫♪]+\s*$|.+\s*[*#¶♫♪]+[)\]])`)
	return lineMod{
		procs: hearingImpairedProcessors(),
		post:  emptyLinePostProcessors(),
		last: []processor{
			reProcessor("HI_music_symbols_only",
				withTags(`(^%(t)s[*#¶♫♪\s]*%(t)s[*#¶♫♪\s]+%(t)s[*#¶♫♪\s]*%(t)s$)`), literal("")),
			// A ReProcessor with entry=True: it removes the whole cue.
			{name: "HI_music", apply: func(content string, ctx *lineContext) (string, error) {
				out := pySubString(music, ctx.entry, "")
				if out == "" {
					return "", errEmptyEntry
				}
				if out != ctx.entry {
					return out, nil
				}
				return content, nil
			}},
		},
	}
})

// commonMod is common.
var commonMod = sync.OnceValue(func() lineMod {
	return lineMod{
		procs: []processor{
			reProcessor("CM_hyphens", `([‑‐﹘﹣])`, literal("-")),
			reProcessor("CM_multidash", `(\w|\b|\s|^)(-\s?-{1,2})`, template(1, "—")),
			reProcessor("CM_non_word_only", `(^\W*[-_.:<>~"']+\W*$)`, literal("")),
			reProcessor("CM_leading_crocodiles", `^\s?>>\s*`, literal("")),
			reProcessor("CM_empty_colon_start", `(^\W*:\s*(?=\w+))`, literal("")),
			reProcessor("CM_music_symbols", `(^[-\s>~]*[*#¶]+\s+)|(\s*[*#¶]+\s*$)`, func(m regexp2.Match) string {
				if group(m, 1) != "" {
					return "♪ "
				}
				return " ♪"
			}),
			reProcessor("CM_double_apostrophe", `(['’ʼ❜‘‛]['’ʼ❜‘‛]+)`, literal(`"`)),
			reProcessor("CM_double_as_single", `([A-zÀ-ž])"([A-zÀ-ž])`, template(1, "'", 2)),
			reProcessor("CM_normalize_quotes", `(\s*["”“‟„])\s*(["”“‟„]["”“‟„\s]*)`, func(m regexp2.Match) string {
				if strings.HasSuffix(group(m, 2), " ") {
					return `" `
				}
				return `"`
			}),
			reProcessor("CM_normalize_squotes", `(['’ʼ❜‘‛])`, literal("'")),
			reProcessor("CM_leading_ellipsis", `^\.\.\.[\s]*`, literal("")),
			reProcessor("CM_crap", `(?i).+downloaded\s+from.+`, literal("")),
			reProcessor("CM_ellipsis_no_space", `\.\.\.(?![\s.,!?'"])(?!$)`, literal("... ")),
			reProcessor("CM_ellipsis_no_space2", `(?<=[^\s])(?<!\s)\. \. \.`, literal(" . . .")),
			reProcessor("CM_multiple_spaces", `[\s]{2,}`, literal(" ")),
			reProcessor("CM_dots", `\.{3,}`, literal("...")),
			reProcessor("CM_dash_space", `^-(?![\s-])`, literal("- ")),
			reProcessor("CM_starting_spacedots", `^(?!\s?(\.\s\.\s\.)|(\s?\.{3}))(?=\.+\s+)[\s.]*`, literal("")),
			reProcessor("CM_uppercase_i_in_word", `([a-zà-ž]+)(I+)`, func(m regexp2.Match) string {
				return group(m, 1) + strings.Repeat("l", len(group(m, 2)))
			}),
			reProcessor("CM_spaces_in_numbers",
				`(\b[0-9]+[0-9:']*(?<!\.\.)\s+(?!\.\.)[0-9,.:'\s]*(?=[0-9]+)[0-9,.:'])`,
				func(m regexp2.Match) string {
					g1 := group(m, 1)
					if strings.Count(g1, " ") == 1 {
						return strings.ReplaceAll(g1, " ", "")
					}
					return g1
				}),
			// Python's (?<!(?=X)) is a lookahead at the same position,
			// written so here.
			reProcessor("CM_uppercase_after_dot", `((?!\s*[A-ZÀ-Ž-_0-9.]\s*)(?:[^.\s])+\.\s+)([a-zà-ž])`,
				func(m regexp2.Match) string {
					g1, g2 := group(m, 1), group(m, 2)
					if utf8.RuneCountInString(g1) > 4 {
						return g1 + strings.ToUpper(g2)
					}
					return g1 + g2
				}),
			withSupport(reProcessor("CM_double_interpunct", `(\s*[,!?])\s*([,.!?][,.!?\s]*)`,
				func(m regexp2.Match) string {
					out := pyStrip(group(m, 1))
					if strings.HasSuffix(group(m, 2), " ") {
						out += " "
					}
					return out
				}),
				// Valid in Spanish; Subzero exempts Language("spa") and
				// Language("spa", "MX").
				func(c *modContext) bool { return c.lang != "spa" || c.region != "" && c.region != "MX" }),
			reProcessor("CM_punctuation_space", `(?:(?<=^)|(?<=\w)) +([!?.,](?![!?.,]| \.))`, template(1)),
			reProcessor("CM_punctuation_space2", `(([^\s]*)([!?.,:])([A-zÀ-ž]{2,}))`, func(m regexp2.Match) string {
				if hasTLD(group(m, 1)) {
					return group(m, 1)
				}
				return group(m, 2) + group(m, 3) + " " + group(m, 4)
			}),
			withSupport(reProcessor("CM_EN_lowercase_i", `(\b)i(\b)`, template(1, "I", 2)),
				func(c *modContext) bool { return c.isLanguage("eng") }),
		},
		post: emptyLinePostProcessors(),
	}
})

// reverseRTLMod is reverse_rtl.
var reverseRTLMod = sync.OnceValue(func() lineMod {
	return lineMod{procs: []processor{
		reProcessor("CM_RTL_reverse", `(^([\s.!?:,'-]*)(.+?)(\s*)(-?\s*)$)`, template(5, 4, 3, 2)),
	}}
})

// ocrMods are OCR_fixes by ISO 639-2/T code.
var ocrMods = sync.OnceValue(func() map[string]lineMod {
	fixF := reProcessor("OCR_fix_F", `(\bF)(')([A-zÀ-ž]*\b)`, template(1, 3))
	mods := map[string]lineMod{}
	for lang, d := range ocrDictionaries() {
		mods[lang] = lineMod{procs: append([]processor{fixF}, d.processors()...)}
	}
	return mods
})

// lineModFor returns a line mod by name.
func lineModFor(name string, ctx *modContext) lineMod {
	switch name {
	case "common":
		return commonMod()
	case "remove_HI":
		return hearingImpairedMod()
	case "reverse_rtl":
		return reverseRTLMod()
	case "OCR_fixes":
		return ocrMods()[ctx.lang]
	}
	return lineMod{}
}

// runProcessors is SubtitleModification._process.
func runProcessors(content string, procs []processor, ctx *lineContext) (string, error) {
	if content == "" {
		return "", nil
	}
	for _, p := range procs {
		if p.supported != nil && !p.supported(ctx.mod) {
			continue
		}
		var err error
		if content, err = p.apply(content, ctx); err != nil {
			return "", err
		}
		if content == "" {
			break
		}
	}
	return content, nil
}

// modify is SubtitleModification.modify.
func (m lineMod) modify(content string, ctx *lineContext, last bool) (string, error) {
	stages := [][]processor{m.pre, m.procs, m.post}
	if last {
		stages = [][]processor{m.last}
	}
	for _, procs := range stages {
		if content == "" {
			return "", nil
		}
		var err error
		if content, err = runProcessors(content, procs, ctx); err != nil {
			return "", err
		}
	}
	return content, nil
}

// fontStyleTagStart starts the {\i1}-style tags Subzero keeps away from
// the mods.
const fontStyleTagStart = `{\`

// applyLineMods is SubtitleModifications.apply_line_mods: each cue line
// by line through the mods, dropping the cues that end up empty.
func (s *Subtitle) applyLineMods(mods []finalMod, mctx *modContext) {
	resolved := make([]lineMod, len(mods))
	for i, m := range mods {
		resolved[i] = lineModFor(m.name, mctx)
	}
	var kept []Cue
	for _, cue := range s.Cues {
		text, ok := modifyEntry(entryText(cue.Text), resolved, mctx)
		if !ok {
			continue
		}
		cue.Text = strings.ReplaceAll(text, `\N`, "\n")
		kept = append(kept, cue)
	}
	if len(kept) > 0 {
		for i := range kept {
			kept[i].Index = i + 1
		}
		s.Cues = kept
	}
}

// modifyEntry applies the line mods to a cue's text, with \N line breaks.
// It reports false when the cue is to be dropped.
func modifyEntry(text string, mods []lineMod, mctx *modContext) (string, bool) {
	const maxLines = 10
	t := pyStrip(text)
	if t == "" {
		return "", false
	}
	split := strings.Split(t, `\N`)
	if len(split) > maxLines {
		return "", false
	}
	ctx := &lineContext{entry: t, mod: mctx}
	var lines, startTags, endTags []string
	for _, line := range split {
		line = pyStrip(line)
		if line == "" {
			continue
		}
		startTag, endTag := "", ""
		if strings.HasPrefix(line, fontStyleTagStart) {
			startTag, line = splitRunes(line, 5)
		}
		if rs := []rune(line); len(rs) >= 5 && string(rs[len(rs)-5:len(rs)-3]) == fontStyleTagStart {
			line, endTag = string(rs[:len(rs)-5]), string(rs[len(rs)-5:])
		}
		var lastMods []lineMod
		skipLine := false
		for _, m := range mods {
			var err error
			line, err = m.modify(pyStrip(line), ctx, false)
			if errors.Is(err, errEmptyEntry) {
				return "", false
			}
			if line == "" {
				skipLine = true
				break
			}
			if len(m.last) > 0 {
				lastMods = append(lastMods, m)
			}
		}
		if skipLine {
			continue
		}
		for _, m := range lastMods {
			var err error
			line, err = m.modify(pyStrip(line), ctx, true)
			if errors.Is(err, errEmptyEntry) {
				return "", false
			}
			if line == "" {
				skipLine = true
				break
			}
		}
		if skipLine {
			continue
		}
		if startTag != "" {
			startTags = append(startTags, startTag)
		}
		if endTag != "" {
			endTags = append(endTags, endTag)
		}
		cleaned, _ := emptyTagProcessor().apply(startTag+line+endTag, ctx)
		cleaned = pyStrip(cleaned)
		if cleaned == "" {
			continue
		}
		// A lone closing tag goes to the end of the previous line.
		if utf8.RuneCountInString(cleaned) == 5 && strings.HasPrefix(cleaned, fontStyleTagStart) &&
			strings.HasSuffix(cleaned, "0}") && len(lines) > 0 {
			lines[len(lines)-1] += cleaned
			continue
		}
		lines = append(lines, cleaned)
	}
	if len(lines) == 0 {
		return "", false
	}
	newText := strings.Join(lines, `\N`)
	// Subzero's "cheap man's approach to avoid open tags".
	if len(startTags) != len(endTags) {
		var addStart, addEnd []string
		for _, tag := range startTags {
			end := strings.ReplaceAll(tag, "1", "0")
			if !slices.Contains(endTags, end) && strings.Count(newText, tag) > strings.Count(newText, end) {
				addEnd = append(addEnd, end)
			}
		}
		for _, tag := range endTags {
			start := strings.ReplaceAll(tag, "0", "1")
			if !slices.Contains(startTags, start) && strings.Count(newText, tag) > strings.Count(newText, start) {
				addStart = append(addStart, start)
			}
		}
		newText = strings.Join(addStart, "") + newText + strings.Join(addEnd, "")
	}
	return newText, true
}

// splitRunes splits s after n runes.
func splitRunes(s string, n int) (string, string) {
	rs := []rune(s)
	n = min(n, len(rs))
	return string(rs[:n]), string(rs[n:])
}
