// Package subx is the provider of SubX, ported from Bazarr's subx: the
// Spanish subtitles of Subdivx found through the SubX API by IMDb ID or
// title, with an account's API key.
package subx

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
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/archive"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/site"
)

// Name is the provider's name.
const Name = "subx"

// userAgent identifies the plugin, as Bazarr's SZ_USER_AGENT.
const userAgent = "Mavio/0.1"

// maxRetries is how many times a search is tried.
const maxRetries = 3

// Definition returns the provider's definition.
func Definition() provider.Definition {
	return provider.Definition{
		Name: Name, DisplayName: "SubX",
		Settings: []provider.Setting{
			{
				Name: "api_key", Label: "API Key", Type: provider.Text,
				Help: "API key required. Get your key from SubX website: https://subx-api.duckdns.org/",
			},
		},
		Languages: []string{"es", "es-MX"},
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
	if s.APIKey == "" {
		return nil, provider.Errorf(provider.ConfigurationError, "SubX API key is required")
	}
	sess := site.New(env.HTTP)
	sess.Header.Set("Authorization", "Bearer "+s.APIKey)
	sess.Header.Set("User-Agent", userAgent)
	sess.Check = func(*site.Response) error { return nil }
	return &Provider{env: env, sess: sess, base: "https://subx-api.duckdns.org", sleep: site.Sleep}, nil
}

// Provider is SubX.
type Provider struct {
	env   provider.Env
	sess  *site.Session
	base  string
	sleep func(context.Context, time.Duration) error
}

// SetBaseURL points the provider at another server, for tests.
func (p *Provider) SetBaseURL(u *url.URL) { p.base = strings.TrimSuffix(u.String(), "/") }

// spain tells Spain's Spanish in descriptions, as Bazarr's Subdivx.
var spain = regexp.MustCompile(`(?i)españa|ib[eé]rico|castellano|gallego|castilla|europ[ae]`)

var (
	dotsUnderscores = regexp.MustCompile(`[._]+`)
	spaces          = regexp.MustCompile(`\s+`)
)

// sanitize cleans a series' title for searching.
func sanitize(title string) string {
	title = dotsUnderscores.ReplaceAllString(title, " ")
	return strings.TrimSpace(spaces.ReplaceAllString(title, " "))
}

// titles are the video's title and alternatives, unique, at most three.
func titles(v *provider.Video) []string {
	all := append([]string{v.Title}, v.AlternativeTitles...)
	if v.Episode {
		all = append([]string{v.Series}, v.AlternativeSeries...)
	}
	var out []string
	for _, t := range all {
		if t != "" && !slices.Contains(out, t) {
			out = append(out, t)
		}
	}
	return out[:min(len(out), 3)]
}

// Search implements provider.Provider, as Bazarr's list_subtitles: for
// each title, the episode, else its season, else the series; or the
// movie.
func (p *Provider) Search(ctx context.Context, v *provider.Video, langs []provider.Language) ([]provider.Subtitle, error) {
	var subs []provider.Subtitle
	for i, raw := range titles(v) {
		if i > 0 {
			// A small pause between titles.
			if err := p.sleep(ctx, time.Second); err != nil {
				return nil, err
			}
		}
		type query struct {
			q               string
			season, episode int
		}
		queries := []query{{raw, -1, -1}}
		videoType := "movie"
		if v.Episode {
			title := sanitize(raw)
			videoType = "episode"
			queries = []query{
				{fmt.Sprintf("%s S%02dE%02d", title, v.Season, v.Number), v.Season, v.Number},
				{fmt.Sprintf("%s S%02d", title, v.Season), v.Season, -1},
				{title, v.Season, -1},
			}
		}
		for _, q := range queries {
			var err error
			if subs, err = p.query(ctx, q.q, v, videoType, q.season, q.episode); err != nil {
				return nil, err
			}
			if len(subs) > 0 {
				break
			}
		}
		if len(subs) > 0 {
			break
		}
	}
	return slices.DeleteFunc(subs, func(s provider.Subtitle) bool {
		return !slices.ContainsFunc(langs, func(l provider.Language) bool { return l.Tag == s.Language.Tag })
	}), nil
}

type item struct {
	ID           json.Number `json:"id"`
	Title        any         `json:"title"`
	Description  string      `json:"description"`
	UploaderName *string     `json:"uploader_name"`
	PageURL      string      `json:"page_url"`
	Season       *int        `json:"season"`
	Episode      *int        `json:"episode"`
}

// query searches the API, as Bazarr's run_query: by the video's IMDb ID,
// else the query, keeping the subtitles of a season (-1 for any) and an
// episode, else the season's packs (-1 for any episode).
func (p *Provider) query(ctx context.Context, q string, v *provider.Video, videoType string, season, episode int) ([]provider.Subtitle, error) {
	params := url.Values{"limit": {"200"}, "video_type": {videoType}}
	switch imdb := v.ExternalIDs["imdb"]; {
	case imdb != "":
		params.Set("imdb_id", imdb)
	case q != "":
		params.Set("title", q)
	default:
		return nil, nil
	}
	if v.Year > 0 {
		params.Set("year", strconv.Itoa(v.Year))
	}
	var data struct {
		Total any    `json:"total"`
		Items []item `json:"items"`
	}
	ok, err := p.search(ctx, params, &data)
	if err != nil || !ok {
		return nil, err
	}

	var subs []provider.Subtitle
	var packs []item
	for _, it := range data.Items {
		if season >= 0 && (it.Season == nil || *it.Season != season) {
			continue
		}
		if episode >= 0 {
			switch {
			case it.Episode != nil && *it.Episode == episode:
			case it.Episode == nil:
				packs = append(packs, it)
				continue
			default:
				continue
			}
		}
		subs = append(subs, p.subtitle(v, it))
	}
	// Without the episode's, the season's packs.
	if episode >= 0 && len(subs) == 0 {
		for _, it := range packs {
			subs = append(subs, p.subtitle(v, it))
		}
	}
	return subs, nil
}

func (p *Provider) subtitle(v *provider.Video, it item) provider.Subtitle {
	page := it.PageURL
	if page == "" && it.ID != "" {
		page = p.base + "/api/subtitles/" + it.ID.String()
	}
	tag := "es-MX"
	if spain.MatchString(it.Description) {
		tag = "es"
	}
	title := strings.TrimSpace(fmt.Sprint(it.Title))
	if it.Title == nil {
		title = "None"
	}
	release := title
	if it.Description != "" {
		release += " | " + it.Description
	}
	var m []string
	if v.Episode {
		m = append(m, "title", "series", "year")
		if it.Season != nil && *it.Season == v.Season {
			m = append(m, "season")
		}
		// Season packs match any episode.
		if it.Episode == nil || *it.Episode == v.Number {
			m = append(m, "episode")
		}
	} else {
		m = append(m, "title", "year")
	}
	uploader := "unknown"
	if it.UploaderName != nil {
		uploader = *it.UploaderName
	}
	return provider.Subtitle{
		ID:           it.ID.String(),
		Language:     provider.Language{Tag: tag},
		ReleaseNames: strings.Split(release, "\n"),
		Matches:      m,
		PageLink:     page,
		Uploader:     uploader,
		Comment:      it.Description,
	}
}

// search makes a search with Bazarr's retries: rate limits waited out,
// server failures backed off from; false when there is nothing.
func (p *Provider) search(ctx context.Context, params url.Values, out any) (bool, error) {
	for attempt := range maxRetries {
		last := attempt == maxRetries-1
		r, err := p.sess.Get(ctx, p.base+"/api/subtitles/search", params)
		if err != nil {
			if ctx.Err() != nil {
				return false, ctx.Err()
			}
			if last {
				return false, err
			}
			p.env.Log.WarnContext(ctx, "SubX API error", "attempt", attempt+1, "err", err)
			if err := p.sleep(ctx, time.Duration(1<<attempt)*time.Second); err != nil {
				return false, err
			}
			continue
		}
		switch code := r.Status; {
		case code == 400:
			p.env.Log.ErrorContext(ctx, "bad request to SubX API", "body", string(r.Body))
			return false, nil
		case code == 401:
			return false, provider.Errorf(provider.ConfigurationError, "invalid SubX API key")
		case code == 404:
			return false, nil
		case code == 429:
			if last {
				return false, provider.Errorf(provider.APIThrottled, "SubX rate limit exceeded")
			}
			wait := time.Duration(60*(attempt+1)) * time.Second
			if s, err := strconv.Atoi(r.Header.Get("Retry-After")); err == nil {
				wait = time.Duration(s) * time.Second
			}
			p.env.Log.WarnContext(ctx, "SubX rate limit hit", "wait", wait)
			if err := p.sleep(ctx, wait); err != nil {
				return false, err
			}
			continue
		case code >= 500:
			if last {
				p.env.Log.ErrorContext(ctx, "SubX server error persists", "status", code)
				return false, nil
			}
			if err := p.sleep(ctx, time.Duration(1<<attempt)*time.Second); err != nil {
				return false, err
			}
			continue
		case code >= 400:
			return false, provider.Errorf(provider.ProviderError, "%s: bad status code: %d", r.URL.Path, code)
		}
		if err := r.JSON(out); err != nil {
			return false, err
		}
		if err := p.slowDown(ctx, r); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

// slowDown waits for the rate limit's window when below a fifth of it is
// left.
func (p *Provider) slowDown(ctx context.Context, r *site.Response) error {
	remaining, err1 := strconv.Atoi(r.Header.Get("X-RateLimit-Remaining"))
	limit, err2 := strconv.Atoi(r.Header.Get("X-RateLimit-Limit"))
	if err1 != nil || err2 != nil || float64(remaining) >= float64(limit)*0.2 {
		return nil
	}
	wait := 2 * time.Second
	if reset, err := strconv.ParseInt(r.Header.Get("X-RateLimit-Reset"), 10, 64); err == nil {
		wait = max(0, time.Unix(reset, 0).Sub(p.env.Now()).Truncate(time.Second))
	}
	p.env.Log.WarnContext(ctx, "approaching SubX rate limit", "remaining", remaining, "limit", limit, "wait", wait)
	return p.sleep(ctx, wait)
}

// Download implements provider.Provider: the episode's subtitle of the
// archive. Failures throttle the provider, as Bazarr's.
func (p *Provider) Download(ctx context.Context, v *provider.Video, id string) (provider.File, error) {
	r, err := p.sess.Get(ctx, p.base+"/api/subtitles/"+url.PathEscape(id)+"/download", nil)
	if err != nil {
		if ctx.Err() != nil {
			return provider.File{}, ctx.Err()
		}
		return provider.File{}, provider.Errorf(provider.APIThrottled, "failed to download subtitle: %w", err)
	}
	if r.Status >= 400 {
		return provider.File{}, provider.Errorf(provider.APIThrottled, "failed to download subtitle: status %d", r.Status)
	}
	a, err := archive.Open(r.Body)
	if errors.Is(err, archive.ErrNotArchive) {
		return provider.File{}, provider.Errorf(provider.APIThrottled, "unknown or unsupported archive format")
	}
	if err != nil {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "%w", err)
	}
	w := archive.Want{}
	if v != nil && v.Episode {
		w.Episode = v.Number
	}
	name, data, err := archive.Subtitle(a, w)
	if err != nil {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "%w", err)
	}
	if name == "" {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "no subtitle of the video in the archive")
	}
	format := strings.TrimPrefix(strings.ToLower(name[strings.LastIndex(name, ".")+1:]), ".")
	if format == "sub" {
		format = archive.Detect(data)
	}
	return provider.File{Data: data, Format: format}, nil
}
