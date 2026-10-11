package xspeeds

import (
	"context"
	"errors"
	"fmt"
	"maps"
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

var links = []string{"https://www.xspeeds.eu/"}

// Definition is XSpeeds.
var Definition = indexer.Definition{
	ID:          "xspeeds",
	Name:        "XSpeeds",
	Description: "XSpeeds (XS) is a Private Torrent Tracker for MOVIES / TV / GENERAL",
	Protocol:    indexer.ProtocolTorrent,
	Privacy:     indexer.Private,
	Links:       links,
	Fields: []indexer.Field{
		indexer.BaseURLField(links),
		indexer.UsernameField,
		indexer.PasswordField,
		{Name: "freeleech_only", Label: "Freeleech Only", Type: indexer.FieldCheckbox, HelpText: "Show freeleech releases only"},
	},
	New: newSite,
}

// Settings are XSpeeds' settings.
type Settings struct {
	indexer.UserPassSettings
	FreeleechOnly bool `json:"freeleech_only,omitempty"`
}

// Site is XSpeeds.
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
	for _, m := range []struct {
		tracker string
		cat     *indexer.Category
		desc    string
	}{
		{"70", indexer.TVAnime, "Anime"},
		{"113", indexer.TVAnime, "Anime Boxsets"},
		{"112", indexer.MoviesOther, "Anime Movies"},
		{"111", indexer.MoviesOther, "Anime TV"},
		{"150", indexer.PC, "Apps"},
		{"153", indexer.Books, "Books"},
		{"154", indexer.AudioAudiobook, "Books Audiobooks"},
		{"155", indexer.Books, "Books eBooks & Magazines"},
		{"68", indexer.MoviesOther, "Cams/TS"},
		{"140", indexer.TVDocumentary, "Documentary"},
		{"10", indexer.MoviesDVD, "DVDR"},
		{"109", indexer.MoviesBluRay, "DVDR Bluray Disc"},
		{"131", indexer.TVSport, "Fighting"},
		{"134", indexer.TVSport, "Fighting Boxing"},
		{"133", indexer.TVSport, "Fighting MMA"},
		{"132", indexer.TVSport, "Fighting Wrestling"},
		{"72", indexer.MoviesForeign, "Foreign"},
		{"116", indexer.TVForeign, "Foreign Boxsets"},
		{"114", indexer.MoviesForeign, "Foreign Movies"},
		{"115", indexer.TVForeign, "Foreign TV"},
		{"103", indexer.ConsoleOther, "Games Console"},
		{"105", indexer.ConsoleOther, "Games Console Nintendo"},
		{"104", indexer.ConsolePS4, "Games Console Playstation"},
		{"106", indexer.ConsoleXBox, "Games Console XBOX"},
		{"6", indexer.PCGames, "Games PC"},
		{"108", indexer.PC, "Games PC Linux"},
		{"107", indexer.PCMac, "Games PC Mac"},
		{"11", indexer.Movies, "Movie Boxsets"},
		{"118", indexer.MoviesUHD, "Movie Boxsets 4K"},
		{"162", indexer.MoviesHD, "Movie Boxsets AV1"},
		{"143", indexer.MoviesHD, "Movie Boxsets HD"},
		{"119", indexer.MoviesHD, "Movie Boxsets HEVC"},
		{"144", indexer.MoviesSD, "Movie Boxsets SD"},
		{"12", indexer.Movies, "Movies"},
		{"117", indexer.MoviesUHD, "Movies 4K"},
		{"163", indexer.MoviesHD, "Movies AV1"},
		{"145", indexer.MoviesHD, "Movies HD"},
		{"100", indexer.MoviesHD, "Movies HEVC"},
		{"146", indexer.MoviesSD, "Movies SD"},
		{"13", indexer.Audio, "Music"},
		{"135", indexer.AudioLossless, "Music FLAC"},
		{"151", indexer.Audio, "Music Karaoke"},
		{"136", indexer.Audio, "Music Boxset"},
		{"148", indexer.AudioVideo, "Music Videos"},
		{"9", indexer.Other, "Other"},
		{"125", indexer.Other, "Other Pictures"},
		{"54", indexer.TVOther, "Other Soaps"},
		{"83", indexer.TVOther, "Other Specials"},
		{"139", indexer.TV, "TOTM (Freeleech)"},
		{"138", indexer.TV, "TOTW (x2 upload)"},
		{"139", indexer.Movies, "TOTM (Freeleech)"},
		{"138", indexer.Movies, "TOTW (x2 upload)"},
		{"20", indexer.TVSport, "Sports"},
		{"88", indexer.TVSport, "Sports/Football"},
		{"86", indexer.TVSport, "Sports/MotorSports"},
		{"89", indexer.TVSport, "Sports/Olympics"},
		{"126", indexer.TV, "TV"},
		{"127", indexer.TVUHD, "TV 4K"},
		{"164", indexer.TVHD, "TV AV1"},
		{"129", indexer.TVHD, "TV HD"},
		{"130", indexer.TVHD, "TV HEVC"},
		{"128", indexer.TVSD, "TV SD"},
		{"149", indexer.TV, "TV Specials"},
		{"21", indexer.TVSD, "TV Boxsets"},
		{"120", indexer.TVUHD, "TV Boxset 4K"},
		{"165", indexer.TVUHD, "TV Boxset AV1"},
		{"76", indexer.TVHD, "TV Boxset HD"},
		{"97", indexer.TVHD, "TV Boxset HEVC"},
		{"147", indexer.TVSD, "TV Boxset SD"},
	} {
		c.Categories.Add(m.tracker, m.cat, m.desc)
	}
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

// separators are the runs of spaces, dashes, dots and underscores, which
// are searched as spaces.
var separators = regexp.MustCompile(`[ -._]+`)

// SearchRequests implements indexer.Site: one search of browse.php's
// names, of one category or all, newest first.
func (s *Site) SearchRequests(_ context.Context, _ *indexer.Indexer, q *indexer.Query) (*indexer.Chain, error) {
	term := q.SanitizedTerm()
	if q.Type == indexer.SearchTV {
		term = q.SanitizedTVString()
	}
	category := "0"
	if cats := s.caps.Categories.MapToTracker(q.Categories, false); len(cats) == 1 {
		category = cats[0]
	}
	var p indexer.Params
	p.Set("category", category)
	p.Set("include_dead_torrents", "yes")
	p.Set("sort", "added")
	p.Set("order", "desc")
	if term = strings.TrimSpace(separators.ReplaceAllString(term, " ")); term != "" {
		p.Set("do", "search")
		p.Set("keywords", term)
		p.Set("search_type", "t_name")
	}
	r := indexer.NewRequest(s.baseURL+"browse.php?"+p.Encode(), indexer.AcceptHTML)
	r.FollowRedirects = true
	var c indexer.Chain
	c.AddRequests(r)
	return &c, nil
}

// LoginNeeded implements indexer.LoginSite: pages without the logout
// link are those of logged out users.
func (s *Site) LoginNeeded(r *indexer.Response) bool {
	return !strings.Contains(r.Content(), "logout.php")
}

var (
	loginError        = cascadia.MustCompile(`.left_side table:nth-of-type(1) tr:nth-of-type(2)`)
	loginNotification = cascadia.MustCompile(`div.notification-body`)
)

// Login implements indexer.LoginSite: it gets the login page's cookies,
// then posts the credentials to takelogin.php, keeping the cookies for 30
// days.
func (s *Site) Login(ctx context.Context, ix *indexer.Indexer) error {
	landingURL := s.baseURL + "login.php"
	page := indexer.NewRequest(landingURL, "")
	page.FollowRedirects = true
	landing, err := ix.Do(ctx, page)
	if err != nil {
		return fmt.Errorf("xspeeds login: %w", err)
	}
	if landing.HasError() {
		return fmt.Errorf("xspeeds login: %w", &indexer.HTTPError{Response: landing})
	}
	var form indexer.Params
	form.Set("username", s.settings.Username)
	form.Set("password", s.settings.Password)
	r := indexer.NewRequest(s.baseURL+"takelogin.php", "")
	r.FollowRedirects = true
	r.Method = http.MethodPost
	r.Body = []byte(form.Encode())
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Referer", landingURL)
	r.Cookies = maps.Clone(landing.Cookies)
	resp, err := ix.Do(ctx, r)
	if err != nil {
		return fmt.Errorf("xspeeds login: %w", err)
	}
	if resp.HasError() {
		return fmt.Errorf("xspeeds login: %w", &indexer.HTTPError{Response: resp})
	}
	if s.LoginNeeded(resp) {
		msg := "Unknown error message, please report."
		if doc, err := dom.Parse(resp.Content()); err == nil {
			text := func(n *html.Node) string {
				return strings.ReplaceAll(strings.TrimSpace(dom.Text(n)), "\n\t", " ")
			}
			if n := loginError.MatchFirst(doc); n != nil && strings.TrimSpace(text(n)) != "" {
				msg = text(n)
			} else if n := loginNotification.MatchFirst(doc); n != nil {
				msg = text(n)
			}
		}
		return fmt.Errorf("%w: %s", indexer.ErrAuth, msg)
	}
	cookies := map[string]string{}
	maps.Copy(cookies, r.Cookies)
	maps.Copy(cookies, resp.Cookies)
	ix.SetCookies(cookies, time.Time{})
	return nil
}

var (
	rows          = cascadia.MustCompile(`table#sortabletable > tbody > tr:has(a[href*="details.php?id="])`)
	freeImg       = cascadia.MustCompile(`img[title^="Free Torrent"], img[title^="Sitewide Free Torrent"]`)
	silverImg     = cascadia.MustCompile(`img[title^="Silver Torrent"]`)
	doubleImg     = cascadia.MustCompile(`img[title^="x2 Torrent"]`)
	detailsLink   = cascadia.MustCompile(`div > a[href*="details.php?id="]`)
	downloadLink  = cascadia.MustCompile(`a[href*="download.php"]`)
	tooltip       = cascadia.MustCompile(`.tooltip-content > div:nth-of-type(2)`)
	sizeCell      = cascadia.MustCompile(`td:nth-of-type(5)`)
	grabsCell     = cascadia.MustCompile(`td:nth-child(6)`)
	seedersCell   = cascadia.MustCompile(`td:nth-of-type(7)`)
	leechersCell  = cascadia.MustCompile(`td:nth-of-type(8)`)
	categoryLink  = cascadia.MustCompile(`td:nth-of-type(1) a`)
	dateCell      = cascadia.MustCompile(`td:nth-of-type(2) > div:last-child`)
	dateAddedExpr = regexp.MustCompile(`\d{2}-\d{2}-\d{4} \d{2}:\d{2}`)
)

// ParseResponse implements indexer.Site, as Prowlarr's XSpeedsParser
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
			return nil, fmt.Errorf("xspeeds: %w", err)
		}
		if rel != nil {
			releases = append(releases, rel)
		}
	}
	return releases, nil
}

