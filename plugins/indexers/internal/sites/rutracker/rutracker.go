package rutracker

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/andybalholm/cascadia"
	"golang.org/x/net/html"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"

	"github.com/mavioai/mavio/plugins/indexers/internal/dom"
	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
	"github.com/mavioai/mavio/plugins/indexers/internal/parse"
)

var links = []string{"https://rutracker.org/", "https://rutracker.net/", "https://rutracker.nl/"}

// Definition is RuTracker.org.
var Definition = indexer.Definition{
	ID:          "rutracker",
	Name:        "RuTracker.org",
	Description: "RuTracker.org is a RUSSIAN Semi-Private site with a thriving file-sharing community",
	Language:    "ru-RU",
	Protocol:    indexer.ProtocolTorrent,
	Privacy:     indexer.SemiPrivate,
	Links:       links,
	Fields: []indexer.Field{
		indexer.BaseURLField(links),
		indexer.UsernameField,
		indexer.PasswordField,
		{Name: "russian_letters", Label: "Strip Russian letters", Type: indexer.FieldCheckbox, HelpText: "Removes russian letters"},
		{Name: "use_magnet_links", Label: "Use Magnet Links", Type: indexer.FieldCheckbox, HelpText: "When enabled this option will disable torrent links"},
		{
			Name: "add_russian_to_title", Label: "Add RUS to titles", Type: indexer.FieldCheckbox,
			HelpText: "Add RUS to end of all titles to improve language detection by Sonarr and Radarr. Will cause English-only results to be misidentified.",
		},
		{Name: "move_first_tags_to_end_of_release_title", Label: "Move first tags to end of release title", Type: indexer.FieldCheckbox},
		{Name: "move_all_tags_to_end_of_release_title", Label: "Move all tags to end of release title", Type: indexer.FieldCheckbox},
	},
	New: newSite,
}

// Settings are RuTracker's settings.
type Settings struct {
	indexer.UserPassSettings
	// RussianLetters strips Cyrillic letters from titles.
	RussianLetters bool `json:"russian_letters,omitempty"`
	// UseMagnetLinks downloads releases by the magnet links of their pages
	// rather than their torrent files.
	UseMagnetLinks bool `json:"use_magnet_links,omitempty"`
	// AddRussianToTitle appends "RUS" to the titles of movies and TV.
	AddRussianToTitle bool `json:"add_russian_to_title,omitempty"`
	// MoveFirstTagsToEndOfReleaseTitle moves the bracketed tags a title
	// starts with to its end.
	MoveFirstTagsToEndOfReleaseTitle bool `json:"move_first_tags_to_end_of_release_title,omitempty"`
	// MoveAllTagsToEndOfReleaseTitle moves all of a title's bracketed tags
	// to its end.
	MoveAllTagsToEndOfReleaseTitle bool `json:"move_all_tags_to_end_of_release_title,omitempty"`
}

// Site is RuTracker.org.
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
	c.RawSearch = true
	addCategories(&c.Categories)
	return c
}

// Info implements indexer.Site.
func (s *Site) Info() indexer.Info {
	return indexer.Info{
		Protocol:       indexer.ProtocolTorrent,
		Privacy:        indexer.SemiPrivate,
		SupportsRSS:    true,
		SupportsSearch: true,
		Encoding:       charmap.Windows1251,
	}
}

// Capabilities implements indexer.Site.
func (s *Site) Capabilities(context.Context, *indexer.Indexer) (*indexer.Capabilities, error) {
	return s.caps, nil
}

// maxCategories is the most forums RuTracker searches at once.
const maxCategories = 200

