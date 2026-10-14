// Package supersubtitles is the provider of SuperSubtitles (feliratok.eu),
// ported from Bazarr's supersubtitles: Hungarian and English subtitles of
// episodes, found through the site's series IDs, and of movies, found on
// the pages of its search.
package supersubtitles

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"

	"github.com/mavioai/mavio/plugins/subtitles/internal/archive"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/site"
)

// Name is the provider's name.
const Name = "supersubtitles"

// serverURL is the site's.
const serverURL = "https://www.feliratok.eu/"

// pause is the wait after a search, as Bazarr's multi_result_throttle.
const pause = 2 * time.Second

// Definition returns the provider's definition.
func Definition() provider.Definition {
	return provider.Definition{
		Name: Name, DisplayName: "SuperSubtitles",
		Languages: []string{"hu", "en"},
		Movies:    true, Episodes: true,
		New: New,
	}
}

// New makes the provider; it has no settings.
func New(env provider.Env, _ json.RawMessage) (provider.Provider, error) {
	s := site.NewBrowser(env.HTTP)
	// The site's answers are read whatever their status, as Bazarr does
	// but for downloads: unreadable ones are no results.
	s.Check = func(r *site.Response) error {
		if r.Status == http.StatusTooManyRequests || r.Status >= 500 {
			return site.Status(r)
		}
		return nil
	}
	p := &Provider{env: env, s: s, pause: pause}
	base, _ := url.Parse(serverURL)
	p.SetBaseURL(base)
	return p, nil
}

// Provider is SuperSubtitles.
type Provider struct {
	env   provider.Env
	s     *site.Session
	base  *url.URL
	pause time.Duration
}

// SetBaseURL points the provider at another server, for tests.
func (p *Provider) SetBaseURL(u *url.URL) {
	p.base = u
	p.s.Header.Set("Referer", u.JoinPath("index.php").String())
}

// link returns the URL of a path and query of the site, as Bazarr's
// server_url + path.
func (p *Provider) link(ref string) string { return p.base.String() + strings.TrimPrefix(ref, "/") }

// result is a subtitle the site lists, as Bazarr's
// SuperSubtitlesSubtitle.
type result struct {
	language provider.Language
	// ref is the download link's path and query.
	ref      string
	id       string
	series   string
	season   int
	episode  int
	releases []string
	year     int
	imdbID   string
	uploader string
}

// releaseInfo is the subtitle's name and releases, as Bazarr's
// release_info.
func (r *result) releaseInfo() []string {
	name := r.series
	if r.year != 0 {
		name += fmt.Sprintf(" (%d)", r.year)
	}
	if r.episode != 0 {
		name += fmt.Sprintf(" S%02dE%02d", r.season, r.episode)
	}
	return append([]string{strings.TrimSpace(name)}, r.releases...)
}

// matches are what the subtitle matches, as Bazarr's get_matches; the
// release names are guessed by the server.
func (r *result) matches(v *provider.Video) []string {
	var out []string
	add := func(ms ...string) {
		for _, m := range ms {
			if !slices.Contains(out, m) {
				out = append(out, m)
			}
		}
	}
	if v.Episode {
		if v.Series != "" && sanitize(r.series) == sanitize(v.Series) {
			add("series")
		}
		if id := v.SeriesExternalIDs["imdb"]; id != "" && r.imdbID != "" && r.imdbID == id {
			add("series_imdb_id", "series", "year")
		}
		if !slices.Contains(out, "year") && slices.Contains(out, "series") && v.OriginalSeries && r.year == 0 {
			add("year")
		}
	} else {
		if v.Title != "" && slices.ContainsFunc(append([]string{v.Title}, v.AlternativeTitles...), func(t string) bool { return sanitize(t) == sanitize(r.series) }) {
			add("title")
		}
		if id := v.ExternalIDs["imdb"]; id != "" && r.imdbID == id {
			add("imdb_id", "title", "year")
		}
		if v.Year != 0 && r.year == v.Year {
			add("year")
		}
	}
	if v.ReleaseGroup != "" && len(r.releases) > 0 {
		groups := equivalentReleaseGroups(sanitizeReleaseGroup(v.ReleaseGroup))
		for _, release := range r.releases {
			if slices.ContainsFunc(groups, func(g string) bool { return strings.Contains(sanitizeReleaseGroup(release), g) }) {
				add("release_group")
				if v.Resolution != "" && strings.Contains(strings.ToLower(release), v.Resolution) {
					add("resolution")
				}
				if v.Source != "" && strings.Contains(strings.ToLower(release), v.Source) {
					add("source")
				}
			}
			if slices.Contains(out, "release_group") && slices.Contains(out, "resolution") && slices.Contains(out, "source") {
				break
			}
		}
	}
	return out
}

