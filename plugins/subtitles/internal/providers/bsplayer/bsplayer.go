// Package bsplayer is the provider of BSPlayer, ported from Bazarr's
// bsplayer: subtitles of movies and episodes found by the video file's
// OpenSubtitles hash and size through the SOAP API of BSPlayer's
// subtitle servers, served gzipped.
package bsplayer

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"encoding/xml"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
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
const Name = "bsplayer"

// searchThrottle is how long a search waits first, as Bazarr's
// SEARCH_THROTTLE.
const searchThrottle = 8 * time.Second

// tries is how many times a request is tried, as Bazarr's api_request.
const tries = 5

// subDomains are those of the subtitle servers, as Bazarr's.
var subDomains = []string{
	"s1", "s2", "s3", "s4", "s5", "s6", "s7", "s8",
	"s101", "s102", "s103", "s104", "s105", "s106", "s107", "s108", "s109",
}

// languages are the provider's languages.
var languages = []string{
	"pt-BR", "ar", "bg", "cs", "da", "de", "el", "en", "fi", "fr", "hu", "it", "ja", "ko", "nl", "pl", "pt",
	"ro", "ru", "es", "sv", "tr", "uk", "zh",
}

// Definition returns the provider's definition.
func Definition() provider.Definition {
	return provider.Definition{
		Name: Name, DisplayName: "BSplayer",
		Languages:      languages,
		HashVerifiable: true, Movies: true, Episodes: true,
		New: New,
	}
}

// New makes the provider; it takes no settings.
func New(env provider.Env, _ json.RawMessage) (provider.Provider, error) {
	return &Provider{
		env: env, s: site.New(env.HTTP), throttle: searchThrottle,
		server: func(domain string) string { return "http://" + domain + ".api.bsplayer-subtitles.com" },
	}, nil
}

// Provider is BSPlayer.
type Provider struct {
	env provider.Env
	s   *site.Session
	// server returns the URL of a subtitle server.
	server   func(domain string) string
	throttle time.Duration

	mu sync.Mutex
	// api is the working server's API, "" until one is found.
	api   string
	token string
}

// SetBaseURL points the provider at another server, for tests.
func (p *Provider) SetBaseURL(u *url.URL) {
	base := strings.TrimSuffix(u.String(), "/")
	p.server = func(string) string { return base }
}

// openSubtitlesCode is a language's code at OpenSubtitles, which the
// servers take, as babelfish's opensubtitles converter.
func openSubtitlesCode(tag string) string {
	if tag == "pt-BR" {
		return "pob"
	}
	return provider.Alpha3B(tag)
}

// fromOpenSubtitles returns the language of an OpenSubtitles code.
func fromOpenSubtitles(code string) string {
	switch strings.ToLower(code) {
	case "pob", "pb":
		return "pt-BR"
	}
	return provider.FromCode(code)
}

// apiURL returns the API of a working server, as Bazarr's
// get_sub_domain: the first of eight random ones that answers.
func (p *Provider) apiURL(ctx context.Context, fresh bool) (string, error) {
	p.mu.Lock()
	api := p.api
	p.mu.Unlock()
	if api != "" && !fresh {
		return api, nil
	}
	domains := slices.Clone(subDomains)
	rand.Shuffle(len(domains), func(i, j int) { domains[i], domains[j] = domains[j], domains[i] })
	for _, d := range domains[:8] {
		u := p.server(d)
		p.env.Log.DebugContext(ctx, "testing BSPlayer sub-domain", "url", u)
		tctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		r, err := p.s.Get(tctx, u, nil)
		cancel()
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		if r == nil {
			continue
		}
		if err != nil {
			return "", err
		}
		if r.Status == http.StatusOK {
			api = u + "/v1.php"
			p.mu.Lock()
			p.api = api
			p.mu.Unlock()
			return api, nil
		}
		if err := site.Sleep(ctx, p.env.RetryWait); err != nil {
			return "", err
		}
	}
	return "", provider.Errorf(provider.ServiceUnavailable, "no API URL template was found")
}

// envelope is a SOAP answer.
type envelope struct {
	Body struct {
		Response struct {
			Return answer `xml:"return"`
		} `xml:",any"`
	} `xml:"Body"`
}

// answer is the return of a SOAP call.
type answer struct {
	Status string `xml:"status"`
	Result *struct {
		Status string `xml:"status"`
	} `xml:"result"`
	Data struct {
		Text  string `xml:",chardata"`
		Items []item `xml:"item"`
	} `xml:"data"`
}

// item is a subtitle the search found.
type item struct {
	ID       string `xml:"subID"`
	Link     string `xml:"subDownloadLink"`
	Language string `xml:"subLang"`
	Name     string `xml:"subName"`
	Format   string `xml:"subFormat"`
}

