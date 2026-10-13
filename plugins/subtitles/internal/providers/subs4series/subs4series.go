// Package subs4series is the provider of Subs4Series, ported from
// Bazarr's subs4series: Greek and English subtitles of episodes, whose
// downloads ask for a reCAPTCHA solved with the captcha solving service.
package subs4series

import (
	"context"
	"encoding/json"
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

	"github.com/andybalholm/cascadia"
	"golang.org/x/net/html"
	"golang.org/x/text/encoding/charmap"

	"github.com/mavioai/mavio/plugins/subtitles/internal/archive"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/site"
	"github.com/mavioai/mavio/plugins/subtitles/internal/sitekit"
)

// Name is the provider's name.
const Name = "subs4series"

const (
	// showExpiration is how long the shows found are kept, as Bazarr's
	// SHOW_EXPIRATION_TIME.
	showExpiration = 7 * 24 * time.Hour
	// userAgent is a desktop browser's, as Bazarr's: mobile browsers are
	// sent to the mobile site.
	userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/83.0.4103.116 Safari/537.36"
	// captchaTries bounds the captchas solved for a download.
	captchaTries = 3
)

// Definition returns the provider's definition.
func Definition() provider.Definition {
	return provider.Definition{
		Name: Name, DisplayName: "Subs4Series",
		Languages: []string{"el", "en"},
		Episodes:  true,
		New:       New,
	}
}

// New makes the provider; it has no settings.
func New(env provider.Env, _ json.RawMessage) (provider.Provider, error) {
	base, _ := url.Parse("https://www.subs4series.com")
	s := site.New(env.HTTP)
	s.Header.Set("User-Agent", userAgent)
	// Statuses are checked where Bazarr does.
	s.Check = func(*site.Response) error { return nil }
	return &Provider{env: env, base: base, sess: s, shows: map[string]shows{}}, nil
}

// Provider is Subs4Series.
type Provider struct {
	env  provider.Env
	base *url.URL
	sess *site.Session

	mu    sync.Mutex
	shows map[string]shows
}

type shows struct {
	list  []suggestion
	until time.Time
}

type suggestion struct{ link, title string }

// SetBaseURL points the provider at another server, for tests.
func (p *Provider) SetBaseURL(u *url.URL) { p.base = u }

var (
	yearRe = regexp.MustCompile(`^\((\d{4})\)$`)
	rows   = cascadia.MustCompile("table .seeDark,.seeMedium")
)

// Search implements provider.Provider, as Bazarr's list_subtitles.
func (p *Provider) Search(ctx context.Context, v *provider.Video, langs []provider.Language) ([]provider.Subtitle, error) {
	if !v.Episode {
		return nil, nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	var links []string
	for _, title := range append([]string{v.Series}, v.AlternativeSeries...) {
		var err error
		if links, err = p.showLinks(ctx, title, v.Year); err != nil {
			return nil, err
		}
		if len(links) > 0 {
			break
		}
	}
	var subs []provider.Subtitle
	for _, link := range links {
		found, err := p.query(ctx, v, link, langs)
		if err != nil {
			return nil, err
		}
		subs = append(subs, found...)
	}
	return subs, nil
}

// showLinks returns the links of the shows of a title and year, as
// Bazarr's get_show_links.
func (p *Provider) showLinks(ctx context.Context, title string, year int) ([]string, error) {
	title = sitekit.Sanitize(title)
	suggestions, err := p.suggestions(ctx, title)
	if err != nil {
		return nil, err
	}
	var links []string
	for _, s := range suggestions {
		t := sitekit.Sanitize(s.title)
		if t == title || year != 0 && t == fmt.Sprintf("%s %d", title, year) {
			parts := strings.Split(s.link, "/")
			links = append(links, strings.Join(parts[max(len(parts)-2, 0):], "/"))
		}
	}
	return links, nil
}

// suggestions searches the shows of a title, kept a week when some are
// found, as Bazarr's _get_suggestions.
func (p *Provider) suggestions(ctx context.Context, title string) ([]suggestion, error) {
	now := p.env.Now()
	if c, ok := p.shows[title]; ok && now.Before(c.until) {
		return c.list, nil
	}
	u := p.base.String() + "/search_report.php?search=" + strings.ReplaceAll(url.QueryEscape(title), "+", "%20") + "&searchType=1"
	r, err := p.get(ctx, u, p.base.String())
	if err != nil {
		return nil, err
	}
	if err := site.Status(r); err != nil {
		return nil, err
	}
	if len(r.Body) == 0 {
		return nil, nil
	}
	doc, err := r.HTML()
	if err != nil {
		return nil, provider.Errorf(provider.ParseResponseError, "suggestions: %w", err)
	}
	var list []suggestion
	for _, o := range doc.All(`select[name="Mov_sel"] > option[value]`) {
		list = append(list, suggestion{link: o.Attr("value"), title: o.Text()})
	}
	if len(list) > 0 {
		p.shows[title] = shows{list: list, until: now.Add(showExpiration)}
	}
	return list, nil
}

func (p *Provider) get(ctx context.Context, u, referer string) (*site.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", u, err)
	}
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	return p.sess.Do(req)
}

