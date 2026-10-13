// Package subsro is the provider of Subs.ro, ported from Bazarr's subsro:
// Romanian and English subtitles found by IMDb ID through the API, with
// API keys taken in turn as they hit their rate limits.
package subsro

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/mavioai/mavio/plugins/subtitles/internal/archive"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/site"
)

// Name is the provider's name.
const Name = "subsro"

// Definition returns the provider's definition.
func Definition() provider.Definition {
	return provider.Definition{
		Name: Name, DisplayName: "Subs.ro",
		Settings: []provider.Setting{
			{
				Name: "api_key", Label: "API Key", Type: provider.Text,
				Help: "API key required. Get your key from Subs.ro website: https://subs.ro/api; several keys separated by commas are taken in turn.",
			},
		},
		Languages: []string{"ro", "en"},
		Movies:    true, Episodes: true,
		New: New,
	}
}

type settings struct {
	APIKey string `json:"api_key"`
}

// New makes the provider from its settings.
func New(env provider.Env, raw json.RawMessage) (provider.Provider, error) {
	var s settings
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, provider.Errorf(provider.ConfigurationError, "settings: %w", err)
		}
	}
	var keys []string
	for k := range strings.SplitSeq(s.APIKey, ",") {
		if k = strings.TrimSpace(k); k != "" {
			keys = append(keys, k)
		}
	}
	if len(keys) == 0 {
		return nil, provider.Errorf(provider.ConfigurationError, "SubsRo requires an API Key")
	}
	sess := site.New(env.HTTP)
	sess.Header.Set("Accept", "application/json")
	sess.Check = func(*site.Response) error { return nil }
	return &Provider{env: env, sess: sess, keys: keys, base: "https://api.subs.ro/v1.0"}, nil
}

// Provider is Subs.ro.
type Provider struct {
	env  provider.Env
	sess *site.Session
	keys []string
	base string

	mu sync.Mutex
	// key is the index of the key in use.
	key int
}

// SetBaseURL points the provider at another server, for tests.
func (p *Provider) SetBaseURL(u *url.URL) { p.base = strings.TrimSuffix(u.String(), "/") + "/v1.0" }

// request makes a request with the key in use, as Bazarr's _request: a
// rate limit takes the next key, until every key has hit it.
func (p *Provider) request(ctx context.Context, u string, query url.Values) (*site.Response, error) {
	for range p.keys {
		if len(query) > 0 {
			u += "?" + query.Encode()
			query = nil
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, provider.Errorf(provider.ProviderError, "%w", err)
		}
		p.mu.Lock()
		index := p.key
		req.Header.Set("X-Subs-Api-Key", p.keys[index])
		p.mu.Unlock()
		r, err := p.sess.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, provider.Errorf(provider.APIThrottled, "SubsRo request failed: %w", err)
		}
		switch {
		case r.Status == http.StatusTooManyRequests:
			p.env.Log.WarnContext(ctx, "SubsRo: API key hit the rate limit", "key", index+1)
			p.mu.Lock()
			if p.key == index {
				p.key = (index + 1) % len(p.keys)
			}
			p.mu.Unlock()
			continue
		case r.Status == http.StatusUnauthorized:
			return nil, provider.Errorf(provider.AuthenticationError, "SubsRo: invalid API key (key #%d)", index+1)
		case r.Status >= 500:
			return nil, provider.Errorf(provider.APIThrottled, "SubsRo: server error %d", r.Status)
		}
		return r, nil
	}
	return nil, provider.Errorf(provider.TooManyRequests, "SubsRo: too many requests (all keys exhausted)")
}

var (
	titleSeason   = regexp.MustCompile(`[Ss]ezon(?:ul)?\s*(\d{1,2})`)
	releaseSeason = regexp.MustCompile(`[Ss]ezon(?:ul)?\s*(\d{1,2})|[Ss](\d{1,2})[Ee]\d+`)
)