// SearchRequests implements indexer.Site, as Prowlarr's
// RuTrackerRequestGenerator does: a search of tracker.php per 200 of the
// forums asked for.
func (s *Site) SearchRequests(_ context.Context, _ *indexer.Indexer, q *indexer.Query) (*indexer.Chain, error) {
	season := 0
	if q.Type == indexer.SearchTV && q.Season != nil {
		season = *q.Season
	}
	var c indexer.Chain
	if len(q.Categories) == 0 {
		c.AddRequests(s.searchRequest(q.Term, nil, season))
		return &c, nil
	}
	for chunk := range slices.Chunk(s.caps.Categories.MapToTracker(q.Categories, false), maxCategories) {
		c.AddRequests(s.searchRequest(q.Term, chunk, season))
	}
	return &c, nil
}

// wildcards are the runs of characters a search term has besides Latin
// and Russian letters and digits.
var wildcards = regexp.MustCompile(`[^a-zA-Zа-яА-ЯёЁ0-9]+`)

func (s *Site) searchRequest(term string, forums []string, season int) *indexer.Request {
	var p indexer.Params
	if strings.TrimSpace(term) == "" {
		// A blank term lists the newest releases.
		p.Set("nm", term)
	} else {
		term = strings.ReplaceAll(wildcards.ReplaceAllString(term, "%"), "-", " ")
		if season != 0 {
			term += " ТВ | Сезон: " + strconv.Itoa(season)
		}
		p.Set("nm", term)
	}
	if len(forums) > 0 {
		p.Set("f", strings.Join(forums, ","))
	}
	return indexer.NewRequest(s.baseURL+"forum/tracker.php?"+p.Encode(), indexer.AcceptHTML)
}

// loggedIn is on the pages of logged in users.
const loggedIn = `id="logged-in-username"`

// refresh reads the URL of a Refresh header, as Prowlarr does.
var refresh = regexp.MustCompile(`^(.*?url)=(.*?)(?:;|$)`)

// LoginNeeded implements indexer.LoginSite: the response redirects to the
// login page or is not a logged in user's page.
func (s *Site) LoginNeeded(r *indexer.Response) bool {
	redirect := r.Header.Get("Location")
	if redirect == "" {
		if m := refresh.FindStringSubmatch(r.Header.Get("Refresh")); m != nil {
			redirect = m[2]
		}
	}
	return strings.Contains(redirect, "login.php") || !strings.Contains(r.Content(), loggedIn)
}

var (
	loginError = cascadia.MustCompile("h4.warnColor1.tCenter.mrg_16, div.msg-main")
	captcha    = cascadia.MustCompile(`input[name="cap_sid"], input[name^="cap_code_"]`)
)

// Login implements indexer.LoginSite: it posts the credentials, encoded
// in Windows-1251, to login.php and keeps the cookies for 30 days.
func (s *Site) Login(ctx context.Context, ix *indexer.Indexer) error {
	loginURL := s.baseURL + "forum/login.php"
	r := indexer.NewRequest(loginURL, "")
	r.Method = http.MethodPost
	r.FollowRedirects = true
	body, err := windows1251Form([][2]string{
		{"login_username", s.settings.Username},
		{"login_password", s.settings.Password},
		{"login", "Login"},
		{"redirect", "index.php"},
	})
	if err != nil {
		return fmt.Errorf("rutracker login: %w", err)
	}
	r.Body = body
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Referer", loginURL)
	resp, err := ix.Do(ctx, r)
	if err != nil {
		return fmt.Errorf("rutracker login: %w", err)
	}
	if content := resp.Content(); !strings.Contains(content, loggedIn) {
		msg := "RuTracker Auth Failed"
		if doc, err := dom.Parse(content); err == nil {
			if captcha.MatchFirst(doc) != nil {
				return fmt.Errorf("%w: RuTracker asks for a captcha, which cannot be solved here; log in on the site with a browser, then try again", indexer.ErrAuth)
			}
			if n := loginError.MatchFirst(doc); n != nil {
				msg = strings.TrimSpace(dom.Text(n))
			}
		}
		return fmt.Errorf("%w: %s", indexer.ErrAuth, msg)
	}
	ix.SetCookies(maps.Clone(resp.Cookies), time.Now().AddDate(0, 0, 30))
	return nil
}

