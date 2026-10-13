// Package subsarr is the provider of Subsarr, ported from Bazarr's
// subsarr: a self-hosted copy of Subscene's subtitles, searched by IMDb ID
// or title.
package subsarr

import (
	"context"
	"encoding/json"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/text/language"

	"github.com/mavioai/mavio/plugins/subtitles/internal/archive"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/site"
)

// Name is the provider's name.
const Name = "subsarr"

// userAgent identifies the plugin, as Bazarr's.
const userAgent = "Mavio/0.1"

// Definition returns the provider's definition.
func Definition() provider.Definition {
	return provider.Definition{
		Name: Name, DisplayName: "Subsarr",
		Settings: []provider.Setting{
			{
				Name: "base_url", Label: "Base URL", Type: provider.Text,
				Help: "Self-hosted Subscene subtitle provider. (requires slimcdk/subsarr https://github.com/slimcdk/subsarr)",
			},
		},
		Languages:                 tags(),
		HearingImpairedVerifiable: true,
		Movies:                    true, Episodes: true,
		New: New,
	}
}

// fromSite are Subsarr's language names, Subscene's in lowercase with
// hyphens, as Bazarr's SubsarrConverter.
var fromSite = map[string]string{
	"english": "en", "farsi_persian": "fa", "abkhazian": "ab", "afrikaans": "af", "albanian": "sq", "amharic": "am",
	"arabic": "ar", "aragonese": "an", "armenian": "hy", "assamese": "as", "azerbaijani": "az", "basque": "eu",
	"belarusian": "be", "bengali": "bn", "bosnian": "bs", "brazilian_portuguese": "pt-BR", "breton": "br",
	"bulgarian": "bg", "burmese": "my", "catalan": "ca", "chinese_bg_code": "zh", "croatian": "hr", "czech": "cs",
	"danish": "da", "dutch": "nl", "esperanto": "eo", "estonian": "et", "finnish": "fi", "french": "fr",
	"gaelic": "gd", "georgian": "ka", "german": "de", "greek": "el", "hebrew": "he", "hindi": "hi",
	"hungarian": "hu", "icelandic": "is", "igbo": "ig", "indonesian": "id", "interlingua": "ia", "irish": "ga",
	"italian": "it", "japanese": "ja", "kannada": "kn", "kazakh": "kk", "cambodian-khmer": "km", "korean": "ko",
	"kurdish": "ku", "latvian": "lv", "lithuanian": "lt", "luxembourgish": "lb", "macedonian": "mk", "malay": "ms",
	"malayalam": "ml", "marathi": "mr", "mongolian": "mn", "navajo": "nv", "nepali": "ne", "northen-sami": "se", //nolint:misspell // the site's spelling
	"norwegian": "no", "occitan": "oc", "polish": "pl", "portuguese": "pt", "pashto": "ps", "romanian": "ro",
	"russian": "ru", "serbian": "sr", "sindhi": "sd", "sinhala": "si", "slovak": "sk", "slovenian": "sl",
	"somali": "so", "spanish": "es", "swahili": "sw", "swedish": "sv", "tagalog": "tl", "tamil": "ta",
	"tatar": "tt", "telugu": "te", "thai": "th", "turkish": "tr", "turkmen": "tk", "ukranian": "uk", "urdu": "ur", //nolint:misspell // Subscene's spelling
	"uzbek": "uz", "vietnamese": "vi", "welsh": "cy",
	"kinyarwanda": "rw", "punjabi": "pa", "sundanese": "su", "yoruba": "yo",
}

// toSite maps languages to the site's names.
var toSite = sync.OnceValue(func() map[string]string {
	out := map[string]string{}
	for name, tag := range fromSite {
		out[tag] = name
	}
	return out
})

// tags are the provider's languages.
var tags = sync.OnceValue(func() []string {
	out := make([]string, 0, len(fromSite))
	for _, tag := range fromSite {
		out = append(out, tag)
	}
	slices.Sort(out)
	return out
})

// siteName returns the site's name of a language, or of its base
// language; "" when it has none.
func siteName(tag string) string {
	if name, ok := toSite()[tag]; ok {
		return name
	}
	base, _ := language.Make(tag).Base()
	return toSite()[base.String()]
}

type settings struct {
	BaseURL string `json:"base_url"`
}

// New makes the provider from its settings.
func New(env provider.Env, raw json.RawMessage) (provider.Provider, error) {
	var s settings
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, provider.Errorf(provider.ConfigurationError, "settings: %w", err)
		}
	}
	switch {
	case s.BaseURL == "":
		return nil, provider.Errorf(provider.ConfigurationError, "base URL must be specified")
	case !strings.HasPrefix(s.BaseURL, "http://") && !strings.HasPrefix(s.BaseURL, "https://"):
		return nil, provider.Errorf(provider.ConfigurationError, "base URL must include scheme (http:// or https://)")
	}
	sess := site.New(env.HTTP)
	sess.Header.Set("User-Agent", userAgent)
	sess.Check = func(*site.Response) error { return nil }
	return &Provider{env: env, sess: sess, base: strings.TrimRight(s.BaseURL, "/")}, nil
}

