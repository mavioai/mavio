// Package zimuku is the provider of Zimuku, ported from Bazarr's zimuku:
// Chinese subtitles behind the site's Yunsuo verification, whose numeric
// image captchas are solved with the captcha solving service.
package zimuku

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"unicode"

	"github.com/mavioai/mavio/plugins/subtitles/internal/archive"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/site"
	"github.com/mavioai/mavio/plugins/subtitles/internal/sitekit"
)

// Name is the provider's name.
const Name = "zimuku"

const (
	// bypassTries bounds the verifications of a page, which Bazarr does
	// not.
	bypassTries = 5
	// redirects bounds the script redirects of a search, which Bazarr
	// does not.
	redirects = 10
)

// Definition returns the provider's definition.
func Definition() provider.Definition {
	return provider.Definition{
		Name: Name, DisplayName: "Zimuku",
		Languages: []string{"zh", "zh-Hans", "zh-Hant", "en"},
		Movies:    true, Episodes: true,
		New: New,
	}
}

// New makes the provider; it has no settings.
func New(env provider.Env, _ json.RawMessage) (provider.Provider, error) {
	base, _ := url.Parse("https://srtku.com")
	s := site.NewBrowser(env.HTTP)
	// Statuses are checked where Bazarr does.
	s.Check = func(*site.Response) error { return nil }
	// The verification is sent without following its redirect.
	direct := *env.HTTP
	direct.Jar = s.Jar()
	direct.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &Provider{env: env, base: base, sess: s, direct: &direct}, nil
}

// Provider is Zimuku.
type Provider struct {
	env    provider.Env
	base   *url.URL
	sess   *site.Session
	direct *http.Client

	mu sync.Mutex
}

// SetBaseURL points the provider at another server, for tests.
func (p *Provider) SetBaseURL(u *url.URL) { p.base = u }

var (
	locationRe = regexp.MustCompile(`self\.location = "(.*)" \+ stringToHex\(`)
	imageRe    = regexp.MustCompile(`<img.*?src="data:image/bmp;base64,(.*?)".*?>`)
	redirectRe = regexp.MustCompile(`url\s*=\s*'([^']*)'\s*\+\s*url`)
	seasonRe   = regexp.MustCompile(`第(.*)季`)
	yearRe     = regexp.MustCompile(`\d{4}`)
)

// Search implements provider.Provider, as Bazarr's list_subtitles: every
// title is searched.
func (p *Provider) Search(ctx context.Context, v *provider.Video, langs []provider.Language) ([]provider.Subtitle, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	titles := append([]string{v.Title}, v.AlternativeTitles...)
	season := 0
	if v.Episode {
		titles, season = append([]string{v.Series}, v.AlternativeSeries...), v.Season
	}
	var subs []provider.Subtitle
	for _, title := range titles {
		found, err := p.query(ctx, v, title, season, langs)
		if err != nil {
			return nil, err
		}
		subs = append(subs, found...)
	}
	return subs, nil
}

func (p *Provider) resolve(ref string) string {
	u, err := url.Parse(ref)
	if err != nil {
		return p.base.String() + ref
	}
	return p.base.ResolveReference(u).String()
}

// query searches a title, as Bazarr's query; shooter result pages are
// ignored, as Bazarr's.
func (p *Provider) query(ctx context.Context, v *provider.Video, keyword string, season int, langs []provider.Language) ([]provider.Subtitle, error) {
	params := keyword
	if season != 0 {
		params += fmt.Sprintf(".S%02d", season)
	} else if v.Year != 0 {
		params += fmt.Sprintf(" %4d", v.Year)
	}
	p.env.Log.DebugContext(ctx, "zimuku: searching", "query", params)
	link := p.resolve("/search?q=" + strings.ReplaceAll(url.QueryEscape(params), "+", "%20"))
	r, err := p.bypass(ctx, link, "")
	if err != nil {
		return nil, err
	}
	if err := site.Status(r); err != nil {
		return nil, err
	}
	if len(r.Body) == 0 {
		return nil, nil
	}
	// The script redirects of the search.
	for range redirects {
		parts := redirectRe.FindAllStringSubmatch(string(r.Body), -1)
		if len(parts) == 0 {
			break
		}
		var b strings.Builder
		for _, part := range slices.Backward(parts) {
			b.WriteString(part[1])
		}
		if r, err = p.get(ctx, p.resolve(b.String()), ""); err != nil {
			return nil, err
		}
	}
	doc, err := r.HTML()
	if err != nil {
		return nil, provider.Errorf(provider.ParseResponseError, "search: %w", err)
	}

	var subs []provider.Subtitle
	for _, item := range doc.All("div.item") {
		a := item.First("p.tt.clearfix a")
		if !a.Valid() {
			return nil, provider.Errorf(provider.ParseResponseError, "search: an item without a title")
		}
		year := v.Year
		if season != 0 {
			title := a.Text()
			// The year of an episode's page is its season's.
			if y := yearRe.FindString(title); y != "" {
				n, _ := strconv.Atoi(y)
				year = n - season + 1
			}
			cn := "一"
			if m := seasonRe.FindStringSubmatch(title); m != nil {
				cn = strings.TrimSpace(m[1])
			}
			if cn != numToCN(season) {
				continue
			}
		}
		found, err := p.episodePage(ctx, v, p.resolve(a.Attr("href")), year, langs)
		if err != nil {
			return nil, err
		}
		subs = append(subs, found...)
	}
	return subs, nil
}

