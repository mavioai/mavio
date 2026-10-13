// Package regielive is the provider of RegieLive, ported from Bazarr's
// regielive: Romanian subtitles of movies and episodes from the site's
// API for Bazarr, served in zip archives.
package regielive

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"net/url"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/archive"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/site"
)

// Name is the provider's name.
const Name = "regielive"

// language is the provider's only language.
const language = "ro"

// apiKey is the key of Bazarr's provider.
const apiKey = "API-BAZARR-YTZ-SL"

// downloadAgent is the browser downloads are made as, as Bazarr's.
const downloadAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36"

// Definition returns the provider's definition.
func Definition() provider.Definition {
	return provider.Definition{
		Name: Name, DisplayName: "RegieLive",
		Languages: []string{language},
		Movies:    true, Episodes: true,
		Throttle: map[provider.Kind]func(time.Time) time.Duration{
			provider.APIThrottled:    provider.Fixed(time.Hour),
			provider.TooManyRequests: provider.Fixed(5 * time.Minute),
			provider.ProviderError:   provider.Fixed(10 * time.Minute),
		},
		New: New,
	}
}

// New makes the provider; it takes no settings.
func New(env provider.Env, _ json.RawMessage) (provider.Provider, error) {
	s := site.New(env.HTTP)
	s.Check = checked
	return &Provider{env: env, s: s, api: "https://api.regielive.ro/bazarr/search.php"}, nil
}

// Provider is RegieLive.
type Provider struct {
	env provider.Env
	s   *site.Session
	api string
}

// SetBaseURL points the provider at another server, for tests.
func (p *Provider) SetBaseURL(u *url.URL) {
	p.api = strings.TrimSuffix(u.String(), "/") + "/bazarr/search.php"
}

// checked maps the status codes of answers to failures, as Bazarr's
// checked: 301 is the API throttling, 429 too many requests; others are
// left to the caller.
func checked(r *site.Response) error {
	switch r.Status {
	case http.StatusMovedPermanently:
		return provider.Errorf(provider.APIThrottled, "%s: moved permanently", r.URL.Path)
	case http.StatusTooManyRequests:
		return provider.Errorf(provider.TooManyRequests, "%s: too many requests", r.URL.Path)
	}
	return nil
}

// do sends a request; failures to send it are provider errors, as Bazarr's
// checked.
func (p *Provider) do(req *http.Request) (*site.Response, error) {
	r, err := p.s.Do(req)
	if err == nil || r != nil {
		return r, err
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return nil, err
	}
	return nil, provider.Errorf(provider.ProviderError, "unhandled exception raised: %w", err)
}

// pyValue is a value as Python's urlencode writes it: 0 is None.
func pyValue(n int) string {
	if n == 0 {
		return "None"
	}
	return strconv.Itoa(n)
}

