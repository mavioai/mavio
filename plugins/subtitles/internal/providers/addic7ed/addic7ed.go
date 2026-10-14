// Package addic7ed is the provider of Addic7ed, ported from Bazarr's
// addic7ed: subtitles of episodes from the season pages of the site's
// shows, and of movies from their pages, with an account logged in with
// a reCAPTCHA or cookies from a browser, and the site's daily download
// limit counted.
package addic7ed

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"

	"github.com/mavioai/mavio/plugins/subtitles/internal/archive"
	"github.com/mavioai/mavio/plugins/subtitles/internal/cache"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/scrape"
	"github.com/mavioai/mavio/plugins/subtitles/internal/site"
	"github.com/mavioai/mavio/plugins/subtitles/internal/sitekit"
)

// Name is the provider's name.
const Name = "addic7ed"

const (
	// showExpiration is how long the show and movie IDs are kept, as
	// Bazarr's SHOW_EXPIRATION_TIME and MOVIE_EXPIRATION_TIME.
	showExpiration = 7 * 24 * time.Hour
	// loginExpiration is how long a login is kept, as Bazarr's
	// load_verification.
	loginExpiration = 180 * 24 * time.Hour
	// loginFile and downloadsFile name the state kept across restarts.
	loginFile     = "login"
	downloadsFile = "downloads"
)

// Definition returns the provider's definition.
func Definition() provider.Definition {
	return provider.Definition{
		Name: Name, DisplayName: "Addic7ed",
		Settings: []provider.Setting{
			{Name: "username", Label: "Username", Help: "Requires a captcha solving service or cookies", Type: provider.Text},
			{Name: "password", Label: "Password", Type: provider.Password},
			{Name: "cookies", Label: "Cookies, e.g., PHPSESSID=abc; wikisubtitlesuser=xyz; wikisubtitlespass=efg", Type: provider.Text},
			{Name: "user_agent", Label: "User-Agent, e.g., Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:95.0) Gecko/20100101 Firefox/95.0", Type: provider.Text},
			{Name: "vip", Label: "VIP", Type: provider.Checkbox, Default: false},
		},
		Languages: []string{
			"ar", "az", "bn", "bs", "bg", "ca", "cs", "da", "de", "el", "en", "eu", "fa", "fi", "fr", "gl", "he", "hr", "hu",
			"hy", "id", "it", "ja", "ko", "mk", "ms", "nl", "no", "pl", "pt", "ro", "ru", "sk", "sl", "es", "sq", "sr", "sv",
			"th", "tr", "uk", "vi", "zh", "pt-BR", "sr-Latn", "sr-Cyrl",
		},
		HearingImpairedVerifiable: true, Movies: true, Episodes: true,
		Throttle: map[provider.Kind]func(time.Time) time.Duration{
			provider.DownloadLimitExceeded: provider.Fixed(3 * time.Hour),
			provider.TooManyRequests:       provider.Fixed(5 * time.Minute),
			provider.IPAddressBlocked:      provider.Fixed(time.Hour),
		},
		New: New,
	}
}

type settings struct {
	Username  string `json:"username"`
	Password  string `json:"password"`
	Cookies   string `json:"cookies"`
	UserAgent string `json:"user_agent"`
	VIP       bool   `json:"vip"`
}

// New makes the provider from its settings.
func New(env provider.Env, raw json.RawMessage) (provider.Provider, error) {
	var s settings
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, provider.Errorf(provider.ConfigurationError, "settings: %w", err)
		}
	}
	switch {
	case (s.Username != "") != (s.Password != ""):
		return nil, provider.Errorf(provider.ConfigurationError, "username and password must be specified")
	case s.Username == "" && s.Cookies == "":
		return nil, provider.Errorf(provider.ConfigurationError, "username and password or cookies must be specified")
	}
	base, _ := url.Parse("https://www.addic7ed.com/")
	return &Provider{env: env, s: s, base: base, pause: time.Second, cache: cache.New(env)}, nil
}

