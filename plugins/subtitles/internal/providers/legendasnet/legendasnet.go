// Package legendasnet is the provider of Legendas.net, ported from
// Bazarr's legendasnet: Brazilian Portuguese subtitles found by IMDb ID or
// name through the API of an account.
package legendasnet

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/archive"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/site"
)

// Name is the provider's name.
const Name = "legendasnet"

// userAgent identifies the plugin, as Bazarr's SZ_USER_AGENT.
const userAgent = "Mavio/0.1"

// language is the site's only language.
const language = "pt-BR"

// Definition returns the provider's definition.
func Definition() provider.Definition {
	return provider.Definition{
		Name: Name, DisplayName: "Legendas.net",
		Settings: []provider.Setting{
			{Name: "username", Label: "Username", Help: "Brazilian Subtitles Provider", Type: provider.Text},
			{Name: "password", Label: "Password", Type: provider.Password},
		},
		Languages: []string{language},
		Movies:    true, Episodes: true,
		New: New,
	}
}

type settings struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// New makes the provider from its settings.
func New(env provider.Env, raw json.RawMessage) (provider.Provider, error) {
	var s settings
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, provider.Errorf(provider.ConfigurationError, "settings: %w", err)
		}
	}
	if s.Username == "" || s.Password == "" {
		return nil, provider.Errorf(provider.ConfigurationError, "username and password must be specified")
	}
	sess := site.New(env.HTTP)
	sess.Header.Set("User-Agent", userAgent)
	sess.Check = func(*site.Response) error { return nil }
	return &Provider{env: env, s: s, sess: sess, site: "https://legendas.net", sleep: site.Sleep}, nil
}

// Provider is Legendas.net.
type Provider struct {
	env   provider.Env
	s     settings
	sess  *site.Session
	site  string
	sleep func(context.Context, time.Duration) error

	mu    sync.Mutex
	token string
}

// SetBaseURL points the provider at another server, for tests.
func (p *Provider) SetBaseURL(u *url.URL) { p.site = strings.TrimSuffix(u.String(), "/") }

func (p *Provider) api() string { return p.site + "/api/v1/" }

// login opens a session, as Bazarr's login.
func (p *Provider) login(ctx context.Context) error {
	r, err := site.Retry(ctx, p.env, func() (*site.Response, error) {
		return p.checked(ctx, func() (*http.Request, error) {
			return jsonRequest(ctx, http.MethodPost, p.api()+"login", map[string]string{"email": p.s.Username, "password": p.s.Password})
		}, call{login: true})
	})
	if err != nil {
		return err
	}
	var res struct {
		AccessToken string `json:"access_token"`
	}
	_ = json.Unmarshal(r.Body, &res)
	if res.AccessToken == "" {
		return provider.Errorf(provider.ConfigurationError, "access token not found in login response")
	}
	p.mu.Lock()
	p.token = res.AccessToken
	p.mu.Unlock()
	return nil
}

func jsonRequest(ctx context.Context, method, u string, body any) (*http.Request, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, u, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "*/*")
	return req, nil
}

type call struct {
	retry, download, login bool
}

