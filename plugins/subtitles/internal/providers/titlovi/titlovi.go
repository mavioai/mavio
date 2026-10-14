// Package titlovi is the Titlovi provider of ex-Yugoslav subtitles,
// ported from Bazarr's titlovi.py: Titlovi's Kodi API with an account's
// token, serving subtitles and season packs in archives.
package titlovi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/archive"
	"github.com/mavioai/mavio/plugins/subtitles/internal/archivemix"
	"github.com/mavioai/mavio/plugins/subtitles/internal/cache"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/scrape"
	"github.com/mavioai/mavio/plugins/subtitles/internal/site"
)

// Name is the provider's, as Bazarr names it.
const Name = "titlovi"

const (
	apiURL = "https://kodi.titlovi.com/api/subtitles"
	// resultsExpiration is how long searches are cached, as Bazarr's
	// SHOW_EXPIRATION_TIME.
	resultsExpiration = 3 * time.Hour
	// maxPages is the last page searched, as Bazarr's.
	maxPages        = 3
	downloadTimeout = 10 * time.Second
	tokenFile       = "token.json"
)

// codes are Titlovi's language names by BCP 47 tag, as Bazarr's
// TitloviConverter, Serbian in Latin script being Serbian's.
var codes = map[string]string{
	"bs": "Bosanski", "en": "English", "hr": "Hrvatski", "mk": "Makedonski", "sr": "Srpski", "sr-Cyrl": "Cirilica",
	"sl": "Slovenski", "sr-Latn": "Srpski",
}

// fromSite returns the tag of a language name of the site; "" when
// unknown.
func fromSite(name string) string {
	switch name {
	case "Srpski":
		return "sr"
	case "Cirilica":
		return "sr-Cyrl"
	}
	for tag, n := range codes {
		if n == name {
			return tag
		}
	}
	return ""
}

// Definition returns the provider's definition, with Bazarr's settings
// and throttling.
func Definition() provider.Definition {
	langs := make([]string, 0, len(codes))
	for tag := range codes {
		langs = append(langs, tag)
	}
	slices.Sort(langs)
	return provider.Definition{
		Name: Name, DisplayName: "Titlovi",
		Settings: []provider.Setting{
			{Name: "username", Label: "Username", Type: provider.Text},
			{Name: "password", Label: "Password", Type: provider.Password},
		},
		Languages: langs,
		Movies:    true, Episodes: true,
		Throttle: map[provider.Kind]func(time.Time) time.Duration{
			provider.TooManyRequests: provider.Fixed(5 * time.Minute),
		},
		New: New,
	}
}

// token is the account's API token, as Bazarr keeps it in its cache.
type token struct {
	Username string    `json:"username"`
	UserID   string    `json:"user_id"`
	Token    string    `json:"token"`
	Expires  time.Time `json:"expires"`
}

// Provider searches Titlovi.
type Provider struct {
	env                provider.Env
	s                  *site.Session
	username, password string
	api                string
	// cache keeps search answers, as Bazarr's region cache.
	cache *cache.Region

	mu    sync.Mutex
	token *token
}

// New makes the provider from its settings; the username and password
// are required, as Bazarr's.
func New(env provider.Env, raw json.RawMessage) (provider.Provider, error) {
	var s struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, provider.Errorf(provider.ConfigurationError, "settings: %w", err)
	}
	if s.Username == "" || s.Password == "" {
		return nil, provider.Errorf(provider.ConfigurationError, "Username and password must be specified")
	}
	hc := env.HTTP
	if hc == nil {
		hc = http.DefaultClient
	}
	sess := site.New(hc)
	sess.Check = func(*site.Response) error { return nil }
	return &Provider{env: env, s: sess, username: s.Username, password: s.Password, api: apiURL, cache: cache.New(env)}, nil
}

// SetBaseURL points the provider at another API, for tests.
func (p *Provider) SetBaseURL(u *url.URL) {
	p.api = strings.TrimRight(u.String(), "/") + "/api/subtitles"
}

