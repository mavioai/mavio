// Package subclub is the provider of SubClub.eu, ported from Bazarr's
// subclub: Estonian subtitles of movies and episodes found by title, each
// file of the site's archives a subtitle.
package subclub

import (
	"archive/zip"
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"net/url"
	"path"
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
const Name = "subclub"

// serverURL is the site's.
const serverURL = "https://www.subclub.eu"

var (
	// titleRe is an anchor's text: "Inception (2010)" or
	// "Game of Thrones (2011) [01x01]", as Bazarr's TITLE_RE.
	titleRe = regexp.MustCompile(`^(?P<title>.+?)\s*\((?P<year>\d{4})\)(?:\s*\[(?P<season>\d+)x(?P<episode>\d+)\])?\s*$`)
	imdbRe  = regexp.MustCompile(`(tt\d+)`)
	downRe  = regexp.MustCompile(`down\.php\?id=(\d+)`)
)

// Definition returns the provider's definition.
func Definition() provider.Definition {
	return provider.Definition{
		Name: Name, DisplayName: "SubClub.eu",
		Languages: []string{"et"},
		Movies:    true, Episodes: true,
		New: New,
	}
}

// New makes the provider; it has no settings.
func New(env provider.Env, _ json.RawMessage) (provider.Provider, error) {
	s := site.NewBrowser(env.HTTP)
	s.Header.Set("Referer", serverURL+"/")
	return &Provider{env: env, s: s, base: serverURL, contents: map[string][]byte{}}, nil
}

// Provider is SubClub.eu.
type Provider struct {
	env  provider.Env
	s    *site.Session
	base string

	mu sync.Mutex
	// contents are the subtitles extracted from archives the site lists
	// no files of, by ID, as Bazarr fills their content while searching.
	contents map[string][]byte
}

// SetBaseURL points the provider at another server, for tests.
func (p *Provider) SetBaseURL(u *url.URL) {
	p.base = strings.TrimSuffix(u.String(), "/")
	p.s.Header.Set("Referer", p.base+"/")
}

// hit is a row of the site's search results.
type hit struct {
	archiveID, pageLink string
	title               string
	year                int
	// season and episode are 0 for a movie.
	season, episode int
	imdbID          string
	rating          float64
	uploader        string
}

// parseTitle parses an anchor's text.
func parseTitle(text string) (title string, year, season, episode int, ok bool) {
	m := titleRe.FindStringSubmatch(text)
	if m == nil {
		return "", 0, 0, 0, false
	}
	year, _ = strconv.Atoi(m[2])
	season, _ = strconv.Atoi(m[3])
	episode, _ = strconv.Atoi(m[4])
	return strings.TrimSpace(m[1]), year, season, episode, true
}

// parseFloat reads a number with a decimal comma, as Bazarr's
// _parse_float; 0 for none.
func parseFloat(text string) float64 {
	fields := strings.Fields(strings.ReplaceAll(text, ",", "."))
	if len(fields) == 0 {
		return 0
	}
	f, _ := strconv.ParseFloat(fields[0], 64)
	return f
}

// search lists the archives of a title, the best rated first, as Bazarr's
// _search.
func (p *Provider) search(ctx context.Context, title string) ([]hit, error) {
	r, err := p.s.Get(ctx, p.base+"/jutud.php", url.Values{"otsing": {title}})
	if err != nil {
		return nil, err
	}
	doc, err := site.Parse(bytes.ToValidUTF8(r.Body, nil), "text/html; charset=utf-8")
	if err != nil {
		return nil, provider.Errorf(provider.ParseResponseError, "search: %w", err)
	}
	table := doc.First("table#tale_list")
	if !table.Valid() {
		return nil, nil
	}
	var hits []hit
	for _, row := range table.All("tbody > tr") {
		var tds []site.Node
		for _, c := range row.Children() {
			if c.Data == "td" {
				tds = append(tds, c)
			}
		}
		if len(tds) < 9 {
			continue
		}
		var link site.Node
		for _, a := range tds[1].All("a.sc_link") {
			if downRe.MatchString(a.Attr("href")) {
				link = a
				break
			}
		}
		if !link.Valid() {
			continue
		}
		id := downRe.FindStringSubmatch(link.Attr("href"))[1]
		text := scrape.Words(link)
		title, year, season, episode, ok := parseTitle(text)
		if !ok {
			p.env.Log.DebugContext(ctx, "subclub: skipping unparsable title", "title", text)
			continue
		}
		h := hit{
			archiveID: id, pageLink: p.base + "/down.php?id=" + id,
			title: title, year: year, season: season, episode: episode,
			uploader: strings.TrimSpace(tds[8].Text()),
		}
		for _, a := range tds[3].All("a[href]") {
			if m := imdbRe.FindString(a.Attr("href")); m != "" {
				h.imdbID = m
			}
			break
		}
		rating := tds[7].Text()
		if span := tds[7].First("span"); span.Valid() {
			rating = span.Text()
		}
		h.rating = parseFloat(rating)
		hits = append(hits, h)
	}
	slices.SortStableFunc(hits, func(a, b hit) int { return cmp.Compare(b.rating, a.rating) })
	return hits, nil
}

// file is a subtitle of an archive.
type file struct {
	name, link string
	data       []byte
}

// archiveFiles lists the subtitles of an archive with their links, as
// Bazarr's _archive_files; none when the site lists none.
func (p *Provider) archiveFiles(ctx context.Context, id string) ([]file, error) {
	r, err := p.s.Get(ctx, p.base+"/subtitles_archivecontent.php", url.Values{"id": {id}})
	if err != nil {
		return nil, err
	}
	doc, err := site.Parse(bytes.ToValidUTF8(r.Body, nil), "text/html; charset=utf-8")
	if err != nil {
		return nil, provider.Errorf(provider.ParseResponseError, "archive content: %w", err)
	}
	var files []file
	for _, a := range doc.All(`a[href*="down.php"]`) {
		href := a.Attr("href")
		if !strings.Contains(href, "filename=") {
			continue
		}
		name := a.TrimmedText()
		if !scrape.IsSubtitle(name) {
			continue
		}
		// The site links "../down.php?id=N&filename=B64".
		files = append(files, file{name: name, link: p.base + "/" + strings.TrimLeft(href, "./")})
	}
	return files, nil
}

// extract downloads an archive and returns its subtitles, as Bazarr's
// _extract_archive: zip and RAR only.
func (p *Provider) extract(ctx context.Context, id string) ([]file, error) {
	r, err := p.s.Get(ctx, p.base+"/down.php", url.Values{"id": {id}})
	if err != nil {
		return nil, err
	}
	if !isZipOrRAR(r.Body) {
		return nil, provider.Errorf(provider.ProviderError, "subclub: unsupported archive format for id=%s", id)
	}
	a, err := archive.Open(r.Body)
	if err != nil {
		return nil, provider.Errorf(provider.ProviderError, "archive %s: %w", id, err)
	}
	var out []file
	for _, name := range a.Names() {
		base := path.Base(name)
		if strings.HasSuffix(name, "/") || base == "" || strings.HasPrefix(base, ".") || !scrape.IsSubtitle(base) {
			continue
		}
		data, err := a.Read(name)
		if err != nil {
			p.env.Log.WarnContext(ctx, "subclub: failed to read a file of an archive", "file", name, "archive", id, "error", err)
			continue
		}
		out = append(out, file{name: base, data: data})
	}
	return out, nil
}

func isZipOrRAR(data []byte) bool {
	if bytes.HasPrefix(data, []byte("Rar!\x1a\x07")) {
		return true
	}
	_, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	return err == nil
}

// matchesVideo reports whether a hit is of the video, as Bazarr's
// _hit_matches_video.
func matchesVideo(h hit, v *provider.Video) bool {
	if v.Episode {
		switch {
		case h.season == 0 || h.episode == 0,
			v.Season != 0 && h.season != v.Season,
			v.Number != 0 && h.episode != v.Number,
			v.Series != "" && scrape.Sanitize(h.title, "") != scrape.Sanitize(v.Series, ""):
			return false
		}
		return true
	}
	switch {
	case h.season != 0 || h.episode != 0,
		v.Title != "" && scrape.Sanitize(h.title, "") != scrape.Sanitize(v.Title, ""),
		v.Year != 0 && h.year != v.Year:
		return false
	}
	return true
}

// matches are what a hit matches of the video, as Bazarr's get_matches
// knows from the site; the file name's are guessed by the server.
func matches(h hit, v *provider.Video) []string {
	var out []string
	if v.Episode {
		if v.Series != "" && scrape.Sanitize(h.title, "") == scrape.Sanitize(v.Series, "") {
			out = append(out, "series")
		}
		if h.season == v.Season {
			out = append(out, "season")
		}
		if h.episode == v.Number {
			out = append(out, "episode")
		}
	} else if v.Title != "" && scrape.Sanitize(h.title, "") == scrape.Sanitize(v.Title, "") {
		out = append(out, "title")
	}
	if v.Year != 0 && h.year == v.Year {
		out = append(out, "year")
	}
	if h.imdbID != "" {
		if !v.Episode && v.ExternalIDs["imdb"] == h.imdbID {
			out = append(out, "imdb_id")
		} else if v.Episode && v.SeriesExternalIDs["imdb"] == h.imdbID {
			out = append(out, "series_imdb_id")
		}
	}
	return out
}

// Search implements provider.Provider: the archives of the video's titles,
// each subtitle file of them a subtitle, as Bazarr's query. Failing
// searches and archives are skipped, as Bazarr does.
func (p *Provider) Search(ctx context.Context, v *provider.Video, langs []provider.Language) ([]provider.Subtitle, error) {
	if !slices.ContainsFunc(langs, func(l provider.Language) bool { return provider.Alpha3(l.Tag) == "est" }) {
		return nil, nil
	}
	titles := append([]string{v.Title}, v.AlternativeTitles...)
	if v.Episode {
		titles = append([]string{v.Series}, v.AlternativeSeries...)
	}
	seen := map[string]bool{}
	var out []provider.Subtitle
	for _, title := range titles {
		if title == "" {
			continue
		}
		hits, err := p.search(ctx, title)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			p.env.Log.WarnContext(ctx, "subclub: search failed", "title", title, "error", err)
			continue
		}
		for _, h := range hits {
			if seen[h.archiveID] || !matchesVideo(h, v) {
				continue
			}
			seen[h.archiveID] = true
			files, err := p.archiveFiles(ctx, h.archiveID)
			if err == nil && len(files) == 0 {
				files, err = p.extract(ctx, h.archiveID)
			}
			if err != nil {
				if ctx.Err() != nil {
					return nil, ctx.Err()
				}
				p.env.Log.WarnContext(ctx, "subclub: archive failed", "id", h.archiveID, "error", err)
				continue
			}
			for _, f := range files {
				id := h.archiveID + "/" + f.name
				if f.data != nil {
					p.mu.Lock()
					p.contents[id] = archive.FixLineEndings(f.data)
					p.mu.Unlock()
				}
				out = append(out, provider.Subtitle{
					ID:           id,
					Language:     provider.Language{Tag: "et"},
					ReleaseNames: []string{f.name},
					Matches:      matches(h, v),
					Format:       format(f.name, f.data),
					PageLink:     h.pageLink,
					Uploader:     h.uploader,
				})
			}
		}
	}
	return out, nil
}

