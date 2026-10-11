package torrentbytes

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/andybalholm/cascadia"
	"golang.org/x/net/html"
	"golang.org/x/text/encoding/charmap"

	"github.com/mavioai/mavio/plugins/indexers/internal/dom"
	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
	"github.com/mavioai/mavio/plugins/indexers/internal/parse"
)

var links = []string{"https://www.torrentbytes.net/"}

// Definition is TorrentBytes.
var Definition = indexer.Definition{
	ID:          "torrentbytes",
	Name:        "Torrent Bytes",
	Description: "A decade of TorrentBytes",
	Language:    "en-US",
	Protocol:    indexer.ProtocolTorrent,
	Privacy:     indexer.Private,
	Links:       links,
	Fields: []indexer.Field{
		indexer.BaseURLField(links),
		indexer.UsernameField,
		indexer.PasswordField,
	},
	New: newSite,
}

// Settings are TorrentBytes' settings.
type Settings struct {
	indexer.UserPassSettings
}

// Site is TorrentBytes.
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
	c.Categories.Add("23", indexer.TVAnime, "Anime")
	c.Categories.Add("52", indexer.PCMac, "Apple/All")
	c.Categories.Add("22", indexer.PC, "Apps/misc")
	c.Categories.Add("1", indexer.PC, "Apps/PC")
	c.Categories.Add("28", indexer.TVForeign, "Foreign Titles")
	c.Categories.Add("50", indexer.Console, "Games/Consoles")
	c.Categories.Add("42", indexer.PCGames, "Games/Pack")
	c.Categories.Add("4", indexer.PCGames, "Games/PC")
	c.Categories.Add("51", indexer.PC, "Linux/All")
	c.Categories.Add("31", indexer.OtherMisc, "Misc")
	c.Categories.Add("20", indexer.MoviesDVD, "Movies/DVD-R")
	c.Categories.Add("12", indexer.MoviesBluRay, "Movies/Full Blu-ray")
	c.Categories.Add("5", indexer.MoviesHD, "Movies/HD")
	c.Categories.Add("40", indexer.Movies, "Movies/Pack")
	c.Categories.Add("19", indexer.MoviesSD, "Movies/SD")
	c.Categories.Add("49", indexer.MoviesUHD, "Movies/UHD")
	c.Categories.Add("25", indexer.Audio, "Music/DVDR")
	c.Categories.Add("48", indexer.AudioLossless, "Music/Flac")
	c.Categories.Add("6", indexer.AudioMP3, "Music/MP3")
	c.Categories.Add("43", indexer.Audio, "Music/Pack")
	c.Categories.Add("34", indexer.AudioVideo, "Music/Videos")
	c.Categories.Add("45", indexer.MoviesBluRay, "NonScene/BRrip")
	c.Categories.Add("46", indexer.MoviesHD, "NonScene/x264")
	c.Categories.Add("44", indexer.MoviesSD, "NonScene/Xvid")
	c.Categories.Add("37", indexer.TVHD, "TV/BRrip")
	c.Categories.Add("38", indexer.TVHD, "TV/HD")
	c.Categories.Add("41", indexer.TV, "TV/Pack")
	c.Categories.Add("33", indexer.TVSD, "TV/SD")
	c.Categories.Add("32", indexer.TVUHD, "TV/UHD")
	c.Categories.Add("39", indexer.XXXx264, "XXX/HD")
	c.Categories.Add("24", indexer.XXXImageSet, "XXX/IMGSET")
	c.Categories.Add("21", indexer.XXXPack, "XXX/Pack")
	c.Categories.Add("9", indexer.XXXXviD, "XXX/SD")
	c.Categories.Add("29", indexer.XXX, "XXX/Web")
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

// SearchRequests implements indexer.Site: one search of browse.php, of
// the titles, or of the descriptions by IMDb ID. Books are not searched.
func (s *Site) SearchRequests(_ context.Context, _ *indexer.Indexer, q *indexer.Query) (*indexer.Chain, error) {
	var c indexer.Chain
	var term, imdb string
	switch q.Type {
	case indexer.SearchBook:
		return &c, nil
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
		p.Add("search", imdb)
		p.Add("sc", "2")
	} else {
		p.Add("search", term)
		p.Add("sc", "1")
	}
	for _, cat := range s.caps.Categories.MapToTracker(q.Categories, false) {
		p.Add("c"+cat, "1")
	}
	r := indexer.NewRequest(strings.TrimRight(s.baseURL, "/")+"/browse.php?"+p.Encode(), indexer.AcceptHTML)
	r.FollowRedirects = true
	c.AddRequests(r)
	return &c, nil
}

// LoginNeeded implements indexer.LoginSite: pages without the link to
// the user's page are those of logged out users.
func (s *Site) LoginNeeded(r *indexer.Response) bool {
	return !strings.Contains(r.Content(), "my.php")
}

var loginError = cascadia.MustCompile(`td.embedded`)

