package bakabt

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/andybalholm/cascadia"
	"golang.org/x/net/html"

	"github.com/mavioai/mavio/plugins/indexers/internal/dom"
	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
	"github.com/mavioai/mavio/plugins/indexers/internal/parse"
)

var links = []string{"https://bakabt.me/"}

// Definition is BakaBT.
var Definition = indexer.Definition{
	ID:          "bakabt",
	Name:        "BakaBT",
	Description: "Anime Community",
	Language:    "en-US",
	Protocol:    indexer.ProtocolTorrent,
	Privacy:     indexer.Private,
	Links:       links,
	Fields: []indexer.Field{
		indexer.BaseURLField(links),
		indexer.UsernameField,
		indexer.PasswordField,
		{Name: "freeleech_only", Label: "Freeleech Only", Type: indexer.FieldCheckbox, HelpText: "Show freeleech torrents only"},
		{Name: "add_romaji_title", Label: "Add Romaji Title", Type: indexer.FieldCheckbox, HelpText: "Add releases for Romaji Title"},
		{Name: "append_season", Label: "Append Season", Type: indexer.FieldCheckbox, HelpText: "Append Season for Sonarr Compatibility"},
		{Name: "adult_content", Label: "Adult Content", Type: indexer.FieldCheckbox, HelpText: "Allow Adult Content (Must be enabled in BakaBT settings)"},
	},
	New: newSite,
}

// Settings are BakaBT's settings.
type Settings struct {
	indexer.UserPassSettings
	FreeleechOnly bool `json:"freeleech_only,omitempty"`
	// AddRomajiTitle adds a release for the romaji title of series
	// listed with an English one.
	AddRomajiTitle bool `json:"add_romaji_title,omitempty"`
	// AppendSeason adds "Season 1" to titles without a season, the site
	// listing only whole seasons.
	AppendSeason bool `json:"append_season,omitempty"`
	AdultContent bool `json:"adult_content,omitempty"`
}

// Site is BakaBT.
type Site struct {
	settings Settings
	baseURL  string
	caps     *indexer.Capabilities
}

func newSite(data []byte) (indexer.Site, error) {
	var settings Settings
	if err := indexer.DecodeSettings(data, &settings); err != nil {
		return nil, err
	}
	if err := settings.Validate(); err != nil {
		return nil, err
	}
	return &Site{settings: settings, baseURL: settings.URL(links, nil), caps: capabilities()}, nil
}

func capabilities() *indexer.Capabilities {
	c := indexer.NewCapabilities()
	c.Params[indexer.SearchTV] = []indexer.Param{indexer.ParamQ, indexer.ParamSeason, indexer.ParamEp}
	c.Params[indexer.SearchMovie] = []indexer.Param{indexer.ParamQ}
	c.Params[indexer.SearchMusic] = []indexer.Param{indexer.ParamQ}
	c.Params[indexer.SearchBook] = []indexer.Param{indexer.ParamQ}
	c.Categories.Add("1", indexer.TVAnime, "Anime Series")
	c.Categories.Add("2", indexer.TVAnime, "OVA")
	c.Categories.Add("3", indexer.AudioOther, "Soundtrack")
	c.Categories.Add("4", indexer.BooksComics, "Manga")
	c.Categories.Add("5", indexer.Movies, "Anime Movie")
	c.Categories.Add("6", indexer.TVOther, "Live Action")
	c.Categories.Add("7", indexer.BooksOther, "Artbook")
	c.Categories.Add("8", indexer.AudioVideo, "Music Video")
	c.Categories.Add("9", indexer.BooksEBook, "Light Novel")
	return c
}

// Info implements indexer.Site.
func (s *Site) Info() indexer.Info {
	return indexer.Info{
		Protocol:       indexer.ProtocolTorrent,
		Privacy:        indexer.Private,
		SupportsRSS:    true,
		SupportsSearch: true,
	}
}

// Capabilities implements indexer.Site.
func (s *Site) Capabilities(context.Context, *indexer.Indexer) (*indexer.Capabilities, error) {
	return s.caps, nil
}

// trailingEpisode is an episode ending a term, which the site, listing
// whole seasons, is not searched by.
var trailingEpisode = regexp.MustCompile(`^(.*)\s(?:[Ee]\d+|\d+)$`)

// SearchRequests implements indexer.Site, as Prowlarr's
// BakaBTRequestGenerator does: every search is by its term alone.
func (s *Site) SearchRequests(_ context.Context, _ *indexer.Indexer, q *indexer.Query) (*indexer.Chain, error) {
	term := q.SanitizedTerm()
	u := s.baseURL + "browse.php?only=0&incomplete=1&lossless=1&hd=1&multiaudio=1&bonus=1&reorder=1&q="
	if s.settings.AdultContent {
		u = s.baseURL + "browse.php?only=0&hentai=1&incomplete=1&lossless=1&hd=1&multiaudio=1&bonus=1&reorder=1&q="
	}
	if m := trailingEpisode.FindStringSubmatch(term); m != nil {
		term = m[1]
	}
	var c indexer.Chain
	c.AddRequests(indexer.NewRequest(u+indexer.URLEncode(term), ""))
	return &c, nil
}

