package libble

import (
	"context"
	"errors"
	"fmt"
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

var links = []string{"https://libble.me/"}

// Definition is Libble.
var Definition = indexer.Definition{
	ID:          "libble",
	Name:        "Libble",
	Description: "Libble is a Private Torrent Tracker for MUSIC",
	Language:    "en-US",
	Protocol:    indexer.ProtocolTorrent,
	Privacy:     indexer.Private,
	Links:       links,
	Fields: []indexer.Field{
		indexer.BaseURLField(links),
		{Name: "username", Label: "Username", Type: indexer.FieldText, HelpText: "Site Username"},
		{Name: "password", Label: "Password", Type: indexer.FieldText, Secret: true, HelpText: "Site Password"},
		{
			Name: "two_factor_auth_code", Label: "2FA code", Type: indexer.FieldText,
			HelpText: "Only fill in the <b>2FA code</b> box if you have enabled <b>2FA</b> on the Libble Web Site. Otherwise just leave it empty.",
		},
	},
	New: newSite,
}

// Settings are Libble's settings.
type Settings struct {
	indexer.UserPassSettings
	TwoFactorAuthCode string `json:"two_factor_auth_code,omitempty"`
}

// Site is Libble.
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
	c.Params[indexer.SearchMusic] = []indexer.Param{indexer.ParamQ, indexer.ParamArtist, indexer.ParamAlbum, indexer.ParamLabel, indexer.ParamYear, indexer.ParamGenre}
	c.Categories.Add("1", indexer.Audio, "Music")
	c.Categories.Add("2", indexer.Audio, "Libble Mixtapes")
	c.Categories.Add("7", indexer.AudioVideo, "Music Videos")
	return c
}

// pageSize is the number of releases of a full page.
const pageSize = 50

// Info implements indexer.Site.
func (s *Site) Info() indexer.Info {
	return indexer.Info{
		Protocol:           indexer.ProtocolTorrent,
		Privacy:            indexer.Private,
		SupportsRSS:        true,
		SupportsSearch:     true,
		SupportsPagination: true,
		PageSize:           pageSize,
	}
}

// Capabilities implements indexer.Site.
func (s *Site) Capabilities(context.Context, *indexer.Indexer) (*indexer.Capabilities, error) {
	return s.caps, nil
}

// year is a year ending an album's name.
var year = regexp.MustCompile(`(.+)\b\d{4}$`)

// SearchRequests implements indexer.Site: one advanced search of
// torrents.php for music and basic searches, none for the others.
func (s *Site) SearchRequests(_ context.Context, _ *indexer.Indexer, q *indexer.Query) (*indexer.Chain, error) {
	var c indexer.Chain
	var p indexer.Params
	switch q.Type {
	case indexer.SearchMusic:
		if strings.TrimSpace(q.Artist) != "" && q.Artist != "VA" {
			p.Set("artistname", q.Artist)
		}
		if strings.TrimSpace(q.Album) != "" {
			p.Set("groupname", strings.TrimSpace(year.ReplaceAllString(q.Album, "$1")))
		}
		if strings.TrimSpace(q.Label) != "" {
			p.Set("recordlabel", q.Label)
		}
		if q.Year != 0 {
			p.Set("year", strconv.Itoa(q.Year))
		}
		if strings.TrimSpace(q.Genre) != "" {
			p.Set("taglist", q.Genre)
			p.Set("tags_type", "0")
		}
	case indexer.SearchBasic:
	default:
		return &c, nil
	}
	p.Set("action", "advanced")
	p.Set("order_by", "time")
	p.Set("order_way", "desc")
	if term := strings.TrimSpace(q.SanitizedTerm()); term != "" {
		p.Set("searchstr", term)
	}
	for _, cat := range s.caps.Categories.MapToTracker(q.Categories, false) {
		p.Set("filter_cat["+cat+"]", "1")
	}
	if q.Limit > 0 && q.Offset > 0 {
		p.Set("page", strconv.Itoa(q.Offset/q.Limit+1))
	}
	r := indexer.NewRequest(strings.TrimRight(s.baseURL, "/")+"/torrents.php?"+p.Encode(), indexer.AcceptHTML)
	r.FollowRedirects = true
	c.AddRequests(r)
	return &c, nil
}

// LoginNeeded implements indexer.LoginSite: when the page has no logout
// link.
func (s *Site) LoginNeeded(r *indexer.Response) bool {
	return !strings.Contains(r.Content(), "logout.php")
}

var loginWarning = cascadia.MustCompile(`#loginform > .warning`)

