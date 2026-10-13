// Package gestdown is the provider of Gestdown, ported from Bazarr's
// gestdown: a proxy of Addic7ed's episode subtitles, found by the
// series' TVDB ID, without an account.
package gestdown

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/archive"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/site"
)

// Name is the provider's name.
const Name = "gestdown"

// baseURL serves the API.
const baseURL = "https://api.gestdown.info"

// userAgent identifies the plugin, as Bazarr's "Bazarr".
const userAgent = "Mavio"

// lockedTries and lockedWait are how often and after how long a search
// the API answers 423 Locked is tried again, as Bazarr's _retry_on_423.
const (
	lockedTries = 3
	lockedWait  = 30 * time.Second
)

// siteNames are Addic7ed's names of the languages, as Bazarr's
// PatchedAddic7edConverter: babelfish's English names but for those
// Addic7ed names its own way.
var siteNames = map[string]string{
	"ara": "Arabic", "aze": "Azerbaijani", "ben": "Bengali", "bos": "Bosnian", "bul": "Bulgarian", "cat": "Català",
	"ces": "Czech", "dan": "Danish", "deu": "German", "ell": "Greek", "eng": "English", "eus": "Euskera",
	"fas": "Persian", "fin": "Finnish", "fra": "French", "glg": "Galego", "heb": "Hebrew", "hrv": "Croatian",
	"hun": "Hungarian", "hye": "Armenian", "ind": "Indonesian", "ita": "Italian", "jpn": "Japanese", "kor": "Korean",
	"mkd": "Macedonian", "msa": "Malay", "nld": "Dutch", "nor": "Norwegian", "pol": "Polish", "por": "Portuguese",
	"ron": "Romanian", "rus": "Russian", "slk": "Slovak", "slv": "Slovenian", "spa": "Spanish", "sqi": "Albanian",
	"srp": "Serbian", "swe": "Swedish", "tha": "Thai", "tur": "Turkish", "ukr": "Ukrainian", "vie": "Vietnamese",
	"zho": "Chinese (Simplified)",
}

// siteName returns Addic7ed's name of a language; "" when it has none.
// Its regional variants have their own names.
func siteName(tag string) string {
	switch tag {
	case "pt-BR":
		return "Portuguese (Brazilian)"
	case "sr-Cyrl":
		return "Serbian (Cyrillic)"
	case "zh-Hant", "zh-TW":
		return "Chinese (Traditional)"
	}
	base, _, _ := strings.Cut(tag, "-")
	if provider.FromCode(base) == "" {
		return ""
	}
	return siteNames[provider.Alpha3(tag)]
}

// Definition returns the provider's definition.
func Definition() provider.Definition {
	langs := []string{"pt-BR", "sr-Latn", "sr-Cyrl"}
	for a3 := range siteNames {
		langs = append(langs, provider.FromCode(a3))
	}
	slices.Sort(langs)
	return provider.Definition{
		Name: Name, DisplayName: "Gestdown (Addic7ed proxy)",
		Languages:                 langs,
		HearingImpairedVerifiable: true,
		Episodes:                  true,
		New:                       New,
	}
}

// New makes the provider; it has no settings.
func New(env provider.Env, _ json.RawMessage) (provider.Provider, error) {
	sess := site.New(env.HTTP)
	sess.Header.Set("User-Agent", userAgent)
	sess.Check = func(r *site.Response) error {
		if r.Status == http.StatusLocked {
			return errLocked
		}
		return site.Status(r)
	}
	return &Provider{env: env, sess: sess, base: baseURL, wait: lockedWait}, nil
}

// errLocked is the API's 423 Locked: it is still fetching the episode.
var errLocked = errors.New("locked")

// Provider is Gestdown.
type Provider struct {
	env  provider.Env
	sess *site.Session
	base string
	wait time.Duration
}

// SetBaseURL points the provider at another server, for tests.
func (p *Provider) SetBaseURL(u *url.URL) { p.base = strings.TrimSuffix(u.String(), "/") }

// show is a show of the API.
type show struct {
	ID     string `json:"id"`
	TvDbID int    `json:"tvDbId"`
}

// subtitle is a subtitle of the API.
type subtitle struct {
	SubtitleID      string   `json:"subtitleId"`
	Version         string   `json:"version"`
	Completed       bool     `json:"completed"`
	HearingImpaired bool     `json:"hearingImpaired"`
	DownloadURI     string   `json:"downloadUri"`
	Qualities       []string `json:"qualities"`
}

// Search implements provider.Provider, trying again the searches the API
// is still preparing, as Bazarr's _retry_on_423; none when it is still
// preparing them.
func (p *Provider) Search(ctx context.Context, v *provider.Video, langs []provider.Language) ([]provider.Subtitle, error) {
	if !v.Episode || v.SeriesExternalIDs["tvdb"] == "" {
		return nil, nil
	}
	for range lockedTries {
		subs, err := p.search(ctx, v, langs)
		if !errors.Is(err, errLocked) {
			return subs, err
		}
		p.env.Log.DebugContext(ctx, "423 returned; retrying", "wait", p.wait)
		if err := site.Sleep(ctx, p.wait); err != nil {
			return nil, err
		}
	}
	p.env.Log.DebugContext(ctx, "retries limit exceeded; ignoring query")
	return nil, nil
}

