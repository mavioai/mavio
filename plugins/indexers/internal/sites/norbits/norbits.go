package norbits

import (
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

var links = []string{"https://norbits.net/"}

// Definition is NorBits.
var Definition = indexer.Definition{
	ID:          "norbits",
	Name:        "NorBits",
	Description: "NorBits is a Norwegian Private site for MOVIES / TV / GENERAL",
	Language:    "nb-NO",
	Protocol:    indexer.ProtocolTorrent,
	Privacy:     indexer.Private,
	Links:       links,
	Fields: []indexer.Field{
		indexer.BaseURLField(links),
		{Name: "username", Label: "Username", Type: indexer.FieldText, HelpText: "Site Username"},
		{Name: "password", Label: "Password", Type: indexer.FieldText, Secret: true, HelpText: "Site Password"},
		{
			Name: "two_factor_auth_code", Label: "2FA code", Type: indexer.FieldText,
			HelpText: "Only fill in the <b>2FA code</b> box if you have enabled <b>2FA</b> on the NorBits Web Site. Otherwise just leave it empty.",
		},
		{Name: "use_full_search", Label: "Use Full Search", Type: indexer.FieldCheckbox, HelpText: "Use Full Search from Site"},
		{Name: "freeleech_only", Label: "FreeLeech Only", Type: indexer.FieldCheckbox, HelpText: "Search FreeLeech torrents only"},
	},
	New: newSite,
}

// Settings are NorBits' settings.
type Settings struct {
	indexer.UserPassSettings
	TwoFactorAuthCode string `json:"two_factor_auth_code,omitempty"`
	UseFullSearch     bool   `json:"use_full_search,omitempty"`
	FreeleechOnly     bool   `json:"freeleech_only,omitempty"`
}

// Site is NorBits.
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
	c.Params[indexer.SearchMovie] = []indexer.Param{indexer.ParamQ, indexer.ParamIMDbID}
	c.Params[indexer.SearchMusic] = []indexer.Param{indexer.ParamQ}
	c.Params[indexer.SearchBook] = []indexer.Param{indexer.ParamQ}
	// The tracker's categories are the search's parameters.
	c.Categories.Add("main_cat[]=1", indexer.Movies, "Filmer")
	c.Categories.Add("main_cat[]=2", indexer.TV, "TV")
	c.Categories.Add("main_cat[]=3", indexer.PC, "Programmer")
	c.Categories.Add("main_cat[]=4", indexer.Console, "Spill")
	c.Categories.Add("main_cat[]=5", indexer.Audio, "Musikk")
	c.Categories.Add("main_cat[]=6", indexer.Books, "Tidsskrift")
	c.Categories.Add("main_cat[]=7", indexer.AudioAudiobook, "Lydbøker")
	c.Categories.Add("main_cat[]=8", indexer.AudioVideo, "Musikkvideoer")
	c.Categories.Add("main_cat[]=40", indexer.AudioOther, "Podcasts")
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

// SearchRequests implements indexer.Site: one search of browse.php, by
// IMDb ID or term.
func (s *Site) SearchRequests(_ context.Context, _ *indexer.Indexer, q *indexer.Query) (*indexer.Chain, error) {
	var term, imdb string
	switch q.Type {
	case indexer.SearchTV:
		term, imdb = q.SanitizedTVString(), q.FullIMDbID()
	case indexer.SearchMovie:
		term, imdb = q.SanitizedTerm(), q.FullIMDbID()
	default:
		term = q.SanitizedTerm()
	}
	var p indexer.Params
	p.Set("incldead", "1")
	p.Set("fullsearch", "0")
	if s.settings.UseFullSearch {
		p.Set("fullsearch", "1")
	}
	p.Set("scenerelease", "0")
	if s.settings.FreeleechOnly {
		p.Set("FL", "1")
	}
	search := "search="
	switch {
	case strings.TrimSpace(imdb) != "":
		search = "imdbsearch=" + imdb
	case strings.TrimSpace(term) != "":
		search = "search=" + indexer.URLEncode(term)
	}
	u := strings.TrimRight(s.baseURL, "/") + "/browse.php?" + search + "&" + p.Encode()
	if cats := s.caps.Categories.MapToTracker(q.Categories, false); len(cats) > 0 {
		u += "&" + strings.Join(cats, "&")
	}
	r := indexer.NewRequest(u, indexer.AcceptHTML)
	r.FollowRedirects = true
	var c indexer.Chain
	c.AddRequests(r)
	return &c, nil
}

// LoginNeeded implements indexer.LoginSite: when the page has no logout
// link.
func (s *Site) LoginNeeded(r *indexer.Response) bool {
	return !strings.Contains(r.Content(), "logout.php")
}

// Login implements indexer.LoginSite as the site's security checks want:
// it gets the home page and the login page, then posts the user name,
// password and 2FA code to takelogin.php with the home page's cookies,
// failing unless the site sets the uid cookie, and keeps the cookies, for
// 30 days.
func (s *Site) Login(ctx context.Context, ix *indexer.Indexer) error {
	get := func(u string, cookies map[string]string) (*indexer.Response, error) {
		r := indexer.NewRequest(u, "")
		r.FollowRedirects = true
		r.Cookies = cookies
		resp, err := ix.Do(ctx, r)
		if err != nil {
			return nil, fmt.Errorf("norbits login: %w", err)
		}
		if resp.HasError() {
			return nil, fmt.Errorf("norbits login: %w", &indexer.HTTPError{Response: resp})
		}
		return resp, nil
	}
	index, err := get(s.baseURL, nil)
	if err != nil {
		return err
	}
	loginURL := strings.TrimRight(s.baseURL, "/") + "/login.php"
	// The login page is not used, but the site's security checks want it
	// fetched.
	if _, err := get(loginURL, index.Cookies); err != nil {
		return err
	}
	r := indexer.NewRequest(strings.TrimRight(s.baseURL, "/")+"/takelogin.php", "")
	var form indexer.Params
	form.Set("username", s.settings.Username)
	form.Set("password", s.settings.Password)
	form.Set("code", s.settings.TwoFactorAuthCode)
	form.Set("logout", "no")
	form.Set("returnto", "/")
	r.Method = http.MethodPost
	r.Body = []byte(form.Encode())
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Referer", loginURL)
	r.Cookies = index.Cookies
	r.FollowRedirects = true
	resp, err := ix.Do(ctx, r)
	if err != nil {
		return fmt.Errorf("norbits login: %w", err)
	}
	// The response's cookies are, as Prowlarr's, those sent and those set.
	cookies := maps.Clone(index.Cookies)
	if cookies == nil {
		cookies = map[string]string{}
	}
	maps.Copy(cookies, resp.Cookies)
	if _, ok := cookies["uid"]; !ok {
		return fmt.Errorf("Login failed: %w", indexer.ErrAuth)
	}
	ix.SetCookies(cookies, time.Time{})
	return nil
}

var (
	rows        = cascadia.MustCompile(`#torrentTable > tbody > tr`)
	downloadLnk = cascadia.MustCompile(`td:nth-of-type(2) > a[href*="download.php?id="]`)
	detailsLnk  = cascadia.MustCompile(`td:nth-of-type(2) > a[href*="details.php?id="]`)
	categoryLnk = cascadia.MustCompile(`td:nth-of-type(1) a[href*="main_cat[]"]`)
	seedersCell = cascadia.MustCompile(`td:nth-of-type(9)`)
	leechCell   = cascadia.MustCompile(`td:nth-of-type(10)`)
	sizeCell    = cascadia.MustCompile(`td:nth-of-type(7)`)
	filesLink   = cascadia.MustCompile(`td:nth-of-type(3) > a`)
	grabsCell   = cascadia.MustCompile(`td:nth-of-type(8)`)
	dateCell    = cascadia.MustCompile(`td:nth-of-type(5)`)
	genresSpan  = cascadia.MustCompile(`span.genres`)
	imdbLink    = cascadia.MustCompile(`a[href*="imdb.com/title/tt"]`)
	free        = cascadia.MustCompile(`img[title="100% freeleech"]`)
	half        = cascadia.MustCompile(`img[title="Halfleech"]`)
	ninety      = cascadia.MustCompile(`img[title="90% Freeleech"]`)
)

// ParseResponse implements indexer.Site, as Prowlarr's NorBitsParser
// does.
func (s *Site) ParseResponse(_ context.Context, _ *indexer.Indexer, r *indexer.Response) ([]*indexer.Release, error) {
	doc, err := dom.Parse(r.Content())
	if err != nil {
		return nil, err
	}
	found := rows.MatchAll(doc)
	if len(found) > 0 {
		found = found[1:] // the header
	}
	var releases []*indexer.Release
	for _, row := range found {
		rel, err := s.release(row)
		if err != nil {
			return nil, err
		}
		releases = append(releases, rel)
	}
	return releases, nil
}

func (s *Site) release(row *html.Node) (*indexer.Release, error) {
	details := detailsLnk.MatchFirst(row)
	infoURL := s.baseURL + strings.TrimLeft(dom.Attr(details, "href"), "/")
	var category string
	if l := categoryLnk.MatchFirst(row); l != nil {
		href := dom.Attr(l, "href")
		for part := range strings.SplitSeq(href[strings.LastIndex(href, "?")+1:], "&") {
			if part = strings.TrimSpace(part); len(part) >= len("main_cat[]=") && strings.EqualFold(part[:len("main_cat[]=")], "main_cat[]=") {
				category = part
				break
			}
		}
	}
	seeders, err := cellInt(seedersCell.MatchFirst(row))
	if err != nil {
		return nil, err
	}
	leechers, err := cellInt(leechCell.MatchFirst(row))
	if err != nil {
		return nil, err
	}
	files, err := cellInt(filesLink.MatchFirst(row))
	if err != nil {
		return nil, err
	}
	grabsNode := grabsCell.MatchFirst(row)
	if grabsNode != nil {
		grabsNode = grabsNode.FirstChild
	}
	grabs, err := cellInt(grabsNode)
	if err != nil {
		return nil, err
	}
	sizeNode := sizeCell.MatchFirst(row)
	if sizeNode == nil {
		return nil, errors.New("norbits: a release has no size")
	}
	size, err := parse.Bytes(dom.Text(sizeNode))
	if err != nil {
		return nil, fmt.Errorf("norbits: %w", err)
	}
	dateNode := dateCell.MatchFirst(row)
	if dateNode == nil {
		return nil, errors.New("norbits: a release has no date")
	}
	// The site's dates are in the time zone of its server, taken as the
	// local one.
	date, err := time.ParseInLocation("2006-01-0215:04:05", strings.TrimSpace(dom.Text(dateNode)), time.Local)
	if err != nil {
		return nil, fmt.Errorf("norbits: %w", err)
	}
	rel := &indexer.Release{
		GUID:                 infoURL,
		InfoURL:              infoURL,
		DownloadURL:          s.baseURL + strings.TrimLeft(dom.Attr(downloadLnk.MatchFirst(row), "href"), "/"),
		Title:                strings.TrimSpace(dom.Attr(details, "title")),
		Protocol:             indexer.ProtocolTorrent,
		Categories:           indexer.IDs(s.caps.Categories.MapTracker(category)),
		Size:                 &size,
		Files:                &files,
		Grabs:                &grabs,
		Seeders:              &seeders,
		Peers:                indexer.Ptr(seeders + leechers),
		PublishDate:          date,
		DownloadVolumeFactor: indexer.Ptr(1.0),
		UploadVolumeFactor:   indexer.Ptr(1.0),
		MinimumRatio:         indexer.Ptr(1.0),
		MinimumSeedTime:      indexer.Ptr(int64(172800)), // 48 hours
	}
	if n := genresSpan.MatchFirst(row); n != nil {
		if genres := dom.Text(n); genres != "" {
			genres = strings.TrimSpace(genres)
			for _, r := range [][2]string{{"\u00a0", " "}, {"(", ""}, {")", ""}, {" | ", ","}} {
				genres = strings.ReplaceAll(genres, r[0], r[1])
			}
			rel.Description = genres
			for g := range strings.SplitSeq(genres, ",") {
				if g = strings.TrimSpace(g); g != "" {
					rel.Genres = append(rel.Genres, g)
				}
			}
		}
	}
	if l := imdbLink.MatchFirst(row); l != nil {
		href := strings.TrimRight(dom.Attr(l, "href"), "/")
		if id, ok := parse.IMDbID(href[strings.LastIndex(href, "/")+1:]); ok {
			rel.IMDbID = id
		}
	}
	switch {
	case free.MatchFirst(row) != nil:
		rel.DownloadVolumeFactor = indexer.Ptr(0.0)
	case half.MatchFirst(row) != nil:
		rel.DownloadVolumeFactor = indexer.Ptr(0.5)
	case ninety.MatchFirst(row) != nil:
		rel.DownloadVolumeFactor = indexer.Ptr(0.1)
	}
	return rel, nil
}

// cellInt reads a node's integer as Prowlarr's ParseUtil.CoerceInt does;
// missing nodes fail.
func cellInt(n *html.Node) (int, error) {
	if n == nil {
		return 0, errors.New("norbits: a release has a missing column")
	}
	v, err := parse.Int(strings.TrimSpace(dom.Text(n)))
	if err != nil {
		return 0, fmt.Errorf("norbits: %w", err)
	}
	return int(v), nil
}
