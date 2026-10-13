package provider

import (
	"strings"
	"sync"

	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
)

// Alpha2 returns a language's ISO 639-1 code, such as "fr"; "" when it
// has none.
func Alpha2(tag string) string {
	b, _ := language.Make(tag).Base()
	if s := b.String(); len(s) == 2 {
		return s
	}
	return ""
}

// Alpha3 returns a language's ISO 639-2/T (and 639-3) code, such as
// "fra", as babelfish's alpha3.
func Alpha3(tag string) string {
	b, _ := language.Make(tag).Base()
	return b.ISO3()
}

// Alpha3B returns a language's ISO 639-2/B code, such as "fre", as
// babelfish's alpha3b.
func Alpha3B(tag string) string {
	a := Alpha3(tag)
	if b, ok := alpha3B[a]; ok {
		return b
	}
	return a
}

// alpha3B are the ISO 639-2 bibliographic codes that differ from the
// terminological ones.
var alpha3B = map[string]string{
	"sqi": "alb", "hye": "arm", "eus": "baq", "mya": "bur", "zho": "chi", "ces": "cze", "nld": "dut", "fra": "fre",
	"kat": "geo", "deu": "ger", "ell": "gre", "isl": "ice", "mkd": "mac", "mri": "mao", "msa": "may", "fas": "per",
	"ron": "rum", "slk": "slo", "bod": "tib", "cym": "wel",
}

// FromCode returns the BCP 47 tag of an ISO 639-1, 639-2/T, 639-2/B or
// 639-3 code; "" when the code is unknown.
func FromCode(code string) string {
	code = strings.ToLower(strings.TrimSpace(code))
	for t, b := range alpha3B {
		if b == code {
			code = t
			break
		}
	}
	b, err := language.ParseBase(code)
	if err != nil {
		return ""
	}
	return b.String()
}

// FromName returns the BCP 47 tag of a language's English name, as
// babelfish's fromname: "French" is "fr"; "" when the name is unknown.
func FromName(name string) string {
	return names()[strings.ToLower(strings.TrimSpace(name))]
}

// names are the languages with ISO 639-1 codes by their English names.
var names = sync.OnceValue(func() map[string]string {
	out := map[string]string{}
	namer := display.English.Languages()
	for a := 'a'; a <= 'z'; a++ {
		for b := 'a'; b <= 'z'; b++ {
			base, err := language.ParseBase(string([]rune{a, b}))
			if err != nil || base.String() != string([]rune{a, b}) {
				continue
			}
			if n := namer.Name(base); n != "" {
				out[strings.ToLower(n)] = base.String()
			}
		}
	}
	return out
})
