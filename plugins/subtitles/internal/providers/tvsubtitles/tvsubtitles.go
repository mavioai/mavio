// Package tvsubtitles is the provider of TVsubtitles.net, ported from
// Bazarr's tvsubtitles (and subliminal's it subclasses): subtitles of
// episodes found through the pages of the show's season and episode.
package tvsubtitles

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/archive"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/site"
)

// Name is the provider's name.
const Name = "tvsubtitles"

// serverURL is the site's.
const serverURL = "http://www.tvsubtitles.net/"

// How long shows' and episodes' IDs are kept, as subliminal's
// SHOW_EXPIRATION_TIME and EPISODE_EXPIRATION_TIME.
const (
	showExpiration    = 3 * 7 * 24 * time.Hour
	episodeExpiration = 3 * 24 * time.Hour
)

// maxFile bounds the subtitle read from the zip.
const maxFile = 10 << 20

// Definition returns the provider's definition.
func Definition() provider.Definition {
	return provider.Definition{
		Name: Name, DisplayName: "TVSubtitles",
		Languages: []string{
			"pt-BR", "ar", "bg", "cs", "da", "de", "el", "en", "fi", "fr", "hu", "it", "ja", "ko", "nl", "pl", "pt",
			"ro", "ru", "es", "sv", "tr", "uk", "zh",
		},
		Episodes: true,
		New:      New,
	}
}

// New makes the provider; it has no settings.
func New(env provider.Env, _ json.RawMessage) (provider.Provider, error) {
	base, _ := url.Parse(serverURL)
	return &Provider{
		env: env, s: site.New(env.HTTP), base: base,
		shows: map[string]cached[int]{}, episodes: map[string]cached[map[int]int]{},
	}, nil
}

// Provider is TVsubtitles.net.
type Provider struct {
	env  provider.Env
	s    *site.Session
	base *url.URL

	mu sync.Mutex
	// shows and episodes cache the IDs of shows and of the episodes of a
	// season, as Bazarr's region cache.
	shows    map[string]cached[int]
	episodes map[string]cached[map[int]int]
}

type cached[T any] struct {
	v     T
	until time.Time
}

// SetBaseURL points the provider at another server, for tests.
func (p *Provider) SetBaseURL(u *url.URL) { p.base = u }

func (p *Provider) link(path string) string {
	return p.base.ResolveReference(&url.URL{Path: path}).String()
}

// fromSite maps the site's language codes, as subliminal's
// TVsubtitlesConverter.
func fromSite(code string) string {
	switch code {
	case "br":
		return "pt-BR"
	case "ua":
		return "uk"
	case "gr":
		return "el"
	case "cn":
		return "zh"
	case "jp":
		return "ja"
	case "cz":
		return "cs"
	}
	return provider.FromCode(code)
}

var (
	// linkRE is a show's name in the search's suggestions.
	linkRE = regexp.MustCompile(`^(.+?)(?: \(?\d{4}\)?| \((?:US|UK)\))? \((\d{4})-\d{4}\)$`)
	// episodeIDRE is the link of an episode's page.
	episodeIDRE = regexp.MustCompile(`^episode-\d+\.html$`)
	// linkPartRE are the parts of the download link the page's script
	// joins.
	linkPartRE = regexp.MustCompile(`s\d=\s'(.*?)';\n`)
)

// Search implements provider.Provider, as Bazarr's list_subtitles: the
// first of the series' names the site has a show of.
func (p *Provider) Search(ctx context.Context, v *provider.Video, langs []provider.Language) ([]provider.Subtitle, error) {
	if !v.Episode {
		return nil, nil
	}
	showID, title := 0, ""
	for _, t := range append([]string{v.Series}, v.AlternativeSeries...) {
		id, err := p.searchShowID(ctx, t, v.Year)
		if err != nil {
			return nil, err
		}
		if id != 0 {
			showID, title = id, t
			break
		}
	}
	if showID == 0 {
		p.env.Log.DebugContext(ctx, "no show id found", "provider", Name, "series", v.Series, "year", v.Year)
		return nil, nil
	}
	subs, err := p.query(ctx, v, showID, title)
	if err != nil {
		return nil, err
	}
	var out []provider.Subtitle
	for _, s := range subs {
		if slices.ContainsFunc(langs, func(l provider.Language) bool { return l.Tag == s.Language.Tag && !l.Forced }) {
			out = append(out, s)
		}
	}
	return out, nil
}

