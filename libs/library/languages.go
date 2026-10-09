package library

import (
	"strings"

	"golang.org/x/text/language"
	"golang.org/x/text/language/display"

	"github.com/mavioai/mavio/libs/naming"
	"github.com/mavioai/mavio/libs/subtitle"
)

// bibliographic are the ISO 639-2/B codes that differ from the
// terminological ones x/text knows; streams carry the bibliographic code.
var bibliographic = map[string]string{
	"sqi": "alb", "hye": "arm", "eus": "baq", "mya": "bur", "zho": "chi", "ces": "cze", "nld": "dut",
	"fra": "fre", "kat": "geo", "deu": "ger", "ell": "gre", "isl": "ice", "mkd": "mac", "mri": "mao",
	"msa": "may", "fas": "per", "ron": "rum", "slk": "slo", "bod": "tib", "cym": "wel",
}

// terminological maps the bibliographic codes back.
var terminological = func() map[string]string {
	m := make(map[string]string, len(bibliographic))
	for t, b := range bibliographic {
		m[b] = t
	}
	return m
}()

// languageFinder recognizes the language of a subtitle or audio file
// name token: ISO 639-1 and 639-2 codes, language tags with a region
// ("pt-BR"), English language names ("French"), and the many ways file
// names write Chinese variants ("chs", "cht", "big5", "简体", "繁體", …),
// which become "zh-Hans", "zh-Hant" or "chi".
type languageFinder struct {
	// names maps lower-case English language names to their tags.
	names map[string]language.Base
}

// newLanguageFinder returns a finder knowing the names of the languages
// with an ISO 639-1 code.
func newLanguageFinder() *languageFinder {
	f := &languageFinder{names: map[string]language.Base{}}
	for a := 'a'; a <= 'z'; a++ {
		for b := 'a'; b <= 'z'; b++ {
			base, err := language.ParseBase(string([]rune{a, b}))
			if err != nil {
				continue
			}
			if name := display.English.Languages().Name(base); name != "" {
				f.names[strings.ToLower(name)] = base
			}
		}
	}
	return f
}

// FindLanguage implements naming.LanguageFinder.
func (f *languageFinder) FindLanguage(token string) (naming.Language, bool) {
	token = strings.TrimSpace(token)
	if token == "" {
		return naming.Language{}, false
	}
	switch subtitle.NormalizeLanguage(token) {
	case "zh-Hans":
		return naming.Language{Name: "zh-Hans", ThreeLetter: "chi"}, true
	case "zh-Hant":
		return naming.Language{Name: "zh-Hant", ThreeLetter: "chi"}, true
	case "zh":
		return naming.Language{Name: "zh", ThreeLetter: "chi"}, true
	}
	lower := strings.ToLower(token)
	if base, ok := f.names[lower]; ok {
		return languageOf(base, ""), true
	}
	// Only codes: two or three letters, optionally with a region.
	code, region, _ := strings.Cut(lower, "-")
	if len(code) < 2 || len(code) > 3 || strings.Trim(code, "abcdefghijklmnopqrstuvwxyz") != "" {
		return naming.Language{}, false
	}
	if b, ok := terminological[code]; ok {
		code = b
	}
	base, err := language.ParseBase(code)
	// Three-letter codes are taken for the major languages only, those
	// with a two-letter code too, so that flags such as "sdh" (a Kurdish
	// language in ISO 639-3) stay flags.
	if err != nil || len(code) == 3 && len(base.String()) != 2 {
		return naming.Language{}, false
	}
	if region != "" {
		r, err := language.ParseRegion(region)
		if err != nil {
			return naming.Language{}, false
		}
		region = r.String()
	}
	return languageOf(base, region), true
}

// languageOf describes a language with its bibliographic three-letter code.
func languageOf(base language.Base, region string) naming.Language {
	three := base.ISO3()
	if b, ok := bibliographic[three]; ok {
		three = b
	}
	name := base.String()
	if region != "" {
		name += "-" + region
	}
	return naming.Language{Name: name, ThreeLetter: three}
}
