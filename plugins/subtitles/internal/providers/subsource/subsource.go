// Package subsource is the subsource.net provider, ported from Bazarr's
// subsource.py: SubSource's JSON API with an API key, finding the title
// by IMDb ID, then its subtitles in one language, served as zip archives.
package subsource

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/archive"
	"github.com/mavioai/mavio/plugins/subtitles/internal/archivemix"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/site"
)

// Name is the provider's, as Bazarr names it.
const Name = "subsource"

const (
	userAgent = "Mavio/0.1"
	apiURL    = "https://api.subsource.net/api/v1/"
	siteURL   = "https://subsource.net"
	// The caches' lifetimes, as Bazarr's.
	titlesExpiration   = 6 * time.Hour
	queriesExpiration  = time.Hour
	archivesExpiration = 15 * time.Minute
	// maxInlineWait is the longest rate limit waited out rather than
	// throttling, as Bazarr's MAX_INLINE_WAIT.
	maxInlineWait = 60
	// retryAmount and retryTimeout are those of Bazarr's retries of
	// network failures.
	retryAmount  = 3
	retryTimeout = 5 * time.Second
)

// names are SubSource's language names by BCP 47 tag, as Bazarr's
// SubsourceConverter.
var names = map[string]string{
	"en": "English", "fa": "Farsi_persian", "ab": "Abkhazian", "af": "Afrikaans", "sq": "Albanian", "am": "Amharic",
	"ar": "Arabic", "an": "Aragonese", "hy": "Armenian", "as": "Assamese", "az": "Azerbaijani", "eu": "Basque",
	"be": "Belarusian", "bn": "Bengali", "bs": "Bosnian", "pt-BR": "Brazilian_portuguese", "br": "Breton",
	"bg": "Bulgarian", "my": "Burmese", "ca": "Catalan", "zh": "Chinese_bg_code", "hr": "Croatian", "cs": "Czech",
	"da": "Danish", "nl": "Dutch", "eo": "Espranto", "et": "Estonian", "fi": "Finnish", "fr": "French", "gd": "Gaelic",
	"ka": "Georgian", "de": "German", "el": "Greek", "he": "Hebrew", "hi": "Hindi", "hu": "Hungarian", "is": "Icelandic",
	"ig": "Igbo", "id": "Indonesian", "ia": "Interlingua", "ga": "Irish", "it": "Italian", "ja": "Japanese",
	"kn": "Kannada", "kk": "Kazakh", "km": "Khmer", "ko": "Korean", "ku": "Kurdish", "lv": "Latvian", "lt": "Lithuanian",
	"lb": "Luxembourgish", "mk": "Macedonian", "ms": "Malay", "ml": "Malayalam", "mr": "Marathi", "mn": "Mongolian",
	"nv": "Navajo", "ne": "Nepali", "se": "Northen Sami", "no": "Norwegian", "oc": "Occitan", "pl": "Polish", //nolint:misspell // the site's spelling
	"pt": "Portuguese", "ps": "Pushto", "ro": "Romanian", "ru": "Russian", "sr": "Serbian", "sd": "Sindhi",
	"si": "Sinhala", "sk": "Slovak", "sl": "Slovenian", "so": "Somali", "es": "Spanish", "sw": "Swahili",
	"sv": "Swedish", "tl": "Tagalog", "ta": "Tamil", "tt": "Tatar", "te": "Telugu", "th": "Thai", "tr": "Turkish",
	"tk": "Turkmen", "uk": "Ukrainian", "ur": "Urdu", "uz": "Uzbek", "vi": "Vietnamese", "cy": "Welsh",
}

// tagOf returns the tag of a language name of the API, ignoring case as
// the API lowercases them; "" when unknown.
func tagOf(name string) string {
	for tag, n := range names {
		if strings.EqualFold(n, name) {
			return tag
		}
	}
	return ""
}

// Definition returns the provider's definition, with Bazarr's settings
// and throttling.
func Definition() provider.Definition {
	langs := make([]string, 0, len(names))
	for tag := range names {
		langs = append(langs, tag)
	}
	return provider.Definition{
		Name: Name, DisplayName: "subsource.net",
		Settings: []provider.Setting{
			{Name: "apikey", Label: "API key", Help: "You can create your own API key on your Subsource profile page", Type: provider.Password},
		},
		Languages:                 langs,
		HearingImpairedVerifiable: true,
		Movies:                    true, Episodes: true,
		Throttle: map[provider.Kind]func(time.Time) time.Duration{
			provider.APIThrottled:        provider.Fixed(10 * time.Minute),
			provider.AuthenticationError: provider.Fixed(time.Hour),
			provider.ForbiddenError:      provider.Fixed(15 * time.Minute),
			provider.TooManyRequests:     provider.Fixed(time.Hour),
		},
		New: New,
	}
}

