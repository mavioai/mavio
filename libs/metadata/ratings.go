package metadata

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strconv"
	"strings"
	"sync"
)

// ratingFiles are the rating systems of Jellyfin, one per country, and
// "0-prefer" with the ratings preferred where countries disagree.
//
//go:embed ratings/*.json
var ratingFiles embed.FS

// ratingSystem maps the lower-cased rating strings of one country to
// scores.
type ratingSystem struct {
	country string
	scores  map[string]int
}

var ratingSystems = sync.OnceValue(func() []ratingSystem {
	names, err := fs.Glob(ratingFiles, "ratings/*.json")
	if err != nil {
		panic(err)
	}
	slices.Sort(names) // "0-prefer" first, as Jellyfin loads them
	var systems []ratingSystem
	for _, name := range names {
		if path.Base(name) == "SOURCES.json" {
			continue
		}
		data, err := ratingFiles.ReadFile(name)
		if err != nil {
			panic(err)
		}
		var file struct {
			CountryCode string `json:"countryCode"`
			Ratings     []struct {
				RatingStrings []string `json:"ratingStrings"`
				RatingScore   *struct {
					Score int `json:"score"`
				} `json:"ratingScore"`
			} `json:"ratings"`
		}
		if err := json.Unmarshal(data, &file); err != nil {
			panic(fmt.Sprintf("rating system %s: %v", name, err))
		}
		s := ratingSystem{country: strings.ToLower(file.CountryCode), scores: map[string]int{}}
		for _, r := range file.Ratings {
			if r.RatingScore == nil {
				continue
			}
			for _, str := range r.RatingStrings {
				s.scores[strings.ToLower(str)] = r.RatingScore.Score
			}
		}
		systems = append(systems, s)
	}
	return systems
})

func ratingSystemOf(country string) *ratingSystem {
	country = strings.ToLower(country)
	for i, s := range ratingSystems() {
		if s.country == country {
			return &ratingSystems()[i]
		}
	}
	return nil
}

// unratedValues mark content as unrated.
var unratedValues = []string{"n/a", "unrated", "not rated", "nr"}

func isUnrated(rating string) bool {
	return slices.Contains(unratedValues, strings.ToLower(strings.TrimSpace(rating)))
}

// RatingScore returns the score of a content rating, the minimum age it
// stands for (1000 and above for adult content), as Jellyfin's
// LocalizationManager.GetRatingScore does: in the rating system of
// country (ISO 3166-1 alpha-2) first, then in the US system and in the
// others, with "Rated" prefixes and country prefixes ("DE:", "DE-")
// removed, plain ages ("16", "18+", "-12") taken as they are, and lists
// separated by "/" taken by their first entry that resolves. It reports
// false for unrated content and ratings it does not know. Sub-scores,
// which Jellyfin keeps for ratings such as "TV-PG-V", are not kept.
func RatingScore(rating, country string) (int, bool) {
	if strings.TrimSpace(rating) == "" || isUnrated(rating) {
		return 0, false
	}
	// Some ratings contain a "/" themselves (e.g. "M/12" in Portugal), so
	// the value as a whole comes first.
	if score, ok := singleRatingScore(rating, country); ok {
		return score, true
	}
	values := strings.Split(rating, "/")
	if len(values) == 1 {
		return 0, false
	}
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" || isUnrated(v) {
			continue
		}
		if score, ok := singleRatingScore(v, country); ok {
			return score, true
		}
	}
	return 0, false
}

// singleRatingScore resolves one rating value.
func singleRatingScore(rating, country string) (int, bool) {
	if score, ok := ageScore(rating); ok {
		return score, true
	}
	// Some write "Rated R".
	for _, prefix := range []string{"Rated :", "Rated:", "Rated "} {
		rating = replaceFold(rating, prefix, "")
	}
	rating = strings.TrimSpace(rating)
	key := strings.ToLower(rating)

	if s := ratingSystemOf(country); s != nil {
		if score, ok := s.scores[key]; ok {
			return score, true
		}
		// A prefix of the country itself, as TMDB writes "IT-VM14".
		if c := strings.ToLower(country); len(key) > len(c) && strings.HasPrefix(key, c) && (key[len(c)] == '-' || key[len(c)] == ':') {
			if score, ok := s.scores[strings.TrimSpace(key[len(c)+1:])]; ok {
				return score, true
			}
		}
	}
	if s := ratingSystemOf("us"); s != nil {
		if score, ok := s.scores[key]; ok {
			return score, true
		}
	}
	for _, s := range ratingSystems() {
		if score, ok := s.scores[key]; ok {
			return score, true
		}
	}
	// A country prefix, as in "US:PG-13", "Germany: FSK-18" or "DE-FSK-18";
	// a "/" marks a list instead.
	if !strings.Contains(rating, "/") {
		for _, sep := range []string{":", "-"} {
			if score, ok, found := ratingScoreBySeparator(rating, sep, country); found {
				return score, ok
			}
		}
	}
	return 0, false
}

// ratingScoreBySeparator resolves a rating with a country prefix before
// sep. It reports found when the rating has such a prefix, even if the
// rating is unknown to the country's system. A prefix that is no known
// country is dropped and the rest looked up as in country.
func ratingScoreBySeparator(rating, sep, country string) (score int, ok, found bool) {
	countryPart, ratingPart, cut := strings.Cut(rating, sep)
	if !cut {
		return 0, false, false
	}
	countryPart, ratingPart = strings.TrimSpace(countryPart), strings.TrimSpace(ratingPart)
	if ratingPart == "" {
		return 0, false, false
	}
	if s := ratingSystemOf(countryPart); s != nil {
		if score, ok := s.scores[strings.ToLower(ratingPart)]; ok {
			return score, true, true
		}
		// Not a rating of the country: perhaps a plain age.
		if score, ok := ageScore(ratingPart); ok {
			return score, true, true
		}
		return 0, false, true
	}
	score, ok = RatingScore(ratingPart, country)
	return score, ok, true
}

// ageScore parses a plain age with an optional trailing "+" ("18+") or a
// leading minus or en dash ("-12" and "–12" are French for "not under
// 12"), never a negative score.
func ageScore(rating string) (int, bool) {
	s := strings.TrimSpace(rating)
	s = strings.TrimLeft(s, "-–")
	s = strings.TrimRight(s, "+")
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 || s == "" || s[0] == '+' || s[0] == '-' {
		return 0, false
	}
	return n, true
}

// replaceFold replaces old in s ignoring case.
func replaceFold(s, old, repl string) string {
	if i := strings.Index(strings.ToLower(s), strings.ToLower(old)); i >= 0 {
		return s[:i] + repl + s[i+len(old):]
	}
	return s
}