// Search implements provider.Provider.
func (p *Provider) Search(ctx context.Context, v *provider.Video, langs []provider.Language) ([]provider.Subtitle, error) {
	if !slices.ContainsFunc(langs, func(l provider.Language) bool { return l.Tag == language }) {
		return nil, nil
	}
	payload := url.Values{}
	if v.Episode {
		payload.Set("nume", v.Series)
		payload.Set("sezon", pyValue(v.Season))
		payload.Set("episod", pyValue(v.Number))
	} else {
		payload.Set("nume", v.Title)
	}
	payload.Set("an", pyValue(v.Year))
	// As Bazarr's, the payload is sent both in the query and the body.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.api+"?"+payload.Encode(), strings.NewReader(payload.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("RL-API", apiKey)
	r, err := p.do(req)
	if err != nil {
		return nil, err
	}
	if r.Status != http.StatusOK {
		return nil, nil
	}
	var raw json.RawMessage
	if err := json.Unmarshal(r.Body, &raw); err != nil {
		return nil, provider.Errorf(provider.ProviderError, "unable to parse JSON response")
	}
	// An empty answer is an empty list or object.
	var empty []any
	if json.Unmarshal(raw, &empty) == nil {
		return nil, nil
	}
	var res struct {
		Results map[string]struct {
			Subtitles map[string]struct {
				Title  string `json:"titlu"`
				URL    string `json:"url"`
				Rating struct {
					Score any `json:"nota"`
				} `json:"rating"`
			} `json:"subtitrari"`
		} `json:"rezultate"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return nil, provider.Errorf(provider.ProviderError, "unable to parse JSON response: %w", err)
	}
	var out []provider.Subtitle
	for _, film := range slices.Sorted(maps.Keys(res.Results)) {
		subs := res.Results[film].Subtitles
		for _, id := range slices.Sorted(maps.Keys(subs)) {
			s := subs[id]
			out = append(out, provider.Subtitle{
				ID:           s.URL,
				Language:     provider.Language{Tag: language},
				ReleaseNames: []string{s.Title},
				Matches:      matches(v, s.Title),
				PageLink:     s.URL,
			})
		}
	}
	return out, nil
}

// matches are what the search matched, as Bazarr's get_matches: the
// title, and the release group if the file name has it, which Bazarr
// takes for the hash too.
func matches(v *provider.Video, filename string) []string {
	m := []string{"title", "year"}
	if v.Episode {
		m = []string{"title", "series", "season", "episode", "year"}
	}
	if v.ReleaseGroup != "" && strings.Contains(strings.ToLower(filename), strings.ToLower(v.ReleaseGroup)) {
		m = append(m, "release_group", "hash")
	}
	return m
}

// extensions are subliminal's subtitle extensions but .txt, which some
// archives have for other files.
var extensions = []string{".srt", ".sub", ".smi", ".ssa", ".ass", ".mpl"}

// Download implements provider.Provider: id is the subtitle's link.
func (p *Provider) Download(ctx context.Context, _ *provider.Video, id string) (provider.File, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, id, nil)
	if err != nil {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "malformed subtitle link %q", id)
	}
	req.Header.Set("User-Agent", downloadAgent)
	req.Header.Set("Accept", "application/octet-stream, */*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Referer", "https://subtitrari.regielive.ro")
	r, err := p.do(req)
	if err != nil {
		return provider.File{}, err
	}
	if string(r.Body) == "500" {
		return provider.File{}, provider.Errorf(provider.ProviderError, "RegieLive download failed: server returned HTTP 500")
	}
	if r.Status != http.StatusOK {
		return provider.File{}, provider.Errorf(provider.ProviderError, "RegieLive download failed: unexpected status %d%s", r.Status, detail(r.Body, 400))
	}
	a, err := archive.Open(r.Body)
	if err != nil || !isZip(r.Body) {
		return provider.File{}, provider.Errorf(provider.ProviderError, "RegieLive download failed: provider returned an invalid archive payload%s", detail(r.Body, 200))
	}
	for _, name := range a.Names() {
		if strings.HasPrefix(path.Base(name), ".") || !slices.ContainsFunc(extensions, func(ext string) bool {
			return strings.HasSuffix(strings.ToLower(name), ext)
		}) {
			continue
		}
		data, err := a.Read(name)
		if err != nil {
			return provider.File{}, provider.Errorf(provider.BadSubtitle, "%s: %w", name, err)
		}
		data = archive.FixLineEndings(data)
		return provider.File{Data: data, Format: format(name, data), Language: provider.Language{Tag: language}}, nil
	}
	return provider.File{}, provider.Errorf(provider.ProviderError, "RegieLive download failed: archive did not contain a subtitle file")
}

// isZip reports whether data is a zip archive.
func isZip(data []byte) bool {
	_, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	return err == nil
}

// detail is the start of an answer, for errors.
func detail(body []byte, n int) string {
	d := strings.ReplaceAll(strings.TrimSpace(string(body)), "\n", " ")
	if len(d) > n {
		d = d[:n]
	}
	if d == "" {
		return ""
	}
	return " (" + d + ")"
}

// format is a subtitle's format by its name, or its content.
func format(name string, data []byte) string {
	switch ext := strings.TrimPrefix(strings.ToLower(path.Ext(name)), "."); ext {
	case "srt", "ass", "ssa", "vtt":
		return ext
	}
	return archive.Detect(data)
}

// String names the provider in logs.
func (p *Provider) String() string { return Name }
