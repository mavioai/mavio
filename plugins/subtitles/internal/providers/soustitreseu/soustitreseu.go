// Package soustitreseu is the provider of Sous-Titres.eu, ported from
// Bazarr's soustitreseu: French and English subtitles of series and
// movies, served in archives of a season, an episode or a movie, whose
// files are listed as subtitles.
package soustitreseu

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/mavioai/mavio/plugins/subtitles/internal/archive"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/site"
)

// Name is the provider's name.
const Name = "soustitreseu"

// utf8 is how Bazarr decodes the site's pages.
const utf8 = "text/html; charset=utf-8"

// extensions are subliminal's SUBTITLE_EXTENSIONS but .txt, which some
// archives have files of that are no subtitles.
var extensions = []string{".srt", ".sub", ".smi", ".ssa", ".ass", ".mpl"}

// Definition returns the provider's definition.
func Definition() provider.Definition {
	return provider.Definition{
		Name: Name, DisplayName: "Sous-Titres.eu",
		Languages: []string{"fr", "en"},
		Movies:    true, Episodes: true,
		New: New,
	}
}

// New makes the provider; it has no settings.
func New(env provider.Env, _ json.RawMessage) (provider.Provider, error) {
	p := &Provider{env: env, s: site.New(env.HTTP)}
	p.SetBaseURL(&url.URL{Scheme: "https", Host: "www.sous-titres.eu"})
	return p, nil
}

// Provider is Sous-Titres.eu.
type Provider struct {
	env    provider.Env
	s      *site.Session
	server string
}

// SetBaseURL points the provider at another server, for tests.
func (p *Provider) SetBaseURL(u *url.URL) {
	p.server = strings.TrimSuffix(u.String(), "/") + "/"
	p.s.Header.Set("Referer", p.server)
}

// Search implements provider.Provider.
func (p *Provider) Search(ctx context.Context, v *provider.Video, langs []provider.Language) ([]provider.Subtitle, error) {
	var subs []provider.Subtitle
	var err error
	if v.Episode {
		subs, err = p.querySeries(ctx, v)
	} else {
		subs, err = p.queryMovies(ctx, v)
	}
	if err != nil {
		return nil, err
	}
	var out []provider.Subtitle
	for _, s := range subs {
		if len(langs) == 0 || slices.ContainsFunc(langs, func(l provider.Language) bool { return l.Tag == s.Language.Tag }) {
			out = append(out, s)
		}
	}
	return out, nil
}

// pages returns the links of the search's results whose titles have the
// title.
func (p *Provider) pages(ctx context.Context, selector, title string, has func(text, title string) bool) ([]string, error) {
	r, err := p.s.Get(ctx, p.server+"search.html", url.Values{"q": {title}})
	if err != nil || len(r.Body) == 0 {
		return nil, err
	}
	doc, err := site.Parse(r.Body, utf8)
	if err != nil {
		return nil, provider.Errorf(provider.ParseResponseError, "search: %w", err)
	}
	var out []string
	for _, a := range doc.All(selector) {
		if has(a.Text(), title) {
			out = append(out, a.Attr("href"))
		}
	}
	return out, nil
}

// archives returns the archives of a page of results.
func (p *Provider) archives(ctx context.Context, page string) ([]site.Node, error) {
	r, err := p.s.Get(ctx, p.server+page, nil)
	if err != nil || len(r.Body) == 0 {
		return nil, err
	}
	doc, err := site.Parse(r.Body, utf8)
	if err != nil {
		return nil, provider.Errorf(provider.ParseResponseError, "%s: %w", page, err)
	}
	return doc.All("a.subList"), nil
}

// querySeries lists the subtitles of an episode, as Bazarr's query_series:
// those of the archives of its season or of itself on the pages of the
// series found.
func (p *Provider) querySeries(ctx context.Context, v *provider.Video) ([]provider.Subtitle, error) {
	pages, err := p.pages(ctx, ".serie > h3 > a", v.Series, strings.Contains)
	if err != nil {
		return nil, err
	}
	var out []provider.Subtitle
	for _, page := range pages {
		items, err := p.archives(ctx, page)
		if err != nil {
			return nil, err
		}
		for _, a := range items {
			href := a.Attr("href")
			season, number, ok := archiveEpisode(href, a.First(".episodenum").Text())
			if !ok || season != v.Season || number != 0 && number != v.Number {
				continue
			}
			subs, err := p.fromArchive(ctx, v, p.server+"series/"+href, p.server+page)
			if err != nil {
				return nil, err
			}
			out = append(out, subs...)
		}
	}
	return out, nil
}

// queryMovies lists the subtitles of a movie, as Bazarr's query_movies:
// those of the archives on the pages of the movies found.
func (p *Provider) queryMovies(ctx context.Context, v *provider.Video) ([]provider.Subtitle, error) {
	pages, err := p.pages(ctx, ".film > h3 > a", v.Title, func(text, title string) bool {
		return strings.Contains(strings.ToLower(text), strings.ToLower(title))
	})
	if err != nil {
		return nil, err
	}
	var out []provider.Subtitle
	for _, page := range pages {
		items, err := p.archives(ctx, page)
		if err != nil {
			return nil, err
		}
		for _, a := range items {
			subs, err := p.fromArchive(ctx, v, p.server+"films/"+a.Attr("href"), p.server+page)
			if err != nil {
				return nil, err
			}
			out = append(out, subs...)
		}
	}
	return out, nil
}

