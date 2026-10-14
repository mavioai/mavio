// Package titulky is the provider of Titulky.com, ported from Bazarr's
// titulky: Czech and Slovak subtitles of movies and episodes, browsed by
// IMDb ID with a V.I.P. account of the premium site.
package titulky

import (
	"context"
	"encoding/json"
	"errors"
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
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/site"
	"github.com/mavioai/mavio/plugins/subtitles/internal/sitekit"
)

// Name is the provider's name.
const Name = "titulky"

const (
	// loginFile names the cookies kept across restarts, as Bazarr's
	// titulky_cookiejar, kept as long as Bazarr's cache region.
	loginFile       = "cookies"
	loginExpiration = 30 * 24 * time.Hour
	// maxRedirects bounds the redirects followed, as Bazarr's recursion.
	maxRedirects = 10
	// fpsIcon marks the frame rate on a subtitle's page.
	fpsIcon = "img/ico/Movieroll.png"
)

// Definition returns the provider's definition.
func Definition() provider.Definition {
	return provider.Definition{
		Name: Name, DisplayName: "Titulky.com",
		Settings: []provider.Setting{
			{Name: "username", Label: "Username", Help: "CZ/SK Subtitles Provider. Available only with VIP.", Type: provider.Text},
			{Name: "password", Label: "Password", Type: provider.Password},
			{Name: "approved_only", Label: "Skip unapproved subtitles", Type: provider.Checkbox, Default: false},
			{Name: "skip_wrong_fps", Label: "Skip subtitles with mismatched fps to video's", Type: provider.Checkbox, Default: false},
		},
		Languages: []string{"cs", "sk"},
		Movies:    true, Episodes: true,
		Throttle: map[provider.Kind]func(time.Time) time.Duration{
			provider.TooManyRequests:       provider.Fixed(time.Minute),
			provider.DownloadLimitExceeded: provider.UntilMidnight("Europe/Prague", 0),
		},
		New: New,
	}
}

type settings struct {
	Username     string `json:"username"`
	Password     string `json:"password"`
	ApprovedOnly bool   `json:"approved_only"`
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
	base, _ := url.Parse("https://premium.titulky.com")
	return &Provider{env: env, s: s, base: base, fps: map[string]float64{}, pages: map[string]string{}}, nil
}

// Provider is Titulky.com.
type Provider struct {
	env  provider.Env
	s    settings
	base *url.URL

	mu   sync.Mutex
	sess *site.Session
	// fps are the subtitles' frame rates, 0 for unknown, by ID.
	fps map[string]float64
	// pages are the subtitles' pages, by ID.
	pages map[string]string
}

// SetBaseURL points the provider at another server, for tests.
func (p *Provider) SetBaseURL(u *url.URL) { p.base = u }

// server is the site's address, without a trailing slash.
func (p *Provider) server() string { return strings.TrimRight(p.base.String(), "/") }

// session returns the session, with the kept cookies or logged in, as
// Bazarr's initialize. Redirects are followed by hand, as Bazarr's.
func (p *Provider) session(ctx context.Context) (*site.Session, error) {
	if p.sess != nil {
		return p.sess, nil
	}
	hc := *p.env.HTTP
	hc.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	s := site.NewBrowser(&hc)
	s.Check = func(*site.Response) error { return nil }
	s.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	s.Header.Set("Accept-Language", "sk,cz,en;q=0.5")
	s.Header.Set("DNT", "1")
	s.Header.Set("Upgrade-Insecure-Requests", "1")
	s.Header.Set("Cache-Control", "max-age=0")
	if _, ok, err := sitekit.LoadLogin(p.env.DataDir, loginFile, s.Jar(), p.base, loginExpiration, p.env.Now()); err != nil {
		p.env.Log.WarnContext(ctx, "titulky: reading the kept cookies", "error", err)
	} else if ok {
		p.env.Log.DebugContext(ctx, "titulky: reusing the kept cookies")
		p.sess = s
		return s, nil
	}
	if err := p.login(ctx, s); err != nil {
		return nil, err
	}
	p.sess = s
	return s, nil
}

