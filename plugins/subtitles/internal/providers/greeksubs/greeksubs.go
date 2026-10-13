// Package greeksubs is the provider of GreekSubs, ported from Bazarr's
// greeksubs: Greek subtitles of movies and episodes, found on the site's
// pages of IMDb IDs and downloaded with single-use tokens.
package greeksubs

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/net/html"

	"github.com/mavioai/mavio/plugins/subtitles/internal/archive"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/site"
)

// Name is the provider's name.
const Name = "greeksubs"

// utf8 is how Bazarr decodes the site's pages.
const utf8 = "text/html; charset=utf-8"

// Definition returns the provider's definition.
func Definition() provider.Definition {
	return provider.Definition{
		Name: Name, DisplayName: "GreekSubs",
		Languages: []string{"el"},
		Movies:    true, Episodes: true,
		New: New,
	}
}

// New makes the provider; it has no settings.
func New(env provider.Env, _ json.RawMessage) (provider.Provider, error) {
	hc := *env.HTTP
	next := hc.CheckRedirect
	hc.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if req.Context().Value(noRedirect{}) != nil {
			return http.ErrUseLastResponse
		}
		if next != nil {
			return next(req, via)
		}
		return nil
	}
	return &Provider{env: env, s: site.New(&hc), server: "https://greeksubs.net/"}, nil
}

// noRedirect marks the requests whose redirects are not followed.
type noRedirect struct{}

// Provider is GreekSubs.
type Provider struct {
	env    provider.Env
	s      *site.Session
	server string
}

// SetBaseURL points the provider at another server, for tests.
func (p *Provider) SetBaseURL(u *url.URL) { p.server = strings.TrimSuffix(u.String(), "/") + "/" }

var (
	seasonEpisode = regexp.MustCompile(`Season (\d+) Episode (\d+)`)
	downloadMe    = regexp.MustCompile(`downloadMe\('([^']+)'\)`)
)

