// Package subscenter is the provider of SubsCenter, ported from Bazarr's
// subscenter (subliminal's, patched): Hebrew subtitles of movies and
// episodes found by title.
package subscenter

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/archive"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/scrape"
	"github.com/mavioai/mavio/plugins/subtitles/internal/site"
)

// Name is the provider's name.
const Name = "subscenter"

// serverURL is the site's, as Bazarr's patched one.
const serverURL = "http://www.subscenter.info/he/"

// showExpiration is how long the URL titles of a title are cached, as
// subliminal's SHOW_EXPIRATION_TIME.
const showExpiration = 3 * 7 * 24 * time.Hour

// Definition returns the provider's definition.
func Definition() provider.Definition {
	return provider.Definition{
		Name: Name, DisplayName: "Subscenter",
		Languages:                 []string{"he"},
		HearingImpairedVerifiable: true,
		Movies:                    true, Episodes: true,
		New: New,
	}
}

// New makes the provider; Bazarr passes it no settings, so it does not
// log in.
func New(env provider.Env, _ json.RawMessage) (provider.Provider, error) {
	return &Provider{env: env, s: site.New(env.HTTP), base: serverURL, titles: map[string]cached{}}, nil
}

// Provider is SubsCenter.
type Provider struct {
	env  provider.Env
	s    *site.Session
	base string

	mu sync.Mutex
	// titles caches the URL titles of titles by kind, as Bazarr's region
	// cache.
	titles map[string]cached
}

type cached struct {
	byKind map[string][]string
	until  time.Time
}

// SetBaseURL points the provider at another server, for tests.
func (p *Provider) SetBaseURL(u *url.URL) { p.base = strings.TrimSuffix(u.String(), "/") + "/he/" }

// urlTitles returns the URL titles a title is found as, by kind ("movie",
// "series"), as Bazarr's _search_url_titles.
func (p *Provider) urlTitles(ctx context.Context, title string) (map[string][]string, error) {
	now := p.env.Now()
	p.mu.Lock()
	c, ok := p.titles[title]
	p.mu.Unlock()
	if ok && now.Before(c.until) {
		return c.byKind, nil
	}
	searchURL := p.base + "subtitle/search/"
	r, err := p.s.Get(ctx, searchURL, url.Values{"q": {title}})
	if err != nil {
		return nil, err
	}
	var links []string
	if su, err := url.Parse(searchURL); err == nil && r.URL.Path != su.Path {
		// Redirected to the subtitles page.
		links = []string{r.URL.String()}
	} else {
		doc, err := scrape.HTML(r)
		if err != nil {
			return nil, provider.Errorf(provider.ParseResponseError, "search: %w", err)
		}
		for _, a := range doc.All("#processes div.generalWindowTop a") {
			links = append(links, a.Attr("href"))
		}
	}
	byKind := map[string][]string{}
	for _, link := range links {
		parts := strings.Split(link, "/")
		if len(parts) < 3 {
			continue
		}
		byKind[parts[len(parts)-3]] = append(byKind[parts[len(parts)-3]], parts[len(parts)-2])
	}
	if len(byKind) > 0 {
		p.mu.Lock()
		p.titles[title] = cached{byKind: byKind, until: now.Add(showExpiration)}
		p.mu.Unlock()
	}
	return byKind, nil
}

// item is a subtitle of the site's lists.
type item struct {
	HearingImpaired json.RawMessage `json:"hearing_impaired"`
	ID              int64           `json:"id"`
	Key             string          `json:"key"`
	Version         string          `json:"h_version"`
	Downloaded      int             `json:"downloaded"`
	Release         string          `json:"subtitle_version"`
}

// subtitle is a subtitle with the releases of its items.
type subtitle struct {
	lang            string
	hearingImpaired bool
	id              int64
	key, version    string
	downloaded      int
	releases        []string
}

// query lists the subtitles of a movie, or of an episode, as Bazarr's
// query.
func (p *Provider) query(ctx context.Context, title string, season, episode int) ([]*subtitle, string, error) {
	byKind, err := p.urlTitles(ctx, title)
	if err != nil {
		return nil, "", err
	}
	var listURL, pageLink string
	if season != 0 && episode != 0 {
		if len(byKind["series"]) == 0 {
			p.env.Log.DebugContext(ctx, "subscenter: no URL title found for series", "title", title)
			return nil, "", nil
		}
		t := byKind["series"][0]
		listURL = fmt.Sprintf("%scst/data/series/sb/%s/%d/%d/", p.base, t, season, episode)
		pageLink = fmt.Sprintf("%ssubtitle/series/%s/%d/%d/", p.base, t, season, episode)
	} else {
		if len(byKind["movie"]) == 0 {
			p.env.Log.DebugContext(ctx, "subscenter: no URL title found for movie", "title", title)
			return nil, "", nil
		}
		t := byKind["movie"][0]
		listURL = fmt.Sprintf("%scst/data/movie/sb/%s/", p.base, t)
		pageLink = fmt.Sprintf("%ssubtitle/movie/%s/", p.base, t)
	}
	r, err := p.s.Get(ctx, listURL, nil)
	if err != nil {
		return nil, "", err
	}
	var results map[string]map[string]map[string]map[string]item
	if err := r.JSON(&results); err != nil {
		return nil, "", err
	}
	var out []*subtitle
	byID := map[int64]*subtitle{}
	for _, lang := range sortedKeys(results) {
		for _, qualities := range sortedValues(results[lang]) {
			for _, items := range sortedValues(qualities) {
				for _, it := range sortedValues(items) {
					if s, ok := byID[it.ID]; ok {
						// Another release of the subtitle.
						i, _ := slices.BinarySearch(s.releases, it.Release)
						s.releases = slices.Insert(s.releases, i, it.Release)
						s.downloaded += it.Downloaded
						continue
					}
					s := &subtitle{
						lang: lang, hearingImpaired: truthy(it.HearingImpaired), id: it.ID,
						key: it.Key, version: it.Version, downloaded: it.Downloaded, releases: []string{it.Release},
					}
					byID[it.ID] = s
					out = append(out, s)
				}
			}
		}
	}
	return out, pageLink, nil
}

