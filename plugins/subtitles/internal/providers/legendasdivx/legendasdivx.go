// Package legendasdivx is the provider of LegendasDivx.pt, ported from
// Bazarr's legendasdivx: Portuguese and Brazilian subtitles of movies and
// episodes, searched with an account the site limits to 150 searches a
// day.
package legendasdivx

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"

	"github.com/mavioai/mavio/plugins/subtitles/internal/archive"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/scrape"
	"github.com/mavioai/mavio/plugins/subtitles/internal/site"
	"github.com/mavioai/mavio/plugins/subtitles/internal/sitekit"
)

// Name is the provider's name.
const Name = "legendasdivx"

const (
	// safeSearchLimit is the searches a day before the site blocks the IP
	// address, as Bazarr's SAFE_SEARCH_LIMIT: 150 less a margin.
	safeSearchLimit = 145
	// maxPages bounds the pages of results read, of 10 results each.
	maxPages = 6
	// cookiesFile names the session kept across restarts, as Bazarr's
	// legendasdivx_cookies2, kept as long as Bazarr's cache region.
	cookiesFile       = "cookies"
	cookiesExpiration = 30 * 24 * time.Hour
	// userAgent is Bazarr's default SZ_USER_AGENT.
	userAgent = "Sub-Zero/2"
)

// storedCookies are the cookies of a login kept, as Bazarr's store_cks.
var storedCookies = []string{"PHPSESSID", "phpbb3_2z8zs_sid", "phpbb3_2z8zs_k", "phpbb3_2z8zs_u", "lang"}

// Definition returns the provider's definition.
func Definition() provider.Definition {
	limit := provider.UntilMidnight("Europe/Lisbon", time.Hour)
	return provider.Definition{
		Name: Name, DisplayName: "LegendasDivx",
		Settings: []provider.Setting{
			{Name: "username", Label: "Username", Help: "Brazilian / Portuguese Subtitles Provider", Type: provider.Text},
			{Name: "password", Label: "Password", Type: provider.Password},
			{Name: "skip_wrong_fps", Label: "Skip Wrong FPS", Type: provider.Checkbox, Default: false},
		},
		Languages: []string{"pt", "pt-BR"},
		Movies:    true, Episodes: true,
		Throttle: map[provider.Kind]func(time.Time) time.Duration{
			provider.TooManyRequests:       provider.Fixed(3 * time.Hour),
			provider.DownloadLimitExceeded: limit,
			provider.IPAddressBlocked:      limit,
			provider.SearchLimitReached:    limit,
		},
		New: New,
	}
}

type settings struct {
	Username     string `json:"username"`
	Password     string `json:"password"`
	SkipWrongFPS bool   `json:"skip_wrong_fps"`
}

// New makes the provider from its settings.
func New(env provider.Env, raw json.RawMessage) (provider.Provider, error) {
	var s settings
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, provider.Errorf(provider.ConfigurationError, "settings: %w", err)
		}
	}
	if s.Username == "" || s.Password == "" {
		return nil, provider.Errorf(provider.ConfigurationError, "username and password must be specified")
	}
	base, _ := url.Parse("https://www.legendasdivx.pt")
	return &Provider{env: env, s: s, base: base, pause: time.Second, links: map[string]string{}}, nil
}

// Provider is LegendasDivx.pt.
type Provider struct {
	env   provider.Env
	s     settings
	base  *url.URL
	pause time.Duration

	mu   sync.Mutex
	sess *site.Session
	// links are the download links of the subtitles found, by ID.
	links map[string]string
}

// SetBaseURL points the provider at another server, for tests.
func (p *Provider) SetBaseURL(u *url.URL) { p.base = u }

// login is the session kept across restarts, with the account it is of:
// Bazarr drops it when the account changes.
type login struct {
	Username string            `json:"username"`
	Cookies  map[string]string `json:"cookies"`
	Saved    time.Time         `json:"saved"`
}

