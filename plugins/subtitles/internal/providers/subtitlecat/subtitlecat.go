// Package subtitlecat is the provider of Subtitle Cat, ported from
// Bazarr's subtitlecat: subtitles of movies and episodes in many
// languages, most of them machine translated by the site from the one a
// release was uploaded in.
package subtitlecat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/mavioai/mavio/plugins/subtitles/internal/archive"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/scrape"
	"github.com/mavioai/mavio/plugins/subtitles/internal/site"
	"github.com/mavioai/mavio/plugins/subtitles/internal/sitekit"
)

// Name is the provider's name.
const Name = "subtitlecat"

// serverURL is the site's.
const serverURL = "https://www.subtitlecat.com/"

const (
	// maxDetailPages bounds the detail pages fetched, one per candidate
	// release, as Bazarr's MAX_DETAIL_PAGES.
	maxDetailPages = 5
	// maxSearchTitles bounds the titles searched, as Bazarr's
	// MAX_SEARCH_TITLES.
	maxSearchTitles = 3
)

// languages are the site's, as Bazarr's _build_languages.
var languages = []string{
	"af", "ak", "sq", "am", "ar", "hy", "az", "eu", "be", "bn", "bs",
	"bg", "ca", "ny", "zh", "hr", "cs", "da", "nl", "en", "et", "fi",
	"fr", "fy", "gl", "ka", "de", "el", "gu", "ht", "ha", "he", "hi",
	"hu", "is", "ig", "id", "ga", "it", "ja", "jv", "kn", "kk", "km",
	"rw", "ko", "ku", "ky", "lo", "la", "lv", "ln", "lt", "lg", "lb",
	"mk", "mg", "ms", "ml", "mt", "mi", "mr", "mn", "my", "ne", "no",
	"or", "om", "ps", "fa", "pl", "pt", "pa", "ro", "ru", "sr", "si",
	"sk", "sl", "so", "es", "su", "sw", "sv", "tg", "ta", "te", "th",
	"tr", "uk", "ur", "uz", "vi", "cy", "xh", "yo", "zu",
}

// Definition returns the provider's definition.
func Definition() provider.Definition {
	return provider.Definition{
		Name: Name, DisplayName: "Subtitle Cat",
		Settings: []provider.Setting{{
			Name: "include_machine_translated", Label: "Include subtitles machine translated by SubtitleCat in search results",
			Type: provider.Checkbox, Default: false,
		}},
		Languages: slices.Clone(languages),
		Movies:    true, Episodes: true,
		New: New,
	}
}

// settings are the provider's settings.
type settings struct {
	IncludeMachineTranslated bool `json:"include_machine_translated"`
}

// New makes the provider from its settings.
func New(env provider.Env, raw json.RawMessage) (provider.Provider, error) {
	var s settings
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, provider.Errorf(provider.ConfigurationError, "settings: %w", err)
		}
	}
	sess := site.NewBrowser(env.HTTP)
	sess.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	sess.Header.Set("Accept-Language", "en-US,en;q=0.5")
	// The downloads answer 404 for no subtitle.
	dl := site.New(env.HTTP)
	dl.Header = sess.Header
	dl.Check = func(r *site.Response) error {
		if r.Status == http.StatusNotFound {
			return nil
		}
		return site.Status(r)
	}
	return &Provider{env: env, s: sess, dl: dl, base: serverURL, machineTranslated: s.IncludeMachineTranslated}, nil
}

// Provider is Subtitle Cat.
type Provider struct {
	env               provider.Env
	s, dl             *site.Session
	base              string
	machineTranslated bool
}

// SetBaseURL points the provider at another server, for tests.
func (p *Provider) SetBaseURL(u *url.URL) { p.base = strings.TrimSuffix(u.String(), "/") + "/" }