// Provider is Addic7ed.
type Provider struct {
	env  provider.Env
	s    settings
	base *url.URL
	// pause is Bazarr's pause between the login's requests.
	pause time.Duration

	// mu serializes the provider's use of the site, as Bazarr's.
	mu       sync.Mutex
	sess     *site.Session
	loggedIn bool
	// lastFetch is when the show IDs were last fetched.
	lastFetch time.Time
	downloads *downloads
	// cache keeps the site's show IDs and movie IDs, as Bazarr's region
	// cache.
	cache *cache.Region
}

// showList is the site's show IDs by sanitized name and when they were
// fetched.
type showList struct {
	IDs     map[string]int `json:"ids"`
	Fetched time.Time      `json:"fetched"`
}

// SetBaseURL points the provider at another server, for tests.
func (p *Provider) SetBaseURL(u *url.URL) { p.base = u.JoinPath("/") }

func (p *Provider) url(path string) string { return p.base.String() + strings.TrimPrefix(path, "/") }

// check maps the site's answers to failures: 304 Not Modified is how it
// tells too many requests.
func check(r *site.Response) error {
	if r.Status == http.StatusNotModified {
		return provider.Errorf(provider.TooManyRequests, "%s: not modified", r.URL.Path)
	}
	return site.Status(r)
}

// session returns the logged-in session, logging in first, as Bazarr's
// initialize.
func (p *Provider) session(ctx context.Context) (*site.Session, error) {
	if p.sess != nil && p.loggedIn {
		return p.sess, nil
	}
	s := site.New(p.env.HTTP)
	s.Check = check
	s.Header.Set("User-Agent", cmp.Or(p.s.UserAgent, site.RandomUserAgent()))
	s.Header.Set("Referer", p.base.String())
	p.sess, p.loggedIn = s, false
	if p.s.Cookies != "" {
		s.Jar().SetCookies(p.base, sitekit.ParseCookies(p.s.Cookies))
		ok, err := p.panel(ctx)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, provider.Errorf(provider.AuthenticationError, "cookies not valid anymore")
		}
		return s, p.loggedInNow(ctx)
	}
	l, ok, err := sitekit.LoadLogin(p.env.DataDir, loginFile, s.Jar(), p.base, loginExpiration, p.env.Now())
	if err != nil {
		p.env.Log.WarnContext(ctx, "addic7ed: kept login unreadable", "error", err)
	}
	if ok {
		if l.UserAgent != "" {
			s.Header.Set("User-Agent", l.UserAgent)
		}
		valid, err := p.panel(ctx)
		if err != nil {
			return nil, err
		}
		if valid {
			p.env.Log.InfoContext(ctx, "addic7ed: re-using old login")
			p.loggedIn = true
			return s, nil
		}
		p.env.Log.InfoContext(ctx, "addic7ed: login expired")
		if err := sitekit.Remove(p.env.DataDir, loginFile); err != nil {
			return nil, err
		}
	}
	if err := p.login(ctx); err != nil {
		return nil, err
	}
	return s, nil
}

// panel reports whether the session is logged in: the account's panel
// does not redirect to the login.
func (p *Provider) panel(ctx context.Context) (bool, error) {
	r, err := p.sess.Get(ctx, p.url("panel.php"), nil)
	if err != nil {
		return false, err
	}
	return r.URL.Path == p.base.JoinPath("panel.php").Path, nil
}

func (p *Provider) loggedInNow(ctx context.Context) error {
	if err := sitekit.SaveLogin(p.env.DataDir, loginFile, p.sess.Jar(), p.base, p.sess.Header.Get("User-Agent"), p.env.Now()); err != nil {
		p.env.Log.WarnContext(ctx, "addic7ed: login not kept", "error", err)
	}
	p.loggedIn = true
	return site.Sleep(ctx, 2*p.pause)
}

