// Package prijevodionline is the provider of Prijevodi Online, ported from
// Bazarr's prijevodionline: Croatian, Serbian and Montenegrin subtitles of
// episodes, found in the site's index of series and their pages.
package prijevodionline

import (
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
	"unicode"

	"golang.org/x/net/html"

	"github.com/mavioai/mavio/plugins/subtitles/internal/archive"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/site"
	"github.com/mavioai/mavio/plugins/subtitles/internal/titles"
)

// Name is the provider's name.
const Name = "prijevodionline"

// Languages: Serbo-Croatian takes the subtitles of all three.
const (
	croatian     = "hr"
	serbian      = "sr"
	montenegrin  = "cnr"
	serboCroatia = "sh"
)

// extensions are subliminal's SUBTITLE_EXTENSIONS.
var extensions = []string{".srt", ".sub", ".smi", ".txt", ".ssa", ".ass", ".mpl"}

// keyRe is the key the site requires to list subtitles, which the series'
// page assigns.
var keyRe = regexp.MustCompile(`epizode\.key\s*=\s*['"]([0-9a-f]{32})['"]`)

// Definition returns the provider's definition.
func Definition() provider.Definition {
	return provider.Definition{
		Name: Name, DisplayName: "Prijevodi Online",
		Languages: []string{croatian, montenegrin, serbian, serboCroatia},
		Episodes:  true,
		New:       New,
	}
}

// New makes the provider; it has no settings.
func New(env provider.Env, _ json.RawMessage) (provider.Provider, error) {
	return &Provider{env: env, s: site.NewBrowser(env.HTTP), server: "https://www.prijevodi-online.org"}, nil
}

// Provider is Prijevodi Online.
type Provider struct {
	env    provider.Env
	s      *site.Session
	server string
}

// SetBaseURL points the provider at another server, for tests.
func (p *Provider) SetBaseURL(u *url.URL) { p.server = strings.TrimSuffix(u.String(), "/") }

// Search implements provider.Provider: the subtitles of the first of the
// series' titles that has some.
func (p *Provider) Search(ctx context.Context, v *provider.Video, langs []provider.Language) ([]provider.Subtitle, error) {
	if !v.Episode {
		return nil, nil
	}
	for _, title := range append([]string{v.Series}, v.AlternativeSeries...) {
		subs, err := p.query(ctx, v, langs, title)
		if err != nil || len(subs) > 0 {
			return subs, err
		}
	}
	return nil, nil
}

func (p *Provider) query(ctx context.Context, v *provider.Video, langs []provider.Language, title string) ([]provider.Subtitle, error) {
	if title == "" {
		return nil, nil
	}
	id, slug, err := p.findSeries(ctx, title)
	if err != nil || id == "" {
		return nil, err
	}
	episode, key, seriesURL, err := p.findEpisode(ctx, id, slug, v.Season, v.Number)
	if err != nil || episode == "" {
		return nil, err
	}
	return p.subtitles(ctx, episode, key, seriesURL, langs)
}

// findSeries finds a series in the site's index of its initial, as
// Bazarr's _find_series: its ID and slug, "" for none.
func (p *Provider) findSeries(ctx context.Context, title string) (id, slug string, err error) {
	letter := strings.ToLower(string([]rune(title)[:1]))
	if r := []rune(letter); !unicode.IsLetter(r[0]) {
		letter = "0"
	}
	r, err := p.s.Get(ctx, p.server+"/serije/index/"+url.PathEscape(letter), nil)
	if err != nil {
		return "", "", err
	}
	doc, err := r.HTML()
	if err != nil {
		return "", "", provider.Errorf(provider.ParseResponseError, "index: %w", err)
	}
	want := titles.Sanitize(title)
	for _, row := range doc.All(`tr[id^="serija-"]`) {
		a := row.First("td.naziv > a")
		if !a.Valid() || titles.Sanitize(a.Text()) != want {
			continue
		}
		// The link is /serije/view/{id}/{slug}.
		if parts := strings.Split(a.Attr("href"), "/"); len(parts) >= 5 {
			return parts[3], parts[4], nil
		}
	}
	return "", "", nil
}

// findEpisode finds an episode on its series' page, as Bazarr's
// _find_episode_id: its ID, the key to list its subtitles with and the
// page's URL; "" for none.
func (p *Provider) findEpisode(ctx context.Context, id, slug string, season, number int) (episode, key, seriesURL string, err error) {
	seriesURL = p.server + "/serije/view/" + id + "/" + slug
	r, err := p.s.Get(ctx, seriesURL, nil)
	if err != nil {
		return "", "", "", err
	}
	if m := keyRe.FindSubmatch(r.Body); m != nil {
		key = string(m[1])
	}
	doc, err := r.HTML()
	if err != nil {
		return "", "", "", provider.Errorf(provider.ParseResponseError, "series: %w", err)
	}
	h3 := doc.First(fmt.Sprintf("div#epizode h3#sezona-%d", season))
	if !h3.Valid() {
		return "", "", "", nil
	}
	for n := h3.NextSibling; n != nil; n = n.NextSibling {
		if n.Type != html.ElementNode {
			continue
		}
		if n.Data == "h3" {
			break
		}
		div := site.Node{Node: n}
		if n.Data != "div" || !strings.HasPrefix(div.Attr("id"), "epizoda-") {
			continue
		}
		broj := div.First("li.broj")
		if !broj.Valid() {
			continue
		}
		ep, err := strconv.Atoi(strings.TrimRight(broj.TrimmedText(), "."))
		if err == nil && ep == number {
			return strings.SplitN(div.Attr("id"), "-", 3)[1], key, seriesURL, nil
		}
	}
	return "", "", "", nil
}

