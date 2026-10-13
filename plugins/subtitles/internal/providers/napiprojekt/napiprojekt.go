// Package napiprojekt is the provider of NapiProjekt, ported from Bazarr's
// napiprojekt: Polish subtitles of movies and episodes found by the video
// file's NapiProjekt hash, and scraped from the site's catalogue.
//
// The NapiProjekt hash is the MD5 of the file's first 10 MiB, as
// subliminal's hash_napiprojekt; the server gives it as the video's
// "napiprojekt" external ID.
package napiprojekt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/net/html"

	"github.com/mavioai/mavio/plugins/subtitles/internal/archive"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/site"
)

// Name is the provider's name.
const Name = "napiprojekt"

// language is the provider's only language.
const language = "pl"

// HashID is the key of the video's external IDs that has the NapiProjekt
// hash of its file.
const HashID = "napiprojekt"

// retries is how many times a request is tried, as Bazarr's
// QUERY_RETRIES.
const retries = 3

// Definition returns the provider's definition.
func Definition() provider.Definition {
	return provider.Definition{
		Name: Name, DisplayName: "NapiProjekt",
		Settings: []provider.Setting{
			{
				Name: "hash_only", Label: "Hash-only search", Type: provider.Checkbox, Default: false,
				Help: "Hash-only search is a temporary workaround for catalogue search failures. It searches only for subtitles matching your video file's fingerprint and may return fewer results. Subtitles visible on the NapiProjekt website may not appear. Author filters are ignored while enabled.",
			},
			{Name: "only_authors", Label: "Skip subtitles without authors or possibly AI generated", Type: provider.Checkbox, Default: false},
			{Name: "only_real_names", Label: "Download subtitles with real name authors only", Type: provider.Checkbox, Default: false},
		},
		Languages:      []string{language},
		HashVerifiable: true, Movies: true, Episodes: true,
		New: New,
	}
}

// settings are the provider's settings.
type settings struct {
	HashOnly      bool `json:"hash_only"`
	OnlyAuthors   bool `json:"only_authors"`
	OnlyRealNames bool `json:"only_real_names"`
}

// New makes the provider from its settings.
func New(env provider.Env, raw json.RawMessage) (provider.Provider, error) {
	var s settings
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, provider.Errorf(provider.ConfigurationError, "settings: %w", err)
		}
	}
	ss := site.New(env.HTTP)
	ss.Header.Set("User-Agent", "Subliminal/2.1")
	return &Provider{env: env, s: s, site: ss, api: "https://napiprojekt.pl", www: "https://www.napiprojekt.pl"}, nil
}

// Provider is NapiProjekt.
type Provider struct {
	env  provider.Env
	s    settings
	site *site.Session
	// api serves subtitles by hash, www the catalogue.
	api, www string
}

// SetBaseURL points the provider at another server, for tests.
func (p *Provider) SetBaseURL(u *url.URL) {
	p.api = strings.TrimSuffix(u.String(), "/")
	p.www = p.api
}

// subhash returns the second hash the site asks for, as subliminal's
// get_subhash.
func subhash(hash string) (string, bool) {
	idx := []int{0xe, 0x3, 0x6, 0x8, 0x2}
	mul := []int{2, 2, 5, 4, 3}
	add := []int{0, 0xd, 0x10, 0xb, 0x5}
	var b strings.Builder
	for i := range idx {
		if idx[i] >= len(hash) {
			return "", false
		}
		d, err := strconv.ParseInt(hash[idx[i]:idx[i]+1], 16, 0)
		if err != nil {
			return "", false
		}
		t := add[i] + int(d)
		if t+2 > len(hash) {
			return "", false
		}
		v, err := strconv.ParseInt(hash[t:t+2], 16, 0)
		if err != nil {
			return "", false
		}
		s := strconv.FormatInt(v*int64(mul[i]), 16)
		b.WriteByte(s[len(s)-1])
	}
	return b.String(), true
}

// retry calls f as Bazarr's ProviderRetryMixin.
func (p *Provider) retry(ctx context.Context, f func() (*site.Response, error)) (*site.Response, error) {
	return provider.Retry(ctx, retries, p.env.RetryWait, f)
}

