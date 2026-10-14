// Package wizdom is the provider of Wizdom, ported from Bazarr's wizdom:
// Hebrew subtitles of movies and episodes found by IMDb ID, which TMDB
// gives when the video has none, served in zip archives.
package wizdom

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
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/archive"
	"github.com/mavioai/mavio/plugins/subtitles/internal/cache"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/site"
)

// Name is the provider's name.
const Name = "wizdom"

// language is the provider's only language.
const language = "he"

// tmdbKey is the TMDB API key of Bazarr's provider.
const tmdbKey = "a51ee051bcd762543373903de296e0a3"

// imdbExpiry is how long IMDb IDs found are cached, as subliminal's
// SHOW_EXPIRATION_TIME.
const imdbExpiry = 3 * 7 * 24 * time.Hour

// Definition returns the provider's definition.
func Definition() provider.Definition {
	return provider.Definition{
		Name: Name, DisplayName: "Wizdom",
		Languages: []string{language},
		Movies:    true, Episodes: true,
		New: New,
	}
}

// New makes the provider; it takes no settings.
func New(env provider.Env, _ json.RawMessage) (provider.Provider, error) {
	s := site.New(env.HTTP)
	s.Header.Set("User-Agent", "Subliminal/2.1")
	return &Provider{
		env: env, s: s,
		secure: "https://wizdom.xyz", plain: "http://wizdom.xyz", tmdb: "http://api.tmdb.org",
		cache: cache.New(env),
	}, nil
}

// Provider is Wizdom.
type Provider struct {
	env provider.Env
	s   *site.Session
	// secure and plain are the site's URLs over HTTPS and HTTP, as Bazarr
	// reaches the API over the one and pages and files over the other.
	secure, plain, tmdb string
	// cache keeps the IMDb IDs of titles, as subliminal's region cache.
	cache *cache.Region
}

// SetBaseURL points the provider, TMDB included, at another server, for
// tests.
func (p *Provider) SetBaseURL(u *url.URL) {
	base := strings.TrimSuffix(u.String(), "/")
	p.secure, p.plain, p.tmdb = base, base, base
}

// Search implements provider.Provider, as Bazarr's list_subtitles: the
// subtitles of the first of the video's titles that has some.
func (p *Provider) Search(ctx context.Context, v *provider.Video, langs []provider.Language) ([]provider.Subtitle, error) {
	if !slices.ContainsFunc(langs, func(l provider.Language) bool { return l.Tag == language }) {
		return nil, nil
	}
	titles := append([]string{v.Title}, v.AlternativeTitles...)
	imdb := v.ExternalIDs["imdb"]
	if v.Episode {
		titles = append([]string{v.Series}, v.AlternativeSeries...)
		imdb = v.SeriesExternalIDs["imdb"]
	}
	for _, title := range titles {
		subs, err := p.query(ctx, v, title, imdb)
		if err != nil {
			return nil, err
		}
		if len(subs) > 0 {
			return subs, nil
		}
	}
	return nil, nil
}