// login logs in and keeps the cookies, as Bazarr's login.
func (p *Provider) login(ctx context.Context, s *site.Session) error {
	p.env.Log.DebugContext(ctx, "titulky: logging in")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.server(), strings.NewReader(url.Values{
		"LoginName": {p.s.Username}, "LoginPassword": {p.s.Password},
	}.Encode()))
	if err != nil {
		return fmt.Errorf("login: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", p.server())
	r, err := s.Do(req)
	if err != nil {
		return err
	}
	q := locationQuery(r)
	if r.Status != http.StatusFound || strings.ToLower(q.Get("msg_type")) != "i" {
		return provider.Errorf(provider.AuthenticationError, "login failed")
	}
	if strings.Contains(strings.ToLower(q.Get("msg")), "omezené") {
		return provider.Errorf(provider.AuthenticationError, "a V.I.P. account is required for this provider to work")
	}
	p.env.Log.DebugContext(ctx, "titulky: logged in, keeping the cookies")
	if err := sitekit.SaveLogin(p.env.DataDir, loginFile, s.Jar(), p.base, "", p.env.Now()); err != nil {
		p.env.Log.WarnContext(ctx, "titulky: keeping the cookies", "error", err)
	}
	return nil
}

// locationQuery returns the query of a redirect's location.
func locationQuery(r *site.Response) url.Values {
	u, err := url.Parse(r.Header.Get("Location"))
	if err != nil {
		return url.Values{}
	}
	return u.Query()
}

func isRedirect(r *site.Response) bool {
	return r.Header.Get("Location") != "" && slices.Contains([]int{301, 302, 303, 307, 308}, r.Status)
}

// get fetches a page, following redirects unless they are to an error
// message: one asking to log in logs in again, as Bazarr's get_request.
func (p *Provider) get(ctx context.Context, u, ref string, follow bool) (*site.Response, error) {
	s, err := p.session(ctx)
	if err != nil {
		return nil, err
	}
	for range maxRedirects {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", u, err)
		}
		if ref != "" {
			req.Header.Set("Referer", quote(ref))
		}
		r, err := s.Do(req)
		if err != nil {
			return nil, err
		}
		if !isRedirect(r) || !follow {
			return r, nil
		}
		q := locationQuery(r)
		if strings.ToLower(q.Get("msg_type")) != "e" {
			base := p.base
			if o, err := url.Parse(r.Header.Get("Origin")); err == nil && r.Header.Get("Origin") != "" {
				base = o
			}
			next, err := base.Parse(r.Header.Get("Location"))
			if err != nil {
				return nil, provider.Errorf(provider.ProviderError, "redirect from %s: %w", u, err)
			}
			u, ref = next.String(), u
			continue
		}
		msg := strings.ToLower(q.Get("msg"))
		switch {
		case strings.Contains(msg, "přihlašte"):
			p.env.Log.DebugContext(ctx, "titulky: the login cookies expired")
			if err := p.login(ctx, s); err != nil {
				return nil, err
			}
		case strings.Contains(msg, "omezené"):
			return nil, provider.Errorf(provider.AuthenticationError, "the V.I.P. status expired")
		default:
			return nil, provider.Errorf(provider.ProviderError, "redirected from %s to an error page: %q", u, q.Get("msg"))
		}
	}
	return nil, provider.Errorf(provider.AuthenticationError, "got into a redirect loop")
}

// page fetches a page's document, as Bazarr's fetch_page.
func (p *Provider) page(ctx context.Context, u string) (site.Node, error) {
	r, err := p.get(ctx, u, p.server(), true)
	if err != nil {
		return site.Node{}, err
	}
	if r.Status == http.StatusTooManyRequests {
		return site.Node{}, provider.Errorf(provider.TooManyRequests, "too many requests")
	}
	if r.Status != http.StatusOK {
		if err := site.Status(r); err != nil {
			return site.Node{}, err
		}
		return site.Node{}, provider.Errorf(provider.ProviderError, "%s: status code %d", u, r.Status)
	}
	if len(r.Body) == 0 {
		return site.Node{}, provider.Errorf(provider.ProviderError, "no response returned")
	}
	doc, err := r.HTML()
	if err != nil {
		return site.Node{}, provider.Errorf(provider.ParseResponseError, "%s: %w", u, err)
	}
	return doc, nil
}

