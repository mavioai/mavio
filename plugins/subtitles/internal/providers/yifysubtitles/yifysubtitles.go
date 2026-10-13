// Package yifysubtitles is the provider of YIFY Subtitles, ported from
// Bazarr's yifysubtitles: subtitles of movies found by IMDb ID.
package yifysubtitles

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/mavioai/mavio/plugins/subtitles/internal/archive"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/site"
)

// Name is the provider's name.
const Name = "yifysubtitles"

// serverURL is the site's.
const serverURL = "https://yifysubtitles.ch"

// languages are the site's languages by their names, as Bazarr's
// YifyLanguages.
var languages = []struct{ name, tag string }{
	{"Albanian", "sq"},
	{"Arabic", "ar"},
	{"Bengali", "bn"},
	{"Brazilian Portuguese", "pt-BR"},
	{"Bulgarian", "bg"},
	{"Chinese", "zh"},
	{"Croatian", "hr"},
	{"Czech", "cs"},
	{"Danish", "da"},
	{"Dutch", "nl"},
	{"English", "en"},
	{"Farsi/Persian", "fa"},
	{"Finnish", "fi"},
	{"French", "fr"},
	{"German", "de"},
	{"Greek", "el"},
	{"Hebrew", "he"},
	{"Hungarian", "hu"},
	{"Indonesian", "id"},
	{"Italian", "it"},
	{"Japanese", "ja"},
	{"Korean", "ko"},
	{"Lithuanian", "lt"},
	{"Macedonian", "mk"},
	{"Malay", "ms"},
	{"Norwegian", "no"},
	{"Polish", "pl"},
	{"Portuguese", "pt"},
	{"Romanian", "ro"},
	{"Russian", "ru"},
	{"Serbian", "sr"},
	{"Slovenian", "sl"},
	{"Spanish", "es"},
	{"Swedish", "sv"},
	{"Thai", "th"},
	{"Turkish", "tr"},
	{"Urdu", "ur"},
	{"Vietnamese", "vi"},
}

