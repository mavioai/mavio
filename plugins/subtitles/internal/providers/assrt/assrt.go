// Package assrt is the provider of Assrt (射手网), ported from Bazarr's
// assrt: Chinese and English subtitles found by name through the API of
// an account's token, paced by the account's quota of requests.
package assrt

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
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
const Name = "assrt"

// userAgent identifies the plugin, as Bazarr's SZ_USER_AGENT.
const userAgent = "Mavio/0.1"

// Definition returns the provider's definition.
func Definition() provider.Definition {
	return provider.Definition{
		Name: Name, DisplayName: "Assrt",
		Settings: []provider.Setting{
			{Name: "token", Label: "Token", Help: "Chinese Subtitles Provider", Type: provider.Text},
		},
		Languages: []string{"zh", "zh-Hans", "zh-Hant", "en"},
		Movies:    true, Episodes: true,
		New: New,
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
	sess.Check = check
	return &Provider{env: env, s: s, sess: sess, api: "https://api.assrt.net/v1", minute: time.Minute, sleep: site.Sleep}, nil
}

// Provider is Assrt.
type Provider struct {
	env  provider.Env
	s    settings
	sess *site.Session
	api  string
	// minute is shared by the account's requests per minute.
	minute time.Duration
	sleep  func(context.Context, time.Duration) error

	mu    sync.Mutex
	quota int
}

// SetBaseURL points the provider at another server, for tests.
func (p *Provider) SetBaseURL(u *url.URL) { p.api = strings.TrimSuffix(u.String(), "/") + "/v1" }

// check maps the API's errors to failures, as Bazarr's check_status_code
// and raise_for_status.
func check(r *site.Response) error {
	var e struct {
		Status *int    `json:"status"`
		ErrMsg *string `json:"errmsg"`
	}
	if json.Unmarshal(r.Body, &e) == nil && e.Status != nil && e.ErrMsg != nil {
		return provider.Errorf(provider.ProviderError, "%s (%d)", *e.ErrMsg, *e.Status)
	}
	if r.Status >= 400 {
		return provider.Errorf(provider.ProviderError, "%s: bad status code: %d", r.URL.Path, r.Status)
	}
	return nil
}

// fromSite are the site's language codes, as Bazarr's AssrtConverter,
// with "eng", the code of English in the site's language lists.
var fromSite = map[string]string{
	"简体": "zh-Hans", "繁体": "zh-Hant", "簡體": "zh-Hans", "繁體": "zh-Hant", "英文": "en",
	"chs": "zh-Hans", "cht": "zh-Hant", "chn": "zh-Hans", "twn": "zh-Hant", "eng": "en",
}

// contains reports whether a language asked for takes a subtitle's, as
// Bazarr's language_contains: Chinese takes either script.
func contains(asked, sub string) bool {
	return asked == sub || asked == "zh" && strings.HasPrefix(sub, "zh")
}

// meaningless are video names that say nothing.
var meaningless = []string{"不知道"}

// delay returns the pause before a request, as Bazarr's
// get_request_delay: a minute shared by the quota, rounded up to seconds.
func (p *Provider) delay(quota int) time.Duration {
	return time.Duration(math.Ceil(p.minute.Seconds()/float64(quota))) * time.Second
}

// get makes a request of the API, pausing first to keep within the
// account's quota, which the first request reads, as Bazarr's initialize.
func (p *Provider) get(ctx context.Context, path string, params url.Values) (*site.Response, error) {
	quota, err := p.userQuota(ctx)
	if err != nil {
		return nil, err
	}
	if err := p.sleep(ctx, p.delay(quota)); err != nil {
		return nil, err
	}
	params.Set("token", p.s.Token)
	return p.sess.Get(ctx, p.api+path, params)
}

func (p *Provider) userQuota(ctx context.Context) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.quota > 0 {
		return p.quota, nil
	}
	r, err := p.sess.Get(ctx, p.api+"/user/quota", url.Values{"token": {p.s.Token}})
	if err != nil {
		return 0, err
	}
	var res struct {
		User struct {
			Quota json.RawMessage `json:"quota"`
		} `json:"user"`
	}
	if err := r.JSON(&res); err != nil {
		return 0, err
	}
	quota, err := strconv.Atoi(string(res.User.Quota))
	switch {
	case err != nil:
		return 0, provider.Errorf(provider.ProviderError, "cannot get user request quota per minute from provider: %s", r.Body)
	case quota <= 0:
		return 0, provider.Errorf(provider.ProviderError, "user request quota is not a positive integer: %d", quota)
	}
	p.quota = quota
	return quota, nil
}

var langKey = regexp.MustCompile(`^lang(\w+)`)

