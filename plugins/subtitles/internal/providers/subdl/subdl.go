// Package subdl is the provider of SubDL, ported from Bazarr's subdl:
// subtitles of movies and episodes found by IMDb ID, TMDB ID or title
// through SubDL's API, season packs unpacked by the server, and the
// translations SubDL's API makes of missing languages for its paying
// accounts.
package subdl

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/archive"
	"github.com/mavioai/mavio/plugins/subtitles/internal/archivemix"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/site"
)

// Name is the provider's name.
const Name = "subdl"

// userAgent identifies the plugin, as Bazarr's SZ_USER_AGENT.
const userAgent = "Mavio v0.1"

const (
	apiURL      = "https://api.subdl.com/api/v1/"
	downloadURL = "https://dl.subdl.com"
	siteURL     = "https://subdl.com"
	proURL      = "https://subdl.com/pro?ref=bazarr"
)

const (
	// subsPerPage is the API's cap on results per page.
	subsPerPage = 30
	// maxPages is how many pages of the primary search are read.
	maxPages = 2
	// maxRetryAttempts bounds the waits for rate limits and a busy
	// service together.
	maxRetryAttempts = 6
	// pollInterval is how often a translation job is polled.
	pollInterval = 4 * time.Second
	// minDeadline and maxDeadline bound the wait for a translation.
	minDeadline = 90 * time.Second
	maxDeadline = 180 * time.Second
	// quotaPause is how long no translations are offered once the
	// monthly quota is exhausted.
	quotaPause = 900 * time.Second
)

// Definition returns the provider's definition.
func Definition() provider.Definition {
	return provider.Definition{
		Name: Name, DisplayName: "SubDL",
		Settings: []provider.Setting{
			{Name: "api_key", Label: "API key", Type: provider.Password},
			{
				Name: "ai_translate", Label: "AI-translate missing languages (requires SubDL Plus or Pro; also includes AI-translated subtitles in results)",
				Type: provider.Checkbox, Default: false,
			},
			{Name: "include_ai_translated", Label: "Include AI-translated subtitles in search results", Type: provider.Checkbox, Default: false},
		},
		Languages:                 languages(),
		HearingImpairedVerifiable: true, Movies: true, Episodes: true,
		Throttle: map[provider.Kind]func(time.Time) time.Duration{
			provider.ProviderError:         provider.Fixed(time.Hour),
			provider.DownloadLimitExceeded: provider.UntilMidnight("Etc/GMT", 15*time.Minute),
			provider.APIThrottled:          provider.Fixed(15 * time.Minute),
		},
		New: New,
	}
}

// codes are SubDL's language codes by BCP 47 tag, as Bazarr's
// SubdlConverter; Traditional Chinese is zh-TW, as Bazarr's.
var codes = map[string]string{
	"ar": "AR", "da": "DA", "nl": "NL", "en": "EN", "fa": "FA", "fi": "FI", "fr": "FR", "id": "ID", "it": "IT",
	"no": "NO", "ro": "RO", "es": "ES", "sv": "SV", "vi": "VI", "sq": "SQ", "az": "AZ", "be": "BE", "bn": "BN",
	"bs": "BS", "bg": "BG", "my": "MY", "ca": "CA", "zh": "ZH", "hr": "HR", "cs": "CS", "eo": "EO", "et": "ET",
	"ka": "KA", "de": "DE", "el": "EL", "kl": "KL", "he": "HE", "hi": "HI", "hu": "HU", "is": "IS", "ja": "JA",
	"ko": "KO", "ku": "KU", "lv": "LV", "lt": "LT", "mk": "MK", "ms": "MS", "ml": "ML", "pl": "PL", "pt": "PT",
	"ru": "RU", "sr": "SR", "si": "SI", "sk": "SK", "sl": "SL", "tl": "TL", "ta": "TA", "te": "TE", "th": "TH",
	"tr": "TR", "uk": "UK", "ur": "UR", "hy": "HY", "kk": "KK", "ky": "KY", "km": "KM", "kn": "KN", "mn": "MN",
	"eu": "EU", "gl": "GL", "ga": "GA", "jv": "JV", "su": "SU", "pt-BR": "BR_PT", "zh-TW": "ZH_BG",
}

func languages() []string {
	out := make([]string, 0, len(codes))
	for tag := range codes {
		out = append(out, tag)
	}
	slices.Sort(out)
	return out
}

// fromSite returns the tag of a SubDL language code; "" when unknown.
func fromSite(code string) string {
	for tag, c := range codes {
		if c == code {
			return tag
		}
	}
	return ""
}

// settings are the provider's settings.
type settings struct {
	APIKey              string `json:"api_key"`
	AITranslate         bool   `json:"ai_translate"`
	IncludeAITranslated bool   `json:"include_ai_translated"`
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
		return nil, provider.Errorf(provider.ConfigurationError, "api_key must be specified")
	}
	sess := site.New(env.HTTP)
	sess.Header.Set("User-Agent", userAgent)
	// Answers are checked as Bazarr's checked, not by status alone.
	sess.Check = func(*site.Response) error { return nil }
	return &Provider{
		env: env, s: s, session: sess, api: apiURL, dl: downloadURL,
		sleep: site.Sleep, poll: pollInterval,
		policy: defaultPolicy(), notices: map[string]bool{},
	}, nil
}