// LoginNeeded implements indexer.LoginSite: guests are shown the login
// form.
func (s *Site) LoginNeeded(r *indexer.Response) bool {
	return strings.Contains(r.Content(), "loginForm")
}

var (
	loginKey   = cascadia.MustCompile(`input[name="loginKey"]`)
	loginError = cascadia.MustCompile(`#loginError, .error`)
)

// Login implements indexer.LoginSite: it posts the credentials, with the
// login page's key and cookies, keeping the answer's cookies for 30
// days.
func (s *Site) Login(ctx context.Context, ix *indexer.Indexer) error {
	ix.SetCookies(nil, time.Time{})
	loginURL := s.baseURL + "login.php"
	page, err := ix.Do(ctx, indexer.NewRequest(loginURL, ""))
	if err != nil {
		return fmt.Errorf("bakabt login: %w", err)
	}
	var form indexer.Params
	if doc, err := dom.Parse(page.Content()); err == nil {
		if n := loginKey.MatchFirst(doc); n != nil {
			form.Set("loginKey", dom.Attr(n, "value"))
		}
	}
	form.Set("username", s.settings.Username)
	form.Set("password", s.settings.Password)
	form.Set("returnto", "/index.php")
	r := indexer.NewRequest(loginURL, "")
	r.Method = http.MethodPost
	r.FollowRedirects = true
	r.Cookies = page.Cookies
	r.Body = []byte(form.Encode())
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := ix.Do(ctx, r)
	if err != nil {
		return fmt.Errorf("bakabt login: %w", err)
	}
	if s.LoginNeeded(resp) {
		msg := "Unknown error message, please report."
		if doc, err := dom.Parse(resp.Content()); err == nil {
			if n := loginError.MatchFirst(doc); n != nil {
				msg = strings.TrimSpace(dom.Text(n))
			}
		}
		return fmt.Errorf("%w: %s", indexer.ErrAuth, msg)
	}
	ix.SetCookies(resp.Cookies, time.Time{})
	slog.DebugContext(ctx, "BakaBT authentication succeeded")
	return nil
}

var downloadLink = cascadia.MustCompile(`.download_link`)

// DownloadRequest implements indexer.DownloadSite: releases link to
// their page, which links to the torrent.
func (s *Site) DownloadRequest(ctx context.Context, ix *indexer.Indexer, link string) (*indexer.Request, error) {
	r := indexer.NewRequest(link, "")
	r.Cookies = ix.Cookies()
	resp, err := ix.Do(ctx, r)
	if err != nil {
		return nil, fmt.Errorf("bakabt: %w", err)
	}
	var href string
	if doc, err := dom.Parse(resp.Content()); err == nil {
		href = dom.Attr(downloadLink.MatchFirst(doc), "href")
	}
	if strings.TrimSpace(href) == "" {
		return nil, errors.New("Unable to find download link.") //nolint:staticcheck // Prowlarr's message.
	}
	return indexer.NewRequest(s.baseURL+href, ""), nil
}

var (
	rows      = cascadia.MustCompile(`.torrents tr.torrent, .torrents tr.torrent_alt`)
	freeleech = cascadia.MustCompile(`span.freeleech`)
	titleLink = cascadia.MustCompile(`a.title, a.alt_title`)
	category  = cascadia.MustCompile(`td.category span`)
	tags      = cascadia.MustCompile(`span.tags`)
	peers     = cascadia.MustCompile(`.peers`)
	peerLinks = cascadia.MustCompile(`.peers a`)
	sizeSel   = cascadia.MustCompile(`.size`)
	added     = cascadia.MustCompile(`.added`)
)

// ParseResponse implements indexer.Site, as Prowlarr's BakaBTParser
// does: a release for each title of a row's series.
func (s *Site) ParseResponse(_ context.Context, ix *indexer.Indexer, r *indexer.Response) ([]*indexer.Release, error) {
	doc, err := dom.Parse(r.Content())
	if err != nil {
		return nil, err
	}
	now := time.Now
	if ix != nil && ix.Now != nil {
		now = ix.Now
	}
	// A row without a category is of the previous row's.
	categories := []int{indexer.TVAnime.ID}
	var releases []*indexer.Release
	for _, row := range rows.MatchAll(doc) {
		dvf := 1.0
		if freeleech.MatchFirst(row) != nil {
			dvf = 0
		}
		if s.settings.FreeleechOnly && dvf != 0 {
			continue
		}
		link := titleLink.MatchFirst(row)
		if link == nil {
			continue
		}
		title := strings.TrimSpace(dom.Text(link))
		split := releaseInfoIndex(title)
		series, info := title[:split], title[split:]
		categories = s.nextCategories(row, categories)
		names := splitNonEmpty(series, " | ")
		if len(names) > 1 && !s.settings.AddRomajiTitle {
			names = names[1:]
		}
		for _, name := range names {
			rel, err := s.release(row, link, strings.TrimSpace(name+info), now())
			if err != nil {
				return nil, fmt.Errorf("bakabt: %w", err)
			}
			rel.Categories = categories
			rel.DownloadVolumeFactor = indexer.Ptr(dvf)
			releases = append(releases, rel)
		}
	}
	return releases, nil
}

