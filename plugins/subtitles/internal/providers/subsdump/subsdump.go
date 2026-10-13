// Package subsdump is the SubsDump provider, ported from Bazarr's
// subsdump.py: the JSON API of a self-hosted SubsDump instance serving a
// local Subscene archive, with an optional API key.
package subsdump

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/archive"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/site"
)

// Name is the provider's, as Bazarr names it.
const Name = "subsdump"

const (
	userAgent = "Mavio SubsDump"
	// maxJSON and maxContent bound the answers, as Bazarr's.
	maxJSON    = 2 << 20
	maxContent = 10 << 20
	// infoTimeout and timeout are those of Bazarr's requests.
	infoTimeout = 10 * time.Second
	timeout     = 30 * time.Second
)

// languageCodes are SubsDump's languages, as Bazarr's _LANGUAGE_CODES.
var languageCodes = [...]string{
	"ara", "aze", "bel", "ben", "bos", "bul", "cat", "ces", "dan", "deu", "ell", "eng", "epo", "est", "eus", "fas",
	"fin", "fra", "heb", "hin", "hrv", "hun", "hye", "ind", "isl", "ita", "jpn", "kal", "kan", "kat", "khm", "kin",
	"kor", "kur", "lav", "lit", "mal", "mkd", "mni", "mon", "msa", "mya", "nep", "nld", "nor", "pan", "pol", "por",
	"pus", "ron", "rus", "sin", "slk", "slv", "som", "spa", "sqi", "srp", "sun", "swa", "swe", "tam", "tel", "tgl",
	"tha", "tur", "ukr", "urd", "vie", "yor", "zho",
}

// codes returns SubsDump's codes by BCP 47 tag, Brazilian Portuguese
// being "por-BR" as Bazarr's _language_code.
func codes() map[string]string {
	m := make(map[string]string, len(languageCodes)+1)
	for _, c := range languageCodes {
		m[provider.FromCode(c)] = c
	}
	m["pt-BR"] = "por-BR"
	return m
}

// Definition returns the provider's definition, with Bazarr's settings.
func Definition() provider.Definition {
	var langs []string
	for tag := range codes() {
		langs = append(langs, tag)
	}
	return provider.Definition{
		Name: Name, DisplayName: "SubsDump",
		Settings: []provider.Setting{
			{Name: "base_url", Label: "Base URL", Help: "A self-hosted SubsDump instance", Type: provider.Text},
			{Name: "api_key", Label: "API Key (optional)", Type: provider.Password},
		},
		Languages:                 langs,
		HearingImpairedVerifiable: true,
		Movies:                    true, Episodes: true,
		New: New,
	}
}

// Provider searches a SubsDump instance.
type Provider struct {
	env  provider.Env
	s    *site.Session
	base string

	mu sync.Mutex
	// ready is set once the instance answered as SubsDump's v1 API, as
	// Bazarr's initialize.
	ready bool
}

// New makes the provider from its settings, validating the base URL as
// Bazarr's.
func New(env provider.Env, raw json.RawMessage) (provider.Provider, error) {
	var s struct {
		BaseURL string `json:"base_url"`
		APIKey  string `json:"api_key"`
	}
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, provider.Errorf(provider.ConfigurationError, "settings: %w", err)
	}
	if s.BaseURL == "" {
		return nil, provider.Errorf(provider.ConfigurationError, "SubsDump Base URL is required")
	}
	u, err := url.Parse(s.BaseURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, provider.Errorf(provider.ConfigurationError, "SubsDump Base URL must begin with http:// or https://")
	}
	if u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.Contains(s.BaseURL, "#") {
		return nil, provider.Errorf(provider.ConfigurationError, "SubsDump Base URL cannot contain credentials, a query, or a fragment")
	}
	hc := http.DefaultClient
	if env.HTTP != nil {
		hc = env.HTTP
	}
	// Redirects are not followed, as Bazarr's.
	c := *hc
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	sess := site.New(&c)
	sess.Header.Set("Accept", "application/json")
	sess.Header.Set("User-Agent", userAgent)
	if s.APIKey != "" {
		sess.Header.Set("X-API-Key", s.APIKey)
	}
	sess.Check = func(*site.Response) error { return nil }
	return &Provider{env: env, s: sess, base: strings.TrimRight(s.BaseURL, "/")}, nil
}

// SetBaseURL points the provider at another instance, for tests.
func (p *Provider) SetBaseURL(u *url.URL) { p.base = strings.TrimRight(u.String(), "/") }