// Provider is SubDL.
type Provider struct {
	env     provider.Env
	s       settings
	session *site.Session
	api, dl string
	sleep   func(context.Context, time.Duration) error
	poll    time.Duration

	mu        sync.Mutex
	policy    policy
	logged    int
	quotaFull time.Time
	notices   map[string]bool
}

// SetBaseURL points the provider's API and downloads at another server,
// for tests.
func (p *Provider) SetBaseURL(u *url.URL) {
	base := strings.TrimSuffix(u.String(), "/")
	p.api, p.dl = base+"/api/v1/", base
}

// policy is the bounded policy SubDL's API sends Bazarr.
type policy struct {
	revision                              int
	enabled, aiTranslation                bool
	maxPages                              int
	seasonFallback, titleFallback, unpack bool
	message                               string
}

func defaultPolicy() policy {
	return policy{
		revision: 1, enabled: true, aiTranslation: true, maxPages: maxPages,
		seasonFallback: true, titleFallback: true, unpack: true,
	}
}

func (p *Provider) currentPolicy() policy {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.policy
}

// applyPolicy accepts the policy of an answer, as Bazarr's
// _apply_bazarr_policy.
func (p *Provider) applyPolicy(ctx context.Context, raw map[string]any) {
	pol, ok := raw["bazarr_policy"].(map[string]any)
	if !ok {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for key, field := range map[string]*bool{
		"enabled": &p.policy.enabled, "ai_translation_enabled": &p.policy.aiTranslation,
		"season_fallback_enabled": &p.policy.seasonFallback, "title_fallback_enabled": &p.policy.titleFallback,
		"unpack_enabled": &p.policy.unpack,
	} {
		if b, ok := pol[key].(bool); ok {
			*field = b
		}
	}
	if n, ok := integer(pol["max_pages"]); ok {
		p.policy.maxPages = max(1, min(n, maxPages))
	}
	if n, ok := integer(pol["revision"]); ok && n > 0 {
		p.policy.revision = n
	}
	p.policy.message = ""
	if m, ok := pol["message"].(string); ok && strings.TrimSpace(m) != "" {
		p.policy.message = string([]rune(m)[:min(len([]rune(m)), 160)])
	}
	if p.policy.message != "" && p.logged != p.policy.revision {
		p.env.Log.InfoContext(ctx, "subdl: "+p.policy.message)
		p.logged = p.policy.revision
	}
}

// integer returns a JSON number that is an integer.
func integer(v any) (int, bool) {
	f, ok := v.(float64)
	if !ok || f != float64(int(f)) {
		return 0, false
	}
	return int(f), true
}

// truthy reports whether a JSON value is true as Python reads it.
func truthy(v any) bool {
	switch v := v.(type) {
	case nil:
		return false
	case bool:
		return v
	case float64:
		return v != 0
	case string:
		return v != ""
	case []any:
		return len(v) > 0
	case map[string]any:
		return len(v) > 0
	}
	return true
}

// isErrorPayload reports whether an answer reports an error, as Bazarr's
// _is_error_payload.
func isErrorPayload(raw map[string]any) bool {
	if v, ok := raw["success"]; ok && !truthy(v) {
		return true
	}
	if v, ok := raw["status"]; ok && !truthy(v) {
		return true
	}
	return false
}

var (
	// errRejected is a request refused for a reason of its own, as
	// Bazarr's SubdlRequestRejected: it says nothing of the provider's
	// health.
	errRejected = errors.New("request rejected")
	// errNotFound is a request rejected with a bare 404.
	errNotFound = fmt.Errorf("%w: resource not found", errRejected)
	// errTransport is a request that failed on the network.
	errTransport = errors.New("request failed")
)

// unsafeTitle are the characters the API refuses in titles.
var unsafeTitle = regexp.MustCompile("[<>{}\\[\\]'\"`´’;\\\\/]")

// sanitizeTitle replaces the characters the API refuses with spaces, as
// Bazarr's _sanitize_title.
func sanitizeTitle(title string) string {
	return strings.Join(strings.Fields(unsafeTitle.ReplaceAllString(title, " ")), " ")
}

// safeJSON decodes an answer's object; empty when it is none.
func safeJSON(r *site.Response) map[string]any {
	var out map[string]any
	if err := json.Unmarshal(r.Body, &out); err != nil || out == nil {
		return map[string]any{}
	}
	return out
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

// checked makes a request and maps the API's answers to failures, as
// Bazarr's checked: rate limits and a busy service are waited out a few
// times.
func (p *Provider) checked(ctx context.Context, do func() (*site.Response, error)) (*site.Response, error) {
	busy, attempts := 0, 0
	for {
		r, err := do()
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, fmt.Errorf("%w: %w", errTransport, err)
		}
		switch c := r.Status; {
		case c == http.StatusPaymentRequired:
			msg := "Active paid SubDL subscription required"
			if m := str(safeJSON(r)["message"]); m != "" {
				msg = m
			}
			return nil, provider.Errorf(provider.ProviderError, "%s", msg)
		case c == http.StatusForbidden:
			payload := safeJSON(r)
			reason := strings.ToLower(str(payload["error"]))
			if reason == "" {
				reason = strings.ToLower(str(payload["message"]))
			}
			if len(payload) > 0 && (strings.Contains(reason, "key") || strings.Contains(reason, "auth") || strings.Contains(reason, "forbidden")) {
				return nil, provider.Errorf(provider.AuthenticationError, "invalid API key")
			}
			if reason == "" {
				reason = "no reason given"
			}
			return nil, provider.Errorf(provider.APIThrottled, "request rejected with 403: %s", reason)
		case c == http.StatusNotFound:
			return nil, provider.Errorf(provider.ProviderError, "%w", errNotFound)
		case c == http.StatusTooManyRequests:
			payload := safeJSON(r)
			reason, ok := payload["error"]
			switch {
			case !ok:
			case reason == "daily_limit" || reason == "api_download_limit_exceeded":
				return nil, provider.Errorf(provider.DownloadLimitExceeded, "daily download limit exceeded")
			case reason == "rate_limit":
				if attempts < maxRetryAttempts {
					p.env.Log.DebugContext(ctx, "subdl: API request rate limit hit, waiting and trying again")
					if err := p.sleep(ctx, retryAfter(r, 15)); err != nil {
						return nil, err
					}
					attempts++
					continue
				}
				return nil, provider.Errorf(provider.APIThrottled, "API request limit hit")
			case reason == "service_busy":
				if busy < 5 && attempts < maxRetryAttempts {
					p.env.Log.DebugContext(ctx, "subdl: API service is busy, waiting and trying again")
					if err := p.sleep(ctx, retryAfter(r, 5)); err != nil {
						return nil, err
					}
					busy++
					attempts++
					continue
				}
				return nil, provider.Errorf(provider.ProviderError, "API service is busy")
			}
			return nil, provider.Errorf(provider.APIThrottled, "rate limited without a usable error field")
		case c >= 400 && c < 500:
			payload := safeJSON(r)
			msg := str(payload["error"])
			if msg == "" {
				msg = str(payload["message"])
			}
			if msg == "" {
				msg = "HTTP " + strconv.Itoa(c)
			}
			return nil, provider.Errorf(provider.ProviderError, "%w (%d): %s", errRejected, c, msg)
		case c != http.StatusOK:
			return nil, provider.Errorf(provider.ServiceUnavailable, "subdl returned HTTP %d", c)
		}
		return r, nil
	}
}

// retryAfter returns the wait a response asks for, from 0 to 60 seconds.
func retryAfter(r *site.Response, def int) time.Duration {
	n, err := strconv.Atoi(r.Header.Get("Retry-After"))
	if err != nil {
		n = def
	}
	return time.Duration(max(0, min(n, 60))) * time.Second
}

// search runs one search, as Bazarr's _search: it never fails for this
// search alone, but for the state of the account or the service.
func (p *Provider) search(ctx context.Context, params url.Values, what string, paginate bool) ([]item, map[string]any, error) {
	var items []item
	first := map[string]any{}
	for page := 1; ; page++ {
		pages := 1
		if paginate {
			pages = p.currentPolicy().maxPages
		}
		if page > pages {
			break
		}
		q := url.Values{}
		for k, v := range params {
			q[k] = v
		}
		q.Set("api_key", p.s.APIKey)
		if !p.currentPolicy().unpack {
			q.Del("unpack")
		}
		q.Set("subs_per_page", strconv.Itoa(subsPerPage))
		if page > 1 {
			q.Set("page", strconv.Itoa(page))
		}
		r, err := p.checked(ctx, func() (*site.Response, error) { return p.session.Get(ctx, p.api+"subtitles", q) })
		switch {
		case errors.Is(err, errNotFound):
			return nil, nil, provider.Errorf(provider.ServiceUnavailable, "SubDL search endpoint unavailable")
		case errors.Is(err, errRejected), errors.Is(err, errTransport):
			p.env.Log.DebugContext(ctx, "subdl: search failed, continuing without it", "search", what, "err", err)
			return items, first, nil
		case err != nil:
			return nil, nil, err
		}
		raw := safeJSON(r)
		var payload struct {
			Subtitles  []item `json:"subtitles"`
			TotalPages any    `json:"totalPages"`
		}
		if json.Unmarshal(r.Body, &payload) != nil {
			payload.Subtitles = nil
		}
		if page == 1 {
			first = raw
			p.applyPolicy(ctx, raw)
		}
		if isErrorPayload(raw) {
			p.env.Log.DebugContext(ctx, "subdl: search returned an error", "search", what, "error", raw["error"])
			break
		}
		items = append(items, payload.Subtitles...)
		total, ok := integer(payload.TotalPages)
		if !ok || total == 0 {
			total = 1
		}
		if !paginate || page >= total || len(payload.Subtitles) < subsPerPage {
			break
		}
	}
	return items, first, nil
}

// num is a number of the API that may be missing, null, or a string.
type num struct {
	n  int
	ok bool
}

func (n *num) UnmarshalJSON(b []byte) error {
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch v := v.(type) {
	case float64:
		*n = num{int(v), true}
	case string:
		if i, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			*n = num{i, true}
		}
	}
	return nil
}

