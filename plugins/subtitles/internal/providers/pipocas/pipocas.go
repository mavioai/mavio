// Package pipocas is the provider of Pipocas.tv, ported from Bazarr's
// pipocas: Portuguese, Brazilian, English and Spanish subtitles of movies
// and episodes, with an account.
package pipocas

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/archive"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/site"
	"github.com/mavioai/mavio/plugins/subtitles/internal/sitekit"
)

// Name is the provider's name.
const Name = "pipocas"

const (
	// searchDelay is the pause before each search, as Bazarr's
	// SEARCH_DELAY.
	searchDelay = 5 * time.Second
	// userAgent is Bazarr's default SZ_USER_AGENT.
	userAgent = "Sub-Zero/2"
	// idPrefix starts the subtitles' IDs, as Bazarr's.
	idPrefix = Name + "_"
	// signUp is on the site's pages when logged out.
	signUp = "Cria uma conta"
)

// siteLanguages are the site's names of the languages, as Bazarr's
// PIPOCAS_LANGUAGE_MAP.
var siteLanguages = []struct{ tag, name string }{
	{"pt", "portugues"}, {"pt-BR", "brasileiro"}, {"en", "ingles"}, {"es", "espanhol"}, //nolint:misspell // The site's names.
}

// Definition returns the provider's definition.
func Definition() provider.Definition {
	return provider.Definition{
		Name: Name, DisplayName: "Pipocas.tv",
		Settings: []provider.Setting{
			{Name: "username", Label: "Username", Help: "Portuguese / Brazilian / English / Spanish Subtitles Provider", Type: provider.Text},
			{Name: "password", Label: "Password", Type: provider.Password},
		},
		Languages: []string{"pt", "pt-BR", "en", "es"},
		Movies:    true, Episodes: true,
		New: New,
	}
}

type settings struct {
	Username string `json:"username"`
	Password string `json:"password"`
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
		return nil, provider.Errorf(provider.ConfigurationError, "username/password not configured")
	}
	base, _ := url.Parse("https://pipocas.tv")
	return &Provider{env: env, s: s, base: base, delay: searchDelay}, nil
}

// Provider is Pipocas.tv.
type Provider struct {
	env   provider.Env
	s     settings
	base  *url.URL
	delay time.Duration

	mu   sync.Mutex
	sess *site.Session
}

// SetBaseURL points the provider at another server, for tests.
func (p *Provider) SetBaseURL(u *url.URL) { p.base = u }

var (
	tokenRe    = regexp.MustCompile(`(?i)<meta name="csrf-token" content="(.+?)">`)
	downloadRe = regexp.MustCompile(`/legendas/download/([^"']+)`)
	colorRe    = regexp.MustCompile(`color:\s*#[0-9A-Fa-f]{3,6}`)
	ratingRe   = regexp.MustCompile(`(\d+)\s*/\s*\d+`)
)

// session returns the logged-in session, as Bazarr's initialize.
func (p *Provider) session(ctx context.Context) (*site.Session, error) {
	if p.sess != nil {
		return p.sess, nil
	}
	s := site.New(p.env.HTTP)
	s.Header.Set("User-Agent", userAgent)
	s.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	s.Header.Set("Origin", p.base.String())
	s.Header.Set("Referer", p.base.String())
	loginURL := p.base.JoinPath("login").String()
	r, err := s.Get(ctx, loginURL, nil)
	if err != nil {
		return nil, err
	}
	m := tokenRe.FindSubmatch(r.Body)
	if m == nil {
		return nil, provider.Errorf(provider.ServiceUnavailable, "unable to find the CSRF token on the login page")
	}
	r, err = s.PostForm(ctx, loginURL, url.Values{"username": {p.s.Username}, "password": {p.s.Password}, "_token": {string(m[1])}})
	if err != nil {
		return nil, err
	}
	if bytes.Contains(r.Body, []byte(signUp)) {
		return nil, provider.Errorf(provider.AuthenticationError, "login failed, check credentials")
	}
	p.env.Log.DebugContext(ctx, "pipocas: logged in")
	p.sess = s
	return s, nil
}

