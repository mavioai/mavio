package anidex

import (
	"context"
	"fmt"
	"net/http"
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

var links = []string{"https://anidex.info/"}

// languages are the site's languages, by ID.
var languages = []indexer.Option{
	{Value: 1, Label: "English"},
	{Value: 2, Label: "Japanese"},
	{Value: 3, Label: "Polish"},
	{Value: 4, Label: "Serbo-Croatian"},
	{Value: 5, Label: "Dutch"},
	{Value: 6, Label: "Italian"},
	{Value: 7, Label: "Russian"},
	{Value: 8, Label: "German"},
	{Value: 9, Label: "Hungarian"},
	{Value: 10, Label: "French"},
	{Value: 11, Label: "Finnish"},
	{Value: 12, Label: "Vietnamese"},
	{Value: 13, Label: "Greek"},
	{Value: 14, Label: "Bulgarian"},
	{Value: 15, Label: "Spanish (Spain)"},
	{Value: 16, Label: "Portuguese (Brazil)"},
	{Value: 17, Label: "Portuguese (Portugal)"},
	{Value: 18, Label: "Swedish"},
	{Value: 19, Label: "Arabic"},
	{Value: 20, Label: "Danish"},
	{Value: 21, Label: "Chinese (Simplified)"},
	{Value: 22, Label: "Bengali"},
	{Value: 23, Label: "Romanian"},
	{Value: 24, Label: "Czech"},
	{Value: 25, Label: "Mongolian"},
	{Value: 26, Label: "Turkish"},
	{Value: 27, Label: "Indonesian"},
	{Value: 28, Label: "Korean"},
	{Value: 29, Label: "Spanish (LATAM)"},
	{Value: 30, Label: "Persian"},
	{Value: 31, Label: "Malaysian"},
}

// Definition is Anidex.
var Definition = indexer.Definition{
	ID:          "anidex",
	Name:        "Anidex",
	Description: "Anidex is a Public torrent tracker and indexer, primarily for English fansub groups of anime",
	Language:    "en-US",
	Protocol:    indexer.ProtocolTorrent,
	Privacy:     indexer.Public,
	Links:       links,
	Fields: []indexer.Field{
		indexer.BaseURLField(links),
		{Name: "authorised_only", Label: "Authorised Only", Type: indexer.FieldCheckbox, HelpText: "Search authorised torrents only"},
		{
			Name: "languages_only", Label: "Languages Only", Type: indexer.FieldMultiSelect,
			HelpText: "Search selected languages only. None ticked = ALL.", Options: languages,
		},
	},
	New: newSite,
}

// Settings are Anidex's settings.
type Settings struct {
	indexer.BaseSettings
	AuthorisedOnly bool `json:"authorised_only,omitempty"`
	// LanguagesOnly are the IDs of the languages searched; all when
	// empty.
	LanguagesOnly []int `json:"languages_only,omitempty"`
}

// Site is Anidex.
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
	return &Site{settings: settings, baseURL: settings.URL(links, nil), caps: capabilities()}, nil
}

func capabilities() *indexer.Capabilities {
	c := indexer.NewCapabilities()
	c.Params[indexer.SearchTV] = []indexer.Param{indexer.ParamQ, indexer.ParamSeason, indexer.ParamEp}
	c.Params[indexer.SearchMusic] = []indexer.Param{indexer.ParamQ}
	c.Params[indexer.SearchBook] = []indexer.Param{indexer.ParamQ}
	c.Categories.Add("1", indexer.TVAnime, "Anime - Sub")
	c.Categories.Add("2", indexer.TVAnime, "Anime - Raw")
	c.Categories.Add("3", indexer.TVAnime, "Anime - Dub")
	c.Categories.Add("4", indexer.TVAnime, "LA - Sub")
	c.Categories.Add("5", indexer.TVAnime, "LA - Raw")
	c.Categories.Add("6", indexer.BooksEBook, "Light Novel")
	c.Categories.Add("7", indexer.BooksComics, "Manga - TLed")
	c.Categories.Add("8", indexer.BooksComics, "Manga - Raw")
	c.Categories.Add("9", indexer.AudioMP3, "♫ - Lossy")
	c.Categories.Add("10", indexer.AudioLossless, "♫ - Lossless")
	c.Categories.Add("11", indexer.AudioVideo, "♫ - Video")
	c.Categories.Add("12", indexer.PCGames, "Games")
	c.Categories.Add("13", indexer.PC0day, "Applications")
	c.Categories.Add("14", indexer.XXXImageSet, "Pictures")
	c.Categories.Add("15", indexer.XXX, "Adult Video")
	c.Categories.Add("16", indexer.Other, "Other")
	return c
}

// Info implements indexer.Site.
func (s *Site) Info() indexer.Info {
	return indexer.Info{
		Protocol:       indexer.ProtocolTorrent,
		Privacy:        indexer.Public,
		SupportsRSS:    true,
		SupportsSearch: true,
	}
}

// Capabilities implements indexer.Site.
func (s *Site) Capabilities(context.Context, *indexer.Indexer) (*indexer.Capabilities, error) {
	return s.caps, nil
}

