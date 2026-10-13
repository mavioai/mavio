// Package subssabbz is the provider of Subs.sab.bz, ported from Bazarr's
// subssabbz: Bulgarian and English subtitles of movies and episodes found
// by title, each file of the site's archives a subtitle.
package subssabbz

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
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/mavioai/mavio/plugins/subtitles/internal/archive"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/scrape"
	"github.com/mavioai/mavio/plugins/subtitles/internal/site"
)

// Name is the provider's name.
const Name = "subssabbz"

// serverURL is the site's.
const serverURL = "http://subs.sab.bz"

// tvRenames and movieRenames are the titles the site names differently,
// as Bazarr's fix_tv_naming and fix_movie_naming.
var (
	tvRenames = []string{
		"Marvel's Daredevil", "Daredevil", "Marvel's Luke Cage", "Luke Cage", "Marvel's Iron Fist", "Iron Fist",
		"Marvel's Jessica Jones", "Jessica Jones", "DC's Legends of Tomorrow", "Legends of Tomorrow",
		"Doctor Who (2005)", "Doctor Who", "Star Trek: Deep Space Nine", "Star Trek DS9",
		"Star Trek: The Next Generation", "Star Trek TNG", "Superman & Lois", "Superman and Lois",
	}
	movieRenames = []string{"Back to the Future Part", "Back to the Future"}
)

