// Package scrape is what the providers of sites scraped without accounts
// share: subliminal's title sanitizing and naming fixes, difflib's
// similarity ratio, the text of pages as BeautifulSoup joins it, and
// the season, episode, year and title of release names as guessit reads
// them.
package scrape

import (
	"bytes"
	"net/url"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/text/unicode/norm"

	"github.com/mavioai/mavio/plugins/subtitles/internal/site"
)

// SubtitleExtensions are subliminal_patch's SUBTITLE_EXTENSIONS.
var SubtitleExtensions = []string{".srt", ".sub", ".smi", ".txt", ".ssa", ".ass", ".mpl", ".vtt"}

// IsSubtitle reports whether a file name has a subtitle's extension.
func IsSubtitle(name string) bool {
	name = strings.ToLower(name)
	return slices.ContainsFunc(SubtitleExtensions, func(ext string) bool { return strings.HasSuffix(name, ext) })
}

// videoExtensions are the extensions of video files release names may
// end with.
var videoExtensions = []string{".mkv", ".mp4", ".avi", ".m4v", ".mov", ".wmv", ".ts", ".mpg", ".webm"}

var spaces = regexp.MustCompile(`\s+`)

// Sanitize strips a title of special characters and lowercases it, as
// subliminal's sanitize: "-", ":", "(", ")" and "." become spaces and "'"
// is removed, but for the characters of ignore.
func Sanitize(s, ignore string) string {
	return sanitize(s, ignore, "'")
}

// SanitizePatch is subliminal_patch's sanitize, which also removes "´",
// "`" and "’".
func SanitizePatch(s, ignore string) string {
	return sanitize(s, ignore, "'´`’")
}

func sanitize(s, ignore, removed string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case strings.ContainsRune(ignore, r):
			b.WriteRune(r)
		case strings.ContainsRune("-:().", r):
			b.WriteByte(' ')
		case strings.ContainsRune(removed, r):
		default:
			b.WriteRune(r)
		}
	}
	return strings.ToLower(strings.TrimSpace(spaces.ReplaceAllString(b.String(), " ")))
}

// FixNaming replaces the names sites spell differently, as
// subliminal_patch's fix_inconsistent_naming without sanitizing: renames
// are pairs of a name and the site's, the first pair matching at a
// position winning.
func FixNaming(title string, renames ...string) string {
	if len(renames) < 2 {
		return title
	}
	var alts []string
	for i := 0; i+1 < len(renames); i += 2 {
		alts = append(alts, regexp.QuoteMeta(renames[i]))
	}
	re := regexp.MustCompile(strings.Join(alts, "|"))
	return re.ReplaceAllStringFunc(title, func(m string) string {
		for i := 0; i+1 < len(renames); i += 2 {
			if renames[i] == m {
				return renames[i+1]
			}
		}
		return m
	})
}

// ASCII drops the accents and the characters ASCII lacks, as
// unicodedata.normalize('NFKD', s).encode('ascii', 'ignore').
func ASCII(s string) string {
	var b strings.Builder
	for _, r := range norm.NFKD.String(s) {
		if r < 0x80 {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Words returns the text of an element as BeautifulSoup's
// get_text(" ", strip=True) with its spaces collapsed.
func Words(n site.Node) string {
	if n.Node == nil {
		return ""
	}
	var parts []string
	var walk func(*html.Node)
	walk = func(m *html.Node) {
		if m.Type == html.TextNode {
			parts = append(parts, m.Data)
		}
		for c := m.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n.Node)
	return strings.Join(strings.Fields(strings.Join(parts, " ")), " ")
}

// HTML parses an answer's page, as BeautifulSoup: an empty answer is an
// empty page.
func HTML(r *site.Response) (site.Node, error) {
	if len(bytes.TrimSpace(r.Body)) == 0 {
		return site.Parse([]byte("<html></html>"), "text/html; charset=utf-8")
	}
	return r.HTML()
}

// HasAttr reports whether an element has an attribute, as
// BeautifulSoup's has_attr.
func HasAttr(n site.Node, name string) bool {
	return n.Node != nil && slices.ContainsFunc(n.Node.Attr, func(a html.Attribute) bool { return a.Key == name })
}

// Contents returns the child nodes of an element, text included, as
// BeautifulSoup's contents: an element's text is its Text, a text's its
// data.
func Contents(n site.Node) []string {
	if n.Node == nil {
		return nil
	}
	var out []string
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		switch c.Type {
		case html.TextNode:
			out = append(out, c.Data)
		case html.ElementNode:
			out = append(out, site.Node{Node: c}.Text())
		}
	}
	return out
}

// NextText returns the text node following an element, as
// str(element.next_sibling); "" when none does.
func NextText(n site.Node) string {
	if n.Node == nil || n.NextSibling == nil {
		return ""
	}
	if n.NextSibling.Type == html.TextNode {
		return n.NextSibling.Data
	}
	return site.Node{Node: n.NextSibling}.Text()
}

// Escape escapes a value put in a URL's path or query as requests
// quotes one: spaces become %20.
func Escape(s string) string { return strings.ReplaceAll(url.QueryEscape(s), "+", "%20") }

var (
	// seasonEpisode is "S01E02", "S01E01-07" or "S01E01E02".
	seasonEpisode = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])s(\d{1,3})[\s._-]?e(\d{1,4})((?:[-e]{1,2}\d{1,3})*)(?:[^0-9a-z]|$)`)
	// crossed is "1x02".
	crossed = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])(\d{1,2})x(\d{1,3})(?:[^0-9a-z]|$)`)
	// seasonOnly is "S01", "Season 1" or "Saison 1", and ranges of them:
	// "Season 1-3".
	seasonOnly = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])(?:s|seasons?|saison|series)[\s._-]?(\d{1,2})(?:\s?-\s?(\d{1,2}))?(?:[^0-9a-z]|$)`)
	// episodeOnly is "E02", "Ep 2" or "Episode 2".
	episodeOnly = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])(?:e|ep|episode)[\s._-]?(\d{1,4})(?:[^0-9a-z]|$)`)
	moreEpisode = regexp.MustCompile(`(?i)[-e]{1,2}(\d{1,4})`)
)

