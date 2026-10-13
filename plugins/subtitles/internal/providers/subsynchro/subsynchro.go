// Package subsynchro is the provider of Subsynchro, ported from Bazarr's
// subsynchro: French subtitles of movies found by title and year, served
// in zip archives.
package subsynchro

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/mavioai/mavio/plugins/subtitles/internal/archive"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/site"
)

// Name is the provider's name.
const Name = "subsynchro"

// language is the provider's only language.
const language = "fr"

// Definition returns the provider's definition.
func Definition() provider.Definition {
	return provider.Definition{
		Name: Name, DisplayName: "Subsynchro",
		Languages: []string{language},
		Movies:    true,
		New:       New,
	}
}

// New makes the provider; it takes no settings.
func New(env provider.Env, _ json.RawMessage) (provider.Provider, error) {
	s := site.New(env.HTTP)
	s.Header.Set("User-Agent", "Bazarr")
	p := &Provider{env: env, s: s}
	p.SetBaseURL(&url.URL{Scheme: "https", Host: "www.subsynchro.com"})
	return p, nil
}

// Provider is Subsynchro.
type Provider struct {
	env  provider.Env
	s    *site.Session
	page string
}

// SetBaseURL points the provider at another server, for tests.
func (p *Provider) SetBaseURL(u *url.URL) {
	p.page = strings.TrimSuffix(u.String(), "/")
	p.s.Header.Set("Referer", p.page)
}

// item is a subtitle of the site's search.
type item struct {
	Title         *string `json:"titre"`
	OriginalTitle *string `json:"titre_original"`
	Release       *string `json:"release"`
	Filename      *string `json:"filename"`
	Download      string  `json:"telechargement"`
}

// orNA is a field, "n/a" when missing, as Bazarr's.
func orNA(s *string) string {
	if s == nil {
		return "n/a"
	}
	return *s
}

// Search implements provider.Provider.
func (p *Provider) Search(ctx context.Context, v *provider.Video, langs []provider.Language) ([]provider.Subtitle, error) {
	if v.Episode || !slices.ContainsFunc(langs, func(l provider.Language) bool { return l.Tag == language }) {
		return nil, nil
	}
	year := "None"
	if v.Year != 0 {
		year = strconv.Itoa(v.Year)
	}
	params := url.Values{"title": {v.Title}, "year": {year}}
	p.env.Log.DebugContext(ctx, "searching subtitles", "params", params)
	r, err := p.s.Get(ctx, p.page+"/include/ajax/subMarin.php", params)
	if err != nil {
		return nil, err
	}
	var res struct {
		Status any    `json:"status"`
		Data   []item `json:"data"`
	}
	// An answer that is not JSON is taken for no subtitles, as Bazarr's.
	if json.Unmarshal(r.Body, &res) != nil {
		return nil, nil
	}
	if s, ok := res.Status.(float64); !ok || s != 200 {
		p.env.Log.DebugContext(ctx, "no subtitles found", "status", res.Status)
		return nil, nil
	}
	var out []provider.Subtitle
	for _, it := range res.Data {
		var matches []string
		title := strings.ToLower(v.Title)
		if strings.Contains(strings.ToLower(orNA(it.Title)), title) || strings.Contains(strings.ToLower(orNA(it.OriginalTitle)), title) {
			// The year is matched by the search.
			matches = []string{"title", "year"}
		}
		filename, release := orNA(it.Filename), orNA(it.Release)
		if len(release) <= len(filename) {
			release = filename
		}
		link := p.resolve(it.Download)
		out = append(out, provider.Subtitle{
			ID: link, Language: provider.Language{Tag: language},
			ReleaseNames: slices.Compact([]string{filename, release}),
			Matches:      matches, PageLink: link,
		})
	}
	return out, nil
}

// resolve makes a link of the site absolute.
func (p *Provider) resolve(link string) string {
	base, err := url.Parse(p.page + "/")
	if err != nil {
		return link
	}
	u, err := base.Parse(link)
	if err != nil {
		return link
	}
	return u.String()
}

// Download implements provider.Provider: id is the subtitle's link.
func (p *Provider) Download(ctx context.Context, _ *provider.Video, id string) (provider.File, error) {
	p.env.Log.DebugContext(ctx, "downloading subtitle", "url", id)
	r, err := p.s.Get(ctx, id, nil)
	if err != nil {
		return provider.File{}, err
	}
	if _, err := zip.NewReader(bytes.NewReader(r.Body), int64(len(r.Body))); err != nil {
		return provider.File{}, provider.Errorf(provider.APIThrottled, "unknown file type: %s", id)
	}
	a, err := archive.Open(r.Body)
	if err != nil {
		return provider.File{}, provider.Errorf(provider.APIThrottled, "unknown file type: %s: %w", id, err)
	}
	for _, name := range a.Names() {
		if strings.HasPrefix(path.Base(name), ".") || !hasSubtitleExtension(name) {
			continue
		}
		data, err := a.Read(name)
		if err != nil {
			return provider.File{}, provider.Errorf(provider.BadSubtitle, "%s: %w", name, err)
		}
		data = archive.FixLineEndings(data)
		return provider.File{Data: data, Format: format(name, data), Language: provider.Language{Tag: language}}, nil
	}
	return provider.File{}, provider.Errorf(provider.APIThrottled, "can not find the subtitle in the zip file")
}

// hasSubtitleExtension reports whether a file has one of subliminal's
// subtitle extensions.
func hasSubtitleExtension(name string) bool {
	name = strings.ToLower(name)
	for _, ext := range []string{".srt", ".sub", ".smi", ".txt", ".ssa", ".ass", ".mpl"} {
		if strings.HasSuffix(name, ext) {
			return true
		}
	}
	return false
}

// format is a subtitle's format by its name, or its content.
func format(name string, data []byte) string {
	switch ext := strings.TrimPrefix(strings.ToLower(path.Ext(name)), "."); ext {
	case "srt", "ass", "ssa", "vtt":
		return ext
	}
	return archive.Detect(data)
}

// String names the provider in logs.
func (p *Provider) String() string { return Name }