// get fetches a path of the instance, mapping network failures as
// Bazarr's: a timeout and an unreachable instance are provider errors, a
// failed TLS verification a configuration error.
func (p *Provider) get(ctx context.Context, pth string, query url.Values, limit time.Duration, what string) (*site.Response, error) {
	rctx, cancel := context.WithTimeout(ctx, limit)
	defer cancel()
	r, err := p.s.Get(rctx, p.base+pth, query)
	if err == nil {
		return r, nil
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	switch {
	case isTLS(err):
		return nil, provider.Errorf(provider.ConfigurationError, "SubsDump TLS verification failed: %w", err)
	case errors.Is(err, context.DeadlineExceeded) || provider.KindOf(err) == provider.Timeout:
		return nil, provider.Errorf(provider.ProviderError, "SubsDump %s timed out: %w", what, err)
	}
	return nil, provider.Errorf(provider.ProviderError, "SubsDump is unreachable: %w", err)
}

func isTLS(err error) bool {
	var (
		verification *tls.CertificateVerificationError
		authority    x509.UnknownAuthorityError
		hostname     x509.HostnameError
		invalid      x509.CertificateInvalidError
		header       tls.RecordHeaderError
	)
	return errors.As(err, &verification) || errors.As(err, &authority) || errors.As(err, &hostname) ||
		errors.As(err, &invalid) || errors.As(err, &header)
}

// json fetches an object of the API, as Bazarr's _json.
func (p *Provider) json(ctx context.Context, pth string, query url.Values, limit time.Duration, out any) error {
	r, err := p.get(ctx, pth, query, limit, "request")
	if err != nil {
		return err
	}
	switch {
	case r.Status == http.StatusUnauthorized || r.Status == http.StatusForbidden:
		return provider.Errorf(provider.AuthenticationError, "SubsDump rejected the API key")
	case r.Status == http.StatusServiceUnavailable:
		return provider.Errorf(provider.ServiceUnavailable, "SubsDump is not ready")
	case r.Status != http.StatusOK:
		return provider.Errorf(provider.ProviderError, "SubsDump returned HTTP %d", r.Status)
	case len(r.Body) > maxJSON:
		return provider.Errorf(provider.ProviderError, "SubsDump response exceeded the size limit")
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(r.Body, &obj); err != nil {
		return provider.Errorf(provider.ProviderError, "SubsDump returned malformed JSON: %w", err)
	}
	if obj == nil {
		return provider.Errorf(provider.ProviderError, "SubsDump returned an unexpected response")
	}
	if err := json.Unmarshal(r.Body, out); err != nil {
		return provider.Errorf(provider.ProviderError, "SubsDump returned an unexpected response: %w", err)
	}
	return nil
}

// initialize checks once that the instance serves SubsDump's v1 API, as
// Bazarr's initialize.
func (p *Provider) initialize(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ready {
		return nil
	}
	var info struct {
		Name       any `json:"name"`
		APIVersion any `json:"api_version"`
	}
	if err := p.json(ctx, "/api/v1/info", nil, infoTimeout, &info); err != nil {
		return err
	}
	if info.Name != "SubsDump" || info.APIVersion != "v1" {
		return provider.Errorf(provider.ConfigurationError, "The server does not provide the SubsDump v1 API")
	}
	p.ready = true
	return nil
}

// record is a subtitle of the API, decoded loosely so that malformed ones
// are skipped as Bazarr's _valid_record.
type record struct {
	ID    json.RawMessage `json:"id"`
	Media *struct {
		Title  any  `json:"title"`
		Year   *int `json:"year"`
		Season *int `json:"season"`
		// Episode is the record's episode, nil for none.
		Episode *int `json:"episode"`
	} `json:"media"`
	Language *struct {
		Code any `json:"code"`
	} `json:"language"`
	Releases        []any `json:"releases"`
	HearingImpaired any   `json:"hearing_impaired"`
	Uploader        any   `json:"uploader"`
	Links           *struct {
		Page    any `json:"page"`
		Content any `json:"content"`
	} `json:"links"`
}

// valid returns the record's ID when it is well formed and in the
// language, as Bazarr's _valid_record.
func (r *record) valid(code string) (int, bool) {
	var id int
	if err := json.Unmarshal(r.ID, &id); err != nil || id < 1 || strings.ContainsAny(string(r.ID), ".eE") {
		return 0, false
	}
	if r.Media == nil || r.Links == nil || r.Language == nil {
		return 0, false
	}
	if _, ok := r.Media.Title.(string); !ok || r.Language.Code != code || r.Releases == nil {
		return 0, false
	}
	for _, rel := range r.Releases {
		if _, ok := rel.(string); !ok {
			return 0, false
		}
	}
	return id, r.Links.Page == fmt.Sprintf("/subtitles/%d", id) && r.Links.Content == contentPath(id)
}

func contentPath(id int) string { return fmt.Sprintf("/api/v1/subtitles/%d/content", id) }

// Search lists the subtitles of a video, one query by language, as
// Bazarr's list_subtitles.
func (p *Provider) Search(ctx context.Context, v *provider.Video, langs []provider.Language) ([]provider.Subtitle, error) {
	codes := codes()
	var out []provider.Subtitle
	for _, l := range langs {
		code, ok := codes[l.Tag]
		if !ok {
			continue
		}
		if err := p.initialize(ctx); err != nil {
			return nil, err
		}
		subs, err := p.query(ctx, v, l, code)
		if err != nil {
			return nil, err
		}
		out = append(out, subs...)
	}
	return out, nil
}

// query searches one language, by IMDb ID or by title, as Bazarr's
// _query_language.
func (p *Provider) query(ctx context.Context, v *provider.Video, l provider.Language, code string) ([]provider.Subtitle, error) {
	q := url.Values{"language": {code}, "per_page": {"100"}}
	if name := path.Base(v.FileName); v.FileName != "" {
		q.Set("release", truncate(name, 512))
	}
	pth := "/api/v1/subtitles"
	if v.Episode {
		if imdb := v.SeriesExternalIDs["imdb"]; imdb != "" {
			pth = fmt.Sprintf("/api/v1/series/%s/seasons/%d/episodes/%d/subtitles", url.PathEscape(imdb), v.Season, v.Number)
		} else {
			q.Set("title", v.Series)
			q.Set("media_type", "episode")
			q.Set("season", strconv.Itoa(v.Season))
			q.Set("episode", strconv.Itoa(v.Number))
		}
	} else {
		if imdb := v.IMDbID(); imdb != "" {
			pth = fmt.Sprintf("/api/v1/movies/%s/subtitles", url.PathEscape(imdb))
		} else {
			q.Set("title", v.Title)
			q.Set("media_type", "movie")
		}
		if v.Year != 0 {
			q.Set("year", strconv.Itoa(v.Year))
		}
	}
	var payload struct {
		Subtitles []json.RawMessage `json:"subtitles"`
	}
	if err := p.json(ctx, pth, q, timeout, &payload); err != nil {
		return nil, err
	}
	if payload.Subtitles == nil {
		return nil, provider.Errorf(provider.ProviderError, "SubsDump response is missing subtitles")
	}
	var out []provider.Subtitle
	for _, raw := range payload.Subtitles {
		var r record
		id, ok := 0, json.Unmarshal(raw, &r) == nil
		if ok {
			id, ok = r.valid(code)
		}
		if !ok {
			p.env.Log.DebugContext(ctx, "Skipping malformed SubsDump subtitle record")
			continue
		}
		out = append(out, p.subtitle(v, l, id, &r))
	}
	return out, nil
}

func truncate(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

// subtitle makes a record a subtitle with the matches of Bazarr's
// get_matches; the releases are matched by the server.
func (p *Provider) subtitle(v *provider.Video, l provider.Language, id int, r *record) provider.Subtitle {
	title, _ := r.Media.Title.(string)
	var matches []string
	if v.Episode {
		if title != "" && v.Series != "" && strings.EqualFold(title, v.Series) {
			matches = append(matches, "series")
		}
		if r.Media.Season != nil && *r.Media.Season == v.Season {
			matches = append(matches, "season")
		}
		if r.Media.Episode != nil && *r.Media.Episode == v.Number {
			matches = append(matches, "episode")
		}
	} else {
		if title != "" && v.Title != "" && strings.EqualFold(title, v.Title) {
			matches = append(matches, "title")
		}
		if r.Media.Year != nil && *r.Media.Year != 0 && *r.Media.Year == v.Year {
			matches = append(matches, "year")
		}
	}
	releases := make([]string, 0, len(r.Releases))
	for _, rel := range r.Releases {
		releases = append(releases, rel.(string))
	}
	uploader, _ := r.Uploader.(string)
	page, _ := r.Links.Page.(string)
	return provider.Subtitle{
		ID:           strconv.Itoa(id),
		Language:     provider.Language{Tag: l.Tag, Forced: l.Forced, HearingImpaired: truthy(r.HearingImpaired)},
		ReleaseNames: releases, Matches: matches,
		PageLink: p.base + page, Uploader: uploader,
	}
}

// truthy is Python's bool of a JSON value.
func truthy(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case float64:
		return x != 0
	case string:
		return x != ""
	case nil:
		return false
	}
	return true
}

// Download fetches a subtitle's content, as Bazarr's download_subtitle.
func (p *Provider) Download(ctx context.Context, _ *provider.Video, id string) (provider.File, error) {
	n, err := strconv.Atoi(id)
	if err != nil || n < 1 {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "bad subtitle ID %q", id)
	}
	if err := p.initialize(ctx); err != nil {
		return provider.File{}, err
	}
	r, err := p.get(ctx, contentPath(n), nil, timeout, "download")
	if err != nil {
		return provider.File{}, err
	}
	switch {
	case r.Status == http.StatusUnauthorized || r.Status == http.StatusForbidden:
		return provider.File{}, provider.Errorf(provider.AuthenticationError, "SubsDump rejected the API key")
	case r.Status != http.StatusOK:
		return provider.File{}, provider.Errorf(provider.ProviderError, "SubsDump download returned HTTP %d", r.Status)
	case len(r.Body) == 0 || len(r.Body) > maxContent:
		return provider.File{}, provider.Errorf(provider.ProviderError, "SubsDump returned an invalid subtitle file")
	}
	data := archive.FixLineEndings(r.Body)
	return provider.File{Data: data, Format: archive.Detect(data)}, nil
}
