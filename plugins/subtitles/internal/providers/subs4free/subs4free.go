// Package subs4free is the provider of Subs4Free, ported from Bazarr's
// subs4free: Greek and English subtitles of movies found by title.
package subs4free

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand/v2"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/archive"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/scrape"
	"github.com/mavioai/mavio/plugins/subtitles/internal/site"
)

// Name is the provider's name.
const Name = "subs4free"

// showExpiration is how long the suggestions of a title are cached, as
// subliminal's SHOW_EXPIRATION_TIME.
const showExpiration = 3 * 7 * 24 * time.Hour

// urls are the site's.
type urls struct {
	server string
	// antiBlock are fetched before downloading, as Bazarr's
	// apply_anti_block.
	antiBlock []string
}

var defaultURLs = urls{
	server: "https://www.subs4free.info",
	antiBlock: []string{
		"https://images.subs4free.info/favicon.ico",
		"https://www.subs4series.com/includes/anti-block-layover.php?launch=1",
		"https://www.subs4series.com/includes/anti-block.php",
	},
}

// Definition returns the provider's definition.
func Definition() provider.Definition {
	return provider.Definition{
		Name: Name, DisplayName: "Subs4Free",
		Languages: []string{"el", "en"},
		Movies:    true,
		New:       New,
	}
}

// New makes the provider; it has no settings.
func New(env provider.Env, _ json.RawMessage) (provider.Provider, error) {
	return &Provider{env: env, s: site.New(env.HTTP), u: defaultURLs, suggestions: map[string]cached{}}, nil
}

// Provider is Subs4Free.
type Provider struct {
	env provider.Env
	s   *site.Session
	u   urls

	mu sync.Mutex
	// suggestions caches the site's suggestions of titles, as Bazarr's
	// region cache.
	suggestions map[string]cached
}

type cached struct {
	list  []suggestion
	until time.Time
}

type suggestion struct{ link, title string }

// SetBaseURL points the provider at another server, for tests.
func (p *Provider) SetBaseURL(u *url.URL) {
	base := strings.TrimSuffix(u.String(), "/")
	p.u = urls{server: base, antiBlock: []string{base + "/favicon.ico", base + "/includes/anti-block-layover.php?launch=1", base + "/includes/anti-block.php"}}
}

func (p *Provider) searchURL(search string) string {
	return p.u.server + "/search_report.php?search=" + scrape.Escape(search) + "&searchType=1"
}

// page fetches a page; nil for an empty answer.
func (p *Provider) page(ctx context.Context, u string, header http.Header) (*site.Node, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range header {
		req.Header[k] = v
	}
	r, err := p.s.Do(req)
	if err != nil {
		return nil, err
	}
	if len(r.Body) == 0 {
		p.env.Log.DebugContext(ctx, "subs4free: no data returned from provider")
		return nil, nil
	}
	doc, err := scrape.HTML(r)
	if err != nil {
		return nil, provider.Errorf(provider.ParseResponseError, "%s: %w", u, err)
	}
	return &doc, nil
}

// getSuggestions returns the titles the site suggests for a title, as
// Bazarr's _get_suggestions, cached while there are some.
func (p *Provider) getSuggestions(ctx context.Context, title string) ([]suggestion, error) {
	now := p.env.Now()
	p.mu.Lock()
	c, ok := p.suggestions[title]
	p.mu.Unlock()
	if ok && now.Before(c.until) {
		return c.list, nil
	}
	doc, err := p.page(ctx, p.searchURL(title), http.Header{"Referer": {p.u.server}})
	if err != nil || doc == nil {
		return nil, err
	}
	var out []suggestion
	for _, o := range doc.All(`select[name="Mov_sel"] > option[value]`) {
		out = append(out, suggestion{link: o.Attr("value"), title: o.Text()})
	}
	if len(out) > 0 {
		p.mu.Lock()
		p.suggestions[title] = cached{list: out, until: now.Add(showExpiration)}
		p.mu.Unlock()
	}
	return out, nil
}

// showLinks returns the links of a title's pages, as Bazarr's
// get_show_links.
func (p *Provider) showLinks(ctx context.Context, title string, year int) ([]string, error) {
	title = scrape.Sanitize(title, "")
	suggestions, err := p.getSuggestions(ctx, title)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, s := range suggestions {
		show := scrape.Sanitize(s.title, "")
		if show == title || year != 0 && show == title+" "+strconv.Itoa(year) {
			parts := strings.Split(s.link, "?p=")
			out = append(out, parts[len(parts)-1])
		}
	}
	return out, nil
}

var yearRe = regexp.MustCompile(`^\((\d{4})\)$`)

// result is a subtitle of a page.
type result struct {
	tag, title, version, downloadLink, uploader string
	year                                        int
}

// query lists the subtitles of a movie's page, or of the search for its
// title and year, as Bazarr's query.
func (p *Provider) query(ctx context.Context, movieID, title string, year int) ([]result, string, error) {
	pageLink := p.searchURL(title + " " + strconv.Itoa(year))
	if movieID != "" {
		pageLink = p.u.server + "/" + movieID
	}
	doc, err := p.page(ctx, pageLink, nil)
	if err != nil || doc == nil {
		return nil, pageLink, err
	}
	pageYear := 0
	if div := doc.First("td#dates_header > table div"); div.Valid() {
		if contents := scrape.Contents(div); len(contents) > 2 {
			if m := yearRe.FindStringSubmatch(strings.TrimSpace(contents[2])); m != nil {
				pageYear, _ = strconv.Atoi(m[1])
			}
		}
	}
	showTitle := ""
	if u := doc.First("td#dates_header > table u"); u.Valid() {
		if contents := scrape.Contents(u); len(contents) > 0 {
			showTitle = strings.TrimSpace(contents[0])
		}
	}
	var out []result
	for _, n := range doc.All(".movie-details") {
		sprite := strings.Fields(n.First(".sprite").Attr("class"))
		a := n.First("a")
		if len(sprite) < 2 || !a.Valid() {
			return nil, pageLink, provider.Errorf(provider.ParseResponseError, "%s: malformed subtitle", pageLink)
		}
		code, _, _ := strings.Cut(sprite[1], "gif")
		out = append(out, result{
			tag: provider.FromCode(code), title: showTitle, year: pageYear,
			version:      n.First("span").Text(),
			downloadLink: p.u.server + a.Attr("href"),
			uploader:     n.First(".movie-info p a").Text(),
		})
	}
	return out, pageLink, nil
}

