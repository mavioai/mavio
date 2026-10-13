// Package greeksubtitles is the provider of GreekSubtitles, ported from
// Bazarr's greeksubtitles: Greek and English subtitles of movies and
// episodes, found by the site's search of titles and served in archives.
package greeksubtitles

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/mavioai/mavio/plugins/subtitles/internal/archive"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/site"
)

// Name is the provider's name.
const Name = "greeksubtitles"

// maxPages bounds the pages of results a search follows.
const maxPages = 50

// extensions are subliminal's SUBTITLE_EXTENSIONS.
var extensions = []string{".srt", ".sub", ".smi", ".txt", ".ssa", ".ass", ".mpl"}

// Definition returns the provider's definition.
func Definition() provider.Definition {
	return provider.Definition{
		Name: Name, DisplayName: "GreekSubtitles",
		Languages: []string{"el", "en"},
		Movies:    true, Episodes: true,
		New: New,
	}
}

// New makes the provider; it has no settings.
func New(env provider.Env, _ json.RawMessage) (provider.Provider, error) {
	return &Provider{
		env: env, s: site.New(env.HTTP),
		server: "http://gr.greek-subtitles.com/", download: "http://www.greeksubtitles.info/getp.php",
	}, nil
}

// Provider is GreekSubtitles.
type Provider struct {
	env              provider.Env
	s                *site.Session
	server, download string
}

// SetBaseURL points the provider's search and downloads at another
// server, for tests.
func (p *Provider) SetBaseURL(u *url.URL) {
	base := strings.TrimSuffix(u.String(), "/")
	p.server, p.download = base+"/", base+"/getp.php"
}

// Search implements provider.Provider: each of the video's titles is
// searched, an episode's with its number, else with the video's year.
func (p *Provider) Search(ctx context.Context, v *provider.Video, langs []provider.Language) ([]provider.Subtitle, error) {
	names := append([]string{v.Title}, v.AlternativeTitles...)
	if v.Episode {
		names = append([]string{v.Series}, v.AlternativeSeries...)
	}
	var out []provider.Subtitle
	for _, name := range names {
		keyword := name
		switch {
		case v.Episode && v.Season != 0 && v.Number != 0:
			keyword += fmt.Sprintf(" S%02dE%02d", v.Season, v.Number)
		case v.Year != 0:
			keyword += fmt.Sprintf(" %4d", v.Year)
		}
		subs, err := p.query(ctx, v, keyword)
		if err != nil {
			return nil, err
		}
		for _, s := range subs {
			if len(langs) == 0 || slices.ContainsFunc(langs, func(l provider.Language) bool { return l.Tag == s.Language.Tag }) {
				out = append(out, s)
			}
		}
	}
	return out, nil
}

// query lists the results of a search, following its pages, as Bazarr's
// query.
func (p *Provider) query(ctx context.Context, v *provider.Video, keyword string) ([]provider.Subtitle, error) {
	link := p.server + "search.php?" + url.Values{"name": {keyword}}.Encode()
	var out []provider.Subtitle
	for range maxPages {
		r, err := p.s.Get(ctx, link, nil)
		if err != nil {
			return nil, err
		}
		if len(r.Body) == 0 {
			return nil, nil
		}
		doc, err := site.Parse(r.Body, "text/html; charset=utf-8")
		if err != nil {
			return nil, provider.Errorf(provider.ParseResponseError, "search: %w", err)
		}
		for _, a := range doc.All("td.latest_name > a:nth-of-type(1)") {
			href := a.Attr("href")
			parts := strings.Split(href, "/")
			if len(parts) < 3 {
				continue
			}
			id, err := strconv.Atoi(parts[len(parts)-2])
			if err != nil {
				continue
			}
			src := a.Parent().First("img").Attr("src")
			tag := provider.FromCode(strings.SplitN(path.Base(src), ".", 2)[0])
			if tag == "" {
				continue
			}
			var matches []string
			if v.Episode {
				// As Bazarr's, which matches the year of episodes.
				matches = append(matches, "year")
			}
			out = append(out, provider.Subtitle{
				ID:           url.Values{"id": {strconv.Itoa(id)}, "page": {href}, "lang": {tag}}.Encode(),
				Language:     provider.Language{Tag: tag},
				ReleaseNames: []string{a.TrimmedText()},
				Matches:      matches,
				PageLink:     href,
			})
		}
		next := ""
		for _, a := range doc.All("td a") {
			if strings.Contains(a.Text(), "Next") && strings.Contains(a.Attr("href"), "search.php") {
				next = p.server + a.Attr("href")
				break
			}
		}
		if next == "" || next == link {
			break
		}
		link = next
	}
	return out, nil
}

// Download implements provider.Provider: the first subtitle file of the
// archive the site serves.
func (p *Provider) Download(ctx context.Context, _ *provider.Video, id string) (provider.File, error) {
	q, err := url.ParseQuery(id)
	if err != nil || q.Get("id") == "" {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "malformed subtitle ID %q", id)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.download+"?"+url.Values{"id": {q.Get("id")}}.Encode(), nil)
	if err != nil {
		return provider.File{}, err
	}
	req.Header.Set("Referer", q.Get("page"))
	r, err := p.s.Do(req)
	if err != nil {
		return provider.File{}, err
	}
	if len(r.Body) == 0 {
		return provider.File{}, provider.Errorf(provider.ProviderError, "unable to download the subtitle: no data returned from provider")
	}
	a, err := archive.Open(r.Body)
	if err != nil {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "%w", err)
	}
	for _, name := range a.Names() {
		ext := strings.ToLower(path.Ext(name))
		if strings.HasPrefix(path.Base(name), ".") || !slices.Contains(extensions, ext) {
			continue
		}
		data, err := a.Read(name)
		if err != nil {
			return provider.File{}, provider.Errorf(provider.BadSubtitle, "%s: %w", name, err)
		}
		format := strings.TrimPrefix(ext, ".")
		if !slices.Contains([]string{"srt", "ass", "ssa"}, format) {
			format = archive.Detect(data)
		}
		return provider.File{Data: archive.FixLineEndings(data), Format: format, Language: provider.Language{Tag: q.Get("lang")}}, nil
	}
	return provider.File{}, provider.Errorf(provider.BadSubtitle, "no subtitle in the archive")
}
