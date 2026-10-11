package pixelhd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/andybalholm/cascadia"
	"golang.org/x/net/html"

	"github.com/mavioai/mavio/plugins/indexers/internal/dom"
	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
	"github.com/mavioai/mavio/plugins/indexers/internal/parse"
)

var links = []string{"https://pixelhd.me/"}

// Definition is PiXELHD.
var Definition = indexer.Definition{
	ID:          "pixelhd",
	Name:        "PiXELHD",
	Description: "PixelHD (PxHD) is a ratioless Private Torrent Tracker for HD .MP4 MOVIES / TV",
	Language:    "en-US",
	Protocol:    indexer.ProtocolTorrent,
	Privacy:     indexer.Private,
	Links:       links,
	Fields: []indexer.Field{
		indexer.BaseURLField(links),
		indexer.CookieField,
		{
			Name: "user_agent", Label: "Cookie User-Agent", Type: indexer.FieldText,
			HelpText: "User-Agent associated with cookie used from Browser",
		},
	},
	New: newSite,
}

// Settings are PiXELHD's settings.
type Settings struct {
	indexer.BaseSettings
	Cookie    string `json:"cookie,omitempty"`
	UserAgent string `json:"user_agent,omitempty"`
}

// Validate checks that the cookie and its User-Agent are set.
func (s Settings) Validate() error {
	var errs []error
	if strings.TrimSpace(s.Cookie) == "" {
		errs = append(errs, errors.New("cookie: must not be empty"))
	}
	if strings.TrimSpace(s.UserAgent) == "" {
		errs = append(errs, errors.New("user_agent: must not be empty"))
	}
	return errors.Join(errs...)
}

// Site is PiXELHD.
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
	c.Params[indexer.SearchMovie] = []indexer.Param{indexer.ParamQ, indexer.ParamIMDbID}
	c.Categories.Add("1", indexer.MoviesHD, "")
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

// SearchRequests implements indexer.Site: one search of torrents.php for
// movie and basic searches, none for the others.
func (s *Site) SearchRequests(_ context.Context, _ *indexer.Indexer, q *indexer.Query) (*indexer.Chain, error) {
	var c indexer.Chain
	var imdb string
	switch q.Type {
	case indexer.SearchMovie:
		imdb = q.FullIMDbID()
	case indexer.SearchBasic:
	default:
		return &c, nil
	}
	term := q.SanitizedTerm()
	var p indexer.Params
	p.Set("order_by", "time")
	p.Set("order_way", "desc")
	if strings.TrimSpace(imdb) != "" {
		p.Set("imdbid", imdb)
	}
	if strings.TrimSpace(term) != "" {
		p.Set("groupname", term)
	}
	r := indexer.NewRequest(s.baseURL+"torrents.php?"+p.Encode(), indexer.AcceptHTML)
	r.FollowRedirects = true
	r.Cookies = indexer.ParseCookieHeader(s.settings.Cookie)
	if strings.TrimSpace(s.settings.UserAgent) != "" {
		r.Header.Set("User-Agent", s.settings.UserAgent)
	}
	c.AddRequests(r)
	return &c, nil
}

var (
	groupSel    = cascadia.MustCompile(`div.browsePoster`)
	groupName   = cascadia.MustCompile(`strong:has(a[title="View Torrent"])`)
	imdbLink    = cascadia.MustCompile(`a[href*="imdb.com/title/tt"]`)
	rows        = cascadia.MustCompile(`tr.group_torrent:has(a[href^="torrents.php?id="])`)
	downloadLnk = cascadia.MustCompile(`a[href^="torrents.php?action=download"]`)
	detailsLnk  = cascadia.MustCompile(`a[href^="torrents.php?id="]`)
	seedersCell = cascadia.MustCompile(`td:nth-child(7)`)
	leechCell   = cascadia.MustCompile(`td:nth-child(8)`)
	sizeCell    = cascadia.MustCompile(`td:nth-child(4)`)
	grabsCell   = cascadia.MustCompile(`td:nth-child(6)`)
	timeSpan    = cascadia.MustCompile(`td:nth-child(3) span[class="time"]`)
	addedCell   = cascadia.MustCompile(`td:nth-child(3)`)
)

