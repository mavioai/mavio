package iptorrents

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
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
	"https://iptorrents.com/",
	"https://iptorrents.me/",
	"https://nemo.iptorrents.com/",
	"https://ipt.getcrazy.me/",
	"https://ipt.findnemo.net/",
	"https://ipt.beelyrics.net/",
	"https://ipt.venom.global/",
	"https://ipt.workisboring.net/",
	"https://ipt.lol/",
	"https://ipt.cool/",
	"https://ipt.world/",
	"https://ipt.octopus.town/",
}

// Sort is how searches are sorted.
type Sort int

// The sorts of searches.
const (
	// SortDefault is the site's: the newest first.
	SortDefault Sort = iota
	SortSeeders
	SortLeechers
	SortSnatches
	SortSize
	SortName
)

// sortParams are the values of the "o" parameter of the sorts.
var sortParams = map[Sort]string{
	SortSeeders:  "seeders",
	SortLeechers: "leechers",
	SortSnatches: "completed",
	SortSize:     "size",
	SortName:     "name",
}

// Definition is IPTorrents.
var Definition = indexer.Definition{
	ID:          "iptorrents",
	Name:        "IPTorrents",
	Description: "IPTorrents (IPT) is a Private Torrent Tracker for 0DAY / GENERAL.",
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
		{Name: "freeleech_only", Label: "Freeleech Only", Type: indexer.FieldCheckbox, HelpText: "Search freeleech torrents only"},
		{
			Name: "search_sort_by", Label: "Search Sort By", Type: indexer.FieldSelect,
			HelpText: "Sort order for search results; recent releases always use the site's default order",
			Default:  int(SortDefault),
			Options: []indexer.Option{
				{Value: int(SortDefault), Label: "Default"},
				{Value: int(SortSeeders), Label: "Seeders"},
				{Value: int(SortLeechers), Label: "Leechers"},
				{Value: int(SortSnatches), Label: "Snatches"},
				{Value: int(SortSize), Label: "Size"},
				{Value: int(SortName), Label: "Name"},
			},
		},
	},
	New: newSite,
}