// sourceLanguageRe is the language a release was translated from, as the
// search results annotate it, as Bazarr's SOURCE_LANGUAGE_RE.
var sourceLanguageRe = regexp.MustCompile(`(?i)\(translated from ([^)]+)\)`)

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// words are the words of a title, as Bazarr's _normalize_title.
func words(title string) map[string]bool {
	out := map[string]bool{}
	for w := range strings.FieldsSeq(nonAlnum.ReplaceAllString(strings.ToLower(title), " ")) {
		out[w] = true
	}
	return out
}

// overlap is the share of a target title's words a title has, as Bazarr's
// _title_overlap_ratio.
func overlap(title, target string) float64 {
	want := words(target)
	if len(want) == 0 {
		return 1
	}
	got := words(title)
	n := 0
	for w := range want {
		if got[w] {
			n++
		}
	}
	return float64(n) / float64(len(want))
}

// differs reports whether the numbers guessed of a release differ from a
// video's, as Python compares guessit's int or list with an int.
func differs(guessed []int, want int) bool {
	return want != 0 && len(guessed) > 0 && (len(guessed) > 1 || guessed[0] != want)
}

// candidate reports whether a release may be of the video, as Bazarr's
// _is_candidate_match: of its season and episode or year, and with half
// the words of the title.
func candidate(release string, v *provider.Video, target string) bool {
	if v.Episode {
		seasons, episodes := scrape.SeasonsEpisodes(release)
		if differs(seasons, v.Season) || differs(episodes, v.Number) {
			return false
		}
		if target == "" {
			target = v.Series
		}
	} else {
		if y := scrape.Year(release); v.Year != 0 && y != 0 && y != v.Year {
			return false
		}
		if target == "" {
			target = v.Title
		}
	}
	title := scrape.Title(release)
	return title == "" || target == "" || overlap(title, target) > 0.5
}

// titles are the titles searched, the primary one first, as Bazarr's
// _titles.
func titles(v *provider.Video) []string {
	all := append([]string{v.Title}, v.AlternativeTitles...)
	if v.Episode {
		all = append([]string{v.Series}, v.AlternativeSeries...)
	}
	seen := map[string]bool{}
	var out []string
	for _, t := range all {
		if t == "" || seen[strings.ToLower(t)] {
			continue
		}
		seen[strings.ToLower(t)] = true
		out = append(out, t)
	}
	return out[:min(maxSearchTitles, len(out))]
}

// searchQuery is what a title is searched with, as Bazarr's _search_query.
func searchQuery(v *provider.Video, title string) string {
	switch {
	case v.Episode && v.Season != 0 && v.Number != 0:
		return title + " S" + pad(v.Season) + "E" + pad(v.Number)
	case !v.Episode && v.Year != 0:
		return title + " " + strconv.Itoa(v.Year)
	}
	return title
}

func pad(n int) string {
	if n < 10 && n >= 0 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}

// result is a release of the search results.
type result struct {
	release, href string
	// source is the tag of the language the release was translated from;
	// "" when unknown.
	source string
}

// search returns the releases a query finds, as Bazarr's _search.
func (p *Provider) search(ctx context.Context, query string) ([]result, error) {
	p.env.Log.InfoContext(ctx, "subtitlecat: searching", "query", query)
	r, err := p.s.Get(ctx, p.base+"index.php", url.Values{"search": {query}})
	if err != nil {
		return nil, err
	}
	doc, err := scrape.HTML(r)
	if err != nil {
		return nil, provider.Errorf(provider.ParseResponseError, "search: %w", err)
	}
	var out []result
	for _, row := range doc.All("table.sub-table tbody tr") {
		a := row.First("a[href]")
		if !a.Valid() || !strings.HasPrefix(a.Attr("href"), "subs/") {
			continue
		}
		res := result{release: a.TrimmedText(), href: a.Attr("href")}
		// The "translated from" marker is in the link's cell.
		for c := a.Parent(); c.Valid(); c = c.Parent() {
			if c.Data == "td" {
				if m := sourceLanguageRe.FindStringSubmatch(scrape.Words(c)); m != nil {
					res.source = sitekit.LanguageByName(m[1])
				}
				break
			}
		}
		out = append(out, res)
	}
	return out, nil
}

