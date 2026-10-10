package subtitle

import (
	"path/filepath"
	"strings"
)

// chineseSimplifiedTokens lists tokens representing Simplified Chinese.
var chineseSimplifiedTokens = []string{
	"chs", "sc", "gb", "zh-cn", "zh-sg", "zh-hans", "简", "简体", "简中",
}

// chineseTraditionalTokens lists tokens representing Traditional Chinese.
var chineseTraditionalTokens = []string{
	"cht", "tc", "big5", "zh-tw", "zh-hk", "zh-mo", "zh-hant", "繁", "繁体", "繁體", "繁中",
}

// chineseGeneralTokens lists general Chinese tokens.
var chineseGeneralTokens = []string{
	"zh", "chi", "zho", "chinese", "中文", "双语", "中字", "中英", "中日", "中韩",
}

// iso639Equivalents maps common 2-letter and 3-letter codes to canonical forms.
var iso639Equivalents = map[string]string{
	"en": "en", "eng": "en",
	"ja": "ja", "jpn": "ja",
	"ko": "ko", "kor": "ko",
	"fr": "fr", "fra": "fr", "fre": "fr",
	"de": "de", "deu": "de", "ger": "de",
	"es": "es", "spa": "es",
	"it": "it", "ita": "it",
	"ru": "ru", "rus": "ru",
	"pt": "pt", "por": "pt",
}

// NormalizeLanguage maps language tags, ISO codes, and common locale variants
// to canonical BCP-47 / ISO representations. In particular, Chinese aliases
// and dialect tokens are normalized into "zh-Hans", "zh-Hant", or "zh".
func NormalizeLanguage(tag string) string {
	tag = strings.TrimSpace(strings.ToLower(tag))
	if tag == "" {
		return ""
	}

	for _, token := range chineseSimplifiedTokens {
		if tag == token {
			return "zh-Hans"
		}
	}
	for _, token := range chineseTraditionalTokens {
		if tag == token {
			return "zh-Hant"
		}
	}
	for _, token := range chineseGeneralTokens {
		if tag == token {
			return "zh"
		}
	}

	if canon, ok := iso639Equivalents[tag]; ok {
		return canon
	}
	return tag
}

// IsChinese reports whether the given language tag or token denotes Chinese.
func IsChinese(tag string) bool {
	norm := NormalizeLanguage(tag)
	return norm == "zh" || norm == "zh-Hans" || norm == "zh-Hant"
}

// MatchesLanguage reports whether the actual subtitle language matches the
// user's preferred language.
//
// If preferred is generic Chinese ("zh"), any Chinese dialect/script matches.
// If preferred specifies a script ("zh-Hans" or "zh-Hant"), exact script match
// or generic "zh" is accepted.
// For other languages, 2-letter and 3-letter ISO code equivalences are honored.
func MatchesLanguage(preferred, actual string) bool {
	pNorm := NormalizeLanguage(preferred)
	aNorm := NormalizeLanguage(actual)
	if pNorm == "" || aNorm == "" {
		return false
	}
	if pNorm == aNorm {
		return true
	}
	if pNorm == "zh" && IsChinese(actual) {
		return true
	}
	if (pNorm == "zh-Hans" || pNorm == "zh-Hant") && aNorm == "zh" {
		return true
	}
	return false
}

// ExtractLanguageFromStem attempts to extract a language code from tokens
// present in a subtitle file stem (e.g. "Movie.chs" -> "zh-Hans").
func ExtractLanguageFromStem(stem string) string {
	lower := strings.ToLower(stem)
	delims := []rune{'.', '_', '-', ' ', '[', ']', '(', ')'}
	tokens := strings.FieldsFunc(lower, func(r rune) bool {
		for _, d := range delims {
			if r == d {
				return true
			}
		}
		return false
	})
	for i := len(tokens) - 1; i >= 0; i-- {
		t := tokens[i]
		norm := NormalizeLanguage(t)
		if norm != t || norm == "zh" || norm == "en" || norm == "ja" || norm == "ko" {
			return norm
		}
	}
	return ""
}

// ScoreSubtitle scores a subtitle candidate against a video file stem and
// user language preferences according to docs/architecture.md §9.1:
//
//   - Exact stem match: +1000
//   - Prefix match + boundary delimiter: +500
//   - User preferred language match: +200
//   - Other non-preferred language: -100
//   - Forced subtitle tag: -150
//   - SDH / hearing-impaired / commentary tag: -50
//   - Format bonus: ASS/SSA +20, SRT +10, VTT +5
func ScoreSubtitle(mediaStem, subPath, subLang string, preferredLangs []string, forced, sdh bool) int {
	score := 0
	base := filepath.Base(subPath)
	ext := filepath.Ext(base)
	subStem := strings.TrimSuffix(base, ext)

	// Stem matching
	if strings.EqualFold(subStem, mediaStem) {
		score += 1000
	} else if len(subStem) > len(mediaStem) && strings.EqualFold(subStem[:len(mediaStem)], mediaStem) {
		rem := subStem[len(mediaStem):]
		if rem != "" && (rem[0] == '.' || rem[0] == '_' || rem[0] == '-' || rem[0] == ' ') {
			score += 500
		}
	}

	// Language determination: use metadata subLang if present, else infer from stem.
	lang := subLang
	if lang == "" || lang == "und" {
		lang = ExtractLanguageFromStem(subStem)
	}

	// Language match scoring
	if len(preferredLangs) > 0 {
		matched := false
		for _, pref := range preferredLangs {
			if MatchesLanguage(pref, lang) {
				score += 200
				matched = true
				break
			}
		}
		if !matched && lang != "" && lang != "und" {
			score -= 100
		}
	}

	// Flag adjustments
	if forced {
		score -= 150
	}
	if sdh {
		score -= 50
	}

	// Format bonus
	switch strings.ToLower(strings.TrimPrefix(ext, ".")) {
	case "ass", "ssa":
		score += 20
	case "srt":
		score += 10
	case "vtt":
		score += 5
	}

	return score
}
