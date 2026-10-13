// Package hdbits is the provider of HDBits.org, ported from Bazarr's
// hdbits: the subtitles of the private tracker's torrents of a movie's
// IMDb ID or a season's TVDB ID, through the API of a member's passkey.
package hdbits

import (
	"context"
	"encoding/json"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/text/language"

	"github.com/mavioai/mavio/plugins/subtitles/internal/archive"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/site"
)

// Name is the provider's name.
const Name = "hdbits"

// userAgent identifies the plugin, as Bazarr's.
const userAgent = "Mavio/0.1"

// Definition returns the provider's definition.
func Definition() provider.Definition {
	return provider.Definition{
		Name: Name, DisplayName: "HDBits.org",
		Settings: []provider.Setting{
			{Name: "username", Label: "Username", Help: "You must have 2FA enabled and whitelist your IP if you are running from a server.", Type: provider.Text},
			{Name: "passkey", Label: "Your profile's passkey", Type: provider.Password},
		},
		Languages:                 languages(),
		HearingImpairedVerifiable: true,
		Movies:                    true, Episodes: true,
		New: New,
	}
}

// languages are those with ISO 639-1 codes, and Brazilian Portuguese.
var languages = sync.OnceValue(func() []string {
	out := []string{"pt-BR"}
	for a := 'a'; a <= 'z'; a++ {
		for b := 'a'; b <= 'z'; b++ {
			code := string([]rune{a, b})
			if base, err := language.ParseBase(code); err == nil && base.String() == code {
				out = append(out, code)
			}
		}
	}
	return out
})

type settings struct {
	Username string `json:"username"`
	Passkey  string `json:"passkey"`
}

// New makes the provider from its settings.
func New(env provider.Env, raw json.RawMessage) (provider.Provider, error) {
	var s settings
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, provider.Errorf(provider.ConfigurationError, "settings: %w", err)
		}
	}
	if s.Username == "" || s.Passkey == "" {
		return nil, provider.Errorf(provider.ConfigurationError, "username and passkey must be specified")
	}
	sess := site.New(env.HTTP)
	sess.Header.Set("User-Agent", userAgent)
	sess.Check = func(*site.Response) error { return nil }
	return &Provider{env: env, s: s, sess: sess, base: "https://hdbits.org", sleep: site.Sleep}, nil
}

// Provider is HDBits.org.
type Provider struct {
	env   provider.Env
	s     settings
	sess  *site.Session
	base  string
	sleep func(context.Context, time.Duration) error
}

// SetBaseURL points the provider at another server, for tests.
func (p *Provider) SetBaseURL(u *url.URL) { p.base = strings.TrimSuffix(u.String(), "/") }

// specialLanguages are the site's codes that are not languages'.
var specialLanguages = map[string]string{"uk": "en", "br": "pt-BR", "gr": "el", "se": "sv"}

// fromSite maps the site's language codes; "" for unknown ones.
func fromSite(code string) string {
	if tag, ok := specialLanguages[code]; ok {
		return tag
	}
	return provider.FromCode(code)
}

var (
	// extensions are those of the subtitles listed.
	extensions = []string{".ass", ".srt", ".zip", ".rar"}
	// unwanted are subtitles of extras, commentaries, lyrics or forced
	// parts.
	unwanted = regexp.MustCompile(`extra|commentary|lyrics|forced`)
)

func (p *Provider) post(ctx context.Context, path string, body map[string]any) (*site.Response, error) {
	body["username"], body["passkey"] = p.s.Username, p.s.Passkey
	return p.sess.PostJSON(ctx, p.base+path, body)
}

func raiseForStatus(r *site.Response) error {
	if r.Status >= 400 {
		return provider.Errorf(provider.ProviderError, "%s: bad status code: %d", r.URL.Path, r.Status)
	}
	return nil
}