// truthy is Python's bool() of a JSON value.
func truthy(raw json.RawMessage) bool {
	switch strings.TrimSpace(string(raw)) {
	case "", "0", "false", "null", `""`:
		return false
	}
	return true
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedValues[V any](m map[string]V) []V {
	var out []V
	for _, k := range sortedKeys(m) {
		out = append(out, m[k])
	}
	return out
}

// Search implements provider.Provider, as Bazarr's list_subtitles.
func (p *Provider) Search(ctx context.Context, v *provider.Video, langs []provider.Language) ([]provider.Subtitle, error) {
	title, season, episode := v.Title, 0, 0
	if v.Episode {
		title, season, episode = v.Series, v.Season, v.Number
	}
	subs, pageLink, err := p.query(ctx, title, season, episode)
	if err != nil {
		return nil, err
	}
	var out []provider.Subtitle
	for _, s := range subs {
		tag := provider.FromCode(s.lang)
		if !slices.ContainsFunc(langs, func(l provider.Language) bool { return l.Tag == tag }) {
			continue
		}
		out = append(out, provider.Subtitle{
			ID: url.Values{
				"lang": {s.lang}, "id": {strconv.FormatInt(s.id, 10)}, "v": {s.version}, "key": {s.key}, "page": {pageLink},
			}.Encode(),
			Language:     provider.Language{Tag: tag, HearingImpaired: s.hearingImpaired},
			ReleaseNames: s.releases,
			Matches:      matches(title, season, episode, v),
			PageLink:     pageLink,
			Downloads:    s.downloaded,
		})
	}
	return out, nil
}

// matches are what a subtitle matches of the video, as subliminal's
// get_matches: its series, season and episode are those searched for,
// and its title the title searched for.
func matches(title string, season, episode int, v *provider.Video) []string {
	var out []string
	if v.Episode {
		if v.Series != "" && scrape.Sanitize(title, "") == scrape.Sanitize(v.Series, "") {
			out = append(out, "series")
		}
		if v.Season != 0 && season == v.Season {
			out = append(out, "season")
		}
		if v.Number != 0 && episode == v.Number {
			out = append(out, "episode")
		}
	}
	if v.Title != "" && scrape.Sanitize(title, "") == scrape.Sanitize(v.Title, "") {
		out = append(out, "title")
	}
	return out
}

// Download implements provider.Provider, as Bazarr's download_subtitle:
// the single subtitle of a zip; anything else is the daily limit
// exceeded.
func (p *Provider) Download(ctx context.Context, _ *provider.Video, id string) (provider.File, error) {
	q, err := url.ParseQuery(id)
	if err != nil || q.Get("lang") == "" || q.Get("id") == "" {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "malformed subtitle ID %q", id)
	}
	u := p.base + "subtitle/download/" + url.PathEscape(q.Get("lang")) + "/" + url.PathEscape(q.Get("id")) + "/?" +
		url.Values{"v": {q.Get("v")}, "key": {q.Get("key")}}.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return provider.File{}, err
	}
	req.Header.Set("Referer", q.Get("page"))
	r, err := p.s.Do(req)
	if err != nil {
		return provider.File{}, err
	}
	zr, err := zip.NewReader(bytes.NewReader(r.Body), int64(len(r.Body)))
	if err != nil {
		// No zip is the daily downloads limit exceeded.
		return provider.File{}, provider.Errorf(provider.ProviderError, "Daily limit exceeded")
	}
	var names []*zip.File
	for _, f := range zr.File {
		if !strings.HasSuffix(f.Name, ".txt") {
			names = append(names, f)
		}
	}
	switch len(names) {
	case 0:
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "no file in the zip")
	case 1:
	default:
		return provider.File{}, provider.Errorf(provider.ProviderError, "More than one file to unzip")
	}
	a, err := archive.Open(r.Body)
	if err != nil {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "%w", err)
	}
	data, err := a.Read(names[0].Name)
	if err != nil {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "%s: %w", names[0].Name, err)
	}
	format := strings.TrimPrefix(strings.ToLower(path.Ext(names[0].Name)), ".")
	if format != "srt" && format != "ass" && format != "ssa" && format != "vtt" {
		format = archive.Detect(data)
	}
	return provider.File{
		Data: archive.FixLineEndings(data), Format: format, Language: provider.Language{Tag: provider.FromCode(q.Get("lang"))},
	}, nil
}
