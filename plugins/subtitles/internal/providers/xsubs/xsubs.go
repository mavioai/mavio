// Package xsubs is the provider of XSubs, ported from Bazarr's xsubs:
// Greek subtitles of episodes, listed in the site's XML pages of shows and
// seasons, with an optional account.
package xsubs

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

	"golang.org/x/text/encoding/charmap"

	"github.com/mavioai/mavio/plugins/subtitles/internal/archive"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/site"
	"github.com/mavioai/mavio/plugins/subtitles/internal/sitekit"
)

// Name is the provider's name.
const Name = "xsubs"

const (
	// showExpiration is how long the show IDs are kept, as Bazarr's
	// SHOW_EXPIRATION_TIME.
	showExpiration = 7 * 24 * time.Hour
	// userAgent is subliminal's.
	userAgent = "Subliminal/2.1"
	signIn    = "/xforum/account/signin/"
	// seriesCategory is the category of the shows.
	seriesCategory = "Σειρές"
)

// Definition returns the provider's definition.
func Definition() provider.Definition {
	return provider.Definition{
		Name: Name, DisplayName: "XSubs",
		Settings: []provider.Setting{
			{Name: "username", Label: "Username", Help: "Greek Subtitles Provider", Type: provider.Text},
			{Name: "password", Label: "Password", Type: provider.Password},
		},
		Languages: []string{"el"},
		Episodes:  true,
		New:       New,
	}
}

type settings struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// New makes the provider from its settings; the account is optional, as
// Bazarr's, but both its username and password are needed.
func New(env provider.Env, raw json.RawMessage) (provider.Provider, error) {
	var s settings
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, provider.Errorf(provider.ConfigurationError, "settings: %w", err)
		}
	}
	if (s.Username == "") != (s.Password == "") {
		return nil, provider.Errorf(provider.ConfigurationError, "username and password must be specified")
	}
	base, _ := url.Parse("http://xsubs.tv")
	return &Provider{env: env, s: s, base: base, pages: map[string]string{}}, nil
}

// Provider is XSubs.
type Provider struct {
	env  provider.Env
	s    settings
	base *url.URL

	mu        sync.Mutex
	sess      *site.Session
	shows     map[string]int
	showsTill time.Time
	// pages are the subtitles' pages, by ID.
	pages map[string]string
}

// SetBaseURL points the provider at another server, for tests.
func (p *Provider) SetBaseURL(u *url.URL) { p.base = u }

func (p *Provider) server() string { return strings.TrimRight(p.base.String(), "/") }

var (
	articleRe     = regexp.MustCompile(`^([A-Za-z]{1,3}) (.*)$`)
	episodeRe     = regexp.MustCompile(`^(\d+)(-(\d+))*$`)
	episodeNameRe = regexp.MustCompile(`^(.*?)( [\[(].{2,4}[\])])*$`)
	seriesRe      = regexp.MustCompile(`^(.*?)( \[\D+\])*$`)
)

// session returns the session, logged in with the account when there is
// one, as Bazarr's initialize.
func (p *Provider) session(ctx context.Context) (*site.Session, error) {
	if p.sess != nil {
		return p.sess, nil
	}
	s := site.New(p.env.HTTP)
	s.Header.Set("User-Agent", userAgent)
	if p.s.Username != "" {
		p.env.Log.DebugContext(ctx, "xsubs: logging in")
		u := p.server() + signIn
		if _, err := s.Get(ctx, u, nil); err != nil {
			return nil, err
		}
		token := ""
		for _, c := range s.Jar().Cookies(p.base) {
			if c.Name == "csrftoken" {
				token = c.Value
			}
		}
		if token == "" {
			return nil, provider.Errorf(provider.ProviderError, "no CSRF token")
		}
		s.Check = func(*site.Response) error { return nil }
		r, err := s.PostForm(ctx, u, url.Values{"username": {p.s.Username}, "password": {p.s.Password}, "csrfmiddlewaretoken": {token}})
		if err != nil {
			return nil, err
		}
		s.Check = nil
		// Redirected away from the form: logged in.
		if r.URL.Path == signIn {
			return nil, provider.Errorf(provider.AuthenticationError, "%s", p.s.Username)
		}
	}
	p.sess = s
	return s, nil
}

// retry calls f, and again on a failed request after logging in again, as
// Bazarr's reinitialize_on_error.
func retry[T any](ctx context.Context, p *Provider, f func(*site.Session) (T, error)) (T, error) {
	s, err := p.session(ctx)
	if err != nil {
		var zero T
		return zero, err
	}
	v, err := f(s)
	switch provider.KindOf(err) {
	case provider.ServiceUnavailable, provider.Timeout, provider.ProviderError, provider.TooManyRequests, provider.ForbiddenError, provider.AuthenticationError:
		p.sess = nil
		if s, err = p.session(ctx); err != nil {
			return v, err
		}
		return f(s)
	}
	return v, err
}

