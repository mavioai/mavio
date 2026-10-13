// Package animekalesi is the provider of AnimeKalesi, ported from Bazarr's
// animekalesi: Turkish subtitles of anime episodes, found in the site's
// list of series and their pages of episodes.
package animekalesi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"regexp"
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
const Name = "animekalesi"

// cacheFor is how long the site's lists are cached, as Bazarr's
// SEARCH_EXPIRATION_TIME.
const cacheFor = time.Hour

// Definition returns the provider's definition.
func Definition() provider.Definition {
	return provider.Definition{
		Name: Name, DisplayName: "AnimeKalesi",
		Languages: []string{"tr"},
		Episodes:  true,
		New:       New,
	}
}

// New makes the provider; it has no settings.
func New(env provider.Env, _ json.RawMessage) (provider.Provider, error) {
	s := site.New(env.HTTP)
	s.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	s.Header.Set("Accept-Language", "tr,en-US;q=0.7,en;q=0.3")
	p := &Provider{env: env, s: s, series: map[string]cached[*series]{}, episodes: map[string]cached[[]episode]{}}
	p.SetBaseURL(&url.URL{Scheme: "https", Host: "www.animekalesi.com"})
	return p, nil
}

// Provider is AnimeKalesi.
type Provider struct {
	env    provider.Env
	s      *site.Session
	server string

	mu       sync.Mutex
	series   map[string]cached[*series]
	episodes map[string]cached[[]episode]
}

type cached[T any] struct {
	v     T
	until time.Time
}

type series struct{ title, url string }

type episode struct{ title, url string }

// SetBaseURL points the provider at another server, for tests.
func (p *Provider) SetBaseURL(u *url.URL) {
	p.server = strings.TrimSuffix(u.String(), "/")
	p.s.Header.Set("Referer", p.server)
}

var (
	special = regexp.MustCompile(`[^\p{L}\p{N}_\s-]`)
	spaces  = regexp.MustCompile(`\s+`)
	turkish = strings.NewReplacer("İ", "i", "I", "i", "Ğ", "g", "Ü", "u", "Ş", "s", "Ö", "o", "Ç", "c",
		"ı", "i", "ğ", "g", "ü", "u", "ş", "s", "ö", "o", "ç", "c")
)

// normalize normalizes a series' name for comparisons, as Bazarr's
// normalize_series_name.
func normalize(s string) string {
	s = special.ReplaceAllString(s, "")
	s = spaces.ReplaceAllString(s, " ")
	return strings.TrimSpace(strings.ToLower(turkish.Replace(s)))
}

// Search implements provider.Provider.
func (p *Provider) Search(ctx context.Context, v *provider.Video, langs []provider.Language) ([]provider.Subtitle, error) {
	if !v.Episode || v.Series == "" || v.Number == 0 || v.Season == 0 {
		return nil, nil
	}
	sr, err := p.searchSeries(ctx, v.Series)
	if err != nil || sr == nil {
		return nil, err
	}
	eps, err := p.episodeList(ctx, sr.url)
	if err != nil {
		return nil, err
	}
	var out []provider.Subtitle
	for _, ep := range eps {
		season, number, ok := seasonEpisode(ep.title)
		if !ok || season != v.Season || number != v.Number {
			continue
		}
		sub, err := p.subtitle(ctx, v, sr, ep, season, number)
		if err != nil {
			return nil, err
		}
		if sub != nil {
			out = append(out, *sub)
		}
	}
	return out, nil
}

// searchSeries finds a series in the site's list, as Bazarr's
// _search_anime_list: the one of the same normalized name, else the
// first whose name contains the other's; nil for none.
func (p *Provider) searchSeries(ctx context.Context, name string) (*series, error) {
	now := p.env.Now()
	p.mu.Lock()
	c, ok := p.series[name]
	p.mu.Unlock()
	if ok && now.Before(c.until) {
		return c.v, nil
	}
	r, err := p.s.Get(ctx, p.server+"/tum-anime-serileri.html", nil)
	if err != nil {
		return nil, err
	}
	doc, err := r.HTML()
	if err != nil {
		return nil, provider.Errorf(provider.ParseResponseError, "series list: %w", err)
	}
	want := normalize(name)
	var found *series
	for _, td := range doc.All("td#bolumler") {
		a := td.First("a")
		if !a.Valid() {
			continue
		}
		title, href := a.TrimmedText(), a.Attr("href")
		if !strings.Contains(href, "bolumler-") {
			continue
		}
		got := normalize(title)
		if got == want {
			found = &series{title, href}
			break
		}
		if found == nil && (strings.Contains(got, want) || strings.Contains(want, got)) {
			found = &series{title, href}
		}
	}
	p.mu.Lock()
	p.series[name] = cached[*series]{found, now.Add(cacheFor)}
	p.mu.Unlock()
	return found, nil
}