// login logs in with the account, solving the login page's invisible
// reCAPTCHA, as Bazarr's.
func (p *Provider) login(ctx context.Context) error {
	p.env.Log.InfoContext(ctx, "addic7ed: logging in")
	const tries = 3
	for try := 1; ; try++ {
		r, err := p.sess.Get(ctx, p.url("login.php"), nil)
		if err != nil {
			return err
		}
		form := url.Values{
			"username": {p.s.Username}, "password": {p.s.Password}, "Submit": {"Log in"}, "url": {""}, "remember": {"true"},
		}
		if page := string(r.Body); sitekit.HasRecaptcha(page) {
			field, key := sitekit.RecaptchaKey(page)
			if key == "" {
				p.env.Log.ErrorContext(ctx, "addic7ed: captcha site key not found")
				return nil
			}
			solver, err := sitekit.Captcha(p.env, "Addic7ed")
			if err != nil {
				return err
			}
			p.env.Log.InfoContext(ctx, "addic7ed: solving captcha, which might take a couple of minutes")
			token, err := solver.ReCaptchaV2(ctx, p.url("login.php"), key, true)
			if err != nil || token == "" {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				if try >= tries {
					return provider.Errorf(provider.ProviderError, "couldn't solve captcha: %w", err)
				}
				p.env.Log.InfoContext(ctx, "addic7ed: couldn't solve captcha, retrying", "error", err)
				if err := site.Sleep(ctx, 4*p.pause); err != nil {
					return err
				}
				continue
			}
			form.Set(field, token)
		}
		if err := site.Sleep(ctx, p.pause); err != nil {
			return err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.url("dologin.php"), strings.NewReader(form.Encode()))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Referer", p.url("login.php"))
		r, err = p.sess.Do(req)
		if err != nil {
			return err
		}
		// A login redirects; a failed one answers the form's page.
		if r.URL.Path == p.base.JoinPath("dologin.php").Path {
			page := string(r.Body)
			switch {
			case strings.Contains(page, "relax, slow down"):
				return provider.Errorf(provider.TooManyRequests, "login: %s", p.s.Username)
			case strings.Contains(page, "Wrong password"), strings.Contains(page, "doesn't exist"):
				return provider.Errorf(provider.AuthenticationError, "login: %s", p.s.Username)
			}
			if try >= tries {
				return provider.Errorf(provider.AuthenticationError, "something went wrong when logging in: %s", p.s.Username)
			}
			p.env.Log.InfoContext(ctx, "addic7ed: something went wrong when logging in, retrying")
			if err := site.Sleep(ctx, 4*p.pause); err != nil {
				return err
			}
			continue
		}
		p.env.Log.DebugContext(ctx, "addic7ed: logged in")
		return p.loggedInNow(ctx)
	}
}

// reinitialize calls f, and once more after logging in again when it
// fails reaching the site, as Bazarr's reinitialize_on_error.
func (p *Provider) reinitialize(ctx context.Context, f func(*site.Session) error) error {
	for i := 0; ; i++ {
		s, err := p.session(ctx)
		if err == nil {
			err = f(s)
		}
		if err == nil || i > 0 || ctx.Err() != nil {
			return err
		}
		switch provider.KindOf(err) {
		case provider.ServiceUnavailable, provider.Timeout, provider.ProviderError, provider.ForbiddenError:
		default:
			return err
		}
		p.env.Log.DebugContext(ctx, "addic7ed: reinitializing", "error", err)
		p.sess, p.loggedIn = nil, false
	}
}

// Search implements provider.Provider.
func (p *Provider) Search(ctx context.Context, v *provider.Video, langs []provider.Language) ([]provider.Subtitle, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	var out []provider.Subtitle
	err := p.reinitialize(ctx, func(s *site.Session) error {
		var err error
		if v.Episode {
			out, err = p.searchEpisode(ctx, s, v, langs)
		} else {
			out, err = p.searchMovie(ctx, s, v, langs)
		}
		return err
	})
	return out, err
}

func (p *Provider) searchEpisode(ctx context.Context, s *site.Session, v *provider.Video, langs []provider.Language) ([]provider.Subtitle, error) {
	titles := append([]string{v.Series}, v.AlternativeSeries[:min(5, len(v.AlternativeSeries))]...)
	showID := 0
	for _, title := range titles {
		var err error
		if showID, err = p.showID(ctx, s, title, v.Year, false); err != nil {
			return nil, err
		}
		if showID != 0 {
			break
		}
	}
	if showID == 0 {
		p.env.Log.ErrorContext(ctx, "addic7ed: no show id found", "series", v.Series, "year", v.Year)
		return nil, nil
	}
	return p.query(ctx, s, v, langs, showID)
}

