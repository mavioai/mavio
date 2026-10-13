// Package jimaku is the provider of Jimaku.cc, ported from Bazarr's
// jimaku: Japanese subtitles of anime found by AniList ID, or by name.
package jimaku

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/archive"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/site"
)

// Name is the provider's name.
const Name = "jimaku"

// userAgent identifies the plugin, as Bazarr's SZ_USER_AGENT.
const userAgent = "Mavio/0.1"

const (
	// rateLimitWait is how long a rate limit is waited out.
	rateLimitWait = 5 * time.Second
	// rateLimitTries is how many rate limits a request waits out.
	rateLimitTries = 3
	// corruptSize is the size under which files are thought corrupt.
	corruptSize = 500
	// entryTTL and filesTTL are how long searches are cached, as Bazarr's
	// region caches.
	entryTTL = 7 * 24 * time.Hour
	filesTTL = time.Minute
)

// Definition returns the provider's definition.
func Definition() provider.Definition {
	return provider.Definition{
		Name: Name, DisplayName: "Jimaku.cc",
		Settings: []provider.Setting{
			{Name: "api_key", Label: "API key", Help: "API key required. Subtitles stem from various sources and might have quality/timing issues.", Type: provider.Password},
			{Name: "enable_name_search_fallback", Label: "Search by name if no AniList ID was determined (Less accurate, required for live action)", Type: provider.Checkbox, Default: true},
			{Name: "enable_archives_download", Label: "Also consider archives alongside uncompressed subtitles", Type: provider.Checkbox, Default: false},
			{Name: "enable_ai_subs", Label: "Download AI generated subtitles", Type: provider.Checkbox, Default: false},
		},
		Languages: []string{"ja"},
		Movies:    true, Episodes: true,
		New: New,
	}
}

type settings struct {
	APIKey                   string `json:"api_key"`
	EnableNameSearchFallback *bool  `json:"enable_name_search_fallback"`
	EnableArchivesDownload   bool   `json:"enable_archives_download"`
	EnableAISubs             bool   `json:"enable_ai_subs"`
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
		return nil, provider.Errorf(provider.ConfigurationError, "missing api_key")
	}
	sess := site.New(env.HTTP)
	sess.Header.Set("Content-Type", "application/json")
	sess.Header.Set("Authorization", s.APIKey)
	sess.Header.Set("User-Agent", userAgent)
	sess.Check = func(*site.Response) error { return nil }
	return &Provider{
		env: env, s: s, sess: sess, api: "https://jimaku.cc/api", sleep: site.Sleep,
		cache: map[string]cached{},
	}, nil
}

// Provider is Jimaku.cc.
type Provider struct {
	env   provider.Env
	s     settings
	sess  *site.Session
	api   string
	sleep func(context.Context, time.Duration) error

	mu    sync.Mutex
	cache map[string]cached
}

type cached struct {
	data  json.RawMessage
	until time.Time
}

// SetBaseURL points the provider at another server, for tests.
func (p *Provider) SetBaseURL(u *url.URL) { p.api = strings.TrimSuffix(u.String(), "/") + "/api" }

type entry struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	AniListID   *int   `json:"anilist_id"`
	EnglishName string `json:"english_name"`
	Flags       struct {
		Movie      bool `json:"movie"`
		Unverified bool `json:"unverified"`
	} `json:"flags"`
}

type file struct {
	URL  string `json:"url"`
	Name string `json:"name"`
	Size *int   `json:"size"`
}

var (
	// unhandledArchives are always left out; acceptedArchives are
	// downloaded.
	unhandledArchives = []string{".7z"}
	acceptedArchives  = []string{".zip", ".rar"}
	// aiGenerated are the names of subtitles made by Whisper.
	aiGenerated = regexp.MustCompile(`(?i)[\[(]?(whisperai)[\])]?|[\[(]whisper[\])]`)
)