// call calls an API function, as Bazarr's api_request: tried 5 times, a
// login failing over to another server.
func (p *Provider) call(ctx context.Context, fn, params string) (*answer, error) {
	for range tries {
		a, err := p.callOnce(ctx, fn, params)
		if err == nil {
			return a, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		p.env.Log.ErrorContext(ctx, "exception parsing response", "err", err)
		if fn == "logIn" {
			if _, err := p.apiURL(ctx, true); err != nil {
				return nil, err
			}
		}
		if err := site.Sleep(ctx, p.env.RetryWait); err != nil {
			return nil, err
		}
	}
	return nil, provider.Errorf(provider.TooManyRequests, "too many retries: %d", tries)
}

func (p *Provider) callOnce(ctx context.Context, fn, params string) (*answer, error) {
	api, err := p.apiURL(ctx, false)
	if err != nil {
		return nil, err
	}
	body := `<?xml version="1.0" encoding="UTF-8"?>` + "\n" +
		`<SOAP-ENV:Envelope xmlns:SOAP-ENV="http://schemas.xmlsoap.org/soap/envelope/" ` +
		`xmlns:SOAP-ENC="http://schemas.xmlsoap.org/soap/encoding/" ` +
		`xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" ` +
		`xmlns:xsd="http://www.w3.org/2001/XMLSchema" xmlns:ns1="` + api + `">` +
		`<SOAP-ENV:Body SOAP-ENV:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">` +
		"<ns1:" + fn + ">" + params + "</ns1:" + fn + "></SOAP-ENV:Body></SOAP-ENV:Envelope>"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, api, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "BSPlayer/2.x (1022.12360)")
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	req.Header.Set("SOAPAction", `"http://api.bsplayer-subtitles.com/v1.php#`+fn+`"`)
	req.Close = true
	r, err := p.s.Do(req)
	if err != nil {
		return nil, err
	}
	var env envelope
	if err := xml.Unmarshal(bytes.TrimSpace(r.Body), &env); err != nil {
		return nil, err
	}
	return &env.Body.Response.Return, nil
}

// login opens a session, as Bazarr's login: its token, "" when the
// server refuses.
func (p *Provider) login(ctx context.Context) (string, error) {
	p.mu.Lock()
	token := p.token
	p.mu.Unlock()
	if token != "" {
		return token, nil
	}
	a, err := p.call(ctx, "logIn", "<username></username><password></password><AppID>BSPlayer v2.67</AppID>")
	if err != nil {
		return "", err
	}
	if a.Status != "OK" {
		return "", nil
	}
	token = strings.TrimSpace(a.Data.Text)
	p.mu.Lock()
	p.token = token
	p.mu.Unlock()
	return token, nil
}

// escape escapes text for XML.
func escape(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

// Search implements provider.Provider, as Bazarr's query; it needs the
// video file's hash.
func (p *Provider) Search(ctx context.Context, v *provider.Video, langs []provider.Language) ([]provider.Subtitle, error) {
	if v.FileHash == "" || len(langs) == 0 {
		return nil, nil
	}
	token, err := p.login(ctx)
	if err != nil {
		return nil, err
	}
	if token == "" {
		p.env.Log.DebugContext(ctx, "token not found; can't perform query")
		return nil, nil
	}
	var codes []string
	for _, l := range langs {
		if c := openSubtitlesCode(l.Tag); c != "" && !slices.Contains(codes, c) {
			codes = append(codes, c)
		}
	}
	slices.Sort(codes)
	imdb := v.ExternalIDs["imdb"]
	if imdb == "" {
		imdb = "*"
	}
	if err := site.Sleep(ctx, p.throttle); err != nil {
		return nil, err
	}
	a, err := p.call(ctx, "searchSubtitles",
		"<handle>"+escape(token)+"</handle>"+
			"<movieHash>"+escape(v.FileHash)+"</movieHash>"+
			"<movieSize>"+strconv.FormatInt(v.FileSize, 10)+"</movieSize>"+
			"<languageId>"+escape(strings.Join(codes, ","))+"</languageId>"+
			"<imdbId>"+escape(imdb)+"</imdbId>")
	if err != nil {
		return nil, err
	}
	if a.Result == nil || a.Result.Status != "OK" {
		p.env.Log.DebugContext(ctx, "no subtitles found")
		return nil, nil
	}
	matches := []string{"title", "year", "hash"}
	if v.Episode {
		matches = []string{"title", "series", "season", "episode", "year", "hash"}
	}
	var out []provider.Subtitle
	for _, it := range a.Data.Items {
		tag := fromOpenSubtitles(it.Language)
		l, ok := find(langs, tag)
		if !ok {
			l = provider.Language{Tag: tag}
		}
		out = append(out, provider.Subtitle{
			ID: it.Link, Language: l, ReleaseNames: []string{it.Name}, Matches: matches,
			Format: strings.ToLower(it.Format), PageLink: it.Link,
		})
	}
	return out, nil
}

// find returns the language asked for of a tag.
func find(langs []provider.Language, tag string) (provider.Language, bool) {
	for _, l := range langs {
		if l.Tag == tag {
			return l, true
		}
	}
	return provider.Language{}, false
}

// Download implements provider.Provider: id is the subtitle's link.
func (p *Provider) Download(ctx context.Context, _ *provider.Video, id string) (provider.File, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, id, nil)
	if err != nil {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "malformed subtitle link %q", id)
	}
	req.Header.Set("User-Agent", "Mozilla/4.0 (compatible; Synapse)")
	r, err := p.s.Do(req)
	if err != nil {
		if r != nil {
			return provider.File{}, provider.Errorf(provider.ServiceUnavailable, "problems connecting to the server: %w", err)
		}
		return provider.File{}, err
	}
	if string(r.Body) == "500" {
		return provider.File{}, provider.Errorf(provider.ServiceUnavailable, "error 500 on server")
	}
	zr, err := gzip.NewReader(bytes.NewReader(r.Body))
	if err != nil {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "not gzipped: %w", err)
	}
	data, err := io.ReadAll(io.LimitReader(zr, site.MaxBody+1))
	if err != nil {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "gunzip: %w", err)
	}
	if len(data) > site.MaxBody {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "larger than %d bytes", site.MaxBody)
	}
	data = archive.FixLineEndings(data)
	return provider.File{Data: data, Format: archive.Detect(data)}, nil
}

// String names the provider in logs.
func (p *Provider) String() string { return Name }