// SearchRequests implements indexer.Site, as Prowlarr's
// AnidexRequestGenerator does; movies are not searched.
func (s *Site) SearchRequests(_ context.Context, _ *indexer.Indexer, q *indexer.Query) (*indexer.Chain, error) {
	var c indexer.Chain
	var term string
	switch q.Type {
	case indexer.SearchMovie:
		return &c, nil
	case indexer.SearchTV:
		term = q.SanitizedTVString()
	default:
		term = q.SanitizedTerm()
	}
	var p indexer.Params
	p.Set("page", "search")
	p.Set("s", "upload_timestamp")
	p.Set("o", "desc")
	p.Set("group_id", "0") // all groups
	p.Set("q", term)
	if s.settings.AuthorisedOnly {
		p.Add("a", "1")
	}
	u := s.baseURL + "?" + p.Encode()
	cats := s.caps.Categories.MapToTracker(q.Categories, false)
	if len(cats) > 0 && slices.ContainsFunc(s.caps.Categories.TrackerCategories(), func(c string) bool { return !slices.Contains(cats, c) }) {
		u += "&id=" + strings.Join(cats, ",")
	}
	if len(s.settings.LanguagesOnly) > 0 {
		ids := make([]string, len(s.settings.LanguagesOnly))
		for i, id := range s.settings.LanguagesOnly {
			ids[i] = strconv.Itoa(id)
		}
		u += "&lang_id=" + strings.Join(ids, ",")
	}
	c.AddRequests(indexer.NewRequest(u, indexer.AcceptHTML))
	return &c, nil
}

var (
	rows       = cascadia.MustCompile(`div#content table > tbody > tr`)
	download   = cascadia.MustCompile(`a[href^="/dl/"]`)
	infoLink   = cascadia.MustCompile(`td:nth-child(3) a`)
	titleSpan  = cascadia.MustCompile(`td:nth-child(3) span`)
	language   = cascadia.MustCompile(`td:nth-child(1) img`)
	category   = cascadia.MustCompile(`td:nth-child(1) a`)
	seedersTD  = cascadia.MustCompile(`td:nth-child(9)`)
	leechersTD = cascadia.MustCompile(`td:nth-child(10)`)
	sizeTD     = cascadia.MustCompile(`td:nth-child(7)`)
	grabsTD    = cascadia.MustCompile(`td:nth-child(11)`)
	addedTD    = cascadia.MustCompile(`td:nth-child(8)`)
	magnet     = cascadia.MustCompile(`a[href^="magnet:?"]`)
)

// ParseResponse implements indexer.Site, as Prowlarr's AnidexParser
// does.
func (s *Site) ParseResponse(_ context.Context, _ *indexer.Indexer, r *indexer.Response) ([]*indexer.Release, error) {
	if r.StatusCode != http.StatusOK {
		return nil, indexer.Errorf("Anidex search returned unexpected result. Expected 200 OK but got %d.", r.StatusCode)
	}
	doc, err := dom.Parse(r.Content())
	if err != nil {
		return nil, err
	}
	var releases []*indexer.Release
	for _, row := range rows.MatchAll(doc) {
		rel, err := s.release(row)
		if err != nil {
			return nil, fmt.Errorf("anidex: %w", err)
		}
		releases = append(releases, rel)
	}
	return releases, nil
}

// text returns the text of the first element matching sel, ok false when
// there is none.
func text(row *html.Node, sel cascadia.Selector) (string, bool) {
	n := sel.MatchFirst(row)
	return dom.Text(n), n != nil
}

func (s *Site) release(row *html.Node) (*indexer.Release, error) {
	// The links are absolute paths appended to the base URL ending with
	// a slash, as Prowlarr appends them.
	downloadURL := s.baseURL + dom.Attr(download.MatchFirst(row), "href")
	infoURL := s.baseURL + dom.Attr(infoLink.MatchFirst(row), "href")
	title := strings.TrimSpace(dom.Attr(titleSpan.MatchFirst(row), "title"))
	if lang := strings.TrimSpace(dom.Attr(language.MatchFirst(row), "title")); lang != "" {
		title += " [" + lang + "]"
	}
	cat := category.MatchFirst(row)
	if cat == nil {
		return nil, fmt.Errorf("a row has no category")
	}
	categories := indexer.IDs(s.caps.Categories.MapTracker(parse.QueryArg(dom.Attr(cat, "href"), "id")))
	var counts [3]int64
	for i, sel := range []cascadia.Selector{seedersTD, leechersTD, grabsTD} {
		t, ok := text(row, sel)
		if !ok {
			return nil, fmt.Errorf("a row has no seeders, leechers or grabs")
		}
		n, err := parse.Int(strings.TrimSpace(t))
		if err != nil {
			return nil, err
		}
		counts[i] = n
	}
	sizeText, ok := text(row, sizeTD)
	if !ok {
		return nil, fmt.Errorf("a row has no size")
	}
	size, err := parse.Bytes(strings.TrimSpace(sizeText))
	if err != nil {
		return nil, err
	}
	added := addedTD.MatchFirst(row)
	if added == nil {
		return nil, fmt.Errorf("a row has no date")
	}
	date, err := time.Parse("2006-01-02 15:04:05 UTC", strings.TrimSpace(dom.Attr(added, "title")))
	if err != nil {
		return nil, err
	}
	seeders := int(counts[0])
	return &indexer.Release{
		GUID:                 infoURL,
		InfoURL:              infoURL,
		DownloadURL:          downloadURL,
		MagnetURL:            dom.Attr(magnet.MatchFirst(row), "href"),
		Title:                title,
		Categories:           categories,
		Seeders:              &seeders,
		Peers:                indexer.Ptr(seeders + int(counts[1])),
		Size:                 &size,
		Grabs:                indexer.Ptr(int(counts[2])),
		PublishDate:          date,
		DownloadVolumeFactor: indexer.Ptr(0.0),
		UploadVolumeFactor:   indexer.Ptr(1.0),
	}, nil
}