// Search implements provider.Provider, as Bazarr's list_subtitles: the
// subtitles of each torrent of the movie, or of the episode's season.
func (p *Provider) Search(ctx context.Context, v *provider.Video, langs []provider.Language) ([]provider.Subtitle, error) {
	var (
		lookup  map[string]any
		matches []string
		episode = -1
	)
	if v.Episode {
		var id any = v.SeriesExternalIDs["tvdb"]
		if n, err := strconv.Atoi(v.SeriesExternalIDs["tvdb"]); err == nil {
			id = n
		}
		lookup = map[string]any{"tvdb": map[string]any{"id": id, "season": v.Season}}
		matches = []string{"tvdb_id", "imdb_id", "series", "title", "episode", "season"}
		episode = v.Number
	} else {
		lookup = map[string]any{"imdb": map[string]any{"id": strings.TrimLeft(v.ExternalIDs["imdb"], "t")}}
		matches = []string{"imdb_id", "title", "year"}
	}
	r, err := p.post(ctx, "/api/torrents", lookup)
	if err != nil {
		return nil, err
	}
	if err := raiseForStatus(r); err != nil {
		return nil, err
	}
	var torrents struct {
		Data *[]struct {
			ID json.Number `json:"id"`
		} `json:"data"`
	}
	if err := r.JSON(&torrents); err != nil {
		return nil, err
	}
	if torrents.Data == nil {
		p.env.Log.DebugContext(ctx, "no data found")
		return nil, nil
	}
	var out []provider.Subtitle
	for _, t := range *torrents.Data {
		subs, err := p.subtitles(ctx, t.ID, langs, episode, matches)
		if err != nil {
			return nil, err
		}
		out = append(out, subs...)
		if err := p.sleep(ctx, 500*time.Millisecond); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// subtitles lists a torrent's subtitles, as Bazarr's _parse_subtitles.
func (p *Provider) subtitles(ctx context.Context, torrent json.Number, langs []provider.Language, episode int, matches []string) ([]provider.Subtitle, error) {
	r, err := p.post(ctx, "/api/subtitles", map[string]any{"torrent_id": torrent})
	if err != nil {
		return nil, err
	}
	var res struct {
		Data []struct {
			ID       json.Number `json:"id"`
			Title    string      `json:"title"`
			Filename string      `json:"filename"`
			Language string      `json:"language"`
		} `json:"data"`
	}
	if json.Unmarshal(r.Body, &res) != nil {
		p.env.Log.DebugContext(ctx, "couldn't get the subtitles of a torrent", "torrent", torrent)
		return nil, nil
	}
	var out []provider.Subtitle
	for _, s := range res.Data {
		if !slices.ContainsFunc(extensions, func(e string) bool { return strings.HasSuffix(s.Filename, e) }) {
			continue
		}
		tag := fromSite(s.Language)
		if tag == "" || unwanted.MatchString(strings.ToLower(s.Title)) || unwanted.MatchString(strings.ToLower(s.Filename)) {
			continue
		}
		if !slices.ContainsFunc(langs, func(l provider.Language) bool { return l.Tag == tag }) {
			continue
		}
		if episode >= 0 {
			if n, ok := archive.EpisodeNumber(s.Title); ok && n != episode {
				continue
			}
		}
		out = append(out, provider.Subtitle{
			ID:           s.ID.String(),
			Language:     provider.Language{Tag: tag},
			ReleaseNames: []string{s.Title},
			Matches:      slices.Clone(matches),
		})
	}
	return out, nil
}

// Download implements provider.Provider: the subtitle, or the episode's
// of an archive.
func (p *Provider) Download(ctx context.Context, v *provider.Video, id string) (provider.File, error) {
	r, err := p.sess.Get(ctx, p.base+"/getdox.php", url.Values{"id": {id}, "passkey": {p.s.Passkey}})
	if err != nil {
		return provider.File{}, err
	}
	if err := raiseForStatus(r); err != nil {
		return provider.File{}, err
	}
	w := archive.Want{}
	if v != nil && v.Episode {
		w.Episode = v.Number
	}
	return archive.Extract(r.Body, w, provider.Language{})
}