// initialize returns the account's token: the cached one until it
// expires, else a new one, as Bazarr's initialize.
func (p *Provider) initialize(ctx context.Context) (*token, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.token == nil {
		p.token = p.loadToken(ctx)
	}
	if p.token != nil && !p.env.Now().After(p.token.Expires) {
		return p.token, nil
	}
	p.env.Log.DebugContext(ctx, "Titlovi token expired or not found")
	if err := p.logIn(ctx); err != nil {
		return nil, err
	}
	return p.token, nil
}

// logIn gets a new token, as Bazarr's log_in: an unauthorized account is
// an authentication error, other failures are logged and leave no token.
func (p *Provider) logIn(ctx context.Context) error {
	q := url.Values{"username": {p.username}, "password": {p.password}, "json": {"True"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.api+"/gettoken?"+q.Encode(), nil)
	if err != nil {
		return fmt.Errorf("gettoken: %w", err)
	}
	r, err := p.s.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		p.env.Log.ErrorContext(ctx, "Titlovi login failed", "error", err)
		return nil
	}
	switch r.Status {
	case http.StatusOK:
	case http.StatusUnauthorized:
		return provider.Errorf(provider.AuthenticationError, "API login returned 403 Forbidden. Account not verified/API disabled.")
	default:
		return nil
	}
	var resp struct {
		Token          string `json:"Token"`
		UserID         any    `json:"UserId"`
		ExpirationDate string `json:"ExpirationDate"`
	}
	if err := r.JSON(&resp); err != nil {
		return err
	}
	expires, err := parseDate(resp.ExpirationDate)
	if err != nil {
		return provider.Errorf(provider.ParseResponseError, "ExpirationDate: %w", err)
	}
	userID := fmt.Sprint(resp.UserID)
	if f, ok := resp.UserID.(float64); ok {
		userID = strconv.FormatInt(int64(f), 10)
	}
	p.token = &token{Username: p.username, UserID: userID, Token: resp.Token, Expires: expires}
	p.saveToken(ctx)
	p.env.Log.DebugContext(ctx, "New Titlovi token obtained")
	return nil
}

// parseDate parses the API's dates, local times as dateutil reads them.
func parseDate(s string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05.999999999", "2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("bad date %q", s)
}

// loadToken reads the token kept under the data folder, which survives
// restarts as Bazarr's cache; nil when there is none for the account.
func (p *Provider) loadToken(ctx context.Context) *token {
	if p.env.DataDir == "" {
		return nil
	}
	data, err := os.ReadFile(filepath.Join(p.env.DataDir, tokenFile))
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			p.env.Log.WarnContext(ctx, "Reading the Titlovi token failed", "error", err)
		}
		return nil
	}
	var t token
	if err := json.Unmarshal(data, &t); err != nil || t.Username != p.username {
		return nil
	}
	return &t
}

func (p *Provider) saveToken(ctx context.Context) {
	if p.env.DataDir == "" {
		return
	}
	data, err := json.Marshal(p.token)
	if err == nil {
		err = os.MkdirAll(p.env.DataDir, 0o700)
	}
	if err == nil {
		err = os.WriteFile(filepath.Join(p.env.DataDir, tokenFile), data, 0o600)
	}
	if err != nil {
		p.env.Log.WarnContext(ctx, "Saving the Titlovi token failed", "error", err)
	}
}

// dropToken forgets a token the API refused, so that the next search
// logs in again.
func (p *Provider) dropToken(ctx context.Context) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.token = nil
	if p.env.DataDir != "" {
		if err := os.Remove(filepath.Join(p.env.DataDir, tokenFile)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			p.env.Log.WarnContext(ctx, "Removing the Titlovi token failed", "error", err)
		}
	}
}

