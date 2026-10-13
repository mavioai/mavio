// Package animetoshoxyz is the provider of AnimeTosho.xyz, ported from
// Bazarr's animetosho_xyz: the subtitle tracks of the anime releases the
// site mirrors, found by the episode's AniDB ID and served
// xz-compressed.
package animetoshoxyz

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/ulikunitz/xz"

	"github.com/mavioai/mavio/plugins/subtitles/internal/anidb"
	"github.com/mavioai/mavio/plugins/subtitles/internal/archive"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/site"
)

// Name is the provider's name.
const Name = "animetosho_xyz"

// baseURL serves the site's feed.
const baseURL = "https://feed.animetosho.xyz"

// userAgent identifies the plugin, as Bazarr's SZ_USER_AGENT.
const userAgent = "Mavio"

// maxSubtitle bounds a decompressed subtitle.
const maxSubtitle = 10 << 20

// xzMagic starts xz files.
var xzMagic = []byte("\xFD7zXZ\x00")

// languages are the site's, as Bazarr's supported_languages.
var languages = []string{
	"ar", "en", "fi", "fr", "de", "he", "id", "it", "ja", "pt", "pl", "ru", "es", "sv", "th", "tr", "vi", "pt-BR",
}

// brazilian tells Brazilian Portuguese by its language name: "Portuguese[BR]"
// or "Brazilian Portuguese".
var brazilian = regexp.MustCompile(`(?i)brazil|\bbr\b`)

// Definition returns the provider's definition.
func Definition() provider.Definition {
	return provider.Definition{
		Name: Name, DisplayName: "AnimeTosho.xyz",
		Settings:  anidb.Settings(),
		Languages: languages,
		Episodes:  true,
		New:       New,
	}
}

// New makes the provider from its settings.
func New(env provider.Env, raw json.RawMessage) (provider.Provider, error) {
	var s anidb.Config
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, provider.Errorf(provider.ConfigurationError, "settings: %w", err)
		}
	}
	if err := s.Validate(); err != nil {
		return nil, err
	}
	sess := site.New(env.HTTP)
	sess.Header.Set("User-Agent", userAgent)
	sess.Check = checked
	return &Provider{env: env, sess: sess, anidb: anidb.New(env, s), base: baseURL}, nil
}

// Provider is AnimeTosho.xyz.
type Provider struct {
	env   provider.Env
	sess  *site.Session
	anidb *anidb.Client
	base  string
}

// SetBaseURL points the provider's feed at another server, for tests.
func (p *Provider) SetBaseURL(u *url.URL) { p.base = strings.TrimSuffix(u.String(), "/") }

// AniDB returns the client the provider finds AniDB IDs with.
func (p *Provider) AniDB() *anidb.Client { return p.anidb }

// checked maps the site's answers to failures, as Bazarr's checked.
func checked(r *site.Response) error {
	switch r.Status {
	case http.StatusOK:
		return nil
	case http.StatusNotFound:
		return provider.Errorf(provider.ProviderError, "endpoint not found: %s", r.URL.Path)
	}
	return provider.Errorf(provider.ProviderError, "HTTP error %d", r.Status)
}

// entry is a release of the feed.
type entry struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	Timestamp int64  `json:"timestamp"`
}

// flag is the site's boolean or 0/1 flag.
type flag bool

// UnmarshalJSON reads booleans, 0/1 and null.
func (f *flag) UnmarshalJSON(b []byte) error {
	switch s := string(b); s {
	case "true":
		*f = true
	case "false", "null", "0", `""`:
		*f = false
	default:
		n, err := strconv.ParseFloat(strings.Trim(s, `"`), 64)
		*f = flag(err == nil && n != 0)
	}
	return nil
}

// attachment is a file of a release.
type attachment struct {
	ID   json.RawMessage `json:"id"`
	Type string          `json:"type"`
	URL  string          `json:"url"`
	Info struct {
		Language     string `json:"language"`
		LanguageCode string `json:"language_code"`
		Format       string `json:"format"`
		Forced       flag   `json:"forced"`
	} `json:"info"`
}

// torrent is a release's details.
type torrent struct {
	ID          int64        `json:"id"`
	TorrentName string       `json:"torrent_name"`
	Attachments []attachment `json:"attachments"`
	Files       []struct {
		Attachments []attachment `json:"attachments"`
	} `json:"files"`
}

// subtitles returns the release's subtitle attachments, as Bazarr's
// _iter_subtitle_attachments: those of the release and of its files,
// each once.
func (t *torrent) subtitles() []attachment {
	type key struct{ id, url string }
	seen := map[key]bool{}
	var out []attachment
	collect := func(as []attachment) {
		for _, a := range as {
			k := key{string(a.ID), a.URL}
			if a.Type != "subtitle" || seen[k] {
				continue
			}
			seen[k] = true
			out = append(out, a)
		}
	}
	collect(t.Attachments)
	for _, f := range t.Files {
		collect(f.Attachments)
	}
	return out
}

