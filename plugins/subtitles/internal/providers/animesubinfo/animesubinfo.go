// Package animesubinfo is the provider of AnimeSub.info, ported from
// Bazarr's animesubinfo: Polish subtitles of anime series and movies,
// searched by their English, original and Polish titles.
package animesubinfo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/mavioai/mavio/plugins/subtitles/internal/archive"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/site"
	"github.com/mavioai/mavio/plugins/subtitles/internal/titles"
)

// Name is the provider's name.
const Name = "animesubinfo"

// charset is the site's, which its pages do not declare.
const charset = "text/html; charset=iso-8859-2"

// Definition returns the provider's definition.
func Definition() provider.Definition {
	return provider.Definition{
		Name: Name, DisplayName: "AnimeSub.info",
		Languages: []string{"pl"},
		Movies:    true, Episodes: true,
		New: New,
	}
}

// New makes the provider; it has no settings.
func New(env provider.Env, _ json.RawMessage) (provider.Provider, error) {
	s := site.New(env.HTTP)
	s.Header.Set("Accept-Charset", "ISO-8859-2,utf-8;q=0.7,*;q=0.3")
	return &Provider{env: env, s: s, base: "http://animesub.info"}, nil
}

// Provider is AnimeSub.info.
type Provider struct {
	env  provider.Env
	s    *site.Session
	base string
}

// SetBaseURL points the provider at another server, for tests.
func (p *Provider) SetBaseURL(u *url.URL) { p.base = strings.TrimSuffix(u.String(), "/") }

// search is a search of the site: the kind of title ("en", "org" or "pl")
// and the title.
type search struct{ kind, title string }

// searches are those Bazarr's list_subtitles makes: the video's title in
// English, then its first two alternative titles as original and Polish
// titles; an episode's with its number and absolute number.
func searches(v *provider.Video) []search {
	var out []search
	add := func(title string, eng bool) {
		if eng {
			out = append(out, search{"en", title})
		} else {
			out = append(out, search{"org", title}, search{"pl", title})
		}
	}
	alternatives := v.AlternativeTitles
	main := v.Title
	if v.Episode {
		alternatives, main = v.AlternativeSeries, v.Series
	}
	for i, title := range append([]string{main}, alternatives[:min(2, len(alternatives))]...) {
		eng := i == 0
		if !v.Episode {
			add(title, eng)
			continue
		}
		if v.Number != 0 {
			add(fmt.Sprintf("%s ep%02d", title, v.Number), eng)
		}
		if v.AbsoluteNumber != 0 {
			add(fmt.Sprintf("%s ep%d", title, v.AbsoluteNumber), eng)
		}
		if v.Number == 0 && v.AbsoluteNumber == 0 {
			add(title, eng)
		}
	}
	return out
}

// Search implements provider.Provider.
func (p *Provider) Search(ctx context.Context, v *provider.Video, langs []provider.Language) ([]provider.Subtitle, error) {
	if len(langs) > 0 && !slices.ContainsFunc(langs, func(l provider.Language) bool { return l.Tag == "pl" }) {
		return nil, nil
	}
	seen := map[string]bool{}
	var out []provider.Subtitle
	for _, s := range searches(v) {
		r, err := p.s.Get(ctx, p.base+"/szukaj.php", url.Values{"szukane": {s.title}, "pTitle": {s.kind}, "pSortuj": {"pobrn"}})
		if err != nil {
			return nil, err
		}
		entries, err := parse(r.Body)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if seen[e.id] {
				continue
			}
			seen[e.id] = true
			out = append(out, e.subtitle(v, p.base))
		}
	}
	return out, nil
}

// entry is a subtitle of the site's results.
type entry struct {
	id, hash                     string
	titleOrg, titleEng, titleAlt string
	author, format, size         string
	downloads                    int
	description                  string
}

