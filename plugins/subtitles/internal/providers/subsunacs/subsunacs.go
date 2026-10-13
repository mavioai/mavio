// Package subsunacs is the provider of Subsunacs.net, ported from
// Bazarr's subsunacs: Bulgarian and English subtitles of movies and
// episodes found by title, each file of the site's archives a subtitle.
package subsunacs

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/net/html"

	"github.com/mavioai/mavio/plugins/subtitles/internal/archive"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/scrape"
	"github.com/mavioai/mavio/plugins/subtitles/internal/site"
)

// Name is the provider's name.
const Name = "subsunacs"

// serverURL is the site's.
const serverURL = "https://subsunacs.net"

// tvRenames and movieRenames are the titles the site names differently,
// as Bazarr's fix_tv_naming and fix_movie_naming.
var (
	tvRenames = []string{
		"Marvel's Daredevil", "Daredevil", "Marvel's Luke Cage", "Luke Cage", "Marvel's Iron Fist", "Iron Fist",
		"DC's Legends of Tomorrow", "Legends of Tomorrow", "Doctor Who (2005)", "Doctor Who",
		"Star Trek: Deep Space Nine", "Star Trek DS9", "Star Trek: The Next Generation", "Star Trek TNG",
		"Superman & Lois", "Superman and Lois",
	}
	movieRenames = []string{
		"Back to the Future Part III", "Back to the Future 3", "Back to the Future Part II", "Back to the Future 2",
		"Bill & Ted Face the Music", "Bill Ted Face the Music",
	}
)

// Definition returns the provider's definition.
func Definition() provider.Definition {
	return provider.Definition{
		Name: Name, DisplayName: "Subsunacs.net",
		Languages: []string{"bg", "en"},
		Movies:    true, Episodes: true,
		New: New,
	}
}

// New makes the provider; it has no settings.
func New(env provider.Env, _ json.RawMessage) (provider.Provider, error) {
	// The search is not redirected, as Bazarr's.
	hc := *env.HTTP
	hc.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	p := &Provider{env: env, search: site.New(&hc), s: site.New(env.HTTP), base: serverURL, archives: map[string][]byte{}}
	p.search.Check = func(r *site.Response) error {
		if r.Status >= 300 && r.Status < 400 {
			return nil
		}
		return site.Status(r)
	}
	for _, s := range []*site.Session{p.search, p.s} {
		s.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
		s.Header.Set("Accept-Language", "en-US,en;q=0.5")
		s.Header.Set("DNT", "1")
		s.Header.Set("Upgrade-Insecure-Requests", "1")
		s.Header.Set("Cache-Control", "max-age=0")
	}
	return p, nil
}

// Provider is Subsunacs.net.
type Provider struct {
	env       provider.Env
	search, s *site.Session
	base      string

	mu sync.Mutex
	// archives caches the archives downloaded while searching by link, as
	// Bazarr's region cache.
	archives map[string][]byte
}

// SetBaseURL points the provider at another server, for tests.
func (p *Provider) SetBaseURL(u *url.URL) { p.base = strings.TrimSuffix(u.String(), "/") }

// row is a subtitle of the site's search results.
type row struct {
	link, notes, title string
	year, numCDs       int
	uploader           string
}

var (
	notesRe  = regexp.MustCompile(`(<img.*)(src=")(/)(.*.jpg">)`)
	readmeRe = regexp.MustCompile(`(?i)subsunacs\.net|танете част|прочети|^read ?me|procheti`)
)