// Provider is Subsarr.
type Provider struct {
	env  provider.Env
	sess *site.Session
	base string
}

func (p *Provider) url(path string) string { return p.base + "/api/v1" + path }

type item struct {
	ID          json.RawMessage `json:"id"`
	Language    string          `json:"language"`
	HI          bool            `json:"hi"`
	DownloadURL string          `json:"download_url"`
	Title       string          `json:"title"`
	Releases    any             `json:"releases"`
	Filename    string          `json:"filename"`
}

// search searches the site, retrying failed requests as Bazarr's
// ProviderRetryMixin.
func (p *Provider) search(ctx context.Context, params url.Values) ([]item, error) {
	r, err := site.Retry(ctx, p.env, func() (*site.Response, error) {
		return p.sess.Get(ctx, p.url("/subtitles/search"), params)
	})
	if err != nil {
		return nil, err
	}
	if r.Status != 200 {
		return nil, provider.Errorf(provider.ProviderError, "%s: bad status code: %d", r.URL.Path, r.Status)
	}
	var res struct {
		Items []item `json:"items"`
	}
	if err := r.JSON(&res); err != nil {
		return nil, err
	}
	return res.Items, nil
}

// Search implements provider.Provider, as Bazarr's query: in each of the
// site's languages, by IMDb ID, else by title.
func (p *Provider) Search(ctx context.Context, v *provider.Video, langs []provider.Language) ([]provider.Subtitle, error) {
	title, imdb, season, episode, year := v.Title, v.ExternalIDs["imdb"], 0, 0, 0
	if v.Episode {
		title, imdb, season, episode = v.Series, v.SeriesExternalIDs["imdb"], v.Season, v.Number
	} else {
		year = v.Year
	}
	var names []string
	for _, l := range langs {
		if name := siteName(l.Tag); name != "" && !slices.Contains(names, name) {
			names = append(names, name)
		}
	}

	var out []provider.Subtitle
	for _, name := range names {
		params := url.Values{"language": {name}, "per_page": {"100"}}
		if v.Episode {
			params.Set("season", strconv.Itoa(season))
			params.Set("episode", strconv.Itoa(episode))
		}
		var items []item
		if imdb != "" {
			q := cloneValues(params)
			q.Set("imdb_id", imdb)
			if year > 0 {
				q.Set("year", strconv.Itoa(year))
			}
			var err error
			if items, err = p.search(ctx, q); err != nil {
				return nil, err
			}
		}
		// Without results by IMDb ID, by title.
		if len(items) == 0 && title != "" {
			q := cloneValues(params)
			q.Set("query", title)
			var err error
			if items, err = p.search(ctx, q); err != nil {
				return nil, err
			}
		}
		for _, it := range items {
			tag, ok := fromSite[it.Language]
			if !ok || !slices.ContainsFunc(langs, func(l provider.Language) bool { return l.Tag == tag }) {
				continue
			}
			out = append(out, p.subtitle(v, tag, it))
		}
	}
	return out, nil
}

func cloneValues(v url.Values) url.Values {
	out := url.Values{}
	for k, vs := range v {
		out[k] = slices.Clone(vs)
	}
	return out
}

func (p *Provider) subtitle(v *provider.Video, tag string, it item) provider.Subtitle {
	var releases []string
	if list, ok := it.Releases.([]any); ok {
		for _, r := range list {
			if s, ok := r.(string); ok {
				releases = append(releases, s)
			}
		}
	}
	if len(releases) == 0 {
		releases = []string{it.Filename}
	}
	var m []string
	if v.Episode {
		if v.Series != "" && it.Title != "" && strings.EqualFold(v.Series, it.Title) {
			m = append(m, "series")
		}
		// The subtitles are of the season and episode searched for.
		if v.Season > 0 {
			m = append(m, "season")
		}
		if v.Number > 0 {
			m = append(m, "episode")
		}
	} else if v.Title != "" && it.Title != "" && strings.EqualFold(v.Title, it.Title) {
		m = append(m, "title")
	}
	link := it.DownloadURL
	if u, err := url.Parse(p.base + "/"); err == nil {
		if ref, err := url.Parse(link); err == nil {
			link = u.ResolveReference(ref).String()
		}
	}
	return provider.Subtitle{
		ID:           link,
		Language:     provider.Language{Tag: tag, HearingImpaired: it.HI},
		ReleaseNames: releases,
		Matches:      m,
		PageLink:     link,
	}
}

// Download implements provider.Provider.
func (p *Provider) Download(ctx context.Context, v *provider.Video, id string) (provider.File, error) {
	r, err := site.Retry(ctx, p.env, func() (*site.Response, error) { return p.sess.Get(ctx, id, nil) })
	if err != nil {
		return provider.File{}, err
	}
	if r.Status != 200 {
		return provider.File{}, provider.Errorf(provider.ProviderError, "download failed with status %d", r.Status)
	}
	return archive.Extract(r.Body, archive.WantFor(v, provider.Language{}), provider.Language{})
}
