package immortalseed

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/andybalholm/cascadia"
	"golang.org/x/net/html"

	"github.com/mavioai/mavio/plugins/indexers/internal/dom"
	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
	"github.com/mavioai/mavio/plugins/indexers/internal/parse"
)

var links = []string{"https://immortalseed.me/"}

// Definition is ImmortalSeed.
var Definition = indexer.Definition{
	ID:          "immortalseed",
	Name:        "ImmortalSeed",
	Description: "ImmortalSeed (iS) is a Private Torrent Tracker for MOVIES / TV / GENERAL",
	Protocol:    indexer.ProtocolTorrent,
	Privacy:     indexer.Private,
	Links:       links,
	Fields: []indexer.Field{
		indexer.BaseURLField(links),
		{Name: "username", Label: "Username", Type: indexer.FieldText, HelpText: "Site Username"},
		{Name: "password", Label: "Password", Type: indexer.FieldText, Secret: true, HelpText: "Site Password"},
		{Name: "freeleech_only", Label: "Freeleech Only", Type: indexer.FieldCheckbox, HelpText: "Show freeleech releases only"},
	},
	New: newSite,
}

// Settings are ImmortalSeed's settings.
type Settings struct {
	indexer.UserPassSettings
	FreeleechOnly bool `json:"freeleech_only,omitempty"`
}

// Site is ImmortalSeed.
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
	c.Categories.Add("3", indexer.Other, "Nuked")
	c.Categories.Add("32", indexer.TVAnime, "Anime")
	c.Categories.Add("23", indexer.PC, "Apps")
	c.Categories.Add("35", indexer.AudioAudiobook, "Audiobooks")
	c.Categories.Add("31", indexer.TV, "Childrens/Cartoons")
	c.Categories.Add("54", indexer.TVDocumentary, "Documentary - HD")
	c.Categories.Add("53", indexer.TVDocumentary, "Documentary - SD")
	c.Categories.Add("22", indexer.BooksEBook, "Ebooks")
	c.Categories.Add("41", indexer.BooksComics, "Comics")
	c.Categories.Add("46", indexer.BooksMags, "Magazines")
	c.Categories.Add("25", indexer.PCGames, "Games")
	c.Categories.Add("61", indexer.ConsoleNDS, "Games Nintendo")
	c.Categories.Add("26", indexer.PCGames, "Games-PC ISO")
	c.Categories.Add("28", indexer.ConsolePS4, "Games-PSx")
	c.Categories.Add("29", indexer.ConsoleXBox, "Games Xbox")
	c.Categories.Add("49", indexer.PCMobileOther, "Mobile")
	c.Categories.Add("51", indexer.PCMobileAndroid, "Android")
	c.Categories.Add("50", indexer.PCMobileiOS, "IOS")
	c.Categories.Add("52", indexer.PC0day, "Windows")
	c.Categories.Add("59", indexer.MoviesUHD, "Movies-4k")
	c.Categories.Add("60", indexer.MoviesForeign, "Non-English 4k Movies")
	c.Categories.Add("16", indexer.MoviesHD, "Movies HD")
	c.Categories.Add("18", indexer.MoviesForeign, "Movies HD Non-English")
	c.Categories.Add("17", indexer.MoviesSD, "TS/CAM/PPV")
	c.Categories.Add("34", indexer.MoviesForeign, "Movies Low Def Non-English")
	c.Categories.Add("62", indexer.Movies, "Movies-Packs")
	c.Categories.Add("14", indexer.MoviesSD, "Movies-SD")
	c.Categories.Add("33", indexer.MoviesForeign, "Movies SD Non-English")
	c.Categories.Add("30", indexer.AudioOther, "Music")
	c.Categories.Add("37", indexer.AudioLossless, "FLAC")
	c.Categories.Add("36", indexer.AudioMP3, "MP3")
	c.Categories.Add("39", indexer.AudioOther, "Music Other")
	c.Categories.Add("38", indexer.AudioVideo, "Music Video")
	c.Categories.Add("45", indexer.Other, "Other")
	c.Categories.Add("7", indexer.TVSport, "Sports Tv")
	c.Categories.Add("44", indexer.TVSport, "Sports Fitness-Instructional")
	c.Categories.Add("58", indexer.TVSport, "Olympics")
	c.Categories.Add("67", indexer.TVSport, "World Cup")
	c.Categories.Add("47", indexer.TVSD, "TV - 480p")
	c.Categories.Add("64", indexer.TVUHD, "TV - 4K")
	c.Categories.Add("8", indexer.TVHD, "TV - High Definition")
	c.Categories.Add("48", indexer.TVSD, "TV - Standard Definition - x264")
	c.Categories.Add("9", indexer.TVSD, "TV - Standard Definition - XviD")
	c.Categories.Add("63", indexer.TVUHD, "TV Season Packs - 4K")
	c.Categories.Add("4", indexer.TVHD, "TV Season Packs - HD")
	c.Categories.Add("6", indexer.TVSD, "TV Season Packs - SD")
	return c
}

