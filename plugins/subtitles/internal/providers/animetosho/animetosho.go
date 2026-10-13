// Package animetosho is the provider of Anime Tosho, ported from
// Bazarr's animetosho: the subtitle tracks of the anime releases the site
// mirrors, found by the episode's AniDB ID and served xz-compressed.
package animetosho

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
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
const Name = "animetosho"

// The site's feed and the storage of its attachments.
const (
	feedURL    = "https://feed.animetosho.org/json"
	storageURL = "https://animetosho.org/storage/attach/"
)

// maxSubtitle bounds a decompressed subtitle.
const maxSubtitle = 10 << 20

// xzMagic starts xz files.
var xzMagic = []byte("\xFD7zXZ\x00")

// languages are the site's, as Bazarr's supported_languages.
var languages = []string{
	"ar", "en", "fi", "fr", "de", "he", "id", "it", "ja", "pt", "pl", "ru", "es", "sv", "th", "tr", "vi", "pt-BR",
}

// Definition returns the provider's definition.
func Definition() provider.Definition {
	return provider.Definition{
		Name: Name, DisplayName: "Anime Tosho",
		Settings: append([]provider.Setting{{
			Name: "search_threshold", Label: "Search Threshold", Type: provider.Number, Default: 6,
			Help: "Increase if you often cannot find subtitles for your Anime. Note that increasing the value will decrease the performance of the search for each Episode.",
		}}, anidb.Settings()...),
		Languages: languages,
		Episodes:  true,
		New:       New,
	}
}

// settings are the provider's settings.
type settings struct {
	SearchThreshold *int `json:"search_threshold"`
	anidb.Config
}

// New makes the provider from its settings.
func New(env provider.Env, raw json.RawMessage) (provider.Provider, error) {
	var s settings
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, provider.Errorf(provider.ConfigurationError, "settings: %w", err)
		}
	}
	threshold := 6
	if s.SearchThreshold != nil {
		threshold = *s.SearchThreshold
	}
	if threshold < 1 || threshold > 15 {
		return nil, provider.Errorf(provider.ConfigurationError, "search threshold %d is not from 1 to 15", threshold)
	}
	if err := s.Validate(); err != nil {
		return nil, err
	}
	sess := site.New(env.HTTP)
	return &Provider{
		env: env, threshold: threshold, sess: sess, anidb: anidb.New(env, s.Config),
		feedURL: feedURL, storageURL: storageURL,
	}, nil
}

// Provider is Anime Tosho.
type Provider struct {
	env       provider.Env
	threshold int
	sess      *site.Session
	anidb     *anidb.Client

	feedURL, storageURL string
}

// SetBaseURL points the provider's feed and storage at another server,
// for tests.
func (p *Provider) SetBaseURL(u *url.URL) {
	base := strings.TrimSuffix(u.String(), "/")
	p.feedURL, p.storageURL = base+"/json", base+"/storage/attach/"
}

// AniDB returns the client the provider finds AniDB IDs with.
func (p *Provider) AniDB() *anidb.Client { return p.anidb }

// entry is a release of the feed.
type entry struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	Timestamp int64  `json:"timestamp"`
}

// flag is the site's 0/1 or boolean flag.
type flag bool

// UnmarshalJSON reads 0/1, booleans and null.
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

// torrent is a release's details.
type torrent struct {
	Files []struct {
		Filename    string `json:"filename"`
		Attachments []struct {
			ID   int64  `json:"id"`
			Type string `json:"type"`
			Info struct {
				Lang   string `json:"lang"`
				Name   string `json:"name"`
				Codec  string `json:"codec"`
				Forced flag   `json:"forced"`
			} `json:"info"`
		} `json:"attachments"`
	} `json:"files"`
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
		r, err := p.sess.Get(ctx, p.feedURL, url.Values{"show": {"torrent"}, "id": {strconv.FormatInt(e.ID, 10)}})
		if err != nil {
			return nil, err
		}
		var t torrent
		if err := r.JSON(&t); err != nil {
			return nil, err
		}
		for _, f := range t.Files {
			for _, a := range f.Attachments {
				if a.Type != "subtitle" {
					continue
				}
				lang, ok := language(a.Info.Lang, a.Info.Name, bool(a.Info.Forced))
				if !ok {
					p.env.Log.DebugContext(ctx, "unknown language", "code", a.Info.Lang)
					continue
				}
				if !slices.Contains(langs, lang) {
					continue
				}
				link := fmt.Sprintf("%s%08x/%d.xz", p.storageURL, a.ID, a.ID)
				var names []string
				for _, n := range []string{e.Title, f.Filename} {
					if n != "" && !slices.Contains(names, n) {
						names = append(names, n)
					}
				}
				format := strings.ToLower(a.Info.Codec)
				if !slices.Contains([]string{"srt", "ass", "ssa", "vtt", "sub"}, format) {
					format = ""
				}
				out = append(out, provider.Subtitle{
					// Releases may share an attachment: the release keeps
					// them apart.
					ID:           fmt.Sprintf("%d:%s", e.ID, link),
					Language:     lang,
					ReleaseNames: names,
					Matches:      []string{"title", "series", "tvdb_id", "season", "episode"},
					Format:       format,
					PageLink:     link,
				})
			}
		}
	}
	return out, nil
}

// language reads an attachment's language, as Bazarr's: English when it
// has none, Brazilian Portuguese told from Portuguese by its name only,
// and forced when the track is.
func language(code, name string, forced bool) (provider.Language, bool) {
	if code == "" {
		code = "eng"
	}
	tag := provider.FromCode(code)
	if tag == "" {
		return provider.Language{}, false
	}
	if tag == "pt" && strings.Contains(strings.ToLower(name), "brazil") {
		tag = "pt-BR"
	}
	return provider.Language{Tag: tag, Forced: forced}, true
}

// entries returns the complete releases of an episode, as Bazarr's
// _get_series_entries: the first of the threshold, latest first.
func (p *Provider) entries(ctx context.Context, episodeID int) ([]entry, error) {
	r, err := p.sess.Get(ctx, p.feedURL, url.Values{"eid": {strconv.Itoa(episodeID)}})
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
	out = out[:min(len(out), p.threshold)]
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
