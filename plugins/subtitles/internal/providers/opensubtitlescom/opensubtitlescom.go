// Package opensubtitlescom is the provider of OpenSubtitles.com, ported
// from Bazarr's opensubtitlescom: subtitles of movies and episodes found
// by IMDb ID or the site's title ID, and by file hash.
package opensubtitlescom

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
)

// Name is the provider's name.
const Name = "opensubtitlescom"

// userAgent identifies the plugin, as OpenSubtitles requires.
const userAgent = "Mavio v0.1"

// defaultHost serves the API until a login names another.
const defaultHost = "api.opensubtitles.com"

// maxSubtitle bounds a downloaded subtitle file.
const maxSubtitle = 10 << 20

// retries is how many times a request is tried.
const retries = 3

// Definition returns the provider's definition.
func Definition() provider.Definition {
	return provider.Definition{
		Name: Name, DisplayName: "OpenSubtitles.com",
		Settings: []provider.Setting{
			{Name: "api_key", Label: "API key", Help: "An OpenSubtitles.com API consumer key", Type: provider.Password},
			{Name: "username", Label: "Username", Type: provider.Text},
			{Name: "password", Label: "Password", Type: provider.Password},
			{Name: "use_hash", Label: "Use hash", Help: "Search by the video file's hash too", Type: provider.Checkbox, Default: true},
			{Name: "include_ai_translated", Label: "Include AI-translated subtitles", Type: provider.Checkbox, Default: false},
			{Name: "include_machine_translated", Label: "Include machine-translated subtitles", Type: provider.Checkbox, Default: false},
		},
		HashVerifiable: true, HearingImpairedVerifiable: true, Movies: true, Episodes: true,
		Throttle: map[provider.Kind]func(time.Time) time.Duration{
			provider.TooManyRequests:       provider.Fixed(time.Minute),
			provider.DownloadLimitExceeded: provider.Fixed(6 * time.Hour),
			provider.ProviderError:         provider.Fixed(time.Minute),
		},
		New: New,
	}
}

// settings are the provider's settings.
type settings struct {
	APIKey                   string `json:"api_key"`
	Username                 string `json:"username"`
	Password                 string `json:"password"`
	UseHash                  *bool  `json:"use_hash"`
	IncludeAITranslated      bool   `json:"include_ai_translated"`
	IncludeMachineTranslated bool   `json:"include_machine_translated"`
}

// New makes the provider from its settings.
func New(env provider.Env, raw json.RawMessage) (provider.Provider, error) {
	var s settings
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, provider.Errorf(provider.ConfigurationError, "settings: %w", err)
		}
	}
	switch {
	case s.Username == "" || s.Password == "":
		return nil, provider.Errorf(provider.ConfigurationError, "username and password must be specified")
	case s.APIKey == "":
		return nil, provider.Errorf(provider.ConfigurationError, "API key must be specified")
	}
	return &Provider{env: env, s: s, scheme: "https", host: defaultHost, titles: map[string]titleID{}}, nil
}

// Provider is OpenSubtitles.com.
type Provider struct {
	env    provider.Env
	s      settings
	scheme string

	mu    sync.Mutex
	token string
	host  string
	// titles caches the title IDs of names for a week, as Bazarr's
	// search_titles.
	titles map[string]titleID
}

type titleID struct {
	id    int
	until time.Time
}

// SetBaseURL points the provider at another server, for tests.
func (p *Provider) SetBaseURL(u *url.URL) { p.scheme, p.host = u.Scheme, u.Host }

func (p *Provider) serverURL() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.scheme + "://" + p.host + "/api/v1/"
}

// toSite maps a language to the site's code, from Bazarr's
// custom_languages and the site's codes of Chinese and Portuguese.
func toSite(tag string) string {
	switch tag {
	case "pt":
		return "pt-pt"
	case "zh", "zh-Hans", "zh-CN":
		return "zh-cn"
	case "zh-Hant", "zh-TW":
		return "zh-tw"
	case "es-MX":
		return "ea"
	}
	return strings.ToLower(tag)
}

// fromSite maps the site's language code back.
func fromSite(code string) string {
	switch strings.ToLower(code) {
	case "pt-pt":
		return "pt"
	case "pt-br":
		return "pt-BR"
	case "zh-cn":
		return "zh-Hans"
	case "zh-tw":
		return "zh-Hant"
	case "ea":
		return "es-MX"
	}
	return code
}

// externalID turns an IMDb ID or title ID into the number the site
// takes, as Bazarr's sanitize_external_ids.
func externalID(id string) (int, bool) {
	id = strings.TrimLeft(strings.ToLower(id), "t")
	id = strings.TrimLeft(id, "0")
	n, err := strconv.Atoi(cmp.Or(id, "0"))
	return n, err == nil && n > 0
}