// releaseInfoIndex returns where the release information, in parentheses
// or brackets, starts in a title; its length when it has none.
func releaseInfoIndex(title string) int {
	i := len(title)
	if a := strings.IndexByte(title, '('); a >= 0 {
		i = a
	}
	if b := strings.IndexByte(title, '['); b >= 0 && b < i {
		i = b
	}
	return i
}

// splitNonEmpty splits s around sep, dropping empty parts.
func splitNonEmpty(s, sep string) []string {
	var out []string
	for p := range strings.SplitSeq(s, sep) {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// nextCategories returns the categories of a row: those of its category
// when it has one, TV/Anime when it is unknown, else current.
func (s *Site) nextCategories(row *html.Node, current []int) []int {
	n := category.MatchFirst(row)
	if n == nil {
		return current
	}
	name := dom.Attr(n, "title")
	if strings.TrimSpace(name) == "" {
		return current
	}
	if cats := indexer.IDs(s.caps.Categories.MapTrackerDesc(name)); len(cats) > 0 {
		return cats
	}
	return []int{indexer.TVAnime.ID}
}

func (s *Site) release(row, link *html.Node, title string, now time.Time) (*indexer.Release, error) {
	// The site lists whole seasons only.
	if s.settings.AppendSeason && !strings.Contains(title, "Season") {
		i := releaseInfoIndex(title)
		title = title[:i] + " Season 1 " + title[i:]
	}
	infoURL := s.baseURL + dom.Attr(link, "href")
	pl := peerLinks.MatchAll(row)
	if len(pl) < 2 {
		return nil, fmt.Errorf("a row has no seeders or leechers")
	}
	p := peers.MatchFirst(row)
	if p == nil || p.FirstChild == nil || p.FirstChild.Type != html.TextNode {
		return nil, fmt.Errorf("a row has no grabs")
	}
	grabsText := strings.TrimRight(strings.TrimRight(strings.TrimRight(p.FirstChild.Data, whitespace), "/"), whitespace)
	grabs, err := atoi(strings.ReplaceAll(grabsText, "k", "000"))
	if err != nil {
		return nil, err
	}
	seeders, err := atoi(dom.Text(pl[0]))
	if err != nil {
		return nil, err
	}
	leechers, err := atoi(dom.Text(pl[1]))
	if err != nil {
		return nil, err
	}
	sizeNode := sizeSel.MatchFirst(row)
	if sizeNode == nil {
		return nil, fmt.Errorf("a row has no size")
	}
	size, err := parse.Bytes(dom.Text(sizeNode))
	if err != nil {
		return nil, err
	}
	addedNode := added.MatchFirst(row)
	if addedNode == nil {
		return nil, fmt.Errorf("a row has no date")
	}
	date, err := publishDate(dom.Text(addedNode), now)
	if err != nil {
		return nil, err
	}
	rel := &indexer.Release{
		Title:              title,
		GUID:               infoURL,
		InfoURL:            infoURL,
		DownloadURL:        s.baseURL + dom.Attr(pl[0], "href"),
		Grabs:              indexer.Ptr(grabs),
		Seeders:            indexer.Ptr(seeders),
		Peers:              indexer.Ptr(seeders + leechers),
		MinimumRatio:       indexer.Ptr(1.0),
		MinimumSeedTime:    indexer.Ptr(int64(172800)), // 48 hours
		Size:               &size,
		PublishDate:        date,
		UploadVolumeFactor: indexer.Ptr(1.0),
	}
	if n := tags.MatchFirst(row); n != nil {
		rel.Description = dom.Text(n)
	}
	return rel, nil
}

// whitespace is the white space .NET trims.
const whitespace = " \t\n\v\f\r\u0085 "

// atoi parses a number as .NET's int.Parse does, allowing surrounding
// white space.
func atoi(s string) (int, error) {
	n, err := strconv.ParseInt(strings.Trim(s, whitespace), 10, 32)
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

// publishDate parses the date a release was added: "22 Jul 15", "today"
// or "yesterday".
func publishDate(text string, now time.Time) (time.Time, error) {
	text = strings.ReplaceAll(text, "'", "")
	if day, _, _ := strings.Cut(text, " "); len(day) == 1 {
		text = "0" + text
	}
	switch {
	case strings.EqualFold(text, "yesterday"):
		return now.AddDate(0, 0, -1), nil
	case strings.EqualFold(text, "today"):
		return now, nil
	}
	d, err := time.ParseInLocation("02 Jan 06", text, time.Local)
	if err != nil {
		return time.Time{}, fmt.Errorf("date %q: %w", text, err)
	}
	return d, nil
}