// Definition returns the provider's definition.
func Definition() provider.Definition {
	tags := make([]string, len(languages))
	for i, l := range languages {
		tags[i] = l.tag
	}
	return provider.Definition{
		Name: Name, DisplayName: "YIFY Subtitles",
		Languages:                 tags,
		HearingImpairedVerifiable: true,
		Movies:                    true,
		New:                       New,
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
	// The search is not redirected; any answer but 404 and 2xx is a
	// failure, as Bazarr's raise_for_status.
	hc := *env.HTTP
	hc.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	search := site.New(&hc)
	search.Header = header
	search.Check = func(r *site.Response) error {
		if r.Status == http.StatusNotFound || r.Status >= 300 && r.Status < 400 {
			return nil
		}
		return site.Status(r)
	}
	base, _ := url.Parse(serverURL)
	return &Provider{env: env, s: s, search: search, base: base}, nil
}

// Provider is YIFY Subtitles.
type Provider struct {
	env    provider.Env
	s      *site.Session
	search *site.Session
	base   *url.URL
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

// Search implements provider.Provider: the subtitles of the movie's
// IMDb ID, the best rated first, as Bazarr's query.
func (p *Provider) Search(ctx context.Context, v *provider.Video, langs []provider.Language) ([]provider.Subtitle, error) {
	imdbID := v.ExternalIDs["imdb"]
	if v.Episode || imdbID == "" {
		return nil, nil
	}
	u, _ := p.resolve("/movie-imdb/" + url.PathEscape(imdbID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Referer", p.base.String())
	r, err := p.search.Do(req)
	if err != nil {
		return nil, err
	}
	if r.Status != http.StatusOK {
		p.env.Log.DebugContext(ctx, "no subtitles found", "provider", Name, "imdb", imdbID)
		return nil, nil
	}
	doc, err := r.HTML()
	if err != nil {
		return nil, provider.Errorf(provider.ParseResponseError, "search: %w", err)
	}
	type rated struct {
		sub    provider.Subtitle
		rating int
	}
	var found []rated
	for _, row := range doc.First("table.other-subs").First("tbody").All("tr") {
		s, rating, ok := p.parseRow(row)
		if !ok || !slices.Contains(langs, s.Language) {
			continue
		}
		found = append(found, rated{s, rating})
	}
	slices.SortStableFunc(found, func(a, b rated) int { return cmp.Compare(b.rating, a.rating) })
	out := make([]provider.Subtitle, len(found))
	for i, f := range found {
		out[i] = f.sub
	}
	return out, nil
}

var subtitlePrefix = regexp.MustCompile(`^\nsubtitle `)

// parseRow reads a row of the table of subtitles, as Bazarr's
// _parse_row; rows that cannot be read are skipped.
func (p *Provider) parseRow(row site.Node) (provider.Subtitle, int, bool) {
	td := row.All("td")
	if len(td) < 5 {
		return provider.Subtitle{}, 0, false
	}
	rating, err := strconv.Atoi(td[0].TrimmedText())
	if err != nil {
		return provider.Subtitle{}, 0, false
	}
	name := td[1].Text()
	i := slices.IndexFunc(languages, func(l struct{ name, tag string }) bool { return l.name == name })
	a := td[2].First("a")
	if i < 0 || !a.Valid() {
		return provider.Subtitle{}, 0, false
	}
	pageLink, err := p.resolve(a.Attr("href"))
	if err != nil {
		return provider.Subtitle{}, 0, false
	}
	release := subtitlePrefix.ReplaceAllString(td[2].Text(), "")
	hi := td[3].First("span.hi-subtitle").Valid()
	var releases []string
	for r := range strings.SplitSeq(release, "\n") {
		if r = strings.TrimSpace(r); r != "" {
			releases = append(releases, r)
		}
	}
	return provider.Subtitle{
		ID:           a.Attr("href"),
		Language:     provider.Language{Tag: languages[i].tag, HearingImpaired: hi},
		ReleaseNames: releases,
		// The subtitles are those of the movie's IMDb ID.
		Matches:  []string{"imdb_id"},
		PageLink: pageLink,
		Uploader: td[4].Text(),
	}, rating, true
}

// Download implements provider.Provider: the zip of the subtitle's page's
// download button, whose first valid subtitle file is taken.
func (p *Provider) Download(ctx context.Context, _ *provider.Video, id string) (provider.File, error) {
	pageLink, err := p.resolve(id)
	if err != nil {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "malformed subtitle ID %q", id)
	}
	r, err := p.get(ctx, pageLink, pageLink)
	if err != nil {
		return provider.File{}, err
	}
	doc, err := r.HTML()
	if err != nil {
		return provider.File{}, provider.Errorf(provider.ParseResponseError, "%s: %w", pageLink, err)
	}
	button := doc.First("a.download-subtitle")
	if !button.Valid() {
		return provider.File{}, provider.Errorf(provider.ProviderError, "cannot find download link on this page: %s", pageLink)
	}
	link, err := p.resolve(button.Attr("href"))
	if err != nil {
		return provider.File{}, err
	}
	r, err = p.get(ctx, link, pageLink)
	if err != nil {
		return provider.File{}, err
	}
	a, err := archive.Open(r.Body)
	if err != nil || len(r.Body) < 4 || string(r.Body[:2]) != "PK" {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "unsupported archive")
	}
	for _, name := range a.Names() {
		if lower := strings.ToLower(name); !strings.HasSuffix(lower, ".srt") && !strings.HasSuffix(lower, ".sub") {
			continue
		}
		data, err := a.Read(name)
		if err != nil {
			return provider.File{}, provider.Errorf(provider.BadSubtitle, "%s: %w", name, err)
		}
		data = archive.FixLineEndings(data)
		if format := archive.Detect(data); format != "" {
			return provider.File{Data: data, Format: format}, nil
		}
	}
	return provider.File{}, provider.Errorf(provider.BadSubtitle, "no valid subtitle in the archive")
}

func (p *Provider) get(ctx context.Context, u, referer string) (*site.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Referer", referer)
	return p.s.Do(req)
}

// String names the provider in logs.
func (p *Provider) String() string { return fmt.Sprintf("%s(%s)", Name, p.base.Host) }