func format(name string, data []byte) string {
	f := strings.TrimPrefix(strings.ToLower(path.Ext(name)), ".")
	switch f {
	case "srt", "ass", "ssa", "vtt":
		return f
	}
	if data != nil {
		return archive.Detect(data)
	}
	return ""
}

// Download implements provider.Provider: the content extracted while
// searching, else the file the archive's listing links, else the file
// extracted from the archive.
func (p *Provider) Download(ctx context.Context, _ *provider.Video, id string) (provider.File, error) {
	archiveID, name, ok := strings.Cut(id, "/")
	if !ok {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "malformed subtitle ID %q", id)
	}
	p.mu.Lock()
	data, ok := p.contents[id]
	p.mu.Unlock()
	if !ok {
		var err error
		if data, err = p.fetch(ctx, archiveID, name); err != nil {
			return provider.File{}, err
		}
	}
	return provider.File{Data: data, Format: format(name, data), Language: provider.Language{Tag: "et"}}, nil
}

// fetch downloads a file of an archive.
func (p *Provider) fetch(ctx context.Context, archiveID, name string) ([]byte, error) {
	files, err := p.archiveFiles(ctx, archiveID)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		if files, err = p.extract(ctx, archiveID); err != nil {
			return nil, err
		}
	}
	i := slices.IndexFunc(files, func(f file) bool { return f.name == name })
	switch {
	case i < 0:
		return nil, provider.Errorf(provider.BadSubtitle, "subclub: no file %s in archive %s", name, archiveID)
	case files[i].data != nil:
		return archive.FixLineEndings(files[i].data), nil
	}
	r, err := p.s.Get(ctx, files[i].link, nil)
	if err != nil {
		return nil, err
	}
	if len(r.Body) == 0 {
		return nil, provider.Errorf(provider.ProviderError, "subclub: empty response for id=%s file=%s", archiveID, name)
	}
	return archive.FixLineEndings(r.Body), nil
}
