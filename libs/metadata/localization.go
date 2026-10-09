package metadata

import (
	"bufio"
	"bytes"
	"cmp"
	"embed"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"
)

// localizationFiles are Jellyfin's lists of countries and ISO 639-2
// languages.
//
//go:embed localization/countries.json localization/iso6392.txt
var localizationFiles embed.FS

// Country is an ISO 3166-1 country.
type Country struct {
	// Code is alpha-2, such as "DE"; ThreeLetterCode alpha-3, "DEU".
	Code, ThreeLetterCode string
	// Name is English.
	Name string
}

var countries = sync.OnceValue(func() []Country {
	data, err := localizationFiles.ReadFile("localization/countries.json")
	if err != nil {
		panic(err)
	}
	var file []struct {
		DisplayName              string
		TwoLetterISORegionName   string
		ThreeLetterISORegionName string
	}
	if err := json.Unmarshal(data, &file); err != nil {
		panic(fmt.Sprintf("countries: %v", err))
	}
	out := make([]Country, len(file))
	for i, c := range file {
		out[i] = Country{Code: c.TwoLetterISORegionName, ThreeLetterCode: c.ThreeLetterISORegionName, Name: c.DisplayName}
	}
	return out
})

// Countries lists the countries, by name.
func Countries() []Country { return slices.Clone(countries()) }

// Language is an ISO 639-2 language, as Jellyfin's cultures list them.
type Language struct {
	// Name is the English name, or a language tag such as "zh-cn" for
	// regional variants.
	Name string
	// DisplayName is the English name, which may list several, such as
	// "Dutch; Flemish".
	DisplayName string
	// TwoLetterCode is ISO 639-1 or a tag such as "zh-cn"; empty for
	// languages without one.
	TwoLetterCode string
	// ThreeLetterCodes are ISO 639-2/T, then 639-2/B where it differs.
	ThreeLetterCodes []string
}

// languages and the ISO 639-2/B to /T map, from iso6392.txt: one language
// per line as "T|B|two-letter|English name|French name".
var languages = sync.OnceValues(func() ([]Language, map[string]string) {
	data, err := localizationFiles.ReadFile("localization/iso6392.txt")
	if err != nil {
		panic(err)
	}
	var out []Language
	bToT := map[string]string{}
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line := sc.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) != 5 {
			panic(fmt.Sprintf("languages: bad line %q", line))
		}
		l := Language{Name: parts[3], DisplayName: parts[3], TwoLetterCode: strings.TrimSpace(parts[2])}
		if l.DisplayName == "" {
			continue
		}
		if strings.Contains(l.TwoLetterCode, "-") {
			l.Name = l.TwoLetterCode
		}
		l.ThreeLetterCodes = []string{parts[0]}
		if parts[1] != "" {
			l.ThreeLetterCodes = append(l.ThreeLetterCodes, parts[1])
			if _, ok := bToT[parts[1]]; !ok {
				bToT[parts[1]] = parts[0]
			}
		}
		out = append(out, l)
	}
	return out, bToT
})

// Languages lists the ISO 639-2 languages and their regional variants,
// by ISO 639-2/T code.
func Languages() []Language {
	l, _ := languages()
	return slices.Clone(l)
}

// FindLanguage finds a language by its English name, any of its codes or
// its tag, regardless of case.
func FindLanguage(s string) (Language, bool) {
	if s == "" {
		return Language{}, false
	}
	list, _ := languages()
	for _, l := range list {
		if strings.EqualFold(s, l.DisplayName) || strings.EqualFold(s, l.Name) || strings.EqualFold(s, l.TwoLetterCode) ||
			slices.ContainsFunc(l.ThreeLetterCodes, func(c string) bool { return strings.EqualFold(c, s) }) {
			return l, true
		}
	}
	return Language{}, false
}

// LanguageDisplayName returns a language's English name, the first of
// several: "Dutch" for "Dutch; Flemish".
func LanguageDisplayName(s string) (string, bool) {
	l, ok := FindLanguage(s)
	if !ok {
		return "", false
	}
	name, _, _ := strings.Cut(l.DisplayName, ";")
	name, _, _ = strings.Cut(name, ",")
	return strings.TrimSpace(name), true
}

// ISO6392TFromB returns the ISO 639-2/T code of a bibliographic code that
// differs from it, such as "deu" for "ger".
func ISO6392TFromB(b string) (string, bool) {
	_, bToT := languages()
	t, ok := bToT[strings.ToLower(b)]
	return t, ok
}

// ParentalRating is a rating administrators can choose as a limit.
type ParentalRating struct {
	Name string
	// Score is nil for unrated.
	Score *int
}

// ParentalRatings lists the ratings of a country's rating system, with
// the common ones Jellyfin adds so that every limit can be chosen, by
// score, unrated first.
func ParentalRatings(country string) []ParentalRating {
	var out []ParentalRating
	if s := ratingSystemOf(country); s != nil {
		out = slices.Clone(s.list)
	}
	has := func(f func(r ParentalRating) bool) bool { return slices.ContainsFunc(out, f) }
	score := func(n int) func(ParentalRating) bool {
		return func(r ParentalRating) bool { return r.Score != nil && *r.Score == n }
	}
	add := func(name string, n int) {
		out = append(out, ParentalRating{Name: name, Score: &n})
	}
	if !has(func(r ParentalRating) bool { return r.Score == nil }) {
		out = append(out, ParentalRating{Name: "Unrated"})
	}
	for _, c := range []struct {
		name  string
		score int
	}{{"Approved", 0}, {"10", 10}, {"13", 13}, {"14", 14}} {
		if !has(score(c.score)) {
			add(c.name, c.score)
		}
	}
	if !has(func(r ParentalRating) bool { return r.Score != nil && *r.Score >= 21 }) {
		add("21", 21)
	}
	if !has(score(1000)) {
		add("XXX", 1000)
	}
	if !has(score(1001)) {
		add("Banned", 1001)
	}
	slices.SortStableFunc(out, func(a, b ParentalRating) int {
		switch {
		case a.Score == nil && b.Score == nil:
			return 0
		case a.Score == nil:
			return -1
		case b.Score == nil:
			return 1
		}
		return cmp.Compare(*a.Score, *b.Score)
	})
	return out
}