// Search implements provider.Provider.
func (p *Provider) Search(ctx context.Context, v *provider.Video, langs []provider.Language) ([]provider.Subtitle, error) {
	imdb := v.IMDbID()
	if imdb == "" {
		return nil, nil
	}
	link := p.server + "en/view/" + imdb
	r, err := p.s.Get(ctx, link, nil)
	if r != nil && r.Status == http.StatusNotFound {
		// The IMDb ID is not on the site.
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	doc, err := site.Parse(r.Body, utf8)
	if err != nil {
		return nil, provider.Errorf(provider.ParseResponseError, "%s: %w", imdb, err)
	}
	if !v.Episode {
		return p.subtitles(v, doc, link, langs), nil
	}
	var out []provider.Subtitle
	for _, a := range doc.All("div.col-lg-offset-2.col-md-8.text-center.top30.bottom10 > a") {
		m := seasonEpisode.FindStringSubmatch(a.Text())
		if m == nil || atoi(m[1]) != v.Season || atoi(m[2]) != v.Number {
			continue
		}
		page, err := r.URL.Parse(a.Attr("href"))
		if err != nil {
			continue
		}
		er, err := p.s.Get(ctx, page.String(), nil)
		if err != nil {
			return nil, err
		}
		edoc, err := site.Parse(er.Body, utf8)
		if err != nil {
			return nil, provider.Errorf(provider.ParseResponseError, "%s: %w", page, err)
		}
		out = append(out, p.subtitles(v, edoc, page.String(), langs)...)
	}
	return out, nil
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

// subtitles reads the table of subtitles of a page, as Bazarr's
// _parse_subtitle_row, keeping those in the languages; none when the page
// has no download token.
func (p *Provider) subtitles(v *provider.Video, doc site.Node, referer string, langs []provider.Language) []provider.Subtitle {
	sec := doc.First("input#secCode")
	if !sec.Valid() {
		return nil
	}
	var out []provider.Subtitle
	for _, row := range doc.All("#elSub > tbody > tr") {
		button := row.First(`button[onclick^="downloadMe"]`)
		m := downloadMe.FindStringSubmatch(button.Attr("onclick"))
		img := row.First(`img[src*="/resources/lang/"]`)
		names := row.First(".userNameBox")
		if m == nil || !img.Valid() || !names.Valid() {
			continue
		}
		tag := provider.FromCode(img.Attr("alt"))
		if tag == "" || len(langs) > 0 && !slices.ContainsFunc(langs, func(l provider.Language) bool { return l.Tag == tag }) {
			continue
		}
		cell := button.Parent()
		for cell.Valid() && cell.Data != "td" {
			cell = cell.Parent()
		}
		version := nextCell(nextCell(cell))
		if !version.Valid() {
			continue
		}
		var matches []string
		if v.Episode {
			// As Bazarr's, which blatantly matches the year of episodes.
			matches = append(matches, "year")
		}
		id := url.Values{"id": {m[1]}, "sec": {sec.Attr("value")}, "ref": {referer}, "lang": {tag}}
		out = append(out, provider.Subtitle{
			ID:           id.Encode(),
			Language:     provider.Language{Tag: tag},
			ReleaseNames: []string{strings.ReplaceAll(version.TrimmedText(), "-", ".")},
			Matches:      matches,
			PageLink:     p.downloadLink(m[1], sec.Attr("value")),
			Uploader:     names.TrimmedText(),
		})
	}
	return out
}

// nextCell returns the next sibling cell of a cell, as BeautifulSoup's
// find_next_sibling('td').
func nextCell(n site.Node) site.Node {
	if !n.Valid() {
		return site.Node{}
	}
	for s := n.NextSibling; s != nil; s = s.NextSibling {
		if s.Type == html.ElementNode && s.Data == "td" {
			return site.Node{Node: s}
		}
	}
	return site.Node{}
}

func (p *Provider) downloadLink(id, sec string) string {
	return p.server + "dll/" + id + "/0/" + sec
}

// Download implements provider.Provider: the subtitle file, after the
// download token is refreshed from the subtitle's page if it has expired,
// or posted from the site's download form.
func (p *Provider) Download(ctx context.Context, _ *provider.Video, id string) (provider.File, error) {
	q, err := url.ParseQuery(id)
	if err != nil || q.Get("id") == "" || q.Get("ref") == "" {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "malformed subtitle ID %q", id)
	}
	link := p.downloadLink(q.Get("id"), q.Get("sec"))
	r, err := p.get(ctx, link, q.Get("ref"))
	if r != nil && r.Status == http.StatusFound {
		if link, err = p.refresh(ctx, q.Get("id"), q.Get("ref")); err != nil {
			return provider.File{}, err
		}
		r, err = p.get(ctx, link, q.Get("ref"))
		if r != nil && r.Status == http.StatusFound {
			return provider.File{}, provider.Errorf(provider.ProviderError, "the single-use download token is still invalid after a refresh")
		}
	}
	if err != nil {
		return provider.File{}, err
	}
	lang := provider.Language{Tag: q.Get("lang")}
	data := r.Body
	contentType := strings.ToLower(r.Header.Get("Content-Type"))
	if len(data) == 0 || !strings.Contains(strings.ToLower(r.Header.Get("Content-Disposition")), "attachment") &&
		(contentType == "" || strings.HasPrefix(contentType, "text/html")) {
		doc, err := site.Parse(r.Body, utf8)
		if err != nil {
			return provider.File{}, provider.Errorf(provider.ParseResponseError, "download page: %w", err)
		}
		form := url.Values{}
		for _, name := range []string{"langcode", "uid", "output", "dll"} {
			in := doc.First(`[name="` + name + `"]`)
			if !in.Valid() {
				return provider.File{}, provider.Errorf(provider.ProviderError, "unable to download the subtitle: no data returned from provider")
			}
			form.Set(name, in.Attr("value"))
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, link, strings.NewReader(form.Encode()))
		if err != nil {
			return provider.File{}, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Referer", link)
		pr, err := p.s.Do(req)
		if err != nil {
			return provider.File{}, err
		}
		data = pr.Body
	}
	if len(data) == 0 {
		return provider.File{}, provider.Errorf(provider.ProviderError, "unable to download the subtitle: no data returned from provider")
	}
	return provider.File{Data: archive.FixLineEndings(data), Format: archive.Detect(data), Language: lang}, nil
}

// get fetches a download link from the subtitle's page, not following
// redirects.
func (p *Provider) get(ctx context.Context, link, referer string) (*site.Response, error) {
	req, err := http.NewRequestWithContext(context.WithValue(ctx, noRedirect{}, true), http.MethodGet, link, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Referer", referer)
	return p.s.Do(req)
}

// refresh returns the download link with a new token from the subtitle's
// page, as Bazarr's _refresh_download_link.
func (p *Provider) refresh(ctx context.Context, id, referer string) (string, error) {
	r, err := p.s.Get(ctx, referer, nil)
	if err != nil {
		return "", err
	}
	doc, err := site.Parse(r.Body, utf8)
	if err != nil {
		return "", provider.Errorf(provider.ParseResponseError, "%s: %w", referer, err)
	}
	sec := doc.First("input#secCode").Attr("value")
	if sec == "" {
		return "", provider.Errorf(provider.ProviderError, "unable to refresh the single-use download token")
	}
	return p.downloadLink(id, sec), nil
}
