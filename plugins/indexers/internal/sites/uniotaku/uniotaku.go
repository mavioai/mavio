package uniotaku

import (
	"context"
	"encoding/json"
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
	"github.com/mavioai/mavio/plugins/indexers/internal/jsonx"
	"github.com/mavioai/mavio/plugins/indexers/internal/parse"
)

var links = []string{"https://tracker.uniotaku.com/"}

// Definition is UniOtaku.
var Definition = indexer.Definition{
	ID:          "uniotaku",
	Name:        "UniOtaku",
	Description: "UniOtaku is a BRAZILIAN Semi-Private Torrent Tracker for ANIME",
	Language:    "pt-BR",
	Protocol:    indexer.ProtocolTorrent,
	Privacy:     indexer.SemiPrivate,
	Links:       links,
	Fields: []indexer.Field{
		indexer.BaseURLField(links),
		indexer.UsernameField,
		indexer.PasswordField,
		{Name: "freeleech_only", Label: "Freeleech Only", Type: indexer.FieldCheckbox, HelpText: "Search Freeleech torrents only"},
	},
	New: newSite,
}

// Settings are UniOtaku's settings.
type Settings struct {
	indexer.UserPassSettings
	FreeleechOnly bool `json:"freeleech_only,omitempty"`
}

// Site is UniOtaku.
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
	c.Categories.Add("28", indexer.TVAnime, "Anime")
	c.Categories.Add("47", indexer.MoviesOther, "Filme")
	c.Categories.Add("48", indexer.TVAnime, "OVA")
	c.Categories.Add("49", indexer.BooksComics, "Mangá")
	c.Categories.Add("50", indexer.TVOther, "Dorama")
	c.Categories.Add("51", indexer.Audio, "OST")
	c.Categories.Add("52", indexer.TVAnime, "Anime Completo")
	c.Categories.Add("53", indexer.BooksComics, "Mangá Completo")
	c.Categories.Add("54", indexer.TVOther, "Dorama Completo")
	c.Categories.Add("55", indexer.XXX, "Hentai")
	c.Categories.Add("56", indexer.XXXOther, "H Doujinshi")
	c.Categories.Add("57", indexer.TVOther, "Tokusatsu")
	c.Categories.Add("58", indexer.TVOther, "Live Action")
	return c
}

// Info implements indexer.Site.
func (s *Site) Info() indexer.Info {
	return indexer.Info{
		Protocol:       indexer.ProtocolTorrent,
		Privacy:        indexer.SemiPrivate,
		SupportsRSS:    true,
		SupportsSearch: true,
	}
}

// Capabilities implements indexer.Site.
func (s *Site) Capabilities(context.Context, *indexer.Indexer) (*indexer.Capabilities, error) {
	return s.caps, nil
}

// separators are the runs of spaces, dashes, dots and underscores, which
// the site's search takes as wildcards.
var separators = regexp.MustCompile(`[ -._]+`)

// SearchRequests implements indexer.Site: one search of the table the
// torrents page loads, of one category or all. Episodes are searched by
// the series' name alone.
func (s *Site) SearchRequests(_ context.Context, _ *indexer.Indexer, q *indexer.Query) (*indexer.Chain, error) {
	term := q.SanitizedTerm()
	if strings.TrimSpace(term) != "" {
		term = "%" + strings.TrimSpace(separators.ReplaceAllString(term, "%")) + "%"
	}
	category := "0"
	if cats := s.caps.Categories.MapToTracker(q.Categories, false); len(cats) == 1 {
		category = cats[0]
	}
	status := "0"
	if s.settings.FreeleechOnly {
		status = "1"
	}
	var p indexer.Params
	p.Set("categoria", category)
	p.Set("grupo", "0")
	p.Set("status", status)
	p.Set("ordenar", "0")
	p.Set("start", "0")
	p.Set("length", "100")
	p.Set("search[value]", term)
	p.Set("search[regex]", "false")
	r := indexer.NewRequest(s.baseURL+"torrents_.php?"+p.Encode(), indexer.AcceptHTML)
	r.FollowRedirects = true
	var c indexer.Chain
	c.AddRequests(r)
	return &c, nil
}

// LoginNeeded implements indexer.LoginSite: logged in users have the uid
// and pass cookies, sent or set.
func (s *Site) LoginNeeded(r *indexer.Response) bool {
	has := func(name string) bool {
		_, sent := r.Request.Cookies[name]
		_, set := r.Cookies[name]
		return sent || set
	}
	return !has("uid") || !has("pass")
}

var loginError = cascadia.MustCompile(`.login-content span.text-red`)