// query lists the subtitles of an episode of a show, as Bazarr's query.
func (p *Provider) query(ctx context.Context, v *provider.Video, showID string, langs []provider.Language) ([]provider.Subtitle, error) {
	if showID == "" || v.Season == 0 || v.Number == 0 {
		return nil, nil
	}
	page := fmt.Sprintf("%s/tv-series/%s/season-%d/episode-%d", p.base, showID, v.Season, v.Number)
	r, err := p.get(ctx, page, "")
	if err != nil {
		return nil, err
	}
	if err := site.Status(r); err != nil {
		return nil, err
	}
	if len(r.Body) == 0 {
		return nil, nil
	}
	doc, err := r.HTML()
	if err != nil {
		return nil, provider.Errorf(provider.ParseResponseError, "episode: %w", err)
	}
	header := doc.First("#dates_header_br > table div")
	if !header.Valid() {
		return nil, provider.Errorf(provider.ParseResponseError, "%s: no show header", page)
	}
	year := 0
	if m := yearRe.FindStringSubmatch(strings.TrimSpace(content(header, 2))); m != nil {
		year, _ = strconv.Atoi(m[1])
	}
	show := header.First("u").TrimmedText()

	var subs []provider.Subtitle
	for _, n := range cascadia.QueryAll(doc.Node, rows) {
		row := site.Node{Node: n}
		bs, a, img := row.All("b"), row.First("a"), row.First("img")
		if len(bs) < 2 || !a.Valid() || !img.Valid() {
			continue
		}
		src := img.Attr("src")
		code := src[strings.LastIndex(src, "/")+1:]
		code, _, _ = strings.Cut(code, ".")
		lang, ok := sitekit.Match(langs, code)
		if !ok {
			continue
		}
		version := bs[0].Text()
		subs = append(subs, provider.Subtitle{
			ID: p.base.String() + a.Attr("href"), Language: lang,
			ReleaseNames: []string{version},
			Matches:      matches(v, show, year, version),
			PageLink:     page, Uploader: bs[1].Text(),
		})
	}
	return subs, nil
}

// content returns the markup or text of an element's i-th child, as
// str() of BeautifulSoup's contents[i].
func content(n site.Node, i int) string {
	c := n.FirstChild
	for ; c != nil && i > 0; i-- {
		c = c.NextSibling
	}
	if c == nil {
		return ""
	}
	if c.Type == html.TextNode {
		return c.Data
	}
	var b strings.Builder
	if err := html.Render(&b, c); err != nil {
		return ""
	}
	return b.String()
}

// matches are what the site says of the episode's subtitle, as Bazarr's
// get_matches without guessit's.
func matches(v *provider.Video, show string, year int, version string) []string {
	var out []string
	if slices.ContainsFunc(append([]string{v.Series}, v.AlternativeSeries...), func(s string) bool {
		return sitekit.Sanitize(s) == sitekit.Sanitize(show)
	}) {
		out = append(out, "series")
	}
	if v.OriginalSeries && year == 0 || v.Year != 0 && v.Year == year {
		out = append(out, "year")
	}
	if v.ReleaseGroup != "" && version != "" && slices.ContainsFunc(sitekit.EquivalentReleaseGroups(sitekit.SanitizeReleaseGroup(v.ReleaseGroup)), func(g string) bool {
		return strings.Contains(sitekit.SanitizeReleaseGroup(version), g)
	}) {
		out = append(out, "release_group")
	}
	return out
}

