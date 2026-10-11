package release

import "strings"

// Language is a spoken or subtitle language of a release. The members are
// the union of Sonarr's and Radarr's languages; each Kind only reports the
// languages its app knows.
type Language int

// Languages.
const (
	LanguageUnknown Language = iota
	English
	French
	Spanish
	German
	Italian
	Danish
	Dutch
	Japanese
	Icelandic
	Chinese
	Russian
	Polish
	Vietnamese
	Swedish
	Norwegian
	Finnish
	Turkish
	Portuguese
	Flemish
	Greek
	Korean
	Hungarian
	Hebrew
	Lithuanian
	Czech
	Arabic
	Hindi
	Bulgarian
	Malayalam
	Ukrainian
	Slovak
	Thai
	PortugueseBrazil
	SpanishLatino
	Romanian
	Latvian
	Persian
	Catalan
	Croatian
	Serbian
	Bosnian
	Estonian
	Tamil
	Indonesian
	Macedonian
	Slovenian
	Bengali
	Telugu
	Kannada
	Albanian
	Afrikaans
	Marathi
	Tagalog
	Urdu
	Romansh
	Mongolian
	Georgian
	// Original is the original language of the movie or series.
	Original
	// Any is any language, which profiles and custom formats may want, as
	// Radarr's Language.Any; it is not Unknown, the language of releases
	// whose language is not known. No release is in Any.
	Any
)

var languageNames = [...]string{
	"Unknown", "English", "French", "Spanish", "German", "Italian", "Danish", "Dutch", "Japanese", "Icelandic",
	"Chinese", "Russian", "Polish", "Vietnamese", "Swedish", "Norwegian", "Finnish", "Turkish", "Portuguese",
	"Flemish", "Greek", "Korean", "Hungarian", "Hebrew", "Lithuanian", "Czech", "Arabic", "Hindi", "Bulgarian",
	"Malayalam", "Ukrainian", "Slovak", "Thai", "Portuguese (Brazil)", "Spanish (Latino)", "Romanian", "Latvian",
	"Persian", "Catalan", "Croatian", "Serbian", "Bosnian", "Estonian", "Tamil", "Indonesian", "Macedonian",
	"Slovenian", "Bengali", "Telugu", "Kannada", "Albanian", "Afrikaans", "Marathi", "Tagalog", "Urdu", "Romansh",
	"Mongolian", "Georgian", "Original", "Any",
}

// String returns the English name of the language.
func (l Language) String() string {
	if l < 0 || int(l) >= len(languageNames) {
		return "Unknown"
	}
	return languageNames[l]
}

// LanguageByName returns the language String names, ignoring case, such
// as "English", "Portuguese (Brazil)", "Unknown", "Original" or "Any"; ok is
// false for an unknown name.
func LanguageByName(name string) (l Language, ok bool) {
	for i, n := range languageNames {
		if strings.EqualFold(n, name) {
			return Language(i), true
		}
	}
	return LanguageUnknown, false
}

// kindLanguages lists each app's languages in its declaration order.
var kindLanguages = map[Kind][]Language{
	Series: {
		LanguageUnknown, English, French, Spanish, German, Italian, Danish, Dutch, Japanese, Icelandic, Chinese,
		Russian, Polish, Vietnamese, Swedish, Norwegian, Finnish, Turkish, Portuguese, Flemish, Greek, Korean,
		Hungarian, Hebrew, Lithuanian, Czech, Arabic, Hindi, Bulgarian, Malayalam, Ukrainian, Slovak, Thai,
		PortugueseBrazil, SpanishLatino, Romanian, Latvian, Persian, Catalan, Croatian, Serbian, Bosnian, Estonian,
		Tamil, Indonesian, Macedonian, Slovenian, Original,
	},
	Movie: {
		LanguageUnknown, English, French, Spanish, German, Italian, Danish, Dutch, Japanese, Icelandic, Chinese,
		Russian, Polish, Vietnamese, Swedish, Norwegian, Finnish, Turkish, Portuguese, Flemish, Greek, Korean,
		Hungarian, Hebrew, Lithuanian, Czech, Hindi, Romanian, Thai, Bulgarian, PortugueseBrazil, Arabic, Ukrainian,
		Persian, Bengali, Slovak, Latvian, SpanishLatino, Catalan, Croatian, Serbian, Bosnian, Estonian, Tamil,
		Indonesian, Telugu, Macedonian, Slovenian, Malayalam, Kannada, Albanian, Afrikaans, Marathi, Tagalog, Urdu,
		Romansh, Mongolian, Georgian, Original,
	},
}

