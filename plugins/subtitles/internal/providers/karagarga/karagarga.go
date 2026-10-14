// Package karagarga is the provider of Karagarga, ported from Bazarr's
// karagarga: English subtitles of movies requested on the private
// tracker, attached to the posts of its forum, with the tracker's and the
// forum's accounts.
package karagarga

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/andybalholm/cascadia"
	"golang.org/x/net/html"

	"github.com/mavioai/mavio/plugins/subtitles/internal/archive"
	"github.com/mavioai/mavio/plugins/subtitles/internal/cache"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/site"
	"github.com/mavioai/mavio/plugins/subtitles/internal/sitekit"
)

// Name is the provider's name.
const Name = "karagarga"

// expiration is how long pages are kept, as Bazarr's _cached_get.
const expiration = 7 * 24 * time.Hour

// authKey is the forum's login form key, as Bazarr posts it.
const authKey = "880ea6a14ea49e853634fbdc5015a024"

// Definition returns the provider's definition.
func Definition() provider.Definition {
	return provider.Definition{
		Name: Name, DisplayName: "Karagarga.in",
		Settings: []provider.Setting{
			{Name: "username", Label: "Username", Help: "Movie Subtitles Provider (Private Tracker)", Type: provider.Text},
			{Name: "password", Label: "Password", Type: provider.Password},
			{Name: "f_username", Label: "Forum username", Type: provider.Text},
			{Name: "f_password", Label: "Forum password", Type: provider.Password},
		},
		Languages: []string{"en"},
		Movies:    true,
		New:       New,
	}
}

type settings struct {
	Username  string `json:"username"`
	Password  string `json:"password"`
	FUsername string `json:"f_username"`
	FPassword string `json:"f_password"`
}

// New makes the provider from its settings; the forum's account is the
// tracker's unless set.
func New(env provider.Env, raw json.RawMessage) (provider.Provider, error) {
	var s settings
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, provider.Errorf(provider.ConfigurationError, "settings: %w", err)
		}
	}
	if s.Username == "" || s.Password == "" {
		return nil, provider.Errorf(provider.ConfigurationError, "username/password not provided")
	}
	s.FUsername, s.FPassword = cmp.Or(s.FUsername, s.Username), cmp.Or(s.FPassword, s.Password)
	base, _ := url.Parse("https://karagarga.in")
	forum, _ := url.Parse("https://forum.karagarga.in/index.php")
	return &Provider{env: env, s: s, base: base, forum: forum, cache: cache.New(env)}, nil
}

// Provider is Karagarga.
type Provider struct {
	env   provider.Env
	s     settings
	base  *url.URL
	forum *url.URL
	// cache keeps the pages, as Bazarr's region cache.
	cache *cache.Region

	mu   sync.Mutex
	sess *site.Session
}

// SetBaseURL points the provider, the tracker and its forum, at another
// server, for tests.
func (p *Provider) SetBaseURL(u *url.URL) {
	p.base = u
	p.forum = u.ResolveReference(&url.URL{Path: "/index.php"})
}

// session returns the logged-in session, logging in to the tracker and
// its forum first, as Bazarr's initialize.
func (p *Provider) session(ctx context.Context) (*site.Session, error) {
	if p.sess != nil {
		return p.sess, nil
	}
	s := site.New(p.env.HTTP)
	s.Header.Set("User-Agent", "Bazarr")
	if _, err := s.PostForm(ctx, p.base.JoinPath("takelogin.php").String(), url.Values{
		"username": {p.s.Username}, "password": {p.s.Password},
	}); err != nil {
		return nil, err
	}
	if !hasCookies(s.Jar(), p.base, "pass") {
		return nil, provider.Errorf(provider.AuthenticationError, "invalid username/password")
	}
	p.env.Log.DebugContext(ctx, "karagarga: login OK")
	if _, err := s.PostForm(ctx, p.forum.String()+"?"+url.Values{
		"app": {"core"}, "module": {"global"}, "section": {"login"}, "do": {"process"},
	}.Encode(), url.Values{
		"auth_key": {authKey}, "referer": {"https://forum.karagarga.in/"}, "ips_username": {p.s.FUsername},
		"ips_password": {p.s.FPassword}, "rememberMe": {"1"}, "anonymous": {"1"},
	}); err != nil {
		return nil, err
	}
	if !hasCookies(s.Jar(), p.forum, "session_id", "pass_hash") {
		return nil, provider.Errorf(provider.AuthenticationError, "invalid forum username/password")
	}
	p.env.Log.DebugContext(ctx, "karagarga: forum login OK")
	p.sess = s
	return s, nil
}

func hasCookies(jar http.CookieJar, u *url.URL, names ...string) bool {
	have := map[string]bool{}
	for _, c := range jar.Cookies(u) {
		have[c.Name] = true
	}
	for _, n := range names {
		if !have[n] {
			return false
		}
	}
	return true
}

