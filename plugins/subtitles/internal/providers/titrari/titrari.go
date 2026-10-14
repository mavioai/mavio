// Package titrari is the provider of Titrari.ro, ported from Bazarr's
// titrari: Romanian and English subtitles of movies and episodes found
// by IMDb ID or title with the site's advanced search.
package titrari

import (
	"archive/zip"
	"bytes"
	"cmp"
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
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/archive"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/scrape"
	"github.com/mavioai/mavio/plugins/subtitles/internal/site"
)

// Name is the provider's name.
const Name = "titrari"

// serverURL is the site's.
const serverURL = "https://www.titrari.ro/"

// userAgent is the one Bazarr sends, which the site does not throttle
// for 30 seconds.
const userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36"

// defaultSearchPage is the advanced search's page when the home page does
// not name it.
const defaultSearchPage = "numaicautamcaneiesepenas"

// searchPageExpiration is how long the advanced search's page is kept.
const searchPageExpiration = 24 * time.Hour

// pause is the wait after a search, which spares the site's blocking.
const pause = 5 * time.Second

// Definition returns the provider's definition.
func Definition() provider.Definition {
	return provider.Definition{
		Name: Name, DisplayName: "Titrari.ro",
		Languages: []string{"ro", "en"},
		Movies:    true, Episodes: true,
		Throttle: map[provider.Kind]func(time.Time) time.Duration{
			provider.TooManyRequests: provider.Fixed(10 * time.Minute),
		},
		New: New,
	}
}

// New makes the provider; it has no settings.
func New(env provider.Env, _ json.RawMessage) (provider.Provider, error) {
	base, _ := url.Parse(serverURL)
	s := site.New(env.HTTP)
	s.Header.Set("User-Agent", userAgent)
	s.Check = func(r *site.Response) error {
		if r.Status == http.StatusNotFound && bytes.Contains(r.Body, []byte("Too many requests")) {
			return provider.Errorf(provider.TooManyRequests, "%s", r.Body)
		}
		return site.Status(r)
	}
	return &Provider{env: env, s: s, base: base, pause: pause}, nil
}

// Provider is Titrari.ro.
type Provider struct {
	env   provider.Env
	s     *site.Session
	base  *url.URL
	pause time.Duration

	mu sync.Mutex
	// searchPage is the advanced search's page, as Bazarr's
	// _cached_page_param.
	searchPage  string
	searchUntil time.Time
}

// SetBaseURL points the provider at another server, for tests.
func (p *Provider) SetBaseURL(u *url.URL) { p.base = u }

func (p *Provider) resolve(ref string) (string, error) {
	r, err := url.Parse(ref)
	if err != nil {
		return "", provider.Errorf(provider.ParseResponseError, "link %q: %w", ref, err)
	}
	return p.base.ResolveReference(r).String(), nil
}

var pageParam = regexp.MustCompile(`[?&]page=([^&]+)`)

// advancedSearchPage returns the page of the home page's "Cautare
// Avansata" link, kept a day, as Bazarr's
// _get_advanced_search_page_param; the default one when it cannot be
// read.
func (p *Provider) advancedSearchPage(ctx context.Context) string {
	now := p.env.Now()
	p.mu.Lock()
	page, until := p.searchPage, p.searchUntil
	p.mu.Unlock()
	if page != "" && now.Before(until) {
		return page
	}
	r, err := p.s.Get(ctx, p.base.String(), nil)
	if err != nil {
		p.env.Log.ErrorContext(ctx, "fetch the advanced search page", "provider", Name, "err", err)
		return defaultSearchPage
	}
	doc, err := r.HTML()
	if err != nil {
		p.env.Log.ErrorContext(ctx, "parse the home page", "provider", Name, "err", err)
		return defaultSearchPage
	}
	link := ""
	for _, a := range doc.All("a[href]") {
		if text := strings.ToLower(a.Text()); strings.Contains(text, "cautare avansata") || strings.Contains(text, "cautare avansată") {
			link = a.Attr("href")
			break
		}
	}
	m := pageParam.FindStringSubmatch(link)
	if m == nil {
		p.env.Log.WarnContext(ctx, "no advanced search link", "provider", Name, "link", link)
		return defaultSearchPage
	}
	p.mu.Lock()
	p.searchPage, p.searchUntil = m[1], now.Add(searchPageExpiration)
	p.mu.Unlock()
	return m[1]
}