// showIDs returns the IDs of the shows by sanitized name, kept a week, as
// Bazarr's _get_show_ids.
func (p *Provider) showIDs(ctx context.Context, s *site.Session) (map[string]int, error) {
	now := p.env.Now()
	if p.shows != nil && now.Before(p.showsTill) {
		return p.shows, nil
	}
	r, err := s.Get(ctx, p.server()+"/series/all.xml", nil)
	if err != nil {
		return nil, err
	}
	if len(r.Body) == 0 {
		return nil, nil
	}
	doc, err := r.HTML()
	if err != nil {
		return nil, provider.Errorf(provider.ParseResponseError, "shows: %w", err)
	}
	ids := map[string]int{}
	for _, category := range doc.All("seriesl") {
		if category.Attr("category") != seriesCategory {
			continue
		}
		for _, show := range category.All("series") {
			name := show.Text()
			if m := seriesRe.FindStringSubmatch(name); m != nil {
				name = m[1]
			}
			if id, err := strconv.Atoi(show.Attr("srsid")); err == nil {
				ids[sitekit.Sanitize(name)] = id
			}
		}
		break
	}
	if len(ids) > 0 {
		p.shows, p.showsTill = ids, now.Add(showExpiration)
	}
	return ids, nil
}

// showID returns the ID of a show by its names and year, 0 when unknown,
// as Bazarr's get_show_id.
func showID(ids map[string]int, names []string, year int) int {
	var sanitized []string
	for _, n := range names {
		n = sitekit.Sanitize(n)
		sanitized = append(sanitized, n)
		// With the article at the end.
		if m := articleRe.FindStringSubmatch(n); m != nil {
			sanitized = append(sanitized, m[2]+" "+m[1])
		}
	}
	for _, n := range sanitized {
		if year != 0 {
			if id := ids[fmt.Sprintf("%s %d", n, year)]; id != 0 {
				return id
			}
			if id := ids[fmt.Sprintf("%s [%d]", n, year)]; id != 0 {
				return id
			}
		}
		if id := ids[n]; id != 0 {
			return id
		}
	}
	return 0
}

// sub is a subtitle of a season.
type sub struct {
	series, title, version, page, download string
	season, episode                        int
}

// query lists the subtitles of a season of a show, as Bazarr's query.
func (p *Provider) query(ctx context.Context, s *site.Session, show, season int) ([]sub, error) {
	r, err := s.Get(ctx, fmt.Sprintf("%s/series/%d/main.xml", p.server(), show), nil)
	if err != nil {
		return nil, err
	}
	if len(r.Body) == 0 {
		return nil, nil
	}
	doc, err := r.HTML()
	if err != nil {
		return nil, provider.Errorf(provider.ParseResponseError, "show: %w", err)
	}
	series := doc.First("name").Text()
	if m := episodeNameRe.FindStringSubmatch(series); m != nil {
		series = m[1]
	}
	seasonID := -1
	for _, g := range doc.All("series_group") {
		if n, err := strconv.Atoi(g.Attr("ssnnum")); err == nil && n == season {
			if seasonID, err = strconv.Atoi(g.Attr("ssnid")); err != nil {
				seasonID = -1
				continue
			}
			break
		}
	}
	if seasonID < 0 {
		p.env.Log.DebugContext(ctx, "xsubs: season not found", "season", season)
		return nil, nil
	}
	if r, err = s.Get(ctx, fmt.Sprintf("%s/series/%d/%d.xml", p.server(), show, seasonID), nil); err != nil {
		return nil, err
	}
	if len(r.Body) == 0 {
		return nil, nil
	}
	if doc, err = r.HTML(); err != nil {
		return nil, provider.Errorf(provider.ParseResponseError, "season: %w", err)
	}
	var subs []sub
	for _, group := range doc.All("subg") {
		info := group.First("etitle")
		if !info.Valid() {
			continue
		}
		var episodes []int
		if m := episodeRe.FindStringSubmatch(info.Attr("number")); m != nil {
			for _, e := range []string{m[1], m[3]} {
				if n, err := strconv.Atoi(e); err == nil {
					episodes = append(episodes, n)
				}
			}
		}
		sgt := group.First("sgt")
		if !sgt.Valid() {
			continue
		}
		subSeason, err := strconv.Atoi(sgt.Attr("ssnnum"))
		if err != nil {
			return nil, provider.Errorf(provider.ParseResponseError, "season number %q", sgt.Attr("ssnnum"))
		}
		episodeID, err := strconv.Atoi(sgt.Attr("epsid"))
		if err != nil {
			return nil, provider.Errorf(provider.ParseResponseError, "episode ID %q", sgt.Attr("epsid"))
		}
		for _, sr := range group.All("sr") {
			// Not released yet.
			if sr.Attr("published_on") == "" {
				continue
			}
			release, err := strconv.Atoi(sr.Attr("rlsid"))
			if err != nil {
				return nil, provider.Errorf(provider.ParseResponseError, "release ID %q", sr.Attr("rlsid"))
			}
			for _, e := range episodes {
				subs = append(subs, sub{
					series: series, title: info.Attr("title"), season: subSeason, episode: e,
					version:  sr.First("fmt").Text() + " " + sr.First("team").Text(),
					page:     fmt.Sprintf("%s/ice/xsw.xml?srsid=%d#%d;%d;%d", p.server(), show, seasonID, subSeason, episodeID),
					download: fmt.Sprintf("%s/xthru/getsub/%d", p.server(), release),
				})
			}
		}
	}
	return subs, nil
}

