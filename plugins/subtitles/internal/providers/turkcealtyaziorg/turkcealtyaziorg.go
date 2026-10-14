// Package turkcealtyaziorg is the provider of Turkcealtyazi.org, ported
// from Bazarr's turkcealtyaziorg: Turkish and English subtitles of movies
// and episodes, found by IMDb ID; from outside of Turkey the site needs a
// browser's cookies and User-Agent.
package turkcealtyaziorg

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/mavioai/mavio/plugins/subtitles/internal/archive"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/site"
	"github.com/mavioai/mavio/plugins/subtitles/internal/sitekit"
)

// Name is the provider's name.
const Name = "turkcealtyaziorg"

// identifiers are the site's classes of rips and languages, as Bazarr's
// custom_identifiers.
var identifiers = map[string]string{
	"cps c1": "DVDRip", "cps c2": "HDRip", "cps c3": "TVRip",
	"rps r1": "HD", "rps r2": "DVDRip", "rps r3": "DVDScr", "rps r4": "R5", "rps r5": "CAM", "rps r6": "WEBRip",
	"rps r7": "BDRip", "rps r8": "WEB-DL", "rps r9": "HDRip", "rps r10": "HDTS", "rps r12": "BluRay",
	"rip1": "DVDRip", "rip2": "DVDScr", "rip3": "WEBRip", "rip4": "BDRip", "rip5": "BRRip", "rip6": "CAM",
	"rip7": "HD", "rip8": "R5", "rip9": "WEB-DL", "rip10": "HDRip", "rip11": "HDTS",
}

// flags are the BCP 47 tags of the site's language flags.
var flags = map[string]string{"flagtr": "tr", "flagen": "en", "flages": "es", "flagfr": "fr", "flagger": "de", "flagita": "it"}

// Definition returns the provider's definition.
func Definition() provider.Definition {
	return provider.Definition{
		Name: Name, DisplayName: "Turkcealtyazi.org",
		Settings: []provider.Setting{
			{
				Name: "cookies", Label: "Cookies, e.g., PHPSESSID=abc; wikisubtitlesuser=xyz; wikisubtitlespass=efg",
				Help: "For requests coming from outside of Turkey, cookies and user agent are required. Especially cf_clearance cookie.",
				Type: provider.Text,
			},
			{Name: "user_agent", Label: "User-Agent, e.g., Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:95.0) Gecko/20100101 Firefox/95.0", Type: provider.Text},
		},
		Languages:                 []string{"tr", "en"},
		HearingImpairedVerifiable: true, Movies: true, Episodes: true,
		New: New,
	}
}

type settings struct {
	Cookies   string `json:"cookies"`
	UserAgent string `json:"user_agent"`
}

// New makes the provider from its settings; the cookies and User-Agent
// are optional, as Bazarr's.
func New(env provider.Env, raw json.RawMessage) (provider.Provider, error) {
	var s settings
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, provider.Errorf(provider.ConfigurationError, "settings: %w", err)
		}
	}
	base, _ := url.Parse("https://turkcealtyazi.org")
	return &Provider{env: env, s: s, base: base, pages: map[string]string{}}, nil
}

// Provider is Turkcealtyazi.org.
type Provider struct {
	env  provider.Env
	s    settings
	base *url.URL

	mu   sync.Mutex
	sess *site.Session
	// pages are the subtitles' pages, by ID.
	pages map[string]string
}

// SetBaseURL points the provider at another server, for tests.
func (p *Provider) SetBaseURL(u *url.URL) { p.base = u }

func (p *Provider) server() string { return strings.TrimRight(p.base.String(), "/") }