type attributes struct {
	Language          string  `json:"language"`
	DownloadCount     int     `json:"download_count"`
	HearingImpaired   bool    `json:"hearing_impaired"`
	ForeignPartsOnly  bool    `json:"foreign_parts_only"`
	AITranslated      bool    `json:"ai_translated"`
	MachineTranslated bool    `json:"machine_translated"`
	MovieHashMatch    bool    `json:"moviehash_match"`
	Release           string  `json:"release"`
	Comments          string  `json:"comments"`
	URL               string  `json:"url"`
	Ratings           float64 `json:"ratings"`
	Uploader          struct {
		Name string `json:"name"`
	} `json:"uploader"`
	FeatureDetails struct {
		MovieName     string          `json:"movie_name"`
		Year          json.RawMessage `json:"year"`
		SeasonNumber  *int            `json:"season_number"`
		EpisodeNumber *int            `json:"episode_number"`
		IMDbID        *int            `json:"imdb_id"`
		ParentIMDbID  *int            `json:"parent_imdb_id"`
	} `json:"feature_details"`
	Files []struct {
		FileID   int64  `json:"file_id"`
		FileName string `json:"file_name"`
	} `json:"files"`
}

// realForced reports whether subtitles are forced: foreign parts only,
// and not for the hearing-impaired.
func (a *attributes) realForced() bool { return a.ForeignPartsOnly && !a.HearingImpaired }

// Search implements provider.Provider.
func (p *Provider) Search(ctx context.Context, v *provider.Video, langs []provider.Language) ([]provider.Subtitle, error) {
	fileHash := ""
	if p.s.UseHash == nil || *p.s.UseHash {
		fileHash = v.FileHash
	}
	imdbID, hasIMDb := externalID(v.ExternalIDs["imdb"])
	seriesIMDbID, hasSeriesIMDb := externalID(v.SeriesExternalIDs["imdb"])

	titleID := 0
	if v.Episode && !hasSeriesIMDb || !v.Episode && !hasIMDb {
		title := v.Title
		if v.Episode {
			title = v.Series
		}
		var err error
		if titleID, err = p.searchTitle(ctx, v, title); err != nil {
			return nil, err
		}
	}

	codes := map[string]bool{}
	for _, l := range langs {
		codes[toSite(l.Tag)] = true
	}
	params := url.Values{"languages": {strings.Join(slices.Sorted(func(yield func(string) bool) {
		for c := range codes {
			if !yield(c) {
				return
			}
		}
	}), ",")}}
	if fileHash != "" {
		params.Set("moviehash", fileHash)
	}
	if hasIMDb {
		params.Set("imdb_id", strconv.Itoa(imdbID))
	}
	if v.Episode {
		if !hasIMDb && titleID == 0 && !hasSeriesIMDb {
			return nil, nil
		}
		if v.Number > 0 {
			params.Set("episode_number", strconv.Itoa(v.Number))
		}
		if v.Season > 0 {
			params.Set("season_number", strconv.Itoa(v.Season))
		}
		if hasSeriesIMDb {
			params.Set("parent_imdb_id", strconv.Itoa(seriesIMDbID))
		} else if titleID != 0 {
			params.Set("parent_feature_id", strconv.Itoa(titleID))
		}
	} else {
		if !hasIMDb && titleID == 0 {
			return nil, nil
		}
		if titleID != 0 {
			params.Set("id", strconv.Itoa(titleID))
		}
	}
	if !p.s.IncludeAITranslated {
		params.Set("ai_translated", "exclude")
	}
	if p.s.IncludeMachineTranslated {
		params.Set("machine_translated", "include")
	}

	var res struct {
		Data []struct {
			Attributes attributes `json:"attributes"`
		} `json:"data"`
	}
	if err := p.retry(ctx, func() error {
		return p.call(ctx, http.MethodGet, "subtitles", params, nil, &res, "data")
	}); err != nil {
		return nil, err
	}

	allForced := !slices.ContainsFunc(langs, func(l provider.Language) bool { return !l.Forced })
	anyForced := slices.ContainsFunc(langs, func(l provider.Language) bool { return l.Forced })
	var out []provider.Subtitle
	for _, item := range res.Data {
		a := item.Attributes
		switch {
		case allForced && !a.realForced(), !anyForced && a.realForced():
			continue
		case a.AITranslated && !p.s.IncludeAITranslated, a.MachineTranslated && !p.s.IncludeMachineTranslated:
			continue
		case len(a.Files) == 0:
			continue
		}
		fd := a.FeatureDetails
		imdbMatch := fd.ParentIMDbID != nil && hasSeriesIMDb && *fd.ParentIMDbID == seriesIMDbID ||
			fd.IMDbID != nil && hasIMDb && *fd.IMDbID == imdbID
		var matches []string
		if v.Episode {
			matches = append(matches, "series")
			if fd.SeasonNumber != nil && *fd.SeasonNumber == v.Season {
				matches = append(matches, "season")
			}
			if fd.EpisodeNumber != nil && *fd.EpisodeNumber == v.Number {
				matches = append(matches, "episode")
			}
			if imdbMatch {
				matches = append(matches, "series_imdb_id")
			}
		} else {
			matches = append(matches, "title")
			if imdbMatch {
				matches = append(matches, "imdb_id")
			}
		}
		if year, err := strconv.Atoi(strings.Trim(string(fd.Year), `"`)); imdbMatch || err == nil && year == v.Year && year != 0 {
			matches = append(matches, "year")
		}
		if a.MovieHashMatch {
			matches = append(matches, "hash")
		}
		out = append(out, provider.Subtitle{
			ID: strconv.FormatInt(a.Files[0].FileID, 10),
			Language: provider.Language{
				Tag: fromSite(a.Language), Forced: a.realForced(), HearingImpaired: a.HearingImpaired,
			},
			ReleaseNames: []string{a.Release},
			Matches:      matches,
			Format:       "srt",
			PageLink:     a.URL,
			Uploader:     a.Uploader.Name,
			Comment:      a.Comments,
			Downloads:    a.DownloadCount,
		})
	}
	return out, nil
}