// parse reads the site's results, as Bazarr's _parse_search_results: each
// subtitle is a table of three rows of titles and details and a row with
// its download form and description.
func parse(body []byte) ([]entry, error) {
	if len(body) == 0 {
		return nil, nil
	}
	doc, err := site.Parse(body, charset)
	if err != nil {
		return nil, provider.Errorf(provider.ParseResponseError, "results: %w", err)
	}
	var out []entry
	for _, table := range doc.All(`table.Napisy[style*="text-align:center"]`) {
		rows := table.All("tr.KNap")
		if len(rows) < 3 {
			continue
		}
		var e entry
		cell := func(row site.Node, i int) site.Node {
			cells := row.All("td")
			if i < 0 {
				i += len(cells)
			}
			if i < 0 || i >= len(cells) {
				return site.Node{}
			}
			return cells[i]
		}
		e.titleOrg = cell(rows[0], 0).TrimmedText()
		e.format = cell(rows[0], 3).TrimmedText()
		e.titleEng = cell(rows[1], 0).TrimmedText()
		if c := cell(rows[1], 1); c.Valid() {
			if a := c.First("a"); a.Valid() {
				e.author = a.TrimmedText()
			} else {
				e.author = c.TrimmedText()
			}
			e.author = strings.TrimLeft(e.author, "~")
		}
		e.size = cell(rows[1], -1).TrimmedText()
		e.titleAlt = cell(rows[2], 0).TrimmedText()
		if fields := strings.Fields(cell(rows[2], 3).Text()); len(fields) > 0 {
			e.downloads, _ = strconv.Atoi(fields[0])
		}
		download := table.First("tr.KKom")
		form := download.First(`form[method="POST"]`)
		id, hash := form.First(`input[name="id"]`), form.First(`input[name="sh"]`)
		if !id.Valid() || !hash.Valid() {
			continue
		}
		e.id, e.hash = id.Attr("value"), hash.Attr("value")
		e.description = download.First(`td.KNap[align="left"]`).Text()
		out = append(out, e)
	}
	return out, nil
}

var (
	episodeRe = regexp.MustCompile(`(?i)(?:ep|episode)\s*(\d+)`)
	seasonRe  = regexp.MustCompile(`(?i)(?:Season|S)\s*(\d+)|(\d+)(?:nd|rd|th)\s+Season`)
	yearRe    = regexp.MustCompile(`\b(?:19|20)\d{2}\b`)
	synchroRe = regexp.MustCompile(`(?i)Synchro[:\s]+([^\n<]+)`)
	bracketRe = regexp.MustCompile(`\[([^\]]+)\]`)
	parensRe  = regexp.MustCompile(`\([^)]+\)`)
)

// episode reads the season and episode from the titles, as Bazarr's
// _parse_episode_info; 0 when none has them.
func (e *entry) episode() (season, number int) {
	for _, title := range []string{e.titleOrg, e.titleEng, e.titleAlt} {
		if title == "" {
			continue
		}
		if m := episodeRe.FindStringSubmatch(title); number == 0 && m != nil {
			number, _ = strconv.Atoi(m[1])
		}
		if m := seasonRe.FindStringSubmatch(title); season == 0 && m != nil {
			season, _ = strconv.Atoi(m[1] + m[2])
		}
	}
	return season, number
}

// releaseGroups reads the releases the subtitle was synchronized with from
// its description's "Synchro" line, as Bazarr's _parse_release_groups.
func (e *entry) releaseGroups() []string {
	m := synchroRe.FindStringSubmatch(e.description)
	if m == nil {
		return nil
	}
	text := strings.TrimSpace(m[1])
	var groups []string
	for _, b := range bracketRe.FindAllStringSubmatch(text, -1) {
		groups = append(groups, b[1])
	}
	cleaned := parensRe.ReplaceAllString(bracketRe.ReplaceAllString(text, "$1"), "")
	for part := range strings.SplitSeq(cleaned, ",") {
		if part = strings.TrimSpace(part); len([]rune(part)) > 2 {
			groups = append(groups, part)
		}
	}
	var out []string
	for _, g := range groups {
		if g = strings.TrimSpace(g); g != "" && !slices.Contains(out, g) {
			out = append(out, g)
		}
	}
	return out
}