type item struct {
	ID           json.RawMessage `json:"id"`
	Title        string          `json:"title"`
	Year         json.RawMessage `json:"year"`
	Description  string          `json:"description"`
	DownloadLink string          `json:"downloadLink"`
}

// Search implements provider.Provider, as Bazarr's list_subtitles: the
// subtitles of the movie's, or the series', IMDb ID in each language.
func (p *Provider) Search(ctx context.Context, v *provider.Video, langs []provider.Language) ([]provider.Subtitle, error) {
	imdb := v.IMDbID()
	if len(imdb) <= 2 {
		return nil, nil
	}
	imdb = imdb[2:]
	var out []provider.Subtitle
	for _, l := range langs {
		code := "en"
		if l.Tag == "ro" {
			code = "ro"
		}
		r, err := p.request(ctx, p.base+"/search/imdbid/"+url.PathEscape(imdb), url.Values{"language": {code}})
		if err != nil {
			return nil, err
		}
		var res struct {
			Status json.RawMessage `json:"status"`
			Items  []item          `json:"items"`
		}
		if err := json.Unmarshal(r.Body, &res); err != nil {
			p.env.Log.ErrorContext(ctx, "SubsRo: invalid JSON response", "err", err)
			continue
		}
		if string(res.Status) != "200" {
			p.env.Log.WarnContext(ctx, "SubsRo API returned an error status", "status", string(res.Status))
			continue
		}
		for _, it := range res.Items {
			if it.DownloadLink == "" || it.Title == "" {
				continue
			}
			season := -1
			if m := titleSeason.FindStringSubmatch(it.Title); m != nil {
				season, _ = strconv.Atoi(m[1])
			} else if m := releaseSeason.FindStringSubmatch(it.Description); m != nil {
				season, _ = strconv.Atoi(m[1] + m[2])
			}
			out = append(out, subtitle(v, l.Tag, "tt"+imdb, season, it))
		}
	}
	return out, nil
}

func subtitle(v *provider.Video, tag, imdb string, season int, it item) provider.Subtitle {
	year := number(it.Year)
	var m []string
	if v.Year > 0 && year == v.Year {
		m = append(m, "year")
	}
	// The release names are what Bazarr guesses the matches from.
	var guess string
	if v.Episode {
		if v.Series != "" {
			m = append(m, "series")
		}
		if v.SeriesExternalIDs["imdb"] == imdb {
			m = append(m, "imdb_id")
		}
		if season == v.Season {
			m = append(m, "season")
		}
		if v.SeriesExternalIDs["imdb"] == imdb && season == v.Season {
			m = append(m, "episode")
		}
		guess = fmt.Sprintf("%s %s %s", it.Title, orEmpty(year), it.Description)
	} else {
		if v.Title != "" {
			m = append(m, "title")
		}
		if v.ExternalIDs["imdb"] == imdb {
			m = append(m, "imdb_id")
		}
		guess = fmt.Sprintf("%s %s %s %s", it.Title, orEmpty(season), orEmpty(year), it.Description)
	}
	return provider.Subtitle{
		ID:           it.DownloadLink,
		Language:     provider.Language{Tag: tag},
		ReleaseNames: []string{strings.Join(strings.Fields(guess), " ")},
		Matches:      m,
		PageLink:     it.DownloadLink,
		Comment:      it.Description,
	}
}

func orEmpty(n int) string {
	if n <= 0 {
		return ""
	}
	return strconv.Itoa(n)
}

// number reads a number the site may quote; -1 for none.
func number(raw json.RawMessage) int {
	n, err := strconv.Atoi(strings.Trim(string(raw), `"`))
	if err != nil {
		return -1
	}
	return n
}

// Download implements provider.Provider: the subtitle, or the video's
// of an archive.
func (p *Provider) Download(ctx context.Context, v *provider.Video, id string) (provider.File, error) {
	r, err := p.request(ctx, id, nil)
	if err != nil {
		return provider.File{}, err
	}
	w := archive.Want{}
	if v != nil && v.Episode {
		w.Episode = v.Number
	}
	return archive.Extract(r.Body, w, provider.Language{})
}