// positive returns a number that is above 0, as Bazarr's _episode_number.
func (n num) positive() (int, bool) { return n.n, n.ok && n.n > 0 }

// strs is a list of the API whose strings are kept.
type strs []string

func (s *strs) UnmarshalJSON(b []byte) error {
	var v []any
	if json.Unmarshal(b, &v) != nil {
		return nil
	}
	for _, x := range v {
		if x, ok := x.(string); ok {
			*s = append(*s, x)
		}
	}
	return nil
}

// flag is a boolean of the API, true as Python reads it.
type flag bool

func (f *flag) UnmarshalJSON(b []byte) error {
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	*f = flag(truthy(v))
	return nil
}

// text is a string of the API, or what else it sent printed.
type text string

func (t *text) UnmarshalJSON(b []byte) error {
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch v := v.(type) {
	case nil:
		*t = ""
	case string:
		*t = text(v)
	default:
		*t = text(strings.Trim(string(b), `"`))
	}
	return nil
}

// unpackFile is a file of a pack the server extracted.
type unpackFile struct {
	URL     string `json:"url"`
	FileNID text   `json:"file_n_id"`
	Name    string `json:"name"`
	Season  num    `json:"season"`
	Episode num    `json:"episode"`
	HI      flag   `json:"hi"`
}