// getResult searches, the answers cached, as Bazarr's get_result.
func (p *Provider) getResult(ctx context.Context, q url.Values) ([]byte, error) {
	key := "search:" + q.Encode()
	if body, ok := cache.Get[[]byte](ctx, p.cache, key); ok {
		return body, nil
	}
	r, err := p.s.Get(ctx, p.api+"/search", q)
	if err != nil {
		return nil, err
	}
	switch r.Status {
	case http.StatusTooManyRequests:
		return nil, provider.Errorf(provider.TooManyRequests, "Too many requests")
	case http.StatusUnauthorized:
		return nil, provider.Errorf(provider.AuthenticationError, "API search returned 403 Forbidden. Account not verified/API disabled.")
	}
	cache.Set(ctx, p.cache, key, r.Body, resultsExpiration)
	return r.Body, nil
}

// sub is a subtitle of the API.
type sub struct {
	ID            any     `json:"Id"`
	Title         *string `json:"Title"`
	Year          *int    `json:"Year"`
	Link          string  `json:"Link"`
	Season        *int    `json:"Season"`
	Episode       *int    `json:"Episode"`
	Lang          string  `json:"Lang"`
	DownloadCount int     `json:"DownloadCount"`
	Release       string  `json:"Release"`
}

// page is a page of a search.
type page struct {
	SubtitleResults []sub `json:"SubtitleResults"`
	PagesAvailable  int   `json:"PagesAvailable"`
}

// search returns the results of up to three pages; failures but too many
// requests end it with the results so far, as Bazarr's query.
func (p *Provider) search(ctx context.Context, q url.Values) ([]sub, error) {
	var out []sub
	pages := 1
	for pg := 1; pg <= min(maxPages, pages); pg++ {
		if pg > 1 {
			q.Set("pg", strconv.Itoa(pg))
		}
		body, err := p.getResult(ctx, q)
		var res page
		if err == nil {
			err = json.Unmarshal(body, &res)
		}
		if err != nil {
			switch {
			case ctx.Err() != nil:
				return nil, ctx.Err()
			case provider.KindOf(err) == provider.TooManyRequests:
				return nil, err
			case provider.KindOf(err) == provider.AuthenticationError:
				// A refused token is forgotten and the refusal returned,
				// where Bazarr logs it.
				p.dropToken(ctx)
				return nil, err
			}
			p.env.Log.ErrorContext(ctx, "Titlovi search failed", "error", err)
			return out, nil
		}
		if len(res.SubtitleResults) == 0 && pg > 1 {
			break
		}
		out = append(out, res.SubtitleResults...)
		if pg == 1 {
			pages = res.PagesAvailable
		}
	}
	return out, nil
}

var (
	// aka splits a title from its alternative, as Bazarr's title_re.
	aka = regexp.MustCompile(`(?i) aka `)
	// renames are the titles Titlovi names differently, as Bazarr's
	// fix_inconsistent_naming.
	renames = strings.NewReplacer("DC's Legends of Tomorrow", "Legends of Tomorrow", "Marvel's Jessica Jones", "Jessica Jones")
)

// fixNaming is Bazarr's fix_inconsistent_naming of Titlovi: renamed and
// sanitized.
func fixNaming(title string) string { return scrape.SanitizePatch(renames.Replace(title), "") }

// splitTitle returns a title and its alternative, split at the last
// " aka ", as Bazarr's title_re.
func splitTitle(title string) (string, string) {
	locs := aka.FindAllStringIndex(title, -1)
	if len(locs) == 0 {
		return title, ""
	}
	last := locs[len(locs)-1]
	if last[0] == 0 || last[1] == len(title) {
		return title, ""
	}
	return title[:last[0]], title[last[1]:]
}