// checked makes a request with the session's token and maps the answers
// to failures, as Bazarr's checked: an expired session logs in once more.
func (p *Provider) checked(ctx context.Context, newReq func() (*http.Request, error), c call) (*site.Response, error) {
	if !c.login {
		p.mu.Lock()
		token := p.token
		p.mu.Unlock()
		if token == "" {
			if err := p.login(ctx); err != nil {
				return nil, err
			}
		}
	}
	p.mu.Lock()
	token := p.token
	p.mu.Unlock()
	req, err := newReq()
	if err != nil {
		return nil, provider.Errorf(provider.ProviderError, "%w", err)
	}
	if token != "" && !c.login {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	r, err := p.sess.Do(req)
	if err != nil {
		return nil, err
	}
	switch code := r.Status; {
	case code == http.StatusNotFound:
		return nil, provider.Errorf(provider.ProviderError, "endpoint not found: %s", r.URL.Path)
	case code == http.StatusTooManyRequests && c.download:
		return nil, provider.Errorf(provider.DownloadLimitExceeded, "daily download limit exceeded")
	case code == http.StatusTooManyRequests:
		return nil, provider.Errorf(provider.APIThrottled, "too many requests")
	case code == http.StatusUnauthorized || code == http.StatusForbidden:
		p.mu.Lock()
		p.token = ""
		p.mu.Unlock()
		switch {
		case c.login:
			return nil, provider.Errorf(provider.ConfigurationError, "invalid username or password")
		case c.retry:
			return nil, provider.Errorf(provider.ConfigurationError, "invalid access token")
		}
		p.env.Log.DebugContext(ctx, "access token invalid, retrying login")
		if err := p.sleep(ctx, time.Second); err != nil {
			return nil, err
		}
		if err := p.login(ctx); err != nil {
			return nil, err
		}
		if err := p.sleep(ctx, time.Second); err != nil {
			return nil, err
		}
		c.retry = true
		return p.checked(ctx, newReq, c)
	case code >= 400:
		return nil, provider.Errorf(provider.ProviderError, "%s: bad status code: %d", r.URL.Path, code)
	}
	return r, nil
}

type item struct {
	ID          json.Number     `json:"id"`
	Path        string          `json:"path"`
	ReleaseName string          `json:"release_name"`
	Uploader    json.RawMessage `json:"uploader"`
	Comment     string          `json:"comment"`
	TMDbID      json.RawMessage `json:"tmdb_id"`
	Season      json.RawMessage `json:"season"`
	Episode     json.RawMessage `json:"episode"`
}

// forced reports whether a subtitle's comment says it is forced.
func (it *item) forced() bool {
	c := strings.ToLower(it.Comment)
	return strings.Contains(c, "forced") || strings.Contains(c, "foreign")
}

// Search implements provider.Provider, as Bazarr's query: by IMDb ID, or
// by name without one.
func (p *Provider) Search(ctx context.Context, v *provider.Video, langs []provider.Language) ([]provider.Subtitle, error) {
	params := map[string]any{"page": 1, "per_page": 25}
	endpoint := "search/movie"
	imdb := v.IMDbID()
	if imdb == "" {
		params["imdb_id"] = nil
	} else {
		params["imdb_id"] = imdb
	}
	if v.Episode {
		endpoint = "search/tv"
		params["tv_episode"], params["tv_season"] = v.Number, v.Season
		if imdb == "" {
			params["name"] = v.Series
		}
	} else if imdb == "" {
		params["name"] = v.Title
	}
	r, err := site.Retry(ctx, p.env, func() (*site.Response, error) {
		return p.checked(ctx, func() (*http.Request, error) {
			return jsonRequest(ctx, http.MethodGet, p.api()+endpoint, params)
		}, call{})
	})
	if err != nil {
		return nil, err
	}
	var res struct {
		Success *bool  `json:"success"`
		Status  *bool  `json:"status"`
		Error   string `json:"error"`
		TVShows []item `json:"tv_shows"`
		Movies  []item `json:"movies"`
	}
	if err := r.JSON(&res); err != nil {
		return nil, err
	}
	if res.Success != nil && !*res.Success || res.Status != nil && !*res.Status {
		p.env.Log.DebugContext(ctx, "search failed", "error", res.Error)
		return nil, nil
	}
	items, page := res.Movies, "legenda"
	if v.Episode {
		items, page = res.TVShows, "tv_legenda"
	}
	var out []provider.Subtitle
	for _, it := range items {
		forced := it.forced()
		if !slices.ContainsFunc(langs, func(l provider.Language) bool { return l.Tag == language && l.Forced == forced }) {
			continue
		}
		var m []string
		if v.Episode {
			m = append(m, "series")
			if number(it.Season) == v.Season {
				m = append(m, "season")
			}
			if number(it.Episode) == v.Number {
				m = append(m, "episode")
			}
			m = append(m, "series_imdb_id")
		} else {
			m = append(m, "title", "imdb_id")
		}
		out = append(out, provider.Subtitle{
			ID:           it.Path,
			Language:     provider.Language{Tag: language, Forced: forced},
			ReleaseNames: []string{it.ReleaseName},
			Matches:      m,
			PageLink:     fmt.Sprintf("%s/%s?movie_id=%s&legenda_id=%s", p.site, page, strings.Trim(string(items[0].TMDbID), `"`), it.ID),
			Uploader:     uploader(it.Uploader),
			Comment:      it.Comment,
		})
	}
	return out, nil
}

// number reads a number the site may quote; -1 for none.
func number(raw json.RawMessage) int {
	var n json.Number
	if json.Unmarshal(raw, &n) != nil {
		var s string
		if json.Unmarshal(raw, &s) != nil {
			return -1
		}
		n = json.Number(s)
	}
	i, err := n.Int64()
	if err != nil {
		return -1
	}
	return int(i)
}

func uploader(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	return strings.Trim(string(raw), `"`)
}

// Download implements provider.Provider: the subtitle, or the first file
// of an archive.
func (p *Provider) Download(ctx context.Context, _ *provider.Video, id string) (provider.File, error) {
	base, err := url.Parse(p.site)
	if err != nil {
		return provider.File{}, err
	}
	ref, err := url.Parse(id)
	if err != nil {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "malformed subtitle ID %q", id)
	}
	link := base.ResolveReference(ref).String()
	r, err := site.Retry(ctx, p.env, func() (*site.Response, error) {
		return p.checked(ctx, func() (*http.Request, error) {
			return http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
		}, call{download: true})
	})
	if err != nil {
		return provider.File{}, err
	}
	return archive.Extract(r.Body, archive.Want{First: true}, provider.Language{Tag: language})
}