// showID returns the site's ID of a show, as Bazarr's get_show_id: by
// the name's variants, with the year first, fetching the shows again
// once a day when one is not found; 0 for none.
func (p *Provider) showID(ctx context.Context, s *site.Session, series string, year int, ignoreCache bool) (int, error) {
	ids, err := p.shows(ctx, s, ignoreCache)
	if err != nil {
		return 0, err
	}
	var names []string
	for _, n := range []string{series, strings.ReplaceAll(series, ".", ""), strings.ReplaceAll(series, "&", "and"), strings.ReplaceAll(series, "and", "&")} {
		if n = scrape.SanitizePatch(n, ""); !slices.Contains(names, n) {
			names = append(names, n)
		}
	}
	for _, name := range names {
		if year != 0 {
			if id := ids[fmt.Sprintf("%s %d", name, year)]; id != 0 {
				return id, nil
			}
		}
		if id := ids[name]; id != 0 {
			return id, nil
		}
		if !ignoreCache && !p.lastFetch.IsZero() && p.lastFetch.Add(24*time.Hour).Before(p.env.Now()) {
			p.env.Log.InfoContext(ctx, "addic7ed: show id not found, fetching the show ids again")
			return p.showID(ctx, s, series, year, true)
		}
	}
	return 0, nil
}

var (
	showCells = regexp.MustCompile(`(?s)<td class="(?:version|vr)">.*?</td>`)
	// seriesYear is a show's name with its year.
	seriesYear = regexp.MustCompile(`^([ \p{L}\p{N}_'.:(),*&!?-]+?)(?: \((\d{4})\))?$`)
)

// shows returns the site's show IDs by sanitized name, as Bazarr's
// _get_show_ids, kept a week.
func (p *Provider) shows(ctx context.Context, s *site.Session, refresh bool) (map[string]int, error) {
	now := p.env.Now()
	if l, ok := cache.Get[showList](ctx, p.cache, "shows"); !refresh && ok && len(l.IDs) > 0 {
		p.lastFetch = l.Fetched
		return l.IDs, nil
	}
	p.env.Log.InfoContext(ctx, "addic7ed: getting show ids")
	p.lastFetch = now
	r, err := s.Get(ctx, p.url("shows.php"), nil)
	if err != nil {
		return nil, err
	}
	body := r.Body
	// The site's markup is bad: only its show cells are parsed.
	if cells := showCells.FindAll(body, -1); len(cells) > 0 {
		body = append(append([]byte("<table><tr>"), slices.Concat(cells...)...), "</tr></table>"...)
	}
	doc, err := site.Parse(body, "text/html; charset=utf-8")
	if err != nil {
		return nil, provider.Errorf(provider.ParseResponseError, "shows: %w", err)
	}
	ids := map[string]int{}
	for _, a := range doc.All(`td > h3 > a[href^="/show/"]`) {
		id, err := strconv.Atoi(a.Attr("href")[len("/show/"):])
		if err != nil {
			continue
		}
		name := scrape.SanitizeChars(a.Text(), "-:()./")
		ids[name] = id
		if m := seriesYear.FindStringSubmatch(name); m != nil && m[2] != "" {
			if _, ok := ids[m[1]]; !ok {
				ids[m[1]] = id
			}
		}
	}
	if len(ids) == 0 {
		return nil, provider.Errorf(provider.ProviderError, "no show IDs found")
	}
	cache.Set(ctx, p.cache, "shows", showList{IDs: ids, Fetched: now}, showExpiration)
	return ids, nil
}