// Search implements provider.Provider, as Bazarr's query: the title and
// year, or the series and episode, searched by name.
func (p *Provider) Search(ctx context.Context, v *provider.Video, langs []provider.Language) ([]provider.Subtitle, error) {
	var keywords []string
	if v.Episode {
		if v.Series != "" {
			keywords = append(keywords, v.Series)
		}
		switch {
		case v.Season > 0 && v.Number > 0:
			keywords = append(keywords, fmt.Sprintf("S%02dE%02d", v.Season, v.Number))
		case v.Number > 0:
			keywords = append(keywords, fmt.Sprintf("E%02d", v.Number))
		}
	} else {
		if v.Title != "" {
			keywords = append(keywords, v.Title)
		}
		if v.Year > 0 {
			keywords = append(keywords, strconv.Itoa(v.Year))
		}
	}
	r, err := p.get(ctx, "/sub/search", url.Values{"q": {strings.Join(keywords, " ")}, "is_file": {"1"}})
	if err != nil {
		return nil, err
	}
	var res struct {
		Sub struct {
			Subs []struct {
				ID        json.Number     `json:"id"`
				VideoName string          `json:"videoname"`
				Native    json.RawMessage `json:"native_name"`
				Lang      *struct {
					LangList map[string]json.RawMessage `json:"langlist"`
				} `json:"lang"`
			} `json:"subs"`
		} `json:"sub"`
	}
	if err := r.JSON(&res); err != nil {
		return nil, err
	}
	var out []provider.Subtitle
	seen := map[string]bool{}
	for _, sub := range res.Sub.Subs {
		if sub.Lang == nil {
			continue
		}
		name := videoName(sub.VideoName, sub.Native)
		for _, key := range slices.Sorted(func(yield func(string) bool) {
			for k := range sub.Lang.LangList {
				if !yield(k) {
					return
				}
			}
		}) {
			m := langKey.FindStringSubmatch(key)
			if m == nil {
				continue
			}
			tag, ok := fromSite[m[1]]
			if !ok {
				continue
			}
			i := slices.IndexFunc(langs, func(l provider.Language) bool { return contains(l.Tag, tag) })
			if i < 0 {
				continue
			}
			id := sub.ID.String() + "/" + langs[i].Tag
			if seen[id] {
				continue
			}
			seen[id] = true
			s := provider.Subtitle{
				ID:       id,
				Language: provider.Language{Tag: langs[i].Tag},
				Matches:  matches(v, name),
			}
			if name != "" {
				s.ReleaseNames = releaseNames(name)
			}
			out = append(out, s)
		}
	}
	return out, nil
}

// videoName is the name of a subtitle's video: its video name unless
// meaningless, else its native name.
func videoName(name string, native json.RawMessage) string {
	if !slices.Contains(meaningless, name) {
		return name
	}
	var s string
	if json.Unmarshal(native, &s) == nil {
		return s
	}
	var list []string
	if json.Unmarshal(native, &list) == nil && len(list) > 0 {
		return list[0]
	}
	return ""
}

// leadingCJK is a Chinese title before a release's name, as
// "瑞克和莫蒂.Rick.and.Morty.S07E10".
var leadingCJK = regexp.MustCompile(`^[\x{4e00}-\x{9fff}]+[.\s]+`)

// releaseNames are a video's name and, as Bazarr retries guessing it, the
// name without its Chinese title.
func releaseNames(name string) []string {
	out := []string{name}
	if cleaned := leadingCJK.ReplaceAllString(name, ""); cleaned != name {
		out = append(out, cleaned)
	}
	return out
}

// seasonPack is a release of a whole season, as "Show.S06.1080p".
var seasonPack = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])s(\d{1,3})(?:[^0-9e]|$)`)

// matches are what Bazarr's get_matches adds to the guesses of the name:
// the episode of a season's pack, whose file of the episode is picked at
// download.
func matches(v *provider.Video, name string) []string {
	if !v.Episode || name == "" {
		return nil
	}
	if _, ok := archive.EpisodeNumber(name); ok {
		return nil
	}
	if m := seasonPack.FindStringSubmatch(name); m != nil {
		if n, _ := strconv.Atoi(m[1]); n == v.Season {
			return []string{"episode"}
		}
	}
	return nil
}

// Download implements provider.Provider, as Bazarr's _get_detail: the
// file of the episode and language among the subtitle's files.
func (p *Provider) Download(ctx context.Context, v *provider.Video, id string) (provider.File, error) {
	subID, tag, ok := strings.Cut(id, "/")
	if !ok {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "malformed subtitle ID %q", id)
	}
	r, err := p.get(ctx, "/sub/detail", url.Values{"id": {subID}})
	if err != nil {
		return provider.File{}, err
	}
	type entry struct {
		F   string `json:"f"`
		URL string `json:"url"`
	}
	var res struct {
		Sub struct {
			Subs []struct {
				URL      string  `json:"url"`
				FileList []entry `json:"filelist"`
			} `json:"subs"`
		} `json:"sub"`
	}
	if err := r.JSON(&res); err != nil {
		return provider.File{}, err
	}
	if len(res.Sub.Subs) == 0 {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "can't get subtitle details")
	}
	sub := res.Sub.Subs[0]
	link := sub.URL
	if files := sub.FileList; len(files) > 0 {
		// Files of a season's pack are narrowed to the episode's first.
		if v != nil && v.Episode {
			episodes := slices.DeleteFunc(slices.Clone(files), func(f entry) bool {
				n, ok := archive.EpisodeNumber(f.F)
				return !ok || n != v.Number
			})
			if len(episodes) > 0 {
				files = episodes
			}
		}
		link = files[0].URL
		for _, f := range files {
			if slices.ContainsFunc(fileLanguages(f.F), func(t string) bool { return contains(tag, t) }) {
				link = f.URL
				break
			}
		}
	}
	if link == "" {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "no download link")
	}
	quota, err := p.userQuota(ctx)
	if err != nil {
		return provider.File{}, err
	}
	if err := p.sleep(ctx, p.delay(quota)); err != nil {
		return provider.File{}, err
	}
	r, err = p.sess.Get(ctx, link, nil)
	if err != nil {
		return provider.File{}, err
	}
	w := archive.WantFor(v, provider.Language{Tag: tag})
	return archive.Extract(r.Body, w, provider.Language{Tag: tag})
}

// fileLanguages are the languages of the site's codes in a file name.
func fileLanguages(name string) []string {
	var out []string
	for code, tag := range fromSite {
		if strings.Contains(name, code) && !slices.Contains(out, tag) {
			out = append(out, tag)
		}
	}
	return out
}