// requested reports whether a language tag is requested.
func requested(langs []provider.Language, tag string) bool {
	_, ok := sitekit.Match(langs, tag)
	return ok
}

// Search implements provider.Provider, as Bazarr's query: the primary
// title then the alternative ones are searched until one finds releases of
// the video, else the primary search's releases are taken; the subtitles
// of the first 5 releases in the languages are listed.
func (p *Provider) Search(ctx context.Context, v *provider.Video, langs []provider.Language) ([]provider.Subtitle, error) {
	var candidates, fallback []result
	for _, title := range titles(v) {
		results, err := p.search(ctx, searchQuery(v, title))
		if err != nil {
			return nil, err
		}
		if len(fallback) == 0 {
			fallback = results
		}
		candidates = slices.DeleteFunc(slices.Clone(results), func(r result) bool { return !candidate(r.release, v, title) })
		if len(candidates) > 0 {
			break
		}
		p.env.Log.DebugContext(ctx, "subtitlecat: no matching result, trying the next title", "title", title)
	}
	if len(candidates) == 0 {
		candidates = fallback
	}
	// Releases translated from another language would only offer machine
	// translations.
	if !p.machineTranslated {
		candidates = slices.DeleteFunc(candidates, func(r result) bool { return r.source != "" && !requested(langs, r.source) })
	}
	candidates = candidates[:min(maxDetailPages, len(candidates))]
	var out []provider.Subtitle
	for _, c := range candidates {
		subs, err := p.detail(ctx, c, langs)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			p.env.Log.DebugContext(ctx, "subtitlecat: failed to fetch the detail page", "page", p.base+c.href, "error", err)
			continue
		}
		out = append(out, subs...)
	}
	return out, nil
}

// detail lists the subtitles of a release's page in the languages, as
// Bazarr's _fetch_detail_subtitles: the site machine translates a release
// into every other language, which only the search results' source
// language tells apart.
func (p *Provider) detail(ctx context.Context, c result, langs []provider.Language) ([]provider.Subtitle, error) {
	page := p.base + c.href
	r, err := p.s.Get(ctx, page, nil)
	if err != nil {
		return nil, err
	}
	doc, err := scrape.HTML(r)
	if err != nil {
		return nil, err
	}
	var out []provider.Subtitle
	for _, div := range doc.All("div.sub-single") {
		a := div.First(`a[id^="download_"]`)
		if !a.Valid() {
			continue
		}
		code := strings.TrimPrefix(a.Attr("id"), "download_")
		base, _, _ := strings.Cut(strings.ToLower(strings.TrimSpace(code)), "-")
		l, ok := sitekit.Match(langs, provider.FromCode(base))
		if !ok || len(base) != 2 {
			continue
		}
		machine := c.source != "" && l.Tag != c.source
		if machine && !p.machineTranslated {
			p.env.Log.DebugContext(ctx, "subtitlecat: excluding a machine translated subtitle", "language", l.Tag, "page", page)
			continue
		}
		link := a.Attr("href")
		if link == "" {
			continue
		}
		if !strings.HasPrefix(link, "http") {
			link = p.base + strings.TrimLeft(link, "/")
		}
		s := provider.Subtitle{ID: link, Language: l, PageLink: page, ReleaseNames: []string{c.release}}
		if machine {
			s.Comment = "machine translated"
		}
		out = append(out, s)
	}
	return out, nil
}

// Download implements provider.Provider, as Bazarr's download_subtitle: a
// missing or empty subtitle is none.
func (p *Provider) Download(ctx context.Context, _ *provider.Video, id string) (provider.File, error) {
	r, err := p.dl.Get(ctx, id, nil)
	if err != nil {
		return provider.File{}, err
	}
	if r.Status == http.StatusNotFound || len(r.Body) == 0 {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "no subtitle at %s", id)
	}
	data := archive.FixLineEndings(r.Body)
	return provider.File{Data: data, Format: archive.Detect(data)}, nil
}