// searchShowID returns the site's ID of a show of the series' name and
// first year, if any; 0 for none.
func (p *Provider) searchShowID(ctx context.Context, series string, year int) (int, error) {
	key := fmt.Sprintf("%s\x00%d", series, year)
	p.mu.Lock()
	c, ok := p.shows[key]
	p.mu.Unlock()
	if ok && p.env.Now().Before(c.until) {
		return c.v, nil
	}
	r, err := p.s.PostForm(ctx, p.link("search1.php"), url.Values{"qs": {series}})
	if err != nil {
		return 0, err
	}
	doc, err := r.HTML()
	if err != nil {
		return 0, provider.Errorf(provider.ParseResponseError, "search: %w", err)
	}
	id := 0
	for _, a := range doc.All(`div.left li div a[href^="/tvshow-"]`) {
		m := linkRE.FindStringSubmatch(a.Text())
		if m == nil {
			p.env.Log.ErrorContext(ctx, "failed to match a show", "provider", Name, "text", a.Text())
			continue
		}
		if sanitize(m[1]) != strings.ToLower(series) {
			continue
		}
		if first, _ := strconv.Atoi(m[2]); year != 0 && first != year {
			continue
		}
		href := a.Attr("href")
		if n, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(href, "/tvshow-"), ".html")); err == nil {
			id = n
			break
		}
	}
	p.mu.Lock()
	p.shows[key] = cached[int]{id, p.env.Now().Add(showExpiration)}
	p.mu.Unlock()
	return id, nil
}

// episodeIDs returns the site's IDs of the episodes of a season by their
// numbers.
func (p *Provider) episodeIDs(ctx context.Context, showID, season int) (map[int]int, error) {
	key := fmt.Sprintf("%d-%d", showID, season)
	p.mu.Lock()
	c, ok := p.episodes[key]
	p.mu.Unlock()
	if ok && p.env.Now().Before(c.until) {
		return c.v, nil
	}
	r, err := p.s.Get(ctx, p.link(fmt.Sprintf("tvshow-%d-%d.html", showID, season)), nil)
	if err != nil {
		return nil, err
	}
	doc, err := r.HTML()
	if err != nil {
		return nil, provider.Errorf(provider.ParseResponseError, "season: %w", err)
	}
	ids := map[int]int{}
	for _, row := range doc.All("table#table5 tr") {
		if !slices.ContainsFunc(row.All("a"), func(a site.Node) bool { return episodeIDRE.MatchString(a.Attr("href")) }) {
			continue
		}
		cells := row.All("td")
		if len(cells) < 2 {
			return nil, provider.Errorf(provider.ParseResponseError, "season: an episode's row without cells")
		}
		_, num, _ := strings.Cut(cells[0].Text(), "x")
		episode, err := strconv.Atoi(strings.TrimSpace(num))
		if err != nil {
			return nil, provider.Errorf(provider.ParseResponseError, "season: episode number %q", cells[0].Text())
		}
		href := cells[1].First("a").Attr("href")
		id, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(href, "episode-"), ".html"))
		if err != nil {
			return nil, provider.Errorf(provider.ParseResponseError, "season: episode link %q", href)
		}
		ids[episode] = id
	}
	if len(ids) == 0 {
		p.env.Log.WarnContext(ctx, "no episode ids found", "provider", Name, "show", showID, "season", season)
	}
	p.mu.Lock()
	p.episodes[key] = cached[map[int]int]{ids, p.env.Now().Add(episodeExpiration)}
	p.mu.Unlock()
	return ids, nil
}

// query lists the subtitles on the episode's page.
func (p *Provider) query(ctx context.Context, v *provider.Video, showID int, series string) ([]provider.Subtitle, error) {
	ids, err := p.episodeIDs(ctx, showID, v.Season)
	if err != nil {
		return nil, err
	}
	episodeID, ok := ids[v.Number]
	if !ok {
		p.env.Log.DebugContext(ctx, "episode not found", "provider", Name, "episode", v.Number)
		return nil, nil
	}
	r, err := p.s.Get(ctx, p.link(fmt.Sprintf("episode-%d.html", episodeID)), nil)
	if err != nil {
		return nil, err
	}
	doc, err := r.HTML()
	if err != nil {
		return nil, provider.Errorf(provider.ParseResponseError, "episode: %w", err)
	}
	var out []provider.Subtitle
	for _, row := range doc.All(".subtitlen") {
		src := row.First("h5 img").Attr("src")
		href := row.Parent().Attr("href")
		if len(src) < 17 || len(href) < 15 {
			return nil, provider.Errorf(provider.ParseResponseError, "episode: a subtitle without a flag or link")
		}
		id, err := strconv.Atoi(href[10 : len(href)-5])
		if err != nil {
			return nil, provider.Errorf(provider.ParseResponseError, "episode: subtitle link %q", href)
		}
		rip := strings.TrimSpace(row.First(`p[title="rip"]`).Text())
		release := strings.TrimSpace(row.First("h5").Text())
		out = append(out, provider.Subtitle{
			ID:           strconv.Itoa(id),
			Language:     provider.Language{Tag: fromSite(src[13 : len(src)-4])},
			ReleaseNames: nonEmpty(release, rip),
			Matches:      matches(v, series, release),
			PageLink:     p.link(fmt.Sprintf("subtitle-%d.html", id)),
		})
	}
	return out, nil
}