func (r *result) subtitle(p *Provider, v *provider.Video) provider.Subtitle {
	return provider.Subtitle{
		ID:           r.ref,
		Language:     r.language,
		ReleaseNames: r.releaseInfo(),
		Matches:      r.matches(v),
		PageLink:     p.link(requote(r.ref)),
		Uploader:     r.uploader,
	}
}

// fromSite returns the language of the site's name of it, as Bazarr's
// get_language; false for others.
func fromSite(name string) (provider.Language, bool) {
	switch name {
	case "Magyar":
		return provider.Language{Tag: "hu"}, true
	case "Angol":
		return provider.Language{Tag: "en"}, true
	}
	return provider.Language{}, false
}

// tvNaming and movieNaming rename the titles the site names otherwise,
// as Bazarr's fix_tv_naming and fix_movie_naming.
var (
	tvNaming = strings.NewReplacer(
		"DC's Legends of Tomorrow", "Legends of Tomorrow",
		"Star Trek: The Next Generation", "Star Trek TNG",
		"Loki (aka. Marvel's Loki)", "Loki",
		"Marvel's", "",
	)
	movieNaming = strings.NewReplacer()
)

// Search implements provider.Provider, as Bazarr's list_subtitles: the
// subtitles of the video's first title whose series or title is one of
// the video's.
func (p *Provider) Search(ctx context.Context, v *provider.Video, langs []provider.Language) ([]provider.Subtitle, error) {
	titles := append([]string{v.Title}, v.AlternativeTitles...)
	naming := movieNaming
	if v.Episode {
		titles = append([]string{v.Series}, v.AlternativeSeries...)
		naming = tvNaming
	}
	normalized := make([]string, len(titles))
	for i, t := range titles {
		normalized[i] = strings.ToLower(t)
	}
	// Bazarr returns from the first title's search.
	results, err := p.query(ctx, v, titles[0], langs)
	if err != nil {
		return nil, err
	}
	var out []provider.Subtitle
	for _, r := range results {
		fixed := strings.ToLower(strings.TrimSpace(naming.Replace(r.series)))
		if slices.Contains(normalized, fixed) || slices.Contains(normalized, strings.ToLower(r.series)) {
			out = append(out, r.subtitle(p, v))
		}
	}
	if err := site.Sleep(ctx, p.pause); err != nil {
		return nil, err
	}
	return out, nil
}

// query searches a title, as Bazarr's query: an episode's series by the
// site's ID of it, a movie on the pages of the site's search.
func (p *Provider) query(ctx context.Context, v *provider.Video, title string, langs []provider.Language) ([]*result, error) {
	if !v.Episode {
		return p.processSubs(ctx, v, langs, p.link("index.php?search="+url.QueryEscape(title)+"&soriSorszam=&nyelv=&tab=film"))
	}
	series := v.Series
	seriesID, err := p.findID(ctx, requote(series), v.Year, series)
	if err != nil {
		return nil, err
	}
	if seriesID == "" {
		// Without the series' apostrophes, then by its longest word.
		modified := url.QueryEscape(strings.ReplaceAll(series, "'", ""))
		if seriesID, err = p.findID(ctx, modified, v.Year, series); err != nil {
			return nil, err
		}
		if seriesID == "" && modified != "" {
			longest := ""
			for w := range strings.SplitSeq(modified, "+") {
				if len(w) > len(longest) {
					longest = w
				}
			}
			if seriesID, err = p.findID(ctx, longest, v.Year, series); err != nil {
				return nil, err
			}
		}
		if seriesID == "" {
			return nil, nil
		}
	}
	return p.seriesSubtitles(ctx, v, seriesID, langs)
}