// entry is a cached value.
type entry struct {
	value   any
	expires time.Time
}

// Provider searches SubSource.
type Provider struct {
	env    provider.Env
	s      *site.Session
	apiKey string
	api    string
	// sleep and wait are how rate limits and network failures are waited
	// out; tests make them instant.
	sleep func(context.Context, time.Duration) error
	wait  time.Duration

	mu    sync.Mutex
	cache map[string]entry
}

// New makes the provider from its settings; the API key is required, as
// Bazarr's.
func New(env provider.Env, raw json.RawMessage) (provider.Provider, error) {
	var s struct {
		APIKey string `json:"apikey"`
	}
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, provider.Errorf(provider.ConfigurationError, "settings: %w", err)
	}
	if s.APIKey == "" {
		return nil, provider.Errorf(provider.ConfigurationError, "Api_key must be specified")
	}
	hc := env.HTTP
	if hc == nil {
		hc = http.DefaultClient
	}
	sess := site.New(hc)
	sess.Header.Set("User-Agent", userAgent)
	sess.Check = func(*site.Response) error { return nil }
	return &Provider{
		env: env, s: sess, apiKey: s.APIKey, api: apiURL,
		sleep: site.Sleep, wait: retryTimeout, cache: map[string]entry{},
	}, nil
}

// SetBaseURL points the provider at another API, for tests.
func (p *Provider) SetBaseURL(u *url.URL) { p.api = strings.TrimRight(u.String(), "/") + "/api/v1/" }

// cached returns a value cached under key, or computes and caches it, as
// Bazarr's region cache; failures are not cached.
func cached[T any](p *Provider, key string, ttl time.Duration, f func() (T, error)) (T, error) {
	now := p.env.Now()
	p.mu.Lock()
	if e, ok := p.cache[key]; ok && now.Before(e.expires) {
		p.mu.Unlock()
		return e.value.(T), nil
	}
	p.mu.Unlock()
	v, err := f()
	if err != nil {
		return v, err
	}
	p.mu.Lock()
	for k, e := range p.cache {
		if !now.Before(e.expires) {
			delete(p.cache, k)
		}
	}
	p.cache[key] = entry{value: v, expires: now.Add(ttl)}
	p.mu.Unlock()
	return v, nil
}

// get fetches a path of the API, retrying network failures and waiting
// out a short rate limit once, then mapping the status as Bazarr's
// checked.
func (p *Provider) get(ctx context.Context, pth string, query url.Values) (*site.Response, error) {
	query.Set("api_key", p.apiKey)
	for retried := false; ; retried = true {
		r, err := p.fetch(ctx, p.api+pth, query)
		if err != nil {
			return nil, err
		}
		if r.Status == http.StatusTooManyRequests && !retried {
			if after, ok := retryAfter(r, p.env.Now()); ok && after <= maxInlineWait {
				p.env.Log.DebugContext(ctx, "Rate limit exceeded, waiting and trying again", "seconds", after)
				if err := p.sleep(ctx, time.Duration(after)*time.Second); err != nil {
					return nil, err
				}
				continue
			}
		}
		return r, status(r)
	}
}

// fetch makes a request, retrying network failures as Bazarr's
// ProviderRetryMixin.retry.
func (p *Provider) fetch(ctx context.Context, u string, query url.Values) (*site.Response, error) {
	for i := 1; ; i++ {
		r, err := p.s.Get(ctx, u, query)
		if err == nil || ctx.Err() != nil || i >= retryAmount {
			return r, err
		}
		p.env.Log.DebugContext(ctx, "Retrying SubSource", "try", i, "error", err)
		if err := p.sleep(ctx, p.wait); err != nil {
			return nil, err
		}
	}
}

// status maps the status of an answer as Bazarr's _status_raiser.
func status(r *site.Response) error {
	switch c := r.Status; {
	case c == http.StatusOK:
		return nil
	case c == http.StatusBadRequest:
		return provider.Errorf(provider.APIThrottled, "Invalid request parameters")
	case c == http.StatusUnauthorized:
		return provider.Errorf(provider.AuthenticationError, "Authentication required")
	case c == http.StatusForbidden:
		return provider.Errorf(provider.ForbiddenError, "Access denied")
	case c == http.StatusTooManyRequests:
		return provider.Errorf(provider.TooManyRequests, "Rate limit exceeded")
	case c >= 500:
		return provider.Errorf(provider.ServiceUnavailable, "%s: %d", r.URL.Path, c)
	case c >= 400:
		return provider.Errorf(provider.ProviderError, "%s: %d", r.URL.Path, c)
	}
	return nil
}

