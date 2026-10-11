package speedcd

import (
	"context"
	"errors"
	"fmt"
	"maps"
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

var links = []string{
	"https://speed.cd/",
	"https://speed.click/",
	"https://speeders.me/",
}

// Definition is SpeedCD.
var Definition = indexer.Definition{
	ID:          "speedcd",
	Name:        "SpeedCD",
	Description: "Your home now!",
	Language:    "en-US",
	Protocol:    indexer.ProtocolTorrent,
	Privacy:     indexer.Private,
	Links:       links,
	Fields: []indexer.Field{
		indexer.BaseURLField(links),
		indexer.UsernameField,
		indexer.PasswordField,
		{Name: "freeleech_only", Label: "Freeleech Only", Type: indexer.FieldCheckbox, HelpText: "Search freeleech torrents only"},
		{Name: "exclude_archives", Label: "Exclude Archives", Type: indexer.FieldCheckbox, HelpText: "Exclude torrents containing RAR files from results"},
	},
	New: newSite,
}

// Settings are SpeedCD's settings.
type Settings struct {
	indexer.UserPassSettings
	FreeleechOnly   bool `json:"freeleech_only,omitempty"`
	ExcludeArchives bool `json:"exclude_archives,omitempty"`
}

// Site is SpeedCD.
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
	c.Params[indexer.SearchTV] = []indexer.Param{indexer.ParamQ, indexer.ParamSeason, indexer.ParamEp, indexer.ParamIMDbID}
	c.Params[indexer.SearchMovie] = []indexer.Param{indexer.ParamQ, indexer.ParamIMDbID}
	c.Params[indexer.SearchMusic] = []indexer.Param{indexer.ParamQ}
	c.Params[indexer.SearchBook] = []indexer.Param{indexer.ParamQ}
	c.Categories.Add("1", indexer.MoviesOther, "Movies/XviD")
	c.Categories.Add("42", indexer.Movies, "Movies/Packs")
	c.Categories.Add("32", indexer.Movies, "Movies/Kids")
	c.Categories.Add("43", indexer.MoviesHD, "Movies/HD")
	c.Categories.Add("47", indexer.Movies, "Movies/DiVERSiTY")
	c.Categories.Add("28", indexer.MoviesBluRay, "Movies/B-Ray")
	c.Categories.Add("48", indexer.Movies3D, "Movies/3D")
	c.Categories.Add("40", indexer.MoviesDVD, "Movies/DVD-R")
	c.Categories.Add("56", indexer.Movies, "Movies/Anime")
	c.Categories.Add("50", indexer.TVSport, "TV/Sports")
	c.Categories.Add("52", indexer.TVHD, "TV/B-Ray")
	c.Categories.Add("53", indexer.TVSD, "TV/DVD-R")
	c.Categories.Add("41", indexer.TV, "TV/Packs")
	c.Categories.Add("55", indexer.TV, "TV/Kids")
	c.Categories.Add("57", indexer.TV, "TV/DiVERSiTY")
	c.Categories.Add("49", indexer.TVHD, "TV/HD")
	c.Categories.Add("2", indexer.TVSD, "TV/Episodes")
	c.Categories.Add("30", indexer.TVAnime, "TV/Anime")
	c.Categories.Add("25", indexer.PCISO, "Games/PC ISO")
	c.Categories.Add("39", indexer.ConsoleWii, "Games/Wii")
	c.Categories.Add("45", indexer.ConsolePS3, "Games/PS3")
	c.Categories.Add("35", indexer.Console, "Games/Nintendo")
	c.Categories.Add("33", indexer.ConsoleXBox360, "Games/XboX360")
	c.Categories.Add("46", indexer.PCMobileOther, "Mobile")
	c.Categories.Add("24", indexer.PC0day, "Apps/0DAY")
	c.Categories.Add("51", indexer.PCMac, "Mac")
	c.Categories.Add("54", indexer.Books, "Educational")
	c.Categories.Add("27", indexer.Books, "Books-Mags")
	c.Categories.Add("26", indexer.Audio, "Music/Audio")
	c.Categories.Add("3", indexer.Audio, "Music/Flac")
	c.Categories.Add("44", indexer.Audio, "Music/Pack")
	c.Categories.Add("29", indexer.AudioVideo, "Music/Video")
	return c
}

// Info implements indexer.Site.
func (s *Site) Info() indexer.Info {
	return indexer.Info{
		Protocol:           indexer.ProtocolTorrent,
		Privacy:            indexer.Private,
		SupportsRSS:        true,
		SupportsSearch:     true,
		SupportsPagination: true,
	}
}

// Capabilities implements indexer.Site.
func (s *Site) Capabilities(context.Context, *indexer.Indexer) (*indexer.Capabilities, error) {
	return s.caps, nil
}

