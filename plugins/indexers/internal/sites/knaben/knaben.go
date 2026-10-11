package knaben

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
)

var (
	links       = []string{"https://knaben.org/"}
	legacyLinks = []string{"https://knaben.eu/"}
)

// apiURL is the API's search endpoint, whatever the site's address.
const apiURL = "https://api.knaben.org/v1"

// Definition is Knaben.
var Definition = indexer.Definition{
	ID:          "knaben",
	Name:        "Knaben",
	Description: "Knaben is a Public torrent meta-search engine",
	Protocol:    indexer.ProtocolTorrent,
	Privacy:     indexer.Public,
	Links:       links,
	LegacyLinks: legacyLinks,
	Fields:      []indexer.Field{indexer.BaseURLField(links)},
	New:         newSite,
}

// Site is Knaben.
type Site struct {
	caps *indexer.Capabilities
}

func newSite(data []byte) (indexer.Site, error) {
	var settings indexer.BaseSettings
	if err := indexer.DecodeSettings(data, &settings); err != nil {
		return nil, err
	}
	return &Site{caps: capabilities()}, nil
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

// searchBody is the API's search, its fields in the order Prowlarr
// writes them.
type searchBody struct {
	OrderBy        string `json:"order_by"`
	OrderDirection string `json:"order_direction"`
	From           int    `json:"from"`
	Size           int    `json:"size"`
	HideUnsafe     bool   `json:"hide_unsafe"`
	SearchType     string `json:"search_type,omitempty"`
	SearchField    string `json:"search_field,omitempty"`
	Query          string `json:"query,omitempty"`
	Categories     []int  `json:"categories,omitempty"`
}

// SearchRequests implements indexer.Site: a POST of the newest 100
// releases matching the title and categories.
func (s *Site) SearchRequests(_ context.Context, _ *indexer.Indexer, q *indexer.Query) (*indexer.Chain, error) {
	term := q.SanitizedTerm()
	if q.Type == indexer.SearchTV {
		term = q.SanitizedTVString()
	}
	body := searchBody{OrderBy: "date", OrderDirection: "desc", Size: 100, HideUnsafe: true}
	if term = strings.TrimSpace(term); term != "" {
		body.SearchType, body.SearchField, body.Query = "100%", "title", term
	}
	for _, c := range s.caps.Categories.MapToTracker(q.Categories, false) {
		id, err := strconv.Atoi(c)
		if err != nil {
			return nil, fmt.Errorf("knaben: category %q: %w", c, err)
		}
		if !slices.Contains(body.Categories, id) {
			body.Categories = append(body.Categories, id)
		}
	}
	// Prowlarr sends the body indented, as Newtonsoft.Json writes it.
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(body); err != nil {
		return nil, err
	}
	r := indexer.NewRequest(apiURL, indexer.AcceptJSON)
	// Prowlarr's IndexerRequest of a URL follows redirects.
	r.FollowRedirects = true
	r.Method = http.MethodPost
	r.Header.Set("Content-Type", "application/json")
	r.Body = bytes.TrimSuffix(buf.Bytes(), []byte("\n"))
	var c indexer.Chain
	c.AddRequests(r)
	return &c, nil
}

type response struct {
	Hits []hit `json:"hits"`
}

type hit struct {
	Title       string `json:"title"`
	CategoryIDs []int  `json:"categoryId"`
	InfoHash    string `json:"hash"`
	InfoURL     string `json:"details"`
	DownloadURL string `json:"link"`
	MagnetURL   string `json:"magnetUrl"`
	Size        int64  `json:"bytes"`
	Seeders     int    `json:"seeders"`
	Leechers    int    `json:"peers"`
	Date        string `json:"date"`
}

// zoneRegex finds the UTC offset some dates end with.
var zoneRegex = regexp.MustCompile(`[+-]\d{2}:\d{2}$`)

// ParseResponse implements indexer.Site, as Prowlarr's KnabenParser does:
// releases without seeders are left out.
func (s *Site) ParseResponse(_ context.Context, _ *indexer.Indexer, r *indexer.Response) ([]*indexer.Release, error) {
	if r.StatusCode != http.StatusOK {
		return nil, indexer.Errorf("Unexpected response status %d code from indexer request", r.StatusCode)
	}
	if !r.HasContentType(indexer.AcceptJSON) {
		return nil, indexer.Errorf("Unexpected response header %s from indexer request, expected %s", r.Header.Get("Content-Type"), indexer.AcceptJSON)
	}
	var resp *response
	if err := json.Unmarshal([]byte(r.Content()), &resp); err != nil {
		return nil, fmt.Errorf("knaben: %w", err)
	}
	if resp == nil {
		return nil, nil
	}
	var releases []*indexer.Release
	for _, row := range resp.Hits {
		if row.Seeders <= 0 {
			continue
		}
		date, err := parseDate(row.Date)
		if err != nil {
			return nil, err
		}
		var cats []int
		for _, id := range row.CategoryIDs {
			for _, c := range s.caps.Categories.MapTracker(strconv.Itoa(id)) {
				if !slices.Contains(cats, c.ID) {
					cats = append(cats, c.ID)
				}
			}
		}
		rel := &indexer.Release{
			GUID:                 row.InfoURL,
			Title:                row.Title,
			InfoURL:              row.InfoURL,
			Categories:           cats,
			InfoHash:             row.InfoHash,
			Size:                 indexer.Ptr(row.Size),
			Seeders:              indexer.Ptr(row.Seeders),
			Peers:                indexer.Ptr(row.Leechers + row.Seeders),
			PublishDate:          date,
			DownloadVolumeFactor: indexer.Ptr(0.0),
			UploadVolumeFactor:   indexer.Ptr(1.0),
		}
		if strings.TrimSpace(row.DownloadURL) != "" {
			rel.DownloadURL = row.DownloadURL
		}
		if strings.TrimSpace(row.MagnetURL) != "" {
			rel.MagnetURL = row.MagnetURL
		}
		releases = append(releases, rel)
	}
	return releases, nil
}

// dateLayouts are the forms of the API's dates, once they have a UTC
// offset.
var dateLayouts = []string{
	"2006-01-02T15:04:05.999999999Z07:00",
	"2006-01-02 15:04:05.999999999Z07:00",
	"2006-01-02T15:04Z07:00",
}

// parseDate reads a release's date; those without an offset are at UTC+1.
func parseDate(s string) (time.Time, error) {
	if strings.TrimSpace(s) != "" && !zoneRegex.MatchString(s) {
		s += "+01:00"
	}
	for _, l := range dateLayouts {
		if t, err := time.Parse(l, strings.TrimSpace(s)); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("knaben: unable to parse date %q", s)
}

func capabilities() *indexer.Capabilities {
	c := indexer.NewCapabilities()
	c.Params[indexer.SearchTV] = []indexer.Param{indexer.ParamQ, indexer.ParamSeason, indexer.ParamEp}
	c.Params[indexer.SearchMovie] = []indexer.Param{indexer.ParamQ}
	c.Params[indexer.SearchMusic] = []indexer.Param{indexer.ParamQ}
	c.Params[indexer.SearchBook] = []indexer.Param{indexer.ParamQ}
	c.Categories.Add("1000000", indexer.Audio, "Audio")
	c.Categories.Add("1001000", indexer.AudioMP3, "MP3")
	c.Categories.Add("1002000", indexer.AudioLossless, "Lossless")
	c.Categories.Add("1003000", indexer.AudioAudiobook, "Audiobook")
	c.Categories.Add("1004000", indexer.AudioVideo, "Audio Video")
	c.Categories.Add("1005000", indexer.AudioOther, "Radio")
	c.Categories.Add("1006000", indexer.AudioOther, "Audio Other")
	c.Categories.Add("2000000", indexer.TV, "TV")
	c.Categories.Add("2001000", indexer.TVHD, "TV HD")
	c.Categories.Add("2002000", indexer.TVSD, "TV SD")
	c.Categories.Add("2003000", indexer.TVUHD, "TV UHD")
	c.Categories.Add("2004000", indexer.TVDocumentary, "Documentary")
	c.Categories.Add("2005000", indexer.TVForeign, "TV Foreign")
	c.Categories.Add("2006000", indexer.TVSport, "Sport")
	c.Categories.Add("2007000", indexer.TVOther, "Cartoon")
	c.Categories.Add("2008000", indexer.TVOther, "TV Other")
	c.Categories.Add("3000000", indexer.Movies, "Movies")
	c.Categories.Add("3001000", indexer.MoviesHD, "Movies HD")
	c.Categories.Add("3002000", indexer.MoviesSD, "Movies SD")
	c.Categories.Add("3003000", indexer.MoviesUHD, "Movies UHD")
	c.Categories.Add("3004000", indexer.MoviesDVD, "Movies DVD")
	c.Categories.Add("3005000", indexer.MoviesForeign, "Movies Foreign")
	c.Categories.Add("3006000", indexer.MoviesForeign, "Movies Bollywood")
	c.Categories.Add("3007000", indexer.Movies3D, "Movies 3D")
	c.Categories.Add("3008000", indexer.MoviesOther, "Movies Other")
	c.Categories.Add("4000000", indexer.PC, "PC")
	c.Categories.Add("4001000", indexer.PCGames, "Games")
	c.Categories.Add("4002000", indexer.PC0day, "Software")
	c.Categories.Add("4003000", indexer.PCMac, "Mac")
	c.Categories.Add("4004000", indexer.PCISO, "Unix")
	c.Categories.Add("5000000", indexer.XXX, "XXX")
	c.Categories.Add("5001000", indexer.XXXx264, "XXX Video")
	c.Categories.Add("5002000", indexer.XXXImageSet, "XXX ImageSet")
	c.Categories.Add("5003000", indexer.XXXOther, "XXX Games")
	c.Categories.Add("5004000", indexer.XXXOther, "XXX Hentai")
	c.Categories.Add("5005000", indexer.XXXOther, "XXX Other")
	c.Categories.Add("6000000", indexer.TVAnime, "Anime")
	c.Categories.Add("6001000", indexer.TVAnime, "Anime Subbed")
	c.Categories.Add("6002000", indexer.TVAnime, "Anime Dubbed")
	c.Categories.Add("6003000", indexer.TVAnime, "Anime Dual audio")
	c.Categories.Add("6004000", indexer.TVAnime, "Anime Raw")
	c.Categories.Add("6005000", indexer.AudioVideo, "Music Video")
	c.Categories.Add("6006000", indexer.BooksOther, "Literature")
	c.Categories.Add("6007000", indexer.AudioOther, "Music")
	c.Categories.Add("6008000", indexer.TVAnime, "Anime non-english translated")
	c.Categories.Add("7000000", indexer.Console, "Console")
	c.Categories.Add("7001000", indexer.ConsolePS4, "PS4")
	c.Categories.Add("7002000", indexer.ConsolePS3, "PS3")
	c.Categories.Add("7003000", indexer.ConsolePS3, "PS2")
	c.Categories.Add("7004000", indexer.ConsolePS3, "PS1")
	c.Categories.Add("7005000", indexer.ConsolePSVita, "PS Vita")
	c.Categories.Add("7006000", indexer.ConsolePSP, "PSP")
	c.Categories.Add("7007000", indexer.ConsoleXBox360, "Xbox 360")
	c.Categories.Add("7008000", indexer.ConsoleXBox, "Xbox")
	c.Categories.Add("7009000", indexer.ConsoleNDS, "Switch")
	c.Categories.Add("7010000", indexer.ConsoleNDS, "NDS")
	c.Categories.Add("7011000", indexer.ConsoleWii, "Wii")
	c.Categories.Add("7012000", indexer.ConsoleWiiU, "WiiU")
	c.Categories.Add("7013000", indexer.Console3DS, "3DS")
	c.Categories.Add("7014000", indexer.ConsoleWii, "GameCube")
	c.Categories.Add("7015000", indexer.ConsoleOther, "Other")
	c.Categories.Add("8000000", indexer.PCMobileOther, "Mobile")
	c.Categories.Add("8001000", indexer.PCMobileAndroid, "Android")
	c.Categories.Add("8002000", indexer.PCMobileiOS, "IOS")
	c.Categories.Add("8003000", indexer.PCMobileOther, "PC Other")
	c.Categories.Add("9000000", indexer.Books, "Books")
	c.Categories.Add("9001000", indexer.BooksEBook, "EBooks")
	c.Categories.Add("9002000", indexer.BooksComics, "Comics")
	c.Categories.Add("9003000", indexer.BooksMags, "Magazines")
	c.Categories.Add("9004000", indexer.BooksTechnical, "Technical")
	c.Categories.Add("9005000", indexer.BooksOther, "Books Other")
	c.Categories.Add("10000000", indexer.Other, "Other")
	c.Categories.Add("10001000", indexer.OtherMisc, "Other Misc")
	return c
}