// retryAfter returns the seconds before the rate limit resets, as
// Bazarr's _retry_after: X-RateLimit-Reset, else the body's retryAfter;
// never less than 1, and false when the server does not say.
func retryAfter(r *site.Response, now time.Time) (int, bool) {
	var delay float64
	known := false
	if h := r.Header.Get("X-RateLimit-Reset"); h != "" {
		if at, err := time.Parse(time.RFC3339Nano, h); err == nil {
			delay, known = at.Sub(now).Seconds(), true
		}
	}
	if !known {
		var body map[string]any
		if json.Unmarshal(r.Body, &body) != nil || body == nil {
			return 0, false
		}
		n, ok := body["retryAfter"].(float64)
		if !ok {
			return 0, false
		}
		delay = n
	}
	return max(int(math.Ceil(delay)), 1), true
}

// num is a JSON number or a string of one.
type num int

func (n *num) UnmarshalJSON(b []byte) error {
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch x := v.(type) {
	case float64:
		*n = num(x)
	case string:
		i, _ := strconv.Atoi(strings.TrimSpace(x))
		*n = num(i)
	}
	return nil
}

// title is a result of the title search.
type title struct {
	Title          *string `json:"title"`
	AlternateTitle string  `json:"alternateTitle"`
	ReleaseYear    *num    `json:"releaseYear"`
	MovieID        num     `json:"movieId"`
}

// searchTitles returns the ID of the video's title, by IMDb ID and then
// by its title, as Bazarr's search_titles; 0 when none matches.
func (p *Provider) searchTitles(ctx context.Context, v *provider.Video, name, imdb string, season int) (int, error) {
	key := fmt.Sprintf("titles\x00%s\x00%s\x00%d", name, imdb, season)
	return cached(p, key, titlesExpiration, func() (int, error) {
		text := url.Values{"searchType": {"text"}, "q": {strings.ToLower(name)}}
		q := text
		if imdb != "" {
			q = url.Values{"searchType": {"imdb"}, "imdb": {imdb}}
		}
		if season != 0 {
			q.Set("season", strconv.Itoa(season))
			text.Set("season", strconv.Itoa(season))
		}
		results, err := p.titles(ctx, q)
		if err != nil {
			return 0, err
		}
		if imdb != "" && len(results) == 0 {
			p.env.Log.DebugContext(ctx, "No results for the IMDb ID, falling back to a text search", "imdb", imdb, "title", name)
			if results, err = p.titles(ctx, text); err != nil {
				return 0, err
			}
		}
		wanted := alternativeTitles(v)
		for _, r := range results {
			if r.Title == nil || r.ReleaseYear == nil {
				continue
			}
			subTitles := []string{strings.ToLower(*r.Title)}
			if r.AlternateTitle != "" {
				subTitles = append(subTitles, strings.ToLower(r.AlternateTitle))
			}
			if !matchesAny(wanted, subTitles) {
				continue
			}
			if v.Year == 0 || v.Year == int(*r.ReleaseYear) {
				return int(r.MovieID), nil
			}
		}
		p.env.Log.DebugContext(ctx, "No title match found", "title", name)
		return 0, nil
	})
}

func (p *Provider) titles(ctx context.Context, q url.Values) ([]title, error) {
	r, err := p.get(ctx, "movies/search", q)
	if err != nil {
		return nil, err
	}
	var out struct {
		Data []title `json:"data"`
	}
	if err := r.JSON(&out); err != nil {
		return nil, err
	}
	return out.Data, nil
}

// alternativeTitles are the lowercased titles of a video, as Bazarr's
// get_alternative_titles: the series' for an episode.
func alternativeTitles(v *provider.Video) []string {
	var titles []string
	if v.Episode {
		titles = append([]string{v.Series}, v.AlternativeSeries...)
	} else {
		titles = []string{v.Title}
	}
	var out []string
	for _, t := range titles {
		if t != "" {
			out = append(out, strings.ToLower(t))
		}
	}
	return out
}

// matchesAny reports whether a wanted title is within a title of the
// site.
func matchesAny(wanted, titles []string) bool {
	for _, w := range wanted {
		for _, t := range titles {
			if strings.Contains(t, w) {
				return true
			}
		}
	}
	return false
}