// session returns the session, with the kept cookies or logged in, as
// Bazarr's initialize.
func (p *Provider) session(ctx context.Context) (*site.Session, error) {
	if p.sess != nil {
		return p.sess, nil
	}
	s := site.New(p.env.HTTP)
	// Statuses are checked by hand, as Bazarr's raise_for_status.
	s.Check = func(*site.Response) error { return nil }
	s.Header.Set("User-Agent", userAgent)
	s.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	s.Header.Set("Origin", p.base.String())
	s.Header.Set("Referer", p.base.String())
	var l login
	ok, err := sitekit.Load(p.env.DataDir, cookiesFile, &l)
	if err != nil {
		p.env.Log.WarnContext(ctx, "legendasdivx: reading the kept session", "error", err)
	}
	if ok && l.Username == p.s.Username && len(l.Cookies) > 0 && p.env.Now().Sub(l.Saved) <= cookiesExpiration {
		p.env.Log.DebugContext(ctx, "legendasdivx: re-using the previous session")
		var cookies []*http.Cookie
		for k, v := range l.Cookies {
			cookies = append(cookies, &http.Cookie{Name: k, Value: v, Path: "/"})
		}
		s.Jar().SetCookies(p.base, cookies)
	} else if err := p.login(ctx, s); err != nil {
		return nil, err
	}
	p.sess = s
	return s, nil
}

// login logs in with the login form, keeping the session's cookies, as
// Bazarr's login.
func (p *Provider) login(ctx context.Context, s *site.Session) error {
	p.env.Log.DebugContext(ctx, "legendasdivx: logging in")
	loginPage := p.base.JoinPath("forum", "ucp.php").String() + "?mode=login"
	if err := site.Sleep(ctx, p.pause); err != nil {
		return err
	}
	r, err := s.Get(ctx, loginPage, nil)
	if err != nil {
		return err
	}
	if err := failure(r); err != nil {
		return err
	}
	doc, err := r.HTML()
	if err != nil {
		return provider.Errorf(provider.ServiceUnavailable, "login page: %w", err)
	}
	// The form's hidden fields set the sid.
	form := url.Values{}
	for _, in := range doc.All("input") {
		form.Set(in.Attr("name"), in.Attr("value"))
	}
	form.Set("username", p.s.Username)
	form.Set("password", p.s.Password)
	if err := site.Sleep(ctx, p.pause); err != nil {
		return err
	}
	if r, err = s.PostForm(ctx, loginPage, form); err != nil {
		return err
	}
	if err := failure(r); err != nil {
		return err
	}
	l := login{Username: p.s.Username, Cookies: map[string]string{}, Saved: p.env.Now()}
	for _, c := range s.Jar().Cookies(p.base) {
		for _, name := range storedCookies {
			if c.Name == name {
				l.Cookies[c.Name] = c.Value
			}
		}
	}
	if l.Cookies["PHPSESSID"] == "" {
		return provider.Errorf(provider.AuthenticationError, "couldn't get session ID, check your credentials")
	}
	p.env.Log.DebugContext(ctx, "legendasdivx: logged in")
	if err := sitekit.Save(p.env.DataDir, cookiesFile, l); err != nil {
		p.env.Log.WarnContext(ctx, "legendasdivx: keeping the session", "error", err)
	}
	return nil
}

// failure maps a failed answer, as Bazarr's handling of HTTPError: a
// blocked IP address, or too many requests.
func failure(r *site.Response) error {
	if r.Status < http.StatusBadRequest {
		return nil
	}
	if strings.Contains(strings.ToLower(string(r.Body)), "bloqueado") {
		return provider.Errorf(provider.IPAddressBlocked, "your IP is blocked on this server")
	}
	return provider.Errorf(provider.TooManyRequests, "%s: HTTP error %d", r.URL.Path, r.Status)
}

var (
	searchesCount = regexp.MustCompile(`<!--pesquisas: (\d*)-->`)
	resultsFound  = regexp.MustCompile(`\((.*?) encontradas\)`)
)