// episodePage lists the subtitles of a movie's or season's page, as
// Bazarr's _parse_episode_page: a subtitle in several languages is one
// subtitle of each.
func (p *Provider) episodePage(ctx context.Context, v *provider.Video, link string, year int, langs []provider.Language) ([]provider.Subtitle, error) {
	r, err := p.bypass(ctx, link, "")
	if err != nil {
		return nil, err
	}
	doc, err := r.HTML()
	if err != nil {
		return nil, provider.Errorf(provider.ParseResponseError, "page: %w", err)
	}
	body := doc.First("tbody")
	if !body.Valid() {
		return nil, provider.Errorf(provider.ParseResponseError, "%s: no subtitle list", link)
	}
	var matches []string
	if v.Year == year {
		matches = append(matches, "year")
	}
	var subs []provider.Subtitle
	for _, row := range body.All("tr") {
		a := row.First("a")
		if !a.Valid() {
			continue
		}
		name, _ := splitExt(extractName(a.Text()))
		var tags []string
		for _, img := range row.All("td.tac.lang img") {
			src := img.Attr("src")
			switch china, hongkong := strings.Contains(src, "china"), strings.Contains(src, "hongkong"); {
			case china && hongkong:
				// Bazarr fails on these, adding the languages up.
				tags = append(tags, "zh", "zh-Hant")
			case china || strings.Contains(src, "jollyroger"):
				tags = append(tags, "zh")
			case hongkong:
				tags = append(tags, "zh-Hant")
			}
		}
		page := p.resolve(a.Attr("href"))
		for _, tag := range tags {
			lang, ok := sitekit.Match(langs, tag)
			if !ok {
				continue
			}
			s := provider.Subtitle{ID: page, Language: lang, PageLink: page, Matches: slices.Clone(matches)}
			if name != "" {
				s.ReleaseNames = []string{name}
			}
			subs = append(subs, s)
		}
	}
	return subs, nil
}

func (p *Provider) get(ctx context.Context, u, referer string) (*site.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", u, err)
	}
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	return p.sess.Do(req)
}

// bypass gets a page through the site's Yunsuo verification, as Bazarr's
// yunsuo_bypass: a page that is not found asks for the verification,
// whose code is a captcha image's text or random coordinates.
func (p *Provider) bypass(ctx context.Context, u, referer string) (*site.Response, error) {
	for range bypassTries {
		r, err := p.get(ctx, u, referer)
		if err != nil {
			return nil, err
		}
		page := string(r.Body)
		if r.Status == http.StatusNotFound {
			var code string
			if m := imageRe.FindStringSubmatch(page); m != nil {
				if code, err = p.solve(ctx, m[1]); err != nil {
					return nil, err
				}
			} else {
				code = fmt.Sprintf("%d,%d", 800+rand.IntN(1120), 600+rand.IntN(480)) //nolint:gosec // Coordinates of a click.
			}
			p.sess.Jar().SetCookies(p.base, []*http.Cookie{{Name: "srcurl", Value: toHex(r.URL.String())}})
			if m := locationRe.FindStringSubmatch(page); m != nil {
				if err := p.verify(ctx, p.resolve(m[1]+toHex(code))); err != nil {
					return nil, err
				}
				continue
			}
		}
		if !locationRe.MatchString(page) {
			return r, nil
		}
	}
	return nil, provider.Errorf(provider.ProviderError, "%s: the site's verification failed", u)
}