// item is a subtitle of the API.
type item struct {
	Link            string   `json:"link"`
	SubtitleID      num      `json:"subtitleId"`
	ReleaseInfo     []string `json:"releaseInfo"`
	Language        string   `json:"language"`
	Commentary      any      `json:"commentary"`
	HearingImpaired any      `json:"hearingImpaired"`
	ForeignParts    any      `json:"foreignParts"`
	UploaderID      any      `json:"uploaderId"`
	Contributors    []struct {
		ID          any    `json:"id"`
		DisplayName string `json:"displayname"`
	} `json:"contributors"`
}

// Search lists the subtitles of a video in the first language, as
// Bazarr's query, which searches one language only.
func (p *Provider) Search(ctx context.Context, v *provider.Video, langs []provider.Language) ([]provider.Subtitle, error) {
	if len(langs) == 0 {
		return nil, nil
	}
	l := langs[0]
	key := fmt.Sprintf("query\x00%+v\x00%v\x00%s\x00%v\x00%v\x00%d\x00%d\x00%d\x00%v", l, v.Episode, v.Title, v.Series, v.AlternativeSeries,
		v.Year, v.Season, v.Number, [2]string{v.IMDbID(), v.SeriesExternalIDs["imdb"]})
	return cached(p, key, queriesExpiration, func() ([]provider.Subtitle, error) { return p.query(ctx, v, l) })
}

func (p *Provider) query(ctx context.Context, v *provider.Video, l provider.Language) ([]provider.Subtitle, error) {
	name, imdb, season := v.Title, v.IMDbID(), 0
	if v.Episode {
		name, imdb, season = v.Series, v.SeriesExternalIDs["imdb"], v.Season
	}
	if imdb == "" {
		p.env.Log.DebugContext(ctx, "No title id found for this video")
		return nil, nil
	}
	titleID, err := p.searchTitles(ctx, v, name, imdb, season)
	if err != nil || titleID == 0 {
		return nil, err
	}
	language, ok := names[l.Tag]
	if !ok {
		return nil, provider.Errorf(provider.ConfigurationError, "Unsupported language code for subsource: %s", l.Tag)
	}
	q := url.Values{"language": {strings.ToLower(language)}, "limit": {"100"}, "movieId": {strconv.Itoa(titleID)}}
	if v.Episode {
		q.Set("seasonNumber", strconv.Itoa(v.Season))
		q.Set("episodeNumber", strconv.Itoa(v.Number))
	}
	r, err := p.get(ctx, "subtitles", q)
	if err != nil {
		return nil, err
	}
	var result struct {
		Success *bool  `json:"success"`
		Data    []item `json:"data"`
	}
	if err := r.JSON(&result); err != nil {
		return nil, err
	}
	if result.Success != nil && !*result.Success {
		return nil, nil
	}
	var out []provider.Subtitle
	for i := range result.Data {
		it := &result.Data[i]
		forced := isForced(it)
		if forced && !l.Forced {
			continue
		}
		hi := isHI(it)
		if !hi && l.HearingImpaired {
			continue
		}
		tag := tagOf(it.Language)
		if tag == "" {
			return nil, provider.Errorf(provider.ConfigurationError, "Unsupported language name for subsource: %s", it.Language)
		}
		ref := ref{ID: int(it.SubtitleID), Forced: forced}
		matches := []string{"title", "imdb_id"}
		if v.Episode {
			season, episodes := seasonEpisode(it.ReleaseInfo)
			if season != v.Season || len(episodes) > 1 || len(episodes) == 1 && episodes[0] != v.Number {
				continue
			}
			ref.Season = season
			matches = []string{"series", "series_imdb_id"}
			if len(episodes) == 0 {
				ref.Pack = true
				matches = append(matches, "episode")
			} else {
				ref.Episode = episodes[0]
			}
		}
		out = append(out, provider.Subtitle{
			ID:           ref.String(),
			Language:     provider.Language{Tag: tag, Forced: forced, HearingImpaired: hi},
			ReleaseNames: it.ReleaseInfo, Matches: matches,
			PageLink: siteURL + it.Link, Uploader: uploader(it),
		})
	}
	return out, nil
}

// seasonEpisode returns the season and episodes the release names tell,
// the first of each found, as Bazarr's _get_season_episode_from_release_info.
func seasonEpisode(releases []string) (int, []int) {
	var season int
	var episodes []int
	for _, r := range releases {
		if season != 0 && episodes != nil {
			break
		}
		g := archivemix.Parse(r)
		if season == 0 && g.Season != 0 {
			season = g.Season
		}
		if episodes == nil && len(g.Episodes) > 0 && g.Episodes[0] != 0 {
			episodes = g.Episodes
		}
	}
	return season, episodes
}