func hasSuffix(name string, exts []string) bool {
	return slices.ContainsFunc(exts, func(e string) bool { return strings.HasSuffix(name, e) })
}

// aniListID returns the video's AniList ID: the episode's, or the
// series'; 0 for none.
func aniListID(v *provider.Video) int {
	id := v.ExternalIDs["anilist"]
	if id == "" && v.Episode {
		id = v.SeriesExternalIDs["anilist"]
	}
	n, err := strconv.Atoi(id)
	if err != nil {
		return 0
	}
	return n
}

// Search implements provider.Provider, as Bazarr's _query: the entry of
// the AniList ID or the name, then its files of the episode.
func (p *Provider) Search(ctx context.Context, v *provider.Video, _ []provider.Language) ([]provider.Subtitle, error) {
	mediaName := strings.ToLower(v.Title)
	if v.Episode {
		mediaName = strings.ToLower(v.Series)
		// Entries of seasons after the first have the season's number
		// appended to their names.
		if v.Season > 1 {
			mediaName += " " + strconv.Itoa(v.Season)
		}
	}
	anilist := aniListID(v)

	var entries []entry
	extra := url.Values{}
	for attempt := 1; attempt <= 2; attempt++ {
		params := url.Values{}
		switch {
		case anilist != 0:
			params.Set("anilist_id", strconv.Itoa(anilist))
		case p.s.EnableNameSearchFallback == nil || *p.s.EnableNameSearchFallback || !v.Episode:
			params.Set("query", mediaName)
		default:
			p.env.Log.InfoContext(ctx, "no AniList ID and searching by name is disabled", "name", mediaName)
			return nil, nil
		}
		for k, vs := range extra {
			params[k] = vs
		}
		data, err := p.request(ctx, "entries/search", params, entryTTL)
		if err != nil {
			return nil, err
		}
		if data != nil {
			if err := json.Unmarshal(data, &entries); err != nil {
				return nil, provider.Errorf(provider.ParseResponseError, "entries: %w", err)
			}
			break
		}
		if !params.Has("query") || attempt == 2 {
			return nil, nil
		}
		// Maybe live action: search again without the anime parameter.
		extra = url.Values{"anime": {"false"}}
	}

	i := slices.IndexFunc(entries, func(e entry) bool {
		return e.AniListID == nil && anilist == 0 || e.AniListID != nil && *e.AniListID == anilist
	})
	if i < 0 {
		p.env.Log.WarnContext(ctx, "no entry matched the AniList ID", "entries", len(entries), "anilist_id", anilist)
		return nil, nil
	}
	e := entries[i]
	if v.Episode && e.Flags.Movie {
		p.env.Log.WarnContext(ctx, "the series is a movie on Jimaku", "entry", e.ID)
	}
	p.env.Log.InfoContext(ctx, "matched entry", "entry", e.ID, "name", e.Name, "english_name", e.EnglishName, "movie", e.Flags.Movie)
	if e.Flags.Unverified {
		p.env.Log.WarnContext(ctx, "the entry is unverified", "entry", e.ID)
	}

	isEpisode := v.Episode && !e.Flags.Movie
	params := url.Values{}
	if isEpisode {
		params.Set("episode", strconv.Itoa(v.Number))
	}
	// The absolute number of an episode of a later season is tried first,
	// as Bazarr's AniDB episode offsets.
	hasOffset := isEpisode && v.Season > 1 && v.AbsoluteNumber > v.Number
	episode := v.Number
	onlyArchives := false
	var files []file
	for retry := 0; retry <= 1; retry++ {
		if hasOffset && retry == 0 {
			params.Set("episode", strconv.Itoa(v.AbsoluteNumber))
		}
		data, err := p.request(ctx, "entries/"+strconv.Itoa(e.ID)+"/files", params, filesTTL)
		if err != nil {
			return nil, err
		}
		if data != nil {
			if err := json.Unmarshal(data, &files); err != nil {
				return nil, provider.Errorf(provider.ParseResponseError, "files: %w", err)
			}
			if hasOffset && retry == 0 {
				episode = v.AbsoluteNumber
			}
			break
		}
		switch {
		case hasOffset && retry == 0:
			params.Set("episode", strconv.Itoa(v.Number))
		case isEpisode && retry == 0:
			params = url.Values{}
			onlyArchives = true
		default:
			return nil, nil
		}
	}

	files = slices.DeleteFunc(files, func(f file) bool { return hasSuffix(f.Name, unhandledArchives) })
	archives := 0
	for _, f := range files {
		if hasSuffix(f.Name, acceptedArchives) {
			archives++
		}
	}
	hasOnlyArchives := archives > 0 && archives == len(files)
	if !hasOnlyArchives && onlyArchives {
		files = slices.DeleteFunc(files, func(f file) bool { return !hasSuffix(f.Name, acceptedArchives) })
	}

	var out []provider.Subtitle
	for _, f := range files {
		isArchive := hasSuffix(f.Name, acceptedArchives)
		switch {
		case isArchive && !hasOnlyArchives && !p.s.EnableArchivesDownload:
			continue
		case !p.s.EnableAISubs && aiGenerated.MatchString(f.Name):
			continue
		case len(languages(f.Name)) > 1:
			continue
		case f.Size != nil && *f.Size < corruptSize:
			continue
		}
		id := f.URL
		if episode != v.Number {
			id += "#episode=" + strconv.Itoa(episode)
		}
		out = append(out, provider.Subtitle{
			ID:           id,
			Language:     provider.Language{Tag: "ja"},
			ReleaseNames: []string{f.Name},
			Matches:      matches(v, f.Name),
			PageLink:     f.URL,
		})
	}
	return out, nil
}