// Login implements indexer.LoginSite: it posts the credentials to
// takelogin.php, keeping the cookies for 30 days.
func (s *Site) Login(ctx context.Context, ix *indexer.Indexer) error {
	ix.SetCookies(nil, time.Time{})
	loginURL := s.baseURL + "takelogin.php"
	r := indexer.NewRequest(loginURL, "")
	r.FollowRedirects = true
	r.Method = http.MethodPost
	r.Body = latin1Form([][2]string{
		{"username", s.settings.Username},
		{"password", s.settings.Password},
		{"returnto", "/"},
		{"login", "Log in!"},
	})
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Referer", loginURL)
	resp, err := ix.Do(ctx, r)
	if err != nil {
		return fmt.Errorf("torrentbytes login: %w", err)
	}
	if resp.HasError() {
		return fmt.Errorf("torrentbytes login: %w", &indexer.HTTPError{Response: resp})
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
	rows         = cascadia.MustCompile(`table > tbody:has(tr > td.colhead) > tr:not(:has(td.colhead))`)
	titleLink    = cascadia.MustCompile(`td:nth-of-type(2) a:nth-of-type(2)`)
	categoryLink = cascadia.MustCompile(`td:nth-of-type(1) a`)
	downloadLink = cascadia.MustCompile(`td:nth-of-type(2) a`)
	addedCell    = cascadia.MustCompile(`td:nth-of-type(5)`)
	sizeCell     = cascadia.MustCompile(`td:nth-of-type(7)`)
	seedersCell  = cascadia.MustCompile(`td:nth-of-type(9)`)
	leechersCell = cascadia.MustCompile(`td:nth-of-type(10)`)
	filesCell    = cascadia.MustCompile(`td:nth-child(3)`)
	grabsCell    = cascadia.MustCompile(`td:nth-child(8)`)
	greenFont    = cascadia.MustCompile(`font[color="green"]`)
)

// ParseResponse implements indexer.Site, as Prowlarr's TorrentBytesParser
// does. Searches finding nothing show a table with an empty row.
func (s *Site) ParseResponse(_ context.Context, _ *indexer.Indexer, r *indexer.Response) ([]*indexer.Release, error) {
	doc, err := dom.Parse(r.Content())
	if err != nil {
		return nil, err
	}
	var releases []*indexer.Release
	for _, row := range rows.MatchAll(doc) {
		rel, err := s.release(row)
		if err != nil {
			return nil, fmt.Errorf("torrentbytes: %w", err)
		}
		if rel == nil {
			break
		}
		releases = append(releases, rel)
	}
	return releases, nil
}

// release reads a row; nil for an empty one.
func (s *Site) release(row *html.Node) (*indexer.Release, error) {
	link := titleLink.MatchFirst(row)
	if link == nil {
		return nil, errors.New("a release has no details link")
	}
	details := s.baseURL + dom.Attr(link, "href")
	// Only truncated titles have a title attribute.
	title := dom.Attr(link, "title")
	if strings.TrimSpace(title) == "" {
		title = strings.TrimSpace(dom.Text(link.FirstChild))
	}
	if strings.TrimSpace(title) == "" {
		return nil, nil
	}
	rel := &indexer.Release{
		GUID:                 details,
		InfoURL:              details,
		Title:                title,
		Description:          title,
		DownloadVolumeFactor: indexer.Ptr(1.0),
		UploadVolumeFactor:   indexer.Ptr(1.0),
	}
	// Releases may have no category.
	if a := categoryLink.MatchFirst(row); a != nil {
		if href := dom.Attr(a, "href"); len(href) >= len("browse.php?cat=") {
			rel.Categories = indexer.IDs(s.caps.Categories.MapTracker(href[len("browse.php?cat="):]))
		}
	}
	rel.DownloadURL = s.baseURL + dom.Attr(downloadLink.MatchFirst(row), "href")
	cells := map[string]*html.Node{}
	for name, sel := range map[string]cascadia.Selector{
		"date": addedCell, "size": sizeCell, "seeders": seedersCell, "leechers": leechersCell,
		"files": filesCell, "grabs": grabsCell,
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
	rel.PublishDate = date
	size, err := parse.Bytes(strings.TrimSpace(dom.Text(cells["size"])))
	if err != nil {
		return nil, err
	}
	rel.Size = &size
	var counts []int
	for _, name := range []string{"seeders", "leechers", "files"} {
		n, err := parse.Int(strings.TrimSpace(dom.Text(cells[name])))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		counts = append(counts, int(n))
	}
	rel.Seeders = &counts[0]
	rel.Peers = indexer.Ptr(counts[0] + counts[1])
	rel.Files = &counts[2]
	if grabs := dom.Text(cells["grabs"]); grabs != "----" {
		n, err := parse.Int(grabs)
		if err != nil {
			return nil, fmt.Errorf("grabs: %w", err)
		}
		rel.Grabs = indexer.Ptr(int(n))
	}
	for _, f := range greenFont.MatchAll(row) {
		if t := dom.Text(f); strings.Contains(t, "F") && strings.Contains(t, "L") {
			rel.DownloadVolumeFactor = indexer.Ptr(0.0)
			break
		}
	}
	return rel, nil
}
