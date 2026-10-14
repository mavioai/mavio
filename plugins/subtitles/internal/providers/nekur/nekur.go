// Package nekur is the provider of Nekur, ported from Bazarr's nekur:
// Latvian subtitles of movies, found by the site's search of titles.
package nekur

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/net/html"

	"github.com/mavioai/mavio/plugins/subtitles/internal/archive"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/site"
	"github.com/mavioai/mavio/plugins/subtitles/internal/titles"
)

// Name is the provider's name.
const Name = "nekur"

// Definition returns the provider's definition.
func Definition() provider.Definition {
	return provider.Definition{
		Name: Name, DisplayName: "Nekur",
		Languages: []string{"lv"},
		Movies:    true,
		New:       New,
	}
}

// New makes the provider; it has no settings.
func New(env provider.Env, _ json.RawMessage) (provider.Provider, error) {
	log := env.Log
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	p := &Provider{env: env, log: log, s: site.NewBrowser(env.HTTP)}
	p.SetBaseURL(&url.URL{Scheme: "http", Host: "subtitri.nekur.net"})
	return p, nil
}

// Provider is Nekur.
type Provider struct {
	env    provider.Env
	log    *slog.Logger
	s      *site.Session
	server string
}

// SetBaseURL points the provider at another server, for tests.
func (p *Provider) SetBaseURL(u *url.URL) {
	p.server = strings.TrimSuffix(u.String(), "/") + "/"
	p.s.Header.Set("Referer", p.server)
}

// Search implements provider.Provider: each of the movie's titles is
// searched.
func (p *Provider) Search(ctx context.Context, v *provider.Video, langs []provider.Language) ([]provider.Subtitle, error) {
	if v.Episode || len(langs) > 0 && !slices.ContainsFunc(langs, func(l provider.Language) bool { return l.Tag == "lv" }) {
		return nil, nil
	}
	var out []provider.Subtitle
	for _, title := range append([]string{v.Title}, v.AlternativeTitles...) {
		subs, err := p.query(ctx, v, title)
		if err != nil {
			return nil, err
		}
		out = append(out, subs...)
	}
	return out, nil
}

// query lists the results of a search, as Bazarr's query.
func (p *Provider) query(ctx context.Context, v *provider.Video, title string) ([]provider.Subtitle, error) {
	r, err := p.s.PostForm(ctx, p.server+"modules/Subtitles.php", url.Values{"ajax": {"1"}, "sSearch": {title}})
	if err != nil {
		return nil, err
	}
	if len(r.Body) == 0 {
		return nil, nil
	}
	doc, err := site.Parse(r.Body, "text/html; charset=utf-8")
	if err != nil {
		return nil, provider.Errorf(provider.ParseResponseError, "search: %w", err)
	}
	var out []provider.Subtitle
	for _, row := range doc.All("tbody > tr") {
		a := row.First(".title > a")
		imdb := row.First("td:nth-of-type(4) a")
		if !a.Valid() || !imdb.Valid() {
			continue
		}
		name := ""
		for c := a.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.TextNode {
				name = strings.TrimSpace(c.Data)
				break
			}
		}
		year, _ := strconv.Atoi(strings.Trim(row.First(".year").Text(), "()"))
		link := p.server + a.Attr("href")
		parts := strings.Split(imdb.Attr("href"), "/")
		imdbID := ""
		if len(parts) >= 2 {
			imdbID = parts[len(parts)-2]
		}
		fps := strings.TrimSpace(row.First(".fps").Text())
		notes := strings.TrimSpace(row.First(".notes").Text())

		var matches []string
		if v.Title != "" && titles.Sanitize(name) == titles.Sanitize(v.Title) {
			matches = append(matches, "title")
		}
		if v.Year != 0 && year == v.Year {
			matches = append(matches, "year")
		}
		if id := v.ExternalIDs["imdb"]; id != "" && id == imdbID {
			matches = append(matches, "imdb_id")
		}
		if f, err := strconv.ParseFloat(fps, 64); err == nil && v.FrameRate != 0 && !framerateEqual(v.FrameRate, f) {
			p.log.WarnContext(ctx, "nekur: wrong FPS", "expected", v.FrameRate, "got", fps)
		}
		out = append(out, provider.Subtitle{
			ID:           link,
			Language:     provider.Language{Tag: "lv"},
			ReleaseNames: []string{notes},
			Matches:      matches,
			PageLink:     link,
		})
	}
	return out, nil
}

// framerateEqual reports whether frame rates are the same, as Bazarr's
// framerate_equal: 23.976, 23.98 and 24 are.
func framerateEqual(a, b float64) bool {
	same := []float64{23.976, 23.98, 24.0}
	return a == b || slices.Contains(same, a) && slices.Contains(same, b)
}

// Download implements provider.Provider: the movie's subtitle of the zip
// or RAR archive the site serves, or the subtitle file it serves.
func (p *Provider) Download(ctx context.Context, v *provider.Video, id string) (provider.File, error) {
	if !strings.HasPrefix(id, p.server) {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "malformed subtitle ID %q", id)
	}
	r, err := p.s.Get(ctx, id, nil)
	if err != nil {
		return provider.File{}, err
	}
	lang := provider.Language{Tag: "lv"}
	if bytes.HasPrefix(r.Body, []byte("PK")) || bytes.HasPrefix(r.Body, []byte("Rar!")) {
		return archive.Extract(r.Body, archive.WantFor(v, lang), lang)
	}
	format := archive.Detect(r.Body)
	if format == "" {
		return provider.File{}, provider.Errorf(provider.ProviderError, "unidentified archive type")
	}
	return provider.File{Data: r.Body, Format: format, Language: lang}, nil
}
