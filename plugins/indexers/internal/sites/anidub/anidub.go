package anidub

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
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

var links = []string{"https://tr.anidub.com/"}

// Definition is Anidub.
var Definition = indexer.Definition{
	ID:          "anidub",
	Name:        "Anidub",
	Description: "Anidub is RUSSIAN anime voiceover group and eponymous anime tracker.",
	Language:    "ru-RU",
	Protocol:    indexer.ProtocolTorrent,
	Privacy:     indexer.SemiPrivate,
	Links:       links,
	Fields:      []indexer.Field{indexer.BaseURLField(links), indexer.UsernameField, indexer.PasswordField},
	New:         newSite,
}

// Site is Anidub.
type Site struct {
	settings indexer.UserPassSettings
	baseURL  string
	caps     *indexer.Capabilities
}

func newSite(data []byte) (indexer.Site, error) {
	var settings indexer.UserPassSettings
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
	c.Params[indexer.SearchBook] = []indexer.Param{indexer.ParamQ}
	c.Params[indexer.SearchMusic] = []indexer.Param{indexer.ParamQ}
	c.Categories.Add("2", indexer.TVAnime, "Аниме TV")
	c.Categories.Add("3", indexer.Movies, "Аниме Фильмы")
	c.Categories.Add("4", indexer.TVAnime, "Аниме OVA")
	c.Categories.Add("5", indexer.TVAnime, "Аниме OVA |- Аниме ONA")
	c.Categories.Add("6", indexer.TV, "Дорамы / Японские Сериалы и Фильмы")
	c.Categories.Add("7", indexer.TV, "Дорамы / Корейские Сериалы и Фильмы")
	c.Categories.Add("8", indexer.TV, "Дорамы / Китайские Сериалы и Фильмы")
	c.Categories.Add("9", indexer.TV, "Дорамы")
	c.Categories.Add("10", indexer.TVAnime, "Аниме TV / Аниме Ongoing")
	c.Categories.Add("11", indexer.TVAnime, "Аниме TV / Многосерийный сёнэн")
	c.Categories.Add("12", indexer.Other, "Аниме Ongoing Анонсы")
	c.Categories.Add("13", indexer.XXX, "18+")
	c.Categories.Add("14", indexer.TVAnime, "Аниме TV / Законченные сериалы")
	c.Categories.Add("15", indexer.BooksComics, "Манга")
	c.Categories.Add("16", indexer.Audio, "OST")
	c.Categories.Add("17", indexer.Audio, "Подкасты")
	return c
}

// Info implements indexer.Site.
func (s *Site) Info() indexer.Info {
	return indexer.Info{
		Protocol:       indexer.ProtocolTorrent,
		Privacy:        indexer.SemiPrivate,
		SupportsRSS:    true,
		SupportsSearch: true,
	}
}

// Capabilities implements indexer.Site.
func (s *Site) Capabilities(context.Context, *indexer.Indexer) (*indexer.Capabilities, error) {
	return s.caps, nil
}

// seasonEpisode is a season and episode ending a term, which breaks the
// site's search.
var seasonEpisode = regexp.MustCompile(`(?:[SsEe]?\d{1,4}){1,2}$`)

// SearchRequests implements indexer.Site, as Prowlarr's
// AnidubRequestGenerator does: the main page for recent releases, else a
// search posted to it.
func (s *Site) SearchRequests(_ context.Context, _ *indexer.Indexer, q *indexer.Query) (*indexer.Chain, error) {
	term := q.SanitizedTerm()
	if q.Type == indexer.SearchTV {
		term = q.SanitizedTVString()
	}
	var c indexer.Chain
	if strings.TrimSpace(term) == "" {
		r := indexer.NewRequest(s.baseURL, indexer.AcceptHTML)
		// Prowlarr's IndexerRequest of a URL follows redirects.
		r.FollowRedirects = true
		c.AddRequests(r)
		return &c, nil
	}
	r := indexer.NewRequest(strings.TrimRight(s.baseURL, "/")+"/index.php?do=search", indexer.AcceptHTML)
	// Prowlarr's IndexerRequest of a URL follows redirects.
	r.FollowRedirects = true
	var form indexer.Params
	form.Set("do", "search")
	form.Set("subaction", "search")
	form.Set("search_start", "1")
	form.Set("full_search", "1")
	form.Set("result_from", "1")
	form.Set("story", strings.TrimRight(seasonEpisode.ReplaceAllString(term, ""), " \t\r\n"))
	form.Set("titleonly", "3")
	form.Set("searchuser", "")
	form.Set("replyless", "0")
	form.Set("replylimit", "0")
	form.Set("searchdate", "0")
	form.Set("beforeafter", "after")
	form.Set("sortby", "")
	form.Set("resorder", "desc")
	form.Set("showposts", "1")
	form.Set("catlist[]", "0")
	r.Method = http.MethodPost
	r.Body = []byte(form.Encode())
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c.AddRequests(r)
	return &c, nil
}

