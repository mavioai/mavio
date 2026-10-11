package bithdtv

import (
	"context"
	"errors"
	"fmt"
	"net/url"
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

var links = []string{"https://www.bit-hdtv.com/"}

// Definition is BitHDTV.
var Definition = indexer.Definition{
	ID:          "bithdtv",
	Name:        "BitHDTV",
	Description: "BIT-HDTV - Home of High Definition",
	Language:    "en-US",
	Protocol:    indexer.ProtocolTorrent,
	Privacy:     indexer.Private,
	Links:       links,
	Fields: []indexer.Field{
		indexer.BaseURLField(links),
		indexer.CookieField,
	},
	New: newSite,
}

// Settings are BitHDTV's settings.
type Settings struct {
	indexer.BaseSettings
	Cookie string `json:"cookie,omitempty"`
}

// Validate checks that the cookie is set.
func (s Settings) Validate() error {
	if strings.TrimSpace(s.Cookie) == "" {
		return errors.New("cookie: must not be empty")
	}
	return nil
}

// Site is BitHDTV.
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
	c.Categories.Add("1", indexer.TVAnime, "Anime")
	c.Categories.Add("2", indexer.MoviesBluRay, "Movies/Blu-ray")
	c.Categories.Add("4", indexer.TVDocumentary, "Documentaries")
	c.Categories.Add("6", indexer.AudioLossless, "HQ Audio")
	c.Categories.Add("7", indexer.Movies, "Movies")
	c.Categories.Add("8", indexer.AudioVideo, "Music Videos")
	c.Categories.Add("9", indexer.Other, "Other")
	c.Categories.Add("5", indexer.TVSport, "Sports")
	c.Categories.Add("10", indexer.TV, "TV")
	c.Categories.Add("12", indexer.TV, "TV/Seasonpack")
	c.Categories.Add("11", indexer.XXX, "XXX")
	return c
}

// Info implements indexer.Site.
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

// SearchRequests implements indexer.Site: a search of the titles or, by
// IMDb ID, a search of the IMDb links then one of the titles and
// descriptions.
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
	cat := "0"
	if cats := s.caps.Categories.MapToTracker(q.Categories, true); len(cats) == 1 {
		cat = cats[0]
	}
	// Prowlarr builds the URL with a UriBuilder, which writes the default
	// port; the address is the same without it.
	searchURL := strings.TrimRight(s.baseURL, "/") + "/torrents.php"
	var p indexer.Params
	p.Set("cat", cat)
	var reqs []*indexer.Request
	if strings.TrimSpace(imdb) != "" {
		p.Set("search", imdb)
		// Search the IMDb links.
		p.Set("options", "4")
		reqs = append(reqs, s.request(searchURL+"?"+p.Encode()))
		// Search the titles and descriptions.
		p.Set("options", "1")
		reqs = append(reqs, s.request(searchURL+"?"+p.Encode()))
	} else {
		// The site takes empty searches. Search the titles only.
		p.Set("search", term)
		p.Set("options", "0")
		reqs = append(reqs, s.request(searchURL+"?"+p.Encode()))
	}
	var c indexer.Chain
	c.AddRequests(reqs...)
	return &c, nil
}

func (s *Site) request(u string) *indexer.Request {
	r := indexer.NewRequest(u, indexer.AcceptHTML)
	r.FollowRedirects = true
	r.Cookies = indexer.ParseCookieHeader(s.settings.Cookie)
	return r
}

var (
	needSeed    = cascadia.MustCompile(`#needseed`)
	resultTable = cascadia.MustCompile(`table[align=center] + br + table > tbody`)
	anchor      = cascadia.MustCompile(`a`)
	downloadLnk = cascadia.MustCompile(`a[href^="download.php"]`)
)