// pageURL is a page of the site, as Bazarr's build_url.
func (p *Provider) pageURL(params ...string) string {
	var parts []string
	for i := 0; i+1 < len(params); i += 2 {
		parts = append(parts, params[i]+"="+params[i+1])
	}
	return strings.ReplaceAll(p.server()+"/?"+strings.Join(parts, "&"), " ", "+")
}

// frameRate returns a subtitle's frame rate from its page, 0 when unknown,
// kept, as Bazarr's retrieve_subtitles_fps.
func (p *Provider) frameRate(ctx context.Context, id string) (float64, error) {
	if fps, ok := p.fps[id]; ok {
		return fps, nil
	}
	doc, err := p.page(ctx, p.pageURL("action", "detail", "id", id))
	if err != nil {
		return 0, err
	}
	fps := 0.0
	for _, div := range doc.All("div.ulozil") {
		if !hasChildIcon(div) {
			continue
		}
		fields := strings.Fields(strippedText(div.Node))
		if len(fields) >= 2 && strings.EqualFold(fields[1], "fps") {
			fps, _ = strconv.ParseFloat(strings.ReplaceAll(fields[0], ",", "."), 64)
		}
		break
	}
	p.fps[id] = fps
	return fps, nil
}

func hasChildIcon(n site.Node) bool {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "img" && (site.Node{Node: c}).Attr("src") == fpsIcon {
			return true
		}
	}
	return false
}

// strippedText is an element's text, its pieces stripped and joined, as
// BeautifulSoup's get_text(strip=True).
func strippedText(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(strings.TrimSpace(n.Data))
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}

// child returns an element's i-th child node, text included, as
// BeautifulSoup's contents[i].
func child(n site.Node, i int) *html.Node {
	c := n.FirstChild
	for ; c != nil && i > 0; i-- {
		c = c.NextSibling
	}
	return c
}

var idRe = regexp.MustCompile(`id=(\d+)`)

// found is a subtitle of the browsed page.
type found struct {
	id, release, uploader, details string
	lang                           provider.Language
	approved                       bool
	fps                            float64
}