// search is Bazarr's list_subtitles.
func (p *Provider) search(ctx context.Context, v *provider.Video, langs []provider.Language) ([]provider.Subtitle, error) {
	shows, err := p.shows(ctx, v)
	if err != nil || shows == nil {
		return nil, err
	}
	var out []provider.Subtitle
	searched := map[string]bool{}
	for _, l := range langs {
		name := siteName(l.Tag)
		if name == "" || searched[name] {
			continue
		}
		searched[name] = true
		for _, s := range shows {
			subs, err := p.subtitles(ctx, v, l, name, s)
			if errors.Is(err, errNotFound) {
				return nil, nil
			}
			if err != nil {
				return nil, err
			}
			out = append(out, subs...)
		}
	}
	return out, nil
}

// shows returns the shows of the series' TVDB ID, as Bazarr's
// _search_show; nil for none.
func (p *Provider) shows(ctx context.Context, v *provider.Video) ([]show, error) {
	r, err := p.sess.Get(ctx, p.base+"/shows/external/tvdb/"+url.PathEscape(v.SeriesExternalIDs["tvdb"]), nil)
	if r != nil && r.Status == http.StatusNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var res struct {
		Shows []show `json:"shows"`
	}
	if err := json.Unmarshal(r.Body, &res); err != nil {
		p.env.Log.DebugContext(ctx, "couldn't parse the shows", "series", v.Series, "error", err)
		return nil, nil
	}
	if res.Shows == nil {
		p.env.Log.DebugContext(ctx, "couldn't find the show", "series", v.Series)
	}
	return res.Shows, nil
}

// errNotFound is the API's answer that it has no such show, season or
// episode, which empties the search, as Bazarr's.
var errNotFound = errors.New("not found")

// subtitles returns the subtitles of an episode of a show in a language,
// as Bazarr's _subtitles_search.
func (p *Provider) subtitles(ctx context.Context, v *provider.Video, l provider.Language, name string, s show) ([]provider.Subtitle, error) {
	u := fmt.Sprintf("%s/subtitles/get/%s/%d/%d/%s", p.base, url.PathEscape(s.ID), v.Season, v.Number, url.PathEscape(name))
	r, err := p.sess.Get(ctx, u, nil)
	if r != nil && r.Status == http.StatusNotFound {
		p.env.Log.DebugContext(ctx, "couldn't find the show or its season/episode", "series", v.Series)
		return nil, errNotFound
	}
	if err != nil {
		return nil, err
	}
	var res struct {
		MatchingSubtitles *[]subtitle `json:"matchingSubtitles"`
	}
	if err := json.Unmarshal(r.Body, &res); err != nil || res.MatchingSubtitles == nil {
		p.env.Log.DebugContext(ctx, "couldn't get matching subtitles", "series", v.Series, "error", err)
		return nil, nil
	}
	tvdbMatching := v.SeriesExternalIDs["tvdb"] == strconv.Itoa(s.TvDbID)
	var out []provider.Subtitle
	for _, sub := range *res.MatchingSubtitles {
		if !sub.Completed {
			continue
		}
		var releases []string
		for rel := range strings.SplitSeq(sub.Version, ",") {
			releases = append(releases, strings.TrimSpace(rel))
		}
		matches := []string{"title", "series", "season", "episode", "tvdb_id"}
		if v.Resolution != "" && slices.Contains(sub.Qualities, v.Resolution) {
			matches = append(matches, "resolution")
		}
		if tvdbMatching {
			matches = append(matches, "year")
		}
		out = append(out, provider.Subtitle{
			ID:           sub.DownloadURI,
			Language:     provider.Language{Tag: l.Tag, HearingImpaired: sub.HearingImpaired},
			ReleaseNames: releases,
			Matches:      matches,
			PageLink:     p.base + sub.DownloadURI,
		})
	}
	return out, nil
}

// Download implements provider.Provider.
func (p *Provider) Download(ctx context.Context, _ *provider.Video, id string) (provider.File, error) {
	if !strings.HasPrefix(id, "/") {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "malformed subtitle ID %q", id)
	}
	r, err := p.sess.Get(ctx, p.base+id, nil)
	if errors.Is(err, errLocked) {
		return provider.File{}, provider.Errorf(provider.ProviderError, "%s: locked", id)
	}
	if err != nil {
		return provider.File{}, err
	}
	if len(r.Body) == 0 {
		return provider.File{}, provider.Errorf(provider.ProviderError, "%s: empty subtitle", id)
	}
	return provider.File{Data: r.Body, Format: archive.Detect(r.Body)}, nil
}
