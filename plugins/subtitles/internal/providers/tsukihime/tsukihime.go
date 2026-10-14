// Package tsukihime is the provider of TsukiHime, ported from Bazarr's
// tsukihime: the subtitle tracks of the anime releases the site indexes,
// found by the AniDB ID of an episode's series or the AniList ID of a
// movie, and served xz-compressed.
package tsukihime

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
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/ulikunitz/xz"
	"golang.org/x/text/language"

	"github.com/mavioai/mavio/libs/release"
	"github.com/mavioai/mavio/plugins/subtitles/internal/anidb"
	"github.com/mavioai/mavio/plugins/subtitles/internal/archive"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/site"
)

// Name is the provider's name.
const Name = "tsukihime"

// The API and the storage of the attachments.
const (
	apiURL     = "https://api.tsukihime.org/v1"
	storageURL = "https://storage.tsukihime.org"
)

// Storage paths: AnimeTosho's mirrored releases have their own.
const (
	nativeStorage     = "attach"
	animeToshoStorage = "tosho/attach"
)

// maxTorrents is how many releases are looked into.
const maxTorrents = 20

// userAgent identifies the plugin, as Bazarr's SZ_USER_AGENT.
const userAgent = "Mavio"

// maxSubtitle bounds a decompressed subtitle.
const maxSubtitle = 10 << 20

// xzMagic starts xz files.
var xzMagic = []byte("\xFD7zXZ\x00")

// regionalAliases are the site's codes of regional variants, which are
// neither IETF tags nor carry a region babelfish knows, as Bazarr's
// _REGIONAL_LANGUAGE_ALIASES.
var regionalAliases = map[string]string{
	"es-419": "es-MX", "es-es": "es", "pt-br": "pt-BR", "pt-pt": "pt",
	"zh-cn": "zh-Hans", "zh-hans": "zh-Hans", "zh-hk": "zh-Hant", "zh-hant": "zh-Hant", "zh-tw": "zh-Hant",
}

// languages are the site's, as Bazarr's _SUPPORTED_ALPHA2_CODES and the
// regional variants.
var languages = []string{
	"af", "am", "ar", "az", "be", "bg", "bn", "bs", "ca", "cs",
	"da", "de", "el", "en", "eo", "es", "et", "eu", "fa", "fi",
	"fr", "gl", "he", "hi", "hr", "hu", "hy", "id", "is", "it",
	"ja", "ka", "kk", "ko", "lt", "lv", "mk", "ms", "nl", "no",
	"pl", "pt", "ro", "ru", "sk", "sl", "sq", "sr", "sv", "th",
	"tr", "uk", "ur", "uz", "vi", "zh",
	"es-MX", "pt-BR", "zh-Hans", "zh-Hant",
}

// subtitleCodecs are the codecs of the tracks offered.
var subtitleCodecs = []string{"ass", "srt", "ssa", "sub", "vtt"}

var (
	// hiMarker is the hearing-impaired marker groups add to a track's
	// name: "English (CC)", "[SDH]", "english.hearing impaired".
	hiMarker = regexp.MustCompile(`(?i)(?:^|[\s_.\-\[(])(?:cc|sdh|hearing[\s_.\-]*impaired)(?:$|[\s_.\-)\]])`)
	// signsMarker names the tracks of signs and songs, often without the
	// forced flag: "English [Signs]", "Signs & Songs".
	signsMarker = regexp.MustCompile(`(?i)\bsigns?\b`)
	// nonWord splits names in words, as Python's \W+.
	nonWord = regexp.MustCompile(`[^\p{L}\p{N}_]+`)
)

// Definition returns the provider's definition.
func Definition() provider.Definition {
	return provider.Definition{
		Name: Name, DisplayName: "TsukiHime",
		Languages: languages,
		Movies:    true, Episodes: true,
		New: New,
	}
}

// New makes the provider; it has no settings.
func New(env provider.Env, _ json.RawMessage) (provider.Provider, error) {
	sess := site.New(env.HTTP)
	sess.Header.Set("User-Agent", userAgent)
	sess.Check = func(r *site.Response) error {
		switch r.Status {
		case http.StatusOK:
			return nil
		case http.StatusNotFound:
			return errNotFound
		}
		return provider.Errorf(provider.ProviderError, "TsukiHime returned HTTP %d for %s", r.Status, r.URL.Path)
	}
	return &Provider{env: env, sess: sess, anidb: anidb.New(env, anidb.Config{}), api: apiURL, storage: storageURL}, nil
}