// episodeList returns the episodes of a series with subtitles, as Bazarr's
// _get_episode_list.
func (p *Provider) episodeList(ctx context.Context, seriesURL string) ([]episode, error) {
	now := p.env.Now()
	p.mu.Lock()
	c, ok := p.episodes[seriesURL]
	p.mu.Unlock()
	if ok && now.Before(c.until) {
		return c.v, nil
	}
	r, err := p.s.Get(ctx, p.server+"/"+strings.ReplaceAll(seriesURL, "bolumler-", "altyazib-"), nil)
	if err != nil {
		return nil, err
	}
	doc, err := r.HTML()
	if err != nil {
		return nil, provider.Errorf(provider.ParseResponseError, "episode list: %w", err)
	}
	var eps []episode
	for _, td := range doc.All("td#ayazi_indir") {
		a := td.First("a[href]")
		if !a.Valid() {
			continue
		}
		if strings.Contains(a.Attr("href"), "indir_bolum-") && strings.Contains(a.Attr("title"), "Bölüm Türkçe Altyazısı") {
			eps = append(eps, episode{a.Attr("title"), p.server + "/" + a.Attr("href")})
		}
	}
	p.mu.Lock()
	p.episodes[seriesURL] = cached[[]episode]{eps, now.Add(cacheFor)}
	p.mu.Unlock()
	return eps, nil
}

var (
	episodeRe = regexp.MustCompile(`(\d+)\.\s*Bölüm`)
	seasonRe  = regexp.MustCompile(`(\d+)\.\s*Sezon`)
)

// seasonEpisode reads the season and episode of an episode's title, as
// Bazarr's _parse_season_episode: the season is 1 when the title has none.
func seasonEpisode(title string) (season, number int, ok bool) {
	m := episodeRe.FindStringSubmatch(title)
	if m == nil {
		return 0, 0, false
	}
	number, _ = strconv.Atoi(m[1])
	season = 1
	if m := seasonRe.FindStringSubmatch(title); m != nil {
		season, _ = strconv.Atoi(m[1])
	}
	return season, number, true
}

// subtitle reads an episode's page for its subtitle; nil when it has
// none.
func (p *Provider) subtitle(ctx context.Context, v *provider.Video, sr *series, ep episode, season, number int) (*provider.Subtitle, error) {
	r, err := p.s.Get(ctx, ep.url, nil)
	if err != nil {
		return nil, err
	}
	doc, err := r.HTML()
	if err != nil {
		return nil, provider.Errorf(provider.ParseResponseError, "episode page: %w", err)
	}
	a := doc.First("div#altyazi_indir a[href]")
	if !a.Valid() {
		return nil, nil
	}
	link := p.server + "/" + a.Attr("href")
	uploader := ""
	for _, strong := range doc.All("strong") {
		if strong.Text() != "Altyazı/Çeviri:" {
			continue
		}
		if next := strong.NextSibling; next != nil && next.Type == html.TextNode {
			uploader = strings.TrimSpace(next.Data)
		}
		break
	}
	version := fmt.Sprintf("%s - S%02dE%02d", sr.title, season, number)
	if uploader != "" {
		version += " by " + uploader
	}
	var matches []string
	if seriesMatches(v, sr.title) {
		matches = append(matches, "series")
	}
	matches = append(matches, "season", "episode")
	return &provider.Subtitle{
		ID:           url.Values{"u": {link}, "v": {version}}.Encode(),
		Language:     provider.Language{Tag: "tr"},
		ReleaseNames: []string{version},
		Matches:      matches,
		PageLink:     ep.url,
		Uploader:     uploader,
	}, nil
}

// seriesMatches reports whether the site's series is the video's, as
// Bazarr's get_matches: by name, normalized name or alternative name.
func seriesMatches(v *provider.Video, title string) bool {
	if strings.EqualFold(v.Series, title) || normalize(v.Series) == normalize(title) {
		return true
	}
	for _, alt := range v.AlternativeSeries {
		if normalize(alt) == normalize(title) {
			return true
		}
	}
	return false
}

// Download implements provider.Provider: a subtitle file, or the one of a
// zip archive whose name has the subtitle's version, else its first.
func (p *Provider) Download(ctx context.Context, _ *provider.Video, id string) (provider.File, error) {
	q, err := url.ParseQuery(id)
	if err != nil || q.Get("u") == "" {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "malformed subtitle ID %q", id)
	}
	r, err := p.s.Get(ctx, q.Get("u"), nil)
	if err != nil {
		return provider.File{}, err
	}
	lang := provider.Language{Tag: "tr"}
	if !bytes.HasPrefix(r.Body, []byte("PK\x03\x04")) {
		return provider.File{Data: archive.FixLineEndings(r.Body), Format: format("", r.Body), Language: lang}, nil
	}
	a, err := archive.Open(r.Body)
	if err != nil {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "zip: %w", err)
	}
	var files []string
	for _, name := range a.Names() {
		if ext := strings.ToLower(path.Ext(name)); ext == ".srt" || ext == ".ass" {
			files = append(files, name)
		}
	}
	if len(files) == 0 {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "no subtitle file found in ZIP archive")
	}
	pick := files[0]
	if len(files) > 1 {
		version := strings.ToLower(q.Get("v"))
		for _, name := range files {
			if strings.Contains(strings.ToLower(name), version) {
				pick = name
				break
			}
		}
	}
	data, err := a.Read(pick)
	if err != nil {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "%s: %w", pick, err)
	}
	return provider.File{Data: archive.FixLineEndings(data), Format: format(pick, data), Language: lang}, nil
}

// format returns a subtitle's format by its file's extension, else by its
// content.
func format(name string, data []byte) string {
	switch f := strings.TrimPrefix(strings.ToLower(path.Ext(name)), "."); f {
	case "srt", "ass", "ssa", "vtt":
		return f
	}
	return archive.Detect(data)
}