// searchTitle returns the site's ID of a movie or series by its name, as
// Bazarr's search_titles: the first feature of that title and of the
// video's year, if it has one; 0 for none.
func (p *Provider) searchTitle(ctx context.Context, v *provider.Video, title string) (int, error) {
	key := strings.ToLower(title)
	now := p.env.Now()
	p.mu.Lock()
	cached, ok := p.titles[key]
	p.mu.Unlock()
	if ok && now.Before(cached.until) {
		return cached.id, nil
	}
	var res struct {
		Data []struct {
			ID         string `json:"id"`
			Attributes struct {
				Title string          `json:"title"`
				Year  json.RawMessage `json:"year"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := p.retry(ctx, func() error {
		return p.call(ctx, http.MethodGet, "features", url.Values{"query": {key}}, nil, &res, "data")
	}); err != nil {
		return 0, err
	}
	id := 0
	for _, r := range res.Data {
		year, _ := strconv.Atoi(strings.Trim(string(r.Attributes.Year), `"`))
		if r.Attributes.Title != "" && strings.EqualFold(r.Attributes.Title, title) && (v.Year == 0 || v.Year == year) {
			id, _ = externalID(r.ID)
			break
		}
	}
	p.mu.Lock()
	p.titles[key] = titleID{id: id, until: now.Add(7 * 24 * time.Hour)}
	p.mu.Unlock()
	return id, nil
}

// Download implements provider.Provider.
func (p *Provider) Download(ctx context.Context, _ *provider.Video, id string) (provider.File, error) {
	fileID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "malformed subtitle ID %q", id)
	}
	var link struct {
		Link     string `json:"link"`
		FileName string `json:"file_name"`
	}
	if err := p.retry(ctx, func() error {
		return p.call(ctx, http.MethodPost, "download", nil, map[string]any{"file_id": fileID, "sub_format": "srt"}, &link, "link")
	}); err != nil {
		return provider.File{}, err
	}
	var data []byte
	if err := p.retry(ctx, func() error {
		var err error
		data, err = p.fetch(ctx, link.Link)
		return err
	}); err != nil {
		return provider.File{}, err
	}
	return provider.File{Data: bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n")), Format: "srt"}, nil
}

func (p *Provider) retry(ctx context.Context, f func() error) error {
	_, err := provider.Retry(ctx, retries, p.env.RetryWait, func() (struct{}, error) { return struct{}{}, f() })
	return err
}

// login opens a session, as Bazarr's login: the token, and the host the
// account is served by.
func (p *Provider) login(ctx context.Context) error {
	var out struct {
		Token   string `json:"token"`
		BaseURL string `json:"base_url"`
	}
	if err := p.request(ctx, http.MethodPost, "login", nil, map[string]string{"username": p.s.Username, "password": p.s.Password}, &out, "token", false); err != nil {
		return err
	}
	p.mu.Lock()
	p.token = out.Token
	if out.BaseURL != "" && p.scheme == "https" {
		p.host = out.BaseURL
	}
	p.mu.Unlock()
	return nil
}

// call makes a request with a session, logging in first if there is none
// and once more if the session has expired.
func (p *Provider) call(ctx context.Context, method, path string, query url.Values, body, out any, key string) error {
	p.mu.Lock()
	token := p.token
	p.mu.Unlock()
	if token == "" {
		if err := p.login(ctx); err != nil {
			return err
		}
	}
	err := p.request(ctx, method, path, query, body, out, key, true)
	if !errors.Is(err, errUnauthorized) {
		return err
	}
	p.mu.Lock()
	p.token = ""
	p.mu.Unlock()
	if err := p.login(ctx); err != nil {
		return err
	}
	err = p.request(ctx, method, path, query, body, out, key, true)
	if errors.Is(err, errUnauthorized) {
		return provider.Errorf(provider.AuthenticationError, "login failed")
	}
	return err
}

// errUnauthorized is the answer to an expired session.
var errUnauthorized = errors.New("unauthorized")

// request makes one request, mapping the site's answers to failures as
// Bazarr's checked; the JSON answer must have key.
func (p *Provider) request(ctx context.Context, method, path string, query url.Values, body, out any, key string, auth bool) error {
	var r io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		r = bytes.NewReader(data)
	}
	u := p.serverURL() + path
	if len(query) > 0 {
		// Parameters in order spare a redirect.
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, u, r)
	if err != nil {
		return err
	}
	req.Header.Set("Api-Key", p.s.APIKey)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	p.mu.Lock()
	if auth && p.token != "" {
		req.Header.Set("Authorization", "Bearer "+p.token)
	}
	p.mu.Unlock()
	resp, err := p.env.HTTP.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return provider.Errorf(provider.ServiceUnavailable, "%s: %w", path, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxSubtitle))
	if err != nil {
		return provider.Errorf(provider.ServiceUnavailable, "%s: %w", path, err)
	}
	if err := checkStatus(resp, data, path); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return provider.Errorf(provider.ProviderError, "invalid JSON returned by provider")
	}
	if _, ok := fields[key]; !ok {
		return provider.Errorf(provider.ProviderError, "invalid JSON returned by provider: no %s key in returned json", key)
	}
	if err := json.Unmarshal(data, out); err != nil {
		return provider.Errorf(provider.ProviderError, "%s: %w", path, err)
	}
	return nil
}

