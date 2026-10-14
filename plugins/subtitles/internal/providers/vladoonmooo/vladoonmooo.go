// Package vladoonmooo is the provider of Vladoon Mooo, ported from
// Bazarr's vladoonmooo: Bulgarian subtitles of movies and episodes, and
// season packs, found in the site's JSON API, served in archives.
package vladoonmooo

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/mavioai/mavio/plugins/subtitles/internal/archive"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/site"
)

// Name is the provider's name.
const Name = "vladoonmooo"

// language is the provider's only language.
const language = "bg"

// Definition returns the provider's definition.
func Definition() provider.Definition {
	return provider.Definition{
		Name: Name, DisplayName: "Vladoon Mooo",
		Languages: []string{language},
		Movies:    true, Episodes: true,
		New: New,
	}
}

// New makes the provider; it takes no settings.
func New(env provider.Env, _ json.RawMessage) (provider.Provider, error) {
	p := &Provider{env: env, s: site.NewBrowser(env.HTTP)}
	p.SetBaseURL(&url.URL{Scheme: "https", Host: "vladoon.mooo.com"})
	return p, nil
}

// Provider is Vladoon Mooo.
type Provider struct {
	env  provider.Env
	s    *site.Session
	base string
}

// SetBaseURL points the provider at another server, for tests.
func (p *Provider) SetBaseURL(u *url.URL) {
	p.base = strings.TrimSuffix(u.String(), "/") + "/subs"
	p.s.Header.Set("Accept", "application/json")
	p.s.Header.Set("Referer", p.base+"/")
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
	ID            json.RawMessage `json:"id"`
	OriginalTitle string          `json:"original_title"`
	Title         string          `json:"title"`
	Type          string          `json:"type"`
	Season        *int            `json:"season"`
	Episode       *int            `json:"episode"`
	EpisodeType   string          `json:"episode_type"`
	IMDbID        string          `json:"imdb_id"`
	ReleaseNames  json.RawMessage `json:"release_names"`
	UploadedBy    string          `json:"uploaded_by"`
}

// releases returns the item's release names, a list or a single one.
func (it *item) releases() []string {
	var list []string
	if json.Unmarshal(it.ReleaseNames, &list) == nil {
		return list
	}
	var one string
	if json.Unmarshal(it.ReleaseNames, &one) == nil && one != "" {
		return []string{one}
	}
	return nil
}

// matchesMovie reports whether an item is the movie's, as Bazarr's
// _matches_movie: a movie, of its IMDb ID when both have one.
func (it *item) matchesMovie(v *provider.Video) bool {
	if v.Episode {
		return true
	}
	if it.Type != "" && it.Type != "movie" {
		return false
	}
	if imdb := v.ExternalIDs["imdb"]; imdb != "" && it.IMDbID != "" {
		return imdb == it.IMDbID
	}
	return true
}

// matchesEpisode reports whether an item has the episode, as Bazarr's
// _matches_episode: a series of its IMDb ID, of the episode by its
// numbers or a release, or a pack of its season.
func (it *item) matchesEpisode(v *provider.Video) bool {
	if !v.Episode {
		return true
	}
	if it.Type != "" && it.Type != "tv" {
		return false
	}
	if imdb := v.SeriesExternalIDs["imdb"]; imdb != "" && it.IMDbID != "" && imdb != it.IMDbID {
		return false
	}
	seasonOK := it.Season != nil && *it.Season == v.Season
	if seasonOK && it.Episode != nil && *it.Episode == v.Number {
		return true
	}
	for _, r := range it.releases() {
		if s, e, ok := episodeOf(r); ok && s == v.Season && e == v.Number {
			return true
		}
	}
	return it.EpisodeType == "season" && seasonOK
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
	q := strings.TrimSpace(v.Title)
	if v.Episode {
		q = fmt.Sprintf("%s S%02dE%02d", strings.TrimSpace(v.Series), v.Season, v.Number)
	}
	imdb := v.IMDbID()
	var out []provider.Subtitle
	for _, l := range langs {
		if l.Tag != language {
			continue
		}
		p.env.Log.DebugContext(ctx, "searching Vladoon Mooo subtitles", "q", q)
		r, err := p.s.Get(ctx, p.base+"/search-subtitles", url.Values{"q": {q}})
		if err != nil {
			return nil, err
		}
		var res struct {
			Results []item `json:"results"`
		}
		if string(r.Body) != "null" {
			if err := r.JSON(&res); err != nil {
				return nil, err
			}
		}
		for _, it := range res.Results {
			if !it.matchesMovie(v) || !it.matchesEpisode(v) {
				continue
			}
			id := idString(it.ID)
			if id == "" {
				continue
			}
			names := it.releases()
			if title := cmp.Or(it.OriginalTitle, it.Title); title != "" {
				names = append(names, title)
			}
			sub := provider.Subtitle{
				ID: id, Language: l, ReleaseNames: names,
				PageLink: p.downloadURL(id), Uploader: it.UploadedBy,
			}
			// As Bazarr's, a series' IMDb ID matches as imdb_id too.
			if imdb != "" && it.IMDbID == imdb {
				sub.Matches = []string{"imdb_id"}
			}
			out = append(out, sub)
		}
	}
	return out, nil
}

func (p *Provider) downloadURL(id string) string {
	return p.base + "/download/" + url.PathEscape(id)
}

// Download implements provider.Provider: id is the site's.
func (p *Provider) Download(ctx context.Context, v *provider.Video, id string) (provider.File, error) {
	r, err := p.s.Get(ctx, p.downloadURL(id), nil)
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