// Search lists the subtitles of a video in the languages, as Bazarr's
// list_subtitles and query.
func (p *Provider) Search(ctx context.Context, v *provider.Video, langs []provider.Language) ([]provider.Subtitle, error) {
	var used []provider.Language
	for _, l := range langs {
		if _, ok := codes[l.Tag]; ok {
			used = append(used, l)
		}
	}
	// Serbian and Serbian in Latin script are the same on Titlovi.
	if slices.ContainsFunc(used, func(l provider.Language) bool { return l.Tag == "sr" }) {
		used = slices.DeleteFunc(used, func(l provider.Language) bool { return l == provider.Language{Tag: "sr-Latn"} })
	}
	if len(used) == 0 {
		return nil, nil
	}
	tok, err := p.initialize(ctx)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(used))
	for _, l := range used {
		names = append(names, codes[l.Tag])
	}
	title := v.Title
	if v.Episode {
		title = v.Series
	}
	episode := v.Episode && v.Season != 0 && v.Number != 0
	q := url.Values{"query": {fixNaming(title)}, "lang": {strings.Join(names, "|")}}
	if episode {
		q.Set("season", strconv.Itoa(v.Season))
	}
	if imdb := v.ExternalIDs["imdb"]; imdb != "" {
		q.Set("imdbID", imdb)
	}
	q.Set("json", "True")
	p.env.Log.InfoContext(ctx, "Searching Titlovi", "query", q.Encode())
	if tok != nil {
		q.Set("token", tok.Token)
		q.Set("userid", tok.UserID)
	}
	results, err := p.search(ctx, q)
	if err != nil {
		return nil, err
	}
	var out []provider.Subtitle
	for _, s := range results {
		lang, ok := language(s.Lang, langs)
		if !ok {
			continue
		}
		if s.Title == nil || *s.Title == "" {
			continue
		}
		name, alt := splitTitle(*s.Title)
		ref := url.Values{"link": {s.Link}, "lang": {fromSite(s.Lang)}}
		if episode {
			if s.Season == nil || *s.Season != v.Season || s.Episode == nil || *s.Episode != v.Number && *s.Episode != 0 {
				continue
			}
			ref.Set("season", strconv.Itoa(*s.Season))
			ref.Set("episode", strconv.Itoa(v.Number))
			if *s.Episode == 0 {
				ref.Set("pack", "1")
			}
		}
		var releases []string
		if s.Release != "" {
			releases = []string{s.Release}
		}
		out = append(out, provider.Subtitle{
			ID: ref.Encode(), Language: lang, ReleaseNames: releases,
			Matches: matches(v, name, alt, s, episode), Downloads: s.DownloadCount,
		})
	}
	return out, nil
}

// language returns the requested language a subtitle of the site is in;
// Serbian answers Serbian in Latin script, as Bazarr's tests expect.
func language(name string, langs []provider.Language) (provider.Language, bool) {
	tag := fromSite(name)
	if tag == "" {
		return provider.Language{}, false
	}
	for _, want := range []string{tag, map[string]string{"sr": "sr-Latn"}[tag]} {
		if want != "" && slices.Contains(langs, provider.Language{Tag: want}) {
			return provider.Language{Tag: want}, true
		}
	}
	return provider.Language{}, false
}

// matches are those of Bazarr's get_matches the site's data tells; the
// release is matched by the server.
func matches(v *provider.Video, title, alt string, s sub, episode bool) []string {
	var m []string
	if episode {
		if v.Series != "" && scrape.SanitizePatch(title, "") == fixNaming(v.Series) || alt != "" && scrape.SanitizePatch(alt, "") == fixNaming(v.Series) {
			m = append(m, "series")
		}
		if v.OriginalSeries && s.Year == nil || v.Year != 0 && s.Year != nil && *s.Year == v.Year {
			m = append(m, "year")
		}
		if v.Season != 0 && s.Season != nil && *s.Season == v.Season {
			m = append(m, "season")
		}
		// The subtitle's episode is the one asked for, as Bazarr's.
		if v.Number != 0 {
			m = append(m, "episode")
		}
		return m
	}
	if v.Title != "" && scrape.SanitizePatch(title, "") == fixNaming(v.Title) || alt != "" && scrape.SanitizePatch(alt, "") == fixNaming(v.Title) {
		m = append(m, "title")
	}
	if v.Year != 0 && s.Year != nil && *s.Year == v.Year {
		m = append(m, "year")
	}
	return m
}