// Search implements provider.Provider, as Bazarr's query.
func (p *Provider) Search(ctx context.Context, v *provider.Video, langs []provider.Language) ([]provider.Subtitle, error) {
	var wanted []provider.Language
	for _, tag := range []string{"pt", "pt-BR"} {
		if l, ok := sitekit.Match(langs, tag); ok {
			wanted = append(wanted, l)
		}
	}
	if len(wanted) == 0 {
		return nil, nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	s, err := p.session(ctx)
	if err != nil {
		return nil, err
	}

	dOp, op, langKey := "search", "_jz00", "form_cat"
	var query, season, episode, imdb string
	seriesIMDb := strings.TrimPrefix(v.SeriesExternalIDs["imdb"], "tt")
	switch {
	case !v.Episode:
		query = requote(v.Title)
		if id := v.ExternalIDs["imdb"]; id != "" {
			query = id
		}
	case seriesIMDb != "":
		// Refined by the series' IMDb ID.
		query, langKey, dOp, op = "&faz=pesquisa_episodio", "idioma", "jz_00", ""
	default:
		// Quoted twice, as Bazarr does.
		query = quote(strings.ToLower(fmt.Sprintf("%%22%s%%22%%20S%02dE%02d", v.Series, v.Season, v.Number)))
	}
	if v.Episode {
		season, episode, imdb = strconv.Itoa(v.Season), strconv.Itoa(v.Number), cmp.Or(seriesIMDb, "None")
	}

	var subs []provider.Subtitle
	for _, lang := range wanted {
		// The filters add up, as Bazarr's.
		if lang.Tag == "pt-BR" {
			query += "&" + langKey + "=29"
		} else {
			query += "&" + langKey + "=28"
		}
		searchURL := p.base.String() + "/modules.php?name=Downloads&file=jz&d_op=" + dOp + "&op=" + op + "&query=" + query +
			"&temporada=" + season + "&episodio=" + episode + "&imdb=" + imdb
		doc, found, err := p.search(ctx, s, searchURL, v.Episode)
		if err != nil {
			return nil, err
		}
		if !found {
			continue
		}
		pages := 1
		if bar := doc.First("div.pager_bar"); bar.Valid() {
			if m := resultsFound.FindStringSubmatch(bar.Text()); m != nil {
				n, _ := strconv.Atoi(m[1])
				pages = min(n/10+1, maxPages)
			}
		}
		subs = append(subs, p.parsePage(ctx, v, doc)...)
		for page := 2; page <= pages; page++ {
			if err := site.Sleep(ctx, 2*p.pause); err != nil {
				return nil, err
			}
			r, err := s.Get(ctx, searchURL+"&page="+strconv.Itoa(page), nil)
			if err != nil {
				return nil, err
			}
			next, err := r.HTML()
			if err != nil {
				return nil, provider.Errorf(provider.ParseResponseError, "results page %d: %w", page, err)
			}
			subs = append(subs, p.parsePage(ctx, v, next)...)
		}
	}
	return subs, nil
}

// search reads the first page of results, logging in again when the
// session expired; false when there are none for an episode.
func (p *Provider) search(ctx context.Context, s *site.Session, searchURL string, episode bool) (site.Node, bool, error) {
	get := func() (*site.Response, bool, error) {
		if err := site.Sleep(ctx, p.pause); err != nil {
			return nil, false, err
		}
		r, err := s.Do(p.request(ctx, searchURL))
		if err != nil {
			return nil, false, err
		}
		if err := failure(r); err != nil {
			return nil, false, err
		}
		limited := false
		if m := searchesCount.FindSubmatch(r.Body); m != nil {
			if n, err := strconv.Atoi(string(m[1])); err == nil && n >= safeSearchLimit {
				limited = true
			}
		}
		return r, limited, nil
	}
	r, limited, err := get()
	if err != nil {
		return site.Node{}, false, err
	}
	if strings.Contains(string(r.Body), "A legenda não foi encontrada") {
		p.env.Log.DebugContext(ctx, "legendasdivx: no results", "url", searchURL)
		// Bazarr searches the same URL again for the season's packs; its
		// answer is the same.
		if episode {
			return site.Node{}, false, nil
		}
	}
	// Redirected to the login page: the session expired.
	if path.Base(r.URL.Path) != "modules.php" {
		p.env.Log.DebugContext(ctx, "legendasdivx: logging in again, the session expired")
		if err := sitekit.Remove(p.env.DataDir, cookiesFile); err != nil {
			p.env.Log.WarnContext(ctx, "legendasdivx: dropping the kept session", "error", err)
		}
		if err := p.login(ctx, s); err != nil {
			return site.Node{}, false, err
		}
		if r, limited, err = get(); err != nil {
			return site.Node{}, false, err
		}
	}
	if limited {
		return site.Node{}, false, provider.Errorf(provider.SearchLimitReached, "you've reached the maximum number of searches for the day")
	}
	doc, err := r.HTML()
	if err != nil {
		return site.Node{}, false, provider.Errorf(provider.ParseResponseError, "results: %w", err)
	}
	return doc, true, nil
}

// request is a search's request, from the home page.
func (p *Provider) request(ctx context.Context, u string) *http.Request {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		req, _ = http.NewRequestWithContext(ctx, http.MethodGet, p.base.String(), nil)
	}
	req.Header.Set("Referer", p.base.String()+"/index.php")
	return req
}

