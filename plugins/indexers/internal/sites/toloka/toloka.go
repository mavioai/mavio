package toloka

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/andybalholm/cascadia"
	"golang.org/x/net/html"
	"golang.org/x/text/encoding/unicode"

	"github.com/mavioai/mavio/plugins/indexers/internal/dom"
	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
	"github.com/mavioai/mavio/plugins/indexers/internal/parse"
)

var links = []string{"https://toloka.to/"}

// Definition is Toloka.to.
var Definition = indexer.Definition{
	ID:          "toloka",
	Name:        "Toloka.to",
	Description: "Toloka.to is a Semi-Private Ukrainian torrent site with a thriving file-sharing community",
	Language:    "uk-UA",
	Protocol:    indexer.ProtocolTorrent,
	Privacy:     indexer.SemiPrivate,
	Links:       links,
	Fields: []indexer.Field{
		indexer.BaseURLField(links),
		indexer.UsernameField,
		indexer.PasswordField,
		{Name: "freeleech_only", Label: "Freeleech Only", Type: indexer.FieldCheckbox, HelpText: "Search Freeleech torrents only"},
		{Name: "strip_cyrillic_letters", Label: "Strip Cyrillic Letters", Type: indexer.FieldCheckbox, Default: true},
	},
	New: newSite,
}

// Settings are Toloka.to's settings.
type Settings struct {
	indexer.UserPassSettings
	FreeleechOnly bool `json:"freeleech_only,omitempty"`
	// StripCyrillicLetters removes the Ukrainian and Russian from titles;
	// true when unset.
	StripCyrillicLetters *bool `json:"strip_cyrillic_letters,omitempty"`
}

func (s Settings) stripCyrillic() bool {
	return s.StripCyrillicLetters == nil || *s.StripCyrillicLetters
}

// Site is Toloka.to.
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

// Info implements indexer.Site.
func (s *Site) Info() indexer.Info {
	return indexer.Info{
		Protocol:       indexer.ProtocolTorrent,
		Privacy:        indexer.SemiPrivate,
		Encoding:       unicode.UTF8,
		SupportsRSS:    true,
		SupportsSearch: true,
	}
}

// Capabilities implements indexer.Site.
func (s *Site) Capabilities(context.Context, *indexer.Indexer) (*indexer.Capabilities, error) {
	return s.caps, nil
}

// SearchRequests implements indexer.Site, as Prowlarr's
// TolokaRequestGenerator does: TV searches add the season in Ukrainian.
func (s *Site) SearchRequests(_ context.Context, _ *indexer.Indexer, q *indexer.Query) (*indexer.Chain, error) {
	term := q.SanitizedTerm()
	if q.Type == indexer.SearchTV && q.Season != nil && *q.Season > 0 {
		term += fmt.Sprintf(" Сезон %d", *q.Season)
	}
	var p indexer.Params
	p.Set("o", "1")
	p.Set("s", "2")
	if strings.TrimSpace(term) != "" {
		p.Set("nm", strings.ReplaceAll(term, "-", " "))
	} else {
		p.Set("nm", "")
	}
	if s.settings.FreeleechOnly {
		p.Set("sds", "1")
	}
	u := s.baseURL + "tracker.php?" + p.Encode()
	// Each category is a parameter of its own.
	for _, cat := range s.caps.Categories.MapToTracker(q.Categories, false) {
		u += "&f[]=" + indexer.URLEncode(cat)
	}
	var c indexer.Chain
	r := indexer.NewRequest(u, indexer.AcceptHTML)
	// Prowlarr's IndexerRequest of a URL follows redirects.
	r.FollowRedirects = true
	c.AddRequests(r)
	return &c, nil
}

// LoginNeeded implements indexer.LoginSite: pages without the logout
// link are those of a guest.
func (s *Site) LoginNeeded(r *indexer.Response) bool {
	return !strings.Contains(r.Content(), "logout=true")
}

var loginError = cascadia.MustCompile(`table.forumline table span.gen`)

// Login implements indexer.LoginSite: it posts the credentials, keeping
// the answer's cookies for 30 days.
func (s *Site) Login(ctx context.Context, ix *indexer.Indexer) error {
	loginURL := s.baseURL + "login.php"
	var form indexer.Params
	form.Set("username", s.settings.Username)
	form.Set("password", s.settings.Password)
	form.Set("autologin", "on")
	form.Set("ssl", "on")
	form.Set("redirect", "")
	form.Set("login", "Вхід")
	r := indexer.NewRequest(loginURL, "")
	r.Method = http.MethodPost
	r.FollowRedirects = true
	r.Body = []byte(form.Encode())
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Referer", loginURL)
	resp, err := ix.Do(ctx, r)
	if err != nil {
		return fmt.Errorf("toloka login: %w", err)
	}
	if s.LoginNeeded(resp) {
		msg := "Unknown error message, please report."
		if doc, err := dom.Parse(resp.Content()); err == nil {
			if n := loginError.MatchFirst(doc); n != nil && n.FirstChild != nil {
				msg = dom.Text(n.FirstChild)
			}
		}
		return fmt.Errorf("%w: %s", indexer.ErrAuth, msg)
	}
	ix.SetCookies(resp.Cookies, time.Time{})
	slog.DebugContext(ctx, "Toloka.to authentication succeeded.")
	return nil
}

