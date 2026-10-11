package revolutiontt

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"strings"
	"time"

	"github.com/andybalholm/cascadia"
	"golang.org/x/net/html"

	"github.com/mavioai/mavio/plugins/indexers/internal/dom"
	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
	"github.com/mavioai/mavio/plugins/indexers/internal/parse"
)

var (
	links       = []string{"https://revott.me/"}
	legacyLinks = []string{"https://revolutiontt.me/"}
)

// Definition is RevolutionTT.
var Definition = indexer.Definition{
	ID:          "revolutiontt",
	Name:        "RevolutionTT",
	Description: "The Revolution has begun",
	Protocol:    indexer.ProtocolTorrent,
	Privacy:     indexer.Private,
	Links:       links,
	LegacyLinks: legacyLinks,
	Fields: []indexer.Field{
		indexer.BaseURLField(links),
		indexer.UsernameField,
		indexer.PasswordField,
	},
	New: newSite,
}

// Settings are RevolutionTT's settings.
type Settings struct {
	indexer.UserPassSettings
}

// Site is RevolutionTT.
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
	return &Site{settings: settings, baseURL: settings.URL(links, legacyLinks), caps: capabilities()}, nil
}

func capabilities() *indexer.Capabilities {
	c := indexer.NewCapabilities()
	c.Params[indexer.SearchTV] = []indexer.Param{indexer.ParamQ, indexer.ParamSeason, indexer.ParamEp}
	c.Params[indexer.SearchMovie] = []indexer.Param{indexer.ParamQ, indexer.ParamIMDbID}
	c.Params[indexer.SearchMusic] = []indexer.Param{indexer.ParamQ}
	c.Params[indexer.SearchBook] = []indexer.Param{indexer.ParamQ}
	c.Categories.Add("23", indexer.TVAnime, "")
	c.Categories.Add("22", indexer.PC0day, "")
	c.Categories.Add("1", indexer.PCISO, "")
	c.Categories.Add("36", indexer.Books, "")
	c.Categories.Add("36", indexer.BooksEBook, "")
	c.Categories.Add("4", indexer.PCGames, "")
	c.Categories.Add("21", indexer.PCGames, "")
	c.Categories.Add("16", indexer.ConsolePS3, "")
	c.Categories.Add("40", indexer.ConsoleWii, "")
	c.Categories.Add("39", indexer.ConsoleXBox360, "")
	c.Categories.Add("35", indexer.ConsoleNDS, "")
	c.Categories.Add("34", indexer.ConsolePSP, "")
	c.Categories.Add("2", indexer.PCMac, "")
	c.Categories.Add("10", indexer.MoviesBluRay, "")
	c.Categories.Add("20", indexer.MoviesDVD, "")
	c.Categories.Add("12", indexer.MoviesHD, "")
	c.Categories.Add("44", indexer.MoviesOther, "")
	c.Categories.Add("11", indexer.MoviesSD, "")
	c.Categories.Add("19", indexer.MoviesSD, "")
	c.Categories.Add("6", indexer.Audio, "")
	c.Categories.Add("8", indexer.AudioLossless, "")
	c.Categories.Add("46", indexer.AudioOther, "")
	c.Categories.Add("29", indexer.AudioVideo, "")
	c.Categories.Add("43", indexer.TVOther, "")
	c.Categories.Add("42", indexer.TVHD, "")
	c.Categories.Add("45", indexer.TVOther, "")
	c.Categories.Add("41", indexer.TVSD, "")
	c.Categories.Add("7", indexer.TVSD, "")
	c.Categories.Add("9", indexer.XXX, "")
	c.Categories.Add("49", indexer.XXX, "")
	c.Categories.Add("47", indexer.XXXDVD, "")
	c.Categories.Add("48", indexer.XXX, "")
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

// SearchRequests implements indexer.Site: one search of browse.php, of
// the titles unless by IMDb ID.
func (s *Site) SearchRequests(_ context.Context, _ *indexer.Indexer, q *indexer.Query) (*indexer.Chain, error) {
	var term, imdb string
	switch q.Type {
	case indexer.SearchMovie:
		term, imdb = q.SanitizedTerm(), q.FullIMDbID()
	case indexer.SearchTV:
		term, imdb = q.SanitizedTVString(), q.FullIMDbID()
	default:
		term = q.SanitizedTerm()
	}
	var p indexer.Params
	p.Set("incldead", "1")
	if strings.TrimSpace(imdb) != "" {
		p.Add("titleonly", "0")
		p.Add("search", imdb)
	} else {
		p.Add("titleonly", "1")
		p.Add("search", term)
	}
	for _, c := range s.caps.Categories.MapToTracker(q.Categories, false) {
		p.Add("c"+c, "1")
	}
	r := indexer.NewRequest(s.baseURL+"browse.php?"+p.Encode(), indexer.AcceptHTML)
	var c indexer.Chain
	c.AddRequests(r)
	return &c, nil
}

// LoginNeeded implements indexer.LoginSite: redirects and pages without
// the logout link are those of logged out users.
func (s *Site) LoginNeeded(r *indexer.Response) bool {
	return r.IsRedirect() || !strings.Contains(r.Content(), "/logout.php")
}

// Login implements indexer.LoginSite: it gets the login page's cookies,
// then posts the credentials to takelogin.php, keeping the cookies for 30
// days.
func (s *Site) Login(ctx context.Context, ix *indexer.Indexer) error {
	ix.SetCookies(nil, time.Time{})
	page := indexer.NewRequest(s.baseURL+"login.php", "")
	page.FollowRedirects = true
	loginPage, err := ix.Do(ctx, page)
	if err != nil {
		return fmt.Errorf("revolutiontt login: %w", err)
	}
	if loginPage.HasError() {
		return fmt.Errorf("revolutiontt login: %w", &indexer.HTTPError{Response: loginPage})
	}
	var form indexer.Params
	form.Set("username", s.settings.Username)
	form.Set("password", s.settings.Password)
	r := indexer.NewRequest(s.baseURL+"takelogin.php", "")
	r.FollowRedirects = true
	r.Method = http.MethodPost
	r.Body = []byte(form.Encode())
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Cookies = maps.Clone(loginPage.Cookies)
	resp, err := ix.Do(ctx, r)
	if err != nil {
		return fmt.Errorf("revolutiontt login: %w", err)
	}
	if resp.HasError() {
		return fmt.Errorf("revolutiontt login: %w", &indexer.HTTPError{Response: resp})
	}
	if !strings.Contains(resp.Content(), "/logout.php") {
		return fmt.Errorf("RevolutionTT authentication failed: %w", indexer.ErrAuth)
	}
	cookies := map[string]string{}
	maps.Copy(cookies, r.Cookies)
	maps.Copy(cookies, resp.Cookies)
	ix.SetCookies(cookies, time.Time{})
	return nil
}

var (
	rows         = cascadia.MustCompile(`#torrents-table > tbody > tr`)
	detailsLink  = cascadia.MustCompile(`.br_right > a`)
	bold         = cascadia.MustCompile(`b`)
	downloadLink = cascadia.MustCompile(`td:nth-child(4) > a`)
	dateCell     = cascadia.MustCompile(`td:nth-child(6) nobr`)
	sizeCell     = cascadia.MustCompile(`td:nth-child(7)`)
	filesLink    = cascadia.MustCompile(`td:nth-child(7) > a`)
	grabsCell    = cascadia.MustCompile(`td:nth-child(8)`)
	seedersCell  = cascadia.MustCompile(`td:nth-child(9)`)
	leechersCell = cascadia.MustCompile(`td:nth-child(10)`)
	categoryLink = cascadia.MustCompile(`.br_type > a`)
	imdbLink     = cascadia.MustCompile(`a[href*="www.imdb.com/"]`)
)

// ParseResponse implements indexer.Site, as Prowlarr's RevolutionTTParser
// does: the first row is the table's header.
func (s *Site) ParseResponse(_ context.Context, _ *indexer.Indexer, r *indexer.Response) ([]*indexer.Release, error) {
	doc, err := dom.Parse(r.Content())
	if err != nil {
		return nil, err
	}
	var releases []*indexer.Release
	for i, row := range rows.MatchAll(doc) {
		if i == 0 {
			continue
		}
		rel, err := s.release(row)
		if err != nil {
			return nil, fmt.Errorf("revolutiontt: %w", err)
		}
		if rel != nil {
			releases = append(releases, rel)
		}
	}
	return releases, nil
}

// release reads a row; nil for the support banner.
func (s *Site) release(row *html.Node) (*indexer.Release, error) {
	details := detailsLink.MatchFirst(row)
	link := downloadLink.MatchFirst(row)
	if link == nil {
		return nil, nil
	}
	if details == nil {
		return nil, errors.New("a release has no details link")
	}
	infoURL := s.baseURL + dom.Attr(details, "href")
	// Fulfilled requests have their tag added.
	title := strings.TrimPrefix(dom.Text(bold.MatchFirst(details)), "[REQ] ")
	cells := map[string]*html.Node{}
	for name, sel := range map[string]cascadia.Selector{
		"date": dateCell, "size": sizeCell, "files": filesLink, "grabs": grabsCell,
		"seeders": seedersCell, "leechers": leechersCell, "category": categoryLink,
	} {
		if cells[name] = sel.MatchFirst(row); cells[name] == nil {
			return nil, fmt.Errorf("a release has no %s", name)
		}
	}
	// Dates are such as "2015-04-25<br>23:38:12".
	date, err := time.ParseInLocation("2006-01-0215:04:05", strings.TrimSpace(dom.Text(cells["date"])), time.Local)
	if err != nil {
		return nil, err
	}
	sizeText, _, _ := strings.Cut(innerHTML(cells["size"]), "<")
	size, err := parse.Bytes(strings.TrimSpace(sizeText))
	if err != nil {
		return nil, err
	}
	files, ok := parse.LeadingInt(dom.Text(cells["files"]))
	if !ok {
		return nil, errors.New("a release has no files")
	}
	grabs, ok := parse.LeadingInt(dom.Text(cells["grabs"]))
	if !ok {
		return nil, errors.New("a release has no grabs")
	}
	seeders, err := parse.Int(dom.Text(cells["seeders"]))
	if err != nil {
		return nil, err
	}
	leechers, err := parse.Int(dom.Text(cells["leechers"]))
	if err != nil {
		return nil, err
	}
	category := strings.ReplaceAll(dom.Attr(cells["category"], "href"), "browse.php?cat=", "")
	rel := &indexer.Release{
		GUID:                 infoURL,
		InfoURL:              infoURL,
		DownloadURL:          s.baseURL + dom.Attr(link, "href"),
		Title:                title,
		PublishDate:          date,
		Size:                 &size,
		Seeders:              indexer.Ptr(int(seeders)),
		Peers:                indexer.Ptr(int(seeders + leechers)),
		Grabs:                indexer.Ptr(int(grabs)),
		Files:                indexer.Ptr(int(files)),
		Categories:           indexer.IDs(s.caps.Categories.MapTracker(category)),
		DownloadVolumeFactor: indexer.Ptr(1.0),
		UploadVolumeFactor:   indexer.Ptr(1.0),
		MinimumRatio:         indexer.Ptr(1.0),
		// 48 hours.
		MinimumSeedTime: indexer.Ptr(int64(172800)),
	}
	if a := imdbLink.MatchFirst(row); a != nil {
		href := dom.Attr(a, "href")
		if id, ok := parse.IMDbID(href[strings.LastIndex(href, "/")+1:]); ok {
			rel.IMDbID = id
		}
	}
	return rel, nil
}

// innerHTML returns the HTML of an element's children, as AngleSharp's
// InnerHtml does.
func innerHTML(n *html.Node) string {
	var b bytes.Buffer
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		_ = html.Render(&b, c)
	}
	return b.String()
}
