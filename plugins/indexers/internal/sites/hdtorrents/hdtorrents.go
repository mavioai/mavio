package hdtorrents

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/andybalholm/cascadia"
	"golang.org/x/net/html"

	"github.com/mavioai/mavio/plugins/indexers/internal/dom"
	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
	"github.com/mavioai/mavio/plugins/indexers/internal/parse"
)

var links = []string{
	"https://hdts.ru/",
	"https://hd-torrents.org/",
	"https://hd-torrents.net/",
	"https://hd-torrents.me/",
}

// Definition is HD-Torrents.
var Definition = indexer.Definition{
	ID:          "hdtorrents",
	Name:        "HD-Torrents",
	Description: "HD-Torrents is a private torrent website with HD torrents and strict rules on their content.",
	Protocol:    indexer.ProtocolTorrent,
	Privacy:     indexer.Private,
	Links:       links,
	Fields: []indexer.Field{
		indexer.BaseURLField(links),
		{Name: "username", Label: "Username", Type: indexer.FieldText, HelpText: "Site Username"},
		{Name: "password", Label: "Password", Type: indexer.FieldText, Secret: true, HelpText: "Site Password"},
	},
	New: newSite,
}

// Settings are HD-Torrents' settings.
type Settings = indexer.UserPassSettings

// Site is HD-Torrents.
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
	c.Flags = []indexer.Flag{indexer.FlagInternal}
	c.Categories.Add("70", indexer.MoviesBluRay, "Movie/UHD/Blu-Ray")
	c.Categories.Add("1", indexer.MoviesBluRay, "Movie/Blu-Ray")
	c.Categories.Add("71", indexer.MoviesUHD, "Movie/UHD/Remux")
	c.Categories.Add("2", indexer.MoviesHD, "Movie/Remux")
	c.Categories.Add("5", indexer.MoviesHD, "Movie/1080p/i")
	c.Categories.Add("3", indexer.MoviesHD, "Movie/720p")
	c.Categories.Add("64", indexer.MoviesUHD, "Movie/2160p")
	c.Categories.Add("63", indexer.Audio, "Movie/Audio Track")
	// TV shows.
	c.Categories.Add("72", indexer.TVUHD, "TV Show/UHD/Blu-ray")
	c.Categories.Add("59", indexer.TVHD, "TV Show/Blu-ray")
	c.Categories.Add("73", indexer.TVUHD, "TV Show/UHD/Remux")
	c.Categories.Add("60", indexer.TVHD, "TV Show/Remux")
	c.Categories.Add("30", indexer.TVHD, "TV Show/1080p/i")
	c.Categories.Add("38", indexer.TVHD, "TV Show/720p")
	c.Categories.Add("65", indexer.TVUHD, "TV Show/2160p")
	// Music.
	c.Categories.Add("44", indexer.Audio, "Music/Album")
	c.Categories.Add("61", indexer.AudioVideo, "Music/Blu-Ray")
	c.Categories.Add("62", indexer.AudioVideo, "Music/Remux")
	c.Categories.Add("57", indexer.AudioVideo, "Music/1080p/i")
	c.Categories.Add("45", indexer.AudioVideo, "Music/720p")
	c.Categories.Add("66", indexer.AudioVideo, "Music/2160p")
	// XXX.
	c.Categories.Add("58", indexer.XXX, "XXX/Blu-ray")
	c.Categories.Add("74", indexer.XXX, "XXX/UHD/Blu-ray")
	c.Categories.Add("48", indexer.XXX, "XXX/1080p/i")
	c.Categories.Add("47", indexer.XXX, "XXX/720p")
	c.Categories.Add("67", indexer.XXX, "XXX/2160p")
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

// SearchRequests implements indexer.Site: one search of torrents.php.
func (s *Site) SearchRequests(_ context.Context, _ *indexer.Indexer, q *indexer.Query) (*indexer.Chain, error) {
	var term, imdb string
	switch q.Type {
	case indexer.SearchMovie:
		// Prowlarr searches movies by their terms as they are.
		term, imdb = q.Term, q.FullIMDbID()
	case indexer.SearchTV:
		term, imdb = q.SanitizedTVString(), q.FullIMDbID()
	default:
		term = q.SanitizedTerm()
	}
	u := s.baseURL + "torrents.php?"
	for _, c := range s.caps.Categories.MapToTracker(q.Categories, false) {
		u += "category[]=" + c + "&"
	}
	var search []string
	for _, v := range []string{imdb, term} {
		if v != "" {
			search = append(search, v)
		}
	}
	var p indexer.Params
	p.Set("search", strings.Join(search, " "))
	p.Set("active", "0")
	p.Set("options", "0")
	// Parentheses are escaped against the site's "hacking" detection, and
	// points searched as spaces, which .NET's Uri sends as "%20".
	u += strings.NewReplacer("(", "%28", ")", "%29", ".", "%20").Replace(p.Encode())
	r := indexer.NewRequest(u, indexer.AcceptHTML)
	r.FollowRedirects = true
	var c indexer.Chain
	c.AddRequests(r)
	return &c, nil
}