// ParseResponse implements indexer.Site, as Prowlarr's BitHDTVParser
// does.
func (s *Site) ParseResponse(_ context.Context, _ *indexer.Indexer, r *indexer.Response) ([]*indexer.Release, error) {
	doc, err := dom.Parse(r.Content())
	if err != nil {
		return nil, err
	}
	for _, n := range needSeed.MatchAll(doc) {
		if n.Parent != nil {
			n.Parent.RemoveChild(n)
		}
	}
	table := resultTable.MatchFirst(doc)
	if table == nil {
		return nil, nil // no results
	}
	rows := dom.Children(table)
	if len(rows) > 0 {
		rows = rows[1:] // the header
	}
	var releases []*indexer.Release
	for _, row := range rows {
		cells := dom.Children(row)
		if len(cells) < 10 {
			return nil, fmt.Errorf("bithdtv: a row has %d columns, want = 10", len(cells))
		}
		link := anchor.MatchFirst(cells[2])
		if link == nil {
			return nil, errors.New("bithdtv: a release has no details link")
		}
		details, err := url.Parse(dom.Attr(link, "href"))
		if err != nil || !details.IsAbs() {
			return nil, fmt.Errorf("bithdtv: invalid details link %q", dom.Attr(link, "href"))
		}
		guid := details.String()
		// Irrelevant and repeated entries are skipped.
		if slices.ContainsFunc(releases, func(r *indexer.Release) bool { return r.GUID == guid }) {
			continue
		}
		rel, err := s.release(row, cells, guid, dom.Attr(link, "title"))
		if err != nil {
			return nil, err
		}
		releases = append(releases, rel)
	}
	return releases, nil
}

func (s *Site) release(row *html.Node, cells []*html.Node, guid, title string) (*indexer.Release, error) {
	dl := downloadLnk.MatchFirst(row)
	if dl == nil {
		return nil, errors.New("bithdtv: a release has no download link")
	}
	catLink := firstElement(cells[1])
	if catLink == nil {
		return nil, errors.New("bithdtv: a release has no category link")
	}
	ints := make([]int, 4)
	for i, col := range []int{3, 7, 8, 9} {
		n, err := parse.Int(strings.TrimSpace(dom.Text(cells[col])))
		if err != nil {
			return nil, fmt.Errorf("bithdtv: %w", err)
		}
		ints[i] = int(n)
	}
	// The site's dates are in the time zone of its server, taken as the
	// local one.
	date, err := time.ParseInLocation("2006-01-0215:04:05", strings.TrimSpace(dom.Text(cells[5])), time.Local)
	if err != nil {
		return nil, fmt.Errorf("bithdtv: %w", err)
	}
	size, err := parse.Bytes(dom.Text(cells[6]))
	if err != nil {
		return nil, fmt.Errorf("bithdtv: %w", err)
	}
	rel := &indexer.Release{
		GUID:            guid,
		InfoURL:         guid,
		DownloadURL:     absolute(s.baseURL + dom.Attr(dl, "href")),
		Title:           title,
		Protocol:        indexer.ProtocolTorrent,
		Categories:      indexer.IDs(s.caps.Categories.MapTracker(parse.QueryArg(s.baseURL+dom.Attr(catLink, "href"), "cat"))),
		Size:            &size,
		Files:           &ints[0],
		Grabs:           &ints[1],
		Seeders:         &ints[2],
		Peers:           indexer.Ptr(ints[3] + ints[2]),
		PublishDate:     date,
		MinimumRatio:    indexer.Ptr(1.0),
		MinimumSeedTime: indexer.Ptr(int64(172800)), // 48 hours
	}
	down, up := 1.0, 1.0
	switch dom.Attr(row, "bgcolor") {
	case "#DDDDDD":
		up = 2
	case "#FFFF99":
		down = 0
	case "#CCFF99":
		down, up = 0, 2
	}
	rel.DownloadVolumeFactor = &down
	rel.UploadVolumeFactor = &up
	return rel, nil
}

// firstElement returns the first child element of n; nil for none.
func firstElement(n *html.Node) *html.Node {
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
