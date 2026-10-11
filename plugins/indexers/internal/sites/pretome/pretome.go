package pretome

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/andybalholm/cascadia"
	"golang.org/x/net/html"
	"golang.org/x/text/encoding/charmap"

	"github.com/mavioai/mavio/plugins/indexers/internal/dom"
	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
	"github.com/mavioai/mavio/plugins/indexers/internal/parse"
)

var links = []string{"https://pretome.info/"}

// Definition is PreToMe.
var Definition = indexer.Definition{
	ID:          "pretome",
	Name:        "PreToMe",
	Description: "PreToMe is a ratioless 0Day/General tracker.",
	Language:    "en-US",
	Protocol:    indexer.ProtocolTorrent,
	Privacy:     indexer.Private,
	Links:       links,
	Fields: []indexer.Field{
		indexer.BaseURLField(links),
		indexer.UsernameField,
		indexer.PasswordField,
		{Name: "pin", Label: "Pin", HelpText: "Site Pin", Type: indexer.FieldText, Secret: true},
	},
	New: newSite,
}

// Settings are PreToMe's settings.
type Settings struct {
	indexer.UserPassSettings
	// Pin is the PIN the site asks for besides the password.
	Pin string `json:"pin,omitempty"`
}

// Validate checks that the user name, password and PIN are set.
func (s Settings) Validate() error {
	var errs []error
	if err := s.UserPassSettings.Validate(); err != nil {
		errs = append(errs, err)
	}
	if strings.TrimSpace(s.Pin) == "" {
		errs = append(errs, errors.New("pin: must not be empty"))
	}
	return errors.Join(errs...)
}