// parsePage lists the subtitles of a page of results, as Bazarr's
// _process_page.
func (p *Provider) parsePage(ctx context.Context, v *provider.Video, doc site.Node) []provider.Subtitle {
	var subs []provider.Subtitle
	for _, box := range doc.All("div.sub_box") {
		var (
			hits      int
			lang      provider.Language
			frameRate string
		)
		for _, th := range box.All("th") {
			td := nextTD(th)
			switch th.Text() {
			case "Hits:":
				hits, _ = strconv.Atoi(strings.TrimSpace(td.Text()))
			case "Idioma:":
				src := strings.ToLower(td.First("img").Attr("src"))
				switch {
				case strings.Contains(src, "brazil"):
					lang = provider.Language{Tag: "pt-BR"}
				case strings.Contains(src, "portugal"):
					lang = provider.Language{Tag: "pt"}
				}
			case "Frame Rate:":
				frameRate = td.TrimmedText()
			}
		}
		if lang.Tag == "" {
			continue
		}
		download := box.First("div.sub_footer a.sub_download")
		href := download.Attr("href")
		if !download.Valid() || href == "" {
			p.env.Log.DebugContext(ctx, "legendasdivx: no download link")
			continue
		}
		link := p.base.String() + "/modules.php" + href
		description := box.First("td.td_desc.brd_up").Text()
		uploader := "anonymous"
		if a := box.First("div.sub_header a"); a.Valid() {
			uploader = a.Text()
		}
		id := ""
		if q, err := url.ParseQuery(link); err == nil {
			id = q.Get("lid")
		}
		if id == "" {
			id = fmt.Sprintf("legendasdivx_%s_%s_%s", v.ExternalIDs["imdb"], description, uploader)
		}
		p.links[id] = link
		subs = append(subs, provider.Subtitle{
			ID: id, Language: lang, ReleaseNames: []string{description},
			Matches:  p.matches(ctx, v, description, frameRate),
			PageLink: link, Uploader: uploader, Downloads: hits,
		})
	}
	return subs
}

// nextTD returns the cell following an element in the page, as
// BeautifulSoup's find_next("td").
func nextTD(n site.Node) site.Node {
	for c := next(n.Node); c != nil; c = next(c) {
		if c.Type == html.ElementNode && c.Data == "td" {
			return site.Node{Node: c}
		}
	}
	return site.Node{}
}

// next returns the node following n in document order.
func next(n *html.Node) *html.Node {
	if n.FirstChild != nil {
		return n.FirstChild
	}
	for ; n != nil; n = n.Parent {
		if n.NextSibling != nil {
			return n.NextSibling
		}
	}
	return nil
}