var (
	yearInName  = regexp.MustCompile(`\((\d{4})\)`)
	seasonName  = regexp.MustCompile(`^(.*)\s(?:-\s\d+x\d+|(\(Season\s\d+\)))?\s\((.*)\)$`)
	movieYear   = regexp.MustCompile(`\(((?:19|20)\d{2})\)`)
	imdbInLink  = regexp.MustCompile(`www\.imdb\.com/title/(.*)/"`)
	replaced    = regexp.MustCompile(`[-:().]`)
	removed     = regexp.MustCompile("['´`’]")
	spaces      = regexp.MustCompile(`\s+`)
	bracketed   = regexp.MustCompile(`\[[\w|.]+\]`)
	titleSpaces = strings.NewReplacer("�", "", "& ", "", " ", ".")
)

// findID returns the site's ID of a series, as Bazarr's find_id: the
// first of the site's names of series whose title is the series' and
// whose year is the video's, if it has one; "" for none.
func (p *Provider) findID(ctx context.Context, term string, year int, original string) (string, error) {
	r, err := p.s.Get(ctx, p.link("index.php?term="+term+"&nyelv=0&action=autoname"), nil)
	if err != nil {
		return "", err
	}
	var results []struct {
		Name string          `json:"name"`
		ID   json.RawMessage `json:"ID"`
	}
	if err := json.Unmarshal(r.Body, &results); err != nil {
		p.env.Log.ErrorContext(ctx, "unable to parse returned JSON", "provider", Name, "url", r.URL.String())
		return "", nil
	}
	want := sanitize(strings.ReplaceAll(original, "& ", ""))
	for _, res := range results {
		resultYear := ""
		if m := yearInName.FindStringSubmatch(res.Name); m != nil {
			resultYear = m[1]
		}
		resultTitle := ""
		if all := yearInName.FindAllStringIndex(res.Name, -1); all != nil {
			resultTitle = res.Name[:all[len(all)-1][0]]
		}
		for _, t := range []string{resultTitle, tvNaming.Replace(resultTitle)} {
			t = titleSpaces.Replace(strings.TrimSpace(t))
			if t == "" {
				continue
			}
			// guessit reads the title of "Title.s01e01.year" as the
			// words before the episode.
			guessTitle := sanitize(strings.ReplaceAll(t, ".", " "))
			guessYear, _ := strconv.Atoi(resultYear)
			if want == guessTitle && (year == 0 || guessYear != 0 && year == guessYear) {
				return strings.Trim(string(res.ID), `"`), nil
			}
		}
	}
	return "", nil
}

// flexible is a JSON string or number.
type flexible string

func (f *flexible) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*f = flexible(s)
		return nil
	}
	*f = flexible(bytes.Trim(data, `"`))
	return nil
}

func (f flexible) int() int {
	n, _ := strconv.Atoi(strings.TrimSpace(string(f)))
	return n
}

// listed is a subtitle of the site's list of a series' subtitles.
type listed struct {
	Language string          `json:"language"`
	Name     string          `json:"nev"`
	ID       flexible        `json:"felirat"`
	Season   flexible        `json:"evad"`
	Episode  flexible        `json:"ep"`
	Uploader string          `json:"feltolto"`
	Pack     json.RawMessage `json:"evadpakk"`
}

// seriesSubtitles returns the subtitles of an episode, or of the season's
// packs when it has none, as Bazarr's retrieve_series_subtitles.
func (p *Provider) seriesSubtitles(ctx context.Context, v *provider.Video, seriesID string, langs []provider.Language) ([]*result, error) {
	episodeSubs, _, err := p.subtitleList(ctx, v, seriesID, v.Season, v.Number)
	if err != nil {
		return nil, err
	}
	list := episodeSubs
	if len(list) == 0 {
		// The site lists the season's packs without the episode only.
		if _, list, err = p.subtitleList(ctx, v, seriesID, v.Season, 0); err != nil {
			return nil, err
		}
	}
	var out []*result
	imdbID, looked := "", false
	for _, r := range list {
		if !slices.Contains(langs, r.language) {
			continue
		}
		r.ref = "/index.php?action=letolt&felirat=" + r.id
		// The subtitles of a series' page are all of its IMDb ID.
		if !looked {
			if imdbID, err = p.findIMDbID(ctx, r.id); err != nil {
				return nil, err
			}
			looked = true
		}
		r.imdbID, r.year = imdbID, v.Year
		out = append(out, r)
	}
	return out, nil
}