// Site is PreToMe.
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
	c.Categories.Add("cat[]=22", indexer.PC, "Applications")
	c.Categories.Add("cat[]=22&tags=Windows", indexer.PC0day, "Applications/Windows")
	c.Categories.Add("cat[]=22&tags=MAC", indexer.PCMac, "Applications/MAC")
	c.Categories.Add("cat[]=22&tags=Linux", indexer.PC, "Applications/Linux")
	c.Categories.Add("cat[]=27", indexer.BooksEBook, "Ebooks")
	c.Categories.Add("cat[]=4", indexer.Console, "Games")
	c.Categories.Add("cat[]=4&tags=PC", indexer.PCGames, "Games/PC")
	c.Categories.Add("cat[]=4&tags=RIP", indexer.PCGames, "Games/RIP")
	c.Categories.Add("cat[]=4&tags=ISO", indexer.PCGames, "Games/ISO")
	c.Categories.Add("cat[]=4&tags=XBOX360", indexer.ConsoleXBox360, "Games/XBOX360")
	c.Categories.Add("cat[]=4&tags=PS3", indexer.ConsolePS3, "Games/PS3")
	c.Categories.Add("cat[]=4&tags=Wii", indexer.ConsoleWii, "Games/Wii")
	c.Categories.Add("cat[]=4&tags=PSP", indexer.ConsolePSP, "Games/PSP")
	c.Categories.Add("cat[]=4&tags=GAMES-NSW", indexer.ConsoleOther, "Games/NSW")
	c.Categories.Add("cat[]=4&tags=NDS", indexer.ConsoleNDS, "Games/NDS")
	c.Categories.Add("cat[]=4&tags=Xbox", indexer.ConsoleXBox, "Games/Xbox")
	c.Categories.Add("cat[]=4&tags=NSW", indexer.ConsoleOther, "Games/NSW")
	c.Categories.Add("cat[]=4&tags=PS2", indexer.ConsoleOther, "Games/PS2")
	c.Categories.Add("cat[]=31", indexer.Other, "Miscellaneous")
	c.Categories.Add("cat[]=31&tags=Ebook", indexer.BooksEBook, "Miscellaneous/Ebook")
	c.Categories.Add("cat[]=31&tags=RARFiX", indexer.Other, "Miscellaneous/RARFiX")
	c.Categories.Add("cat[]=19", indexer.Movies, "Movies")
	c.Categories.Add("cat[]=19&tags=x264", indexer.MoviesHD, "Movies/x264")
	c.Categories.Add("cat[]=19&tags=720p", indexer.MoviesHD, "Movies/720p")
	c.Categories.Add("cat[]=19&tags=XviD", indexer.MoviesSD, "Movies/XviD")
	c.Categories.Add("cat[]=19&tags=BluRay", indexer.MoviesHD, "Movies/BluRay")
	c.Categories.Add("cat[]=19&tags=DVDRiP", indexer.MoviesSD, "Movies/DVDRiP")
	c.Categories.Add("cat[]=19&tags=1080p", indexer.MoviesHD, "Movies/1080p")
	c.Categories.Add("cat[]=19&tags=DVD", indexer.MoviesSD, "Movies/DVD")
	c.Categories.Add("cat[]=19&tags=DVDR", indexer.MoviesSD, "Movies/DVDR")
	c.Categories.Add("cat[]=19&tags=WMV", indexer.Movies, "Movies/WMV")
	c.Categories.Add("cat[]=19&tags=CAM", indexer.Movies, "Movies/CAM")
	c.Categories.Add("cat[]=19&tags=2160p", indexer.MoviesUHD, "Movies/2160p")
	c.Categories.Add("cat[]=19&tags=DolbyVision", indexer.Movies, "Movies/DolbyVision")
	c.Categories.Add("cat[]=19&tags=x265", indexer.MoviesHD, "Movies/x265")
	c.Categories.Add("cat[]=6", indexer.Audio, "Music")
	c.Categories.Add("cat[]=6&tags=MP3", indexer.AudioMP3, "Music/MP3")
	c.Categories.Add("cat[]=6&tags=V2", indexer.AudioMP3, "Music/V2")
	c.Categories.Add("cat[]=6&tags=FLAC", indexer.AudioLossless, "Music/FLAC")
	c.Categories.Add("cat[]=6&tags=320kbps", indexer.AudioMP3, "Music/320kbps")
	c.Categories.Add("cat[]=52", indexer.TVSport, "Sport")
	c.Categories.Add("cat[]=52&tags=1080p", indexer.TVSport, "Sport/1080p")
	c.Categories.Add("cat[]=52&tags=720p", indexer.TVSport, "Sport/720p")
	c.Categories.Add("cat[]=52&tags=2160p", indexer.TVSport, "Sport/2160p")
	c.Categories.Add("cat[]=7", indexer.TV, "TV")
	c.Categories.Add("cat[]=7&tags=x264", indexer.TVHD, "TV/x264")
	c.Categories.Add("cat[]=7&tags=720p", indexer.TVHD, "TV/720p")
	c.Categories.Add("cat[]=7&tags=HDTV", indexer.TVHD, "TV/HDTV")
	c.Categories.Add("cat[]=7&tags=XviD", indexer.TVSD, "TV/XviD")
	c.Categories.Add("cat[]=7&tags=BluRay", indexer.TVHD, "TV/BluRay")
	c.Categories.Add("cat[]=7&tags=DVDRiP", indexer.TVSD, "TV/DVDRiP")
	c.Categories.Add("cat[]=7&tags=DVD", indexer.TVSD, "TV/DVD")
	c.Categories.Add("cat[]=7&tags=Documentary", indexer.TVDocumentary, "TV/Documentary")
	c.Categories.Add("cat[]=7&tags=PDTV", indexer.TVSD, "TV/PDTV")
	c.Categories.Add("cat[]=7&tags=HD-DVD", indexer.TVSD, "TV/HD-DVD")
	c.Categories.Add("cat[]=7&tags=1080p", indexer.TVHD, "TV/1080p")
	c.Categories.Add("cat[]=7&tags=2160p", indexer.TVUHD, "TV/2160p")
	c.Categories.Add("cat[]=7&tags=DolbyVision", indexer.TVUHD, "TV/DolbyVision")
	c.Categories.Add("cat[]=7&tags=x265", indexer.TVHD, "TV/x265")
	c.Categories.Add("cat[]=51", indexer.XXX, "XXX")
	c.Categories.Add("cat[]=51&tags=XviD", indexer.XXXXviD, "XXX/XviD")
	c.Categories.Add("cat[]=51&tags=DVDRiP", indexer.XXXDVD, "XXX/DVDRiP")
	c.Categories.Add("cat[]=51&tags=1080p", indexer.XXXx264, "XXX/1080p")
	c.Categories.Add("cat[]=51&tags=2160p", indexer.XXXUHD, "XXX/2160p")
	c.Categories.Add("cat[]=51&tags=720p", indexer.XXXx264, "XXX/720p")
	c.Categories.Add("cat[]=51&tags=VR180", indexer.XXXOther, "XXX/VR180")
	c.Categories.Add("cat[]=51&tags=x264", indexer.XXXx264, "XXX/x264")
	c.Categories.Add("cat[]=51&tags=x265", indexer.XXXx264, "XXX/x265")
	c.Categories.Add("cat[]=51&tags=XXX-VR180", indexer.XXXOther, "XXX/VR180")
	return c
}