// item is a subtitle of a search.
type item struct {
	URL          string       `json:"url"`
	Name         string       `json:"name"`
	Language     string       `json:"language"`
	SubtitlePage string       `json:"subtitlePage"`
	Releases     strs         `json:"releases"`
	Author       string       `json:"author"`
	Comment      string       `json:"comment"`
	HI           flag         `json:"hi"`
	Season       num          `json:"season"`
	Episode      num          `json:"episode"`
	EpisodeFrom  num          `json:"episode_from"`
	EpisodeEnd   num          `json:"episode_end"`
	FullSeason   flag         `json:"full_season"`
	AITranslated flag         `json:"ai_translated"`
	UnpackFiles  []unpackFile `json:"unpack_files"`
}

// texts are what the heuristics read of an item.
func (it *item) texts() []string {
	return append([]string{it.Comment, it.Name}, it.Releases...)
}

var (
	// hiWord are the markers of subtitles for the hearing-impaired, as
	// whole words.
	hiWord = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])(?:hi|sdh|cc)(?:[^a-z0-9]|$)|_hi_|𝓢𝓓𝓗|hearing[\s._-]?impaired|closed[\s._-]?caption`)
	// nonHI are the explicit markers of subtitles that are not.
	nonHI = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])(?:non[\s._-]?(?:hi|sdh)|(?:hi|sdh)[\s._-]?removed?|removed?[\s._-]?(?:hi|sdh))(?:[^a-z0-9]|$)`)
	// forcedWord are the markers of forced subtitles.
	forcedWord = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])(?:forced|foreign[\s._-]?parts?)(?:[^a-z0-9]|$)`)
)

func anyMatch(re *regexp.Regexp, texts []string) bool {
	return slices.ContainsFunc(texts, re.MatchString)
}

// isHI reports whether the texts of an item mark it for the
// hearing-impaired, as Bazarr's _is_hi.
func isHI(it *item) bool {
	return !anyMatch(nonHI, it.texts()) && anyMatch(hiWord, it.texts())
}

// hearingImpaired combines the API's flag with the heuristics, as
// Bazarr's _is_hearing_impaired: explicit non-HI markers win.
func hearingImpaired(it *item) bool {
	if anyMatch(nonHI, it.texts()) {
		return false
	}
	return bool(it.HI) || isHI(it)
}

// isForced reports whether an item is forced, as Bazarr's _is_forced.
func isForced(it *item) bool { return anyMatch(forcedWord, it.texts()) }

// episodeRange reads the range of episodes of a pack from its release
// names, as Bazarr's _parse_episode_range_from_releases.
func episodeRange(releases []string) (int, int, bool) {
	for _, r := range releases {
		if eps := archivemix.Parse(r).Episodes; len(eps) >= 2 {
			return eps[0], eps[len(eps)-1], true
		}
	}
	return 0, 0, false
}

// selectUnpack picks the file of a pack for the episode, as Bazarr's
// _select_unpack_entry: the server's episode number is checked against
// the file's name, and a file of the pack's HI class is preferred.
func selectUnpack(files []unpackFile, target, absolute int, preferHI bool) *unpackFile {
	wanted := []int{target}
	if absolute != 0 {
		wanted = append(wanted, absolute)
	}
	type ranked struct{ rank, mismatch, index int }
	var best *ranked
	var picked *unpackFile
	for i := range files {
		f := &files[i]
		claimed := f.Episode.ok && slices.Contains(wanted, f.Episode.n)
		nameEp, named := archivemix.Single(f.Name)
		var rank int
		switch {
		case named && slices.Contains(wanted, nameEp):
			rank = 1
			if claimed {
				rank = 0
			}
		case claimed && !named:
			rank = 2
		default:
			continue
		}
		mismatch := 1
		if bool(f.HI) == preferHI {
			mismatch = 0
		}
		r := ranked{rank, mismatch, i}
		if best == nil || r.rank < best.rank || r.rank == best.rank && r.mismatch < best.mismatch {
			best, picked = &r, f
		}
	}
	return picked
}

// candidate is a subtitle found, before it is filtered.
type candidate struct {
	sub             provider.Subtitle
	season, episode num
	pack            bool
}

// Search implements provider.Provider.
func (p *Provider) Search(ctx context.Context, v *provider.Video, langs []provider.Language) ([]provider.Subtitle, error) {
	title := v.Title
	if v.Episode {
		title = v.Series
	}
	title = sanitizeTitle(title)
	imdbID, tmdbID := "", ""
	if v.Episode {
		imdbID = v.SeriesExternalIDs["imdb"]
	} else {
		imdbID, tmdbID = v.ExternalIDs["imdb"], v.ExternalIDs["tmdb"]
	}

	var siteCodes []string
	for _, l := range langs {
		if c, ok := codes[l.Tag]; ok && !slices.Contains(siteCodes, c) {
			siteCodes = append(siteCodes, c)
		}
	}
	slices.Sort(siteCodes)
	base := url.Values{
		"bazarr": {"1"}, "comment": {"1"}, "languages": {strings.Join(siteCodes, ",")}, "releases": {"1"}, "unpack": {"1"},
	}
	if imdbID != "" {
		base.Set("imdb_id", imdbID)
	} else {
		base.Set("film_name", title)
	}
	// Results of an ID search may claim the ID's matches; those of a
	// fuzzy title search may not.
	matchedID, matchedTitle := imdbID != "", imdbID == ""

	var all []item
	seen := map[string]bool{}
	merge := func(items []item) int {
		added := 0
		for _, it := range items {
			key := it.SubtitlePage
			if key == "" {
				key = it.Name
			}
			if seen[key] {
				continue
			}
			seen[key] = true
			all = append(all, it)
			added++
		}
		return added
	}
	with := func(kv ...string) url.Values {
		q := url.Values{}
		for k, val := range base {
			q[k] = val
		}
		for i := 0; i < len(kv); i += 2 {
			q.Set(kv[i], kv[i+1])
		}
		return q
	}

	var first map[string]any
	if v.Episode {
		items, payload, err := p.search(ctx, with("type", "tv", "episode_number", strconv.Itoa(v.Number), "season_number", strconv.Itoa(v.Season)), "episode", true)
		if err != nil {
			return nil, err
		}
		first = payload
		merge(items)
		if !p.currentPolicy().enabled {
			return nil, nil
		}
		if v.AbsoluteNumber != 0 && v.AbsoluteNumber != v.Number {
			items, _, err := p.search(ctx, with("type", "tv", "episode_number", strconv.Itoa(v.AbsoluteNumber)), "absolute episode", false)
			if err != nil {
				return nil, err
			}
			merge(items)
		}
		if p.currentPolicy().seasonFallback {
			items, _, err := p.search(ctx, with("type", "tv", "season_number", strconv.Itoa(v.Season)), "season-only", false)
			if err != nil {
				return nil, err
			}
			merge(items)
		}
		if len(all) == 0 && p.currentPolicy().titleFallback {
			items, _, err := p.search(ctx, with("type", "tv"), "title-only", false)
			if err != nil {
				return nil, err
			}
			merge(items)
		}
	} else {
		items, payload, err := p.search(ctx, with("type", "movie"), "movie", true)
		if err != nil {
			return nil, err
		}
		first = payload
		merge(items)
		if !p.currentPolicy().enabled {
			return nil, nil
		}
		if len(all) == 0 && tmdbID != "" {
			q := with("type", "movie", "tmdb_id", tmdbID)
			q.Del("film_name")
			q.Del("imdb_id")
			items, payload, err := p.search(ctx, q, "tmdb", true)
			if err != nil {
				return nil, err
			}
			if merge(items) > 0 {
				matchedID, matchedTitle = true, false
			}
			if len(first) == 0 {
				first = payload
			}
		}
	}

	var found []candidate
	for i := range all {
		c, ok := p.candidate(v, &all[i], matchedID, matchedTitle)
		if !ok {
			continue
		}
		if slices.Contains(langs, c.sub.Language) {
			found = append(found, c)
		}
	}
	found = append(found, p.translations(ctx, v, langs, found, first["translation"], matchedID, matchedTitle)...)
	out := make([]provider.Subtitle, 0, len(found))
	for _, c := range found {
		out = append(out, c.sub)
	}
	return out, nil
}

// candidate makes the subtitle of an item, as Bazarr's query; false when
// the item is left out.
func (p *Provider) candidate(v *provider.Video, it *item, matchedID, matchedTitle bool) (candidate, bool) {
	if bool(it.AITranslated) && !p.s.IncludeAITranslated && !p.s.AITranslate {
		return candidate{}, false
	}
	tag := fromSite(it.Language)
	if tag == "" {
		return candidate{}, false
	}
	hi := hearingImpaired(it)
	c := candidate{season: it.Season, episode: it.Episode}
	link, kind := it.URL, ""
	if v.Episode {
		from, hasFrom := it.EpisodeFrom.positive()
		end, hasEnd := it.EpisodeEnd.positive()
		fullSeason := bool(it.FullSeason)
		// The API's range is authoritative; release names are read only
		// when it gave none.
		if !hasFrom && !hasEnd && !fullSeason {
			if f, e, ok := episodeRange(it.Releases); ok && f != e {
				from, end, hasFrom, hasEnd = f, e, true, true
			}
		}
		hasRange := hasFrom && hasEnd && from != end
		if hasRange {
			in := func(e int) bool { return from <= e && e <= end }
			if !in(v.Number) && (v.AbsoluteNumber == 0 || !in(v.AbsoluteNumber)) {
				return candidate{}, false
			}
		}
		if hasRange || fullSeason {
			c.pack, kind = true, "pack"
			// The file the server extracted is preferred to the pack.
			if f := selectUnpack(it.UnpackFiles, v.Number, v.AbsoluteNumber, hi); f != nil {
				link = f.URL
				if s, ok := f.Season.positive(); ok {
					c.season = num{s, true}
				}
				wanted := []int{v.Number}
				if v.AbsoluteNumber != 0 {
					wanted = append(wanted, v.AbsoluteNumber)
				}
				nameEp, named := archivemix.Single(f.Name)
				serverEp, server := f.Episode.positive()
				switch {
				case named && slices.Contains(wanted, nameEp):
					c.episode = num{nameEp, true}
				case server && slices.Contains(wanted, serverEp):
					c.episode = num{serverEp, true}
				}
				c.pack, kind = false, "file"
			}
		}
	}
	uploader := it.Author
	switch {
	case bool(it.AITranslated) && uploader != "":
		uploader += " (AI translated)"
	case bool(it.AITranslated):
		uploader = "AI translated"
	}
	id := link
	if kind != "" {
		id = kind + ":" + link
	}
	c.sub = provider.Subtitle{
		ID:           id,
		Language:     provider.Language{Tag: tag, Forced: isForced(it), HearingImpaired: hi},
		ReleaseNames: it.Releases,
		PageLink:     resolve(siteURL, it.SubtitlePage),
		Uploader:     uploader,
		Comment:      it.Comment,
	}
	c.sub.Matches = matches(v, c, matchedID, matchedTitle)
	return c, true
}

// matches returns what a subtitle matches of the video, as Bazarr's
// get_matches; the server adds those of the release names.
func matches(v *provider.Video, c candidate, matchedID, matchedTitle bool) []string {
	var m []string
	if v.Episode {
		switch {
		case matchedID && v.SeriesExternalIDs["imdb"] != "":
			m = append(m, "series", "series_imdb_id")
			if v.Year != 0 {
				m = append(m, "year")
			}
		case matchedTitle:
			m = append(m, "series")
		}
		byAbsolute := false
		switch {
		case c.episode.ok && c.episode.n == v.Number:
			m = append(m, "episode")
		case v.AbsoluteNumber != 0 && c.episode.ok && c.episode.n == v.AbsoluteNumber:
			m = append(m, "episode")
			byAbsolute = true
		case c.pack:
			m = append(m, "episode")
		}
		switch {
		case c.season.ok && c.season.n == v.Season:
			m = append(m, "season")
		case c.pack && v.AbsoluteNumber != 0, byAbsolute:
			// Arcs and absolute numbers tie the season too.
			m = append(m, "season")
		}
		return m
	}
	switch {
	case matchedID:
		if v.ExternalIDs["imdb"] != "" {
			m = append(m, "imdb_id")
		}
		if v.ExternalIDs["tmdb"] != "" {
			m = append(m, "tmdb_id")
		}
		m = append(m, "title")
		if v.Year != 0 {
			m = append(m, "year")
		}
	case matchedTitle:
		m = append(m, "title")
	}
	return m
}

// resolve joins a link to a base URL, as urljoin.
func resolve(base, link string) string {
	b, err := url.Parse(base)
	if err != nil {
		return link
	}
	l, err := url.Parse(link)
	if err != nil {
		return link
	}
	return b.ResolveReference(l).String()
}

// translation is the availability of translations a search answers.
type translation struct {
	Entitled         bool     `json:"entitled"`
	MissingLanguages []string `json:"missing_languages"`
	UpgradeURL       string   `json:"upgrade_url"`
	QuotaResetAt     string   `json:"quota_reset_at"`
	Sources          []struct {
		NID      text   `json:"n_id"`
		Language string `json:"language"`
		HI       flag   `json:"hi"`
		Releases strs   `json:"releases"`
	} `json:"sources"`
}

// notice logs a notice once per video.
func (p *Provider) notice(ctx context.Context, scope, msg string) {
	p.mu.Lock()
	key := scope + "\x00" + msg
	seen := p.notices[key]
	p.notices[key] = true
	p.mu.Unlock()
	if !seen {
		p.env.Log.InfoContext(ctx, msg)
	}
}

func scope(v *provider.Video) string {
	if v.FileName != "" {
		return v.FileName
	}
	return v.Title
}

// translations returns the candidates of translations SubDL makes of the
// wanted languages it has no subtitles in, as Bazarr's
// _add_ai_translation_candidates.
func (p *Provider) translations(ctx context.Context, v *provider.Video, langs []provider.Language, found []candidate, raw any, matchedID, matchedTitle bool) []candidate {
	obj, ok := raw.(map[string]any)
	if !p.s.AITranslate || !p.currentPolicy().aiTranslation || !ok {
		return nil
	}
	p.mu.Lock()
	paused := p.env.Now().Before(p.quotaFull)
	p.mu.Unlock()
	if paused {
		return nil
	}
	data, _ := json.Marshal(obj)
	var tr translation
	if json.Unmarshal(data, &tr) != nil || len(tr.MissingLanguages) == 0 {
		return nil
	}
	if !tr.Entitled {
		upgrade := tr.UpgradeURL
		if upgrade == "" {
			upgrade = proURL
		}
		missing := slices.Sorted(slices.Values(tr.MissingLanguages))
		p.notice(ctx, scope(v), fmt.Sprintf("subdl: missing languages (%s) can be AI-translated in about a minute with a SubDL Plus/Pro subscription: %s",
			strings.Join(missing, ", "), upgrade))
		return nil
	}
	if len(tr.Sources) == 0 {
		reset := tr.QuotaResetAt
		if reset == "" {
			reset = "the 1st of next month"
		}
		p.notice(ctx, scope(v), "subdl: AI translation quota exhausted; more translations available after "+reset)
		return nil
	}
	// One best source per HI class: the server's order first, ties broken
	// by the release names that match the video.
	best := func(hi bool) int {
		pick, score := -1, -1
		for i, s := range tr.Sources {
			if bool(s.HI) != hi || s.NID == "" {
				continue
			}
			if n := releaseMatches(v, s.Releases); n > score {
				pick, score = i, n
			}
		}
		return pick
	}
	type key struct {
		tag    string
		hi, fo bool
	}
	existing := map[key]bool{}
	for _, c := range found {
		// Rows of other episodes do not count.
		if !v.Episode || slices.Contains(c.sub.Matches, "episode") {
			existing[key{c.sub.Language.Tag, c.sub.Language.HearingImpaired, c.sub.Language.Forced}] = true
		}
	}
	var out []candidate
	for _, l := range langs {
		if l.Forced {
			continue
		}
		i := best(l.HearingImpaired)
		if i < 0 || existing[key{l.Tag, l.HearingImpaired, false}] {
			continue
		}
		target, ok := codes[l.Tag]
		if !ok || !slices.Contains(tr.MissingLanguages, target) {
			continue
		}
		src := tr.Sources[i]
		c := candidate{pack: false}
		if v.Episode {
			c.season, c.episode = num{v.Season, true}, num{v.Number, true}
		}
		c.sub = provider.Subtitle{
			ID:           "ai:" + string(src.NID) + ":" + target,
			Language:     provider.Language{Tag: l.Tag, HearingImpaired: bool(src.HI)},
			ReleaseNames: src.Releases,
			PageLink:     proURL,
			Uploader:     "SubDL AI",
			Comment:      "AI translation (SubDL) from " + src.Language,
		}
		c.sub.Matches = matches(v, c, matchedID, matchedTitle)
		p.env.Log.DebugContext(ctx, "subdl: offering AI translation candidate", "target", target, "source", string(src.NID))
		out = append(out, c)
	}
	return out
}

// normalize lowercases a name and keeps its letters and digits.
func normalize(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// releaseMatches counts the properties of the video release names show,
// as the matches Bazarr's update_matches ranks translation sources by.
func releaseMatches(v *provider.Video, releases []string) int {
	n := 0
	for _, field := range []string{v.ReleaseGroup, v.Resolution, v.Source, v.VideoCodec, v.AudioCodec, v.StreamingService, v.Edition} {
		f := normalize(field)
		if f == "" {
			continue
		}
		if slices.ContainsFunc(releases, func(r string) bool { return strings.Contains(normalize(r), f) }) {
			n++
		}
	}
	return n
}

// Download implements provider.Provider.
func (p *Provider) Download(ctx context.Context, v *provider.Video, id string) (provider.File, error) {
	if rest, ok := strings.CutPrefix(id, "ai:"); ok {
		nID, target, ok := strings.Cut(rest, ":")
		if !ok || nID == "" || fromSite(target) == "" {
			return provider.File{}, provider.Errorf(provider.BadSubtitle, "malformed subtitle ID %q", id)
		}
		return p.downloadTranslation(ctx, v, nID, target)
	}
	kind, link, ok := strings.Cut(id, ":")
	if !ok || kind != "pack" && kind != "file" {
		kind, link = "", id
	}
	r, err := p.checked(ctx, func() (*site.Response, error) { return p.session.Get(ctx, resolve(p.dl, link), nil) })
	switch {
	case errors.Is(err, errRejected):
		// A subtitle that no longer exists fails this download only.
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "download rejected: %w", err)
	case errors.Is(err, errTransport):
		return provider.File{}, errors.Unwrap(err)
	case err != nil:
		return provider.File{}, err
	case len(r.Body) == 0:
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "could not download subtitle")
	}
	a, err := archive.Open(r.Body)
	if errors.Is(err, archive.ErrNotArchive) && kind == "file" {
		// The server's unpacked file is the subtitle itself.
		sub := archive.FixLineEndings(r.Body)
		return provider.File{Data: sub, Format: format(link, sub)}, nil
	}
	if err != nil {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "could not open subtitle archive: %w", err)
	}
	var name string
	var data []byte
	if kind == "pack" {
		name, data, err = p.fromPack(a, v)
	} else {
		name, data, err = firstSubtitle(a)
	}
	switch {
	case err != nil:
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "could not read archive: %w", err)
	case name == "":
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "no subtitle of the video in the archive")
	}
	return provider.File{Data: data, Format: format(name, data)}, nil
}

// fromPack returns the episode's subtitle of a pack, by its number, its
// title, then its absolute number, as Bazarr's _extract.
func (p *Provider) fromPack(a *archive.Archive, v *provider.Video) (string, []byte, error) {
	target := v.Number
	if target == 0 {
		target = v.AbsoluteNumber
	}
	name, data, err := archive.Subtitle(a, archive.Want{Episode: target, EpisodeTitle: v.Title})
	if err != nil || name != "" || v.AbsoluteNumber == 0 {
		return name, data, err
	}
	return archive.Subtitle(a, archive.Want{Episode: v.AbsoluteNumber})
}

// firstSubtitle returns the first subtitle of an archive, past folders
// and macOS resource forks, as Bazarr's _first_subtitle_in_archive.
func firstSubtitle(a *archive.Archive) (string, []byte, error) {
	for _, name := range a.Names() {
		if strings.HasSuffix(name, "/") {
			continue
		}
		base := path.Base(name)
		if base == "" || strings.HasPrefix(base, "._") || strings.HasPrefix(name, "__MACOSX/") {
			continue
		}
		switch strings.ToLower(path.Ext(base)) {
		case ".srt", ".sub", ".ssa", ".ass":
			data, err := a.Read(name)
			if err != nil {
				return "", nil, fmt.Errorf("%s: %w", name, err)
			}
			return name, archive.FixLineEndings(data), nil
		}
	}
	return "", nil, nil
}

// format returns the format of a subtitle by its name or content.
func format(name string, data []byte) string {
	f := strings.TrimPrefix(strings.ToLower(path.Ext(name)), ".")
	if f == "sub" || f == "" {
		return archive.Detect(data)
	}
	return f
}

// translationJob is the state of a translation job.
type translationJob struct {
	Status        string  `json:"status"`
	DownloadReady bool    `json:"download_ready"`
	Error         any     `json:"error"`
	ETAms         float64 `json:"eta_ms"`
}

// failTranslation is a translation that could not be made: the candidate
// fails, but the provider is not throttled for it.
func failTranslation(format string, args ...any) error {
	return provider.Errorf(provider.BadSubtitle, "AI translation: "+format, args...)
}

// downloadTranslation asks SubDL to translate the subtitle of a source,
// waits for the job and returns the translation, as Bazarr's
// _download_ai_translation.
func (p *Provider) downloadTranslation(ctx context.Context, v *provider.Video, nID, target string) (provider.File, error) {
	params := url.Values{"api_key": {p.s.APIKey}}
	body := map[string]any{"n_id": nID, "target_language": target}
	if v != nil && v.Episode {
		body["season"], body["episode"] = v.Season, v.Number
	}
	data, err := json.Marshal(body)
	if err != nil {
		return provider.File{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.api+"pro/translate/subtitles?"+params.Encode(), bytes.NewReader(data))
	if err != nil {
		return provider.File{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	r, err := p.session.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return provider.File{}, ctx.Err()
		}
		return provider.File{}, failTranslation("request failed: %w", err)
	}
	payload := safeJSON(r)
	if r.Status != http.StatusOK && r.Status != http.StatusAccepted {
		code := str(payload["error"])
		msg := str(payload["message"])
		if msg == "" {
			msg = code
		}
		if msg == "" {
			msg = "HTTP " + strconv.Itoa(r.Status)
		}
		switch code {
		case "translation_quota_exhausted":
			p.mu.Lock()
			p.quotaFull = p.env.Now().Add(quotaPause)
			p.mu.Unlock()
			p.notice(ctx, nID+":"+target, "subdl: AI translation quota exhausted; more translations available next month")
		case "translation_not_entitled":
			p.notice(ctx, nID+":"+target, "subdl: AI translation requires a SubDL Plus/Pro subscription: "+proURL)
		default:
			p.env.Log.ErrorContext(ctx, "subdl: AI translation request rejected", "message", msg)
		}
		return provider.File{}, failTranslation("request rejected: %s", msg)
	}
	var started struct {
		RequestID text           `json:"request_id"`
		Job       translationJob `json:"job"`
		Estimated float64        `json:"estimated_duration_ms"`
	}
	_ = json.Unmarshal(r.Body, &started)
	if started.RequestID == "" {
		return provider.File{}, failTranslation("response missing request_id")
	}
	job := url.PathEscape(string(started.RequestID))
	estimated := started.Estimated
	if estimated == 0 {
		estimated = started.Job.ETAms
	}
	if estimated == 0 {
		estimated = 60000
	}
	wait := min(max(time.Duration(estimated*2)*time.Millisecond, minDeadline), maxDeadline)
	deadline := p.env.Now().Add(wait)
	ready := started.Job.DownloadReady
	failures := 0
	for !ready && p.env.Now().Before(deadline) {
		if err := p.sleep(ctx, p.poll); err != nil {
			return provider.File{}, err
		}
		poll, err := p.session.Get(ctx, p.api+"pro/translate/jobs/"+job, params)
		if err != nil && ctx.Err() != nil {
			return provider.File{}, ctx.Err()
		}
		if err != nil || poll.Status != http.StatusOK {
			if failures++; failures >= 5 {
				return provider.File{}, failTranslation("polling failed repeatedly")
			}
			continue
		}
		failures = 0
		var state struct {
			Job translationJob `json:"job"`
		}
		_ = json.Unmarshal(poll.Body, &state)
		if state.Job.Status == "failed" {
			return provider.File{}, failTranslation("job %s failed: %v", started.RequestID, state.Job.Error)
		}
		ready = state.Job.DownloadReady
	}
	if !ready {
		p.env.Log.InfoContext(ctx, "subdl: AI translation job still running; the finished translation will be available as a regular subtitle on the next search",
			"job", string(started.RequestID))
		return provider.File{}, failTranslation("job %s still running", started.RequestID)
	}
	dl, err := p.session.Get(ctx, p.api+"pro/translate/jobs/"+job+"/download", params)
	if err != nil {
		if ctx.Err() != nil {
			return provider.File{}, ctx.Err()
		}
		return provider.File{}, failTranslation("download failed: %w", err)
	}
	if dl.Status != http.StatusOK || len(dl.Body) == 0 {
		return provider.File{}, failTranslation("download failed (HTTP %d)", dl.Status)
	}
	sub := archive.FixLineEndings(dl.Body)
	return provider.File{Data: sub, Format: archive.Detect(sub), Language: provider.Language{Tag: fromSite(target)}}, nil
}

// String names the provider in logs.
func (p *Provider) String() string { return Name }
