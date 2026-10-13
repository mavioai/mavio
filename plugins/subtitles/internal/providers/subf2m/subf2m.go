// Package subf2m is the provider of subf2m.co, ported from Bazarr's
// subf2m: subtitles of movies and of seasons found by title, the
// episode's picked from the archives of season packs.
package subf2m

import (
	"cmp"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/mavioai/mavio/plugins/subtitles/internal/archive"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/scrape"
	"github.com/mavioai/mavio/plugins/subtitles/internal/site"
)

// Name is the provider's name.
const Name = "subf2m"

// baseURL is the site's.
const baseURL = "https://subf2m.co"

// seasons are the ordinals the site names seasons by, as Bazarr's
// _SEASONS.
var seasons = []string{
	"First", "Second", "Third", "Fourth", "Fifth", "Sixth", "Seventh", "Eighth", "Ninth", "Tenth",
	"Eleventh", "Twelfth", "Thirteenth", "Fourteenth", "Fifteenth", "Sixteenth", "Seventeenth", "Eighteenth",
	"Nineteenth", "Twentieth",
}

// languages are the site's language paths by tag, as Bazarr's
// _LANGUAGE_MAP and brazillian-portuguese.
var languages = map[string]string{
	"en": "english", "fa": "farsi_persian", "ar": "arabic", "es": "spanish", "pt": "portuguese", "it": "italian",
	"nl": "dutch", "he": "hebrew", "id": "indonesian", "da": "danish", "no": "norwegian", "bn": "bengali",
	"bg": "bulgarian", "hr": "croatian", "sv": "swedish", "vi": "vietnamese", "cs": "czech", "fi": "finnish",
	"fr": "french", "de": "german", "el": "greek", "hu": "hungarian", "is": "icelandic", "ja": "japanese",
	"mk": "macedonian", "ms": "malay", "pl": "polish", "ro": "romanian", "ru": "russian", "sr": "serbian",
	"th": "thai", "tr": "turkish", "pt-BR": "brazillian-portuguese",
}

// headers are Bazarr's _DEFAULT_HEADERS.
var headers = map[string]string{
	"authority":                 "subf2m.co",
	"accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7",
	"accept-language":           "en-US,en;q=0.9",
	"referer":                   "https://subf2m.co",
	"sec-ch-ua":                 `"Chromium";v="111", "Not(A:Brand";v="8"`,
	"sec-ch-ua-mobile":          "?0",
	"sec-ch-ua-platform":        `"Unknown"`,
	"sec-fetch-dest":            "document",
	"sec-fetch-mode":            "navigate",
	"sec-fetch-site":            "same-origin",
	"sec-fetch-user":            "?1",
	"upgrade-insecure-requests": "1",
}

// Definition returns the provider's definition.
func Definition() provider.Definition {
	tags := slices.Sorted(func(yield func(string) bool) {
		for t := range languages {
			if !yield(t) {
				return
			}
		}
	})
	return provider.Definition{
		Name: Name, DisplayName: "subf2m.co",
		Settings: []provider.Setting{
			{Name: "verify_ssl", Label: "Verify SSL", Type: provider.Checkbox, Default: true},
			{Name: "user_agent", Label: "User-agent header", Help: "Make sure to use a unique and credible user agent.", Type: provider.Text},
		},
		Languages: tags,
		Movies:    true, Episodes: true,
		New: New,
	}
}

// settings are the provider's settings.
type settings struct {
	VerifySSL *bool  `json:"verify_ssl"`
	UserAgent string `json:"user_agent"`
}

// New makes the provider from its settings: a user agent is required.
func New(env provider.Env, raw json.RawMessage) (provider.Provider, error) {
	var s settings
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, provider.Errorf(provider.ConfigurationError, "settings: %w", err)
		}
	}
	if strings.TrimSpace(s.UserAgent) == "" {
		return nil, provider.Errorf(provider.ConfigurationError, "User-agent config missing")
	}
	hc := env.HTTP
	if s.VerifySSL != nil && !*s.VerifySSL {
		hc = insecure(hc)
	}
	sess := site.New(hc)
	for k, v := range headers {
		sess.Header.Set(k, v)
	}
	sess.Header.Set("User-Agent", s.UserAgent)
	// Bazarr reads the pages the site answers 403, 404 and 503 with.
	sess.Check = func(r *site.Response) error {
		switch r.Status {
		case http.StatusForbidden, http.StatusNotFound, http.StatusServiceUnavailable:
			return nil
		}
		return site.Status(r)
	}
	return &Provider{env: env, s: sess, base: baseURL}, nil
}

