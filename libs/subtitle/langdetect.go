package subtitle

import (
	"cmp"
	"maps"
	"slices"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

// Language detection ported from guess_language-spirit, which Bazarr uses
// for subtitles without a language: the Unicode blocks of the letters
// narrow the candidates, and trigram models choose among them.

const (
	guessMaxLength = 4096
	guessMinLength = 20
	guessMaxGrams  = 300
)

// guessLanguageData is the data of guess_language: block names by code
// point >> 4 ("" where it has none) and the trigram models, each trigram
// mapped to its rank.
type guessLanguageData struct {
	blocks []string
	models map[string]map[string]int
}

var guessData = sync.OnceValue(func() *guessLanguageData {
	file := loadData[struct {
		Blocks [][3]any            `json:"blocks"`
		Models map[string][]string `json:"models"`
	}]("guesslang.json")
	d := &guessLanguageData{models: map[string]map[string]int{}}
	for _, b := range file.Blocks {
		lo, hi, name := int(b[0].(float64)), int(b[1].(float64)), b[2].(string)
		if len(d.blocks) < hi {
			d.blocks = append(d.blocks, make([]string, hi-len(d.blocks))...)
		}
		for i := lo; i < hi; i++ {
			d.blocks[i] = name
		}
	}
	for lang, grams := range file.Models {
		ranks := make(map[string]int, len(grams))
		for i, g := range grams {
			ranks[g] = i
		}
		d.models[lang] = ranks
	}
	return d
})

// The candidate languages of guess_language, by script.
var (
	guessBasicLatin = []string{
		"ceb", "en", "eu", "ha", "haw", "id", "la", "nr", "nso", "so", "ss", "st",
		"sw", "tlh", "tn", "ts", "xh", "zu",
	}
	guessExtendedLatin = []string{
		"af", "az", "ca", "cs", "cy", "da", "de", "eo", "es", "et", "fi", "fr",
		"hr", "hu", "is", "it", "lt", "lv", "nb", "nl", "pl", "pt", "ro", "sk",
		"sl", "sq", "sv", "tl", "tr", "ve", "vi",
	}
	guessCyrillic   = []string{"bg", "kk", "ky", "mk", "mn", "ru", "sr", "uk", "uz"}
	guessArabic     = []string{"ar", "fa", "ps", "ur"}
	guessDevanagari = []string{"hi", "ne"}
	guessPortuguese = []string{"pt_BR", "pt_PT"}
	guessSingletons = [][2]string{
		{"Armenian", "hy"},
		{"Hebrew", "he"},
		{"Bengali", "bn"},
		{"Gurmukhi", "pa"},
		{"Greek", "el"},
		{"Gujarati", "gu"},
		{"Oriya", "or"},
		{"Tamil", "ta"},
		{"Telugu", "te"},
		{"Kannada", "kn"},
		{"Malayalam", "ml"},
		{"Sinhala", "si"},
		{"Thai", "th"},
		{"Lao", "lo"},
		{"Tibetan", "bo"},
		{"Burmese", "my"},
		{"Georgian", "ka"},
		{"Mongolian", "mn-Mong"},
		{"Khmer", "km"},
	}
)

// DetectLanguage guesses the language of a text, such as the cues of a
// sidecar subtitle without a language, as Bazarr does with
// guess_language. It returns a BCP 47 tag, "" when it cannot tell.
// Chinese is "zh-Hans" or "zh-Hant" by the script of its characters (see
// DetectChineseScript), the script of most characters unique to one script
// when both appear, and "zh" when the characters do not tell; Portuguese
// is "pt-BR" or "pt-PT".
func DetectLanguage(text string) string {
	tag := guessLanguage(text)
	switch tag {
	case "zh":
		script, simplified, traditional := chineseScript(text)
		switch {
		case script == ChineseTraditional || script == ChineseMixed && traditional > simplified:
			return "zh-Hant"
		case script == ChineseSimplified || script == ChineseMixed && simplified > traditional:
			return "zh-Hans"
		}
	case "pt_BR", "pt_PT":
		return strings.ReplaceAll(tag, "_", "-")
	}
	return tag
}

// guessLanguage is guess_language.guess_language without spelling
// dictionaries: its tags, "" for UNKNOWN.
func guessLanguage(text string) string {
	if utf8.RuneCountInString(text) > guessMaxLength {
		text = string([]rune(text)[:guessMaxLength])
	}
	words := guessWords(strings.ReplaceAll(text, "’", "'"))
	scripts := findRuns(words)
	has := func(names ...string) bool {
		for _, n := range names {
			if scripts[n] {
				return true
			}
		}
		return false
	}
	switch {
	case has("Hangul Syllables", "Hangul Jamo", "Hangul Compatibility Jamo", "Hangul"):
		return "ko"
	case has("Greek and Coptic"):
		return "el"
	case has("Kana"):
		return "ja"
	case has("CJK Unified Ideographs", "Bopomofo", "Bopomofo Extended", "KangXi Radicals"):
		return "zh"
	case has("Cyrillic"):
		return checkWithModels(words, guessCyrillic)
	case has("Arabic", "Arabic Presentation Forms-A", "Arabic Presentation Forms-B"):
		return checkWithModels(words, guessArabic)
	case has("Devanagari"):
		return checkWithModels(words, guessDevanagari)
	}
	for _, s := range guessSingletons {
		if scripts[s[0]] {
			return s[1]
		}
	}
	if has("Extended Latin") {
		lang := checkWithModels(words, guessExtendedLatin)
		if lang == "pt" {
			return checkWithModels(words, guessPortuguese)
		}
		return lang
	}
	if has("Basic Latin") {
		return checkWithModels(words, slices.Concat(guessBasicLatin, guessExtendedLatin))
	}
	return ""
}

// guessWords is WORD_RE.findall: runs of letters and apostrophes.
func guessWords(text string) []string {
	isWordRune := func(r rune) bool {
		return r == '\'' || r == '’' || isPyWord(r) && !unicode.IsDigit(r) && r != '_'
	}
	return strings.FieldsFunc(text, func(r rune) bool { return !isWordRune(r) })
}

// findRuns returns the blocks of at least 40% of the letters, or 15% for
// Basic Latin.
func findRuns(words []string) map[string]bool {
	blocks := guessData().blocks
	counts := map[string]int{}
	total := 0
	for _, w := range words {
		for _, r := range w {
			block := ""
			if i := int(r >> 4); i < len(blocks) {
				block = blocks[i]
			}
			counts[block]++
			total++
		}
	}
	runs := map[string]bool{}
	for block, n := range counts {
		pct := n * 100 / total
		if block != "" && (pct >= 40 || pct >= 15 && block == "Basic Latin") {
			runs[block] = true
		}
	}
	return runs
}

// checkWithModels returns the language whose trigram model is closest to
// the words, "" for text too short to tell.
func checkWithModels(words, languages []string) string {
	sample := strings.Join(words, " ")
	if utf8.RuneCountInString(sample) < guessMinLength {
		return ""
	}
	model := orderedTrigrams(sample)
	models := guessData().models
	best, bestDist := "", -1
	for _, lang := range languages {
		known, ok := models[strings.ToLower(lang)]
		if !ok {
			continue
		}
		dist := 0
		for i, gram := range model[:min(len(model), guessMaxGrams)] {
			if rank, ok := known[gram]; ok {
				dist += abs(i - rank)
			} else {
				dist += guessMaxGrams
			}
		}
		if bestDist < 0 || dist < bestDist || dist == bestDist && lang < best {
			best, bestDist = lang, dist
		}
	}
	return best
}

// orderedTrigrams returns the trigrams of the lower-cased text, the most
// frequent first.
func orderedTrigrams(text string) []string {
	rs := []rune(pyLower(text))
	counts := map[string]int{}
	for i := 0; i+2 < len(rs); i++ {
		counts[string(rs[i:i+3])]++
	}
	return slices.SortedFunc(maps.Keys(counts), func(a, b string) int {
		return cmp.Or(cmp.Compare(counts[b], counts[a]), cmp.Compare(a, b))
	})
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