// Search implements provider.Provider, as Bazarr's list_subtitles: the
// subtitles of the pages the site suggests for the first of the movie's
// titles that has some, else of the search for its title and year.
func (p *Provider) Search(ctx context.Context, v *provider.Video, langs []provider.Language) ([]provider.Subtitle, error) {
	if v.Episode {
		return nil, nil
	}
	var links []string
	for _, title := range append([]string{v.Title}, v.AlternativeTitles...) {
		var err error
		if links, err = p.showLinks(ctx, title, v.Year); err != nil {
			return nil, err
		}
		if len(links) > 0 {
			break
		}
	}
	if len(links) == 0 {
		links = []string{""}
	}
	var out []provider.Subtitle
	for _, link := range links {
		title := v.Title
		if link == "" {
			title = scrape.Sanitize(v.Title, "")
		}
		results, pageLink, err := p.query(ctx, link, title, v.Year)
		if err != nil {
			return nil, err
		}
		for _, r := range results {
			if !slices.ContainsFunc(langs, func(l provider.Language) bool { return l.Tag == r.tag }) {
				continue
			}
			out = append(out, provider.Subtitle{
				ID:           url.Values{"link": {r.downloadLink}, "page": {pageLink}}.Encode(),
				Language:     provider.Language{Tag: r.tag},
				ReleaseNames: []string{r.version},
				Matches:      matches(r, v),
				PageLink:     pageLink,
				Uploader:     r.uploader,
			})
		}
	}
	return out, nil
}

// matches are what a subtitle matches of a movie, as Bazarr's get_matches
// knows from the page; its version's are guessed by the server.
func matches(r result, v *provider.Video) []string {
	var out []string
	if v.Title != "" {
		title := scrape.Sanitize(r.title, "")
		for _, name := range append([]string{v.Title}, v.AlternativeTitles...) {
			if scrape.Sanitize(name, "") == title {
				out = append(out, "title")
				break
			}
		}
	}
	if v.Year != 0 && r.year == v.Year {
		out = append(out, "year")
	}
	return out
}

// Download implements provider.Provider, as Bazarr's download_subtitle:
// the download page's form is posted past the site's anti-block pages,
// and the first subtitle of the archive answered taken.
func (p *Provider) Download(ctx context.Context, _ *provider.Video, id string) (provider.File, error) {
	q, err := url.ParseQuery(id)
	link, pageLink := q.Get("link"), q.Get("page")
	if err != nil || link == "" {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "malformed subtitle ID %q", id)
	}
	doc, err := p.page(ctx, link, http.Header{"Referer": {pageLink}})
	if err != nil {
		return provider.File{}, err
	}
	if doc == nil {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "unable to download subtitle: no data returned from provider")
	}
	subtitleID := doc.First(`input[name="id"]`).Attr("value")
	if subtitleID == "" {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "unable to download subtitle: no download link found")
	}
	image := doc.First(`input[type="image"]`)
	width, _ := strconv.Atoi(strings.Trim(image.Attr("width"), "px"))
	height, _ := strconv.Atoi(strings.Trim(image.Attr("height"), "px"))

	for _, u := range p.u.antiBlock {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return provider.File{}, err
		}
		req.Header.Set("Referer", link)
		if _, err := p.s.Do(req); err != nil {
			return provider.File{}, err
		}
	}

	form := url.Values{
		"id": {subtitleID},
		"x":  {strconv.Itoa(rand.IntN(width + 1))},  //nolint:gosec // Where the image was clicked.
		"y":  {strconv.Itoa(rand.IntN(height + 1))}, //nolint:gosec // Where the image was clicked.
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.u.server+"/getSub.php", strings.NewReader(form.Encode()))
	if err != nil {
		return provider.File{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", link)
	r, err := p.s.Do(req)
	if err != nil {
		return provider.File{}, err
	}
	if len(r.Body) == 0 {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "unable to download subtitle: no data returned from provider")
	}
	return extract(r.Body)
}

// extract returns the first subtitle of a zip or RAR archive, else the
// content as it is.
func extract(data []byte) (provider.File, error) {
	a, err := archive.Open(data)
	switch {
	case errors.Is(err, archive.ErrNotArchive):
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "the download is not a subtitle or an archive")
	case err != nil:
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "%w", err)
	}
	for _, name := range a.Names() {
		if strings.HasPrefix(path.Base(name), ".") || !scrape.IsSubtitle(name) {
			continue
		}
		sub, err := a.Read(name)
		if err != nil {
			return provider.File{}, provider.Errorf(provider.BadSubtitle, "%s: %w", name, err)
		}
		format := strings.TrimPrefix(strings.ToLower(path.Ext(name)), ".")
		if format != "srt" && format != "ass" && format != "ssa" && format != "vtt" {
			format = archive.Detect(sub)
		}
		return provider.File{Data: archive.FixLineEndings(sub), Format: format}, nil
	}
	return provider.File{}, provider.Errorf(provider.BadSubtitle, "could not extract subtitle from the archive")
}