// query returns the subtitle of a hash, as Bazarr's query; nil when the
// site has none.
func (p *Provider) query(ctx context.Context, hash string) ([]byte, error) {
	t, ok := subhash(hash)
	if !ok {
		return nil, provider.Errorf(provider.BadSubtitle, "malformed hash %q", hash)
	}
	params := url.Values{
		"v": {"dreambox"}, "kolejka": {"false"}, "nick": {""}, "pass": {""}, "napios": {"Linux"},
		"l": {strings.ToUpper(language)}, "f": {hash}, "t": {t},
	}
	p.env.Log.InfoContext(ctx, "searching subtitle", "hash", hash)
	r, err := p.retry(ctx, func() (*site.Response, error) { return p.site.Get(ctx, p.api+"/unit_napisy/dl.php", params) })
	if err != nil {
		return nil, err
	}
	if bytes.HasPrefix(r.Body, []byte("NPc0")) {
		p.env.Log.DebugContext(ctx, "no subtitles found")
		return nil, nil
	}
	return r.Body, nil
}

// Search implements provider.Provider, as Bazarr's list_subtitles: the
// subtitle of the file's hash, unless authors are filtered, and those of
// the catalogue, unless only the hash is searched.
func (p *Provider) Search(ctx context.Context, v *provider.Video, langs []provider.Language) ([]provider.Subtitle, error) {
	hash := strings.ToLower(v.ExternalIDs[HashID])
	var out []provider.Subtitle
	if hash != "" && (p.s.HashOnly || !p.s.OnlyAuthors && !p.s.OnlyRealNames) {
		for _, l := range langs {
			if l.Tag != language {
				continue
			}
			data, err := p.query(ctx, hash)
			if err != nil {
				return nil, err
			}
			if data != nil {
				out = append(out, provider.Subtitle{ID: hash, Language: l, ReleaseNames: []string{hash}, Matches: []string{"hash"}})
			}
		}
	}
	if p.s.HashOnly {
		return out, nil
	}
	for _, l := range langs {
		if l.Tag != language {
			continue
		}
		subs, err := p.scrape(ctx, v, l, hash)
		if err != nil {
			return nil, err
		}
		out = append(out, subs...)
	}
	return out, nil
}

// authorLabel is the site's label of authors, in Polish.
const authorLabel = "Autor" //nolint:misspell // Polish

var (
	authorRe     = regexp.MustCompile(`<b>` + authorLabel + `:</b> (.*?)\(`)
	resolutionRe = regexp.MustCompile(`<b>Video rozdzielczość:</b> (.*?)<`)
	fpsRe        = regexp.MustCompile(`<b>Video FPS:</b> (.*?)<`)
	twoWordsRe   = regexp.MustCompile(`^[\p{L}\p{N}_]+\s[\p{L}\p{N}_]+$`)
)

// machineAuthors are the authors of subtitles that are not, or are
// possibly machine-made.
var machineAuthors = []string{"brak", "automat", "si", "chatgpt", "ai", "robot", "maszynowe", "tłumaczenie maszynowe"}

// realName reports whether an author is a real name, as Bazarr's: two
// capitals and a lowercase letter, or two words.
func realName(author string) bool {
	upper, lower := 0, false
	for _, r := range author {
		switch {
		case r >= 'A' && r <= 'Z':
			upper++
		case r >= 'a' && r <= 'z':
			lower = true
		}
	}
	return upper >= 2 && lower || twoWordsRe.MatchString(author)
}

// firstText returns the text of an element's first child, as
// BeautifulSoup's contents[0].
func firstText(n site.Node) string {
	if n.Node == nil || n.FirstChild == nil {
		return ""
	}
	c := n.FirstChild
	if c.Type == html.TextNode {
		return c.Data
	}
	return site.Node{Node: c}.Text()
}

// match returns the first group of a pattern in s, trimmed; "" for none.
func match(re *regexp.Regexp, s string) string {
	if m := re.FindStringSubmatch(s); m != nil {
		return strings.TrimSpace(m[1])
	}
	return ""
}

