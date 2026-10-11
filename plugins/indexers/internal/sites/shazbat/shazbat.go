package shazbat

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/andybalholm/cascadia"
	"golang.org/x/net/html"
	"golang.org/x/text/encoding/unicode"

	"github.com/mavioai/mavio/plugins/indexers/internal/dom"
	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
)

var (
	links  = []string{"https://www.shazbat.tube/"}
	legacy = []string{"https://www.shazbat.tv/"}
)

// Definition is Shazbat.
var Definition = indexer.Definition{
	ID:          "shazbat",
	Name:        "Shazbat",
	Description: "Shazbat is a PRIVATE Torrent Tracker with highly curated TV content",
	Language:    "en-US",
	Protocol:    indexer.ProtocolTorrent,
	Privacy:     indexer.Private,
	Links:       links,
	LegacyLinks: legacy,
	Fields: []indexer.Field{
		indexer.BaseURLField(links),
		indexer.UsernameField,
		indexer.PasswordField,
		{Name: "show_pages_fetch_limit", Label: "Show Pages Fetch Limit", Type: indexer.FieldNumber, HelpText: "The number of show pages should Prowlarr fetch when searching. Default: 2."},
	},
	New: newSite,
}

// Settings are Shazbat's settings.
type Settings struct {
	indexer.UserPassSettings
	// ShowPagesFetchLimit is how many show pages a search fetches; 2 when
	// nil.
	ShowPagesFetchLimit *int `json:"show_pages_fetch_limit,omitempty"`
}

// Validate checks the credentials, and that the show pages fetch limit is
// between 1 and 5 when set.
func (s Settings) Validate() error {
	if err := s.UserPassSettings.Validate(); err != nil {
		return err
	}
	if l := s.ShowPagesFetchLimit; l != nil {
		if *l <= 0 {
			return errors.New("show_pages_fetch_limit: Should be greater than zero")
		}
		if *l > 5 {
			return errors.New("show_pages_fetch_limit: Should be less than or equal to 5")
		}
	}
	return nil
}

// Site is Shazbat.
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
	return &Site{settings: settings, baseURL: settings.URL(links, legacy), caps: capabilities()}, nil
}

func capabilities() *indexer.Capabilities {
	c := indexer.NewCapabilities()
	c.Params[indexer.SearchTV] = []indexer.Param{indexer.ParamQ, indexer.ParamSeason, indexer.ParamEp}
	c.Flags = []indexer.Flag{indexer.FlagScene}
	c.Categories.Add("1", indexer.TV, "")
	c.Categories.Add("2", indexer.TVSD, "")
	c.Categories.Add("3", indexer.TVHD, "")
	c.Categories.Add("4", indexer.TVUHD, "")
	return c
}

// Info implements indexer.Site.
func (s *Site) Info() indexer.Info {
	return indexer.Info{
		Protocol:       indexer.ProtocolTorrent,
		Privacy:        indexer.Private,
		SupportsRSS:    true,
		SupportsSearch: true,
		RateLimit:      5100 * time.Millisecond,
		Encoding:       unicode.UTF8,
	}
}

// Capabilities implements indexer.Site.
func (s *Site) Capabilities(context.Context, *indexer.Indexer) (*indexer.Capabilities, error) {
	return s.caps, nil
}

// LoginNeeded implements indexer.LoginSite, as Prowlarr's
// CheckIfLoginNeeded does: redirects to the login page, and pages asking
// to sign in.
func (s *Site) LoginNeeded(r *indexer.Response) bool {
	if r.IsRedirect() && strings.Contains(strings.ToLower(r.RedirectURL()), "login") {
		return true
	}
	content := strings.ToLower(r.Content())
	return strings.Contains(content, "sign in now") ||
		(strings.Contains(content, "fullredirect") && strings.Contains(content, "login"))
}

var loginError = cascadia.MustCompile(`div#fail .modal-body`)