// SearchRequests implements indexer.Site: one search of the browse page,
// whose filters are path segments. Searches by IMDb ID search deep, in
// the descriptions too.
func (s *Site) SearchRequests(_ context.Context, _ *indexer.Indexer, q *indexer.Query) (*indexer.Chain, error) {
	imdb := q.FullIMDbID()
	var term string
	deep := false
	switch q.Type {
	case indexer.SearchMovie:
		term = q.SanitizedTerm()
		if strings.TrimSpace(imdb) != "" {
			term, deep = imdb, true
		}
		term = strings.TrimSpace(term)
	case indexer.SearchTV:
		term = q.SanitizedTVString()
		if strings.TrimSpace(imdb) != "" {
			term, deep = imdb, true
			if ep := q.EpisodeString(); strings.TrimSpace(ep) != "" {
				term += " " + ep
				if q.Season != nil && strings.TrimSpace(q.Episode) == "" {
					term += "*"
				}
			}
		}
		term = strings.TrimSpace(term)
	default:
		term = q.SanitizedTerm()
	}
	parts := s.caps.Categories.MapToTracker(q.Categories, false)
	if s.settings.FreeleechOnly {
		parts = append(parts, "freeleech")
	}
	if s.settings.ExcludeArchives {
		parts = append(parts, "norar")
	}
	if deep {
		parts = append(parts, "deep")
	}
	if strings.TrimSpace(term) != "" {
		parts = append(parts, "q", indexer.URLEncode(term))
	}
	if q.Limit > 0 && q.Offset > 0 {
		parts = append(parts, "p", strconv.Itoa(q.Offset/q.Limit+1))
	}
	r := indexer.NewRequest(strings.TrimRight(s.baseURL, "/")+"/browse/"+strings.Join(parts, "/"), indexer.AcceptHTML)
	r.FollowRedirects = true
	var c indexer.Chain
	c.AddRequests(r)
	return &c, nil
}

// LoginNeeded implements indexer.LoginSite: pages without links to the
// browse page are those of logged out users.
func (s *Site) LoginNeeded(r *indexer.Response) bool {
	return !strings.Contains(r.Content(), "/browse.php")
}

var (
	// loginToken is the token of the password form, which the API
	// answers in JSON.
	loginToken = regexp.MustCompile(`name=\\"a\\" value=\\"([^"]+)\\"`)
	loginError = cascadia.MustCompile(`h5`)
)

// Login implements indexer.LoginSite: it posts the user name to the
// checkpoint's API for the password form's token, then posts the
// password with it, keeping the cookies for 30 days.
func (s *Site) Login(ctx context.Context, ix *indexer.Indexer) error {
	base := strings.TrimRight(s.baseURL, "/")
	var form indexer.Params
	form.Set("username", s.settings.Username)
	r := indexer.NewRequest(base+"/checkpoint/API", "")
	r.FollowRedirects = true
	postForm(r, form)
	loginPage, err := ix.Do(ctx, r)
	if err != nil {
		return fmt.Errorf("speedcd login: %w", err)
	}
	if loginPage.HasError() {
		return fmt.Errorf("speedcd login: %w", &indexer.HTTPError{Response: loginPage})
	}
	m := loginToken.FindStringSubmatch(loginPage.Content())
	if m == nil {
		return fmt.Errorf("%w: Error parsing the login form", indexer.ErrAuth)
	}
	ix.SetCookies(nil, time.Time{})
	var form2 indexer.Params
	form2.Set("pwd", s.settings.Password)
	form2.Set("a", m[1])
	r2 := indexer.NewRequest(base+"/checkpoint/", "")
	r2.FollowRedirects = true
	postForm(r2, form2)
	r2.Cookies = maps.Clone(loginPage.Cookies)
	resp, err := ix.Do(ctx, r2)
	if err != nil {
		return fmt.Errorf("speedcd login: %w", err)
	}
	if resp.HasError() {
		return fmt.Errorf("speedcd login: %w", &indexer.HTTPError{Response: resp})
	}
	if s.LoginNeeded(resp) {
		msg := "Unknown error message, please report."
		content := resp.Content()
		if doc, err := dom.Parse(content); err == nil {
			if n := loginError.MatchFirst(doc); n != nil {
				msg = strings.TrimSpace(dom.Text(n))
			}
		}
		if strings.Contains(content, "Wrong Captcha!") {
			msg = "Captcha required due to a failed login attempt. Login via a browser to whitelist your IP and then reconfigure Mavio."
		}
		return fmt.Errorf("%w: %s", indexer.ErrAuth, msg)
	}
	cookies := map[string]string{}
	maps.Copy(cookies, r2.Cookies)
	maps.Copy(cookies, resp.Cookies)
	ix.SetCookies(cookies, time.Time{})
	return nil
}

// postForm makes a request a POST of a form, encoded as Prowlarr encodes
// forms.
func postForm(r *indexer.Request, form indexer.Params) {
	r.Method = http.MethodPost
	r.Body = []byte(form.Encode())
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
}