// windows1251Form encodes a form as Prowlarr does with the Windows-1251
// encoding: keys as they are, values encoded in Windows-1251, characters
// it lacks as '?', then URL encoded.
func windows1251Form(fields [][2]string) ([]byte, error) {
	enc := encoding.ReplaceUnsupported(charmap.Windows1251.NewEncoder())
	parts := make([]string, len(fields))
	for i, f := range fields {
		v, err := enc.String(f[1])
		if err != nil {
			return nil, fmt.Errorf("encode %s: %w", f[0], err)
		}
		parts[i] = f[0] + "=" + indexer.URLEncode(v)
	}
	return []byte(strings.Join(parts, "&")), nil
}

var magnetLink = cascadia.MustCompile(`table.attach a.magnet-link[href^="magnet:?"]`)

// DownloadRequest implements indexer.DownloadSite: with magnet links,
// releases are downloaded by the magnet links of their pages.
func (s *Site) DownloadRequest(ctx context.Context, ix *indexer.Indexer, link string) (*indexer.Request, error) {
	if !s.settings.UseMagnetLinks || !strings.Contains(link, "viewtopic.php?t=") {
		return indexer.NewRequest(link, ""), nil
	}
	r := indexer.NewRequest(link, indexer.AcceptHTML)
	r.Cookies = ix.Cookies()
	if r.Cookies == nil {
		r.Cookies = map[string]string{}
	}
	resp, err := ix.Do(ctx, r)
	if err != nil {
		return nil, fmt.Errorf("rutracker: fetch %s: %w", link, err)
	}
	doc, err := dom.Parse(resp.Content())
	if err != nil {
		return nil, fmt.Errorf("rutracker: parse %s: %w", link, err)
	}
	var magnet string
	if n := magnetLink.MatchFirst(doc); n != nil {
		magnet = dom.Attr(n, "href")
	}
	if magnet == "" {
		return nil, fmt.Errorf("Failed to fetch magnet link from %s", link) //nolint:staticcheck // Prowlarr's message.
	}
	return indexer.NewRequest(magnet, ""), nil
}

var (
	rowsSel     = cascadia.MustCompile("table#tor-tbl > tbody > tr")
	downloadSel = cascadia.MustCompile("td.tor-size > a.tr-dl")
	detailsSel  = cascadia.MustCompile("td.t-title-col > div.t-title > a.tLink")
	forumSel    = cascadia.MustCompile("td.f-name-col > div.f-name > a")
	sizeSel     = cascadia.MustCompile("td.tor-size")
	seedersSel  = cascadia.MustCompile("td:nth-child(7)")
	boldSel     = cascadia.MustCompile("b")
	leechersSel = cascadia.MustCompile("td:nth-child(8)")
	grabsSel    = cascadia.MustCompile("td:nth-child(9)")
	dateSel     = cascadia.MustCompile("td:nth-child(10)")
)

// errMissing is a row lacking a cell Prowlarr reads.
var errMissing = errors.New("missing")

// ParseResponse implements indexer.Site, as Prowlarr's RuTrackerParser
// does: the rows of the tracker's table, newest first.
func (s *Site) ParseResponse(_ context.Context, _ *indexer.Indexer, r *indexer.Response) ([]*indexer.Release, error) {
	doc, err := dom.Parse(r.Content())
	if err != nil {
		return nil, fmt.Errorf("rutracker: parse page: %w", err)
	}
	var releases []*indexer.Release
	for _, row := range rowsSel.MatchAll(doc) {
		rel, err := s.parseRow(row)
		if err != nil {
			return nil, fmt.Errorf("rutracker: parse row: %w", err)
		}
		if rel != nil {
			releases = append(releases, rel)
		}
	}
	slices.SortStableFunc(releases, func(a, b *indexer.Release) int { return b.PublishDate.Compare(a.PublishDate) })
	return releases, nil
}