// matches are what Bazarr's get_matches knows of the subtitles of an
// entry: the series and its season, or the title, and the year.
func matches(v *provider.Video, name string) []string {
	var m []string
	if v.Episode {
		if v.Series != "" {
			m = append(m, "series")
		}
		if v.Season > 0 {
			m = append(m, "season")
		}
	} else if v.Title != "" {
		m = append(m, "title")
	}
	if v.Year > 0 {
		m = append(m, "year")
	}
	// SubRip files are preferred by repurposing the audio codec's match.
	if strings.HasSuffix(name, ".srt") {
		m = append(m, "audio_codec")
	}
	return m
}

var (
	brackets   = regexp.MustCompile(`[\[\]()]+`)
	separators = regexp.MustCompile(`[,\-+& ]+`)
	digit      = regexp.MustCompile(`\d`)
)

// squash maps the codes subtitles are named with to languages.
var squash = map[string]string{"jp": "ja", "jap": "ja", "chs": "zho", "cht": "zho", "zhi": "zho", "cn": "zho"}

// languages guesses the languages of a subtitle from the last group of
// its name, as Bazarr's _try_determine_subtitle_languages: Japanese
// alone unless the group names several languages, Japanese among them.
func languages(filename string) []string {
	dots := strings.Split(filename, ".")
	bracketed := brackets.Split(filename, -1)
	group := ""
	switch {
	case len(dots) > 2:
		group = dots[len(dots)-2]
	case len(bracketed) > 2:
		group = bracketed[len(bracketed)-2]
	}
	if group == "" {
		return []string{"ja"}
	}
	candidates := separators.Split(group, -1)
	for _, c := range candidates {
		// Groups with numbers are of the encoding, long ones other
		// metadata.
		if digit.MatchString(c) || len(c) >= 5 {
			return []string{"ja"}
		}
	}
	var langs []string
	for i := 0; i < len(candidates); i++ {
		c := strings.ToLower(candidates[i])
		if c == "ass" || c == "srt" {
			continue
		}
		// Languages hide in blocks of four, as "JPSC".
		if len(c) == 4 {
			candidates = append(candidates, c[:2], c[2:])
			continue
		}
		// Codes have extra information, as "ja[cc]".
		if len(dots) > 2 && strings.ContainsAny(c, "[]()") {
			c = brackets.Split(c, -1)[0]
		}
		if s, ok := squash[c]; ok {
			c = s
		}
		tag := provider.FromCode(c)
		if tag == "" {
			continue
		}
		if !slices.Contains(langs, tag) {
			langs = append(langs, tag)
		}
	}
	// Groups of codecs read as languages: without Japanese, the
	// subtitles are Japanese alone.
	if len(langs) > 1 && slices.Contains(langs, "ja") {
		return langs
	}
	return []string{"ja"}
}