// Search implements provider.Provider, as Bazarr's list_subtitles: each
// language is searched by the IMDb ID of the movie or series, else by
// the video's title.
func (p *Provider) Search(ctx context.Context, v *provider.Video, langs []provider.Language) ([]provider.Subtitle, error) {
	title := fixNaming(v.Title)
	imdbID := v.IMDbID()
	if len(imdbID) > 2 {
		imdbID = imdbID[2:]
	} else {
		imdbID = ""
	}
	var out []provider.Subtitle
	for _, l := range langs {
		subs, err := p.query(ctx, v, l, title, imdbID)
		if err != nil {
			return nil, err
		}
		out = append(out, subs...)
	}
	return out, nil
}

// query searches subtitles in a language, the most downloaded first.
func (p *Provider) query(ctx context.Context, v *provider.Video, l provider.Language, title, imdbID string) ([]provider.Subtitle, error) {
	// The site requires every parameter: z2 is the comment, z3 the frame
	// rate, z4 the number of CDs, z5 the IMDb ID, z6 the order, z7 the
	// title, z8 the language, z9 the genre and z11 the type.
	params := url.Values{
		"page": {p.advancedSearchPage(ctx)},
		"z7":   {""}, "z2": {""}, "z5": {""}, "z3": {"-1"}, "z4": {"-1"}, "z8": {"-1"}, "z9": {"All"}, "z11": {"0"}, "z6": {"0"},
	}
	if imdbID != "" {
		params.Set("z5", imdbID)
	} else {
		params.Set("z7", title)
	}
	// Forced languages are searched in all languages, as Bazarr compares
	// them to "ro" and "en".
	switch {
	case l.Tag == "ro" && !l.Forced:
		params.Set("z8", "1")
	case l.Tag == "en" && !l.Forced:
		params.Set("z8", "2")
	}
	r, err := p.s.Get(ctx, p.base.String(), params)
	if err != nil {
		return nil, err
	}
	if len(r.Body) == 0 {
		p.env.Log.DebugContext(ctx, "no data returned from provider", "provider", Name)
		return nil, nil
	}
	doc, err := r.HTML()
	if err != nil {
		return nil, provider.Errorf(provider.ParseResponseError, "search: %w", err)
	}
	type counted struct {
		sub       provider.Subtitle
		downloads int
	}
	var found []counted
	for index, row := range doc.All(`td[rowspan="5"]`) {
		s, err := p.parseRow(ctx, v, l, row, index)
		if err != nil {
			return nil, err
		}
		found = append(found, counted{s, s.Downloads})
	}
	slices.SortStableFunc(found, func(a, b counted) int { return cmp.Compare(b.downloads, a.downloads) })
	out := make([]provider.Subtitle, len(found))
	for i, f := range found {
		out[i] = f.sub
	}
	if err := site.Sleep(ctx, p.pause); err != nil {
		return nil, err
	}
	return out, nil
}

// parseRow reads a result, as Bazarr's query: what cannot be read is
// left out but the download link.
func (p *Provider) parseRow(ctx context.Context, v *provider.Video, l provider.Language, row site.Node, index int) (provider.Subtitle, error) {
	a := row.First("a")
	if !a.Valid() {
		return provider.Subtitle{}, provider.Errorf(provider.ParseResponseError, "search: a result without a download link")
	}
	parent, table := row.Parent(), row.Parent().Parent()
	heading := parent.First("h1 a")
	fullTitle := heading.Text()
	title, rest, _ := strings.Cut(fullTitle, "(")
	year := 0
	if y, _, ok := strings.Cut(rest, ")"); ok {
		year, _ = strconv.Atoi(strings.TrimSpace(y))
	}
	if year == 0 {
		p.env.Log.DebugContext(ctx, "no year", "provider", Name, "title", fullTitle)
	}
	downloads := 0
	if spans := table.All("span"); index < len(spans) {
		if text := []rune(spans[index].Text()); len(text) > 12 {
			downloads, _ = strconv.Atoi(strings.TrimSpace(string(text[12:])))
		}
	}
	imdbID := ""
	for _, img := range table.All("[src]") {
		if strings.Contains(img.Attr("src"), "imdb") {
			parts := strings.Split(img.Parent().Attr("href"), "tt")
			imdbID = "tt" + parts[len(parts)-1]
			break
		}
	}
	comments := ""
	if cs := table.All(".comment"); index*2+1 < len(cs) {
		comments = cs[index*2+1].Text()
	}
	pageLink := ""
	if heading.Valid() {
		pageLink, _ = p.resolve(heading.Attr("href"))
	}
	uploader := ""
	if us := parent.All("td.row1.stanga a"); len(us) > 0 {
		uploader = us[len(us)-1].Text()
	}
	return provider.Subtitle{
		ID:           a.Attr("href"),
		Language:     l,
		ReleaseNames: []string{comments},
		Matches:      matches(v, title, imdbID, comments, year),
		PageLink:     pageLink,
		Uploader:     uploader,
		Comment:      comments,
		Downloads:    downloads,
	}, nil
}