// Login implements indexer.LoginSite: it posts the credentials to
// account-login.php, keeping the cookies for 30 days.
func (s *Site) Login(ctx context.Context, ix *indexer.Indexer) error {
	ix.SetCookies(nil, time.Time{})
	var form indexer.Params
	form.Set("username", s.settings.Username)
	form.Set("password", s.settings.Password)
	form.Set("manter", "1")
	r := indexer.NewRequest(s.baseURL+"account-login.php", "")
	r.Method = http.MethodPost
	r.Body = []byte(form.Encode())
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Referer", s.baseURL)
	resp, err := ix.Do(ctx, r)
	if err != nil {
		return fmt.Errorf("uniotaku login: %w", err)
	}
	if resp.HasError() {
		return fmt.Errorf("uniotaku login: %w", &indexer.HTTPError{Response: resp})
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

var downloadLink = cascadia.MustCompile(`a[href^="download.php?id="]`)

// DownloadRequest implements indexer.DownloadSite: releases link to their
// details pages, whose download link is the torrent's.
func (s *Site) DownloadRequest(ctx context.Context, ix *indexer.Indexer, link string) (*indexer.Request, error) {
	r := indexer.NewRequest(link, indexer.AcceptHTML)
	r.Cookies = ix.Cookies()
	resp, err := ix.Do(ctx, r)
	if err != nil {
		return nil, fmt.Errorf("uniotaku: %w", err)
	}
	if resp.HasError() {
		return nil, fmt.Errorf("uniotaku: %w", &indexer.HTTPError{Response: resp})
	}
	doc, err := dom.Parse(resp.Content())
	if err != nil {
		return nil, err
	}
	a := downloadLink.MatchFirst(doc)
	if a == nil {
		return nil, fmt.Errorf("Failed to fetch download link from %s", link) //nolint:staticcheck // Prowlarr's message.
	}
	return indexer.NewRequest(s.baseURL+strings.TrimSpace(dom.Attr(a, "href")), ""), nil
}

var (
	titleLink   = cascadia.MustCompile(`a[href^="torrents-details.php?id="]`)
	categoryImg = cascadia.MustCompile(`img[alt]`)
	groupLink   = cascadia.MustCompile(`a[href*="teams-view.php?id="]`)
	freeImg     = cascadia.MustCompile(`img[src*="images/free.gif"]`)
	silverImg   = cascadia.MustCompile(`img[src*="images/silverdownload.gif"]`)
)

// ParseResponse implements indexer.Site, as Prowlarr's UniotakuParser
// does: the table's cells are HTML or numbers. Torrents have no dates;
// they are given one a minute apart, newest first.
func (s *Site) ParseResponse(_ context.Context, _ *indexer.Indexer, r *indexer.Response) ([]*indexer.Release, error) {
	var resp struct {
		Data [][]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal([]byte(r.Content()), &resp); err != nil {
		return nil, fmt.Errorf("uniotaku: %w", err)
	}
	if resp.Data == nil {
		return nil, errors.New("uniotaku: the response has no data")
	}
	date := time.Now()
	releases := make([]*indexer.Release, 0, len(resp.Data))
	for _, item := range resp.Data {
		date = date.Add(-time.Minute)
		rel, err := s.release(item, date)
		if err != nil {
			return nil, fmt.Errorf("uniotaku: %w", err)
		}
		releases = append(releases, rel)
	}
	return releases, nil
}

func (s *Site) release(item []json.RawMessage, date time.Time) (*indexer.Release, error) {
	text := func(i int) (string, error) {
		if i >= len(item) {
			return "", fmt.Errorf("a row has no column %d", i)
		}
		var v jsonx.String
		if err := json.Unmarshal(item[i], &v); err != nil {
			return "", fmt.Errorf("column %d: %w", i, err)
		}
		return string(v), nil
	}
	number := func(i int) (*int, error) {
		if i >= len(item) {
			return nil, nil
		}
		var v *jsonx.Int
		if err := json.Unmarshal(item[i], &v); err != nil {
			return nil, fmt.Errorf("column %d: %w", i, err)
		}
		if v == nil {
			return nil, nil
		}
		return indexer.Ptr(int(*v)), nil
	}
	var docs [3]*html.Node
	for j, i := range []int{0, 1, 7} {
		h, err := text(i)
		if err != nil {
			return nil, err
		}
		if docs[j], err = dom.Parse(h); err != nil {
			return nil, err
		}
	}
	details, categoryDoc, groupDoc := docs[0], docs[1], docs[2]
	var title, href string
	if a := titleLink.MatchFirst(details); a != nil {
		title, href = strings.TrimSpace(dom.Text(a)), dom.Attr(a, "href")
	}
	infoURL := s.baseURL + href
	category := "Anime"
	if img := categoryImg.MatchFirst(categoryDoc); img != nil {
		category = strings.TrimSpace(dom.Attr(img, "alt"))
	}
	if a := groupLink.MatchFirst(groupDoc); a != nil {
		if group := strings.TrimSpace(dom.Text(a)); group != "" {
			title += " [" + group + "]"
		}
	}
	seeders, err := number(3)
	if err != nil {
		return nil, err
	}
	leechers, err := number(4)
	if err != nil {
		return nil, err
	}
	grabs, err := number(5)
	if err != nil {
		return nil, err
	}
	sizeText, err := text(6)
	if err != nil {
		return nil, err
	}
	size, err := parse.Bytes(sizeText)
	if err != nil {
		return nil, err
	}
	rel := &indexer.Release{
		GUID:                 infoURL,
		InfoURL:              infoURL,
		DownloadURL:          infoURL,
		Title:                title,
		Categories:           indexer.IDs(s.caps.Categories.MapTrackerDesc(category)),
		Size:                 &size,
		Grabs:                grabs,
		Seeders:              seeders,
		PublishDate:          date,
		DownloadVolumeFactor: indexer.Ptr(1.0),
		UploadVolumeFactor:   indexer.Ptr(1.0),
		MinimumRatio:         indexer.Ptr(0.7),
	}
	if seeders != nil && leechers != nil {
		rel.Peers = indexer.Ptr(*seeders + *leechers)
	}
	switch {
	case freeImg.MatchFirst(details) != nil:
		rel.DownloadVolumeFactor = indexer.Ptr(0.0)
	case silverImg.MatchFirst(details) != nil:
		rel.DownloadVolumeFactor = indexer.Ptr(0.5)
	}
	return rel, nil
}