// text returns the text of a row's first match; "" when nothing matches.
func text(row *html.Node, sel cascadia.Selector) string {
	if n := sel.MatchFirst(row); n != nil {
		return dom.Text(n)
	}
	return ""
}

// release reads a row; nil for those skipped as not freeleech.
func (s *Site) release(row *html.Node) (*indexer.Release, error) {
	dvf := 1.0
	switch {
	case freeImg.MatchFirst(row) != nil:
		dvf = 0
	case silverImg.MatchFirst(row) != nil:
		dvf = 0.5
	}
	if s.settings.FreeleechOnly && dvf != 0 {
		return nil, nil
	}
	var title, infoURL string
	if a := detailsLink.MatchFirst(row); a != nil {
		title, infoURL = strings.TrimSpace(dom.Text(a)), dom.Attr(a, "href")
	}
	var downloadURL string
	if a := downloadLink.MatchFirst(row); a != nil {
		downloadURL = dom.Attr(a, "href")
	}
	var description string
	if n := tooltip.MatchFirst(row); n != nil {
		description = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(dom.Text(n), "|", ","), " ", ""))
	}
	seeders, err := parse.Int(text(row, seedersCell))
	if err != nil {
		return nil, err
	}
	leechers, err := parse.Int(strings.TrimSpace(text(row, leechersCell)))
	if err != nil {
		return nil, err
	}
	cat := categoryLink.MatchFirst(row)
	if cat == nil {
		return nil, errors.New("a release has no category")
	}
	size, err := parse.Bytes(strings.TrimSpace(text(row, sizeCell)))
	if err != nil {
		return nil, err
	}
	grabs, err := parse.Int(text(row, grabsCell))
	if err != nil {
		return nil, err
	}
	uvf := 1.0
	if doubleImg.MatchFirst(row) != nil {
		uvf = 2
	}
	rel := &indexer.Release{
		GUID:                 infoURL,
		InfoURL:              infoURL,
		DownloadURL:          downloadURL,
		Title:                title,
		Description:          description,
		Categories:           indexer.IDs(s.caps.Categories.MapTracker(parse.QueryArg(dom.Attr(cat, "href"), "category"))),
		Seeders:              indexer.Ptr(int(seeders)),
		Peers:                indexer.Ptr(int(seeders + leechers)),
		Size:                 &size,
		Grabs:                indexer.Ptr(int(grabs)),
		DownloadVolumeFactor: indexer.Ptr(dvf),
		UploadVolumeFactor:   indexer.Ptr(uvf),
		MinimumRatio:         indexer.Ptr(0.8),
	}
	date := dateCell.MatchFirst(row)
	if date == nil {
		return nil, errors.New("a release has no date")
	}
	if m := dateAddedExpr.FindString(strings.TrimSpace(dom.Text(date))); m != "" {
		if rel.PublishDate, err = time.ParseInLocation("02-01-2006 15:04", m, time.Local); err != nil {
			return nil, err
		}
	}
	return rel, nil
}