var (
	rows         = cascadia.MustCompile(`table.forumline > tbody > tr[class*=prow]`)
	downloadLink = cascadia.MustCompile(`td:nth-child(6) > a`)
	titleLink    = cascadia.MustCompile(`td:nth-child(3) > a`)
	categoryLink = cascadia.MustCompile(`td:nth-child(2) > a`)
	seedersSel   = cascadia.MustCompile(`td:nth-child(10) > b`)
	leechersSel  = cascadia.MustCompile(`td:nth-child(11) > b`)
	sizeSel      = cascadia.MustCompile(`td:nth-child(7)`)
	grabsSel     = cascadia.MustCompile(`td:nth-child(9)`)
	addedSel     = cascadia.MustCompile(`td:nth-child(13)`)
	gold         = cascadia.MustCompile(`img[src="images/gold.gif"], img[src="images/authors.gif"]`)
	silver       = cascadia.MustCompile(`img[src="images/silver.gif"]`)
	bronze       = cascadia.MustCompile(`img[src="images/bronze.gif"]`)
)

// ParseResponse implements indexer.Site, as Prowlarr's TolokaParser does.
func (s *Site) ParseResponse(_ context.Context, ix *indexer.Indexer, r *indexer.Response) ([]*indexer.Release, error) {
	doc, err := dom.Parse(r.Content())
	if err != nil {
		return nil, err
	}
	now := time.Now
	if ix != nil && ix.Now != nil {
		now = ix.Now
	}
	var releases []*indexer.Release
	for _, row := range rows.MatchAll(doc) {
		dl := downloadLink.MatchFirst(row)
		// Releases awaiting moderation cannot be downloaded.
		if dl == nil || !hasAttr(dl, "href") {
			continue
		}
		rel, err := s.release(row, dom.Attr(dl, "href"), now())
		if err != nil {
			return nil, fmt.Errorf("toloka: %w", err)
		}
		releases = append(releases, rel)
	}
	return releases, nil
}

func hasAttr(n *html.Node, name string) bool {
	for _, a := range n.Attr {
		if a.Key == name {
			return true
		}
	}
	return false
}

// number reads a number of a row as Prowlarr's CoerceInt does, failing
// when the row has no such cell.
func number(row *html.Node, sel cascadia.Selector, name string) (int, error) {
	n := sel.MatchFirst(row)
	if n == nil {
		return 0, fmt.Errorf("a row has no %s", name)
	}
	v, err := parse.Int(strings.TrimSpace(dom.Text(n)))
	return int(v), err
}

func (s *Site) release(row *html.Node, download string, now time.Time) (*indexer.Release, error) {
	link := titleLink.MatchFirst(row)
	infoURL := s.baseURL + dom.Attr(link, "href")
	var title string
	if link != nil {
		title = strings.TrimSpace(dom.Text(link))
	}
	cat := parse.QueryArg(dom.Attr(categoryLink.MatchFirst(row), "href"), "f")
	categories := indexer.IDs(s.caps.Categories.MapTracker(cat))
	seeders, err := number(row, seedersSel, "seeders")
	if err != nil {
		return nil, err
	}
	leechers, err := number(row, leechersSel, "leechers")
	if err != nil {
		return nil, err
	}
	grabs, err := number(row, grabsSel, "grabs")
	if err != nil {
		return nil, err
	}
	sizeNode := sizeSel.MatchFirst(row)
	if sizeNode == nil {
		return nil, fmt.Errorf("a row has no size")
	}
	size, err := parse.Bytes(strings.TrimSpace(dom.Text(sizeNode)))
	if err != nil {
		return nil, err
	}
	var added string
	if n := addedSel.MatchFirst(row); n != nil {
		added = strings.TrimSpace(dom.Text(n))
	}
	date, err := parse.Fuzzy(now, added, "")
	if err != nil {
		return nil, err
	}
	dvf := 1.0
	switch {
	case gold.MatchFirst(row) != nil:
		dvf = 0
	case silver.MatchFirst(row) != nil:
		dvf = 0.5
	case bronze.MatchFirst(row) != nil:
		dvf = 0.75
	}
	return &indexer.Release{
		GUID:                 infoURL,
		InfoURL:              infoURL,
		DownloadURL:          s.baseURL + download,
		Title:                parseTitle(title, categories, s.settings.stripCyrillic()),
		Description:          title,
		Categories:           categories,
		Seeders:              indexer.Ptr(seeders),
		Peers:                indexer.Ptr(seeders + leechers),
		Size:                 &size,
		Grabs:                indexer.Ptr(grabs),
		PublishDate:          date,
		DownloadVolumeFactor: indexer.Ptr(dvf),
		UploadVolumeFactor:   indexer.Ptr(1.0),
		MinimumRatio:         indexer.Ptr(1.0),
		MinimumSeedTime:      indexer.Ptr(int64(0)),
	}, nil
}