// subtitleList lists a season's subtitles, or an episode's, as Bazarr's
// get_subtitle_list: the subtitles of episodes and the season's packs,
// a subtitle listed for each of its releases.
func (p *Provider) subtitleList(ctx context.Context, v *provider.Video, seriesID string, season, episode int) (episodes, packs []*result, err error) {
	u := p.link(fmt.Sprintf("index.php?action=xbmc&sid=%s&ev=%d", seriesID, season))
	if episode != 0 {
		u += "&rtol=" + strconv.Itoa(episode)
	}
	r, err := p.s.Get(ctx, u, nil)
	if err != nil {
		return nil, nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(r.Body))
	if t, err := dec.Token(); err != nil || t != json.Delim('{') {
		p.env.Log.DebugContext(ctx, "invalid results", "provider", Name, "url", u)
		return nil, nil, nil
	}
	byID := map[string]*result{}
	for dec.More() {
		if _, err := dec.Token(); err != nil {
			break
		}
		var l listed
		if err := dec.Decode(&l); err != nil {
			break
		}
		id := strconv.Itoa(l.ID.int())
		name, release := "", ""
		if m := seasonName.FindStringSubmatch(l.Name); m != nil {
			name, release = m[1], m[3]
		}
		if r, ok := byID[id]; ok {
			r.releases = append(r.releases, release)
			continue
		}
		var pack flexible
		_ = json.Unmarshal(l.Pack, &pack)
		// A pack's flag is a string, true unless empty, to Bazarr: the
		// episode is the video's but for a flag of 0 sent as a number.
		ep := v.Number
		if f := string(l.Pack); f == "0" || f == `""` || f == "false" || f == "null" {
			ep = l.Episode.int()
		}
		lang, _ := fromSite(l.Language)
		res := &result{
			language: lang, id: id, series: name, season: l.Season.int(), episode: ep,
			uploader: l.Uploader, releases: []string{release},
		}
		byID[id] = res
		if pack.int() != 0 {
			packs = append(packs, res)
		} else {
			episodes = append(episodes, res)
		}
	}
	return episodes, packs, nil
}

// findIMDbID returns the IMDb ID of a subtitle's page, as Bazarr's
// find_imdb_id; "" for none.
func (p *Provider) findIMDbID(ctx context.Context, id string) (string, error) {
	r, err := p.s.Get(ctx, p.link("index.php?tipus=adatlap&azon=a_"+id), nil)
	if err != nil {
		return "", err
	}
	doc, err := r.HTML()
	if err != nil {
		return "", nil
	}
	for _, a := range doc.All("a") {
		if s := outerHTML(a); strings.Contains(s, "imdb.com") {
			if m := imdbInLink.FindStringSubmatch(s); m != nil {
				return m[1], nil
			}
			return "", nil
		}
	}
	return "", nil
}

// processSubs reads the subtitles of a movie on the site's search page,
// as Bazarr's process_subs.
func (p *Provider) processSubs(ctx context.Context, v *provider.Video, langs []provider.Language, u string) ([]*result, error) {
	r, err := p.s.Get(ctx, u, nil)
	if err != nil {
		return nil, err
	}
	doc, err := r.HTML()
	if err != nil {
		return nil, provider.Errorf(provider.ParseResponseError, "search: %w", err)
	}
	table := doc.First("table")
	if !table.Valid() {
		p.env.Log.DebugContext(ctx, "no tables found", "provider", Name, "url", u)
		return nil, nil
	}
	var out []*result
	for i, row := range table.All("tr") {
		if i <= 1 || !strings.Contains(outerHTML(row), "vilagit") {
			continue
		}
		res, err := p.parseMovieRow(ctx, row)
		if err != nil {
			return nil, err
		}
		if res != nil && slices.Contains(langs, res.language) {
			out = append(out, res)
		}
	}
	return out, nil
}