// Search implements provider.Provider, as Bazarr's list_subtitles.
func (p *Provider) Search(ctx context.Context, v *provider.Video, langs []provider.Language) ([]provider.Subtitle, error) {
	lang, ok := sitekit.Match(langs, "el")
	if !v.Episode || !ok {
		return nil, nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return retry(ctx, p, func(s *site.Session) ([]provider.Subtitle, error) {
		ids, err := p.showIDs(ctx, s)
		if err != nil {
			return nil, err
		}
		show := showID(ids, append([]string{v.Series}, v.AlternativeSeries...), v.Year)
		if show == 0 {
			p.env.Log.DebugContext(ctx, "xsubs: no show ID found", "series", v.Series, "year", v.Year)
			return nil, nil
		}
		subs, err := p.query(ctx, s, show, v.Season)
		if err != nil {
			return nil, err
		}
		var out []provider.Subtitle
		for _, sub := range subs {
			if sub.season != v.Season || sub.episode != v.Number {
				continue
			}
			p.pages[sub.download] = sub.page
			out = append(out, provider.Subtitle{
				ID: sub.download, Language: lang, ReleaseNames: []string{sub.version},
				Matches: matches(v, sub), PageLink: sub.page,
			})
		}
		return out, nil
	})
}

// matches are what the site says of the episode's subtitle, as Bazarr's
// get_matches without guessit's.
func matches(v *provider.Video, s sub) []string {
	var out []string
	if v.Series != "" && slices.ContainsFunc(append([]string{v.Series}, v.AlternativeSeries...), func(n string) bool {
		return sitekit.Sanitize(n) == sitekit.Sanitize(s.series)
	}) {
		out = append(out, "series")
	}
	if v.Season != 0 && s.season == v.Season {
		out = append(out, "season")
	}
	if v.Number != 0 && s.episode == v.Number {
		out = append(out, "episode")
	}
	if v.Title != "" && sitekit.Sanitize(s.title) == sitekit.Sanitize(v.Title) {
		out = append(out, "title")
	}
	// The subtitle's year is the video's, as Bazarr's query is given.
	if v.OriginalSeries && v.Year == 0 || v.Year != 0 {
		out = append(out, "year")
	}
	if v.ReleaseGroup != "" && s.version != "" && slices.ContainsFunc(sitekit.EquivalentReleaseGroups(sitekit.SanitizeReleaseGroup(v.ReleaseGroup)), func(g string) bool {
		return strings.Contains(sitekit.SanitizeReleaseGroup(s.version), g)
	}) {
		out = append(out, "release_group")
	}
	return out
}

// Download implements provider.Provider, as Bazarr's download_subtitle:
// the site's subtitles are in Windows-1253 unless UTF-8.
func (p *Provider) Download(ctx context.Context, _ *provider.Video, id string) (provider.File, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	data, err := retry(ctx, p, func(s *site.Session) ([]byte, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, id, nil)
		if err != nil {
			return nil, provider.Errorf(provider.BadSubtitle, "malformed subtitle ID %q: %w", id, err)
		}
		req.Header.Set("Referer", cmp.Or(p.pages[id], p.server()))
		r, err := s.Do(req)
		if err != nil {
			return nil, err
		}
		return r.Body, nil
	})
	if err != nil {
		return provider.File{}, err
	}
	if len(data) == 0 {
		return provider.File{}, provider.Errorf(provider.ProviderError, "no data returned")
	}
	data = sitekit.Decode(archive.FixLineEndings(data), charmap.Windows1253)
	return provider.File{Data: data, Format: archive.Detect(data)}, nil
}