// LoginNeeded implements indexer.LoginSite: pages without the logout
// link are those of a guest.
func (s *Site) LoginNeeded(r *indexer.Response) bool {
	return !strings.Contains(r.Content(), "index.php?action=logout")
}

var loginError = cascadia.MustCompile(`#content .berror .berror_c`)

// Login implements indexer.LoginSite: it posts the credentials to the
// main page with the cookies it sets, keeping those of the answer for 30
// days.
func (s *Site) Login(ctx context.Context, ix *indexer.Indexer) error {
	ix.SetCookies(nil, time.Time{})
	mainReq := indexer.NewRequest(s.baseURL, "")
	mainReq.FollowRedirects = true // new HttpRequest(BaseUrl) follows redirects.
	main, err := ix.Do(ctx, mainReq)
	if err != nil {
		return fmt.Errorf("anidub login: %w", err)
	}
	r := indexer.NewRequest(s.baseURL+"index.php", "")
	r.FollowRedirects = true
	r.Cookies = main.Cookies
	var form indexer.Params
	form.Set("login_name", s.settings.Username)
	form.Set("login_password", s.settings.Password)
	form.Set("login", "submit")
	r.Method = http.MethodPost
	r.Body = []byte(form.Encode())
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := ix.Do(ctx, r)
	if err != nil {
		return fmt.Errorf("anidub login: %w", err)
	}
	if !s.LoginNeeded(resp) {
		ix.SetCookies(resp.Cookies, time.Time{})
		slog.DebugContext(ctx, "Anidub authentication succeeded")
		return nil
	}
	msg := "Unknown error message, please report."
	if doc, err := dom.Parse(resp.Content()); err == nil {
		if n := loginError.MatchFirst(doc); n != nil {
			msg = strings.TrimSpace(dom.Text(n))
		}
	}
	return fmt.Errorf("%w: %s", indexer.ErrAuth, msg)
}

var releaseLinks = cascadia.MustCompile(`.searchitem > h3 > a[href], #dle-content > .story > .story_h > .lcol > h2 > a[href]`)

// ParseResponse implements indexer.Site, as Prowlarr's AnidubParser does:
// each release listed is read from its own page.
func (s *Site) ParseResponse(ctx context.Context, ix *indexer.Indexer, r *indexer.Response) ([]*indexer.Release, error) {
	doc, err := dom.Parse(r.Content())
	if err != nil {
		return nil, err
	}
	var releases []*indexer.Release
	for _, link := range releaseLinks.MatchAll(doc) {
		u := indexer.ResolveURL(r.URL, dom.Attr(link, "href"))
		req := indexer.NewRequest(u, indexer.AcceptHTML)
		req.Header.Set("Referer", s.baseURL)
		req.Cookies = ix.Cookies()
		resp, err := ix.Do(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("anidub: %w", err)
		}
		if resp.HasError() {
			if resp.StatusCode == http.StatusTooManyRequests {
				return nil, &indexer.RateLimitError{}
			}
			return nil, indexer.Errorf("HTTP Error - %d. %s", resp.StatusCode, u)
		}
		rels, err := s.parseRelease(ctx, ix, resp)
		if err != nil {
			return nil, err
		}
		releases = append(releases, rels...)
	}
	return releases, nil
}

var (
	tabs     = cascadia.MustCompile(`#tabs .torrent_c > div`)
	newsName = cascadia.MustCompile(`#news-title`)
	leechers = cascadia.MustCompile(`.list.down > .li_swing_m`)
	seeders  = cascadia.MustCompile(`.list.down > .li_distribute_m`)
	grabs    = cascadia.MustCompile(`.list.down > .li_download_m`)
	sizeSel  = cascadia.MustCompile(`.list.down > .red`)
	dateSel  = cascadia.MustCompile(`.story_inf > li:nth-child(2)`)
)

// categoryPaths map the paths of release pages to the site's categories,
// the first matching winning.
var categoryPaths = []struct{ path, id string }{
	{"/anime_tv/full", "14"},
	{"/anime_tv/anime_ongoing", "10"},
	{"/anime_tv/shonen", "11"},
	{"/anime_tv", "2"},
	{"/xxx", "13"},
	{"/manga", "15"},
	{"/ost", "16"},
	{"/podcast", "17"},
	{"/anime_movie", "3"},
	{"/anime_ova/anime_ona", "5"},
	{"/anime_ova", "4"},
	{"/dorama/japan_dorama", "6"},
	{"/dorama/korea_dorama", "7"},
	{"/dorama/china_dorama", "8"},
	{"/dorama", "9"},
	{"/anons_ongoing", "12"},
}

