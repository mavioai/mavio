// Package bayflix is the provider of Bayflix, ported from Bazarr's
// bayflix: Bulgarian subtitles of movies and episodes found by title in
// the site's JSON API, served in archives.
package bayflix

import (
	"context"
	"encoding/json"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/mavioai/mavio/plugins/subtitles/internal/archive"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/site"
)

// Name is the provider's name.
const Name = "bayflix"

// language is the provider's only language.
const language = "bg"

// Definition returns the provider's definition.
func Definition() provider.Definition {
	return provider.Definition{
		Name: Name, DisplayName: "Bayflix",
		Languages: []string{language},
		Movies:    true, Episodes: true,
		New: New,
	}
}

// New makes the provider; it takes no settings.
func New(env provider.Env, _ json.RawMessage) (provider.Provider, error) {
	p := &Provider{env: env, s: site.New(env.HTTP)}
	p.SetBaseURL(&url.URL{Scheme: "https", Host: "bayflix.sb"})
	return p, nil
}

// Provider is Bayflix.
type Provider struct {
	env  provider.Env
	s    *site.Session
	base string
}

// SetBaseURL points the provider at another server, for tests.
func (p *Provider) SetBaseURL(u *url.URL) {
	p.base = strings.TrimSuffix(u.String(), "/")
	p.s.Header.Set("Accept", "application/json, text/plain, */*")
	p.s.Header.Set("Accept-Language", "en-US,en;q=0.9")
	p.s.Header.Set("Referer", p.base+"/")
}

func (p *Provider) downloadURL(id string) string {
	return p.base + "/api/subtitles/download/" + url.PathEscape(id)
}

var (
	seasonEpisode = regexp.MustCompile(`\b[Ss](\d{1,2})[Ee](\d{1,2})\b`)
	crossEpisode  = regexp.MustCompile(`\b(\d{1,2})x(\d{1,2})\b`)
)

// episodeOf returns the season and episode a text names, as Bazarr's
// _episode_tuple.
func episodeOf(text string) (season, episode int, ok bool) {
	m := seasonEpisode.FindStringSubmatch(text)
	if m == nil {
		m = crossEpisode.FindStringSubmatch(text)
	}
	if m == nil {
		return 0, 0, false
	}
	s, _ := strconv.Atoi(m[1])
	e, _ := strconv.Atoi(m[2])
	return s, e, true
}

// item is a subtitle of the site's search.
type item struct {
	ID           json.RawMessage `json:"_id"`
	Title        string          `json:"title"`
	Description  string          `json:"description"`
	ReleaseName  json.RawMessage `json:"release_name"`
	ReleaseDate  string          `json:"release_date"`
	MediaType    string          `json:"media_type"`
	SubtitleLink string          `json:"subtitle_link"`
}

// releases returns the item's release names, a list or a single one.
func (it *item) releases() []string {
	var list []string
	if json.Unmarshal(it.ReleaseName, &list) == nil {
		return list
	}
	var one string
	if json.Unmarshal(it.ReleaseName, &one) == nil && one != "" {
		return []string{one}
	}
	return nil
}

// matchesEpisode reports whether an item has the episode, as Bazarr's
// _matches_episode: one of its releases or a line of its description
// names it.
func (it *item) matchesEpisode(v *provider.Video) bool {
	if !v.Episode {
		return true
	}
	for _, text := range append(it.releases(), strings.Split(it.Description, "\n")...) {
		if s, e, ok := episodeOf(text); ok && s == v.Season && e == v.Number {
			return true
		}
	}
	return false
}

// year returns the year of the item's release date; 0 for none.
func (it *item) year() (int, bool) {
	y := it.ReleaseDate
	if len(y) > 4 {
		y = y[:4]
	}
	if y == "" {
		return 0, false
	}
	n, err := strconv.Atoi(y)
	return n, err == nil
}

// matchesYear reports whether an item is of the movie's year, give or
// take one, as Bazarr's _matches_movie_year.
func (it *item) matchesYear(v *provider.Video) bool {
	if v.Episode || v.Year == 0 {
		return true
	}
	y, ok := it.year()
	return !ok || y-v.Year <= 1 && v.Year-y <= 1
}

// idString returns an ID that is a string or a number; "" for none.
func idString(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var n json.Number
	if json.Unmarshal(raw, &n) == nil {
		return n.String()
	}
	return ""
}

// Search implements provider.Provider.
func (p *Provider) Search(ctx context.Context, v *provider.Video, langs []provider.Language) ([]provider.Subtitle, error) {
	title := strings.TrimSpace(v.Title)
	if v.Episode {
		title = strings.TrimSpace(v.Series)
	}
	var out []provider.Subtitle
	for _, l := range langs {
		if l.Tag != language {
			continue
		}
		p.env.Log.DebugContext(ctx, "searching Bayflix subtitles", "title", title)
		r, err := p.s.Get(ctx, p.base+"/api/subtitles/search", url.Values{"title": {title}})
		if err != nil {
			return nil, err
		}
		var items []item
		if err := r.JSON(&items); err != nil {
			return nil, err
		}
		for _, it := range items {
			if !it.matchesYear(v) || !it.matchesEpisode(v) {
				continue
			}
			fileID := idString(it.ID)
			if fileID == "" {
				continue
			}
			// The ID is the site's, unless the subtitle has a link of its own.
			id, link := fileID, p.downloadURL(fileID)
			if it.SubtitleLink != "" {
				id, link = it.SubtitleLink, it.SubtitleLink
			}
			names := it.releases()
			if it.Title != "" {
				names = append(names, it.Title)
			}
			sub := provider.Subtitle{ID: id, Language: l, ReleaseNames: names, PageLink: link}
			if y, ok := it.year(); ok && !v.Episode && v.Year != 0 && y == v.Year {
				sub.Matches = []string{"year"}
			}
			out = append(out, sub)
		}
	}
	return out, nil
}

// Download implements provider.Provider: id is the site's, or a link.
func (p *Provider) Download(ctx context.Context, v *provider.Video, id string) (provider.File, error) {
	link := id
	if !strings.Contains(id, "://") {
		link = p.downloadURL(id)
	}
	r, err := p.s.Get(ctx, link, nil)
	if err != nil {
		return provider.File{}, err
	}
	l := provider.Language{Tag: language}
	w := archive.WantFor(v, l)
	w.First = v == nil || !v.Episode
	return archive.Extract(r.Body, w, l)
}

// String names the provider in logs.
func (p *Provider) String() string { return Name }