// request gets a path of the API, cached for ttl, as Bazarr's
// _do_jimaku_request: nil when it answers nothing.
func (p *Provider) request(ctx context.Context, path string, params url.Values, ttl time.Duration) (json.RawMessage, error) {
	u := p.api + "/" + path + "?" + params.Encode()
	now := p.env.Now()
	p.mu.Lock()
	c, ok := p.cache[u]
	p.mu.Unlock()
	if ok && now.Before(c.until) {
		return c.data, nil
	}
	data, err := p.fetch(ctx, u)
	if err != nil {
		return nil, err
	}
	p.mu.Lock()
	p.cache[u] = cached{data: data, until: now.Add(ttl)}
	p.mu.Unlock()
	return data, nil
}

func (p *Provider) fetch(ctx context.Context, u string) (json.RawMessage, error) {
	for range rateLimitTries {
		r, err := p.sess.Get(ctx, u, nil)
		if err != nil {
			return nil, err
		}
		switch {
		case r.Status == 429:
			p.env.Log.WarnContext(ctx, "Jimaku rate limit hit", "wait", rateLimitWait)
			if err := p.sleep(ctx, rateLimitWait); err != nil {
				return nil, err
			}
			continue
		case r.Status == 401:
			return nil, provider.Errorf(provider.AuthenticationError, "unauthorized: API key possibly invalid")
		case r.Status >= 400:
			return nil, statusError(r)
		}
		var list []json.RawMessage
		if json.Unmarshal(r.Body, &list) == nil {
			if len(list) == 0 {
				return nil, nil
			}
			return r.Body, nil
		}
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(r.Body, &obj); err != nil {
			return nil, provider.Errorf(provider.ParseResponseError, "%s: %w", r.URL.Path, err)
		}
		if len(obj) == 0 {
			return nil, nil
		}
		if msg, ok := obj["error"]; ok {
			return nil, provider.Errorf(provider.ServiceUnavailable, "Jimaku returned an error: %s, code: %s", msg, obj["code"])
		}
		return r.Body, nil
	}
	return nil, provider.Errorf(provider.APIThrottled, "Jimaku rate limit: %d tries, aborting", rateLimitTries)
}

// Download implements provider.Provider: the file, or the episode's
// subtitle of an archive.
func (p *Provider) Download(ctx context.Context, v *provider.Video, id string) (provider.File, error) {
	link, frag, _ := strings.Cut(id, "#")
	w := archive.Want{}
	if v != nil && v.Episode {
		w.Episode, w.EpisodeTitle = v.Number, v.Title
		if n, ok := strings.CutPrefix(frag, "episode="); ok {
			if ep, err := strconv.Atoi(n); err == nil {
				w.Episode = ep
			}
		}
	}
	r, err := p.sess.Get(ctx, link, nil)
	if err != nil {
		return provider.File{}, err
	}
	if r.Status >= 400 {
		return provider.File{}, statusError(r)
	}
	f, err := archive.Extract(r.Body, w, provider.Language{Tag: "ja"})
	if err != nil {
		return provider.File{}, fmt.Errorf("%s: %w", link, err)
	}
	return f, nil
}

// statusError is an answer's failing status, as requests'
// raise_for_status.
func statusError(r *site.Response) error {
	return provider.Errorf(provider.ProviderError, "%s: bad status code: %d", r.URL.Path, r.Status)
}