// subtitle returns the entry as a subtitle of the video, with the matches
// of Bazarr's get_matches.
func (e *entry) subtitle(v *provider.Video, base string) provider.Subtitle {
	titles3 := []string{e.titleOrg, e.titleEng, e.titleAlt}
	contains := func(name string) bool {
		name = titles.Sanitize(name)
		if name == "" {
			return false
		}
		return slices.ContainsFunc(titles3, func(t string) bool {
			t = titles.Sanitize(t)
			return t != "" && strings.Contains(t, name)
		})
	}
	var matches []string
	name := v.Title
	if v.Episode {
		name = v.Series
	}
	if contains(name) {
		matches = append(matches, "title")
	}
	if v.Episode {
		season, number := e.episode()
		if contains(v.Series) {
			matches = append(matches, "series")
		}
		// As Bazarr's, which adds the video's type to the matches.
		matches = append(matches, "episode")
		episodeMatch := number != 0 && (number == v.Number || number == v.AbsoluteNumber)
		if v.Season != 0 && season == v.Season || season == 0 && episodeMatch {
			matches = append(matches, "season")
		}
		if v.ReleaseGroup != "" && slices.ContainsFunc(e.releaseGroups(), func(g string) bool { return strings.EqualFold(g, v.ReleaseGroup) }) {
			matches = append(matches, "release_group")
		}
	} else if v.Year != 0 {
		for _, t := range titles3 {
			if y, _ := strconv.Atoi(yearRe.FindString(t)); y == v.Year {
				matches = append(matches, "year")
				break
			}
		}
	}
	// As Bazarr's, which favors Advanced SSA subtitles so.
	if strings.Contains(e.format, "Advanced SSA") {
		matches = append(matches, "audio_codec")
	}
	return provider.Subtitle{
		ID:           url.Values{"id": {e.id}, "sh": {e.hash}}.Encode(),
		Language:     provider.Language{Tag: "pl"},
		ReleaseNames: []string{e.titleOrg + " - " + e.titleEng},
		Matches:      matches,
		PageLink:     base + "/",
		Uploader:     e.author,
		Downloads:    e.downloads,
		Comment:      strings.TrimSpace(e.description),
	}
}

// Download implements provider.Provider: the subtitle file, or the first
// of a zip archive.
func (p *Provider) Download(ctx context.Context, _ *provider.Video, id string) (provider.File, error) {
	q, err := url.ParseQuery(id)
	if err != nil || q.Get("id") == "" {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "malformed subtitle ID %q", id)
	}
	r, err := p.s.PostForm(ctx, p.base+"/sciagnij.php", url.Values{"id": {q.Get("id")}, "sh": {q.Get("sh")}, "single_file": {"Pobierz napisy"}})
	if err != nil {
		return provider.File{}, err
	}
	if len(r.Body) < 50 {
		return provider.File{}, provider.Errorf(provider.ProviderError, "the downloaded file is too small")
	}
	lang := provider.Language{Tag: "pl"}
	if !bytes.HasPrefix(r.Body, []byte("PK")) {
		return provider.File{Data: archive.FixLineEndings(r.Body), Format: archive.Detect(r.Body), Language: lang}, nil
	}
	a, err := archive.Open(r.Body)
	if err != nil {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "zip: %w", err)
	}
	for _, name := range a.Names() {
		ext := strings.ToLower(path.Ext(name))
		if !slices.Contains([]string{".srt", ".ass", ".ssa", ".sub"}, ext) {
			continue
		}
		data, err := a.Read(name)
		if err != nil {
			return provider.File{}, provider.Errorf(provider.BadSubtitle, "%s: %w", name, err)
		}
		format := strings.TrimPrefix(ext, ".")
		if format == "sub" {
			format = archive.Detect(data)
		}
		return provider.File{Data: archive.FixLineEndings(data), Format: format, Language: lang}, nil
	}
	return provider.File{}, provider.Errorf(provider.BadSubtitle, "no subtitle file found in ZIP archive")
}