// matches are what the site's subtitle of the episode matches, as
// subliminal's TVsubtitlesSubtitle.get_matches: the subtitle is of the
// season and episode searched, and of the video's year.
func matches(v *provider.Video, series, release string) []string {
	var out []string
	if v.Series != "" && slices.Contains(sanitizeAll(append([]string{v.Series}, v.AlternativeSeries...)), sanitize(series)) {
		out = append(out, "series")
	}
	if v.Season != 0 {
		out = append(out, "season")
	}
	if v.Number != 0 {
		out = append(out, "episode")
	}
	if v.OriginalSeries && v.Year == 0 || v.Year != 0 {
		out = append(out, "year")
	}
	if v.ReleaseGroup != "" && release != "" {
		sanitized := sanitizeReleaseGroup(release)
		for _, g := range equivalentReleaseGroups(sanitizeReleaseGroup(v.ReleaseGroup)) {
			if strings.Contains(sanitized, g) {
				out = append(out, "release_group")
				break
			}
		}
	}
	return out
}

func nonEmpty(ss ...string) []string {
	var out []string
	for _, s := range ss {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// Download implements provider.Provider: the download page's script
// gives the link of a zip of the subtitle.
func (p *Provider) Download(ctx context.Context, _ *provider.Video, id string) (provider.File, error) {
	n, err := strconv.Atoi(id)
	if err != nil {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "malformed subtitle ID %q", id)
	}
	r, err := p.s.Get(ctx, p.link(fmt.Sprintf("download-%d.html", n)), nil)
	if err != nil {
		return provider.File{}, err
	}
	var link strings.Builder
	for _, m := range linkPartRE.FindAllSubmatch(r.Body, -1) {
		link.Write(m[1])
	}
	if link.Len() == 0 {
		return provider.File{}, provider.Errorf(provider.ProviderError, "cannot get download link")
	}
	ref, err := url.Parse(link.String())
	if err != nil {
		return provider.File{}, provider.Errorf(provider.ProviderError, "download link %q: %w", link.String(), err)
	}
	r, err = p.s.Get(ctx, p.base.ResolveReference(ref).String(), nil)
	if err != nil {
		return provider.File{}, err
	}
	zr, err := zip.NewReader(bytes.NewReader(r.Body), int64(len(r.Body)))
	if err != nil {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "download: %w", err)
	}
	if len(zr.File) > 1 {
		return provider.File{}, provider.Errorf(provider.ProviderError, "more than one file to unzip")
	}
	if len(zr.File) == 0 {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "download: an empty zip")
	}
	f, err := zr.File[0].Open()
	if err != nil {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "download: %w", err)
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxFile))
	if err != nil {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "download: %w", err)
	}
	data = archive.FixLineEndings(data)
	return provider.File{Data: data, Format: archive.Detect(data)}, nil
}

var (
	replaced  = regexp.MustCompile(`[-:().]`)
	spaces    = regexp.MustCompile(`\s+`)
	bracketed = regexp.MustCompile(`\[[\w|.]+\]`)
)

// sanitize strips a name's special characters, as subliminal's sanitize.
func sanitize(s string) string {
	s = replaced.ReplaceAllString(s, " ")
	s = strings.ReplaceAll(s, "'", "")
	return strings.ToLower(strings.TrimSpace(spaces.ReplaceAllString(s, " ")))
}

func sanitizeAll(ss []string) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = sanitize(s)
	}
	return out
}

// sanitizeReleaseGroup strips what is in square brackets, as subliminal's
// sanitize_release_group.
func sanitizeReleaseGroup(s string) string {
	return strings.ToUpper(strings.TrimSpace(bracketed.ReplaceAllString(s, "")))
}

// equivalentReleaseGroups returns the release groups of the same
// releases, as subliminal's get_equivalent_release_groups.
func equivalentReleaseGroups(group string) []string {
	for _, g := range [][]string{{"FRAMESTOR", "W4NK3R", "BHDSTUDIO"}, {"LOL", "DIMENSION"}, {"ASAP", "IMMERSE", "FLEET"}, {"AVS", "SVA"}} {
		if slices.Contains(g, group) {
			return g
		}
	}
	return []string{group}
}

// String names the provider in logs.
func (p *Provider) String() string { return fmt.Sprintf("%s(%s)", Name, p.base.Host) }