// LoginNeeded implements indexer.LoginSite: when the site says the
// request is not authorized.
func (s *Site) LoginNeeded(r *indexer.Response) bool {
	return strings.Contains(r.Content(), "Error:You're not authorized")
}

var loginError = cascadia.MustCompile(`div > font[color="#FF0000"]`)

// Login implements indexer.LoginSite: it posts the user name and password
// to login.php, failing with the site's error unless it answers with the
// page asking for JavaScript, and keeps the cookies set, for 30 days.
func (s *Site) Login(ctx context.Context, ix *indexer.Indexer) error {
	loginURL := s.baseURL + "login.php"
	r := indexer.NewRequest(loginURL, "")
	var form indexer.Params
	form.Set("uid", s.settings.Username)
	form.Set("pwd", s.settings.Password)
	r.Method = http.MethodPost
	r.Body = []byte(form.Encode())
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Referer", loginURL)
	resp, err := ix.Do(ctx, r)
	if err != nil {
		return fmt.Errorf("hdtorrents login: %w", err)
	}
	if resp.HasError() {
		return fmt.Errorf("hdtorrents login: %w", &indexer.HTTPError{Response: resp})
	}
	content := resp.Content()
	if !strings.Contains(strings.ToLower(content), strings.ToLower("If your browser doesn't have javascript enabled")) {
		msg := "Couldn't login"
		doc, err := dom.Parse(content)
		if err != nil {
			return err
		}
		if n := loginError.MatchFirst(doc); n != nil {
			msg = strings.TrimSpace(dom.Text(n))
		}
		return fmt.Errorf("%s: %w", msg, indexer.ErrAuth)
	}
	ix.SetCookies(resp.Cookies, time.Time{})
	return nil
}

var (
	userInfo    = cascadia.MustCompile(`table.navus tr`)
	rows        = cascadia.MustCompile(`table.mainblockcontenttt tr:has(td.mainblockcontent)`)
	anchor      = cascadia.MustCompile(`a`)
	span        = cascadia.MustCompile(`span`)
	noRatio     = cascadia.MustCompile(`img[src$="no_ratio.png"]`)
	free        = cascadia.MustCompile(`img[src$="free.png"]`)
	half        = cascadia.MustCompile(`img[src$="50.png"]`)
	quarter     = cascadia.MustCompile(`img[src$="25.png"]`)
	threeFourth = cascadia.MustCompile(`img[src$="75.png"]`)
	internal    = cascadia.MustCompile(`img[src$="internal.png"]`)
	imdbLink    = cascadia.MustCompile(`a[href*="www.imdb.com/title/"]`)
	// poster is the poster's path in a title's onmouseover script.
	poster = regexp.MustCompile(`(?i)src=\\'./([^']+)\\'`)
)

// freeleechRanks are the user ranks downloading everything freely.
var freeleechRanks = []string{"VIP", "Uploader", "HD Internal", "Moderator", "Administrator", "Owner"}

// ParseResponse implements indexer.Site, as Prowlarr's HDTorrentsParser
// does.
func (s *Site) ParseResponse(_ context.Context, _ *indexer.Indexer, r *indexer.Response) ([]*indexer.Release, error) {
	doc, err := dom.Parse(r.Content())
	if err != nil {
		return nil, err
	}
	var hasFreeleech bool
	if info := userInfo.MatchFirst(doc); info != nil {
		cells := dom.Children(info)
		if len(cells) < 2 {
			return nil, errors.New("hdtorrents: the user's information has no rank")
		}
		rank := strings.TrimSpace(strings.ReplaceAll(dom.Text(cells[1]), "Rank:", ""))
		for _, fr := range freeleechRanks {
			hasFreeleech = hasFreeleech || strings.EqualFold(fr, rank)
		}
	}
	found := rows.MatchAll(doc)
	if len(found) > 0 {
		found = found[1:] // the header
	}
	var releases []*indexer.Release
	for _, row := range found {
		rel, err := s.release(row, hasFreeleech)
		if err != nil {
			return nil, err
		}
		releases = append(releases, rel)
	}
	return releases, nil
}

