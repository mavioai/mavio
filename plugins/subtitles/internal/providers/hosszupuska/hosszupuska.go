// Package hosszupuska is the provider of Hosszupuska, ported from Bazarr's
// hosszupuska: Hungarian and English subtitles of episodes, found by the
// site's search of a series' season and episode and served in archives.
package hosszupuska

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/net/html"

	"github.com/mavioai/mavio/plugins/subtitles/internal/archive"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/site"
	"github.com/mavioai/mavio/plugins/subtitles/internal/titles"
)

// Name is the provider's name.
const Name = "hosszupuska"

// Definition returns the provider's definition.
func Definition() provider.Definition {
	return provider.Definition{
		Name: Name, DisplayName: "Hosszupuska",
		Languages: []string{"hu", "en"},
		Episodes:  true,
		New:       New,
	}
}

// New makes the provider; it has no settings.
func New(env provider.Env, _ json.RawMessage) (provider.Provider, error) {
	s := site.New(env.HTTP)
	s.Header.Set("User-Agent", "Sub-Zero/2")
	return &Provider{env: env, s: s, server: "http://hosszupuskasub.com/"}, nil
}

// Provider is Hosszupuska.
type Provider struct {
	env    provider.Env
	s      *site.Session
	server string
}

// SetBaseURL points the provider at another server, for tests.
func (p *Provider) SetBaseURL(u *url.URL) { p.server = strings.TrimSuffix(u.String(), "/") + "/" }

const (
	// rowMark and tableMark tell the tables and rows of results, as
	// Bazarr's.
	rowMark   = "this.style.backgroundImage='url(css/over2.jpg)"
	tableMark = "css/infooldal.png"
)

var (
	englishName = regexp.MustCompile(`s(\d{1,2})e(\d{1,2})`)
	yearRe      = regexp.MustCompile(`\((\d{4})\)`)
	seasonRe    = regexp.MustCompile(`s(\d{1,2})`)
	episodeRe   = regexp.MustCompile(`e(\d{1,2})`)
	// inconsistent are the series the site names otherwise, as Bazarr's
	// fix_inconsistent_naming.
	inconsistent = strings.NewReplacer("Stargate Origins", "Stargate: Origins",
		"Marvel's Agents of S.H.I.E.L.D.", "Marvels+Agents+of+S.H.I.E.L.D", "Mayans M.C.", "Mayans MC")
	// equivalentGroups are subliminal's equivalent release groups.
	equivalentGroups = [][]string{{"FRAMESTOR", "W4NK3R", "BHDSTUDIO"}, {"LOL", "DIMENSION"}, {"ASAP", "IMMERSE", "FLEET"}, {"AVS", "SVA"}}
	bracketed        = regexp.MustCompile(`\[[\w|.]+\]`)
)

// Search implements provider.Provider. As Bazarr's, only the series'
// first title is searched.
func (p *Provider) Search(ctx context.Context, v *provider.Video, langs []provider.Language) ([]provider.Subtitle, error) {
	if !v.Episode || v.Series == "" {
		return nil, nil
	}
	link := fmt.Sprintf("%ssorozatok.php?cim=%s&evad=%02d&resz=%02d&nyelvtipus=%%25&x=24&y=8",
		p.server, strings.ReplaceAll(v.Series, " ", "+"), v.Season, v.Number)
	r, err := p.s.Get(ctx, link, nil)
	if err != nil {
		return nil, err
	}
	doc, err := r.HTML()
	if err != nil {
		return nil, provider.Errorf(provider.ParseResponseError, "search: %w", err)
	}
	seen := map[*html.Node]bool{}
	var out []provider.Subtitle
	for _, table := range doc.All("table") {
		if !contains(table.Node, rowMark) || !contains(table.Node, tableMark) {
			continue
		}
		for _, row := range table.All("tr") {
			if seen[row.Node] || !contains(row.Node, rowMark) {
				continue
			}
			seen[row.Node] = true
			s, ok := p.parseRow(v, row)
			if ok && (len(langs) == 0 || slices.ContainsFunc(langs, func(l provider.Language) bool { return l.Tag == s.Language.Tag })) {
				out = append(out, s)
			}
		}
	}
	return out, nil
}