// get returns a page, kept a week, as Bazarr's _cached_get.
func (p *Provider) get(ctx context.Context, u string, query url.Values) (site.Node, error) {
	key := u + "?" + query.Encode()
	body, ok := cache.Get[[]byte](ctx, p.cache, key)
	if !ok {
		s, err := p.session(ctx)
		if err != nil {
			return site.Node{}, err
		}
		r, err := s.Get(ctx, u, query)
		if err != nil {
			return site.Node{}, err
		}
		// Bazarr's 302 to the login page, followed here.
		if req, _ := url.Parse(u); req != nil && r.URL.Path != req.Path {
			return site.Node{}, provider.Errorf(provider.AuthenticationError, "not logged in")
		}
		body = r.Body
		cache.Set(ctx, p.cache, key, body, expiration)
	}
	doc, err := site.Parse(body, "")
	if err != nil {
		return site.Node{}, provider.Errorf(provider.ParseResponseError, "%s: %w", u, err)
	}
	return doc, nil
}

// Search implements provider.Provider: the most downloaded subtitle of
// the movie's approved requests, as Bazarr's list_subtitles.
func (p *Provider) Search(ctx context.Context, v *provider.Video, langs []provider.Language) ([]provider.Subtitle, error) {
	lang, ok := sitekit.Match(langs, "en")
	if v.Episode || !ok {
		return nil, nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	subs, err := p.searchMovie(ctx, v.Title, v.Year, lang)
	if err != nil || len(subs) == 0 {
		return nil, err
	}
	slices.SortStableFunc(subs, func(a, b provider.Subtitle) int { return cmp.Compare(b.Downloads, a.Downloads) })
	return subs[:1], nil
}

// maxScans bounds the forum's threads read for a search.
const maxScans = 3

// searchMovie lists the subtitles of the approved English requests of a
// movie of a year, as Bazarr's _search_movie.
func (p *Provider) searchMovie(ctx context.Context, title string, year int, lang provider.Language) ([]provider.Subtitle, error) {
	doc, err := p.get(ctx, p.base.JoinPath("pots.php").String(), url.Values{"search": {title}, "status": {"completed"}})
	if err != nil {
		return nil, err
	}
	table := doc.First(`table[cellspacing="5"]`)
	if !table.Valid() {
		p.env.Log.DebugContext(ctx, "karagarga: failed to get table")
		return nil, nil
	}
	var subs []provider.Subtitle
	scans := 0
	for _, tr := range table.All("tr") {
		if !strings.Contains(render(tr), "forum.karagarga") {
			continue
		}
		tds := tr.All("td")
		if len(tds) != 11 {
			continue
		}
		if !strings.Contains(tds[1].Text(), fmt.Sprintf("(%d", year)) {
			p.env.Log.DebugContext(ctx, "karagarga: year doesn't match", "title", tds[1].Text())
			continue
		}
		if !strings.Contains(tds[5].Text(), "English") {
			continue
		}
		if !strings.Contains(render(tds[9]), "approved") {
			continue
		}
		forumURL := tds[9].First("a").Attr("href")
		if forumURL == "" {
			continue
		}
		if scans >= maxScans {
			p.env.Log.DebugContext(ctx, "karagarga: forum scans limit exceeded")
			break
		}
		found, err := p.parseForum(ctx, forumURL, lang)
		if err != nil {
			return nil, err
		}
		subs = append(subs, found...)
		scans++
	}
	return subs, nil
}

// potentials are the elements of a post that may hold an attachment, as
// Bazarr selects them.
var potentials = cascadia.MustCompile("p,li.attachment,div")

// parseForum lists the subtitles attached to the posts of a forum's
// thread, as Bazarr's _gen_subtitles.
func (p *Provider) parseForum(ctx context.Context, u string, lang provider.Language) ([]provider.Subtitle, error) {
	doc, err := p.get(ctx, u, nil)
	if err != nil {
		return nil, err
	}
	var out []provider.Subtitle
	for _, post := range doc.All("div.post.entry-content") {
		seen := map[string]bool{}
		for _, m := range cascadia.QueryAll(post.Node, potentials) {
			n := site.Node{Node: m}
			desc := n.First("span.desc.lighter")
			if !desc.Valid() {
				continue
			}
			fields := strings.Fields(desc.Text())
			if len(fields) == 0 {
				continue
			}
			count, err := strconv.Atoi(fields[0])
			if err != nil {
				continue
			}
			var item site.Node
			for _, a := range n.All("a") {
				if a.First("strong").Valid() {
					item = a
					break
				}
			}
			link := item.Attr("href")
			if !item.Valid() || link == "" || seen[link] {
				continue
			}
			seen[link] = true
			out = append(out, provider.Subtitle{
				ID: link, Language: lang,
				ReleaseNames: []string{item.First("strong").Text()},
				// The movie was searched by its title, and its year checked.
				Matches:   []string{"title", "year"},
				PageLink:  link,
				Downloads: count,
			})
		}
	}
	return out, nil
}

// render returns an element's markup, as str() of a tag.
func render(n site.Node) string {
	var b strings.Builder
	if err := html.Render(&b, n.Node); err != nil {
		return ""
	}
	return b.String()
}

// Download implements provider.Provider.
func (p *Provider) Download(ctx context.Context, v *provider.Video, id string) (provider.File, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	s, err := p.session(ctx)
	if err != nil {
		return provider.File{}, err
	}
	r, err := s.Get(ctx, id, nil)
	if err != nil {
		return provider.File{}, err
	}
	lang := provider.Language{Tag: "en"}
	return archive.Extract(r.Body, archive.WantFor(v, lang), lang)
}
