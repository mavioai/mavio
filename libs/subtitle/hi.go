package subtitle

import (
	"strings"
	"sync"

	"github.com/dlclark/regexp2"
)

// hearingImpairedPatterns are Bazarr's HI_REGEX_WITH_PARENTHESIS and
// HI_REGEX_WITHOUT_PARENTHESIS (subliminal_patch/core.py).
var hearingImpairedPatterns = sync.OnceValue(func() [2]*regexp2.Regexp {
	return [2]*regexp2.Regexp{
		pyRegex(`[*¶♫♪].{3,}[*¶♫♪]|[\[\(\{].{3,}[\]\)\}](?<!\{\\an\d\})`),
		pyRegex(`[*¶♫♪].{3,}[*¶♫♪]|[\[\{].{3,}[\]\}](?<!\{\\an\d\})`),
	}
})

// DetectHearingImpaired reports whether the text of a subtitle file looks
// like hearing-impaired subtitles, as Bazarr's parse_for_hi_regex tells
// it: text of at least three characters on one line between music
// symbols, brackets, braces or parentheses, though not between the
// parentheses of Arabic and Persian, which mark translator notes, nor in
// an {\anN} position tag. language is the BCP 47 tag of the subtitles, ""
// when unknown.
func DetectHearingImpaired(text, language string) bool {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	patterns := hearingImpairedPatterns()
	re := patterns[0]
	switch newModContext(language).lang {
	case "ara", "fas":
		re = patterns[1]
	}
	return pySearch(re, text)
}