// IsoLanguage is a language with its ISO 639 codes.
type IsoLanguage struct {
	TwoLetterCode   string
	CountryCode     string
	ThreeLetterCode string
	Language        Language
}

var isoLanguages = map[Kind][]IsoLanguage{
	Series: {
		{"en", "", "eng", English},
		{"fr", "fr", "fra", French},
		{"es", "", "spa", Spanish},
		{"de", "de", "deu", German},
		{"it", "", "ita", Italian},
		{"da", "", "dan", Danish},
		{"nl", "", "nld", Dutch},
		{"ja", "", "jpn", Japanese},
		{"is", "", "isl", Icelandic},
		{"zh", "cn", "zho", Chinese},
		{"ru", "", "rus", Russian},
		{"pl", "", "pol", Polish},
		{"vi", "", "vie", Vietnamese},
		{"sv", "", "swe", Swedish},
		{"no", "", "nor", Norwegian},
		{"nb", "", "nob", Norwegian},
		{"fi", "", "fin", Finnish},
		{"tr", "", "tur", Turkish},
		{"pt", "pt", "por", Portuguese},
		{"nl", "", "nld", Flemish},
		{"el", "", "ell", Greek},
		{"ko", "", "kor", Korean},
		{"hu", "", "hun", Hungarian},
		{"he", "", "heb", Hebrew},
		{"lt", "", "lit", Lithuanian},
		{"cs", "", "ces", Czech},
		{"ar", "", "ara", Arabic},
		{"hi", "", "hin", Hindi},
		{"bg", "", "bul", Bulgarian},
		{"ml", "", "mal", Malayalam},
		{"uk", "", "ukr", Ukrainian},
		{"sk", "", "slk", Slovak},
		{"th", "th", "tha", Thai},
		{"pt", "br", "por", PortugueseBrazil},
		{"es", "mx", "spa", SpanishLatino},
		{"ro", "", "ron", Romanian},
		{"lv", "", "lav", Latvian},
		{"fa", "", "fas", Persian},
		{"ca", "", "cat", Catalan},
		{"hr", "", "hrv", Croatian},
		{"sr", "", "srp", Serbian},
		{"bs", "", "bos", Bosnian},
		{"et", "", "est", Estonian},
		{"ta", "", "tam", Tamil},
		{"id", "", "ind", Indonesian},
		{"mk", "", "mkd", Macedonian},
		{"sl", "", "slv", Slovenian},
	},
	Movie: {
		{"en", "", "eng", English},
		{"fr", "fr", "fra", French},
		{"es", "", "spa", Spanish},
		{"de", "de", "deu", German},
		{"it", "", "ita", Italian},
		{"da", "", "dan", Danish},
		{"nl", "", "nld", Dutch},
		{"ja", "", "jpn", Japanese},
		{"is", "", "isl", Icelandic},
		{"zh", "cn", "zho", Chinese},
		{"ru", "", "rus", Russian},
		{"pl", "", "pol", Polish},
		{"vi", "", "vie", Vietnamese},
		{"sv", "", "swe", Swedish},
		{"no", "", "nor", Norwegian},
		{"nb", "", "nob", Norwegian},
		{"fi", "", "fin", Finnish},
		{"tr", "", "tur", Turkish},
		{"pt", "pt", "por", Portuguese},
		{"el", "", "ell", Greek},
		{"ko", "", "kor", Korean},
		{"hu", "", "hun", Hungarian},
		{"he", "", "heb", Hebrew},
		{"cs", "", "ces", Czech},
		{"hi", "", "hin", Hindi},
		{"th", "", "tha", Thai},
		{"bg", "", "bul", Bulgarian},
		{"ro", "", "ron", Romanian},
		{"pt", "br", "", PortugueseBrazil},
		{"ar", "", "ara", Arabic},
		{"uk", "", "ukr", Ukrainian},
		{"fa", "", "fas", Persian},
		{"bn", "", "ben", Bengali},
		{"lt", "", "lit", Lithuanian},
		{"sk", "", "slk", Slovak},
		{"lv", "", "lav", Latvian},
		{"es", "mx", "spa", SpanishLatino},
		{"ca", "", "cat", Catalan},
		{"hr", "", "hrv", Croatian},
		{"sr", "", "srp", Serbian},
		{"bs", "", "bos", Bosnian},
		{"et", "", "est", Estonian},
		{"ta", "", "tam", Tamil},
		{"id", "", "ind", Indonesian},
		{"te", "", "tel", Telugu},
		{"mk", "", "mkd", Macedonian},
		{"sl", "", "slv", Slovenian},
		{"ml", "", "mal", Malayalam},
		{"kn", "", "kan", Kannada},
		{"sq", "", "sqi", Albanian},
		{"af", "", "afr", Afrikaans},
		{"mr", "", "mar", Marathi},
		{"tl", "", "tgl", Tagalog},
		{"ur", "", "urd", Urdu},
		{"rm", "", "roh", Romansh},
		{"mn", "", "mon", Mongolian},
		{"ka", "", "kat", Georgian},
	},
}