var (
	rows         = cascadia.MustCompile(`div.boxContent > table > tbody > tr`)
	titleLink    = cascadia.MustCompile(`td:nth-child(2) > div > a[href^="/t/"]`)
	downloadLink = cascadia.MustCompile(`td:nth-child(4) a[href^="/download/"]`)
	seedersCell  = cascadia.MustCompile(`td:nth-child(8)`)
	leechersCell = cascadia.MustCompile(`td:nth-child(9)`)
	sizeCell     = cascadia.MustCompile(`td:nth-child(6)`)
	grabsCell    = cascadia.MustCompile(`td:nth-child(7)`)
	dateSpan     = cascadia.MustCompile(`td:nth-child(2) span[class^="elapsedDate"]`)
	categoryLink = cascadia.MustCompile(`td:nth-child(1) a`)
	// tagSpans are those of the tags, such as "[Freeleech]".
	tagSpans = cascadia.MustCompile(`td:nth-child(2) span`)
)

// ParseResponse implements indexer.Site, as Prowlarr's SpeedCDParser
// does.
func (s *Site) ParseResponse(_ context.Context, _ *indexer.Indexer, r *indexer.Response) ([]*indexer.Release, error) {
	doc, err := dom.Parse(r.Content())
	if err != nil {
		return nil, err
	}
	var releases []*indexer.Release
	for _, row := range rows.MatchAll(doc) {
		rel, err := s.release(row)
		if err != nil {
			return nil, fmt.Errorf("speedcd: %w", err)
		}
		releases = append(releases, rel)
	}
	return releases, nil
}

func (s *Site) release(row *html.Node) (*indexer.Release, error) {
	link := titleLink.MatchFirst(row)
	if link == nil {
		return nil, errors.New("a release has no title")
	}
	infoURL := s.baseURL + strings.TrimLeft(dom.Attr(link, "href"), "/")
	var download string
	if a := downloadLink.MatchFirst(row); a != nil {
		download = strings.TrimLeft(dom.Attr(a, "href"), "/")
	}
	number := func(sel cascadia.Selector, name string) (int, error) {
		cell := sel.MatchFirst(row)
		if cell == nil {
			return 0, fmt.Errorf("a release has no %s", name)
		}
		n, err := parse.Int(dom.Text(cell))
		if err != nil {
			return 0, fmt.Errorf("%s: %w", name, err)
		}
		return int(n), nil
	}
	seeders, err := number(seedersCell, "seeders")
	if err != nil {
		return nil, err
	}
	leechers, err := number(leechersCell, "leechers")
	if err != nil {
		return nil, err
	}
	grabs, err := number(grabsCell, "grabs")
	if err != nil {
		return nil, err
	}
	sizeText := sizeCell.MatchFirst(row)
	if sizeText == nil {
		return nil, errors.New("a release has no size")
	}
	size, err := parse.Bytes(dom.Text(sizeText))
	if err != nil {
		return nil, err
	}
	span := dateSpan.MatchFirst(row)
	if span == nil {
		return nil, errors.New("a release has no date")
	}
	// Dates are such as "Monday, January 2, 2023 at 3:04PM", in UTC.
	date, err := time.ParseInLocation("Monday, January 2, 2006 3:04PM", strings.ReplaceAll(dom.Attr(span, "title"), " at", ""), time.UTC)
	if err != nil {
		return nil, err
	}
	var cat string
	if a := categoryLink.MatchFirst(row); a != nil {
		href := dom.Attr(a, "href")
		cat = href[strings.LastIndex(href, "/")+1:]
	}
	dvf := 1.0
	for _, span := range tagSpans.MatchAll(row) {
		if strings.Contains(dom.Text(span), "[Freeleech]") {
			dvf = 0
			break
		}
	}
	return &indexer.Release{
		GUID:                 infoURL,
		InfoURL:              infoURL,
		DownloadURL:          s.baseURL + download,
		Title:                cleanTitle(dom.Text(link)),
		Categories:           indexer.IDs(s.caps.Categories.MapTracker(cat)),
		PublishDate:          date,
		Size:                 &size,
		Grabs:                &grabs,
		Seeders:              &seeders,
		Peers:                indexer.Ptr(seeders + leechers),
		DownloadVolumeFactor: &dvf,
		UploadVolumeFactor:   indexer.Ptr(1.0),
		MinimumRatio:         indexer.Ptr(1.0),
		// 72 hours.
		MinimumSeedTime: indexer.Ptr(int64(259200)),
	}, nil
}

var requested = regexp.MustCompile(`(?i)\[REQ(UEST)?\]`)

// cleanTitle removes the tags of requests from a title.
func cleanTitle(title string) string {
	return strings.Trim(requested.ReplaceAllString(title, ""), " .")
}