// insecure returns a client that does not verify certificates, when its
// transport is the standard one; as is otherwise.
func insecure(hc *http.Client) *http.Client {
	t, ok := hc.Transport.(*http.Transport)
	if !ok {
		return hc
	}
	t = t.Clone()
	if t.TLSClientConfig == nil {
		t.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	t.TLSClientConfig.InsecureSkipVerify = true //nolint:gosec // The user turned Verify SSL off.
	c := *hc
	c.Transport = t
	return &c
}

// Provider is subf2m.co.
type Provider struct {
	env  provider.Env
	s    *site.Session
	base string
}

// SetBaseURL points the provider at another server, for tests.
func (p *Provider) SetBaseURL(u *url.URL) { p.base = strings.TrimSuffix(u.String(), "/") }

// page fetches a page, as Bazarr's _safe_get_text: a 404 or 503 is tried
// again up to 3 times, a 403 read as it is.
func (p *Provider) page(ctx context.Context, u string) (site.Node, error) {
	var r *site.Response
	for n := range 3 {
		var err error
		if r, err = p.s.Get(ctx, u, nil); err != nil {
			return site.Node{}, err
		}
		if r.Status != http.StatusNotFound && r.Status != http.StatusServiceUnavailable {
			break
		}
		p.env.Log.DebugContext(ctx, "subf2m: 503/404 returned, trying again", "try", n+1)
		if err := site.Sleep(ctx, p.env.RetryWait); err != nil {
			return site.Node{}, err
		}
	}
	doc, err := scrape.HTML(r)
	if err != nil {
		return site.Node{}, provider.Errorf(provider.ParseResponseError, "%s: %w", u, err)
	}
	return doc, nil
}

// results are the titles the site finds for a query, as Bazarr's
// _gen_results.
func (p *Provider) results(ctx context.Context, query string) ([]site.Node, error) {
	doc, err := p.page(ctx, p.base+"/subtitles/searchbytitle?query="+scrape.Escape(query)+"&l=")
	if err != nil {
		return nil, err
	}
	return doc.All("li div[class='title'] a"), nil
}

type scored struct {
	href       string
	similarity float64
}

// best returns the hrefs of the 3 most similar results.
func best(results []scored) []string {
	slices.SortStableFunc(results, func(a, b scored) int { return cmp.Compare(b.similarity, a.similarity) })
	var out []string
	for _, r := range results[:min(3, len(results))] {
		if !slices.Contains(out, r.href) {
			out = append(out, r.href)
		}
	}
	return out
}

var (
	movieTitleRe     = regexp.MustCompile(`^(.+?)(\s+\((\d{4})\))?$`)
	tvShowTitleRe    = regexp.MustCompile(`^(.+?)\s+[-(]\s?(.*?)\s+(season|series)\)?(\s+\((\d{4})\))?$`)
	tvShowTitleAltRe = regexp.MustCompile(`^(.+)\s(\d{1,2})(?:\s|$)`)
)

// searchMovie returns the paths of a movie's pages, as Bazarr's
// _search_movie: the titles of the year most similar to the title.
func (p *Provider) searchMovie(ctx context.Context, title string, year int) ([]string, error) {
	title = strings.ToLower(title)
	found, err := p.results(ctx, title)
	if err != nil {
		return nil, err
	}
	var results []scored
	for _, a := range found {
		m := movieTitleRe.FindStringSubmatch(strings.ToLower(strings.TrimSpace(a.Text())))
		if m == nil || year == 0 || m[3] != strconv.Itoa(year) {
			continue
		}
		results = append(results, scored{a.Attr("href"), scrape.Ratio(title, m[1])})
	}
	return best(results), nil
}

// searchSeason returns the paths of a season's pages, as Bazarr's
// _search_tv_show_season: the titles of the season, or of complete
// series, most similar to the title, those of its year first.
func (p *Provider) searchSeason(ctx context.Context, title string, season, year int) ([]string, error) {
	// Python's negative indexes wrap.
	i := season - 1
	if i < 0 {
		i += len(seasons)
	}
	if i < 0 || i >= len(seasons) {
		p.env.Log.DebugContext(ctx, "subf2m: season number not supported", "season", season)
		return nil, nil
	}
	seasonStrs := []string{strings.ToLower(seasons[i]), strconv.Itoa(season)}
	found, err := p.results(ctx, title)
	if err != nil {
		return nil, err
	}
	var results []scored
	for _, a := range found {
		text := strings.ToLower(strings.TrimSpace(a.Text()))
		m := tvShowTitleRe.FindStringSubmatch(text)
		if m == nil {
			m = tvShowTitleAltRe.FindStringSubmatch(text)
		}
		if m == nil {
			p.env.Log.DebugContext(ctx, "subf2m: series title not matched", "title", text)
			continue
		}
		matchSeason := strings.ToLower(strings.TrimSpace(m[2]))
		if !slices.Contains(seasonStrs, matchSeason) && !strings.Contains(matchSeason, "complete") {
			continue
		}
		plus := 0.0
		if year != 0 && strings.Contains(text, strconv.Itoa(year)) {
			plus = 0.1
		}
		results = append(results, scored{a.Attr("href"), scrape.Ratio(title, strings.TrimSpace(m[1])) + plus})
	}
	return best(results), nil
}

// imdbHeader is where a subtitles page links the title's IMDb page.
const imdbHeader = "#content > div.subtitles.byFilm > div.box.clearfix > div.top.left > div.header > h2 > a"

// matchIMDb reports whether a subtitles page is of an IMDb ID, as
// Bazarr's _match_imdb: pages without one match any.
func matchIMDb(doc site.Node, imdbID string) bool {
	a := doc.First(imdbHeader)
	if !a.Valid() || !scrape.HasAttr(a, "href") {
		return true
	}
	parts := strings.Split(a.Attr("href"), "/")
	return strings.TrimSpace(parts[len(parts)-1]) == imdbID
}

// specialEpisodeRe is "Season 2 - 05", as Bazarr's _EPISODE_SPECIAL_RE.
var specialEpisodeRe = regexp.MustCompile(`(?i)(season|s)\s*?(\d{0,2})\s?[-−]\s?(\d{0,2})`)

// episodeFromRelease reads "Season 2 - 05" as season 2, episode 5, as
// Bazarr's _get_episode_from_release; ok is false otherwise.
func episodeFromRelease(release string) (season, episode int, ok bool) {
	m := specialEpisodeRe.FindStringSubmatch(release)
	if m == nil {
		return 0, 0, false
	}
	s, err1 := strconv.Atoi(m[2])
	e, err2 := strconv.Atoi(m[3])
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return s, e, true
}

// item is a subtitle of a subtitles page.
type item struct {
	pageLink string
	releases []string
}

// parseItem reads a subtitle of a page, as Bazarr's
// _get_subtitle_from_item; ok is false without a download link.
func (p *Provider) parseItem(n site.Node) (item, bool) {
	var it item
	if ul := n.First("ul.scrolllist"); ul.Valid() {
		for _, li := range scrape.Contents(ul) {
			if r := strings.TrimSpace(li); r != "" {
				it.releases = append(it.releases, r)
			}
		}
	}
	if para := n.First("div.comment-col p"); para.Valid() {
		if c := strings.TrimSpace(strings.ReplaceAll(para.Text(), "\n", " ")); c != "" {
			it.releases = append(it.releases, c)
		}
	}
	a := n.First("a.download.icon-download")
	if !a.Valid() || !scrape.HasAttr(a, "href") {
		return item{}, false
	}
	it.pageLink = p.base + a.Attr("href")
	return it, true
}

// subtitlesPage fetches the subtitles of a page in a language.
func (p *Provider) subtitlesPage(ctx context.Context, page, tag string) (site.Node, error) {
	return p.page(ctx, p.base+page+"/"+languages[tag])
}

// findMovie returns the subtitles of a movie's page, as Bazarr's
// _find_movie_subtitles.
func (p *Provider) findMovie(ctx context.Context, page, tag, imdbID string) ([]item, error) {
	doc, err := p.subtitlesPage(ctx, page, tag)
	if err != nil || !matchIMDb(doc, imdbID) {
		return nil, err
	}
	var out []item
	for _, n := range doc.All("li.item") {
		if it, ok := p.parseItem(n); ok {
			out = append(out, it)
		}
	}
	return out, nil
}

// findEpisode returns the subtitles of an episode on a season's page, as
// Bazarr's _find_episode_subtitles: those of the episode, and season
// packs.
func (p *Provider) findEpisode(ctx context.Context, page string, season, episode int, tag, imdbID string) ([]item, error) {
	doc, err := p.subtitlesPage(ctx, page, tag)
	if err != nil || !matchIMDb(doc, imdbID) {
		return nil, err
	}
	var out []item
	for _, n := range doc.All("li.item") {
		text := strings.Join(strings.Fields(n.Text()), " ")
		if text == "" {
			continue
		}
		var gs, ge []int
		if s, e, ok := episodeFromRelease(text); ok {
			gs, ge = []int{s}, []int{e}
		} else {
			gs, ge = scrape.SeasonsEpisodes(text)
		}
		if len(gs) == 0 {
			if !strings.Contains(strings.ToLower(text), "complete series") {
				continue
			}
			gs = []int{season}
		}
		if !slices.Contains(gs, season) || len(ge) > 0 && !slices.Contains(ge, episode) {
			continue
		}
		if it, ok := p.parseItem(n); ok {
			out = append(out, it)
		}
	}
	return out, nil
}

// Search implements provider.Provider, as Bazarr's list_subtitles: the
// subtitles of the first of the pages found that has some.
func (p *Provider) Search(ctx context.Context, v *provider.Video, langs []provider.Language) ([]provider.Subtitle, error) {
	var paths []string
	var err error
	imdbID := v.ExternalIDs["imdb"]
	if v.Episode {
		paths, err = p.searchSeason(ctx, v.Series, v.Season, v.Year)
	} else {
		paths, err = p.searchMovie(ctx, v.Title, v.Year)
		if err == nil && len(paths) == 0 && imdbID != "" {
			// The title may be a localized one the site does not index;
			// it finds IMDb IDs too.
			paths, err = p.searchMovie(ctx, imdbID, v.Year)
		}
	}
	if err != nil || len(paths) == 0 {
		return nil, err
	}
	var tags []string
	for _, l := range langs {
		if _, ok := languages[l.Tag]; ok && !slices.Contains(tags, l.Tag) {
			tags = append(tags, l.Tag)
		}
	}
	matches := []string{"title", "year", "imdb_id"}
	if v.Episode {
		matches = []string{"title", "series", "year", "season", "episode", "imdb_id"}
	}
	var out []provider.Subtitle
	for _, page := range paths {
		for _, tag := range tags {
			var items []item
			if v.Episode {
				items, err = p.findEpisode(ctx, page, v.Season, v.Number, tag, v.SeriesExternalIDs["imdb"])
			} else {
				items, err = p.findMovie(ctx, page, tag, imdbID)
			}
			if err != nil {
				return nil, err
			}
			for _, it := range items {
				out = append(out, provider.Subtitle{
					ID: it.pageLink, Language: provider.Language{Tag: tag},
					ReleaseNames: it.releases, Matches: slices.Clone(matches),
					PageLink: it.pageLink,
				})
			}
		}
		if len(out) > 0 {
			p.env.Log.DebugContext(ctx, "subf2m: good path found, not running over others", "path", page)
			break
		}
	}
	return out, nil
}

// Download implements provider.Provider, as Bazarr's download_subtitle:
// the archive the subtitle's page links, the episode's subtitle picked
// from season packs by its number and title.
func (p *Provider) Download(ctx context.Context, v *provider.Video, id string) (provider.File, error) {
	doc, err := p.page(ctx, id)
	if err != nil {
		return provider.File{}, err
	}
	a := doc.First("a[id='downloadButton']")
	if !a.Valid() || !scrape.HasAttr(a, "href") {
		return provider.File{}, provider.Errorf(provider.APIThrottled, "couldn't get download url from %s", id)
	}
	r, err := p.s.Get(ctx, p.base+a.Attr("href"), nil)
	if err != nil {
		return provider.File{}, err
	}
	arch, err := archive.Open(r.Body)
	if errors.Is(err, archive.ErrNotArchive) {
		return provider.File{}, provider.Errorf(provider.APIThrottled, "invalid archive: %s", id)
	}
	if err != nil {
		return provider.File{}, provider.Errorf(provider.APIThrottled, "invalid archive: %s: %w", id, err)
	}
	var w archive.Want
	if v != nil && v.Episode {
		w = archive.Want{Episode: v.Number, EpisodeTitle: v.Title}
	}
	name, data, err := archive.Subtitle(arch, w)
	if err != nil {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "%s: %w", id, err)
	}
	if name == "" {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "no subtitle of the video in the archive of %s", id)
	}
	format := strings.TrimPrefix(strings.ToLower(path.Ext(name)), ".")
	if format == "sub" || format == "" {
		format = archive.Detect(data)
	}
	return provider.File{Data: data, Format: format}, nil
}