// query lists the subtitles of a title, as Bazarr's query.
func (p *Provider) query(ctx context.Context, v *provider.Video, title, imdb string) ([]provider.Subtitle, error) {
	movie := !v.Episode || v.Season == 0 || v.Number == 0
	if imdb == "" {
		var err error
		if imdb, err = p.searchIMDbID(ctx, title, v.Year, movie); err != nil {
			return nil, err
		}
		if imdb == "" {
			return nil, nil
		}
	}
	p.env.Log.DebugContext(ctx, "using IMDb ID", "imdb", imdb)
	kind := "series"
	if movie {
		kind = "movies"
	}
	pageLink := fmt.Sprintf("%s/%s/%s", p.plain, kind, imdb)

	r, err := p.s.Get(ctx, p.secure+"/api/releases/"+url.PathEscape(imdb), nil)
	if r != nil && r.Status == http.StatusInternalServerError {
		p.env.Log.DebugContext(ctx, "no subtitles found", "imdb", imdb)
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var res struct {
		Subs json.RawMessage `json:"subs"`
	}
	if json.Unmarshal(r.Body, &res) != nil {
		return nil, nil
	}
	results := releases(res.Subs, movie, v.Season, v.Number)

	seen := map[string]bool{}
	var out []provider.Subtitle
	for _, rel := range results {
		id := idString(rel.ID)
		if seen[id] {
			continue
		}
		seen[id] = true
		matches := []string{"title"}
		if v.Episode {
			// The title searched for is one of the series'.
			matches = []string{"series"}
			if v.Season != 0 {
				matches = append(matches, "season")
			}
			if v.Number != 0 {
				matches = append(matches, "episode")
			}
			if s := v.SeriesExternalIDs["imdb"]; s != "" && s == imdb {
				matches = append(matches, "series_imdb_id")
			}
		}
		out = append(out, provider.Subtitle{
			ID: id, Language: provider.Language{Tag: language}, ReleaseNames: []string{rel.Version},
			Matches: matches, PageLink: pageLink,
		})
	}
	return out, nil
}

// release is a subtitle of the site.
type release struct {
	ID      json.RawMessage `json:"id"`
	Version string          `json:"version"`
}

// releases returns the subtitles of a movie, or of an episode from the
// seasons of a series: a list indexed by season or an object keyed by
// it, of objects keyed by episode.
func releases(subs json.RawMessage, movie bool, season, episode int) []release {
	var out []release
	if movie {
		_ = json.Unmarshal(subs, &out)
		return out
	}
	var seasons []json.RawMessage
	var bySeason json.RawMessage
	if json.Unmarshal(subs, &seasons) == nil {
		if season < len(seasons) {
			bySeason = seasons[season]
		}
	} else {
		var m map[string]json.RawMessage
		if json.Unmarshal(subs, &m) != nil {
			return nil
		}
		bySeason = m[strconv.Itoa(season)]
	}
	var episodes map[string][]release
	if json.Unmarshal(bySeason, &episodes) != nil {
		return nil
	}
	return episodes[strconv.Itoa(episode)]
}

// idString returns an ID that is a string or a number.
func idString(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	return string(raw)
}

// searchIMDbID returns the IMDb ID of a title, as Bazarr's
// _search_imdb_id: that of the first movie or series TMDB finds; "" for
// none.
func (p *Provider) searchIMDbID(ctx context.Context, title string, year int, movie bool) (string, error) {
	key := fmt.Sprintf("%s|%d|%t", title, year, movie)
	if id, ok := cache.Get[string](ctx, p.cache, key); ok {
		return id, nil
	}
	p.env.Log.InfoContext(ctx, "searching IMDb ID", "title", title, "year", year)
	category := "tv"
	if movie {
		category = "movie"
	}
	q := url.Values{"api_key": {tmdbKey}, "query": {strings.ReplaceAll(title, "'", "")}, "language": {"en"}}
	if year != 0 {
		q.Set("year", strconv.Itoa(year))
	}
	r, err := p.s.Get(ctx, p.tmdb+"/3/search/"+category, q)
	if err != nil {
		return "", err
	}
	var found struct {
		Results []struct {
			ID json.RawMessage `json:"id"`
		} `json:"results"`
	}
	if err := r.JSON(&found); err != nil {
		return "", err
	}
	id := ""
	if len(found.Results) > 0 {
		if tmdbID := idString(found.Results[0].ID); tmdbID != "" && tmdbID != "null" && tmdbID != "0" {
			u := p.tmdb + "/3/" + category + "/" + tmdbID
			if !movie {
				u += "/external_ids"
			}
			r, err := p.s.Get(ctx, u, url.Values{"api_key": {tmdbKey}, "language": {"en"}})
			if err != nil {
				return "", err
			}
			var ids struct {
				IMDbID *string `json:"imdb_id"`
			}
			if err := r.JSON(&ids); err != nil {
				return "", err
			}
			if ids.IMDbID != nil {
				id = *ids.IMDbID
			}
		}
	}
	cache.Set(ctx, p.cache, key, id, imdbExpiry)
	return id, nil
}

// Download implements provider.Provider: id is the site's.
func (p *Provider) Download(ctx context.Context, _ *provider.Video, id string) (provider.File, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.plain+"/api/files/sub/"+url.PathEscape(id), nil)
	if err != nil {
		return provider.File{}, err
	}
	req.Header.Set("Referer", p.plain+"/")
	r, err := p.s.Do(req)
	if err != nil {
		return provider.File{}, err
	}
	if len(r.Body) == 0 {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "empty download")
	}
	a, err := archive.Open(r.Body)
	if err != nil {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "%w", err)
	}
	var names []string
	for _, n := range a.Names() {
		if ext := path.Ext(n); ext == ".srt" || ext == ".sub" {
			names = append(names, n)
		}
	}
	if len(names) == 0 {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "no subtitle in the archive")
	}
	l := provider.Language{Tag: language}
	data, err := a.Read(names[0])
	if err != nil {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "%s: %w", names[0], err)
	}
	// The site sometimes has the same subtitle in UTF-8 and Windows-1255:
	// the second is taken when the first is not one.
	if archive.Detect(data) == "" && len(names) > 1 {
		if data, err = a.Read(names[1]); err != nil {
			return provider.File{}, provider.Errorf(provider.BadSubtitle, "%s: %w", names[1], err)
		}
	}
	data = archive.FixLineEndings(data)
	return provider.File{Data: data, Format: archive.Detect(data), Language: l}, nil
}

// String names the provider in logs.
func (p *Provider) String() string { return Name }