// Download fetches a subtitle's archive and picks its subtitle: a pack's
// episode, Serbian's script, or as the archive mixin, as Bazarr's
// download_subtitle.
func (p *Provider) Download(ctx context.Context, v *provider.Video, id string) (provider.File, error) {
	ref, err := url.ParseQuery(id)
	if err != nil || ref.Get("link") == "" {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "bad subtitle ID %q", id)
	}
	if _, err := p.initialize(ctx); err != nil {
		return provider.File{}, err
	}
	dctx, cancel := context.WithTimeout(ctx, downloadTimeout)
	defer cancel()
	r, err := p.s.Get(dctx, ref.Get("link"), nil)
	if err != nil {
		return provider.File{}, err
	}
	switch r.Status {
	case http.StatusTooManyRequests:
		return provider.File{}, provider.Errorf(provider.TooManyRequests, "Too many requests")
	case http.StatusUnauthorized:
		return provider.File{}, provider.Errorf(provider.AuthenticationError, "API download returned 403 Forbidden. Account not verified/API disabled.")
	}
	if err := site.Status(r); err != nil {
		return provider.File{}, err
	}
	a, err := archive.Open(r.Body)
	if err != nil {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "Unidentified archive type: %w", err)
	}
	season, _ := strconv.Atoi(ref.Get("season"))
	episode, _ := strconv.Atoi(ref.Get("episode"))
	pack, lang := ref.Get("pack") == "1", ref.Get("lang")
	names := a.Names()
	var name string
	switch {
	case len(names) > 1 && pack:
		name = fromPack(names, season, episode)
	case len(names) > 1 && (lang == "sr" || lang == "sr-Cyrl"):
		name = fromBundle(names, lang)
	default:
		w := archivemix.Want{Season: season, Episode: episode, Pack: pack}
		if v != nil && v.Episode {
			w.AskedForEpisode, w.ReleaseGroup = episode, v.ReleaseGroup
		}
		name = archivemix.Pick(names, w)
	}
	if name == "" {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "no subtitle of the video in the archive")
	}
	data, err := a.Read(name)
	if err != nil {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "%s: %w", name, err)
	}
	data = archive.FixLineEndings(data)
	return provider.File{Data: data, Format: archive.Detect(data)}, nil
}

// fromPack returns a pack's file of the episode, named SSxEE or SxxEyy,
// as Bazarr's get_subtitle_from_pack.
func fromPack(names []string, season, episode int) string {
	format1 := fmt.Sprintf("%02dx%02d", season, episode)
	format2 := fmt.Sprintf("s%02de%02d", season, episode)
	for _, name := range names {
		lower := strings.ToLower(name)
		if strings.Contains(lower, format1) || strings.Contains(lower, format2) {
			return name
		}
	}
	return ""
}

// fromBundle returns the file of a Serbian script among the bundled
// ones, as Bazarr's get_subtitle_from_bundled_archive.
func fromBundle(names []string, lang string) string {
	var latin, cyrillic []string
	for _, name := range names {
		lower := strings.ToLower(name)
		if !strings.Contains(lower, ".cyr") && !strings.Contains(lower, ".cir") && !strings.Contains(lower, "cyr)") {
			latin = append(latin, name)
		}
		if (strings.Contains(name, ".cyr") || strings.Contains(lower, ".cir")) && !strings.Contains(lower, ".lat") {
			cyrillic = append(cyrillic, name)
		}
	}
	files := latin
	if lang == "sr-Cyrl" {
		files = cyrillic
	}
	if len(files) == 0 {
		return ""
	}
	return files[0]
}
