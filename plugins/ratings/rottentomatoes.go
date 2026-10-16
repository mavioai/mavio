package main

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"google.golang.org/protobuf/proto"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

// The public keys of the Algolia search Rotten Tomatoes' site uses, which
// Seerr uses too.
const (
	algoliaAppID  = "79FRDP12PN"
	algoliaAPIKey = "175588f6e5f8319b27702e4cc4013561"
)

// rtHit is a title Rotten Tomatoes' search finds.
type rtHit struct {
	Title          string   `json:"title"`
	Titles         []string `json:"titles"`
	AKA            []string `json:"aka"`
	ReleaseYear    int      `json:"releaseYear"`
	Vanity         string   `json:"vanity"`
	RottenTomatoes *struct {
		CriticsScore   *float64 `json:"criticsScore"`
		AudienceScore  float64  `json:"audienceScore"`
		CertifiedFresh bool     `json:"certifiedFresh"`
	} `json:"rottenTomatoes"`
}

// The tunables of Seerr's matching of search results to a title.
const (
	inexactTitleFactor   = 0.25
	alternateTitleFactor = 0.8
	perYearPenalty       = 0.4
	minimumScore         = 0.175
)

var leadingThe = regexp.MustCompile(`(?i)\bthe\b ?`)

// rottenTomatoes finds a movie or series by its name and year with Rotten
// Tomatoes' search, as Seerr does, and tells its critics' and audience
// scores; none when it is not found or not scored.
func (p *plugin) rottenTomatoes(ctx context.Context, name string, year int, series bool) ([]*pluginv1.Rating, error) {
	kind, query, path := "movie", leadingThe.ReplaceAllString(name, ""), "/m/"
	if series {
		kind, query, path = "tv", name, "/tv/"
	}
	filters := url.QueryEscape(`isEmsSearchable=1 AND type:"` + kind + `"`)
	body, err := json.Marshal(map[string]any{"requests": []map[string]string{{
		"indexName": "content_rt", "query": query, "params": "filters=" + filters + "&hitsPerPage=20",
	}}})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.algolia+"/queries", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Algolia-Application-Id", algoliaAppID)
	req.Header.Set("X-Algolia-Api-Key", algoliaAPIKey)
	var res struct {
		Results []struct {
			Index string  `json:"index"`
			Hits  []rtHit `json:"hits"`
		} `json:"results"`
	}
	if err := p.do(req, "Rotten Tomatoes", &res); err != nil {
		return nil, err
	}
	var hits []rtHit
	for _, r := range res.Results {
		if r.Index == "content_rt" {
			hits = r.Hits
		}
	}
	hit, ok := bestHit(hits, name, year)
	if !ok || hit.RottenTomatoes == nil {
		return nil, nil
	}
	page := "https://www.rottentomatoes.com" + path + hit.Vanity
	var out []*pluginv1.Rating
	if s := hit.RottenTomatoes.CriticsScore; s != nil {
		out = append(out, pluginv1.Rating_builder{
			Source: pluginv1.RatingSource_RATING_SOURCE_ROTTEN_TOMATOES_CRITICS.Enum(), Score: proto.Float64(*s), Url: proto.String(page),
			Certified: proto.Bool(!series && hit.RottenTomatoes.CertifiedFresh),
		}.Build())
	}
	if s := hit.RottenTomatoes.AudienceScore; s > 0 {
		out = append(out, pluginv1.Rating_builder{
			Source: pluginv1.RatingSource_RATING_SOURCE_ROTTEN_TOMATOES_AUDIENCE.Enum(), Score: proto.Float64(s), Url: proto.String(page),
		}.Build())
	}
	return out, nil
}

// bestHit is the search result that best matches the name and year, by
// Seerr's scores: an exact title, the main one before others, the year
// within two of the title's, and scores before none.
func bestHit(hits []rtHit, name string, year int) (rtHit, bool) {
	type scored struct {
		hit   rtHit
		score float64
	}
	var out []scored
	for _, h := range hits {
		if s := hitScore(h, name, year); s > minimumScore {
			out = append(out, scored{h, s})
		}
	}
	if len(out) == 0 {
		return rtHit{}, false
	}
	return slices.MaxFunc(out, func(a, b scored) int { return cmp.Compare(a.score, b.score) }).hit, true
}

func hitScore(h rtHit, name string, year int) float64 {
	want := normTitle(name)
	title := 0.0
	for i, t := range slices.Concat([]string{h.Title}, h.AKA, h.Titles) {
		s := similarity(normTitle(t), want)
		if i > 0 {
			s *= alternateTitleFactor
		}
		title = max(title, s)
	}
	years := 1.0
	if year > 0 {
		d := h.ReleaseYear - year
		years = max(0, 1-float64(max(d, -d))*perYearPenalty)
	}
	extra := 1.0
	if h.RottenTomatoes == nil {
		extra = 0.5
	}
	return title * years * extra
}

// normTitle lowercases a title and keeps its letters, digits and spaces.
func normTitle(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case unicode.IsLetter(r) || unicode.IsNumber(r):
			return unicode.ToLower(r)
		case r == ' ':
			return r
		}
		return -1
	}, s)
}

// similarity is 1 for equal titles, a quarter of their Jaro similarity
// otherwise.
func similarity(a, b string) float64 {
	if a == b {
		return 1
	}
	return jaro(a, b) * inexactTitleFactor
}

// jaro is the Jaro similarity of two strings, 0–1.
func jaro(a, b string) float64 {
	ra, rb := []rune(a), []rune(b)
	if len(ra) == 0 || len(rb) == 0 {
		if len(ra) == len(rb) {
			return 1
		}
		return 0
	}
	window := max(0, max(len(ra), len(rb))/2-1)
	ma, mb := make([]bool, len(ra)), make([]bool, len(rb))
	matches := 0
	for i, r := range ra {
		for j := max(0, i-window); j < min(len(rb), i+window+1); j++ {
			if !mb[j] && rb[j] == r {
				ma[i], mb[j] = true, true
				matches++
				break
			}
		}
	}
	if matches == 0 {
		return 0
	}
	transpositions, k := 0, 0
	for i, r := range ra {
		if !ma[i] {
			continue
		}
		for !mb[k] {
			k++
		}
		if rb[k] != r {
			transpositions++
		}
		k++
	}
	m := float64(matches)
	return (m/float64(len(ra)) + m/float64(len(rb)) + (m-float64(transpositions)/2)/m) / 3
}

// do sends a request and decodes its JSON answer into out.
func (p *plugin) do(req *http.Request, service string, out any) error {
	resp, err := p.http.Do(req)
	if err != nil {
		return fmt.Errorf("%s: %w", service, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return errNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: %s", service, resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("%s: %w", service, err)
	}
	return nil
}
