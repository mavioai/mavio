// Package subtitulamostv is the provider of Subtitulamos.tv, ported from
// Bazarr's subtitulamostv: English and Spanish subtitles of episodes,
// found through the pages of the show's season and episode.
package subtitulamostv

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/mavioai/mavio/plugins/subtitles/internal/archive"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/site"
)

// Name is the provider's name.
const Name = "subtitulamostv"

// serverURL is the site's.
const serverURL = "https://www.subtitulamos.tv/"

// userAgent is Bazarr's default SZ_USER_AGENT.
const userAgent = "Sub-Zero/2"

// Definition returns the provider's definition.
func Definition() provider.Definition {
	return provider.Definition{
		Name: Name, DisplayName: "Subtitulamos.tv",
		Languages: []string{"en", "es"},
		Episodes:  true,
		New:       New,
	}
}

// New makes the provider; it has no settings.
func New(env provider.Env, _ json.RawMessage) (provider.Provider, error) {
	base, _ := url.Parse(serverURL)
	s := site.New(env.HTTP)
	s.Header.Set("User-Agent", userAgent)
	return &Provider{env: env, s: s, base: base}, nil
}

// Provider is Subtitulamos.tv.
type Provider struct {
	env  provider.Env
	s    *site.Session
	base *url.URL
}

// SetBaseURL points the provider at another server, for tests.
func (p *Provider) SetBaseURL(u *url.URL) { p.base = u }

func (p *Provider) resolve(ref string) (string, error) {
	r, err := url.Parse(ref)
	if err != nil {
		return "", provider.Errorf(provider.ParseResponseError, "link %q: %w", ref, err)
	}
	return p.base.ResolveReference(r).String(), nil
}

// Search implements provider.Provider, as Bazarr's query: the shows of
// the series' name, the page of the season, then of the episode, whose
// releases with a download link are the subtitles.
func (p *Provider) Search(ctx context.Context, v *provider.Video, langs []provider.Language) ([]provider.Subtitle, error) {
	if !v.Episode {
		return nil, nil
	}
	u, _ := p.resolve("search/query")
	r, err := p.s.Get(ctx, u, url.Values{"q": {v.Series}})
	if err != nil {
		return nil, err
	}
	var shows []struct {
		ShowID   int    `json:"show_id"`
		ShowName string `json:"show_name"`
	}
	if err := r.JSON(&shows); err != nil {
		return nil, err
	}
	var out []provider.Subtitle
	for _, show := range shows {
		if !strings.EqualFold(v.Series, show.ShowName) {
			continue
		}
		u, _ := p.resolve("shows/" + strconv.Itoa(show.ShowID))
		r, err := p.s.Get(ctx, u, nil)
		if err != nil {
			return nil, err
		}
		r, ok, err := p.choose(ctx, r, "#season-choices a", v.Season)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		r, ok, err = p.choose(ctx, r, "#episode-choices a", v.Number)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		doc, err := r.HTML()
		if err != nil {
			return nil, provider.Errorf(provider.ParseResponseError, "episode: %w", err)
		}
		episodeURL := r.URL.String()
		for _, lang := range doc.All("div.language-container") {
			name := lang.First("div.language-name").Text()
			var tag string
			switch {
			case strings.Contains(name, "English"):
				tag = "en"
			case strings.Contains(name, "Español"):
				tag = "es"
			default:
				continue
			}
			if !slices.ContainsFunc(langs, func(l provider.Language) bool { return l.Tag == tag && !l.Forced }) {
				continue
			}
			for _, release := range lang.All("div.version-container") {
				links := release.All(`a[href*="/download"]`)
				if len(links) != 1 {
					// An incomplete translation has no download link.
					continue
				}
				ps := release.All("div.version-container p")
				if len(ps) < 2 {
					return nil, provider.Errorf(provider.ParseResponseError, "episode: a release without a name")
				}
				name := ps[1].Text()
				// The site's pages are those of the episode, as Bazarr's
				// get_matches.
				matches := []string{"series", "season", "episode", "year", "title"}
				if v.ReleaseGroup != "" && strings.Contains(strings.ToLower(name), strings.ToLower(v.ReleaseGroup)) {
					matches = append(matches, "release_group")
				}
				out = append(out, provider.Subtitle{
					ID:           links[0].Attr("href"),
					Language:     provider.Language{Tag: tag},
					ReleaseNames: []string{name},
					Matches:      matches,
					PageLink:     episodeURL,
				})
			}
		}
	}
	return out, nil
}

// choose follows the link of a season or an episode, unless its page is
// the one selected already; ok reports whether the number has a link.
func (p *Provider) choose(ctx context.Context, r *site.Response, selector string, number int) (*site.Response, bool, error) {
	doc, err := r.HTML()
	if err != nil {
		return nil, false, provider.Errorf(provider.ParseResponseError, "%s: %w", r.URL.Path, err)
	}
	for _, a := range doc.All(selector) {
		if strings.TrimSpace(a.Text()) != strconv.Itoa(number) {
			continue
		}
		if slices.Contains(strings.Fields(a.Attr("class")), "selected") {
			return r, true, nil
		}
		u, err := p.resolve(a.Attr("href"))
		if err != nil {
			return nil, false, err
		}
		next, err := p.s.Get(ctx, u, nil)
		if err != nil {
			return nil, false, err
		}
		return next, true, nil
	}
	return nil, false, nil
}

// Download implements provider.Provider.
func (p *Provider) Download(ctx context.Context, _ *provider.Video, id string) (provider.File, error) {
	link, err := p.resolve(id)
	if err != nil {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "malformed subtitle ID %q", id)
	}
	r, err := p.s.Get(ctx, link, nil)
	if err != nil {
		return provider.File{}, err
	}
	if len(r.Body) == 0 {
		return provider.File{}, provider.Errorf(provider.ProviderError, "could not download subtitle from %s", link)
	}
	data := archive.FixLineEndings(r.Body)
	return provider.File{Data: data, Format: archive.Detect(data)}, nil
}

// String names the provider in logs.
func (p *Provider) String() string { return fmt.Sprintf("%s(%s)", Name, p.base.Host) }