// matches are what a subtitle's description shows of the video, as
// Bazarr's get_matches without guessit's; none when its frame rate is
// another and such subtitles are skipped.
func (p *Provider) matches(ctx context.Context, v *provider.Video, description, frameRate string) []string {
	if fps, err := strconv.ParseFloat(frameRate, 64); err == nil && fps != 0 && v.FrameRate != 0 && !sitekit.FramerateEqual(v.FrameRate, fps) {
		if p.s.SkipWrongFPS {
			p.env.Log.DebugContext(ctx, "legendasdivx: skipping a subtitle of another frame rate", "want", v.FrameRate, "got", frameRate)
			return nil
		}
	}
	desc := scrape.SanitizePatch(description, "")
	set := map[string]bool{}
	name := path.Base(strings.ReplaceAll(v.FileName, `\`, "/"))
	name = strings.TrimSuffix(name, path.Ext(name))
	if f := scrape.SanitizePatch(scrape.SanitizeReleaseGroup(name), ""); v.FileName != "" && strings.Contains(desc, f) {
		set["title"] = true
		if v.Episode {
			set["series"], set["season"], set["episode"] = true, true, true
		}
	}
	if v.Year != 0 && strings.Contains(desc, fmt.Sprintf("%04d", v.Year)) {
		set["year"] = true
	}
	if !v.Episode {
		for _, t := range append([]string{v.Title}, v.AlternativeTitles...) {
			if t != "" && strings.Contains(desc, scrape.SanitizePatch(t, "")) {
				set["title"] = true
			}
		}
	} else {
		if v.Title != "" && strings.Contains(desc, scrape.SanitizePatch(v.Title, "")) {
			set["title"] = true
		}
		for _, s := range append([]string{v.Series}, v.AlternativeSeries...) {
			if s != "" && strings.Contains(desc, scrape.SanitizePatch(s, "")) {
				set["series"] = true
			}
		}
		if v.Season != 0 && strings.Contains(desc, fmt.Sprintf("s%02d", v.Season)) {
			set["season"] = true
		}
		if v.Number != 0 && strings.Contains(desc, fmt.Sprintf("e%02d", v.Number)) {
			set["episode"] = true
		}
		// The search was by the series' IMDb ID: a result is of the episode,
		// or a pack of its season.
		if v.SeriesExternalIDs["imdb"] != "" {
			set["series"], set["series_imdb_id"], set["season"], set["episode"] = true, true, true, true
		}
	}
	if v.ReleaseGroup != "" && strings.Contains(scrape.SanitizeReleaseGroup(description), scrape.SanitizeReleaseGroup(v.ReleaseGroup)) {
		set["release_group"] = true
	}
	var out []string
	for _, m := range []string{"title", "series", "series_imdb_id", "season", "episode", "year", "release_group"} {
		if set[m] {
			out = append(out, m)
		}
	}
	return out
}

// Download implements provider.Provider, as Bazarr's download_subtitle.
func (p *Provider) Download(ctx context.Context, v *provider.Video, id string) (provider.File, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	s, err := p.session(ctx)
	if err != nil {
		return provider.File{}, err
	}
	link, ok := p.links[id]
	if !ok {
		link = p.base.String() + "/modules.php?name=Downloads&d_op=getit&lid=" + url.QueryEscape(id)
	}
	if err := site.Sleep(ctx, p.pause); err != nil {
		return provider.File{}, err
	}
	r, err := s.Get(ctx, link, nil)
	if err != nil {
		return provider.File{}, err
	}
	if err := failure(r); err != nil {
		return provider.File{}, err
	}
	if strings.Contains(strings.ToLower(string(r.Body)), "limite de downloads diário atingido") {
		return provider.File{}, provider.Errorf(provider.DownloadLimitExceeded, "daily download limit reached")
	}
	if !bytes.HasPrefix(r.Body, []byte("PK")) && !bytes.HasPrefix(r.Body, []byte("Rar!")) {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "unsupported compressed format")
	}
	lang := provider.Language{Tag: "pt"}
	return archive.Extract(r.Body, archive.WantFor(v, lang), lang)
}

// quote percent-encodes all but letters, digits, "_.-~" and "/", as
// Python's urllib.parse.quote.
func quote(s string) string { return escape(s, "_.-~/") }

// requote percent-encodes what a URL can't hold, keeping escapes, as
// requests' requote_uri.
func requote(s string) string { return escape(s, "_.-~!#$%&'()*+,/:;=?@[]") }

func escape(s, safe string) string {
	var b strings.Builder
	for i := range len(s) {
		c := s[i]
		if c < 0x80 && (c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.IndexByte(safe, c) >= 0) {
			b.WriteByte(c)
			continue
		}
		fmt.Fprintf(&b, "%%%02X", c)
	}
	return b.String()
}
