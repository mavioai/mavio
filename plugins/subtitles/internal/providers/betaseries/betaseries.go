// Package betaseries is the provider of BetaSeries, ported from Bazarr's
// betaseries: French and English subtitles of episodes found by their
// TVDB IDs through the API of an account's key.
package betaseries

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/mavioai/mavio/plugins/subtitles/internal/archive"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/site"
)

// Name is the provider's name.
const Name = "betaseries"

// userAgent identifies the plugin, as Bazarr's SZ_USER_AGENT.
const userAgent = "Mavio/0.1"

// Definition returns the provider's definition.
func Definition() provider.Definition {
	return provider.Definition{
		Name: Name, DisplayName: "BetaSeries",
		Settings: []provider.Setting{
			{Name: "token", Label: "API KEY", Help: "French / English Provider for TV Shows Only", Type: provider.Text},
		},
		Languages: []string{"fr", "en"},
		Episodes:  true,
		New:       New,
	}
}

type settings struct {
	Token string `json:"token"`
}

// New makes the provider from its settings.
func New(env provider.Env, raw json.RawMessage) (provider.Provider, error) {
	var s settings
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, provider.Errorf(provider.ConfigurationError, "settings: %w", err)
		}
	}
	if s.Token == "" {
		return nil, provider.Errorf(provider.ConfigurationError, "token must be specified")
	}
	sess := site.New(env.HTTP)
	sess.Header.Set("User-Agent", userAgent)
	sess.Check = func(*site.Response) error { return nil }
	return &Provider{env: env, s: s, sess: sess, api: "https://api.betaseries.com/"}, nil
}

// Provider is BetaSeries.
type Provider struct {
	env  provider.Env
	s    settings
	sess *site.Session
	api  string
}

// SetBaseURL points the provider at another server, for tests.
func (p *Provider) SetBaseURL(u *url.URL) { p.api = strings.TrimSuffix(u.String(), "/") + "/" }

type apiError struct {
	Code int `json:"code"`
}

type subtitle struct {
	ID       json.Number `json:"id"`
	Language string      `json:"language"`
	Source   string      `json:"source"`
	File     string      `json:"file"`
	URL      string      `json:"url"`
}

// fromSite maps the site's languages: original version is English,
// French version French.
func fromSite(code string) string {
	switch strings.ToLower(code) {
	case "vo":
		return "en"
	case "vf":
		return "fr"
	}
	return ""
}

// Search implements provider.Provider, as Bazarr's query: the episode by
// its TVDB ID, else by its series' and its numbers.
func (p *Provider) Search(ctx context.Context, v *provider.Video, langs []provider.Language) ([]provider.Subtitle, error) {
	var (
		endpoint string
		params   = url.Values{"key": {p.s.Token}, "v": {"3.0"}, "subtitles": {"1"}}
		match    string
	)
	switch {
	case v.ExternalIDs["tvdb"] != "":
		endpoint, match = "episodes/display", "tvdb_id"
		params.Set("thetvdb_id", v.ExternalIDs["tvdb"])
	case v.SeriesExternalIDs["tvdb"] != "":
		endpoint, match = "shows/episodes", "series_tvdb_id"
		params.Set("thetvdb_id", v.SeriesExternalIDs["tvdb"])
		params.Set("season", strconv.Itoa(v.Season))
		params.Set("episode", strconv.Itoa(v.Number))
	default:
		p.env.Log.DebugContext(ctx, "the show has no TVDB ID: the search can't be done")
		return nil, nil
	}
	r, err := p.sess.Get(ctx, p.api+endpoint, params)
	if err != nil {
		return nil, err
	}
	var res struct {
		Errors  []apiError `json:"errors"`
		Episode *struct {
			Subtitles []subtitle `json:"subtitles"`
		} `json:"episode"`
		Episodes []struct {
			Subtitles []subtitle `json:"subtitles"`
		} `json:"episodes"`
	}
	jsonErr := json.Unmarshal(r.Body, &res)
	if r.Status == 400 && jsonErr == nil && len(res.Errors) > 0 {
		switch res.Errors[0].Code {
		case 4001:
			// No series found.
			return nil, nil
		case 1001:
			return nil, provider.Errorf(provider.AuthenticationError, "invalid token provided")
		}
	}
	if r.Status >= 400 {
		return nil, provider.Errorf(provider.ProviderError, "%s: bad status code: %d", r.URL.Path, r.Status)
	}
	if jsonErr != nil {
		return nil, provider.Errorf(provider.ParseResponseError, "%s: %w", r.URL.Path, jsonErr)
	}
	if len(res.Errors) > 0 {
		p.env.Log.DebugContext(ctx, "status error", "errors", res.Errors)
		return nil, nil
	}
	var subs []subtitle
	switch {
	case res.Episode != nil:
		subs = res.Episode.Subtitles
	case len(res.Episodes) > 0:
		subs = res.Episodes[0].Subtitles
	}

	var out []provider.Subtitle
	for _, s := range subs {
		tag := fromSite(s.Language)
		// SeriesSub has shut down: its links are dead.
		if tag == "" || s.Source == "seriessub" || !slices.ContainsFunc(langs, func(l provider.Language) bool { return l.Tag == tag }) {
			continue
		}
		out = append(out, provider.Subtitle{
			ID:           s.URL,
			Language:     provider.Language{Tag: tag},
			ReleaseNames: []string{s.File},
			Matches:      []string{match},
			PageLink:     s.URL,
			Comment:      s.Source,
		})
	}
	return out, nil
}

// extensions are subliminal's SUBTITLE_EXTENSIONS.
var extensions = []string{".srt", ".sub", ".smi", ".txt", ".ssa", ".ass", ".mpl"}

// Download implements provider.Provider: the subtitle, or that of an
// archive named for the video's release group, else its first.
func (p *Provider) Download(ctx context.Context, v *provider.Video, id string) (provider.File, error) {
	r, err := p.sess.Get(ctx, id, nil)
	if err != nil {
		return provider.File{}, err
	}
	switch {
	case r.Status == 404:
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "%s: not found", r.URL.Path)
	case r.Status >= 400:
		return provider.File{}, provider.Errorf(provider.ProviderError, "%s: bad status code: %d", r.URL.Path, r.Status)
	}
	a, err := archive.Open(r.Body)
	if errors.Is(err, archive.ErrNotArchive) {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "the download is not a subtitle or an archive")
	}
	if err != nil {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "%w", err)
	}
	var names []string
	for _, n := range a.Names() {
		if !strings.HasPrefix(path.Base(n), ".") && slices.ContainsFunc(extensions, func(e string) bool { return strings.HasSuffix(strings.ToLower(n), e) }) {
			names = append(names, n)
		}
	}
	if len(names) == 0 {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "no subtitle in the archive")
	}
	name := names[0]
	if v != nil && v.ReleaseGroup != "" {
		if i := slices.IndexFunc(names, func(n string) bool { return strings.Contains(n, v.ReleaseGroup) }); i >= 0 {
			name = names[i]
		}
	}
	data, err := a.Read(name)
	if err != nil {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "%s: %w", name, err)
	}
	if len(data) == 0 {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "%s: empty", name)
	}
	data = archive.FixLineEndings(data)
	format := strings.TrimPrefix(strings.ToLower(path.Ext(name)), ".")
	switch format {
	case "srt", "ass", "ssa":
	default:
		format = archive.Detect(data)
	}
	return provider.File{Data: data, Format: format}, nil
}