// Search implements provider.Provider, as Bazarr's list_subtitles: by the
// IMDb ID of the movie or the series.
func (p *Provider) Search(ctx context.Context, v *provider.Video, langs []provider.Language) ([]provider.Subtitle, error) {
	imdb, season, episode := v.ExternalIDs["imdb"], 0, 0
	if v.Episode {
		imdb, season, episode = v.SeriesExternalIDs["imdb"], v.Season, v.Number
	}
	if len(imdb) < 3 {
		p.env.Log.DebugContext(ctx, "titulky: skipping a video without an IMDb ID")
		return nil, nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	subs, err := p.query(ctx, langs, imdb, season, episode)
	if err != nil {
		return nil, err
	}
	var out []provider.Subtitle
	for _, f := range subs {
		p.pages[f.id] = f.details
		s := provider.Subtitle{
			ID: f.id, Language: f.lang, Matches: p.matches(v, season, episode, f.fps),
			PageLink: f.details, Uploader: f.uploader,
		}
		if f.release != "" {
			s.ReleaseNames = []string{f.release}
		}
		out = append(out, s)
	}
	return out, nil
}

// query lists the subtitles of an episode, or a movie as the episode 0 of
// the season 0, from the page of the season, as Bazarr's query.
func (p *Provider) query(ctx context.Context, langs []provider.Language, imdb string, season, episode int) ([]found, error) {
	doc, err := p.page(ctx, p.pageURL("action", "serial", "step", strconv.Itoa(season), "id", imdb[2:]))
	if err != nil {
		return nil, err
	}
	form := doc.First("form.cloudForm")
	if !form.Valid() {
		p.env.Log.DebugContext(ctx, "titulky: no subtitles found")
		return nil, nil
	}
	episodes := map[int][]found{}
	last := -1
	for _, row := range form.All("div.row") {
		classes := strings.Fields(row.Attr("class"))
		approved := slices.Contains(classes, "pbl1")
		listed := approved || slices.Contains(classes, "pbl0")
		if h5 := row.First("h5"); h5.Valid() {
			num := strings.TrimRight(strings.TrimSpace(h5.Text()), ".")
			last = 0
			if num != "" {
				if last, err = strconv.Atoi(num); err != nil {
					return nil, provider.Errorf(provider.ProviderError, "could not parse the episode number %q", num)
				}
			}
			continue
		}
		a := row.First("a")
		if !listed || !a.Valid() {
			continue
		}
		if last < 0 {
			return nil, provider.Errorf(provider.ProviderError, "previous episode number missing")
		}
		release := strippedText(a.Node)
		if release == "???" {
			release = ""
		}
		href := a.Attr("href")
		details := p.server() + href[min(1, len(href)):]
		id := ""
		if m := idRe.FindStringSubmatch(details); m != nil {
			id = m[1]
		}
		uploader := ""
		if c := child(row, 5); c != nil {
			uploader = strippedText(c)
		}
		cz, sk := row.First(`img[src*="flag-CZ"]`).Valid(), row.First(`img[src*="flag-SK"]`).Valid()
		tag := ""
		switch {
		case cz && !sk:
			tag = "cs"
		case sk && !cz:
			tag = "sk"
		default:
			p.env.Log.DebugContext(ctx, "titulky: unknown language")
			continue
		}
		lang, ok := sitekit.Match(langs, tag)
		if !ok {
			continue
		}
		if p.s.ApprovedOnly && !approved {
			continue
		}
		f := found{id: id, release: release, uploader: uploader, details: details, lang: lang, approved: approved}
		if p.s.SkipWrongFPS {
			if f.fps, err = p.frameRate(ctx, id); err != nil {
				return nil, err
			}
		}
		episodes[last] = append(episodes[last], f)
	}
	return episodes[episode], nil
}

// matches are what the browsed page says of the video's subtitle, as
// Bazarr's get_matches without guessit's; none when its frame rate is
// another and such subtitles are skipped.
func (p *Provider) matches(v *provider.Video, season, episode int, fps float64) []string {
	if p.s.SkipWrongFPS && v.FrameRate != 0 && fps != 0 && !sitekit.FramerateEqual(v.FrameRate, fps) {
		return nil
	}
	if !v.Episode {
		// The page is the movie's.
		return []string{"imdb_id", "title", "year"}
	}
	// The page is the series' season's, and its subtitles the episode's.
	out := []string{"series_imdb_id", "series", "year"}
	if season != 0 {
		out = append(out, "season")
	}
	if episode != 0 {
		out = append(out, "episode")
	}
	return out
}

// Download implements provider.Provider, as Bazarr's download_subtitle:
// an archive of a single file without a subtitle is the day's downloads
// exceeded.
func (p *Provider) Download(ctx context.Context, v *provider.Video, id string) (provider.File, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	ref, ok := p.pages[id]
	if !ok {
		ref = p.pageURL("action", "detail", "id", id)
	}
	r, err := p.get(ctx, p.server()+"/download.php?id="+url.QueryEscape(id), ref, false)
	if err != nil {
		return provider.File{}, err
	}
	if r.Status == http.StatusTooManyRequests {
		return provider.File{}, provider.Errorf(provider.TooManyRequests, "too many requests")
	}
	if r.Status >= http.StatusBadRequest {
		if err := site.Status(r); err != nil {
			return provider.File{}, err
		}
	}
	a, err := archive.Open(r.Body)
	switch {
	case errors.Is(err, archive.ErrNotArchive):
		return provider.File{}, provider.Errorf(provider.ProviderError, "no subtitles provided")
	case err != nil:
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "%w", err)
	}
	lang := provider.Language{}
	name, data, err := archive.Subtitle(a, archive.WantFor(v, lang))
	if err != nil {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "%w", err)
	}
	if name == "" {
		if len(a.Names()) == 1 {
			return provider.File{}, provider.Errorf(provider.DownloadLimitExceeded, "the subtitles download limit has been exceeded")
		}
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "no proper subtitle file in the archive")
	}
	format := archive.Detect(data)
	return provider.File{Data: data, Format: format}, nil
}

// quote percent-encodes all but letters, digits, "_.-~" and "/", as
// Python's urllib.parse.quote, which Bazarr sends the Referer through.
func quote(s string) string {
	var b strings.Builder
	for i := range len(s) {
		c := s[i]
		if c < 0x80 && (c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.IndexByte("_.-~/", c) >= 0) {
			b.WriteByte(c)
			continue
		}
		fmt.Fprintf(&b, "%%%02X", c)
	}
	return b.String()
}