// ParseResponse implements indexer.Site, as Prowlarr's PixelHDParser
// does: a release of each torrent of each group of torrents.
func (s *Site) ParseResponse(_ context.Context, ix *indexer.Indexer, r *indexer.Response) ([]*indexer.Release, error) {
	if r.StatusCode != http.StatusOK {
		return nil, indexer.Errorf("Search returned unexpected result. Expected 200 OK but got %s.", statusName(r.StatusCode))
	}
	doc, err := dom.Parse(r.Content())
	if err != nil {
		return nil, err
	}
	now := time.Now()
	if ix != nil && ix.Now != nil {
		now = ix.Now()
	}
	var releases []*indexer.Release
	for _, group := range groupSel.MatchAll(doc) {
		var name string
		if n := groupName.MatchFirst(group); n != nil {
			name = strings.ReplaceAll(dom.Text(n), " ]", "]")
		}
		var imdbID int
		if l := imdbLink.MatchFirst(group); l != nil {
			href := strings.TrimRight(dom.Attr(l, "href"), "/")
			imdbID, _ = parse.IMDbID(href[strings.LastIndex(href, "/")+1:])
		}
		for _, row := range rows.MatchAll(group) {
			rel, err := s.release(now, row, name)
			if err != nil {
				return nil, err
			}
			rel.IMDbID = imdbID
			releases = append(releases, rel)
		}
	}
	return releases, nil
}

func (s *Site) release(now time.Time, row *html.Node, groupName string) (*indexer.Release, error) {
	details := detailsLnk.MatchFirst(row)
	infoURL := s.baseURL + dom.Attr(details, "href")
	var title string
	if details != nil {
		title = strings.TrimSpace(dom.Text(details))
	}
	seeders, err := cellInt(seedersCell.MatchFirst(row))
	if err != nil {
		return nil, err
	}
	leechers, err := cellInt(leechCell.MatchFirst(row))
	if err != nil {
		return nil, err
	}
	grabs, err := cellInt(grabsCell.MatchFirst(row))
	if err != nil {
		return nil, err
	}
	sizeNode := sizeCell.MatchFirst(row)
	if sizeNode == nil {
		return nil, errors.New("pixelhd: a release has no size")
	}
	size, err := parse.Bytes(strings.TrimSpace(dom.Text(sizeNode)))
	if err != nil {
		return nil, fmt.Errorf("pixelhd: %w", err)
	}
	date, err := publishDate(now, row)
	if err != nil {
		return nil, err
	}
	return &indexer.Release{
		GUID:                 infoURL,
		InfoURL:              infoURL,
		DownloadURL:          s.baseURL + dom.Attr(downloadLnk.MatchFirst(row), "href"),
		Title:                strings.TrimSpace(groupName + " " + title),
		Protocol:             indexer.ProtocolTorrent,
		Categories:           []int{indexer.Movies.ID, indexer.MoviesHD.ID},
		Seeders:              &seeders,
		Peers:                indexer.Ptr(seeders + leechers),
		Size:                 &size,
		Grabs:                &grabs,
		PublishDate:          date,
		DownloadVolumeFactor: indexer.Ptr(0.0),
		UploadVolumeFactor:   indexer.Ptr(1.0),
		MinimumRatio:         indexer.Ptr(1.0),
		MinimumSeedTime:      indexer.Ptr(int64(259200)), // 72 hours
	}, nil
}

// publishDate reads when a torrent was added: the date of its time span's
// title, in the time zone of the site's server taken as the local one,
// else the time ago of its cell.
func publishDate(now time.Time, row *html.Node) (time.Time, error) {
	if n := timeSpan.MatchFirst(row); n != nil {
		if t, err := time.ParseInLocation("Jan 02 2006, 15:04", strings.TrimSpace(dom.Attr(n, "title")), time.Local); err == nil {
			return t, nil
		}
	}
	n := addedCell.MatchFirst(row)
	if n == nil {
		return time.Time{}, errors.New("pixelhd: a release has no date")
	}
	t, err := parse.TimeAgo(now, strings.TrimSpace(dom.Text(n)))
	if err != nil {
		return time.Time{}, fmt.Errorf("pixelhd: %w", err)
	}
	return t, nil
}

// cellInt reads a cell's integer as Prowlarr's ParseUtil.CoerceInt does;
// missing cells fail.
func cellInt(n *html.Node) (int, error) {
	if n == nil {
		return 0, errors.New("pixelhd: a release has a missing column")
	}
	v, err := parse.Int(dom.Text(n))
	if err != nil {
		return 0, fmt.Errorf("pixelhd: %w", err)
	}
	return int(v), nil
}

// statusName names a status as .NET's HttpStatusCode does, such as
// "NotFound"; its number when it has no name.
func statusName(code int) string {
	if t := http.StatusText(code); t != "" {
		return strings.NewReplacer(" ", "", "-", "").Replace(t)
	}
	return fmt.Sprint(code)
}