// truthy is Python's bool of a JSON value.
func truthy(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case float64:
		return x != 0
	case string:
		return x != ""
	case []any:
		return len(x) > 0
	case map[string]any:
		return len(x) > 0
	}
	return true
}

func containsAny(s string, subs []string) bool {
	for _, x := range subs {
		if strings.Contains(s, x) {
			return true
		}
	}
	return false
}

var (
	nonHITags  = []string{"hi remove", "non hi", "nonhi", "non-hi", "non-sdh", "non sdh", "nonsdh", "sdh remove"}
	hiTags     = []string{"_hi_", " hi ", ".hi.", "hi ", " hi", "sdh", "𝓢𝓓𝓗", "_cc_", " cc ", ".cc.", "closed caption"}
	forcedTags = []string{"forced", "foreign"}
)

// isHI tells hearing impaired subtitles by their flag or commentary, as
// Bazarr's _is_hi.
func isHI(it *item) bool {
	if truthy(it.HearingImpaired) {
		return true
	}
	c, ok := it.Commentary.(string)
	if !ok {
		return false
	}
	c = strings.ToLower(c)
	return !containsAny(c, nonHITags) && containsAny(c, hiTags)
}

// isForced tells forced subtitles by their flag or commentary, as
// Bazarr's _is_forced.
func isForced(it *item) bool {
	if truthy(it.ForeignParts) {
		return true
	}
	c, ok := it.Commentary.(string)
	return ok && containsAny(strings.ToLower(c), forcedTags)
}

// uploader returns the display name of the uploader among the
// contributors, as Bazarr's _get_uploader_name.
func uploader(it *item) string {
	for _, c := range it.Contributors {
		if fmt.Sprint(c.ID) == fmt.Sprint(it.UploaderID) {
			return c.DisplayName
		}
	}
	return ""
}

// ref is what Download needs of a subtitle: the fields of Bazarr's
// subtitle the archive mixin reads.
type ref struct {
	ID, Season, Episode int
	Pack, Forced        bool
}

func (r ref) String() string {
	return fmt.Sprintf("%d:%d:%d:%t:%t", r.ID, r.Season, r.Episode, r.Pack, r.Forced)
}

func parseRef(s string) (ref, error) {
	var r ref
	if _, err := fmt.Sscanf(s, "%d:%d:%d:%t:%t", &r.ID, &r.Season, &r.Episode, &r.Pack, &r.Forced); err != nil {
		return r, provider.Errorf(provider.BadSubtitle, "bad subtitle ID %q: %w", s, err)
	}
	return r, nil
}

// Download fetches a subtitle's zip archive and picks its subtitle, as
// Bazarr's download_subtitle with the archive mixin.
func (p *Provider) Download(ctx context.Context, v *provider.Video, id string) (provider.File, error) {
	r, err := parseRef(id)
	if err != nil {
		return provider.File{}, err
	}
	link := fmt.Sprintf("subtitles/%d/download", r.ID)
	data, err := cached(p, "archive\x00"+link, archivesExpiration, func() ([]byte, error) {
		resp, err := p.get(ctx, link, url.Values{})
		if err != nil {
			return nil, err
		}
		return resp.Body, nil
	})
	if err != nil {
		return provider.File{}, err
	}
	if _, err := zip.NewReader(bytes.NewReader(data), int64(len(data))); err != nil {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "could not unzip subtitle %d", r.ID)
	}
	a, err := archive.Open(data)
	if err != nil {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "subtitle %d: %w", r.ID, err)
	}
	w := archivemix.Want{Forced: r.Forced, Season: r.Season, Episode: r.Episode, Pack: r.Pack}
	if v != nil && v.Episode {
		w.AskedForEpisode, w.ReleaseGroup = v.Number, v.ReleaseGroup
	}
	sub, name, err := archivemix.Subtitle(a, w)
	if err != nil {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "subtitle %d: %w", r.ID, err)
	}
	if sub == nil {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "no subtitle of the video in subtitle %d", r.ID)
	}
	format := strings.TrimPrefix(strings.ToLower(name[strings.LastIndex(name, ".")+1:]), ".")
	if format == "sub" {
		format = archive.Detect(sub)
	}
	return provider.File{Data: sub, Format: format}, nil
}