// Login implements indexer.LoginSite: it posts the user name, password
// and 2FA code to login.php, failing with the site's warning, and keeps
// the cookies set, for 30 days.
func (s *Site) Login(ctx context.Context, ix *indexer.Indexer) error {
	loginURL := s.baseURL + "login.php"
	r := indexer.NewRequest(loginURL, "")
	var form indexer.Params
	form.Set("username", s.settings.Username)
	form.Set("password", s.settings.Password)
	form.Set("code", s.settings.TwoFactorAuthCode)
	form.Set("keeplogged", "1")
	form.Set("login", "Login")
	r.Method = http.MethodPost
	r.Body = []byte(form.Encode())
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Referer", loginURL)
	r.FollowRedirects = true
	resp, err := ix.Do(ctx, r)
	if err != nil {
		return fmt.Errorf("libble login: %w", err)
	}
	if resp.HasError() {
		return fmt.Errorf("libble login: %w", &indexer.HTTPError{Response: resp})
	}
	if s.LoginNeeded(resp) {
		msg := "Unknown error message, please report."
		doc, err := dom.Parse(resp.Content())
		if err != nil {
			return err
		}
		if n := loginWarning.MatchFirst(doc); n != nil {
			msg = strings.TrimSpace(dom.Text(n))
		}
		return fmt.Errorf("%s: %w", msg, indexer.ErrAuth)
	}
	ix.SetCookies(resp.Cookies, time.Time{})
	return nil
}

var (
	groups       = cascadia.MustCompile(`table#torrent_table > tbody > tr.group:has(strong > a[href*="torrents.php?id="])`)
	albumLink    = cascadia.MustCompile(`strong > a[href*="torrents.php?id="]`)
	artistLinks  = cascadia.MustCompile(`strong > a[href*="artist.php?id="]`)
	groupTitle   = cascadia.MustCompile(`td:nth-child(4) > strong`)
	tags         = cascadia.MustCompile(`div.tags`)
	thumbnail    = cascadia.MustCompile(`.thumbnail`)
	categoryIcon = cascadia.MustCompile(`td.cats_col div.cat_icon`)
	detailsLink  = cascadia.MustCompile(`a[href^="torrents.php?id="]`)
	downloadLink = cascadia.MustCompile(`a[href^="torrents.php?action=download&id="]`)
	files        = cascadia.MustCompile(`td:nth-child(2)`)
	added        = cascadia.MustCompile(`td:nth-child(3)`)
	addedTitle   = cascadia.MustCompile(`td:nth-child(3) > span[title]`)
	size         = cascadia.MustCompile(`td:nth-child(4)`)
	grabs        = cascadia.MustCompile(`td:nth-child(5)`)
	seeders      = cascadia.MustCompile(`td:nth-child(6)`)
	leechers     = cascadia.MustCompile(`td:nth-child(7)`)
	freeleech    = cascadia.MustCompile(`a[href^="torrents.php?id="] strong`)
	releaseYear  = regexp.MustCompile(`\[(\d{4})\]$`)
	categoryName = regexp.MustCompile(`\bcats_(.*?)\b`)
)

// ParseResponse implements indexer.Site, as Prowlarr's LibbleParser does:
// a release of each torrent of each group of torrents.
func (s *Site) ParseResponse(_ context.Context, ix *indexer.Indexer, r *indexer.Response) ([]*indexer.Release, error) {
	doc, err := dom.Parse(r.Content())
	if err != nil {
		return nil, err
	}
	now := time.Now()
	if ix != nil && ix.Now != nil {
		now = ix.Now()
	}
	var releases []*indexer.Release
	for _, group := range groups.MatchAll(doc) {
		groupID := parse.QueryArg(dom.Attr(albumLink.MatchFirst(group), "href"), "id")
		artist := "Various Artists"
		if artists := artistLinks.MatchAll(group); len(artists) > 0 {
			names := make([]string, len(artists))
			for i, a := range artists {
				names[i] = strings.TrimSpace(dom.Text(a))
			}
			artist = strings.Join(names, ", ")
		}
		album := strings.TrimSpace(dom.Text(albumLink.MatchFirst(group)))
		var title string
		if n := groupTitle.MatchFirst(group); n != nil {
			title = strings.TrimSpace(dom.Text(n))
		}
		albumYear := releaseYear.FindString(title)
		var description, poster string
		if n := tags.MatchFirst(group); n != nil {
			description = strings.TrimSpace(dom.Text(n))
		}
		if n := thumbnail.MatchFirst(group); n != nil {
			poster = strings.TrimSpace(dom.Attr(n, "title"))
		}
		var genres []string
		for g := range strings.SplitSeq(description, ",") {
			if g = strings.TrimSpace(g); g != "" {
				genres = append(genres, g)
			}
		}
		categories, err := parseCategories(group)
		if err != nil {
			return nil, err
		}
		rowsSel, err := cascadia.Compile(`table#torrent_table > tbody > tr.group_torrent.groupid_` + groupID + `:has(a[href*="torrents.php?id="])`)
		if err != nil {
			return nil, fmt.Errorf("libble: group %q: %w", groupID, err)
		}
		for _, row := range rowsSel.MatchAll(doc) {
			rel, err := s.release(now, row)
			if err != nil {
				return nil, err
			}
			rel.Title = strings.Trim(fmt.Sprintf("%s - %s %s %s", artist, album, albumYear, rel.Title), " -")
			rel.Artist = artist
			rel.Album = album
			rel.Categories = categories
			rel.Description = description
			rel.Genres = genres
			rel.PosterURL = poster
			releases = append(releases, rel)
		}
	}
	return releases, nil
}