// query lists the subtitles of the video in a language, as Bazarr's
// query: those of the first 20 results, each file of their archives.
func (p *Provider) query(ctx context.Context, l provider.Language, v *provider.Video) ([]sub, error) {
	params := url.Values{
		"m": {""}, "l": {"0"}, "c": {""}, "y": {""}, "action": {"   Търси   "}, "a": {""}, "d": {""}, "u": {""},
		"g": {""}, "t": {""}, "imdbcheck": {"1"},
	}
	if v.Episode {
		params.Set("m", fmt.Sprintf("%s %02d %02d", scrape.SanitizePatch(scrape.FixNaming(v.Series, tvRenames...), "'"), v.Season, v.Number))
	} else {
		// requests leaves out a year of None.
		params.Del("y")
		if v.Year != 0 {
			params.Set("y", strconv.Itoa(v.Year))
		}
		params.Set("m", scrape.SanitizePatch(scrape.FixNaming(v.Title, movieRenames...), "'"))
	}
	if provider.Alpha3(l.Tag) == "eng" {
		params.Set("l", "1")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.base+"/search.php?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Referer", p.base+"/index.php")
	r, err := p.search.Do(req)
	if err != nil {
		return nil, err
	}
	if r.Status != http.StatusOK {
		p.env.Log.DebugContext(ctx, "subsunacs: no subtitles found")
		return nil, nil
	}
	doc, err := scrape.HTML(r)
	if err != nil {
		return nil, provider.Errorf(provider.ParseResponseError, "search: %w", err)
	}
	var out []sub
	rows := doc.All("tr[onmouseover]")
	for _, tr := range rows[:min(20, len(rows))] {
		a := tr.First("td.tdMovie a.tooltip")
		if !a.Valid() {
			continue
		}
		rw := row{link: a.Attr("href"), title: a.Text(), notes: notesRe.ReplaceAllString(a.Attr("title"), "")}
		if span := nextSpan(a, "smGray"); span.Valid() {
			rw.year, _ = strconv.Atoi(strings.Trim(span.Text(), " ()"))
		}
		tds := tr.All("td")
		if len(tds) > 1 {
			rw.numCDs, _ = strconv.Atoi(tds[1].Text())
		}
		if len(tds) > 5 {
			rw.uploader = tds[5].Text()
		}
		files, err := p.archive(ctx, p.base+rw.link)
		if err != nil {
			return nil, err
		}
		for _, f := range files {
			out = append(out, sub{row: rw, file: f.name, data: f.data, single: len(files) == 1 && rw.numCDs == 1})
		}
	}
	return out, nil
}

// nextSpan returns the next sibling span of an element of a class, as
// BeautifulSoup's find_next_sibling.
func nextSpan(n site.Node, class string) site.Node {
	for s := n.NextSibling; s != nil; s = s.NextSibling {
		if s.Type == html.ElementNode && s.Data == "span" && slices.Contains(strings.Fields(site.Node{Node: s}.Attr("class")), class) {
			return site.Node{Node: s}
		}
	}
	return site.Node{}
}

// sub is a file of a result's archive.
type sub struct {
	row
	file   string
	data   []byte
	single bool
}

type file struct {
	name string
	data []byte
}

// archive returns the subtitles of an archive, as Bazarr's
// download_archive_and_add_subtitle_files: the valid .srt, .sub and .txt
// files of zip, RAR and 7z archives but readmes, sorted; none of another
// content.
func (p *Provider) archive(ctx context.Context, link string) ([]file, error) {
	p.mu.Lock()
	data, ok := p.archives[link]
	p.mu.Unlock()
	if !ok {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Referer", p.base+"/search.php")
		r, err := p.s.Do(req)
		if err != nil {
			return nil, err
		}
		data = r.Body
	}
	a, err := openArchive(data)
	if err != nil {
		p.env.Log.ErrorContext(ctx, "subsunacs: ignoring an unsupported archive", "link", link, "error", err)
		p.mu.Lock()
		delete(p.archives, link)
		p.mu.Unlock()
		return nil, nil
	}
	p.mu.Lock()
	p.archives[link] = data
	p.mu.Unlock()
	var out []file
	for _, name := range slices.Sorted(slices.Values(a.Names())) {
		lower := strings.ToLower(name)
		if !strings.HasSuffix(lower, "srt") && !strings.HasSuffix(lower, ".sub") && !strings.HasSuffix(lower, ".txt") {
			continue
		}
		if strings.HasSuffix(lower, ".txt") && readmeRe.MatchString(name) {
			p.env.Log.DebugContext(ctx, "subsunacs: ignoring a readme", "file", name)
			continue
		}
		data, err := a.Read(name)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		if archive.Detect(data) == "" {
			continue
		}
		out = append(out, file{name: name, data: archive.FixLineEndings(data)})
	}
	return out, nil
}