// Download implements provider.Provider, as Bazarr's download_subtitle:
// the download page's reCAPTCHA is solved, the site's anti-block pages
// visited, and the subtitle posted for.
func (p *Provider) Download(ctx context.Context, _ *provider.Video, id string) (provider.File, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	form := url.Values{"my_recaptcha_challenge_field": {"manual_challenge"}}
	var r *site.Response
	for try := 1; ; try++ {
		var err error
		if r, err = p.get(ctx, id, ""); err != nil {
			return provider.File{}, err
		}
		page := string(r.Body)
		if !sitekit.HasRecaptcha(page) {
			break
		}
		_, key := sitekit.RecaptchaKey(page)
		if key == "" {
			return provider.File{}, provider.Errorf(provider.ProviderError, "captcha site key not found")
		}
		solver, err := sitekit.Captcha(p.env, Name)
		if err != nil {
			return provider.File{}, err
		}
		p.env.Log.InfoContext(ctx, "subs4series: solving a captcha")
		token, err := solver.ReCaptchaV2(ctx, id, key, true)
		if err == nil && token != "" {
			form.Set("g-recaptcha-response", token)
			break
		}
		if try >= captchaTries {
			return provider.File{}, fmt.Errorf("subs4series: couldn't solve the captcha: %w", err)
		}
		p.env.Log.InfoContext(ctx, "subs4series: couldn't solve the captcha, retrying", "error", err)
	}
	doc, err := r.HTML()
	if err != nil {
		return provider.File{}, provider.Errorf(provider.ParseResponseError, "download page: %w", err)
	}
	target := doc.First("a.style55ws").Attr("href")
	if target == "" {
		target = doc.First(`form[method="post"]`).Attr("action")
	}
	if target == "" {
		return provider.File{}, provider.Errorf(provider.ProviderError, "no download link found")
	}
	// The anti-block pages, as Bazarr's apply_anti_block.
	for _, u := range []string{"/includes/anti-block-layover.php?launch=1", "/includes/anti-block.php"} {
		if _, err := p.get(ctx, p.base.String()+u, id); err != nil {
			return provider.File{}, err
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.base.String()+target, strings.NewReader(form.Encode()))
	if err != nil {
		return provider.File{}, fmt.Errorf("%s: %w", target, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", id)
	if r, err = p.sess.Do(req); err != nil {
		return provider.File{}, err
	}
	if r.Status == http.StatusForbidden {
		return provider.File{}, provider.Errorf(provider.ProviderError, "the captcha expired waiting to be solved")
	}
	if len(r.Body) == 0 {
		return provider.File{}, provider.Errorf(provider.ProviderError, "no data returned")
	}
	data, err := extract(r.Body)
	if err != nil {
		return provider.File{}, err
	}
	data = sitekit.Decode(archive.FixLineEndings(data), charmap.Windows1253)
	return provider.File{Data: data, Format: archive.Detect(data)}, nil
}

// subtitleExtensions are subliminal's SUBTITLE_EXTENSIONS.
var subtitleExtensions = []string{".srt", ".sub", ".smi", ".txt", ".ssa", ".ass", ".mpl"}

// extract returns the first subtitle of an archive, or the download when
// it is a subtitle, as Bazarr's _get_subtitle_from_archive.
func extract(data []byte) ([]byte, error) {
	a, err := archive.Open(data)
	if err != nil {
		return nil, provider.Errorf(provider.BadSubtitle, "%w", err)
	}
	for _, name := range a.Names() {
		base := path.Base(name)
		if strings.HasPrefix(base, ".") || !slices.Contains(subtitleExtensions, strings.ToLower(path.Ext(base))) {
			continue
		}
		sub, err := a.Read(name)
		if err != nil {
			return nil, provider.Errorf(provider.BadSubtitle, "%s: %w", name, err)
		}
		return sub, nil
	}
	return nil, provider.Errorf(provider.BadSubtitle, "no subtitle in the archive")
}