// parseMovieRow reads a row of the search page; nil when it has no
// download link.
func (p *Provider) parseMovieRow(ctx context.Context, row site.Node) (*result, error) {
	english := row.First("div.eredeti")
	name := strippedText(english)
	year := 0
	if m := movieYear.FindStringSubmatch(name); m != nil {
		year, _ = strconv.Atoi(m[1])
	}
	name, _, _ = strings.Cut(name, " (")
	name = strings.ReplaceAll(name, "&amp;", "&")
	version := "n/a"
	if text := english.Text(); english.Valid() && strings.Contains(text, "(") {
		version = text[strings.LastIndex(text, "(")+1:]
		version, _, _ = strings.Cut(version, ")")
	}
	lang, _ := fromSite(row.First("small").Text())
	links := row.All("a")
	if len(links) == 0 {
		return nil, nil
	}
	href := links[len(links)-1].Attr("href")
	ref := strings.TrimPrefix(href, "/")
	forced := strings.Contains(row.First("div.magyar").Text(), "szinkronoshoz")
	if forced || strings.Contains(p.link(ref), "forced") {
		lang.Forced = true
	}
	id := ""
	if _, after, ok := strings.Cut(href, "felirat="); ok {
		id = after
	}
	var releases []string
	for s := range strings.SplitSeq(version, ",") {
		releases = append(releases, strings.TrimSpace(s))
	}
	uploader := ""
	if c := child(row, 7); c != nil && c.Type == html.ElementNode {
		// The last of the cell's contents.
		for item := c.FirstChild; item != nil; item = item.NextSibling {
			text := item.Data
			if item.Type == html.ElementNode {
				text = site.Node{Node: item}.Text()
			}
			uploader = strings.TrimRight(strings.TrimLeft(text, "\r\n\t"), "\r\n\t")
		}
	}
	imdbID, err := p.findIMDbID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &result{
		language: lang, ref: ref, id: id, series: strings.TrimSpace(name),
		releases: releases, year: year, imdbID: imdbID, uploader: uploader,
	}, nil
}

// child returns a node's i-th child, text included, as BeautifulSoup's
// contents[i]; nil when it has fewer.
func child(n site.Node, i int) *html.Node {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if i == 0 {
			return c
		}
		i--
	}
	return nil
}

// strippedText returns the element's text of stripped strings, as
// BeautifulSoup's get_text(strip=True).
func strippedText(n site.Node) string {
	if !n.Valid() {
		return ""
	}
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(m *html.Node) {
		if m.Type == html.TextNode {
			b.WriteString(strings.TrimSpace(m.Data))
		}
		for c := m.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n.Node)
	return b.String()
}

// outerHTML renders an element, as str() of BeautifulSoup's tags.
func outerHTML(n site.Node) string {
	var b bytes.Buffer
	if err := html.Render(&b, n.Node); err != nil {
		return ""
	}
	return b.String()
}

// requote escapes what a URL cannot hold, as requests does with the
// URLs it is given.
func requote(s string) string {
	var b strings.Builder
	for _, c := range []byte(s) {
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.IndexByte("-._~!#$%&'()*+,/:;=?@[]", c) >= 0 {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

// Download implements provider.Provider: the subtitle of the archive, the
// episode's for an episode, or the subtitle file served.
func (p *Provider) Download(ctx context.Context, v *provider.Video, id string) (provider.File, error) {
	link := p.link(requote(id))
	if _, err := url.Parse(link); err != nil {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "malformed subtitle ID %q", id)
	}
	r, err := p.s.Get(ctx, link, nil)
	if err != nil {
		return provider.File{}, err
	}
	if err := site.Status(r); err != nil {
		return provider.File{}, err
	}
	return archive.Extract(r.Body, archive.WantFor(v, provider.Language{}), provider.Language{})
}

// sanitize strips a title's special characters, as Bazarr's sanitize.
func sanitize(s string) string {
	s = replaced.ReplaceAllString(s, " ")
	s = removed.ReplaceAllString(s, "")
	return strings.ToLower(strings.TrimSpace(spaces.ReplaceAllString(s, " ")))
}

// sanitizeReleaseGroup strips what is in square brackets, as subliminal's
// sanitize_release_group.
func sanitizeReleaseGroup(s string) string {
	return strings.ToUpper(strings.TrimSpace(bracketed.ReplaceAllString(s, "")))
}

// equivalentReleaseGroups returns the release groups of the same
// releases, as subliminal's get_equivalent_release_groups.
func equivalentReleaseGroups(group string) []string {
	for _, g := range [][]string{{"FRAMESTOR", "W4NK3R", "BHDSTUDIO"}, {"LOL", "DIMENSION"}, {"ASAP", "IMMERSE", "FLEET"}, {"AVS", "SVA"}} {
		if slices.Contains(g, group) {
			return g
		}
	}
	return []string{group}
}

// String names the provider in logs.
func (p *Provider) String() string { return fmt.Sprintf("%s(%s)", Name, p.base.Host) }