// query lists the subtitles of an episode from its season's page, as
// Bazarr's query.
func (p *Provider) query(ctx context.Context, s *site.Session, v *provider.Video, langs []provider.Language, showID int) ([]provider.Subtitle, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.url("ajax_loadShow.php")+"?"+url.Values{
		"show": {strconv.Itoa(showID)}, "season": {strconv.Itoa(v.Season)}, "langs": {"|"},
	}.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Referer", p.url(fmt.Sprintf("show/%d", showID)))
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	r, err := s.Do(req)
	if err != nil {
		return nil, err
	}
	if len(r.Body) == 0 {
		p.env.Log.ErrorContext(ctx, "addic7ed: no data returned from provider")
		return nil, nil
	}
	doc, err := r.HTML()
	if err != nil {
		return nil, provider.Errorf(provider.ParseResponseError, "season: %w", err)
	}
	var out []provider.Subtitle
	for _, row := range doc.All("tr.epeven") {
		cells := row.All("td")
		if len(cells) < 10 {
			continue
		}
		// Incomplete subtitles tell their progress.
		if strings.Contains(cells[5].Text(), "%") {
			continue
		}
		tag := languageTag(cells[3].Text())
		season, err1 := strconv.Atoi(strings.TrimSpace(cells[0].Text()))
		episode, err2 := strconv.Atoi(strings.TrimSpace(cells[1].Text()))
		page, dl := cells[2].First("a").Attr("href"), cells[9].First("a").Attr("href")
		if tag == "" || err1 != nil || err2 != nil || page == "" || dl == "" || episode != v.Number {
			continue
		}
		lang, ok := sitekit.Match(langs, tag)
		if !ok {
			continue
		}
		lang.HearingImpaired = cells[6].Text() != ""
		title, version := cells[2].Text(), cells[4].Text()
		matches := []string{"series"}
		if v.Season != 0 && season == v.Season {
			matches = append(matches, "season")
		}
		if v.Number != 0 && episode == v.Number {
			matches = append(matches, "episode")
		}
		if v.Year != 0 || v.OriginalSeries {
			matches = append(matches, "year")
		}
		if v.Title != "" && scrape.SanitizePatch(title, "") == scrape.SanitizePatch(v.Title, "") {
			matches = append(matches, "title")
		}
		out = append(out, p.subtitle(lang, page[1:], dl[1:], version, "", matches))
	}
	return out, nil
}

func (p *Provider) subtitle(lang provider.Language, page, dl, version, uploader string, matches []string) provider.Subtitle {
	pageLink := p.url(page)
	return provider.Subtitle{
		ID:           url.Values{"dl": {dl}, "ref": {pageLink}}.Encode(),
		Language:     lang,
		ReleaseNames: []string{strings.ReplaceAll(version, "+", ",")},
		Matches:      matches,
		PageLink:     pageLink,
		Uploader:     uploader,
	}
}

// names are the site's names of languages that are not their English
// names, as Bazarr's addic7ed language converter.
var names = map[string]string{
	"Català": "ca", "Chinese (Simplified)": "zh", "Chinese (Traditional)": "zh", "Euskera": "eu", "Galego": "gl",
	"Greek": "el", "Malay": "ms", "Portuguese (Brazilian)": "pt-BR", "Serbian (Cyrillic)": "sr-Cyrl",
	"Serbian (Latin)": "sr", "Spanish (Latin America)": "es", "Spanish (Spain)": "es", "French (Canadian)": "fr-CA",
}

// languageTag returns the tag of a language the site names; "" when
// unknown.
func languageTag(name string) string {
	name = strings.TrimSpace(name)
	if tag, ok := names[name]; ok {
		return tag
	}
	return sitekit.LanguageByName(name)
}

func (p *Provider) searchMovie(ctx context.Context, s *site.Session, v *provider.Video, langs []provider.Language) ([]provider.Subtitle, error) {
	titles := append([]string{v.Title}, v.AlternativeTitles[:min(5, len(v.AlternativeTitles))]...)
	id, title := "", ""
	for _, t := range titles {
		var err error
		if id, err = p.movieID(ctx, s, t, v.Year); err != nil {
			return nil, err
		}
		if id != "" {
			title = t
			break
		}
	}
	if id == "" {
		p.env.Log.ErrorContext(ctx, "addic7ed: no movie id found", "title", v.Title, "year", v.Year)
		return nil, nil
	}
	return p.queryMovie(ctx, s, v, langs, id, title)
}

var titleYear = regexp.MustCompile(`(.+)\s\((\d{4})\)$`)