// checkStatus maps the site's status codes to failures, as Bazarr's
// checked.
func checkStatus(resp *http.Response, data []byte, path string) error {
	var msg struct {
		Message   string `json:"message"`
		Requests  int    `json:"requests"`
		Remaining int    `json:"remaining"`
		ResetTime string `json:"reset_time"`
	}
	switch code := resp.StatusCode; {
	case code == http.StatusOK:
		return nil
	case code == http.StatusBadRequest:
		if err := json.Unmarshal(data, &msg); err != nil {
			return provider.Errorf(provider.ProviderError, "invalid JSON returned by provider")
		}
		return provider.Errorf(provider.ConfigurationError, "%s", msg.Message)
	case code == http.StatusUnauthorized:
		return errUnauthorized
	case code == http.StatusForbidden:
		return provider.Errorf(provider.ProviderError, "the API key was refused")
	case code == http.StatusNotAcceptable:
		if err := json.Unmarshal(data, &msg); err != nil {
			return provider.Errorf(provider.ProviderError, "invalid JSON returned by provider")
		}
		return provider.Errorf(provider.DownloadLimitExceeded, "daily download limit reached: %d subtitles have been downloaded and %d remain; the quota is reset in %s",
			msg.Requests, msg.Remaining, msg.ResetTime)
	case code == http.StatusGone:
		return provider.Errorf(provider.ProviderError, "download link has expired")
	case code == http.StatusTooManyRequests:
		return provider.Errorf(provider.TooManyRequests, "%s: too many requests", path)
	case code == http.StatusBadGateway:
		return provider.Errorf(provider.APIThrottled, "%s: bad gateway", path)
	case code >= 500:
		return provider.Errorf(provider.ProviderError, "%s: %s", path, resp.Status)
	}
	return provider.Errorf(provider.ProviderError, "%s: bad status code: %d", path, resp.StatusCode)
}

// fetch downloads a subtitle file from its link.
func (p *Provider) fetch(ctx context.Context, link string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
	if err != nil {
		return nil, provider.Errorf(provider.ProviderError, "download link %q: %w", link, err)
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := p.env.HTTP.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, provider.Errorf(provider.ServiceUnavailable, "download: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxSubtitle+1))
	if err != nil {
		return nil, provider.Errorf(provider.ServiceUnavailable, "download: %w", err)
	}
	if err := checkStatus(resp, data, "download"); err != nil {
		return nil, err
	}
	switch {
	case len(data) > maxSubtitle:
		return nil, provider.Errorf(provider.BadSubtitle, "download: larger than %d bytes", maxSubtitle)
	case len(data) == 0:
		return nil, provider.Errorf(provider.ProviderError, "download link %s returned empty content", link)
	}
	return data, nil
}

// String names the provider in logs.
func (p *Provider) String() string { return fmt.Sprintf("%s(%s)", Name, p.s.Username) }