// found is a subtitle found, with its rating in stars.
type found struct {
	sub   provider.Subtitle
	stars int
}

// Search implements provider.Provider, as Bazarr's list_subtitles: the
// best rated and most downloaded first.
func (p *Provider) Search(ctx context.Context, v *provider.Video, langs []provider.Language) ([]provider.Subtitle, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	var all []found
	for _, sl := range siteLanguages {
		lang, ok := sitekit.Match(langs, sl.tag)
		if !ok {
			continue
		}
		subs, err := p.searchLanguage(ctx, v, lang, sl.name)
		if err != nil {
			switch provider.KindOf(err) {
			case provider.ParseResponseError, provider.Other:
				p.env.Log.WarnContext(ctx, "pipocas: searching", "language", lang.Tag, "error", err)
				continue
			}
			return nil, err
		}
		all = append(all, subs...)
	}
	slices.SortStableFunc(all, func(a, b found) int {
		return cmp.Or(cmp.Compare(b.stars, a.stars), cmp.Compare(b.sub.Downloads, a.sub.Downloads))
	})
	out := make([]provider.Subtitle, 0, len(all))
	for _, f := range all {
		out = append(out, f.sub)
	}
	return out, nil
}

// query is what a video is searched by, as Bazarr's _build_search_query.
func query(v *provider.Video) string {
	if v.Episode {
		if v.Series != "" && v.Season != 0 && v.Number != 0 {
			return fmt.Sprintf("%s S%02dE%02d", v.Series, v.Season, v.Number)
		}
		if v.Series != "" {
			return v.Series
		}
	} else if v.Title != "" {
		return v.Title
	}
	if v.FileName != "" {
		name := path.Base(strings.ReplaceAll(v.FileName, `\`, "/"))
		return strings.TrimSuffix(name, path.Ext(name))
	}
	return ""
}

// searchLanguage lists the subtitles of a language, read from their
// pages, as Bazarr's _search_language.
func (p *Provider) searchLanguage(ctx context.Context, v *provider.Video, lang provider.Language, siteLang string) ([]found, error) {
	q := query(v)
	if q == "" {
		return nil, nil
	}
	s, err := p.session(ctx)
	if err != nil {
		return nil, err
	}
	if err := site.Sleep(ctx, p.delay); err != nil {
		return nil, err
	}
	r, err := s.Get(ctx, p.base.JoinPath("legendas").String(), url.Values{"t": {"rel"}, "l": {siteLang}, "page": {"1"}, "s": {q}})
	if err != nil {
		return nil, err
	}
	if bytes.Contains(r.Body, []byte(signUp)) {
		return nil, provider.Errorf(provider.AuthenticationError, "not authenticated during search")
	}
	doc, err := r.HTML()
	if err != nil {
		return nil, provider.Errorf(provider.ParseResponseError, "results: %w", err)
	}
	var out []found
	for _, a := range doc.All("a.text-dark.no-decoration[href]") {
		href := a.Attr("href")
		if !strings.Contains(href, "/legendas/info/") {
			continue
		}
		if !strings.HasPrefix(href, "http") {
			href = strings.TrimRight(p.base.String(), "/") + href
		}
		f, ok, err := p.details(ctx, s, v, href, lang)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, f)
		}
	}
	return out, nil
}

// details reads a subtitle's page, as Bazarr's _parse_details_page.
func (p *Provider) details(ctx context.Context, s *site.Session, v *provider.Video, u string, lang provider.Language) (found, bool, error) {
	r, err := s.Get(ctx, u, nil)
	if err != nil {
		return found{}, false, err
	}
	doc, err := r.HTML()
	if err != nil {
		return found{}, false, provider.Errorf(provider.ParseResponseError, "%s: %w", u, err)
	}
	release := doc.First("h3.title span.font-normal").TrimmedText()
	var id string
	for _, a := range doc.All("a[href]") {
		if m := downloadRe.FindStringSubmatch(a.Attr("href")); m != nil {
			id = m[1]
			break
		}
	}
	if id == "" {
		return found{}, false, nil
	}
	hits, _ := strconv.Atoi(doc.First("span.hits.hits-pd div").TrimmedText())
	uploader := "pipocas-bot"
	for _, span := range doc.All("span[style]") {
		if colorRe.MatchString(span.Attr("style")) {
			if t := span.TrimmedText(); t != "" {
				uploader = t
			}
			break
		}
	}
	rating := 0
	if m := ratingRe.FindStringSubmatch(doc.First(`h2[class="mt-3 text-center"]`).Text()); m != nil {
		rating, _ = strconv.Atoi(m[1])
	}
	// Stars of 0 to 5, from the rating and hits, as Bazarr's.
	hitFactor := 0.0
	if rating != 0 {
		hitFactor = min(float64(hits)/100, 5)
	}
	stars := int(math.RoundToEven((float64(rating) + hitFactor) / 2))
	sub := provider.Subtitle{
		ID: idPrefix + id, Language: lang, Matches: matches(v, release),
		PageLink: p.base.JoinPath("legendas", "download", id).String(), Uploader: uploader, Downloads: hits,
	}
	if release != "" {
		sub.ReleaseNames = []string{release}
	}
	return found{sub: sub, stars: stars}, true, nil
}

// matches are what a release shows of the video, as Bazarr's get_matches
// without guessit's.
func matches(v *provider.Video, release string) []string {
	rel := strings.ToLower(release)
	var out []string
	if v.Episode {
		if v.Series != "" && strings.Contains(rel, strings.ToLower(v.Series)) {
			out = append(out, "series")
		}
		if v.Season != 0 && strings.Contains(rel, fmt.Sprintf("s%02d", v.Season)) {
			out = append(out, "season")
		}
		if v.Number != 0 && strings.Contains(rel, fmt.Sprintf("e%02d", v.Number)) {
			out = append(out, "episode")
		}
	} else if v.Title != "" && strings.Contains(rel, strings.ToLower(v.Title)) {
		out = append(out, "title")
	}
	if v.Year != 0 && strings.Contains(release, strconv.Itoa(v.Year)) {
		out = append(out, "year")
	}
	return out
}

// Download implements provider.Provider, as Bazarr's download_subtitle:
// a failed request logs in again and is tried once more.
func (p *Provider) Download(ctx context.Context, v *provider.Video, id string) (provider.File, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	u := p.base.JoinPath("legendas", "download", strings.TrimPrefix(id, idPrefix)).String()
	r, err := p.get(ctx, u)
	if err != nil {
		p.sess = nil
		if r, err = p.get(ctx, u); err != nil {
			return provider.File{}, err
		}
	}
	if bytes.Contains(r.Body, []byte(signUp)) {
		return provider.File{}, provider.Errorf(provider.AuthenticationError, "not authenticated during download")
	}
	if len(r.Body) == 0 {
		return provider.File{}, provider.Errorf(provider.ServiceUnavailable, "empty download response")
	}
	isArchive := bytes.HasPrefix(r.Body, []byte("PK")) || bytes.HasPrefix(r.Body, []byte("Rar!"))
	lang := provider.Language{}
	f, err := archive.Extract(r.Body, archive.WantFor(v, lang), lang)
	switch {
	case err == nil:
		return f, nil
	case isArchive:
		return provider.File{}, provider.Errorf(provider.ServiceUnavailable, "no suitable subtitle file in the archive: %w", err)
	default:
		return provider.File{}, provider.Errorf(provider.ProviderError, "unidentified archive type: %w", err)
	}
}

func (p *Provider) get(ctx context.Context, u string) (*site.Response, error) {
	s, err := p.session(ctx)
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, u, nil)
}