func (s *Site) release(row *html.Node, hasFreeleech bool) (*indexer.Release, error) {
	cells := dom.Children(row)
	if len(cells) < 8 {
		return nil, fmt.Errorf("hdtorrents: a row has %d columns, want = at least 8", len(cells))
	}
	mainLink := anchor.MatchFirst(cells[2])
	if mainLink == nil {
		return nil, errors.New("hdtorrents: a release has no details link")
	}
	details := absolute(s.baseURL + dom.Attr(mainLink, "href"))
	var posterURL string
	if m := poster.FindStringSubmatch(dom.Attr(mainLink, "onmouseover")); m != nil {
		posterURL = s.baseURL + strings.ReplaceAll(m[1], `\`, "/")
	}
	dl := firstElement(cells[4])
	if dl == nil {
		return nil, errors.New("hdtorrents: a release has no download link")
	}
	var description string
	if sp := span.MatchFirst(cells[2]); sp != nil {
		description = strings.TrimSpace(dom.Text(sp))
	}
	size, err := parse.Bytes(dom.Text(cells[7]))
	if err != nil {
		return nil, fmt.Errorf("hdtorrents: %w", err)
	}
	date, err := publishDate(cells[6])
	if err != nil {
		return nil, err
	}
	var cat string
	if first := firstElement(row); first != nil {
		cat = parse.QueryArg(dom.Attr(firstElement(first), "href"), "category")
	}

	// The uploader's column is missing at times, moving the seeders',
	// leechers' and grabs'.
	end := len(cells)
	switch {
	case dom.Text(cells[end-1]) == "Edit":
		end--
	case end >= 4 && dom.Text(cells[end-4]) == "Edit":
		// Moderators get links to delete, recommend and like too.
		end -= 4
	}
	rel := &indexer.Release{
		GUID:                 details,
		InfoURL:              details,
		DownloadURL:          absolute(s.baseURL + dom.Attr(dl, "href")),
		Title:                dom.Text(mainLink),
		Description:          description,
		Protocol:             indexer.ProtocolTorrent,
		PosterURL:            posterURL,
		PublishDate:          date,
		Categories:           indexer.IDs(s.caps.Categories.MapTracker(cat)),
		Size:                 &size,
		DownloadVolumeFactor: indexer.Ptr(1.0),
		UploadVolumeFactor:   indexer.Ptr(1.0),
		MinimumRatio:         indexer.Ptr(1.0),
		MinimumSeedTime:      indexer.Ptr(int64(172800)), // 48 hours
	}
	if end >= 3 {
		if seeders, ok := tryInt(dom.Text(cells[end-3])); ok {
			rel.Seeders = &seeders
			if leechers, ok := tryInt(dom.Text(cells[end-2])); ok {
				rel.Peers = indexer.Ptr(leechers + seeders)
			}
		}
	}
	if end >= 1 {
		if grabs, ok := tryInt(dom.Text(cells[end-1])); ok {
			rel.Grabs = &grabs
		}
	}
	switch {
	case noRatio.MatchFirst(row) != nil:
		rel.DownloadVolumeFactor = indexer.Ptr(0.0)
		rel.UploadVolumeFactor = indexer.Ptr(0.0)
	case hasFreeleech || free.MatchFirst(row) != nil:
		rel.DownloadVolumeFactor = indexer.Ptr(0.0)
	case half.MatchFirst(row) != nil:
		rel.DownloadVolumeFactor = indexer.Ptr(0.5)
	case quarter.MatchFirst(row) != nil:
		rel.DownloadVolumeFactor = indexer.Ptr(0.75)
	case threeFourth.MatchFirst(row) != nil:
		rel.DownloadVolumeFactor = indexer.Ptr(0.25)
	}
	if l := imdbLink.MatchFirst(row); l != nil {
		href := strings.TrimRight(dom.Attr(l, "href"), "/")
		if id, ok := parse.IMDbID(href[strings.LastIndex(href, "/")+1:]); ok {
			rel.IMDbID = id
		}
	}
	if internal.MatchFirst(row) != nil {
		rel.Flags = append(rel.Flags, indexer.FlagInternal)
	}
	return rel, nil
}

// publishDate reads the date a release was added, which the site writes
// as the names of the first four attributes of the cell's first element,
// such as <span 06 oct 2023 12:34:56>. It is in the time zone of the site's
// server, taken as the local one.
func publishDate(cell *html.Node) (time.Time, error) {
	el := firstElement(cell)
	if el == nil {
		return time.Time{}, errors.New("hdtorrents: a release has no date")
	}
	var names []string
	for _, a := range el.Attr {
		if len(names) == 4 {
			break
		}
		names = append(names, a.Key)
	}
	date, err := time.ParseInLocation("02 Jan 2006 15:04:05", strings.Join(names, " "), time.Local)
	if err != nil {
		return time.Time{}, fmt.Errorf("hdtorrents: %w", err)
	}
	return date, nil
}

// tryInt reads an integer as Prowlarr's ParseUtil.TryCoerceInt does.
func tryInt(s string) (int, bool) {
	n, err := parse.Int(s)
	if err != nil || n != int64(int32(n)) {
		return 0, false
	}
	return int(n), true
}

// firstElement returns the first child element of n; nil for none or a
// nil n.
func firstElement(n *html.Node) *html.Node {
	if n == nil {
		return nil
	}
	if c := dom.Children(n); len(c) > 0 {
		return c[0]
	}
	return nil
}

// absolute returns a URL normalized as .NET's Uri.AbsoluteUri does; as
// given when it does not parse.
func absolute(u string) string {
	p, err := url.Parse(u)
	if err != nil {
		return u
	}
	return p.String()
}