// Info implements indexer.Site.
func (s *Site) Info() indexer.Info {
	return indexer.Info{
		Protocol:       indexer.ProtocolTorrent,
		Privacy:        indexer.Private,
		SupportsRSS:    true,
		SupportsSearch: true,
		RateLimit:      5 * time.Second,
	}
}

// Capabilities implements indexer.Site.
func (s *Site) Capabilities(context.Context, *indexer.Indexer) (*indexer.Capabilities, error) {
	return s.caps, nil
}

// separators are the characters searched as spaces: the range from the
// space to the point, and the underscore.
var separators = regexp.MustCompile(`[ -._]+`)

// SearchRequests implements indexer.Site: one search of browse.php.
func (s *Site) SearchRequests(_ context.Context, _ *indexer.Indexer, q *indexer.Query) (*indexer.Chain, error) {
	term := q.SanitizedTerm()
	if q.Type == indexer.SearchTV {
		term = q.SanitizedTVString()
	}
	var p indexer.Params
	p.Set("category", "0")
	p.Set("include_dead_torrents", "yes")
	p.Set("sort", "added")
	p.Set("order", "desc")
	term = strings.TrimSpace(separators.ReplaceAllString(term, " "))
	if term != "" {
		p.Set("do", "search")
		p.Set("keywords", term)
		p.Set("search_type", "t_name")
	}
	if cats := s.caps.Categories.MapToTracker(q.Categories, false); len(cats) > 0 {
		p.Set("selectedcats2", strings.Join(cats, ","))
	}
	r := indexer.NewRequest(s.baseURL+"browse.php?"+p.Encode(), indexer.AcceptHTML)
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

// Login implements indexer.LoginSite: it posts the user name and password
// to takelogin.php and keeps the cookies set, for 30 days.
func (s *Site) Login(ctx context.Context, ix *indexer.Indexer) error {
	r := indexer.NewRequest(s.baseURL+"takelogin.php", "")
	var form indexer.Params
	form.Set("username", s.settings.Username)
	form.Set("password", s.settings.Password)
	r.Method = http.MethodPost
	r.Body = []byte(form.Encode())
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.FollowRedirects = true
	resp, err := ix.Do(ctx, r)
	if err != nil {
		return fmt.Errorf("immortalseed login: %w", err)
	}
	if resp.HasError() {
		return fmt.Errorf("immortalseed login: %w", &indexer.HTTPError{Response: resp})
	}
	if !strings.Contains(resp.Content(), "logout.php") {
		return fmt.Errorf("ImmortalSeed Auth Failed: %w", indexer.ErrAuth)
	}
	ix.SetCookies(resp.Cookies, time.Time{})
	return nil
}

var (
	rows        = cascadia.MustCompile(`table#sortabletable > tbody > tr:has(a[href*="details.php?id="])`)
	free        = cascadia.MustCompile(`img[title^="Free Torrent"], img[title^="Sitewide Free Torrent"]`)
	silver      = cascadia.MustCompile(`img[title^="Silver Torrent"]`)
	double      = cascadia.MustCompile(`img[title^="x2 Torrent"]`)
	detailsLink = cascadia.MustCompile(`div > a[href*="details.php?id="]`)
	tooltip     = cascadia.MustCompile(`.tooltip-content > div:nth-of-type(1)`)
	tooltipInfo = cascadia.MustCompile(`.tooltip-content > div:nth-of-type(2)`)
	downloadLnk = cascadia.MustCompile(`a[href*="download.php"]`)
	seedersCell = cascadia.MustCompile(`td:nth-of-type(7)`)
	leechCell   = cascadia.MustCompile(`td:nth-of-type(8)`)
	categoryLnk = cascadia.MustCompile(`td:nth-of-type(1) a`)
	sizeCell    = cascadia.MustCompile(`td:nth-of-type(5)`)
	grabsCell   = cascadia.MustCompile(`td:nth-child(6)`)
	dateCell    = cascadia.MustCompile(`td:nth-of-type(2) > div:last-child`)
	dateAdded   = regexp.MustCompile(`\d{4}-\d{2}-\d{2} \d{2}:\d{2} [AaPp][Mm]`)
)

// ParseResponse implements indexer.Site, as Prowlarr's ImmortalSeedParser
// does.
func (s *Site) ParseResponse(_ context.Context, _ *indexer.Indexer, r *indexer.Response) ([]*indexer.Release, error) {
	doc, err := dom.Parse(r.Content())
	if err != nil {
		return nil, err
	}
	var releases []*indexer.Release
	for _, row := range rows.MatchAll(doc) {
		down := 1.0
		switch {
		case free.MatchFirst(row) != nil:
			down = 0
		case silver.MatchFirst(row) != nil:
			down = 0.5
		}
		if s.settings.FreeleechOnly && down != 0 {
			continue
		}
		rel, err := s.release(row, down)
		if err != nil {
			return nil, err
		}
		releases = append(releases, rel)
	}
	return releases, nil
}

func (s *Site) release(row *html.Node, down float64) (*indexer.Release, error) {
	// The details link, whose text is the title, cut when it is long.
	details := detailsLink.MatchFirst(row)
	// The title is the tooltip's, when there is one.
	titleNode := tooltip.MatchFirst(row)
	if titleNode == nil {
		titleNode = details
	}
	var title, description string
	if titleNode != nil {
		title = strings.TrimSpace(dom.Text(titleNode))
	}
	if n := tooltipInfo.MatchFirst(row); n != nil {
		description = strings.TrimSpace(strings.NewReplacer("|", ",", " ", "").Replace(dom.Text(n)))
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
	catLink := categoryLnk.MatchFirst(row)
	if catLink == nil {
		return nil, errors.New("immortalseed: a release has no category link")
	}
	sizeNode := sizeCell.MatchFirst(row)
	if sizeNode == nil {
		return nil, errors.New("immortalseed: a release has no size")
	}
	size, err := parse.Bytes(strings.TrimSpace(dom.Text(sizeNode)))
	if err != nil {
		return nil, fmt.Errorf("immortalseed: %w", err)
	}
	infoURL := dom.Attr(details, "href")
	up := 1.0
	if double.MatchFirst(row) != nil {
		up = 2
	}
	rel := &indexer.Release{
		GUID:                 infoURL,
		InfoURL:              infoURL,
		DownloadURL:          dom.Attr(downloadLnk.MatchFirst(row), "href"),
		Title:                title,
		Description:          description,
		Protocol:             indexer.ProtocolTorrent,
		Categories:           indexer.IDs(s.caps.Categories.MapTracker(parse.QueryArg(dom.Attr(catLink, "href"), "category"))),
		Seeders:              &seeders,
		Peers:                indexer.Ptr(seeders + leechers),
		Size:                 &size,
		Grabs:                &grabs,
		DownloadVolumeFactor: &down,
		UploadVolumeFactor:   &up,
		MinimumRatio:         indexer.Ptr(1.0),
		MinimumSeedTime:      indexer.Ptr(int64(86400)), // 24 hours
	}
	date := dateCell.MatchFirst(row)
	if date == nil {
		return nil, errors.New("immortalseed: a release has no date")
	}
	if m := dateAdded.FindString(strings.TrimSpace(dom.Text(date))); m != "" {
		// The site's dates are in the time zone of its server, taken as
		// the local one.
		t, err := time.ParseInLocation("2006-01-02 03:04 PM", strings.ToUpper(m), time.Local)
		if err != nil {
			return nil, fmt.Errorf("immortalseed: %w", err)
		}
		rel.PublishDate = t
	}
	return rel, nil
}

// cellInt reads a cell's integer as Prowlarr's ParseUtil.CoerceInt does;
// missing cells fail.
func cellInt(n *html.Node) (int, error) {
	if n == nil {
		return 0, errors.New("immortalseed: a release has a missing column")
	}
	v, err := parse.Int(dom.Text(n))
	if err != nil {
		return 0, fmt.Errorf("immortalseed: %w", err)
	}
	return int(v), nil
}