// Search implements provider.Provider.
func (p *Provider) Search(ctx context.Context, v *provider.Video, langs []provider.Language) ([]provider.Subtitle, error) {
	ids, err := p.anidb.Refine(ctx, v, true)
	if err != nil {
		return nil, err
	}
	if ids.Episode == 0 {
		p.env.Log.DebugContext(ctx, "skipping the video: not an anime, or its AniDB episode ID is unknown", "series", v.Series)
		return nil, nil
	}
	entries, err := p.entries(ctx, ids.Episode)
	if err != nil {
		return nil, err
	}
	var out []provider.Subtitle
	for _, e := range entries {
		r, err := p.sess.Get(ctx, p.base+"/json", url.Values{"show": {"torrent"}, "id": {strconv.FormatInt(e.ID, 10)}})
		if err != nil {
			return nil, err
		}
		var t torrent
		if err := r.JSON(&t); err != nil {
			return nil, err
		}
		for _, a := range t.subtitles() {
			lang, ok := language(a.Info.LanguageCode, a.Info.Language, bool(a.Info.Forced))
			if !ok {
				p.env.Log.DebugContext(ctx, "unknown language", "code", a.Info.LanguageCode)
				continue
			}
			if !slices.Contains(langs, lang) {
				continue
			}
			var names []string
			for _, n := range []string{e.Title, t.TorrentName} {
				if n != "" && !slices.Contains(names, n) {
					names = append(names, n)
				}
			}
			format := strings.ToLower(a.Info.Format)
			if !slices.Contains([]string{"srt", "ass", "ssa", "vtt", "sub"}, format) {
				format = ""
			}
			out = append(out, provider.Subtitle{
				// Releases may share an attachment: the release keeps them
				// apart.
				ID:           fmt.Sprintf("%d:%s", t.ID, a.URL),
				Language:     lang,
				ReleaseNames: names,
				Matches:      []string{"series", "season", "episode"},
				Format:       format,
				PageLink:     a.URL,
			})
		}
	}
	return out, nil
}

// language reads an attachment's language, as Bazarr's: English when it
// has none, Brazilian Portuguese told from Portuguese by its name only,
// and forced when the track is.
func language(code, name string, forced bool) (provider.Language, bool) {
	tag := provider.FromCode(cmp.Or(code, "eng"))
	if tag == "" {
		return provider.Language{}, false
	}
	if tag == "pt" && brazilian.MatchString(name) {
		tag = "pt-BR"
	}
	return provider.Language{Tag: tag, Forced: forced}, true
}

// entries returns the complete releases of an episode, latest first, as
// Bazarr's _get_series_entries.
func (p *Provider) entries(ctx context.Context, episodeID int) ([]entry, error) {
	r, err := p.sess.Get(ctx, p.base+"/feed/json", url.Values{"eid": {strconv.Itoa(episodeID)}})
	if err != nil {
		return nil, err
	}
	var all []entry
	if err := r.JSON(&all); err != nil {
		return nil, err
	}
	var out []entry
	for _, e := range all {
		if e.Status == "complete" {
			out = append(out, e)
		}
	}
	slices.SortStableFunc(out, func(a, b entry) int { return cmp.Compare(b.Timestamp, a.Timestamp) })
	return out, nil
}

// Download implements provider.Provider.
func (p *Provider) Download(ctx context.Context, _ *provider.Video, id string) (provider.File, error) {
	_, link, ok := strings.Cut(id, ":")
	if u, err := url.Parse(link); !ok || err != nil || u.Scheme != "http" && u.Scheme != "https" {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "malformed subtitle ID %q", id)
	}
	r, err := p.sess.Get(ctx, link, nil)
	if err != nil {
		return provider.File{}, err
	}
	data, err := decompress(r.Body)
	if err != nil {
		return provider.File{}, err
	}
	return provider.File{Data: data, Format: archive.Detect(data)}, nil
}

// decompress decompresses an xz file, as Bazarr's download_subtitle.
func decompress(data []byte) ([]byte, error) {
	if !bytes.HasPrefix(data, xzMagic) {
		return nil, provider.Errorf(provider.ProviderError, "unidentified archive type")
	}
	zr, err := xz.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, provider.Errorf(provider.ProviderError, "xz: %w", err)
	}
	out, err := io.ReadAll(io.LimitReader(zr, maxSubtitle+1))
	switch {
	case err != nil:
		return nil, provider.Errorf(provider.ProviderError, "xz: %w", err)
	case len(out) > maxSubtitle:
		return nil, provider.Errorf(provider.BadSubtitle, "larger than %d bytes", maxSubtitle)
	}
	return out, nil
}
