// Package yavkanet is the provider of Yavka.net, ported from Bazarr's
// yavkanet: Bulgarian and other subtitles of movies and episodes found by
// IMDb ID, each file of the archives the site serves being a subtitle.
package yavkanet

import (
	"archive/zip"
	"bytes"
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

	"golang.org/x/net/html"

	"github.com/mavioai/mavio/plugins/subtitles/internal/archive"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/site"
)

// Name is the provider's name.
const Name = "yavkanet"

// serverURL is the site's.
const serverURL = "https://yavka.net/"

// pause is the wait before the requests of a subtitle, which spares the
// site's throttling.
const pause = time.Second

// archiveExpiration is how long downloaded archives are kept, from the
// search to the download.
const archiveExpiration = 24 * time.Hour

// rows is how many of the page's last rows are read.
const rows = 50

// Definition returns the provider's definition.
func Definition() provider.Definition {
	return provider.Definition{
		Name: Name, DisplayName: "Yavka.net",
		Languages: []string{"bg", "en", "ru", "es", "it"},
		Movies:    true, Episodes: true,
		New: New,
	}
}

// New makes the provider; it has no settings.
func New(env provider.Env, _ json.RawMessage) (provider.Provider, error) {
	header := http.Header{
		"Accept":                    {"text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8"},
		"Accept-Language":           {"en-US,en;q=0.5"},
		"Dnt":                       {"1"},
		"Upgrade-Insecure-Requests": {"1"},
		"Cache-Control":             {"max-age=0"},
	}
	s := site.New(env.HTTP)
	s.Header = header
	// The archives are posted for without following redirects.
	hc := *env.HTTP
	hc.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	post := site.New(&hc)
	post.Header = header
	post.Check = func(r *site.Response) error {
		if r.Status >= 300 && r.Status < 400 {
			return nil
		}
		return site.Status(r)
	}
	base, _ := url.Parse(serverURL)
	return &Provider{env: env, s: s, post: post, base: base, pause: pause, archives: map[string]cached{}}, nil
}

// Provider is Yavka.net.
type Provider struct {
	env   provider.Env
	s     *site.Session
	post  *site.Session
	base  *url.URL
	pause time.Duration

	mu sync.Mutex
	// archives are the archives of subtitles' links, as Bazarr's region
	// cache.
	archives map[string]cached
}

type cached struct {
	data  []byte
	until time.Time
}

// SetBaseURL points the provider at another server, for tests.
func (p *Provider) SetBaseURL(u *url.URL) { p.base = u }

// subtitleLink returns the link of a subtitle's page, as Bazarr's
// 'https://yavka.net' + link + '/'.
func (p *Provider) subtitleLink(link string) string {
	return strings.TrimSuffix(p.base.String(), "/") + link + "/"
}

var (
	notesRE = regexp.MustCompile(`(?s)<p.*><img [A-z0-9='/\. :;#]*>(.*)</p>`)
	bracket = regexp.MustCompile(`\[\w+\]$`)
)

// Search implements provider.Provider, as Bazarr's list_subtitles: each
// language is searched by the IMDb ID of the movie or series, and the
// subtitles are the files of the archives of the results.
func (p *Provider) Search(ctx context.Context, v *provider.Video, langs []provider.Language) ([]provider.Subtitle, error) {
	imdbID := v.IMDbID()
	if imdbID == "" {
		return nil, nil
	}
	var out []provider.Subtitle
	for _, l := range langs {
		subs, err := p.query(ctx, v, l, imdbID)
		if err != nil {
			return nil, err
		}
		out = append(out, subs...)
	}
	return out, nil
}

// query lists the subtitles of an IMDb ID, labeled with the language
// searched, as Bazarr's query.
func (p *Provider) query(ctx context.Context, v *provider.Video, l provider.Language, imdbID string) ([]provider.Subtitle, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.base.JoinPath("imdb", imdbID).String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Referer", p.base.String())
	r, err := p.s.Do(req)
	if err != nil {
		return nil, err
	}
	if len(r.Body) == 0 || r.Status != http.StatusOK {
		p.env.Log.DebugContext(ctx, "no subtitles found", "provider", Name, "imdb", imdbID)
		return nil, nil
	}
	doc, err := r.HTML()
	if err != nil {
		return nil, provider.Errorf(provider.ParseResponseError, "search: %w", err)
	}
	trs := doc.All("tr")
	trs = trs[max(0, len(trs)-rows):]
	var out []provider.Subtitle
	for _, row := range trs {
		el, ok := result(row)
		if !ok {
			continue
		}
		link := el.Attr("href")
		notes := notesRE.ReplaceAllString(el.Attr("content"), "$1")
		title := el.Text()
		year := 0
		for s := el.NextSibling; s != nil; s = s.NextSibling {
			if s.Type == html.ElementNode && s.Data == "span" {
				year, _ = strconv.Atoi(strings.TrimSpace(strings.Trim(site.Node{Node: s}.Text(), "()")))
				break
			}
		}
		fps := 0.0
		if s := row.First(`span[title="Кадри в секунда"]`); s.Valid() {
			fps, _ = strconv.ParseFloat(s.TrimmedText(), 64)
		}
		uploader := row.First("a.click").Text()

		files, err := p.archiveFiles(ctx, link)
		if err != nil {
			return nil, err
		}
		for _, name := range files {
			release := name
			if fps != 0 {
				release += fmt.Sprintf(" [%.3f]", fps)
			}
			out = append(out, provider.Subtitle{
				ID:           url.Values{"link": {link}, "file": {name}, "language": {l.Tag}}.Encode(),
				Language:     l,
				ReleaseNames: []string{release, title},
				Matches:      matches(v, name, notes, year, len(files) == 1),
				PageLink:     p.subtitleLink(link),
				Uploader:     uploader,
				Comment:      notes,
			})
		}
	}
	return out, nil
}

