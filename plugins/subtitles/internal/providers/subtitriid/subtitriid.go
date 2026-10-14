// Package subtitriid is the provider of subtitri.id.lv (subtitri.do.am),
// ported from Bazarr's subtitriid: Latvian subtitles of movies found by
// title on the site's pages.
package subtitriid

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/mavioai/mavio/plugins/subtitles/internal/archive"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/site"
)

// Name is the provider's name.
const Name = "subtitriid"

// serverURL is the site's.
const serverURL = "https://subtitri.do.am/"

// Definition returns the provider's definition.
func Definition() provider.Definition {
	return provider.Definition{
		Name: Name, DisplayName: "subtitri.id.lv",
		Languages: []string{"lv"},
		Movies:    true,
		New:       New,
	}
}

// New makes the provider; it has no settings.
func New(env provider.Env, _ json.RawMessage) (provider.Provider, error) {
	base, _ := url.Parse(serverURL)
	p := &Provider{env: env, s: site.NewBrowser(env.HTTP)}
	p.SetBaseURL(base)
	return p, nil
}

// Provider is subtitri.id.lv.
type Provider struct {
	env  provider.Env
	s    *site.Session
	base *url.URL
}

// SetBaseURL points the provider at another server, for tests.
func (p *Provider) SetBaseURL(u *url.URL) {
	p.base = u
	p.s.Header.Set("Referer", strings.TrimSuffix(u.String(), "/"))
}

func (p *Provider) resolve(ref string) (string, error) {
	r, err := url.Parse(ref)
	if err != nil {
		return "", provider.Errorf(provider.ParseResponseError, "link %q: %w", ref, err)
	}
	return p.base.ResolveReference(r).String(), nil
}

// Search implements provider.Provider: the movie's titles are searched,
// as Bazarr's list_subtitles.
func (p *Provider) Search(ctx context.Context, v *provider.Video, langs []provider.Language) ([]provider.Subtitle, error) {
	if v.Episode || !slices.ContainsFunc(langs, func(l provider.Language) bool { return l.Tag == "lv" && !l.Forced }) {
		return nil, nil
	}
	var out []provider.Subtitle
	for _, title := range append([]string{v.Title}, v.AlternativeTitles...) {
		subs, err := p.query(ctx, v, title)
		if err != nil {
			return nil, err
		}
		out = append(out, subs...)
	}
	return out, nil
}

// query searches a title, reading each result's page, as Bazarr's query.
func (p *Provider) query(ctx context.Context, v *provider.Video, title string) ([]provider.Subtitle, error) {
	u, _ := p.resolve("search/")
	r, err := p.s.Get(ctx, u, url.Values{"q": {title}})
	if err != nil {
		return nil, err
	}
	if len(r.Body) == 0 {
		p.env.Log.DebugContext(ctx, "no data returned from provider", "provider", Name)
		return nil, nil
	}
	doc, err := r.HTML()
	if err != nil {
		return nil, provider.Errorf(provider.ParseResponseError, "search: %w", err)
	}
	var out []provider.Subtitle
	for _, row := range doc.All(".eBlock") {
		a := row.First(".eTitle > a")
		if !a.Valid() {
			return nil, provider.Errorf(provider.ParseResponseError, "search: a result without a link")
		}
		pageLink, err := p.resolve(a.Attr("href"))
		if err != nil {
			return nil, err
		}
		sub, err := p.page(ctx, v, pageLink)
		if err != nil {
			return nil, err
		}
		out = append(out, sub)
	}
	return out, nil
}

// page reads a subtitle's page: the movie's title, year and IMDb ID, and
// the download link.
func (p *Provider) page(ctx context.Context, v *provider.Video, pageLink string) (provider.Subtitle, error) {
	r, err := p.s.Get(ctx, pageLink, nil)
	if err != nil {
		return provider.Subtitle{}, err
	}
	doc, err := r.HTML()
	if err != nil {
		return provider.Subtitle{}, provider.Errorf(provider.ParseResponseError, "%s: %w", pageLink, err)
	}
	header, yearEl, imdb, dl := doc.First(".main-header"), doc.First("#film-page-year"), doc.First("#actors-page > a"), doc.First(".hvr")
	if !header.Valid() || !yearEl.Valid() || !imdb.Valid() || !dl.Valid() {
		return provider.Subtitle{}, provider.Errorf(provider.ParseResponseError, "%s: not a subtitle's page", pageLink)
	}
	titles := strings.Split(header.TrimmedText(), " / ")
	title := titles[len(titles)-1]
	year, _ := strconv.Atoi(yearEl.TrimmedText())
	parts := strings.Split(imdb.Attr("href"), "/")
	imdbID := ""
	if len(parts) >= 2 {
		imdbID = parts[len(parts)-2]
	}

	var matches []string
	if v.Title != "" && sanitize(title) == sanitize(v.Title) {
		matches = append(matches, "title")
	}
	if v.Year != 0 && year == v.Year {
		matches = append(matches, "year")
	}
	if id := v.ExternalIDs["imdb"]; id != "" && imdbID == id {
		matches = append(matches, "imdb_id")
	}
	return provider.Subtitle{
		// The download link is the site's path, as Bazarr's server_url + href.
		ID:       dl.Attr("href"),
		Language: provider.Language{Tag: "lv"},
		Matches:  matches,
		PageLink: pageLink,
	}, nil
}

// Download implements provider.Provider: the subtitle of the archive, or
// the subtitle file served.
func (p *Provider) Download(ctx context.Context, v *provider.Video, id string) (provider.File, error) {
	link, err := p.resolve(id)
	if err != nil {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "malformed subtitle ID %q", id)
	}
	r, err := p.s.Get(ctx, link, nil)
	if err != nil {
		return provider.File{}, err
	}
	lang := provider.Language{Tag: "lv"}
	if _, err := archive.Open(r.Body); errors.Is(err, archive.ErrNotArchive) {
		return provider.File{}, provider.Errorf(provider.ProviderError, "unidentified archive type")
	}
	return archive.Extract(r.Body, archive.WantFor(v, lang), lang)
}

var (
	replaced = regexp.MustCompile(`[-:().]`)
	removed  = regexp.MustCompile(`'`)
	spaces   = regexp.MustCompile(`\s+`)
)

// sanitize strips a title's special characters, as subliminal's
// sanitize.
func sanitize(s string) string {
	s = replaced.ReplaceAllString(s, " ")
	s = removed.ReplaceAllString(s, "")
	return strings.ToLower(strings.TrimSpace(spaces.ReplaceAllString(s, " ")))
}

// String names the provider in logs.
func (p *Provider) String() string { return fmt.Sprintf("%s(%s)", Name, p.base.Host) }