// errNotFound is the API's 404.
var errNotFound = errors.New("not found")

// Provider is TsukiHime.
type Provider struct {
	env          provider.Env
	sess         *site.Session
	anidb        *anidb.Client
	api, storage string
}

// SetBaseURL points the provider's API and storage at another server,
// for tests.
func (p *Provider) SetBaseURL(u *url.URL) {
	base := strings.TrimSuffix(u.String(), "/")
	p.api, p.storage = base+"/v1", base
}

// AniDB returns the client the provider finds AniDB and AniList IDs
// with.
func (p *Provider) AniDB() *anidb.Client { return p.anidb }

// languageFromCode returns the language of a site's code, as Bazarr's
// _language_from_code: its regional aliases, an IETF tag, or an ISO
// 639-2/B code; "" when unknown.
func languageFromCode(code string) string {
	if code == "" {
		return ""
	}
	normalized := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(code)), "_", "-")
	if tag, ok := regionalAliases[normalized]; ok {
		return tag
	}
	for _, c := range []string{strings.TrimSpace(code), normalized} {
		if tag := fromIETF(c); tag != "" {
			return tag
		}
	}
	return provider.FromCode(normalized)
}

// fromIETF reads an IETF tag as babelfish: a language, then scripts of
// four letters and regions of two.
func fromIETF(code string) string {
	parts := strings.Split(code, "-")
	if len(parts[0]) != 2 && len(parts[0]) != 3 {
		return ""
	}
	tag := provider.FromCode(parts[0])
	if tag == "" {
		return ""
	}
	for _, p := range parts[1:] {
		switch len(p) {
		case 4:
			s, err := language.ParseScript(p)
			if err != nil {
				return ""
			}
			tag += "-" + s.String()
		case 2:
			r, err := language.ParseRegion(p)
			if err != nil || !r.IsCountry() {
				return ""
			}
			tag += "-" + r.String()
		default:
			return ""
		}
	}
	return tag
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

// anime is the site's anime.
type anime struct {
	ID          int64 `json:"id"`
	ReleaseYear int   `json:"release_year"`
}

// entry is a release of an anime or episode.
type entry struct {
	ID         int64    `json:"id"`
	Name       string   `json:"name"`
	State      string   `json:"state"`
	SubLangs   []string `json:"sublangs"`
	SourceDate int64    `json:"source_date"`
	AnimeTosho bool     `json:"animetosho"`
}

// file is a file of a release.
type file struct {
	Filename    string `json:"filename"`
	Attachments []struct {
		ID   int64 `json:"id"`
		Type int   `json:"type"`
		Info struct {
			Cached *int   `json:"cached"`
			Codec  string `json:"codec"`
			Lang   string `json:"lang"`
			Name   string `json:"name"`
			Forced flag   `json:"forced"`
		} `json:"info"`
	} `json:"attachments"`
}

// Search implements provider.Provider.
func (p *Provider) Search(ctx context.Context, v *provider.Video, langs []provider.Language) ([]provider.Subtitle, error) {
	a, episodeNo, err := p.anime(ctx, v)
	if err != nil || a == nil {
		return nil, err
	}
	var path string
	if v.Episode {
		path = fmt.Sprintf("/animes/%d/episodes/%d", a.ID, episodeNo)
	} else {
		path = fmt.Sprintf("/animes/%d", a.ID)
	}
	var res struct {
		Results []entry `json:"results"`
	}
	if found, err := p.getJSON(ctx, path, &res); err != nil || !found {
		return nil, err
	}
	var entries []entry
	for _, e := range res.Results {
		if e.State == "completed" && hasLanguage(e.SubLangs, langs) {
			entries = append(entries, e)
		}
	}
	// The releases most like the video's come first, then the latest.
	original := originalName(v)
	slices.SortStableFunc(entries, func(a, b entry) int {
		return cmp.Or(cmp.Compare(overlap(b.Name, original), overlap(a.Name, original)), cmp.Compare(b.SourceDate, a.SourceDate))
	})

	var out []provider.Subtitle
	seen := map[string]bool{}
	for _, e := range entries[:min(len(entries), maxTorrents)] {
		if e.ID == 0 {
			continue
		}
		var detail struct {
			Files []file `json:"files"`
		}
		if found, err := p.getJSON(ctx, fmt.Sprintf("/torrents/%d", e.ID), &detail); err != nil {
			return nil, err
		} else if !found {
			continue
		}
		for _, f := range matchingFiles(v, episodeNo, detail.Files) {
			for i := range f.Attachments {
				s, ok := p.subtitle(ctx, v, a, e, f, i, langs)
				if ok && !seen[s.ID] {
					seen[s.ID] = true
					out = append(out, s)
				}
			}
		}
	}
	return out, nil
}

// anime returns the site's anime of a video, by the AniDB ID of an
// episode's series or the AniList ID of a movie, and the episode's
// number in the anime; nil for none.
func (p *Provider) anime(ctx context.Context, v *provider.Video) (*anime, int, error) {
	var path string
	episodeNo := 0
	if v.Episode {
		ids, err := p.anidb.Refine(ctx, v, false)
		if err != nil {
			return nil, 0, err
		}
		if ids.Series == 0 {
			p.env.Log.DebugContext(ctx, "skipping the video: no AniDB series ID", "series", v.Series)
			return nil, 0, nil
		}
		path, episodeNo = fmt.Sprintf("/animes/anidb/%d", ids.Series), cmp.Or(ids.EpisodeNumber, v.Number)
	} else {
		id, err := p.anidb.AniListID(ctx, v, 0)
		if err != nil {
			return nil, 0, err
		}
		if id == 0 {
			p.env.Log.DebugContext(ctx, "skipping the video: no AniList ID", "title", v.Title)
			return nil, 0, nil
		}
		path = fmt.Sprintf("/animes/anilist/%d", id)
	}
	var a anime
	if found, err := p.getJSON(ctx, path, &a); err != nil || !found {
		return nil, 0, err
	}
	return &a, episodeNo, nil
}

// subtitle returns the subtitle of a release's attachment, as Bazarr's
// _subtitle_from_attachment, if it is one in a language asked for.
func (p *Provider) subtitle(ctx context.Context, v *provider.Video, a *anime, e entry, f file, i int, langs []provider.Language) (provider.Subtitle, bool) {
	att := f.Attachments[i]
	info := att.Info
	// Native releases' tracks can be downloaded once extracted; those of
	// AnimeTosho have no such flag.
	if att.Type != 1 || info.Cached != nil && *info.Cached == 0 {
		return provider.Subtitle{}, false
	}
	tag := languageFromCode(info.Lang)
	if tag == "" {
		p.env.Log.DebugContext(ctx, "unsupported language code", "code", info.Lang)
		return provider.Subtitle{}, false
	}
	lang := provider.Language{Tag: tag}
	switch {
	case bool(info.Forced) || signsMarker.MatchString(info.Name):
		lang.Forced = true
	case hiMarker.MatchString(info.Name):
		lang.HearingImpaired = true
	}
	codec := strings.ToLower(info.Codec)
	if !slices.Contains(langs, lang) || !slices.Contains(subtitleCodecs, codec) || att.ID == 0 {
		return provider.Subtitle{}, false
	}
	storage := nativeStorage
	if e.AnimeTosho {
		storage = animeToshoStorage
	}
	link := fmt.Sprintf("%s/%s/%08X/%d.xz", p.storage, storage, att.ID, att.ID)
	matches := []string{"title"}
	if v.Episode {
		matches = []string{"series", "season", "episode"}
	}
	if v.Year != 0 && a.ReleaseYear == v.Year {
		matches = append(matches, "year")
	}
	name := cmp.Or(e.Name, f.Filename)
	return provider.Subtitle{
		// Releases share attachments: the release keeps them apart.
		ID:           fmt.Sprintf("%d:%s", e.ID, link),
		Language:     lang,
		ReleaseNames: []string{name},
		Matches:      matches,
		Format:       codec,
		PageLink:     link,
		// Releases of a name from several streaming services, and several
		// tracks of a language in a release, are told apart by the
		// service or the track's name, as Bazarr's.
		Comment: cmp.Or(release.ParseStreamingService(name), trackLabel(info.Name)),
	}, true
}

// trackLabel returns a track's name unless it only names its language,
// as Bazarr's _track_label.
func trackLabel(name string) string {
	label := strings.TrimSpace(name)
	if label == "" || provider.FromName(label) != "" {
		return ""
	}
	return label
}

// hasLanguage reports whether a release has subtitles in a language
// asked for, regions aside.
func hasLanguage(subLangs []string, langs []provider.Language) bool {
	for _, code := range subLangs {
		tag := languageFromCode(code)
		if tag != "" && slices.ContainsFunc(langs, func(l provider.Language) bool { return provider.Alpha3(l.Tag) == provider.Alpha3(tag) }) {
			return true
		}
	}
	return false
}

// originalName returns the name of the video's release, without its
// extension.
func originalName(v *provider.Video) string {
	name := cmp.Or(v.ReleaseName, v.FileName)
	return strings.TrimSuffix(name, filepath.Ext(name))
}

// overlap returns the share of a candidate's words that a reference
// has, as Bazarr's _name_overlap.
func overlap(candidate, reference string) float64 {
	words := func(s string) map[string]bool {
		out := map[string]bool{}
		for _, w := range nonWord.Split(s, -1) {
			out[w] = true
		}
		return out
	}
	c, r := words(candidate), words(reference)
	n := 0
	for w := range c {
		if r[w] {
			n++
		}
	}
	return float64(n) / float64(len(c))
}

// matchingFiles returns the files of a release the video is, as Bazarr's
// _matching_files: the one most like a movie's name, or an episode's,
// all when none is.
func matchingFiles(v *provider.Video, episodeNo int, files []file) []file {
	if len(files) <= 1 {
		return files
	}
	if !v.Episode {
		original := strings.ToLower(originalName(v))
		best := 0
		for i, f := range files {
			if overlap(strings.ToLower(f.Filename), original) > overlap(strings.ToLower(files[best].Filename), original) {
				best = i
			}
		}
		return files[best : best+1]
	}
	var out []file
	for _, f := range files {
		if n, ok := archive.EpisodeNumber(f.Filename); ok && (n == episodeNo || n == v.Number) {
			out = append(out, f)
		}
	}
	if len(out) == 0 {
		return files
	}
	return out
}

// getJSON reads an answer of the API; false when it has none.
func (p *Provider) getJSON(ctx context.Context, path string, out any) (bool, error) {
	r, err := p.sess.Get(ctx, p.api+path, nil)
	if errors.Is(err, errNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := json.Unmarshal(r.Body, out); err != nil {
		return false, provider.Errorf(provider.ProviderError, "TsukiHime returned invalid JSON for %s: %w", path, err)
	}
	return true, nil
}

// Download implements provider.Provider.
func (p *Provider) Download(ctx context.Context, _ *provider.Video, id string) (provider.File, error) {
	_, link, ok := strings.Cut(id, ":")
	if u, err := url.Parse(link); !ok || err != nil || u.Scheme != "http" && u.Scheme != "https" {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "malformed subtitle ID %q", id)
	}
	r, err := p.sess.Get(ctx, link, nil)
	if errors.Is(err, errNotFound) {
		return provider.File{}, provider.Errorf(provider.ProviderError, "TsukiHime returned HTTP 404 for %s", link)
	}
	if err != nil {
		return provider.File{}, err
	}
	if !bytes.HasPrefix(r.Body, xzMagic) {
		return provider.File{}, provider.Errorf(provider.ProviderError, "TsukiHime returned an unidentified archive type")
	}
	zr, err := xz.NewReader(bytes.NewReader(r.Body))
	if err != nil {
		return provider.File{}, provider.Errorf(provider.ProviderError, "TsukiHime subtitle decompression failed: %w", err)
	}
	data, err := io.ReadAll(io.LimitReader(zr, maxSubtitle+1))
	switch {
	case err != nil:
		return provider.File{}, provider.Errorf(provider.ProviderError, "TsukiHime subtitle decompression failed: %w", err)
	case len(data) > maxSubtitle:
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "larger than %d bytes", maxSubtitle)
	}
	return provider.File{Data: data, Format: archive.Detect(data)}, nil
}