// Login implements indexer.LoginSite: it posts the credentials to the
// login page, keeping the cookies for 30 days.
func (s *Site) Login(ctx context.Context, ix *indexer.Indexer) error {
	loginURL := s.baseURL + "login"
	var form indexer.Params
	form.Set("referer", "")
	form.Set("query", "")
	form.Set("tv_timezone", "0")
	form.Set("username", s.settings.Username)
	form.Set("password", s.settings.Password)
	r := indexer.NewRequest(loginURL, "")
	r.Method = http.MethodPost
	r.FollowRedirects = true
	r.Body = []byte(form.Encode())
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Referer", loginURL)
	resp, err := ix.Do(ctx, r)
	if err != nil {
		return fmt.Errorf("shazbat login: %w", err)
	}
	if resp.HasError() {
		return fmt.Errorf("shazbat login: %w", &indexer.HTTPError{Response: resp})
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
	ix.SetCookies(responseCookies(resp), time.Time{})
	slog.DebugContext(ctx, "authentication succeeded")
	return nil
}

// responseCookies returns a response's cookies as Prowlarr's GetCookies
// does: the request's, updated by the response's.
func responseCookies(r *indexer.Response) map[string]string {
	c := map[string]string{}
	if r.Request != nil {
		maps.Copy(c, r.Request.Cookies)
	}
	maps.Copy(c, r.Cookies)
	return c
}

var (
	episodeRe = regexp.MustCompile(`(?i)\b[S|E]\d+\b`)
	yearRe    = regexp.MustCompile(`(.+)\b\d{4}(\.\d{2}\.\d{2})?\b`)
	spacesRe  = regexp.MustCompile(`[\.\s\(\)\[\]]+`)
)

// fixSearchTerm drops seasons, episodes, years and dates from a term,
// which the site searches shows by.
func fixSearchTerm(term string) string {
	term = episodeRe.ReplaceAllString(term, "")
	term = yearRe.ReplaceAllString(term, "${1}")
	term = spacesRe.ReplaceAllString(term, " ")
	return strings.TrimSpace(strings.ToLower(term))
}

// SearchRequests implements indexer.Site, as Prowlarr's
// ShazbatRequestGenerator does: the search of shows by name, else the
// latest torrents; only TV and basic searches.
func (s *Site) SearchRequests(_ context.Context, _ *indexer.Indexer, q *indexer.Query) (*indexer.Chain, error) {
	var c indexer.Chain
	if q.Type != indexer.SearchTV && q.Type != indexer.SearchBasic {
		return &c, nil
	}
	var r *indexer.Request
	if term := fixSearchTerm(q.SanitizedTerm()); strings.TrimSpace(term) != "" {
		u := indexer.AddQueryParam(s.baseURL+"search", "search", term)
		u = indexer.AddQueryParam(u, "portlet", "true")
		r = indexer.NewRequest(u, indexer.AcceptHTML)
		r.Header.Set("X-Requested-With", "XMLHttpRequest")
	} else {
		r = indexer.NewRequest(s.baseURL+"torrents", indexer.AcceptHTML)
	}
	r.Header.Set("Referer", s.baseURL)
	c.AddRequests(r)
	return &c, nil
}

var (
	globalFreeleechSel = cascadia.MustCompile(`span:contains("Freeleech until:"):has(span.datetime)`)
	showSel            = cascadia.MustCompile(`div.show[data-id]`)
	rowSel             = cascadia.MustCompile(`#torrent-table tr.eprow, table tr.eprow`)
	downloadSel        = cascadia.MustCompile(`td:nth-of-type(5) a[href^="load_torrent?"]`)
	infoSel            = cascadia.MustCompile(`td:nth-of-type(5) [href^="torrent_info?"]`)
	titleSel           = cascadia.MustCompile(`td:nth-of-type(3)`)
	infoStringSel      = cascadia.MustCompile(`td:nth-of-type(4)`)
	dateSel            = cascadia.MustCompile(`.datetime[data-timestamp]`)
	tagSel             = cascadia.MustCompile(`label.label-tag`)
	torrentInfoRe      = regexp.MustCompile(`\((?P<size>\d+)\)\s*:(?P<seeders>\d+) / :(?P<leechers>\d+)$`)
)

// ParseResponse implements indexer.Site, as Prowlarr's ShazbatParser
// does: the torrents of the page, then those of the shows it found, a
// page of torrents each.
func (s *Site) ParseResponse(ctx context.Context, ix *indexer.Indexer, r *indexer.Response) ([]*indexer.Release, error) {
	doc, err := dom.Parse(r.Content())
	if err != nil {
		return nil, fmt.Errorf("shazbat: %w", err)
	}
	globalFreeleech := globalFreeleechSel.MatchFirst(doc) != nil
	releases, err := s.parseResults(doc, globalFreeleech)
	if err != nil {
		return nil, err
	}
	shows := showSel.MatchAll(doc)
	if len(shows) == 0 {
		return releases, nil
	}
	limit := 2
	if s.settings.ShowPagesFetchLimit != nil {
		limit = *s.settings.ShowPagesFetchLimit
	}
	if limit < 1 || limit > 5 {
		return nil, indexer.Errorf("Value for Show Pages Fetch Limit should be between 1 and 5. Current value: %d.", limit)
	}
	if len(shows) > limit {
		slog.DebugContext(ctx, fmt.Sprintf("Your search returned %d shows. Use a more specific search term for more relevant results.", len(shows)))
	}
	cookies := responseCookies(r)
	if len(cookies) == 0 {
		return nil, indexer.Errorf("Invalid cookies. Most likely your session expired or was killed.")
	}
	for _, show := range shows[:min(limit, len(shows))] {
		page, err := s.showPage(ctx, ix, dom.Attr(show, "data-id"), cookies, globalFreeleech)
		if err != nil {
			return nil, err
		}
		releases = append(releases, page...)
	}
	return releases, nil
}

// showPage returns the releases of a show's page of torrents.
func (s *Site) showPage(ctx context.Context, ix *indexer.Indexer, id string, cookies map[string]string, globalFreeleech bool) ([]*indexer.Release, error) {
	u := indexer.AddQueryParam(s.baseURL+"show", "id", id)
	referer := u
	for _, p := range [][2]string{{"show_mode", "torrents"}, {"portlet", "true"}, {"tab", "true"}} {
		u = indexer.AddQueryParam(u, p[0], p[1])
	}
	r := indexer.NewRequest(u, indexer.AcceptHTML)
	r.Cookies = maps.Clone(cookies)
	r.Header.Set("X-Requested-With", "XMLHttpRequest")
	r.Header.Set("Referer", referer)
	r.RateLimit = s.Info().RateLimit
	slog.DebugContext(ctx, "downloading feed", "url", u)
	resp, err := ix.Do(ctx, r)
	if err != nil {
		return nil, fmt.Errorf("shazbat: %w", err)
	}
	if (resp.IsRedirect() && strings.Contains(strings.ToLower(resp.RedirectURL()), "login")) ||
		strings.Contains(strings.ToLower(resp.Content()), "sign in now") {
		ix.SetCookies(nil, time.Time{})
		return nil, fmt.Errorf("%w: %s", indexer.ErrAuth, "We are being redirected to the Shazbat login page. Most likely your session expired or was killed.")
	}
	if resp.HasError() {
		if resp.StatusCode == http.StatusTooManyRequests {
			return nil, &indexer.RateLimitError{}
		}
		return nil, indexer.Errorf("HTTP Error - %d. %s", resp.StatusCode, u)
	}
	doc, err := dom.Parse(resp.Content())
	if err != nil {
		return nil, fmt.Errorf("shazbat: %w", err)
	}
	return s.parseResults(doc, globalFreeleech)
}

// parseResults returns the releases of a page's rows. Rows without a
// date are taken as a minute older than the row before.
func (s *Site) parseResults(doc *html.Node, globalFreeleech bool) ([]*indexer.Release, error) {
	if !globalFreeleech {
		globalFreeleech = globalFreeleechSel.MatchFirst(doc) != nil
	}
	factor := 1.0
	if globalFreeleech {
		factor = 0
	}
	date := time.Now().UTC()
	rows := rowSel.MatchAll(doc)
	releases := make([]*indexer.Release, 0, len(rows))
	for _, row := range rows {
		title, err := parseTitle(titleSel.MatchFirst(row))
		if err != nil {
			return nil, err
		}
		info := s.baseURL + attr(infoSel.MatchFirst(row), "href")
		var size int64
		var seeders, leechers int
		if n := infoStringSel.MatchFirst(row); n != nil {
			if m := torrentInfoRe.FindStringSubmatch(strings.TrimSpace(dom.Text(n))); m != nil {
				size, _ = strconv.ParseInt(m[1], 10, 64)
				seeders, _ = strconv.Atoi(m[2])
				leechers, _ = strconv.Atoi(m[3])
			}
		}
		if ts, err := strconv.ParseInt(strings.TrimSpace(attr(dateSel.MatchFirst(row), "data-timestamp")), 10, 64); err == nil {
			date = time.Unix(ts, 0).UTC()
		} else {
			date = date.Add(-time.Minute)
		}
		genres := []string{}
		for _, t := range tagSel.MatchAll(row) {
			genres = append(genres, strings.TrimSpace(dom.Text(t)))
		}
		releases = append(releases, &indexer.Release{
			GUID:                 info,
			InfoURL:              info,
			DownloadURL:          s.baseURL + attr(downloadSel.MatchFirst(row), "href"),
			Title:                title,
			Categories:           categories(title),
			Size:                 indexer.Ptr(size),
			Seeders:              indexer.Ptr(seeders),
			Peers:                indexer.Ptr(seeders + leechers),
			PublishDate:          date,
			Scene:                true,
			Genres:               genres,
			DownloadVolumeFactor: indexer.Ptr(factor),
			UploadVolumeFactor:   indexer.Ptr(1.0),
			MinimumRatio:         indexer.Ptr(1.0),
			MinimumSeedTime:      indexer.Ptr(int64(172800)), // 48 hours
		})
	}
	return releases, nil
}

// attr returns an attribute of a node; "" when the node is nil.
func attr(n *html.Node, name string) string {
	if n == nil {
		return ""
	}
	return dom.Attr(n, name)
}

// parseTitle returns the first text of a cell that is not blank.
func parseTitle(cell *html.Node) (string, error) {
	if cell == nil {
		return "", indexer.Errorf("Shazbat row without a title")
	}
	for n := cell.FirstChild; n != nil; n = n.NextSibling {
		if n.Type == html.TextNode && strings.TrimSpace(n.Data) != "" {
			return strings.TrimSpace(n.Data), nil
		}
	}
	return "", indexer.Errorf("Shazbat row without a title")
}

// categories returns TV and the TV category of a title's resolution.
func categories(title string) []int {
	sub := indexer.TVSD
	switch {
	case strings.Contains(title, "1080p") || strings.Contains(title, "1080i") || strings.Contains(title, "720p"):
		sub = indexer.TVHD
	case strings.Contains(title, "2160p"):
		sub = indexer.TVUHD
	}
	return []int{indexer.TV.ID, sub.ID}
}
