// Package subtitrarinoi is the provider of Subtitrari-noi.ro, ported from
// Bazarr's subtitrarinoi: Romanian subtitles of movies and series found by
// IMDb ID, else by title.
package subtitrarinoi

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
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
const Name = "subtitrarinoi"

// serverURL is the site's.
const serverURL = "https://www.subtitrari-noi.ro/"

// userAgent is Bazarr's.
const userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/93.0.4535.2 Safari/537.36"

// renames are the titles the site names differently, as Bazarr's
// fix_inconsistent_naming.
var renames = []string{"DC's Legends of Tomorrow", "Legends of Tomorrow", "Marvel's Jessica Jones", "Jessica Jones"}

// Definition returns the provider's definition.
func Definition() provider.Definition {
	return provider.Definition{
		Name: Name, DisplayName: "Subtitrari-noi.ro",
		Languages: []string{"ro"},
		Movies:    true, Episodes: true,
		New: New,
	}
}

// New makes the provider; it has no settings.
func New(env provider.Env, _ json.RawMessage) (provider.Provider, error) {
	s := site.New(env.HTTP)
	s.Header.Set("User-Agent", userAgent)
	s.Header.Set("X-Requested-With", "XMLHttpRequest")
	s.Header.Set("Referer", serverURL)
	return &Provider{env: env, s: s, base: serverURL}, nil
}

// Provider is Subtitrari-noi.ro.
type Provider struct {
	env  provider.Env
	s    *site.Session
	base string
}

// SetBaseURL points the provider at another server, for tests.
func (p *Provider) SetBaseURL(u *url.URL) {
	p.base = strings.TrimSuffix(u.String(), "/") + "/"
	p.s.Header.Set("Referer", p.base)
}

func (p *Provider) apiURL() string { return p.base + "paginare_filme.php" }

// hit is a subtitle of the search results.
type hit struct {
	link, title, imdbID, uploader, page string
	// comments are the releases, separated by commas.
	comments  string
	year      int
	downloads int
}

// fix is a title as Bazarr's fix_inconsistent_naming makes it: renamed
// and sanitized.
func fix(title string) string { return scrape.SanitizePatch(scrape.FixNaming(title, renames...), "") }

var (
	seasonRe     = regexp.MustCompile(`(?i)(s|(season|sezonul)\s)(\d{1,2})`)
	seasonlessRe = regexp.MustCompile(`\s-\sSezonul\s\d+$`)
	imdbSrcRe    = regexp.MustCompile(`imdb`)
)

// matches are what a subtitle matches of the video, as Bazarr's
// get_matches knows from the site; those guessed of the releases are the
// server's. An episode matches when its series' IMDb ID and season do.
func (h hit) matches(v *provider.Video) []string {
	var out []string
	if v.Year != 0 && h.year == v.Year {
		out = append(out, "year")
	}
	if v.ReleaseGroup != "" && strings.Contains(h.comments, v.ReleaseGroup) {
		out = append(out, "release_group")
	}
	if !v.Episode {
		if v.Title != "" && scrape.SanitizePatch(h.title, "") == fix(v.Title) {
			out = append(out, "title")
		}
		if id := v.ExternalIDs["imdb"]; id != "" && h.imdbID == id {
			out = append(out, "imdb_id")
		}
		return out
	}
	seasonless := seasonlessRe.ReplaceAllString(strings.TrimRight(h.title, " \t\n\r\f\v"), "")
	if v.Series != "" && fix(v.Series) == scrape.SanitizePatch(seasonless, "") {
		out = append(out, "series")
	}
	if id := v.SeriesExternalIDs["imdb"]; id != "" && h.imdbID == id {
		out = append(out, "imdb_id")
	}
	if m := seasonRe.FindStringSubmatch(h.comments); m != nil {
		if n, _ := strconv.Atoi(m[3]); n == v.Season {
			out = append(out, "season")
		}
	}
	if slices.Contains(out, "imdb_id") && slices.Contains(out, "season") {
		out = append(out, "episode")
	}
	return out
}