// result returns a row's link to a subtitle, the first a.balon or
// a.selector.
func result(row site.Node) (site.Node, bool) {
	for _, a := range row.All("a") {
		classes := strings.Fields(a.Attr("class"))
		if slices.Contains(classes, "balon") || slices.Contains(classes, "selector") {
			return a, true
		}
	}
	return site.Node{}, false
}

// matches are what a subtitle matches, as Bazarr's
// YavkaNetSubtitle.get_matches: the hash when the subtitle file is named
// as the video, or is the only one of notes naming the video.
func matches(v *provider.Video, name, notes string, year int, single bool) []string {
	var out []string
	if video := stem(v.FileName); video != "" && (video == stem(name) || single && strings.Contains(strings.ToUpper(notes), video)) {
		out = append(out, "hash")
	}
	if v.Year != 0 && year == v.Year {
		out = append(out, "year")
	}
	return out
}

// stem is a file's name without its folder, extension and a bracketed
// suffix, in upper case.
func stem(name string) string {
	name = path.Base(strings.ReplaceAll(name, `\`, "/"))
	if name == "." || name == "/" {
		return ""
	}
	name = strings.TrimSuffix(name, path.Ext(name))
	return strings.ToUpper(strings.TrimSpace(bracket.ReplaceAllString(name, "")))
}

// archiveFiles returns the names of the subtitle files of a result's
// archive.
func (p *Provider) archiveFiles(ctx context.Context, link string) ([]string, error) {
	a, err := p.archive(ctx, link)
	if a == nil || err != nil {
		return nil, err
	}
	var out []string
	for _, name := range a.Names() {
		if lower := strings.ToLower(name); strings.HasSuffix(lower, ".srt") || strings.HasSuffix(lower, ".sub") {
			out = append(out, name)
		}
	}
	return out, nil
}

// archive returns the archive of a result, kept from the search to the
// download, as Bazarr's download_archive_and_add_subtitle_files: the
// subtitle's page has a form that is posted for it. nil when there is
// none.
func (p *Provider) archive(ctx context.Context, link string) (*archive.Archive, error) {
	key := p.subtitleLink(link)
	now := p.env.Now()
	p.mu.Lock()
	c, ok := p.archives[key]
	p.mu.Unlock()
	if ok && now.Before(c.until) {
		if a, err := archive.Open(c.data); err == nil {
			return a, nil
		}
	}
	if err := site.Sleep(ctx, p.pause); err != nil {
		return nil, err
	}
	r, err := p.s.Get(ctx, key[:len(key)-1], nil)
	if err != nil {
		return nil, err
	}
	if len(r.Body) == 0 {
		p.env.Log.InfoContext(ctx, "subtitle page did not load", "provider", Name, "link", link)
		return nil, nil
	}
	doc, err := r.HTML()
	if err != nil {
		return nil, provider.Errorf(provider.ParseResponseError, "%s: %w", link, err)
	}
	form := doc.First(`form[method="POST"]`)
	if !form.Valid() {
		p.env.Log.InfoContext(ctx, "could not find subtitle form data", "provider", Name, "link", link)
		return nil, nil
	}
	data := url.Values{}
	for _, in := range form.All("input") {
		if name := in.Attr("name"); name != "" {
			data.Set(name, in.Attr("value"))
		}
	}
	if err := site.Sleep(ctx, p.pause); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, key, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", key)
	r, err = p.post.Do(req)
	if err != nil {
		return nil, err
	}
	if len(r.Body) == 0 {
		return nil, nil
	}
	// Only RAR and zip archives are served.
	a, err := archive.Open(r.Body)
	if _, zerr := zip.NewReader(bytes.NewReader(r.Body), int64(len(r.Body))); err != nil || zerr != nil && !bytes.HasPrefix(r.Body, []byte("Rar!\x1a\x07")) {
		p.env.Log.ErrorContext(ctx, "ignore unsupported archive", "provider", Name, "link", link, "content_type", r.Header.Get("Content-Type"))
		return nil, nil
	}
	p.mu.Lock()
	for k, c := range p.archives {
		if !now.Before(c.until) {
			delete(p.archives, k)
		}
	}
	p.archives[key] = cached{r.Body, now.Add(archiveExpiration)}
	p.mu.Unlock()
	return a, nil
}

// Download implements provider.Provider: the file of the result's
// archive.
func (p *Provider) Download(ctx context.Context, _ *provider.Video, id string) (provider.File, error) {
	q, err := url.ParseQuery(id)
	if err != nil || q.Get("link") == "" || q.Get("file") == "" {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "malformed subtitle ID %q", id)
	}
	a, err := p.archive(ctx, q.Get("link"))
	if err != nil {
		return provider.File{}, err
	}
	if a == nil {
		return provider.File{}, provider.Errorf(provider.ProviderError, "no archive for %s", q.Get("link"))
	}
	data, err := a.Read(q.Get("file"))
	if err != nil {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "%s: %w", q.Get("file"), err)
	}
	data = archive.FixLineEndings(data)
	format := archive.Detect(data)
	if format == "" {
		format = strings.TrimPrefix(strings.ToLower(path.Ext(q.Get("file"))), ".")
	}
	return provider.File{Data: data, Format: format, Language: provider.Language{Tag: q.Get("language")}}, nil
}

// String names the provider in logs.
func (p *Provider) String() string { return fmt.Sprintf("%s(%s)", Name, p.base.Host) }