// solve reads the captcha image of the verification.
func (p *Provider) solve(ctx context.Context, image string) (string, error) {
	solver, err := sitekit.Captcha(p.env, "Zimuku")
	if err != nil {
		return "", err
	}
	bmp, err := base64.StdEncoding.DecodeString(image)
	if err != nil {
		return "", provider.Errorf(provider.ParseResponseError, "captcha image: %w", err)
	}
	png, err := bmpToPNG(bmp)
	if err != nil {
		return "", provider.Errorf(provider.ParseResponseError, "captcha image: %w", err)
	}
	p.env.Log.InfoContext(ctx, "zimuku: solving a captcha")
	code, err := solver.Image(ctx, png)
	if err != nil {
		return "", fmt.Errorf("zimuku: couldn't solve the captcha: %w", err)
	}
	return code, nil
}

// verify sends the verification's code, not following its redirect.
func (p *Provider) verify(ctx context.Context, u string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return fmt.Errorf("%s: %w", u, err)
	}
	req.Header.Set("User-Agent", p.sess.Header.Get("User-Agent"))
	resp, err := p.direct.Do(req)
	if err != nil {
		return provider.Errorf(provider.ServiceUnavailable, "verification: %w", err)
	}
	defer resp.Body.Close()
	if _, err := io.Copy(io.Discard, io.LimitReader(resp.Body, site.MaxBody)); err != nil {
		return provider.Errorf(provider.ServiceUnavailable, "verification: %w", err)
	}
	return nil
}

// toHex is the site's stringToHex, as Bazarr's string_to_hex: the code
// points in hexadecimal, unpadded.
func toHex(s string) string {
	var b strings.Builder
	for _, r := range s {
		b.WriteString(strconv.FormatInt(int64(r), 16))
	}
	return b.String()
}

// numToCN writes a number of 1 to 99 in Chinese, as Bazarr's num_to_cn;
// "" for others.
func numToCN(n int) string {
	digits := []string{"", "一", "二", "三", "四", "五", "六", "七", "八", "九"}
	switch {
	case n < 1 || n > 99:
		return ""
	case n < 10:
		return digits[n]
	}
	tens := "十"
	if n/10 != 1 {
		tens = digits[n/10] + tens
	}
	return tens + digits[n%10]
}

// splitExt splits a name's extension, as Python's os.path.splitext.
func splitExt(name string) (string, string) {
	lead := len(name) - len(strings.TrimLeft(name, "."))
	if i := strings.LastIndexByte(name, '.'); i > lead {
		return name[:i], name[i:]
	}
	return name, ""
}

func isHan(r rune) bool   { return r >= 0x4e00 && r <= 0x9fff }
func isLatin(r rune) bool { return r < unicode.MaxASCII && unicode.IsLetter(r) }

// extractName filters the Chinese out of a subtitle's name, as Bazarr's
// _extract_name: the Latin part before or after the Chinese, or the
// longest Latin run between Chinese; "" for a name without Latin letters.
func extractName(full string) string {
	stem, suffix := splitExt(full)
	name := []rune(stem)
	var latin, han []int
	for i, r := range name {
		switch {
		case isLatin(r):
			latin = append(latin, i)
		case isHan(r):
			han = append(han, i)
		}
	}
	if len(latin) == 0 {
		return ""
	}
	first, last := latin[0], latin[len(latin)-1]
	firstHan, lastHan := -1, -1
	if len(han) > 0 {
		firstHan, lastHan = han[0], han[len(han)-1]
	}
	var out []rune
	switch {
	case lastHan < first:
		out = name[first:]
	case last < firstHan:
		out = name[:firstHan]
	default:
		isL := func(i int) bool { return isLatin(name[i]) }
		isH := func(i int) bool { return isHan(name[i]) }
		from, to, end := 0, 1, 0
		for end < len(name) {
			for end < len(name) && !isL(end) {
				end++
			}
			if end == len(name) {
				break
			}
			start := end
			for end < len(name) && !isH(end) {
				end++
			}
			if end-start > to-from {
				from, to = start, end
			}
			end++
		}
		out = name[from:to]
	}
	return strings.TrimSpace(string(out)) + suffix
}

