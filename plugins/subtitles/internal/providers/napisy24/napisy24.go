// Package napisy24 is the provider of Napisy24, ported from Bazarr's
// napisy24: Polish subtitles of movies and episodes found by the video
// file's OpenSubtitles hash, size and name.
package napisy24

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mavioai/mavio/plugins/subtitles/internal/archive"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/site"
)

// Name is the provider's name.
const Name = "napisy24"

// language is the provider's only language.
const language = "pl"

// The account of subliminal's provider, used when none is set.
const (
	defaultUsername = "subliminal"
	defaultPassword = "lanimilbus"
)

// Definition returns the provider's definition.
func Definition() provider.Definition {
	return provider.Definition{
		Name: Name, DisplayName: "Napisy24",
		Settings: []provider.Setting{
			{Name: "username", Label: "Username", Help: "The provided credentials must have API access. Leave empty to use the defaults.", Type: provider.Text},
			{Name: "password", Label: "Password", Type: provider.Password},
		},
		Languages:      []string{language},
		HashVerifiable: true, Movies: true, Episodes: true,
		New: New,
	}
}

// settings are the provider's settings.
type settings struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// New makes the provider from its settings: subliminal's account unless
// both a username and a password are set.
func New(env provider.Env, raw json.RawMessage) (provider.Provider, error) {
	var s settings
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, provider.Errorf(provider.ConfigurationError, "settings: %w", err)
		}
	}
	if s.Username == "" || s.Password == "" {
		s = settings{Username: defaultUsername, Password: defaultPassword}
	}
	ss := site.New(env.HTTP)
	ss.Header.Set("User-Agent", "Subliminal/2.1")
	return &Provider{env: env, s: s, site: ss, api: "http://napisy24.pl/run/CheckSubAgent.php"}, nil
}

// Provider is Napisy24.
type Provider struct {
	env  provider.Env
	s    settings
	site *site.Session
	api  string
}

// SetBaseURL points the provider at another server, for tests.
func (p *Provider) SetBaseURL(u *url.URL) {
	p.api = strings.TrimSuffix(u.String(), "/") + "/run/CheckSubAgent.php"
}

// found is a subtitle the site has for a video file.
type found struct {
	imdbID string
	zip    []byte
}

// query asks the site for the subtitle of a video file, as Bazarr's
// query; nil when it has none.
func (p *Provider) query(ctx context.Context, v *provider.Video) (*found, error) {
	form := url.Values{
		"postAction": {"CheckSub"},
		"ua":         {p.s.Username},
		"ap":         {p.s.Password},
		"fs":         {strconv.FormatInt(v.FileSize, 10)},
		"fh":         {v.FileHash},
		"fn":         {filepath.Base(strings.ReplaceAll(v.FileName, `\`, "/"))},
		"n24pref":    {"1"},
	}
	r, err := p.site.PostForm(ctx, p.api, form)
	if err != nil {
		return nil, err
	}
	head, content, _ := bytes.Cut(r.Body, []byte("||"))
	data := string(head)
	if !strings.HasPrefix(data, "OK") {
		if strings.HasPrefix(data, "login error") {
			return nil, provider.Errorf(provider.AuthenticationError, "login failed")
		}
		p.env.Log.ErrorContext(ctx, "unknown response", "response", string(r.Body))
		return nil, nil
	}
	status := data[:min(len(data), 4)]
	info := map[string]string{}
	for i, part := range strings.Split(data, "|") {
		if k, v, ok := strings.Cut(part, ":"); ok && i > 0 {
			info[k] = v
		}
	}
	switch status {
	case "OK-0":
		p.env.Log.InfoContext(ctx, "no subtitles found")
		return nil, nil
	case "OK-1":
		p.env.Log.InfoContext(ctx, "no subtitles found but got video info")
		return nil, nil
	case "OK-3":
		p.env.Log.InfoContext(ctx, "found subtitles but not from Napisy24 database")
		return nil, nil
	}
	imdb := info["imdb"]
	if len(imdb) < 7 {
		imdb = strings.Repeat("0", 7-len(imdb)) + imdb
	}
	return &found{imdbID: "tt" + imdb, zip: content}, nil
}

// Search implements provider.Provider: the video file's hash is needed,
// as Bazarr's required_hash.
func (p *Provider) Search(ctx context.Context, v *provider.Video, langs []provider.Language) ([]provider.Subtitle, error) {
	if v.FileHash == "" {
		return nil, nil
	}
	var out []provider.Subtitle
	for _, l := range langs {
		if l.Tag != language {
			continue
		}
		f, err := p.query(ctx, v)
		if err != nil {
			return nil, err
		}
		if f == nil {
			continue
		}
		matches := []string{"hash"}
		if imdb := v.ExternalIDs["imdb"]; imdb != "" && imdb == f.imdbID {
			matches = append(matches, "imdb_id")
		}
		out = append(out, provider.Subtitle{ID: v.FileHash, Language: l, Matches: matches})
	}
	return out, nil
}

// Download implements provider.Provider: the subtitle is asked for again,
// as the site gives it with the search, for the video's file.
func (p *Provider) Download(ctx context.Context, v *provider.Video, id string) (provider.File, error) {
	if v == nil || v.FileHash == "" {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "the video file of subtitle %s is unknown", id)
	}
	if v.FileHash != id {
		v2 := *v
		v2.FileHash = id
		v = &v2
	}
	f, err := p.query(ctx, v)
	if err != nil {
		return provider.File{}, err
	}
	if f == nil {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "subtitle %s is gone", id)
	}
	a, err := archive.Open(f.zip)
	if err != nil || len(a.Names()) == 0 {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "the subtitle is not in a zip archive: %v", err)
	}
	data, err := a.Read(a.Names()[0])
	if err != nil {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "%s: %w", a.Names()[0], err)
	}
	data = archive.FixLineEndings(data)
	return provider.File{Data: data, Format: archive.Detect(data), Language: provider.Language{Tag: language}}, nil
}

// String names the provider in logs.
func (p *Provider) String() string { return fmt.Sprintf("%s(%s)", Name, p.s.Username) }
