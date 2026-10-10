package release

import (
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

type languageRules struct {
	// words are languages recognized by their English name anywhere in the
	// title, in the app's order; English is matched last by Sonarr.
	words []languageWord
	// caseSensitive and caseInsensitive name the languages of their groups.
	caseSensitive, caseInsensitive         *regex
	caseSensitiveGroups, insensitiveGroups []languageGroup
	// cleanTitle strips the series title before matching.
	cleanTitle []regexReplace
}

type languageWord struct {
	words    []string
	language Language
}

type languageGroup struct {
	name     string
	language Language
}

func words(language Language, w ...string) languageWord { return languageWord{w, language} }

var languageRuleSets = map[Kind]languageRules{
	Series: {
		cleanTitle: []regexReplace{rr(`.*?[_. ](S\d{2}(?:E\d{2,4})*[_. ].*)`, "$1", ignoreCase)},
		words: []languageWord{
			words(Spanish, "spanish"), words(Danish, "danish"), words(Dutch, "dutch"), words(Japanese, "japanese"),
			words(Icelandic, "icelandic"), words(Chinese, "mandarin", "cantonese", "chinese"), words(Korean, "korean"),
			words(Russian, "russian"), words(Polish, "polish"), words(Vietnamese, "vietnamese"),
			words(Swedish, "swedish"), words(Norwegian, "norwegian"), words(Finnish, "finnish"),
			words(Turkish, "turkish"), words(Portuguese, "portuguese"), words(Hungarian, "hungarian"),
			words(Hebrew, "hebrew"), words(Arabic, "arabic"), words(Hindi, "hindi"), words(Malayalam, "malayalam"),
			words(Ukrainian, "ukrainian"), words(Bulgarian, "bulgarian"), words(Slovak, "slovak"),
			words(PortugueseBrazil, "brazilian", "dublado"), words(SpanishLatino, "latino"), words(Latvian, "latvian"),
		},
		caseInsensitive: re(`(?:\W|_)(?<english>\b(?:ing|eng)\b)|(?<italian>\b(?:ita|italian)\b)|(?<german>german\b|videomann|ger[. ]dub)|(?<flemish>flemish)|(?<greek>greek)|(?<french>(?:\W|_)(?:FR|VF|VF2|VFF|VFI|VFQ|TRUEFRENCH|FRENCH)(?:\W|_))|(?<russian>\b(?:rus|ru)\b)|(?<hungarian>\b(?:HUNDUB|HUN)\b)|(?<hebrew>\bHebDub\b)|(?<polish>\b(?:PL\W?DUB|DUB\W?PL|LEK\W?PL|PL\W?LEK)\b)|(?<chinese>\[(?:CH[ST]|BIG5|GB)\]|简|繁|字幕)|(?<bulgarian>\bbgaudio\b)|(?<spanish>\b(?:español|castellano|esp|spa(?!\(Latino\)))\b)|(?<ukrainian>\b(?:\dx?)?(?:ukr))|(?<thai>\b(?:THAI)\b)|(?<romanian>\b(?:RoDubbed|ROMANIAN)\b)|(?<catalan>[-,. ]cat[. ](?:DD|subs)|\b(?:catalan|catalán)\b)|(?<latvian>\b(?:lat|lav|lv)\b)|(?<turkish>\b(?:tur)\b)|(?<original>\b(?:orig|original)\b)`, ignoreCase),
		insensitiveGroups: []languageGroup{
			{"english", English},
			{"italian", Italian},
			{"german", German},
			{"flemish", Flemish},
			{"greek", Greek},
			{"french", French},
			{"russian", Russian},
			{"dutch", Dutch},
			{"hungarian", Hungarian},
			{"hebrew", Hebrew},
			{"polish", Polish},
			{"chinese", Chinese},
			{"bulgarian", Bulgarian},
			{"ukrainian", Ukrainian},
			{"spanish", Spanish},
			{"thai", Thai},
			{"romanian", Romanian},
			{"catalan", Catalan},
			{"latvian", Latvian},
			{"turkish", Turkish},
			{"original", Original},
		},
		caseSensitive: re(`(?:(?i)(?<!SUB[\W|_|^]))(?:(?<lithuanian>\bLT\b)|(?<czech>\bCZ\b)|(?<polish>\bPL\b)|(?<bulgarian>\bBG\b)|(?<slovak>\bSK\b))(?:(?i)(?![\W|_|^]SUB))`),
		caseSensitiveGroups: []languageGroup{
			{"lithuanian", Lithuanian}, {"czech", Czech}, {"polish", Polish}, {"bulgarian", Bulgarian}, {"slovak", Slovak},
		},
	},
	Movie: {
		words: []languageWord{
			words(English, "english"), words(Spanish, "spanish"), words(Danish, "danish"), words(Dutch, "dutch"),
			words(Japanese, "japanese"), words(Icelandic, "icelandic"), words(Chinese, "mandarin", "cantonese", "chinese"),
			words(Korean, "korean"), words(Russian, "russian"), words(Romanian, "romanian"), words(Hindi, "hindi"),
			words(Arabic, "arabic"), words(Thai, "thai"), words(Bulgarian, "bulgarian"), words(Polish, "polish"),
			words(Vietnamese, "vietnamese"), words(Swedish, "swedish"), words(Norwegian, "norwegian"),
			words(Finnish, "finnish"), words(Turkish, "turkish"), words(Portuguese, "portuguese"),
			words(PortugueseBrazil, "brazilian"), words(Hungarian, "hungarian"), words(Hebrew, "hebrew"),
			words(Ukrainian, "ukrainian"), words(Persian, "persian"), words(Bengali, "bengali"), words(Slovak, "slovak"),
			words(Latvian, "latvian"), words(SpanishLatino, "latino"), words(Tamil, "tamil"), words(Telugu, "telugu"),
			words(Malayalam, "malayalam"), words(Kannada, "kannada"), words(Albanian, "albanian"),
			words(Afrikaans, "afrikaans"), words(Marathi, "marathi"), words(Tagalog, "tagalog"),
		},
		caseInsensitive: re(`(?:\W|_|^)(?<english>\beng\b)|
                                                                            (?<italian>\b(?:ita|italian)\b)|
                                                                            (?<german>(?:swiss)?german\b|videomann|ger[. ]dub|\bger\b)|
                                                                            (?<flemish>flemish)|
                                                                            (?<bulgarian>bgaudio)|
                                                                            (?<romanian>rodubbed)|
                                                                            (?<brazilian>\b(dublado|pt-BR)\b)|
                                                                            (?<greek>greek)|
                                                                            (?<french>\b(?:FR|VO|VF|VFF|VFQ|VFI|VF2|TRUEFRENCH|FRENCH|FRE|FRA)\b)|
                                                                            (?<russian>\b(?:rus|ru)\b)|
                                                                            (?<hungarian>\b(?:HUNDUB|HUN)\b)|
                                                                            (?<hebrew>\b(?:HebDub|HebDubbed)\b)|
                                                                            (?<polish>\b(?:PL\W?DUB|DUB\W?PL|LEK\W?PL|PL\W?LEK)\b)|
                                                                            (?<chinese>\[(?:CH[ST]|BIG5|GB)\]|简|繁|字幕)|
                                                                            (?<ukrainian>(?:(?:\dx)?UKR))|
                                                                            (?<spanish>\b(?:español|castellano)\b)|
                                                                            (?<catalan>\b(?:catalan?|catalán|català)\b)|
                                                                            (?<latvian>\b(?:lat|lav|lv)\b)|
                                                                            (?<telugu>\btel\b)|
                                                                            (?<vietnamese>\bVIE\b)|
                                                                            (?<japanese>\bJAP\b)|
                                                                            (?<korean>\bKOR\b)|
                                                                            (?<urdu>\burdu\b)|
                                                                            (?<romansh>\b(?:romansh|rumantsch|romansch)\b)|
                                                                            (?<mongolian>\b(?:mongolian|khalkha)\b)|
                                                                            (?<georgian>\b(?:georgian|geo|ka|kat)\b)|
                                                                            (?<original>\b(?:orig|original)\b)`, ignoreCase, patternWS),
		insensitiveGroups: []languageGroup{
			{"english", English},
			{"italian", Italian},
			{"german", German},
			{"flemish", Flemish},
			{"greek", Greek},
			{"french", French},
			{"russian", Russian},
			{"bulgarian", Bulgarian},
			{"brazilian", PortugueseBrazil},
			{"dutch", Dutch},
			{"hungarian", Hungarian},
			{"hebrew", Hebrew},
			{"polish", Polish},
			{"chinese", Chinese},
			{"spanish", Spanish},
			{"catalan", Catalan},
			{"ukrainian", Ukrainian},
			{"latvian", Latvian},
			{"romanian", Romanian},
			{"telugu", Telugu},
			{"vietnamese", Vietnamese},
			{"japanese", Japanese},
			{"korean", Korean},
			{"urdu", Urdu},
			{"romansh", Romansh},
			{"mongolian", Mongolian},
			{"georgian", Georgian},
			{"original", Original},
		},
		caseSensitive: re(`(?:(?i)(?<!SUB[\W|_|^]))(?:(?<english>\bEN\b)|
                                                                                                          (?<lithuanian>\bLT\b)|
                                                                                                          (?<czech>\bCZ\b)|
                                                                                                          (?<polish>\bPL\b)|
                                                                                                          (?<bulgarian>\bBG\b)|
                                                                                                          (?<slovak>\bSK\b)|
                                                                                                          (?<german>\bDE\b)|
                                                                                                          (?<spanish>\b(?<!DTS[._ -])ES\b))(?:(?i)(?![\W|_|^]SUB))`, patternWS),
		caseSensitiveGroups: []languageGroup{
			{"english", English},
			{"lithuanian", Lithuanian},
			{"czech", Czech},
			{"polish", Polish},
			{"bulgarian", Bulgarian},
			{"slovak", Slovak},
			{"spanish", Spanish},
			{"german", German},
		},
	},
}

var (
	germanDualLanguage    = re(`(?<!WEB[-_. ]?)\bDL\b`, ignoreCase)
	germanMultiLanguage   = re(`\bML\b`, ignoreCase)
	subtitleLanguage      = re(`.+?([-_. ](?<tags>forced|foreign|default|cc|psdh|sdh))*[-_. ](?<iso_code>[a-z]{2,3})([-_. ](?<tags>forced|foreign|default|cc|psdh|sdh))*$`, ignoreCase)
	subtitleLanguageTitle = re(`.+?(\.((?<tags1>forced|foreign|default|cc|psdh|sdh)|(?<iso_code>[a-z]{2,3})))*[-_. ](?<title>[^.]*)(\.((?<tags2>forced|foreign|default|cc|psdh|sdh)|(?<iso_code>[a-z]{2,3})))*$`, ignoreCase)
	subtitleTitle         = re(`^((?<title>.+) - )?(?<copy>(?<!\d+)\d{1,3}(?!\d+))$`)
)

// ParseLanguages returns the languages named in a release title, or
// LanguageUnknown when there are none.
func ParseLanguages(title string, kind Kind) []Language {
	rules := languageRuleSets[kind]
	for _, r := range rules.cleanTitle {
		if r.tryReplace(&title) {
			break
		}
	}
	lower := strings.ToLower(title)
	var languages []Language
	for _, w := range rules.words {
		if slices.ContainsFunc(w.words, func(s string) bool { return strings.Contains(lower, s) }) {
			languages = append(languages, w.language)
		}
	}
	if kind == Series {
		// Sonarr matches the case-sensitive tags once.
		if m := rules.caseSensitive.match(title); m != nil {
			languages = appendGroups(languages, m, rules.caseSensitiveGroups)
		}
	} else {
		for _, m := range rules.caseSensitive.matches(title) {
			languages = appendGroups(languages, m, rules.caseSensitiveGroups)
		}
	}
	for _, m := range rules.caseInsensitive.matches(title) {
		languages = appendGroups(languages, m, rules.insensitiveGroups)
	}
	if kind == Series && strings.Contains(lower, "english") {
		languages = append(languages, English)
	}
	if len(languages) == 0 {
		languages = append(languages, LanguageUnknown)
	}
	if len(languages) == 1 && languages[0] == German {
		switch {
		case germanDualLanguage.isMatch(title):
			languages = append(languages, Original)
		case germanMultiLanguage.isMatch(title):
			languages = append(languages, Original, English)
		}
	}
	return distinct(languages)
}

func appendGroups(languages []Language, m *match, groups []languageGroup) []Language {
	for _, g := range groups {
		if m.has(g.name) {
			languages = append(languages, g.language)
		}
	}
	return languages
}

func distinct[T comparable](s []T) []T {
	out := s[:0:0]
	for _, v := range s {
		if !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	return out
}

// ParseSubtitleLanguage returns the language of a subtitle file from its
// name, such as "Title.en.forced.srt".
func ParseSubtitleLanguage(fileName string, kind Kind) Language {
	name := fileNameWithoutExtension(fileName)
	if m := subtitleLanguage.match(name); m != nil {
		if iso, ok := FindIsoLanguage(strings.ToLower(m.value("iso_code")), kind); ok {
			return iso.Language
		}
		return LanguageUnknown
	}
	for _, l := range kindLanguages[kind] {
		if strings.HasSuffix(strings.ToLower(name), strings.ToLower(l.String())) {
			return l
		}
	}
	return LanguageUnknown
}

// ParseLanguageTags returns the tags of a subtitle file name, such as
// "forced" and "sdh".
func ParseLanguageTags(fileName string) []string {
	m := subtitleLanguage.match(fileNameWithoutExtension(fileName))
	if m == nil {
		return []string{}
	}
	return captureTags(m, "tags")
}

func captureTags(m *match, groups ...string) []string {
	tags := []string{}
	for _, g := range groups {
		for _, c := range m.captures(g) {
			if c.Value != "" {
				tags = append(tags, strings.ToLower(c.Value))
			}
		}
	}
	return tags
}

// SubtitleTitleInfo describes a subtitle file name.
type SubtitleTitleInfo struct {
	Language     Language
	LanguageTags []string
	// RawTitle is the title of the track, such as "Commentary - 2"; nil
	// when the name has none.
	RawTitle *string
	// Title is RawTitle without its copy number.
	Title *string
	// Copy distinguishes tracks with the same language and title.
	Copy int
	// TitleFirst reports whether the title precedes the language.
	TitleFirst bool
}

// ParseSubtitleLanguageInformation parses the language, tags and title of a
// subtitle file name.
func ParseSubtitleLanguageInformation(fileName string, kind Kind) SubtitleTitleInfo {
	name := fileNameWithoutExtension(fileName)
	m := subtitleLanguageTitle.match(name)
	if m == nil || !m.has("title") || len(m.captures("iso_code")) != 1 {
		return SubtitleTitleInfo{
			LanguageTags: ParseLanguageTags(fileName),
			Language:     ParseSubtitleLanguage(fileName, kind),
		}
	}
	language := LanguageUnknown
	if iso, ok := FindIsoLanguage(strings.ToLower(m.value("iso_code")), kind); ok {
		language = iso.Language
	}
	raw := m.value("title")
	info := SubtitleTitleInfo{
		TitleFirst:   len(m.captures("tags1")) == 0,
		LanguageTags: captureTags(m, "tags1", "tags2"),
		RawTitle:     &raw,
		Language:     language,
	}
	info.updateTitle()
	return info
}

// updateTitle splits RawTitle into Title and Copy.
func (s *SubtitleTitleInfo) updateTitle() {
	switch {
	case s.RawTitle == nil:
		s.Title, s.Copy = nil, 0
	case subtitleTitle.match(*s.RawTitle) != nil:
		m := subtitleTitle.match(*s.RawTitle)
		s.Title = nil
		if t, ok := m.group("title"); ok {
			s.Title = &t
		}
		s.Copy, _ = strconv.Atoi(m.value("copy"))
	default:
		s.Title, s.Copy = s.RawTitle, 0
	}
}

// fileNameWithoutExtension returns the last element of a Unix or Windows
// path without its extension.
func fileNameWithoutExtension(path string) string {
	if i := strings.LastIndexAny(path, `/\`); i >= 0 {
		path = path[i+1:]
	}
	return strings.TrimSuffix(path, filepath.Ext(path))
}