// scrape lists the subtitles of the video in the site's catalogue, as
// Bazarr's _scrape.
func (p *Provider) scrape(ctx context.Context, v *provider.Video, l provider.Language, hash string) ([]provider.Subtitle, error) {
	title, matches, err := p.findTitle(ctx, v)
	if err != nil || title == "" {
		return nil, err
	}
	episode := ""
	if v.Episode {
		episode = fmt.Sprintf("-s%02de%02d", v.Season, v.Number)
		matches = append(matches, "season", "episode")
	}
	r, err := p.retry(ctx, func() (*site.Response, error) {
		return p.site.Get(ctx, p.www+"/napisy1,7,0-dla-"+title+episode, nil)
	})
	if err != nil {
		return nil, err
	}
	doc, err := r.HTML()
	if err != nil {
		return nil, provider.Errorf(provider.ParseResponseError, "subtitles page: %w", err)
	}
	var out []provider.Subtitle
	for _, row := range doc.All("tr[title]") {
		for _, link := range row.All("a.tableA") {
			info := row.Attr("title")
			subHash := strings.TrimPrefix(link.Attr("href"), "napiprojekt:")
			data := row.All("p")
			field := func(i int) string {
				if i < len(data) {
					return firstText(data[i])
				}
				return ""
			}
			size, length, author, added := field(1), field(3), field(4), field(5)
			if author == "" {
				author = match(authorRe, info)
			}
			if p.s.OnlyAuthors && slices.Contains(machineAuthors, strings.ToLower(author)) {
				continue
			}
			if p.s.OnlyRealNames && !realName(author) {
				continue
			}
			release := authorLabel + ": " + author + " | " + match(resolutionRe, info) + " | " + match(fpsRe, info) + " | " + size + " | " + added + " | " + length
			m := append([]string(nil), matches...)
			if hash != "" && subHash == hash {
				m = append(m, "hash")
			}
			out = append(out, provider.Subtitle{
				ID: subHash, Language: l, ReleaseNames: []string{release}, Matches: m, Uploader: author,
			})
		}
	}
	p.env.Log.DebugContext(ctx, "found subtitles", "count", len(out))
	return out, nil
}

// findTitle returns the catalogue's page of the movie or series and what
// it matches, as Bazarr's _find_title: the title of the video's IMDb ID,
// else the first found; "" for none.
func (p *Provider) findTitle(ctx context.Context, v *provider.Video) (string, []string, error) {
	query, kind, year := v.Title, "2", ""
	if v.Episode {
		query, kind = v.Series, "1"
	}
	if v.Year != 0 {
		year = strconv.Itoa(v.Year)
	}
	form := url.Values{"queryString": {query}, "queryKind": {kind}, "queryYear": {year}, "associate": {""}}
	r, err := p.retry(ctx, func() (*site.Response, error) {
		return p.site.PostForm(ctx, p.www+"/ajax/search_catalog.php", form)
	})
	if err != nil {
		return "", nil, err
	}
	doc, err := r.HTML()
	if err != nil {
		return "", nil, provider.Errorf(provider.ParseResponseError, "catalogue: %w", err)
	}
	if imdb := v.IMDbID(); imdb != "" {
		for _, entry := range doc.All("div.greyBoxCatcher") {
			if !hasLink(entry, "https://www.imdb.com/title/"+imdb) {
				continue
			}
			if link := entry.First("a.movieTitleCat[href]"); link.Valid() {
				matches := []string{"title", "year", "imdb_id"}
				if v.Episode {
					matches = []string{"series", "year", "series_imdb_id"}
				}
				return strings.TrimPrefix(link.Attr("href"), "napisy-"), matches, nil
			}
		}
	}
	link := doc.First("a.movieTitleCat[href]")
	if !link.Valid() {
		return "", nil, nil
	}
	// The catalogue's title is matched as is; Bazarr also guesses matches
	// from it, which the server does from release names.
	title := firstText(link)
	var matches []string
	if v.Year != 0 {
		matches = append(matches, "year")
	}
	switch {
	case v.Episode && title == v.Series:
		matches = append(matches, "series")
	case !v.Episode && title == v.Title:
		matches = append(matches, "title")
	}
	return strings.TrimPrefix(link.Attr("href"), "napisy-"), matches, nil
}

// hasLink reports whether an element has a link starting with prefix.
func hasLink(n site.Node, prefix string) bool {
	for _, a := range n.All("[href]") {
		if strings.HasPrefix(a.Attr("href"), prefix) {
			return true
		}
	}
	return false
}

// Download implements provider.Provider: id is the hash the site has the
// subtitle for.
func (p *Provider) Download(ctx context.Context, _ *provider.Video, id string) (provider.File, error) {
	data, err := p.query(ctx, strings.ToLower(id))
	if err != nil {
		return provider.File{}, err
	}
	if data == nil {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "no subtitle for hash %s", id)
	}
	return provider.File{Data: data, Format: archive.Detect(data), Language: provider.Language{Tag: language}}, nil
}

// String names the provider in logs.
func (p *Provider) String() string { return Name }