func (s *Site) parseRow(row *html.Node) (*indexer.Release, error) {
	dl := downloadSel.MatchFirst(row)
	if dl == nil {
		// The release awaits moderation.
		return nil, nil
	}
	details := detailsSel.MatchFirst(row)
	if details == nil {
		return nil, fmt.Errorf("details link: %w", errMissing)
	}
	infoURL := s.baseURL + "forum/" + dom.Attr(details, "href")
	downloadURL := s.baseURL + "forum/" + dom.Attr(dl, "href")
	if s.settings.UseMagnetLinks {
		downloadURL = infoURL
	}
	title := strings.TrimSpace(dom.Text(details))
	var forum string
	if n := forumSel.MatchFirst(row); n != nil {
		forum = dom.Attr(n, "href")
	}
	cats := indexer.IDs(s.caps.Categories.MapTracker(parse.QueryArg(forum, "f")))

	sizeCell := sizeSel.MatchFirst(row)
	if sizeCell == nil {
		return nil, fmt.Errorf("size: %w", errMissing)
	}
	size, err := parse.Bytes(dom.Attr(sizeCell, "data-ts_text"))
	if err != nil {
		return nil, fmt.Errorf("size: %w", err)
	}
	seeders, err := rowSeeders(row)
	if err != nil {
		return nil, err
	}
	leechers, err := cellInt(row, leechersSel, "leechers")
	if err != nil {
		return nil, err
	}
	grabs, err := cellInt(row, grabsSel, "grabs")
	if err != nil {
		return nil, err
	}
	dateCell := dateSel.MatchFirst(row)
	if dateCell == nil {
		return nil, fmt.Errorf("date: %w", errMissing)
	}
	unix, err := strconv.ParseInt(dom.Attr(dateCell, "data-ts_text"), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("date: %w", err)
	}
	return &indexer.Release{
		GUID:        infoURL,
		InfoURL:     infoURL,
		DownloadURL: downloadURL,
		Title: ParseTitle(title, cats, TitleOptions{
			StripCyrillic:     s.settings.RussianLetters,
			MoveFirstTags:     s.settings.MoveFirstTagsToEndOfReleaseTitle,
			MoveAllTags:       s.settings.MoveAllTagsToEndOfReleaseTitle,
			AddRussianToTitle: s.settings.AddRussianToTitle,
		}),
		Description:          title,
		Categories:           cats,
		Size:                 indexer.Ptr(size),
		Seeders:              indexer.Ptr(seeders),
		Peers:                indexer.Ptr(leechers + seeders),
		Grabs:                indexer.Ptr(grabs),
		PublishDate:          time.Unix(unix, 0).UTC(),
		DownloadVolumeFactor: indexer.Ptr(1.0),
		UploadVolumeFactor:   indexer.Ptr(1.0),
		MinimumRatio:         indexer.Ptr(1.0),
		MinimumSeedTime:      indexer.Ptr(int64(0)),
	}, nil
}

// rowSeeders reads the seeders of a row; 0 when the cell says how many
// days ago a seeder was last seen.
func rowSeeders(row *html.Node) (int, error) {
	cell := seedersSel.MatchFirst(row)
	if cell == nil || strings.Contains(dom.Text(cell), "дн") {
		return 0, nil
	}
	b := boldSel.MatchFirst(cell)
	if b == nil {
		return 0, fmt.Errorf("seeders: %w", errMissing)
	}
	text := dom.Text(b)
	if strings.TrimSpace(text) == "" {
		return 0, nil
	}
	n, err := parse.Int(text)
	if err != nil {
		return 0, fmt.Errorf("seeders: %w", err)
	}
	return int(n), nil
}

func cellInt(row *html.Node, sel cascadia.Selector, name string) (int, error) {
	cell := sel.MatchFirst(row)
	if cell == nil {
		return 0, fmt.Errorf("%s: %w", name, errMissing)
	}
	n, err := parse.Int(dom.Text(cell))
	if err != nil {
		return 0, fmt.Errorf("%s: %w", name, err)
	}
	return int(n), nil
}