// movieID returns the site's ID of a movie of a year, as Bazarr's
// get_movie_id, kept a week; "" for none.
func (p *Provider) movieID(ctx context.Context, s *site.Session, movie string, year int) (string, error) {
	key := fmt.Sprintf("movie:%s|%d", movie, year)
	if id, ok := cache.Get[string](ctx, p.cache, key); ok {
		return id, nil
	}
	p.env.Log.InfoContext(ctx, "addic7ed: getting movie id", "movie", movie)
	r, err := s.Get(ctx, p.url("search.php")+"?search="+url.QueryEscape(movie), nil)
	if err != nil {
		return "", err
	}
	doc, err := r.HTML()
	if err != nil {
		return "", provider.Errorf(provider.ParseResponseError, "search: %w", err)
	}
	id := ""
	for _, row := range doc.First("table.tabel").All("tr") {
		link := row.First("a[href]")
		href := link.Attr("href")
		if !strings.HasPrefix(href, "movie/") {
			continue
		}
		parts := strings.Split(href, "/")
		if len(parts) != 2 {
			continue
		}
		if m := titleYear.FindStringSubmatch(link.Text()); m != nil &&
			scrape.SanitizePatch(strings.ToLower(m[1]), "") == scrape.SanitizePatch(strings.ToLower(movie), "") && m[2] == strconv.Itoa(year) {
			id = parts[1]
		}
	}
	if id == "" {
		p.env.Log.DebugContext(ctx, "addic7ed: cannot find this movie", "movie", movie, "year", year)
	}
	cache.Set(ctx, p.cache, key, id, showExpiration)
	return id, nil
}

// queryMovie lists the subtitles of a movie from its page, as Bazarr's
// query_movie.
func (p *Provider) queryMovie(ctx context.Context, s *site.Session, v *provider.Video, langs []provider.Language, id, title string) ([]provider.Subtitle, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.url("movie/"+id), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Referer", p.base.String())
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	r, err := s.Do(req)
	if err != nil {
		return nil, err
	}
	if len(r.Body) == 0 {
		p.env.Log.ErrorContext(ctx, "addic7ed: no data returned from provider")
		return nil, nil
	}
	doc, err := r.HTML()
	if err != nil {
		return nil, provider.Errorf(provider.ParseResponseError, "movie: %w", err)
	}
	var out []provider.Subtitle
	for _, table := range doc.All(`table[align="center"][border="0"][class="tabel95"][width="100%"]`) {
		if !table.First("td.NewsTitle").Valid() {
			continue
		}
		sub, ok := p.movieSubtitle(v, langs, table, id, title)
		if ok {
			out = append(out, sub)
		}
	}
	return out, nil
}

var versionComma = regexp.MustCompile(`(.+),.+`)

// movieSubtitle reads a subtitle's table of a movie's page, by the
// positions of its rows and cells, text included, as Bazarr's.
func (p *Provider) movieSubtitle(v *provider.Video, langs []provider.Language, table site.Node, id, title string) (provider.Subtitle, bool) {
	// The parser's implicit tbody is not the page's.
	var rows []*html.Node
	for _, c := range contents(table) {
		if c.Type == html.ElementNode && c.Data == "tbody" {
			rows = append(rows, contents(node(c))...)
		} else {
			rows = append(rows, c)
		}
	}
	row1, row2, row3 := at(rows, 1), at(rows, 4), at(rows, 6)
	if row1 == nil || row2 == nil || row3 == nil {
		return provider.Subtitle{}, false
	}
	if status := at(contents(node(row2)), 6); status == nil || strings.Contains(node(status).Text(), "%") {
		return provider.Subtitle{}, false
	}
	langCell := at(contents(node(row2)), 4)
	if langCell == nil {
		return provider.Subtitle{}, false
	}
	tag := languageTag(strings.Trim(node(langCell).Text(), "\n"))
	if tag == "" {
		return provider.Subtitle{}, false
	}
	lang, ok := sitekit.Match(langs, tag)
	if !ok {
		return provider.Subtitle{}, false
	}
	if img := at(contents(node(at(contents(node(row3)), 1))), 1); img != nil {
		lang.HearingImpaired = strings.HasSuffix(node(img).Attr("src"), "hi.jpg")
	}
	// The first word is "Version" in the user's language.
	version := ""
	if n := at(contents(node(at(contents(node(row1)), 1))), 1); n != nil {
		if fields := strings.Fields(render(n)); len(fields) > 1 {
			if m := versionComma.FindStringSubmatch(strings.Join(fields[1:], " ")); m != nil {
				version = m[1]
			}
		}
	}
	cell := contents(node(at(contents(node(row2)), 8)))
	dl := node(at(cell, 3)).Attr("href")
	if dl == "" {
		dl = node(at(cell, 2)).Attr("href")
	}
	if dl == "" {
		return provider.Subtitle{}, false
	}
	uploader := strings.TrimSpace(node(at(contents(node(at(contents(node(row1)), 2))), 8)).Text())
	var matches []string
	if v.Year != 0 {
		matches = append(matches, "year")
	}
	if v.Title != "" && scrape.SanitizePatch(title, "") == scrape.SanitizePatch(v.Title, "") {
		matches = append(matches, "title")
	}
	return p.subtitle(lang, "movie/"+id, dl[1:], version, uploader, matches), true
}