// Definition returns the provider's definition.
func Definition() provider.Definition {
	return provider.Definition{
		Name: Name, DisplayName: "Subs.sab.bz",
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

// Provider is Subs.sab.bz.
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
	fps                float64
	uploader, imdbID   string
}

var (
	notesRe = regexp.MustCompile(`ddrivetip\('<div.*/></div>(.*)','#[0-9]+'\)`)
	imdbRe  = regexp.MustCompile(`imdb.com/title/(tt\d+)/?$`)
	yearRe  = regexp.MustCompile(`\s*\(\d{4}\).*$`)
)

// ignoredFolders are folder names no title is read from.
var ignoredFolders = []string{"season", "specials", "filmi", "serials", "media", "movies", "tv"}

// searchTitle returns the title searched for, as Bazarr's query: the
// title's without accents, else the folder's or file's of the video, else
// an alternative title's.
func searchTitle(v *provider.Video) string {
	title := scrape.SanitizePatch(scrape.FixNaming(v.Title, movieRenames...), "'")
	alts := v.AlternativeTitles
	if v.Episode {
		title = scrape.SanitizePatch(scrape.FixNaming(v.Series, tvRenames...), "'")
		alts = v.AlternativeSeries
	}
	if s := strings.TrimSpace(scrape.ASCII(title)); s != "" {
		return s
	}
	if v.FileName != "" {
		folder := strings.TrimSpace(yearRe.ReplaceAllString(parentFolder(v.FileName), ""))
		if f := strings.TrimSpace(scrape.ASCII(folder)); f != "" && !slices.Contains(ignoredFolders, strings.ToLower(f)) {
			return f
		}
		if g := scrape.Title(filepath.Base(v.FileName)); g != "" {
			if s := strings.TrimSpace(scrape.ASCII(g)); s != "" {
				return s
			}
		}
	}
	for _, alt := range alts {
		if s := strings.TrimSpace(scrape.ASCII(alt)); s != "" {
			return s
		}
	}
	return ""
}

// parentFolder is the name of the folder of a file; "" for none.
func parentFolder(name string) string {
	dir := filepath.Dir(name)
	if dir == "." || dir == string(filepath.Separator) {
		return ""
	}
	return filepath.Base(dir)
}

// query lists the subtitles of the video in a language, as Bazarr's
// query: those of the first 25 results, each file of their archives.
func (p *Provider) query(ctx context.Context, l provider.Language, v *provider.Video) ([]sub, error) {
	params := url.Values{"act": {"search"}, "movie": {""}, "select-language": {"2"}, "upldr": {""}, "yr": {""}, "release": {""}}
	if !v.Episode && v.Year != 0 {
		params.Set("yr", strconv.Itoa(v.Year))
	}
	title := searchTitle(v)
	if title == "" {
		// Never query with an empty title: the site dumps unrelated uploads.
		p.env.Log.WarnContext(ctx, "subssabbz: empty search title, skipping the query", "file", v.FileName)
		return nil, nil
	}
	params.Set("movie", title)
	if provider.Alpha3(l.Tag) == "eng" {
		params.Set("select-language", "1")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.base+"/index.php?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Referer", p.base+"/")
	r, err := p.search.Do(req)
	if err != nil {
		return nil, err
	}
	if r.Status != http.StatusOK {
		p.env.Log.DebugContext(ctx, "subssabbz: no subtitles found")
		return nil, nil
	}
	doc, err := scrape.HTML(r)
	if err != nil {
		return nil, provider.Errorf(provider.ParseResponseError, "search: %w", err)
	}
	videoIMDb := v.ExternalIDs["imdb"]
	if videoIMDb == "" && v.Episode {
		videoIMDb = v.SeriesExternalIDs["imdb"]
	}
	var out []sub
	rows := doc.All("tr.subs-row")
	for _, tr := range rows[:min(25, len(rows))] {
		a := tr.First("td.c2field a")
		if !a.Valid() {
			continue
		}
		rw := row{
			link: a.Attr("href"), title: a.Text(),
			notes: notesRe.ReplaceAllString(a.Attr("onmouseover"), "$1"),
		}
		rw.year, _ = strconv.Atoi(strings.Trim(scrape.NextText(a), " ()"))
		tds := tr.All("td")
		if len(tds) > 6 {
			rw.numCDs, _ = strconv.Atoi(tds[6].Text())
		}
		if len(tds) > 7 {
			rw.fps, _ = strconv.ParseFloat(tds[7].Text(), 64)
		}
		if len(tds) > 8 {
			rw.uploader = tds[8].Text()
		}
		if len(tds) > 9 {
			if m := imdbRe.FindStringSubmatch(tds[9].First("a").Attr("href")); m != nil {
				rw.imdbID = m[1]
			}
		}
		if videoIMDb != "" && rw.imdbID != "" && videoIMDb != rw.imdbID {
			p.env.Log.DebugContext(ctx, "subssabbz: skipping a subtitle of another IMDb ID", "link", rw.link, "imdb", rw.imdbID)
			continue
		}
		files, err := p.archive(ctx, rw.link)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			p.env.Log.WarnContext(ctx, "subssabbz: failed to download", "link", rw.link, "error", err)
			continue
		}
		for _, f := range files {
			out = append(out, sub{row: rw, file: f.name, data: f.data, single: len(files) == 1 && rw.numCDs == 1})
		}
	}
	return out, nil
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
// download_archive_and_add_subtitle_files: the .srt and .sub files of zip
// and RAR archives, sorted; none of another content.
func (p *Provider) archive(ctx context.Context, link string) ([]file, error) {
	p.mu.Lock()
	data, ok := p.archives[link]
	p.mu.Unlock()
	if !ok {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Referer", p.base+"/index.php?")
		r, err := p.s.Do(req)
		if err != nil {
			return nil, err
		}
		data = r.Body
	}
	a, err := openArchive(data)
	if err != nil {
		p.env.Log.ErrorContext(ctx, "subssabbz: ignoring an unsupported archive", "link", link, "error", err)
		p.mu.Lock()
		delete(p.archives, link)
		p.mu.Unlock()
		return nil, nil
	}
	p.mu.Lock()
	p.archives[link] = data
	p.mu.Unlock()
	names := slices.Sorted(slices.Values(a.Names()))
	var out []file
	for _, name := range names {
		lower := strings.ToLower(name)
		if !strings.HasSuffix(lower, ".srt") && !strings.HasSuffix(lower, ".sub") {
			continue
		}
		data, err := a.Read(name)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		out = append(out, file{name: name, data: archive.FixLineEndings(data)})
	}
	return out, nil
}

// openArchive opens a zip or RAR archive.
func openArchive(data []byte) (*archive.Archive, error) {
	isZip := false
	if _, err := zip.NewReader(bytes.NewReader(data), int64(len(data))); err == nil {
		isZip = true
	}
	if !isZip && !bytes.HasPrefix(data, []byte("Rar!\x1a\x07")) {
		return nil, errors.New("not a zip or RAR archive")
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
// guessed by the server. Files of movies on several discs match nothing.
func matches(s sub, v *provider.Video) []string {
	var out []string
	if vf := stem(v.FileName); v.FileName != "" && (vf == stem(s.file) || s.single && strings.Contains(strings.ToUpper(s.notes), vf)) {
		out = append(out, "hash")
	}
	if v.Year != 0 && s.year == v.Year {
		out = append(out, "year")
	}
	if !v.Episode {
		if id := v.ExternalIDs["imdb"]; id != "" && s.imdbID == id {
			out = append(out, "imdb_id", "title")
		}
		if strings.TrimSpace(scrape.ASCII(v.Title)) == "" && v.FileName != "" {
			folder := strings.TrimSpace(yearRe.ReplaceAllString(parentFolder(v.FileName), ""))
			if folder != "" && strings.Contains(strings.ToLower(s.title), strings.ToLower(folder)) && !slices.Contains(out, "title") {
				out = append(out, "title")
			}
		}
	} else {
		id := v.SeriesExternalIDs["imdb"]
		if id == "" {
			id = v.ExternalIDs["imdb"]
		}
		if id != "" && s.imdbID == id {
			out = append(out, "series_imdb_id", "series")
		}
	}
	return out
}

// multiDisc reports whether a subtitle is of a movie on several discs.
func multiDisc(s sub, v *provider.Video) bool {
	return !v.Episode && (s.numCDs > 1 || cdRe.MatchString(s.file))
}

// Search implements provider.Provider, as Bazarr's list_subtitles: each
// language is searched. A search the site forbids is tried again up to 3
// times, then skipped, as Bazarr's _retry_on_403.
func (p *Provider) Search(ctx context.Context, v *provider.Video, langs []provider.Language) ([]provider.Subtitle, error) {
	var out []provider.Subtitle
	for _, l := range langs {
		subs, err := p.queryRetrying(ctx, l, v)
		if err != nil {
			return nil, err
		}
		for _, s := range subs {
			st := provider.Subtitle{
				ID:       url.Values{"link": {s.link}, "file": {s.file}}.Encode(),
				Language: l, PageLink: s.link, Uploader: s.uploader, Comment: s.notes,
				Format: format(s.file, s.data),
			}
			if !multiDisc(s, v) {
				st.Matches = matches(s, v)
				st.ReleaseNames = []string{s.file, s.title}
			}
			out = append(out, st)
		}
	}
	return out, nil
}

func (p *Provider) queryRetrying(ctx context.Context, l provider.Language, v *provider.Video) ([]sub, error) {
	for range 3 {
		subs, err := p.query(ctx, l, v)
		if provider.KindOf(err) != provider.ForbiddenError {
			return subs, err
		}
		p.env.Log.DebugContext(ctx, "subssabbz: 403 returned, retrying")
		if err := site.Sleep(ctx, p.env.RetryWait); err != nil {
			return nil, err
		}
	}
	p.env.Log.DebugContext(ctx, "subssabbz: retries limit exceeded, ignoring the query")
	return nil, nil
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