var seasonSuffix = regexp.MustCompile(`\s-\sSezonul\s\d+$`)

// matches are what a result matches, as Bazarr's
// TitrariSubtitle.get_matches; an episode is matched when its series'
// IMDb ID and season are.
func matches(v *provider.Video, title, imdbID, comments string, year int) []string {
	var out []string
	if v.Year != 0 && year == v.Year {
		out = append(out, "year")
	}
	if v.ReleaseGroup != "" && strings.Contains(comments, v.ReleaseGroup) {
		out = append(out, "release_group")
	}
	if !v.Episode {
		if v.Title != "" && scrape.SanitizePatch(title, "") == fixNaming(v.Title) {
			out = append(out, "title")
		}
		if id := v.ExternalIDs["imdb"]; id != "" && imdbID == id {
			out = append(out, "imdb_id")
		}
		return out
	}
	if v.Series != "" && fixNaming(v.Series) == scrape.SanitizePatch(seasonSuffix.ReplaceAllString(strings.TrimRight(title, " \t\r\n"), ""), "") {
		out = append(out, "series")
	}
	if id := v.SeriesExternalIDs["imdb"]; id != "" && imdbID == id {
		out = append(out, "imdb_id")
	}
	if strings.Contains(title, fmt.Sprintf("Sezonul %d", v.Season)) {
		out = append(out, "season")
	}
	if slices.Contains(out, "imdb_id") && slices.Contains(out, "season") {
		out = append(out, "episode")
	}
	return out
}

// subtitleExtensions are subliminal's SUBTITLE_EXTENSIONS.
var subtitleExtensions = []string{".srt", ".sub", ".smi", ".txt", ".ssa", ".ass", ".mpl"}

// Download implements provider.Provider: the subtitle of the archive,
// that of the episode for an episode, or the subtitle file served.
func (p *Provider) Download(ctx context.Context, v *provider.Video, id string) (provider.File, error) {
	link, err := p.resolve(id)
	if err != nil {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "malformed subtitle ID %q", id)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
	if err != nil {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "malformed subtitle ID %q", id)
	}
	req.Header.Set("Referer", p.base.String())
	r, err := p.s.Do(req)
	if err != nil {
		return provider.File{}, err
	}
	a, err := archive.Open(r.Body)
	if errors.Is(err, archive.ErrNotArchive) {
		return provider.File{}, provider.Errorf(provider.ProviderError, "unidentified archive type")
	}
	if err != nil || v == nil || !v.Episode {
		return archive.Extract(r.Body, archive.WantFor(v, provider.Language{}), provider.Language{})
	}
	if format := archive.Detect(r.Body); format != "" && !isArchive(r.Body) {
		return provider.File{Data: archive.FixLineEndings(r.Body), Format: format}, nil
	}
	// An episode's subtitle is the one named with its number, as Bazarr's
	// _get_subtitle_from_archive.
	for _, name := range a.Names() {
		lower := strings.ToLower(name)
		if strings.HasPrefix(path.Base(name), ".") || !slices.ContainsFunc(subtitleExtensions, func(ext string) bool { return strings.HasSuffix(lower, ext) }) {
			continue
		}
		if n, ok := archive.EpisodeNumber(name); ok && n == v.Number {
			data, err := a.Read(name)
			if err != nil {
				return provider.File{}, provider.Errorf(provider.BadSubtitle, "%s: %w", name, err)
			}
			data = archive.FixLineEndings(data)
			format := strings.TrimPrefix(path.Ext(lower), ".")
			if detected := archive.Detect(data); detected != "" {
				format = detected
			}
			return provider.File{Data: data, Format: format}, nil
		}
	}
	return provider.File{}, provider.Errorf(provider.BadSubtitle, "no subtitle of episode %d in the archive", v.Number)
}

// isArchive reports whether data is a RAR, 7z or zip archive rather than
// a subtitle file.
func isArchive(data []byte) bool {
	if bytes.HasPrefix(data, []byte("Rar!\x1a\x07")) || bytes.HasPrefix(data, []byte("7z\xbc\xaf\x27\x1c")) {
		return true
	}
	_, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	return err == nil
}

// inconsistent are the titles the site names otherwise.
var inconsistent = strings.NewReplacer("DC's Legends of Tomorrow", "Legends of Tomorrow", "Marvel's Jessica Jones", "Jessica Jones")

// fixNaming renames the titles the site names otherwise and sanitizes
// them, as Bazarr's fix_inconsistent_naming.
func fixNaming(title string) string { return scrape.SanitizePatch(inconsistent.Replace(title), "") }

// String names the provider in logs.
func (p *Provider) String() string { return fmt.Sprintf("%s(%s)", Name, p.base.Host) }