// query searches the site, as Bazarr's query: by the IMDb ID's digits,
// else by title.
func (p *Provider) query(ctx context.Context, imdbID, title string) ([]hit, error) {
	search := title
	if imdbID != "" {
		search = imdbID
	}
	form := url.Values{
		"search_q": {"1"}, "tip": {"2"}, "an": {"Toti anii"}, "gen": {"Toate"},
		"cautare": {search}, "query_q": {search},
	}
	r, err := p.s.PostForm(ctx, p.apiURL(), form)
	if err != nil {
		return nil, err
	}
	doc, err := scrape.HTML(r)
	if err != nil {
		return nil, provider.Errorf(provider.ParseResponseError, "search: %w", err)
	}
	rows := doc.All(`div[id="round"]`)
	if len(rows) == 0 {
		p.env.Log.DebugContext(ctx, "subtitrarinoi: no data returned")
		return nil, nil
	}
	// The releases are outside of the subtitles' elements, in order.
	comments := doc.All("div:not([class]):not([id]):not([align])")
	var out []hit
	for i, row := range rows {
		a := row.First(".buton a")
		if !a.Valid() {
			return nil, provider.Errorf(provider.ParseResponseError, "no download link in a result")
		}
		h := hit{link: p.base + a.Attr("href")}
		titleA := row.First("#content-main a")
		full := titleA.Text()
		h.title, _, _ = strings.Cut(full, "(")
		h.page = titleA.Attr("href")
		if ps := row.All("#content-main p"); len(ps) > 4 {
			h.uploader = after(ps[4].Text(), 10)
		}
		h.downloads, _ = strconv.Atoi(after(row.First("#content-right p").Text(), 12))
		if _, rest, ok := strings.Cut(full, "("); ok {
			y, _, _ := strings.Cut(rest, ")")
			h.year, _ = strconv.Atoi(y)
		}
		h.imdbID = imdbOf(row)
		if i < len(comments) {
			h.comments = strings.ReplaceAll(comments[i].Text(), ";", ",")
		}
		out = append(out, h)
	}
	slices.SortStableFunc(out, func(a, b hit) int { return cmp.Compare(b.downloads, a.downloads) })
	return out, nil
}

// after is a text past its first n characters, as Python's text[n:].
func after(s string, n int) string {
	r := []rune(s)
	if n >= len(r) {
		return ""
	}
	return string(r[n:])
}

// imdbOf is the IMDb ID a result links to, as Bazarr's
// getImdbIdFromSubtitle: of the last link of its right column, when it
// holds IMDb's image; "" for none.
func imdbOf(row site.Node) string {
	links := row.All("div[id=content-right] a")
	if len(links) == 0 {
		return ""
	}
	for _, n := range links[len(links)-1].All("[src]") {
		if imdbSrcRe.MatchString(n.Attr("src")) {
			id := n.Parent().Attr("href")
			if i := strings.LastIndex(id, "tt"); i >= 0 {
				id = id[i+len("tt"):]
			}
			return strings.TrimRight("tt"+id, "/")
		}
	}
	return ""
}

// Search implements provider.Provider, as Bazarr's list_subtitles: the
// subtitles are in the first language requested, the most downloaded
// first.
func (p *Provider) Search(ctx context.Context, v *provider.Video, langs []provider.Language) ([]provider.Subtitle, error) {
	if len(langs) == 0 {
		return nil, nil
	}
	imdbID := strings.TrimPrefix(v.IMDbID(), "tt")
	hits, err := p.query(ctx, imdbID, fix(v.Title))
	if err != nil {
		return nil, err
	}
	var out []provider.Subtitle
	for _, h := range hits {
		s := provider.Subtitle{
			ID: h.link, Language: langs[0], Matches: h.matches(v),
			PageLink: h.page, Uploader: h.uploader, Comment: h.comments, Downloads: h.downloads,
		}
		if h.comments != "" {
			s.ReleaseNames = []string{h.comments}
		}
		out = append(out, s)
	}
	return out, nil
}

// Download implements provider.Provider, as Bazarr's download_subtitle:
// the episode's subtitle of an archive, else the subtitle downloaded.
func (p *Provider) Download(ctx context.Context, v *provider.Video, id string) (provider.File, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, id, nil)
	if err != nil {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "malformed subtitle ID %q: %w", id, err)
	}
	req.Header.Set("Referer", p.apiURL())
	r, err := p.s.Do(req)
	if err != nil {
		return provider.File{}, err
	}
	if _, err := archive.Open(r.Body); errors.Is(err, archive.ErrNotArchive) {
		data := archive.FixLineEndings(r.Body)
		if archive.Detect(data) == "" {
			return provider.File{}, provider.Errorf(provider.BadSubtitle, "the download is not a subtitle")
		}
		return provider.File{Data: data, Format: archive.Detect(data)}, nil
	}
	var w archive.Want
	if v != nil && v.Episode {
		w.Episode = v.Number
	}
	return archive.Extract(r.Body, w, provider.Language{})
}