// parseRelease reads the releases of a release's page, one for each of
// its torrents.
func (s *Site) parseRelease(ctx context.Context, ix *indexer.Indexer, r *indexer.Response) ([]*indexer.Release, error) {
	doc, err := dom.Parse(r.Content())
	if err != nil {
		return nil, err
	}
	pageURL := r.Request.URL
	var categories []int
	if u, err := url.Parse(pageURL); err == nil {
		for _, c := range categoryPaths {
			if strings.HasPrefix(u.Path, c.path) {
				categories = indexer.IDs(s.caps.Categories.MapTracker(c.id))
				break
			}
		}
	}
	now := time.Now
	if ix.Now != nil {
		now = ix.Now
	}
	var releases []*indexer.Release
	for _, t := range tabs.MatchAll(doc) {
		title, err := releaseTitle(doc, t)
		if err != nil {
			return nil, err
		}
		size, err := parse.Bytes(dom.Text(sizeSel.MatchFirst(t)))
		if err != nil {
			return nil, fmt.Errorf("anidub: %w", err)
		}
		seeds := count(t, seeders)
		id := strings.ReplaceAll(strings.ReplaceAll(dom.Attr(t, "id"), "torrent_", ""), "_info", "")
		releases = append(releases, &indexer.Release{
			Title:                title,
			InfoURL:              pageURL,
			DownloadVolumeFactor: indexer.Ptr(0.0),
			UploadVolumeFactor:   indexer.Ptr(1.0),
			GUID:                 pageURL + dom.Attr(t, "id"),
			Seeders:              indexer.Ptr(seeds),
			Peers:                indexer.Ptr(seeds + count(t, leechers)),
			Grabs:                indexer.Ptr(count(t, grabs)),
			Categories:           categories,
			PublishDate:          pageDate(ctx, doc, now().UTC()),
			DownloadURL:          s.baseURL + "engine/download.php?id=" + id,
			Size:                 &size,
			// Prowlarr's Resolution reads the ID of the torrent's own
			// block, which names no quality; the quality block the
			// title reads is meant.
			Resolution: quality(t.Parent),
		})
	}
	return releases, nil
}

// releaseTitle returns the page's title with the quality of the torrent.
func releaseTitle(doc, tab *html.Node) (string, error) {
	n := newsName.MatchFirst(doc)
	if n == nil {
		return "", fmt.Errorf("anidub: a release's page has no title")
	}
	title := strings.TrimSpace(dom.Text(n))
	if q := quality(tab.Parent); strings.TrimSpace(q) != "" {
		return title + " [" + q + "]", nil
	}
	return title, nil
}

// quality returns the quality of a torrent's block, by its ID; "" for
// none.
func quality(n *html.Node) string {
	id := strings.TrimSpace(dom.Attr(n, "id"))
	if id == "" {
		return ""
	}
	switch strings.ToLower(id) {
	case "tv720":
		return "HDTV 720p"
	case "tv1080":
		return "HDTV 1080p"
	case "bd720":
		return "BDRip 720p"
	case "bd1080":
		return "BDRip 1080p"
	case "hwp":
		return "SDTV"
	}
	return strings.ToUpper(id)
}

// count reads a number of a torrent's block; 0 when it is not one.
func count(tab *html.Node, sel cascadia.Selector) int {
	n, err := strconv.ParseInt(strings.TrimSpace(dom.Text(sel.MatchFirst(tab))), 10, 32)
	if err != nil {
		return 0
	}
	return int(n)
}

// moscow is Russian Standard Time, which has no daylight saving time.
var moscow = time.FixedZone("MSK", 3*60*60)

// pageDate returns the date a release's page gives, "Сегодня" (today)
// or "Вчера" (yesterday) and a time, or a date and time, in Moscow's
// time; now when it gives none.
func pageDate(ctx context.Context, doc *html.Node, now time.Time) time.Time {
	var text string
	if li := dateSel.MatchFirst(doc); li != nil && li.LastChild != nil && li.LastChild.Type == html.TextNode {
		text = strings.TrimSpace(li.LastChild.Data)
	}
	local := now.In(moscow)
	text = strings.ReplaceAll(text, "Вчера", local.AddDate(0, 0, -1).Format("2-01-2006"))
	text = strings.ReplaceAll(text, "Сегодня", local.Format("2-01-2006"))
	d, err := time.ParseInLocation("2-01-2006, 15:04", text, moscow)
	if err != nil {
		slog.WarnContext(ctx, "Anidub date could not be parsed", "text", text)
		return now
	}
	return d.UTC()
}