// SeasonsEpisodes returns the seasons and episodes of a release name, as
// guessit reads them with enforce_list: "S01E01-07" is season 1,
// episodes 1 to 7, "Season 2" season 2 alone.
func SeasonsEpisodes(name string) (seasons, episodes []int) {
	add := func(list *[]int, n int) {
		if !slices.Contains(*list, n) {
			*list = append(*list, n)
		}
	}
	for _, m := range seasonEpisode.FindAllStringSubmatch(name, -1) {
		add(&seasons, atoi(m[1]))
		first := atoi(m[2])
		add(&episodes, first)
		for _, more := range moreEpisode.FindAllStringSubmatch(m[3], -1) {
			last := atoi(more[1])
			for e := first + 1; e <= last; e++ {
				add(&episodes, e)
			}
			add(&episodes, last)
			first = last
		}
	}
	for _, m := range crossed.FindAllStringSubmatch(name, -1) {
		add(&seasons, atoi(m[1]))
		add(&episodes, atoi(m[2]))
	}
	if len(seasons) == 0 {
		for _, m := range seasonOnly.FindAllStringSubmatch(name, -1) {
			first := atoi(m[1])
			add(&seasons, first)
			if m[2] != "" {
				for s := first + 1; s <= atoi(m[2]); s++ {
					add(&seasons, s)
				}
			}
		}
	}
	if len(episodes) == 0 {
		for _, m := range episodeOnly.FindAllStringSubmatch(name, -1) {
			add(&episodes, atoi(m[1]))
		}
	}
	return seasons, episodes
}

var year = regexp.MustCompile(`(?:^|[^0-9])((?:19|20)\d{2})(?:[^0-9]|$)`)

// Year returns the year of a release name, as guessit: a year from 1900
// to 2099 after the title; 0 for none.
func Year(name string) int {
	for _, m := range year.FindAllStringSubmatchIndex(name, -1) {
		if m[2] > 0 {
			return atoi(name[m[2]:m[3]])
		}
	}
	return 0
}

// titleEnd is where the title of a release name ends: at its year, its
// season or episode, or its first property.
var titleEnd = regexp.MustCompile(`(?i)(?:^|[\s._\-\[(])(?:(?:19|20)\d{2}|s\d{1,3}(?:[\s._-]?e\d{1,4})?|\d{1,2}x\d{1,3}|seasons?|episode|\d{3,4}[pi]|4k|blu-?ray|bdrip|brrip|dvdrip|dvd|web-?dl|web-?rip|web|hdtv|hdrip|xvid|divx|[xh][\s.]?26[45]|hevc|avc|remux|complete)(?:[\s._\-\])]|$)`)

// Title returns the title of a release name, as guessit: what precedes
// its year, season or episode and properties, with dots and underscores
// as spaces.
func Title(name string) string {
	if ext := strings.ToLower(path.Ext(name)); IsSubtitle(ext) || slices.Contains(videoExtensions, ext) {
		name = strings.TrimSuffix(name, path.Ext(name))
	}
	for _, loc := range titleEnd.FindAllStringIndex(name, -1) {
		if loc[0] > 0 {
			name = name[:loc[0]]
			break
		}
	}
	name = strings.NewReplacer(".", " ", "_", " ").Replace(name)
	return strings.Trim(strings.Join(strings.Fields(name), " "), " -")
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}