// Settings are IPTorrents' settings.
type Settings struct {
	indexer.BaseSettings
	Cookie        string `json:"cookie,omitempty"`
	UserAgent     string `json:"user_agent,omitempty"`
	FreeleechOnly bool   `json:"freeleech_only,omitempty"`
	SearchSortBy  Sort   `json:"search_sort_by,omitempty"`
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

// Site is IPTorrents.
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
	c.Categories.Add("72", indexer.Movies, "Movies")
	c.Categories.Add("87", indexer.Movies3D, "Movie/3D")
	c.Categories.Add("77", indexer.MoviesSD, "Movie/480p")
	c.Categories.Add("101", indexer.MoviesUHD, "Movie/4K")
	c.Categories.Add("89", indexer.MoviesHD, "Movie/BD-R")
	c.Categories.Add("90", indexer.MoviesSD, "Movie/BD-Rip")
	c.Categories.Add("96", indexer.MoviesSD, "Movie/Cam")
	c.Categories.Add("6", indexer.MoviesDVD, "Movie/DVD-R")
	c.Categories.Add("48", indexer.MoviesBluRay, "Movie/HD/Bluray")
	c.Categories.Add("54", indexer.Movies, "Movie/Kids")
	c.Categories.Add("62", indexer.MoviesSD, "Movie/MP4")
	c.Categories.Add("38", indexer.MoviesForeign, "Movie/Non-English")
	c.Categories.Add("68", indexer.Movies, "Movie/Packs")
	c.Categories.Add("20", indexer.MoviesWEBDL, "Movie/Web-DL")
	c.Categories.Add("7", indexer.MoviesSD, "Movie/Xvid")
	c.Categories.Add("100", indexer.Movies, "Movie/x265")
	c.Categories.Add("73", indexer.TV, "TV")
	c.Categories.Add("26", indexer.TVDocumentary, "TV/Documentaries")
	c.Categories.Add("55", indexer.TVSport, "Sports")
	c.Categories.Add("78", indexer.TVSD, "TV/480p")
	c.Categories.Add("23", indexer.TVHD, "TV/BD")
	c.Categories.Add("24", indexer.TVSD, "TV/DVD-R")
	c.Categories.Add("25", indexer.TVSD, "TV/DVD-Rip")
	c.Categories.Add("66", indexer.TVSD, "TV/Mobile")
	c.Categories.Add("82", indexer.TVForeign, "TV/Non-English")
	c.Categories.Add("65", indexer.TV, "TV/Packs")
	c.Categories.Add("83", indexer.TVForeign, "TV/Packs/Non-English")
	c.Categories.Add("79", indexer.TVSD, "TV/SD/x264")
	c.Categories.Add("22", indexer.TVWEBDL, "TV/Web-DL")
	c.Categories.Add("5", indexer.TVHD, "TV/x264")
	c.Categories.Add("99", indexer.TVHD, "TV/x265")
	c.Categories.Add("4", indexer.TVSD, "TV/Xvid")
	c.Categories.Add("74", indexer.Console, "Games")
	c.Categories.Add("2", indexer.ConsoleOther, "Games/Mixed")
	c.Categories.Add("47", indexer.ConsoleNDS, "Games/Nintendo DS")
	c.Categories.Add("43", indexer.PCISO, "Games/PC-ISO")
	c.Categories.Add("45", indexer.PCGames, "Games/PC-Rip")
	c.Categories.Add("71", indexer.ConsolePS3, "Games/PS3")
	c.Categories.Add("50", indexer.ConsoleWii, "Games/Wii")
	c.Categories.Add("44", indexer.ConsoleXBox360, "Games/Xbox-360")
	c.Categories.Add("75", indexer.Audio, "Music")
	c.Categories.Add("3", indexer.AudioMP3, "Music/Audio")
	c.Categories.Add("80", indexer.AudioLossless, "Music/Flac")
	c.Categories.Add("93", indexer.Audio, "Music/Packs")
	c.Categories.Add("37", indexer.AudioVideo, "Music/Video")
	c.Categories.Add("21", indexer.AudioVideo, "Podcast")
	c.Categories.Add("76", indexer.Other, "Other/Miscellaneous")
	c.Categories.Add("60", indexer.TVAnime, "Anime")
	c.Categories.Add("1", indexer.PC0day, "Appz")
	c.Categories.Add("86", indexer.PC0day, "Appz/Non-English")
	c.Categories.Add("64", indexer.AudioAudiobook, "AudioBook")
	c.Categories.Add("35", indexer.Books, "Books")
	c.Categories.Add("102", indexer.Books, "Books/Non-English")
	c.Categories.Add("94", indexer.BooksComics, "Books/Comics")
	c.Categories.Add("95", indexer.BooksOther, "Books/Educational")
	c.Categories.Add("98", indexer.Other, "Other/Fonts")
	c.Categories.Add("69", indexer.PCMac, "Appz/Mac")
	c.Categories.Add("92", indexer.BooksMags, "Books/Magazines & Newspapers")
	c.Categories.Add("58", indexer.PCMobileOther, "Appz/Mobile")
	c.Categories.Add("36", indexer.Other, "Other/Pics/Wallpapers")
	c.Categories.Add("88", indexer.XXX, "XXX")
	c.Categories.Add("85", indexer.XXXOther, "XXX/Magazines")
	c.Categories.Add("8", indexer.XXX, "XXX/Movie")
	c.Categories.Add("81", indexer.XXX, "XXX/Movie/0Day")
	c.Categories.Add("91", indexer.XXXPack, "XXX/Packs")
	c.Categories.Add("84", indexer.XXXImageSet, "XXX/Pics/Wallpapers")
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

// SearchRequests implements indexer.Site: one search of the site's "t"
// page.
func (s *Site) SearchRequests(_ context.Context, _ *indexer.Indexer, q *indexer.Query) (*indexer.Chain, error) {
	var term, imdb string
	switch q.Type {
	case indexer.SearchMovie:
		term, imdb = q.Term, q.FullIMDbID()
	case indexer.SearchTV:
		term, imdb = q.SanitizedTVString(), q.FullIMDbID()
		if q.Season != nil && *q.Season > 0 && strings.TrimSpace(q.Episode) == "" {
			term += "*"
		}
		term = strings.TrimSpace(term)
	default:
		term = q.SanitizedTerm()
	}
	var parts []string
	var cats []string
	for _, c := range s.caps.Categories.MapToTracker(q.Categories, false) {
		if !slices.Contains(cats, c) {
			cats = append(cats, c)
			parts = append(parts, c+"=")
		}
	}
	if s.settings.FreeleechOnly {
		parts = append(parts, "free=on")
	}
	if o, ok := sortParams[s.settings.SearchSortBy]; ok && !q.IsRSS() {
		parts = append(parts, "o="+o)
	}
	if strings.TrimSpace(imdb) != "" {
		// The site searches with Sphinx, which takes boolean operators and
		// groups; qf=all searches the descriptions too.
		parts = append(parts, "q="+indexer.URLEncode("+("+imdb+")"), "qf=all")
	}
	if strings.TrimSpace(term) != "" {
		parts = append(parts, "q="+indexer.URLEncode("+("+term+")"))
	}
	if q.Limit > 0 && q.Offset > 0 {
		parts = append(parts, "p="+strconv.Itoa(q.Offset/q.Limit+1))
	}
	u := s.baseURL + "t"
	if len(parts) > 0 {
		u += "?" + strings.Join(parts, "&")
	}
	r := indexer.NewRequest(u, indexer.AcceptHTML)
	r.Cookies = indexer.ParseCookieHeader(s.settings.Cookie)
	r.Header.Set("User-Agent", s.settings.UserAgent)
	var c indexer.Chain
	c.AddRequests(r)
	return &c, nil
}

var (
	headerCells = cascadia.MustCompile(`table[id="torrents"] > thead > tr > th`)
	rows        = cascadia.MustCompile(`table[id="torrents"] > tbody > tr`)
	titleLink   = cascadia.MustCompile(`a.hv`)
	downloadLnk = cascadia.MustCompile(`a[href^="/download.php/"]`)
	sub         = cascadia.MustCompile(`div.sub`)
	categoryLnk = cascadia.MustCompile(`td:nth-of-type(1) a[href^="?"]`)
	free        = cascadia.MustCompile(`span.free`)
)

// ParseResponse implements indexer.Site, as Prowlarr's IPTorrentsParser
// does. Pages without the logout link are those of a failed login.
func (s *Site) ParseResponse(_ context.Context, _ *indexer.Indexer, r *indexer.Response) ([]*indexer.Release, error) {
	content := r.Content()
	if !strings.Contains(content, "lout.php") {
		return nil, fmt.Errorf("IPTorrents authentication with cookies failed: %w", indexer.ErrAuth)
	}
	doc, err := dom.Parse(content)
	if err != nil {
		return nil, err
	}
	var header []string
	for _, th := range headerCells.MatchAll(doc) {
		header = append(header, strings.TrimSpace(dom.Text(th)))
	}
	sizeIndex := columnIndex(header, "Sort by size", 5)
	filesIndex := columnIndex(header, "Sort by files", -1)
	now := time.Now()
	var releases []*indexer.Release
	for _, row := range rows.MatchAll(doc) {
		link := titleLink.MatchFirst(row)
		if link == nil {
			continue // no results
		}
		rel, err := s.release(now, row, link, header, sizeIndex, filesIndex)
		if err != nil {
			return nil, err
		}
		releases = append(releases, rel)
	}
	return releases, nil
}

func (s *Site) release(now time.Time, row, link *html.Node, header []string, sizeIndex, filesIndex int) (*indexer.Release, error) {
	details := s.baseURL + strings.TrimLeft(dom.Attr(link, "href"), "/")
	dl := downloadLnk.MatchFirst(row)
	if dl == nil {
		return nil, errors.New("iptorrents: a release has no download link")
	}
	descr := strings.Split(dom.Text(sub.MatchFirst(row)), "|")
	when, _, _ := strings.Cut(descr[len(descr)-1], " by ")
	date, err := parse.TimeAgo(now, when)
	if err != nil {
		return nil, fmt.Errorf("iptorrents: %w", err)
	}
	var description string
	if len(descr) > 1 {
		description = "Tags: " + strings.TrimSpace(descr[0])
	}
	icon := categoryLnk.MatchFirst(row)
	if icon == nil {
		return nil, indexer.Errorf("Please, change the 'Torrents - Category column' option to 'Icons' in the website Settings. Wait a minute (cache) and then try again.")
	}
	cells := dom.Children(row)
	cell := func(i int) (string, error) {
		if i < 0 || i >= len(cells) {
			return "", fmt.Errorf("iptorrents: a row has no column %d", i)
		}
		return dom.Text(cells[i]), nil
	}
	sizeText, err := cell(sizeIndex)
	if err != nil {
		return nil, err
	}
	size, err := parse.Bytes(sizeText)
	if err != nil {
		return nil, fmt.Errorf("iptorrents: %w", err)
	}
	rel := &indexer.Release{
		GUID:                 details,
		InfoURL:              details,
		DownloadURL:          s.baseURL + strings.TrimLeft(dom.Attr(dl, "href"), "/"),
		Title:                cleanTitle(dom.Text(link)),
		Description:          description,
		Protocol:             indexer.ProtocolTorrent,
		Categories:           indexer.IDs(s.caps.Categories.MapTracker(strings.TrimPrefix(dom.Attr(icon, "href"), "?"))),
		Size:                 &size,
		PublishDate:          date,
		DownloadVolumeFactor: indexer.Ptr(1.0),
		UploadVolumeFactor:   indexer.Ptr(1.0),
		MinimumRatio:         indexer.Ptr(1.0),
		// 336 hours.
		MinimumSeedTime: indexer.Ptr(int64(1209600)),
	}
	if free.MatchFirst(row) != nil {
		rel.DownloadVolumeFactor = indexer.Ptr(0.0)
	}
	if filesIndex != -1 {
		text, err := cell(filesIndex)
		if err != nil {
			return nil, err
		}
		n, err := parse.Int(strings.ReplaceAll(text, "Go to files", ""))
		if err != nil {
			return nil, fmt.Errorf("iptorrents: files: %w", err)
		}
		rel.Files = indexer.Ptr(int(n))
	}
	col := 6
	if len(cells) == 10 {
		col = 7
	}
	var counts [3]int
	for i, name := range []string{"Sort by snatches", "Sort by seeders", "Sort by leechers"} {
		text, err := cell(columnIndex(header, name, col+i))
		if err != nil {
			return nil, err
		}
		n, err := parse.Int(text)
		if err != nil {
			return nil, fmt.Errorf("iptorrents: %s: %w", strings.TrimPrefix(name, "Sort by "), err)
		}
		counts[i] = int(n)
	}
	rel.Grabs = &counts[0]
	rel.Seeders = &counts[1]
	rel.Peers = indexer.Ptr(counts[1] + counts[2])
	return rel, nil
}

// columnIndex returns the index of a column by its header; def when
// there is none.
func columnIndex(header []string, name string, def int) int {
	if i := slices.Index(header, name); i != -1 {
		return i
	}
	return def
}

var (
	// invalidChars are those cropping up in some titles.
	invalidChars = regexp.MustCompile(`[\x{0}-\x{8}\x{A}-\x{1F}\x{100}-\x{10FFFF}]`)
	requested    = regexp.MustCompile(`(?i)[\(\[\{]REQ(UEST(ED)?)?[\)\]\}]`)
	// bracketedLanguage is a language between brackets before a title,
	// which conflicts with parsing anime release groups.
	bracketedLanguage = regexp.MustCompile(`(?i)^\[[a-z0-9 ._-]+\][-._ ](.*-[a-z0-9]+)$`)
)

// cleanTitle cleans up a title as Prowlarr does.
func cleanTitle(title string) string {
	title = invalidChars.ReplaceAllString(title, "")
	title = requested.ReplaceAllString(title, "")
	title = bracketedLanguage.ReplaceAllString(title, "$1")
	return strings.Trim(title, " -:")
}

// CleanupReleases implements indexer.ReleaseCleaner: the site searches
// loosely, so releases not matching the query are dropped.
func (s *Site) CleanupReleases(releases []*indexer.Release, q *indexer.Query) []*indexer.Release {
	return indexer.FilterByQuery(releases, q)
}