// release reads a torrent's row, its title the torrent's tags.
func (s *Site) release(now time.Time, row *html.Node) (*indexer.Release, error) {
	details := detailsLink.MatchFirst(row)
	if details == nil {
		return nil, errors.New("libble: a torrent has no details link")
	}
	dl := downloadLink.MatchFirst(row)
	if dl == nil {
		return nil, errors.New("libble: a torrent has no download link")
	}
	cell := func(sel cascadia.Selector) (string, error) {
		n := sel.MatchFirst(row)
		if n == nil {
			return "", errors.New("libble: a torrent has a missing column")
		}
		return dom.Text(n), nil
	}
	var ints [4]int
	for i, sel := range []cascadia.Selector{files, grabs, seeders, leechers} {
		text, err := cell(sel)
		if err != nil {
			return nil, err
		}
		n, err := parse.Int(text)
		if err != nil {
			return nil, fmt.Errorf("libble: %w", err)
		}
		ints[i] = int(n)
	}
	sizeText, err := cell(size)
	if err != nil {
		return nil, err
	}
	sz, err := parse.Bytes(strings.TrimSpace(sizeText))
	if err != nil {
		return nil, fmt.Errorf("libble: %w", err)
	}
	var torrentTags string
	if details.FirstChild != nil {
		torrentTags = strings.Trim(dom.Text(details.FirstChild), " /")
	}
	infoURL := s.baseURL + strings.TrimSpace(dom.Attr(details, "href"))
	rel := &indexer.Release{
		GUID:                 infoURL,
		InfoURL:              infoURL,
		DownloadURL:          s.baseURL + strings.TrimSpace(dom.Attr(dl, "href")),
		Title:                torrentTags,
		Protocol:             indexer.ProtocolTorrent,
		Size:                 &sz,
		Files:                &ints[0],
		Grabs:                &ints[1],
		Seeders:              &ints[2],
		Peers:                indexer.Ptr(ints[2] + ints[3]),
		DownloadVolumeFactor: indexer.Ptr(1.0),
		UploadVolumeFactor:   indexer.Ptr(1.0),
		MinimumRatio:         indexer.Ptr(1.0),
		MinimumSeedTime:      indexer.Ptr(int64(259200)), // 72 hours
	}
	if rel.PublishDate, err = publishDate(now, row); err != nil {
		return nil, err
	}
	if n := freeleech.MatchFirst(row); n != nil {
		switch strings.Trim(strings.ToLower(dom.Text(n)), " !") {
		case "neutral":
			rel.DownloadVolumeFactor = indexer.Ptr(0.0)
			rel.UploadVolumeFactor = indexer.Ptr(0.0)
		case "freeleech":
			rel.DownloadVolumeFactor = indexer.Ptr(0.0)
			rel.UploadVolumeFactor = indexer.Ptr(1.0)
		}
	}
	return rel, nil
}

// publishDate reads when a torrent was added: the date of its span's
// title, in UTC, else the time ago of its cell.
func publishDate(now time.Time, row *html.Node) (time.Time, error) {
	if n := addedTitle.MatchFirst(row); n != nil {
		if t, err := time.ParseInLocation("Jan 02 2006, 15:04", strings.TrimSpace(dom.Attr(n, "title")), time.UTC); err == nil {
			return t, nil
		}
	}
	n := added.MatchFirst(row)
	if n == nil {
		return time.Time{}, errors.New("libble: a torrent has no date")
	}
	t, err := parse.TimeAgo(now, strings.TrimSpace(dom.Text(n)))
	if err != nil {
		return time.Time{}, fmt.Errorf("libble: %w", err)
	}
	return t, nil
}

// parseCategories returns a group's categories by its category icon's
// class.
func parseCategories(group *html.Node) ([]int, error) {
	icon := categoryIcon.MatchFirst(group)
	if icon == nil {
		return nil, errors.New("libble: a group has no category icon")
	}
	cat := strings.TrimSpace(dom.Attr(icon, "class"))
	if m := categoryName.FindStringSubmatch(cat); m != nil {
		cat = strings.TrimSpace(m[1])
	}
	switch cat {
	case "music", "libblemixtapes":
		return []int{indexer.Audio.ID}, nil
	case "musicvideos":
		return []int{indexer.AudioVideo.ID}, nil
	}
	return []int{indexer.Other.ID}, nil
}