// archiveEpisode reads the season and episode of an archive from its
// episode number, "1×02" or "S1", and else its name; the episode is 0 for
// an archive of a season.
func archiveEpisode(href, num string) (season, number int, ok bool) {
	num = strings.TrimSpace(num)
	if s, e, found := strings.Cut(num, "×"); found {
		season, err1 := strconv.Atoi(strings.TrimSpace(s))
		number, err2 := strconv.Atoi(strings.TrimSpace(e))
		if err1 == nil && err2 == nil {
			return season, number, true
		}
	}
	if len([]rune(num)) < 2 {
		return 0, 0, false
	}
	season, err := strconv.Atoi(string([]rune(num)[1:]))
	if err != nil {
		return 0, 0, false
	}
	name := path.Base(href)
	if unescaped, err := url.PathUnescape(name); err == nil {
		name = unescaped
	}
	_, number, _ = seasonEpisode(strings.TrimSuffix(name, path.Ext(name)))
	return season, number, true
}

var (
	sxe        = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])s(\d{1,2})[\s._-]*e(\d{1,3})`)
	xNumbers   = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])(\d{1,2})x(\d{1,3})(?:[^0-9]|$)`)
	threeDigit = regexp.MustCompile(`(?:^|[\s._-])(\d)(\d{2})(?:[\s._-]|$)`)
)

// seasonEpisode reads the season and episode of a file's name, as guessit
// reads them: from "S01E02", "1x02" or "102".
func seasonEpisode(name string) (season, number int, ok bool) {
	for _, re := range []*regexp.Regexp{sxe, xNumbers, threeDigit} {
		if m := re.FindStringSubmatch(name); m != nil {
			season, _ = strconv.Atoi(m[1])
			number, _ = strconv.Atoi(m[2])
			return season, number, true
		}
	}
	return 0, 0, false
}

// fromArchive lists the subtitles of an archive, as Bazarr's
// _get_subtitle_from_archive: its subtitle files, in English when named
// so and else in French; an episode's must be named for it.
func (p *Provider) fromArchive(ctx context.Context, v *provider.Video, link, page string) ([]provider.Subtitle, error) {
	r, err := p.s.Get(ctx, link, nil)
	if err != nil {
		return nil, err
	}
	// As Bazarr's, the archives that cannot be opened are skipped.
	var names []string
	if bytes.HasPrefix(r.Body, []byte("PK")) || bytes.HasPrefix(r.Body, []byte("Rar!")) {
		if a, err := archive.Open(r.Body); err == nil {
			names = a.Names()
		}
	}
	var out []provider.Subtitle
	for _, name := range names {
		base := path.Base(name)
		if strings.HasPrefix(base, ".") || !slices.Contains(extensions, strings.ToLower(path.Ext(name))) {
			continue
		}
		tag := "fr"
		if strings.Contains(strings.ToLower(name), ".en.") {
			tag = "en"
		}
		release := strings.TrimRight(strings.TrimRight(strings.TrimRight(strings.ToLower(strings.TrimSuffix(name, path.Ext(name))), "tag"), "en"), "fr")
		if v.Episode {
			season, number, ok := seasonEpisode(release)
			if !ok || season != v.Season || number != v.Number {
				continue
			}
		}
		// The archive is of a series or movie whose name has the video's.
		matches := []string{"title"}
		if v.Episode {
			matches = []string{"series"}
		}
		out = append(out, provider.Subtitle{
			ID:           url.Values{"u": {link}, "f": {name}, "lang": {tag}}.Encode(),
			Language:     provider.Language{Tag: tag},
			ReleaseNames: []string{name},
			Matches:      matches,
			PageLink:     page,
		})
	}
	return out, nil
}

// Download implements provider.Provider: the subtitle file of the archive
// Search found it in.
func (p *Provider) Download(ctx context.Context, _ *provider.Video, id string) (provider.File, error) {
	q, err := url.ParseQuery(id)
	if err != nil || !strings.HasPrefix(q.Get("u"), p.server) || q.Get("f") == "" {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "malformed subtitle ID %q", id)
	}
	r, err := p.s.Get(ctx, q.Get("u"), nil)
	if err != nil {
		return provider.File{}, err
	}
	a, err := archive.Open(r.Body)
	if err != nil {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "%w", err)
	}
	data, err := a.Read(q.Get("f"))
	if err != nil {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "%w", err)
	}
	format := strings.TrimPrefix(strings.ToLower(path.Ext(q.Get("f"))), ".")
	if !slices.Contains([]string{"srt", "ass", "ssa"}, format) {
		format = archive.Detect(data)
	}
	return provider.File{Data: archive.FixLineEndings(data), Format: format, Language: provider.Language{Tag: q.Get("lang")}}, nil
}