// Info implements indexer.Site. The site's pages are in Latin-1.
func (s *Site) Info() indexer.Info {
	return indexer.Info{
		Protocol:       indexer.ProtocolTorrent,
		Privacy:        indexer.Private,
		SupportsRSS:    true,
		SupportsSearch: true,
		Encoding:       charmap.ISO8859_1,
	}
}

// Capabilities implements indexer.Site.
func (s *Site) Capabilities(context.Context, *indexer.Indexer) (*indexer.Capabilities, error) {
	return s.caps, nil
}

// SearchRequests implements indexer.Site: one search of browse.php.
// Searches by IMDb ID search the descriptions too.
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
	// The keys repeat, so they are not Params.
	params := [][2]string{{"st", "1"}}
	if strings.TrimSpace(imdb) != "" {
		params = append(params, [2]string{"search", imdb}, [2]string{"sd", "1"})
	} else {
		params = append(params, [2]string{"search", term})
	}
	// Categories are such as "cat[]=7&tags=x264".
	var cats, tags []string
	for _, c := range s.caps.Categories.MapToTracker(q.Categories, false) {
		group, tag, hasTag := strings.Cut(c, "&")
		if _, v, ok := strings.Cut(group, "="); ok && !slices.Contains(cats, v) {
			cats = append(cats, v)
		}
		if hasTag {
			if _, v, ok := strings.Cut(tag, "="); ok && !slices.Contains(tags, v) {
				tags = append(tags, v)
			}
		}
	}
	for _, c := range cats {
		params = append(params, [2]string{"cat[]", c})
	}
	// Too many tags are too much for the site's servers. Without tags all
	// must match; any would give random results.
	if len(tags) < 7 {
		tf := "all"
		if len(tags) > 0 {
			tf = "any"
		}
		params = append(params, [2]string{"tags", strings.Join(tags, ",")}, [2]string{"tf", tf})
	}
	parts := make([]string, len(params))
	for i, p := range params {
		parts[i] = p[0] + "=" + indexer.URLEncode(p[1])
	}
	r := indexer.NewRequest(s.baseURL+"browse.php?"+strings.Join(parts, "&"), indexer.AcceptHTML)
	var c indexer.Chain
	c.AddRequests(r)
	return &c, nil
}

// LoginNeeded implements indexer.LoginSite: redirects and pages without
// the logout link are those of logged out users.
func (s *Site) LoginNeeded(r *indexer.Response) bool {
	return r.IsRedirect() || !strings.Contains(r.Content(), "logout.php")
}

var loginError = cascadia.MustCompile(`table.body_table font[color~="red"]`)