// contains reports whether an element, its attributes or its descendants
// have a text, as Bazarr's search of the element's markup.
func contains(n *html.Node, text string) bool {
	if n.Type == html.TextNode && strings.Contains(n.Data, text) {
		return true
	}
	for _, a := range n.Attr {
		if strings.Contains(a.Val, text) {
			return true
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if contains(c, text) {
			return true
		}
	}
	return false
}

// parseRow reads a row of results, as Bazarr's _loop_over_table: the
// subtitle of the episode, if the row is.
func (p *Provider) parseRow(v *provider.Video, row site.Node) (provider.Subtitle, bool) {
	cells := row.All("td")
	if len(cells) < 7 {
		return provider.Subtitle{}, false
	}
	text := cells[1].Text()
	var name, year, version string
	switch strings.Count(text, "(") {
	case 1:
		// The name follows the first episode number, up to the next.
		if m := englishName.FindAllStringIndex(text, 2); len(m) > 0 {
			end := len(text)
			if len(m) == 2 {
				end = m[1][0]
			}
			name = text[m[0][1]:end]
		}
		version = between(strings.SplitN(text, "(", 2)[1])
	case 2:
		if m := yearRe.FindStringSubmatch(strings.TrimSpace(text)); m != nil {
			year = m[1]
		}
		before := strings.SplitN(text, "(", 2)[0]
		if m := englishName.FindStringIndex(before); m != nil {
			name = before[:m[0]]
		} else {
			name = before
		}
		version = between(strings.SplitN(text, "(", 3)[2])
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return provider.Subtitle{}, false
	}
	bold := cells[1].First("b").Text()
	sm, em := seasonRe.FindStringSubmatch(bold), episodeRe.FindStringSubmatch(bold)
	if sm == nil || em == nil {
		return provider.Subtitle{}, false
	}
	season, _ := strconv.Atoi(sm[1])
	number, _ := strconv.Atoi(em[1])
	if season != v.Season || number != v.Number {
		return provider.Subtitle{}, false
	}
	var tag string
	src := cells[2].First("img").Attr("src")
	switch src[strings.LastIndex(src, "/")+1:] {
	case "1.gif":
		tag = "hu"
	case "2.gif":
		tag = "en"
	default:
		return provider.Subtitle{}, false
	}
	link := downloadLink(cells[6], p.server)
	if link == "" {
		return provider.Subtitle{}, false
	}
	var releases []string
	for r := range strings.SplitSeq(version, ",") {
		releases = append(releases, strings.TrimSpace(r))
	}
	return provider.Subtitle{
		ID:           url.Values{"u": {link}, "lang": {tag}}.Encode(),
		Language:     provider.Language{Tag: tag},
		ReleaseNames: releases,
		Matches:      matches(v, name, year, version),
		PageLink:     link,
	}, true
}

// between returns the text up to a closing parenthesis.
func between(s string) string { return strings.SplitN(s, ")", 2)[0] }

// downloadLink returns the link of the download cell: the site's own,
// rather than an ad redirector's that comes first.
func downloadLink(cell site.Node, server string) string {
	links := cell.All("a[href]")
	if len(links) == 0 {
		return ""
	}
	for _, a := range links {
		if strings.HasPrefix(a.Attr("href"), server) {
			return a.Attr("href")
		}
	}
	return links[0].Attr("href")
}

// matches are those of Bazarr's get_matches.
func matches(v *provider.Video, name, year, version string) []string {
	var out []string
	series := false
	if s := titles.Sanitize(name); s == titles.Sanitize(inconsistent.Replace(v.Series)) || s == titles.Sanitize(v.Series) {
		out = append(out, "series")
		series = true
	}
	out = append(out, "season", "episode")
	if y, _ := strconv.Atoi(year); series && v.OriginalSeries && year == "" || v.Year != 0 && v.Year == y {
		out = append(out, "year")
	}
	if v.ReleaseGroup != "" && version != "" {
		sanitized := releaseGroup(version)
		if slices.ContainsFunc(equivalents(releaseGroup(v.ReleaseGroup)), func(g string) bool { return strings.Contains(sanitized, g) }) {
			out = append(out, "release_group")
		}
	}
	return out
}

// releaseGroup sanitizes a release group, as subliminal's
// sanitize_release_group.
func releaseGroup(s string) string {
	return strings.ToUpper(strings.TrimSpace(bracketed.ReplaceAllString(s, "")))
}

// equivalents returns a release group's equivalents, as subliminal's
// get_equivalent_release_groups.
func equivalents(group string) []string {
	for _, g := range equivalentGroups {
		if slices.Contains(g, group) {
			return g
		}
	}
	return []string{group}
}

// Download implements provider.Provider: the episode's subtitle of the
// zip or RAR archive the site serves.
func (p *Provider) Download(ctx context.Context, v *provider.Video, id string) (provider.File, error) {
	q, err := url.ParseQuery(id)
	if err != nil || q.Get("u") == "" {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "malformed subtitle ID %q", id)
	}
	r, err := p.s.Get(ctx, q.Get("u"), nil)
	if err != nil {
		return provider.File{}, err
	}
	if !bytes.HasPrefix(r.Body, []byte("PK")) && !bytes.HasPrefix(r.Body, []byte("Rar!")) {
		return provider.File{}, provider.Errorf(provider.ProviderError, "unidentified archive type")
	}
	lang := provider.Language{Tag: q.Get("lang")}
	return archive.Extract(r.Body, archive.WantFor(v, lang), lang)
}