// session returns the session, checking the cookies on the home page, as
// Bazarr's initialize.
func (p *Provider) session(ctx context.Context) (*site.Session, error) {
	if p.sess != nil {
		return p.sess, nil
	}
	s := site.New(p.env.HTTP)
	s.Check = func(*site.Response) error { return nil }
	s.Header.Set("User-Agent", cmp.Or(p.s.UserAgent, site.RandomUserAgent()))
	s.Header.Set("Referer", p.server())
	if p.s.Cookies != "" {
		s.Jar().SetCookies(p.base, sitekit.ParseCookies(p.s.Cookies))
	}
	r, err := s.Get(ctx, p.server(), nil)
	if err != nil {
		return nil, err
	}
	if r.Status == http.StatusForbidden {
		return nil, provider.Errorf(provider.AuthenticationError, "the cookies with the User-Agent are not valid anymore")
	}
	p.sess = s
	return s, nil
}

// Search implements provider.Provider, as Bazarr's list_subtitles: by the
// IMDb ID of the movie or the series.
func (p *Provider) Search(ctx context.Context, v *provider.Video, langs []provider.Language) ([]provider.Subtitle, error) {
	imdb := v.ExternalIDs["imdb"]
	if v.Episode {
		imdb = v.SeriesExternalIDs["imdb"]
	}
	if imdb == "" {
		p.env.Log.DebugContext(ctx, "turkcealtyaziorg: no IMDb ID to search with")
		return nil, nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	s, err := p.session(ctx)
	if err != nil {
		return nil, err
	}
	r, err := s.Get(ctx, p.server()+"/find.php", url.Values{"cat": {"sub"}, "find": {imdb}})
	if err != nil {
		return nil, err
	}
	if r.Status == http.StatusNotFound {
		return nil, nil
	}
	if r.Status != http.StatusOK {
		if err := site.Status(r); err != nil {
			return nil, err
		}
	}
	doc, err := site.Parse([]byte(strings.ToValidUTF8(string(r.Body), "")), "text/html; charset=utf-8")
	if err != nil {
		return nil, provider.Errorf(provider.ParseResponseError, "results: %w", err)
	}
	meta := doc.First(`meta[name="description"]`)
	if !meta.Valid() || strings.Contains(meta.Attr("content"), "404 Error") {
		p.env.Log.DebugContext(ctx, "turkcealtyaziorg: IMDb ID not found", "imdb", imdb)
		return nil, nil
	}
	selector := "div.altyazi-list-wrapper > div > div.altsonsez2"
	if v.Episode {
		selector = fmt.Sprintf("div.altyazi-list-wrapper > div > div.altsonsez1.sezon_%d", v.Season)
	}
	var subs []provider.Subtitle
	for _, item := range doc.All(selector) {
		e, err := p.entry(item, v)
		if err != nil {
			// As Bazarr's, a malformed entry ends the list.
			p.env.Log.DebugContext(ctx, "turkcealtyaziorg: parsing an entry", "error", err)
			break
		}
		lang, ok := sitekit.Match(langs, e.lang)
		if !ok || e.lang == "" || v.Episode && (e.season != v.Season || !e.pack && e.episode != v.Number) {
			continue
		}
		lang.HearingImpaired = e.hearingImpaired
		id := e.page
		if v.Episode {
			if e.pack {
				e.episode = v.Number
			}
			id += fmt.Sprintf("S%02dE%02d", e.season, e.episode)
		}
		p.pages[id] = e.page
		subs = append(subs, provider.Subtitle{
			ID: id, Language: lang, ReleaseNames: e.releases,
			Matches:  matches(v, e.releases),
			PageLink: e.page, Uploader: e.uploader,
		})
	}
	return subs, nil
}

// entry is a subtitle of the results.
type entry struct {
	page, lang, uploader string
	season, episode      int
	pack                 bool
	releases             []string
	hearingImpaired      bool
}

// entry parses an entry of the results, as Bazarr's query.
func (p *Provider) entry(item site.Node, v *provider.Video) (entry, error) {
	var e entry
	a := item.First("div.alisim > div.fl > a")
	if !a.Valid() {
		return e, fmt.Errorf("no link")
	}
	e.page = p.server() + a.Attr("href")
	flag := item.First("div.aldil > span")
	if !flag.Valid() {
		return e, fmt.Errorf("no language")
	}
	e.lang = flags[firstClass(flag)]
	if v.Episode {
		alcd := item.First("div.alcd")
		bs := alcd.All("b")
		if len(bs) != 2 {
			return e, fmt.Errorf("no season and episode")
		}
		var err error
		if e.season, err = strconv.Atoi(bs[0].Text()); err != nil {
			return e, fmt.Errorf("season: %w", err)
		}
		if e.episode, err = strconv.Atoi(bs[1].Text()); err != nil {
			e.pack = true
		}
	}
	translator := item.First("div.alcevirmen")
	if !translator.Valid() {
		return e, fmt.Errorf("no uploader")
	}
	if t := translator.Text(); t != "" {
		e.uploader = strings.TrimSpace(t)
	} else {
		e.uploader = identifiers[strings.Join(strings.Fields(translator.First("span").Attr("class")), " ")]
	}
	rips := item.First("div.ta-container > div.ripdiv")
	if !rips.Valid() {
		return e, fmt.Errorf("no rips")
	}
	for _, span := range rips.All("span") {
		if r := identifiers[strings.Join(strings.Fields(span.Attr("class")), " ")]; r != "" {
			e.releases = append(e.releases, r)
		}
	}
	for _, part := range strings.Split(strings.TrimSpace(rips.Text()), "/") {
		if part = strings.TrimSpace(part); part != "" {
			e.releases = append(e.releases, part)
		}
	}
	e.hearingImpaired = rips.First(`img[src="/images/isitme.png"]`).Valid()
	return e, nil
}

func firstClass(n site.Node) string {
	if f := strings.Fields(n.Attr("class")); len(f) > 0 {
		return f[0]
	}
	return ""
}

// matches are what the search by IMDb ID and the releases say of the
// video, as Bazarr's get_matches without guessit's.
func matches(v *provider.Video, releases []string) []string {
	var out []string
	if v.Episode {
		out = []string{"series", "season", "episode", "series_imdb_id"}
	} else {
		out = []string{"imdb_id"}
	}
	info := sitekit.SanitizeReleaseGroup(strings.Join(releases, ","))
	if v.ReleaseGroup != "" && info != "" {
		for _, g := range sitekit.EquivalentReleaseGroups(sitekit.SanitizeReleaseGroup(v.ReleaseGroup)) {
			if strings.Contains(info, g) {
				out = append(out, "release_group")
				break
			}
		}
	}
	return out
}

var episodeSuffix = regexp.MustCompile(`S\d{2,}E\d{2,}$`)

// Download implements provider.Provider, as Bazarr's download_subtitle:
// the subtitle's form posted from its page.
func (p *Provider) Download(ctx context.Context, v *provider.Video, id string) (provider.File, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	page, ok := p.pages[id]
	if !ok {
		page = episodeSuffix.ReplaceAllString(id, "")
	}
	s, err := p.session(ctx)
	if err != nil {
		return provider.File{}, err
	}
	r, err := s.Get(ctx, page, nil)
	if err != nil {
		return provider.File{}, err
	}
	if err := site.Status(r); err != nil {
		return provider.File{}, err
	}
	doc, err := site.Parse([]byte(strings.ToValidUTF8(string(r.Body), "")), "text/html; charset=utf-8")
	if err != nil {
		return provider.File{}, provider.Errorf(provider.ParseResponseError, "subtitle page: %w", err)
	}
	form := url.Values{}
	for _, name := range []string{"idid", "altid", "sidid"} {
		in := doc.First(fmt.Sprintf(`input[name="%s"]`, name))
		if !in.Valid() {
			return provider.File{}, provider.Errorf(provider.ParseResponseError, "no %s on the subtitle page", name)
		}
		form.Set(name, in.Attr("value"))
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.server()+"/ind", strings.NewReader(form.Encode()))
	if err != nil {
		return provider.File{}, fmt.Errorf("download: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", page)
	if r, err = s.Do(req); err != nil {
		return provider.File{}, err
	}
	if len(r.Body) == 0 {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "no data returned")
	}
	lang := provider.Language{}
	return archive.Extract(r.Body, archive.WantFor(v, lang), lang)
}