// Login implements indexer.LoginSite: it gets the login page's cookies,
// then posts the credentials to takelogin.php, keeping the cookies for 30
// days.
func (s *Site) Login(ctx context.Context, ix *indexer.Indexer) error {
	ix.SetCookies(nil, time.Time{})
	page := indexer.NewRequest(s.baseURL+"login.php", "")
	page.FollowRedirects = true
	loginPage, err := ix.Do(ctx, page)
	if err != nil {
		return fmt.Errorf("pretome login: %w", err)
	}
	if loginPage.HasError() {
		return fmt.Errorf("pretome login: %w", &indexer.HTTPError{Response: loginPage})
	}
	r := indexer.NewRequest(s.baseURL+"takelogin.php", "")
	r.FollowRedirects = true
	r.Method = http.MethodPost
	r.Cookies = maps.Clone(loginPage.Cookies)
	r.Body = latin1Form([][2]string{
		{"username", s.settings.Username},
		{"password", s.settings.Password},
		{"login_pin", s.settings.Pin},
		{"returnto", "%2F"},
		{"login", "Login"},
	})
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := ix.Do(ctx, r)
	if err != nil {
		return fmt.Errorf("pretome login: %w", err)
	}
	if resp.HasError() {
		return fmt.Errorf("pretome login: %w", &indexer.HTTPError{Response: resp})
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
	cookies := maps.Clone(r.Cookies)
	if cookies == nil {
		cookies = map[string]string{}
	}
	maps.Copy(cookies, resp.Cookies)
	ix.SetCookies(cookies, time.Time{})
	return nil
}

// latin1Form encodes a form as Prowlarr does with a Latin-1 encoding:
// keys as they are, values encoded in Latin-1, characters it lacks as
// '?', then URL encoded.
func latin1Form(fields [][2]string) []byte {
	parts := make([]string, len(fields))
	for i, f := range fields {
		b := make([]byte, 0, len(f[1]))
		for _, r := range f[1] {
			if r < 0x100 {
				b = append(b, byte(r))
			} else {
				b = append(b, '?')
			}
		}
		parts[i] = f[0] + "=" + indexer.URLEncode(string(b))
	}
	return []byte(strings.Join(parts, "&"))
}

var (
	rows        = cascadia.MustCompile(`table > tbody > tr.browse`)
	detailsLink = cascadia.MustCompile(`a[href^="details.php?id="]`)
	downloadLnk = cascadia.MustCompile(`a[href^="download.php"]`)
	categoryLnk = cascadia.MustCompile(`td:nth-of-type(1) a[href^="browse.php"]`)
	brTag       = regexp.MustCompile(`\<br[\s]{0,1}[\/]{0,1}\>`)
)

// column returns the selector of a row's nth cell.
func column(n int) cascadia.Selector {
	return cascadia.MustCompile(fmt.Sprintf("td:nth-of-type(%d)", n))
}

var (
	addedCell    = column(6)
	filesCell    = column(4)
	sizeCell     = column(8)
	grabsCell    = column(9)
	seedersCell  = column(10)
	leechersCell = column(11)
)

// ParseResponse implements indexer.Site, as Prowlarr's PreToMeParser
// does.
func (s *Site) ParseResponse(_ context.Context, _ *indexer.Indexer, r *indexer.Response) ([]*indexer.Release, error) {
	doc, err := dom.Parse(r.Content())
	if err != nil {
		return nil, err
	}
	now := time.Now()
	var releases []*indexer.Release
	for _, row := range rows.MatchAll(doc) {
		rel, err := s.release(now, row)
		if err != nil {
			return nil, fmt.Errorf("pretome: %w", err)
		}
		releases = append(releases, rel)
	}
	return releases, nil
}

func (s *Site) release(now time.Time, row *html.Node) (*indexer.Release, error) {
	details := detailsLink.MatchFirst(row)
	if details == nil {
		return nil, errors.New("a release has no details link")
	}
	infoURL := s.baseURL + dom.Attr(details, "href")
	added := addedCell.MatchFirst(row)
	if added == nil {
		return nil, errors.New("a release has no date")
	}
	date, err := parse.TimeAgo(now, strings.TrimSpace(brTag.ReplaceAllString(innerHTML(added), " ")))
	if err != nil {
		return nil, err
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
	files, err := number(filesCell, "files")
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
	var cat string
	if a := categoryLnk.MatchFirst(row); a != nil {
		href := dom.Attr(a, "href")
		cat = href[strings.LastIndex(href, "?")+1:]
	}
	var download string
	if a := downloadLnk.MatchFirst(row); a != nil {
		download = dom.Attr(a, "href")
	}
	return &indexer.Release{
		GUID:        infoURL,
		InfoURL:     infoURL,
		DownloadURL: s.baseURL + download,
		Title:       dom.Attr(details, "title"),
		Categories:  indexer.IDs(s.caps.Categories.MapTracker(cat)),
		PublishDate: date,
		Size:        &size,
		Files:       &files,
		Grabs:       &grabs,
		Seeders:     &seeders,
		Peers:       indexer.Ptr(seeders + leechers),
		// The site is ratioless.
		DownloadVolumeFactor: indexer.Ptr(0.0),
		UploadVolumeFactor:   indexer.Ptr(1.0),
		MinimumRatio:         indexer.Ptr(0.75),
		// 60 hours.
		MinimumSeedTime: indexer.Ptr(int64(216000)),
	}, nil
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