// openArchive opens a zip, RAR or 7z archive.
func openArchive(data []byte) (*archive.Archive, error) {
	_, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil && !bytes.HasPrefix(data, []byte("Rar!\x1a\x07")) && !bytes.HasPrefix(data, []byte("7z\xbc\xaf\x27\x1c")) {
		return nil, errors.New("not a zip, RAR or 7z archive")
	}
	return archive.Open(data)
}

var (
	bracketed = regexp.MustCompile(`\[\w+\]$`)
	cdRe      = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])cd\s?\d`)
)

// stem is a file's name without its folder, extension and bracketed
// suffix, uppercased.
func stem(name string) string {
	base := path.Base(strings.ReplaceAll(name, `\`, "/"))
	base = strings.TrimSuffix(base, path.Ext(base))
	return strings.ToUpper(strings.TrimSpace(bracketed.ReplaceAllString(base, "")))
}

// matches are what a subtitle matches of the video, as Bazarr's
// get_matches knows from the site; the file name's and the title's are
// guessed by the server.
func matches(s sub, v *provider.Video) []string {
	var out []string
	if vf := stem(v.FileName); v.FileName != "" && (vf == stem(s.file) || s.single && strings.Contains(strings.ToUpper(s.notes), vf)) {
		out = append(out, "hash")
	}
	if v.Year != 0 && s.year == v.Year {
		out = append(out, "year")
	}
	return out
}

// Search implements provider.Provider, as Bazarr's list_subtitles: each
// language is searched. Files of movies on several discs match nothing.
func (p *Provider) Search(ctx context.Context, v *provider.Video, langs []provider.Language) ([]provider.Subtitle, error) {
	var out []provider.Subtitle
	for _, l := range langs {
		subs, err := p.query(ctx, l, v)
		if err != nil {
			return nil, err
		}
		for _, s := range subs {
			st := provider.Subtitle{
				ID:       url.Values{"link": {p.base + s.link}, "file": {s.file}}.Encode(),
				Language: l, PageLink: p.base + s.link, Uploader: s.uploader, Comment: s.notes,
				Format: format(s.file, s.data),
			}
			if v.Episode || s.numCDs <= 1 && !cdRe.MatchString(s.file) {
				st.Matches = matches(s, v)
				st.ReleaseNames = []string{s.file, s.title}
			}
			out = append(out, st)
		}
	}
	return out, nil
}

func format(name string, data []byte) string {
	if strings.EqualFold(path.Ext(name), ".srt") {
		return "srt"
	}
	return archive.Detect(data)
}

// Download implements provider.Provider: the file of the archive,
// downloaded again unless cached, as Bazarr's download_subtitle.
func (p *Provider) Download(ctx context.Context, _ *provider.Video, id string) (provider.File, error) {
	q, err := url.ParseQuery(id)
	link, name := q.Get("link"), q.Get("file")
	if err != nil || link == "" || name == "" {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "malformed subtitle ID %q", id)
	}
	files, err := p.archive(ctx, link)
	if err != nil {
		return provider.File{}, err
	}
	for _, f := range files {
		if f.name == name {
			return provider.File{Data: f.data, Format: format(f.name, f.data)}, nil
		}
	}
	return provider.File{}, provider.Errorf(provider.BadSubtitle, "no file %s in the archive of %s", name, link)
}