// Download implements provider.Provider, as Bazarr's download_subtitle:
// the subtitle's page leads to its download page, which leads to the file.
func (p *Provider) Download(ctx context.Context, _ *provider.Video, id string) (provider.File, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	down, err := p.link(ctx, id, id, "a#down1")
	if err != nil {
		return provider.File{}, err
	}
	link, err := p.link(ctx, id, down, `a[rel="nofollow"]`)
	if err != nil {
		return provider.File{}, err
	}
	r, err := p.bypass(ctx, link, id)
	if err != nil {
		return provider.File{}, err
	}
	if err := site.Status(r); err != nil {
		return provider.File{}, err
	}
	filename := strings.ToLower(r.Header.Get("Content-Disposition"))
	if filename == "" {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "no file name")
	}
	if len(r.Body) == 0 {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "no data returned")
	}
	data, err := extract(r.Body, filename)
	if err != nil {
		return provider.File{}, err
	}
	data = archive.FixLineEndings(data)
	return provider.File{Data: data, Format: archive.Detect(data)}, nil
}

// link follows a link of a page, relative to the subtitle's page.
func (p *Provider) link(ctx context.Context, sub, page, selector string) (string, error) {
	r, err := p.bypass(ctx, page, "")
	if err != nil {
		return "", err
	}
	doc, err := r.HTML()
	if err != nil {
		return "", provider.Errorf(provider.ParseResponseError, "download: %w", err)
	}
	a := doc.First(selector)
	if !a.Valid() {
		return "", provider.Errorf(provider.ParseResponseError, "%s: no download link", page)
	}
	base, err := url.Parse(sub)
	if err != nil {
		return "", fmt.Errorf("%s: %w", sub, err)
	}
	ref, err := url.Parse(a.Attr("href"))
	if err != nil {
		return "", provider.Errorf(provider.ParseResponseError, "download link: %w", err)
	}
	return base.ResolveReference(ref).String(), nil
}

// subtitleExtensions are subliminal's SUBTITLE_EXTENSIONS.
var subtitleExtensions = []string{".srt", ".sub", ".smi", ".txt", ".ssa", ".ass", ".mpl"}

var rarMagic = []byte("Rar!\x1a\x07")

// extract returns the subtitle of a download: of an archive, whose file
// name must say so, the best scored, or the download when its file name
// is a subtitle's.
func extract(data []byte, filename string) ([]byte, error) {
	kind := ""
	if bytes.HasPrefix(data, rarMagic) {
		kind = ".rar"
	} else if isZip(data) {
		kind = ".zip"
	}
	if kind == "" {
		if !slices.ContainsFunc(subtitleExtensions, func(ext string) bool { return strings.Contains(filename, ext) }) {
			return nil, provider.Errorf(provider.BadSubtitle, "unknown subtitle extension in %q", filename)
		}
		return data, nil
	}
	if !strings.Contains(filename, kind) {
		return nil, provider.Errorf(provider.BadSubtitle, "%s should be in the file name %q", kind, filename)
	}
	a, err := archive.Open(data)
	if err != nil {
		return nil, provider.Errorf(provider.BadSubtitle, "%w", err)
	}
	name := best(a.Names())
	if name == "" {
		return nil, provider.Errorf(provider.BadSubtitle, "no subtitle in the archive")
	}
	sub, err := a.Read(name)
	if err != nil {
		return nil, provider.Errorf(provider.BadSubtitle, "%s: %w", name, err)
	}
	return sub, nil
}

// isZip tells a zip archive, as Python's zipfile.is_zipfile.
func isZip(data []byte) bool {
	_, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	return err == nil
}

// best returns the subtitle of an archive to extract, as Bazarr's
// _get_subtitle_from_archive: subtitles of both Chinese scripts or
// bilingual are preferred; "" when there is no subtitle.
func best(names []string) string {
	pick, top := "", -1
	for _, name := range names {
		if strings.HasPrefix(path.Base(name), ".") ||
			!slices.ContainsFunc(subtitleExtensions, func(ext string) bool { return strings.HasSuffix(strings.ToLower(name), ext) }) {
			continue
		}
		score := 0
		if containsAny(name, "ass", "ssa", "srt") {
			score++
		}
		if containsAny(name, "简体", "chs", ".gb.") {
			score += 2
		}
		if containsAny(name, "繁体", "cht", ".big5.") {
			score += 2
		}
		if containsAny(name, "chs.eng", "chs&eng", "cht.eng", "cht&eng") {
			score += 2
		}
		if containsAny(name, "中英", "简英", "繁英", "双语", "简体&英文", "繁体&英文") {
			score += 4
		}
		if score > top {
			pick, top = name, score
		}
	}
	return pick
}

func containsAny(s string, subs ...string) bool {
	return slices.ContainsFunc(subs, func(sub string) bool { return strings.Contains(s, sub) })
}