// subtitles lists an episode's subtitles in the languages, as Bazarr's
// _fetch_subtitles: the verified first.
func (p *Provider) subtitles(ctx context.Context, episode, key, seriesURL string, langs []provider.Language) ([]provider.Subtitle, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.server+"/prijevod/get/"+episode,
		strings.NewReader(url.Values{"key": {key}}.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Referer", seriesURL)
	req.Header.Set("Origin", p.server)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	r, err := p.s.Do(req)
	if err != nil {
		return nil, err
	}
	doc, err := r.HTML()
	if err != nil {
		return nil, provider.Errorf(provider.ParseResponseError, "subtitles: %w", err)
	}
	wanted := func(tag string) bool {
		return len(langs) == 0 || slices.ContainsFunc(langs, func(l provider.Language) bool { return l.Tag == tag })
	}
	var verified, unverified []provider.Subtitle
	for _, row := range doc.All(`tr[id^="prijevod-"]`) {
		rowID := row.Attr("id")
		if strings.Contains(rowID, "opis") {
			continue
		}
		subID, err := strconv.Atoi(strings.SplitN(rowID, "-", 3)[1])
		if err != nil {
			continue
		}
		a := row.First("td.naziv a[href]")
		if !a.Valid() {
			continue
		}
		href := a.Attr("href")
		var tag string
		switch lower := strings.ToLower(href); {
		case strings.HasSuffix(lower, "-hr"):
			tag = croatian
		case strings.HasSuffix(lower, "-sr"):
			tag = serbian
		case strings.HasSuffix(lower, "-cg"):
			tag = montenegrin
		default:
			continue
		}
		hbs := len(langs) > 0 && wanted(serboCroatia)
		if !wanted(tag) && !hbs {
			continue
		}
		if hbs {
			tag = serboCroatia
		}
		var releases []string
		if opis := doc.First(fmt.Sprintf("tr#prijevod-opis-%d", subID)); opis.Valid() {
			td := opis.First("td.opis")
			if !td.Valid() {
				td = opis.First("td")
			}
			if info := td.TrimmedText(); info != "" {
				for rel := range strings.SplitSeq(info, "/") {
					releases = append(releases, strings.TrimSpace(rel))
				}
			}
		}
		s := provider.Subtitle{
			ID:           url.Values{"u": {href}, "lang": {tag}}.Encode(),
			Language:     provider.Language{Tag: tag},
			ReleaseNames: releases,
			Matches:      []string{"series", "season", "episode"},
			PageLink:     p.server + href,
		}
		if strings.Contains(row.First("td.status").Text(), "provjereno") {
			verified = append(verified, s)
		} else {
			unverified = append(unverified, s)
		}
	}
	return append(verified, unverified...), nil
}

// Download implements provider.Provider: the first subtitle file of the
// zip or RAR archive the site serves.
func (p *Provider) Download(ctx context.Context, _ *provider.Video, id string) (provider.File, error) {
	q, err := url.ParseQuery(id)
	if err != nil || !strings.HasPrefix(q.Get("u"), "/") {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "malformed subtitle ID %q", id)
	}
	r, err := p.s.Get(ctx, p.server+q.Get("u"), nil)
	if err != nil {
		return provider.File{}, err
	}
	if !bytes.HasPrefix(r.Body, []byte("PK")) && !bytes.HasPrefix(r.Body, []byte("Rar!")) {
		return provider.File{}, provider.Errorf(provider.ProviderError, "unrecognized archive format for subtitle %s", q.Get("u"))
	}
	a, err := archive.Open(r.Body)
	if err != nil {
		return provider.File{}, provider.Errorf(provider.ProviderError, "%w", err)
	}
	for _, name := range a.Names() {
		ext := strings.ToLower(path.Ext(name))
		if !slices.Contains(extensions, ext) {
			continue
		}
		data, err := a.Read(name)
		if err != nil {
			return provider.File{}, provider.Errorf(provider.ProviderError, "%s: %w", name, err)
		}
		format := strings.TrimPrefix(ext, ".")
		if !slices.Contains([]string{"srt", "ass", "ssa"}, format) {
			format = archive.Detect(data)
		}
		return provider.File{Data: archive.FixLineEndings(data), Format: format, Language: provider.Language{Tag: q.Get("lang")}}, nil
	}
	return provider.File{}, provider.Errorf(provider.ProviderError, "no subtitle file found in the archive")
}