// contents returns a node's children, text included, as BeautifulSoup's
// contents.
func contents(n site.Node) []*html.Node {
	if n.Node == nil {
		return nil
	}
	var out []*html.Node
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		out = append(out, c)
	}
	return out
}

func at(nodes []*html.Node, i int) *html.Node {
	if i < len(nodes) {
		return nodes[i]
	}
	return nil
}

func node(n *html.Node) site.Node { return site.Node{Node: n} }

// render returns a node as str() does: the text of a string, the markup
// of an element.
func render(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var b strings.Builder
	if err := html.Render(&b, n); err != nil {
		return ""
	}
	return b.String()
}

// downloads are the times of the day's downloads, kept across restarts,
// as Bazarr's addic7ed_dls.
type downloads struct {
	Times []time.Time `json:"times"`
}

// Download implements provider.Provider.
func (p *Provider) Download(ctx context.Context, _ *provider.Video, id string) (provider.File, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	q, err := url.ParseQuery(id)
	if err != nil || q.Get("dl") == "" {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "malformed subtitle ID %q", id)
	}
	if p.downloads == nil {
		p.downloads = &downloads{}
		if _, err := sitekit.Load(p.env.DataDir, downloadsFile, p.downloads); err != nil {
			p.env.Log.WarnContext(ctx, "addic7ed: download count unreadable", "error", err)
		}
	}
	last := p.downloads
	now := p.env.Now()
	last.Times = slices.DeleteFunc(last.Times, func(t time.Time) bool { return !t.Add(24 * time.Hour).After(now) })
	limit := 40
	if p.s.VIP {
		limit = 80
	}
	if len(last.Times) >= limit {
		return provider.File{}, provider.Errorf(provider.DownloadLimitExceeded, "downloads per day exceeded (%d)", limit)
	}
	var data []byte
	err = p.reinitialize(ctx, func(s *site.Session) error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.url(q.Get("dl")), nil)
		if err != nil {
			return err
		}
		req.Header.Set("Referer", q.Get("ref"))
		r, err := s.Do(req)
		if err != nil {
			return err
		}
		if len(r.Body) == 0 {
			return provider.Errorf(provider.ProviderError, "unable to download subtitle: no data returned from provider")
		}
		// The site answers a page instead once the limit is reached.
		if r.Header.Get("Content-Type") == "text/html" {
			return provider.Errorf(provider.DownloadLimitExceeded, "download limit exceeded")
		}
		data = r.Body
		return nil
	})
	if err != nil {
		return provider.File{}, err
	}
	last.Times = append(last.Times, p.env.Now())
	if err := sitekit.Save(p.env.DataDir, downloadsFile, last); err != nil {
		p.env.Log.WarnContext(ctx, "addic7ed: download count not kept", "error", err)
	}
	p.env.Log.InfoContext(ctx, "addic7ed: used downloads", "used", len(last.Times), "limit", limit)
	data = archive.FixLineEndings(data)
	return provider.File{Data: data, Format: archive.Detect(data)}, nil
}