// alternateIsoCodes maps codes outside ISO 639-1 and 639-2/T to languages.
var alternateIsoCodes = map[Kind]map[string]Language{
	Series: {"cze": Czech, "dut": Dutch, "mac": Macedonian, "rum": Romanian, "yue": Chinese, "zhtw": Chinese},
	Movie:  {"cn": Chinese},
}

// iso639BT maps ISO 639-2/B codes to their 639-2/T equivalents.
var iso639BT = map[Kind]map[string]string{
	Series: {
		"alb": "sqi", "arm": "hye", "baq": "eus", "bur": "mya", "chi": "zho", "cze": "ces", "dut": "nld",
		"fre": "fra", "geo": "kat", "ger": "deu", "gre": "ell", "ice": "isl", "mac": "mkd", "mao": "mri",
		"may": "msa", "per": "fas", "rum": "ron", "slo": "slk", "tib": "bod", "wel": "cym",
	},
	Movie: {
		"alb": "sqi", "arm": "hye", "baq": "eus", "bur": "mya", "chi": "zho", "cze": "ces", "dut": "nld",
		"fre": "fra", "geo": "kat", "ger": "deu", "gre": "ell", "gsw": "deu", "ice": "isl", "mac": "mkd",
		"mao": "mri", "may": "msa", "per": "fas", "rum": "ron", "slo": "slk", "tib": "bod", "wel": "cym",
		"khk": "mon", "mvf": "mon",
	},
}

// FindIsoLanguage looks up an ISO 639-1 code, optionally with a country
// ("pt-BR"), or an ISO 639-2 code.
func FindIsoLanguage(code string, kind Kind) (IsoLanguage, bool) {
	parts := strings.Split(code, "-")
	lang := strings.ToLower(parts[0])
	alternate, isAlternate := alternateIsoCodes[kind][code]
	// Radarr checks alternate codes first, Sonarr after two-letter codes.
	if isAlternate && (kind == Movie || len(lang) != 2) {
		return isoLanguageOf(alternate, kind)
	}
	all := isoLanguages[kind]
	switch len(lang) {
	case 2:
		var candidates []IsoLanguage
		for _, l := range all {
			if l.TwoLetterCode == lang {
				candidates = append(candidates, l)
			}
		}
		if len(parts) > 1 {
			country := strings.ToLower(parts[1])
			var byCountry, noCountry []IsoLanguage
			for _, l := range candidates {
				if l.CountryCode == country {
					byCountry = append(byCountry, l)
				}
				if l.CountryCode == "" {
					noCountry = append(noCountry, l)
				}
			}
			candidates = noCountry
			if len(byCountry) > 0 {
				candidates = byCountry
			}
		}
		if len(candidates) > 0 {
			return candidates[0], true
		}
	case 3:
		if mapped, ok := iso639BT[kind][lang]; ok {
			lang = mapped
		}
		for _, l := range all {
			if l.ThreeLetterCode == lang {
				return l, true
			}
		}
	}
	return IsoLanguage{}, false
}

func isoLanguageOf(language Language, kind Kind) (IsoLanguage, bool) {
	for _, l := range isoLanguages[kind] {
		if l.Language == language {
			return l, true
		}
	}
	return IsoLanguage{}, false
}
