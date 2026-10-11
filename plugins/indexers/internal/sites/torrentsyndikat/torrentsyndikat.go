package torrentsyndikat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
	"github.com/mavioai/mavio/plugins/indexers/internal/jsonx"
)

var links = []string{"https://torrent-syndikat.org/"}

// ReleaseType is a kind of release the site lists.
type ReleaseType int

// The release types.
const (
	ReleaseP2P    ReleaseType = 1
	ReleaseScene  ReleaseType = 2
	ReleaseOScene ReleaseType = 3
)

// String returns the type's name in the API.
func (t ReleaseType) String() string {
	switch t {
	case ReleaseP2P:
		return "P2P"
	case ReleaseScene:
		return "Scene"
	case ReleaseOScene:
		return "O-Scene"
	}
	return strconv.Itoa(int(t))
}

// Definition is TorrentSyndikat.
var Definition = indexer.Definition{
	ID:          "torrentsyndikat",
	Name:        "TorrentSyndikat",
	Description: "A German general tracker",
	Language:    "de-DE",
	Protocol:    indexer.ProtocolTorrent,
	Privacy:     indexer.Private,
	Links:       links,
	Fields: []indexer.Field{
		indexer.BaseURLField(links),
		{Name: "api_key", Label: "ApiKey", HelpText: "Site API Key", Type: indexer.FieldText, Secret: true},
		{Name: "products_only", Label: "Products Only", Type: indexer.FieldCheckbox, HelpText: "Limit search to torrents linked to a product"},
		{
			Name: "release_types", Label: "Release Types", Type: indexer.FieldMultiSelect,
			Options: []indexer.Option{
				{Value: int(ReleaseP2P), Label: "P2P"},
				{Value: int(ReleaseScene), Label: "Scene"},
				{Value: int(ReleaseOScene), Label: "O-Scene"},
			},
		},
	},
	New: newSite,
}

// Settings are TorrentSyndikat's settings.
type Settings struct {
	indexer.BaseSettings
	APIKey       string        `json:"api_key,omitempty"`
	ProductsOnly bool          `json:"products_only,omitempty"`
	ReleaseTypes []ReleaseType `json:"release_types,omitempty"`
}

// Validate checks that the API key is set.
func (s Settings) Validate() error {
	if strings.TrimSpace(s.APIKey) == "" {
		return errors.New("api_key: must not be empty")
	}
	return nil
}

// Site is TorrentSyndikat.
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
	c.Params[indexer.SearchMovie] = []indexer.Param{indexer.ParamQ, indexer.ParamIMDbID}
	c.Params[indexer.SearchMusic] = []indexer.Param{indexer.ParamQ}
	c.Params[indexer.SearchBook] = []indexer.Param{indexer.ParamQ}
	c.Categories.Add("2", indexer.PC, "Apps / Windows")
	c.Categories.Add("13", indexer.PC, "Apps / Linux")
	c.Categories.Add("4", indexer.PCMac, "Apps / MacOS")
	c.Categories.Add("6", indexer.PC, "Apps / Misc")
	c.Categories.Add("50", indexer.PCGames, "Spiele / Windows")
	c.Categories.Add("51", indexer.PCGames, "Spiele / MacOS")
	c.Categories.Add("52", indexer.PCGames, "Spiele / Linux")
	c.Categories.Add("8", indexer.ConsoleOther, "Spiele / Playstation")
	c.Categories.Add("7", indexer.ConsoleOther, "Spiele / Nintendo")
	c.Categories.Add("32", indexer.ConsoleOther, "Spiele / XBOX")
	c.Categories.Add("42", indexer.MoviesUHD, "Filme / 2160p")
	c.Categories.Add("9", indexer.MoviesHD, "Filme / 1080p")
	c.Categories.Add("20", indexer.MoviesHD, "Filme / 720p")
	c.Categories.Add("10", indexer.MoviesSD, "Filme / SD")
	c.Categories.Add("43", indexer.TVUHD, "Serien / 2160p")
	c.Categories.Add("53", indexer.TVHD, "Serien / 1080p")
	c.Categories.Add("54", indexer.TVHD, "Serien / 720p")
	c.Categories.Add("15", indexer.TVSD, "Serien / SD")
	c.Categories.Add("30", indexer.TVSport, "Serien / Sport")
	c.Categories.Add("44", indexer.TVUHD, "Serienpacks / 2160p")
	c.Categories.Add("55", indexer.TVHD, "Serienpacks / 1080p")
	c.Categories.Add("56", indexer.TVHD, "Serienpacks / 720p")
	c.Categories.Add("27", indexer.TVSD, "Serienpacks / SD")
	c.Categories.Add("24", indexer.AudioLossless, "Audio / Musik / FLAC")
	c.Categories.Add("25", indexer.AudioMP3, "Audio / Musik / MP3")
	c.Categories.Add("35", indexer.AudioOther, "Audio / Other")
	c.Categories.Add("18", indexer.AudioAudiobook, "Audio / aBooks")
	c.Categories.Add("33", indexer.AudioVideo, "Audio / Videos")
	c.Categories.Add("17", indexer.Books, "Misc / eBooks")
	c.Categories.Add("5", indexer.PCMobileOther, "Misc / Mobile")
	c.Categories.Add("39", indexer.Other, "Misc / Bildung")
	c.Categories.Add("36", indexer.TVForeign, "Englisch / Serien")
	c.Categories.Add("57", indexer.TVForeign, "Englisch / Serienpacks")
	c.Categories.Add("37", indexer.MoviesForeign, "Englisch / Filme")
	c.Categories.Add("47", indexer.Books, "Englisch / eBooks")
	c.Categories.Add("48", indexer.Other, "Englisch / Bildung")
	c.Categories.Add("49", indexer.TVSport, "Englisch / Sport")
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

// seasonOnly is the first season without an episode, such as "s01",
// in a term.
var seasonOnly = regexp.MustCompile(`(?i)(^|\s)(s\d{2})(\s|$)`)

// SearchRequests implements indexer.Site: one search of the API, of 50
// releases.
func (s *Site) SearchRequests(_ context.Context, _ *indexer.Indexer, q *indexer.Query) (*indexer.Chain, error) {
	var term, imdb string
	switch q.Type {
	case indexer.SearchMovie:
		term, imdb = q.SanitizedTerm(), q.IMDbID
	case indexer.SearchTV:
		term, imdb = q.SanitizedTVString(), q.IMDbID
	default:
		term = q.SanitizedTerm()
	}
	var p indexer.Params
	p.Set("apikey", s.settings.APIKey)
	p.Add("limit", "50")
	p.Add("ponly", strconv.FormatBool(s.settings.ProductsOnly))
	for _, t := range s.settings.ReleaseTypes {
		p.Add("release_type", t.String())
	}
	switch {
	case imdb != "":
		p.Add("imdbId", imdb)
	case strings.TrimSpace(term) != "":
		// Searches of whole seasons find their single episodes too.
		p.Add("searchstring", seasonOnly.ReplaceAllString(strings.TrimSpace(term), "${1}${2}*${3}"))
	}
	if cats := strings.Join(s.caps.Categories.MapToTracker(q.Categories, false), ","); cats != "" {
		p.Add("cats", cats)
	}
	r := indexer.NewRequest(strings.TrimRight(s.baseURL, "/")+"/api_9djWe8Tb2NE3p6opyqnh/v1/browse.php?"+p.Encode(), indexer.AcceptHTML)
	r.FollowRedirects = true
	var c indexer.Chain
	c.AddRequests(r)
	return &c, nil
}

// row is a torrent of the API.
type row struct {
	ID          jsonx.String   `json:"id"`
	Name        jsonx.String   `json:"name"`
	Category    jsonx.Int      `json:"category"`
	Added       jsonx.Int64    `json:"added"`
	Size        jsonx.Int64    `json:"size"`
	NumFiles    jsonx.Int      `json:"numfiles"`
	Seeders     jsonx.Int      `json:"seeders"`
	Leechers    jsonx.Int      `json:"leechers"`
	Snatched    jsonx.Int      `json:"snatched"`
	IMDbID      *jsonx.Int     `json:"imdbId"`
	TVDBID      *jsonx.Int     `json:"tvdbId"`
	TMDBID      *jsonx.Int     `json:"tmdbId"`
	Poster      jsonx.String   `json:"poster"`
	Title       jsonx.String   `json:"title"`
	TitleOrigin jsonx.String   `json:"title_origin"`
	Year        *jsonx.Int     `json:"year"`
	PID         *jsonx.Int     `json:"pid"`
	ReleaseType jsonx.String   `json:"release_type"`
	Tags        []jsonx.String `json:"tags"`
	Genres      []jsonx.String `json:"genres"`
}

// ParseResponse implements indexer.Site, as Prowlarr's
// TorrentSyndikatParser does.
func (s *Site) ParseResponse(_ context.Context, _ *indexer.Indexer, r *indexer.Response) ([]*indexer.Release, error) {
	var resp struct {
		Rows []row `json:"rows"`
	}
	if err := json.Unmarshal([]byte(r.Content()), &resp); err != nil {
		return nil, fmt.Errorf("torrentsyndikat: %w", err)
	}
	if resp.Rows == nil {
		return nil, errors.New("torrentsyndikat: the response has no rows")
	}
	releases := make([]*indexer.Release, 0, len(resp.Rows))
	for _, row := range resp.Rows {
		releases = append(releases, s.release(row))
	}
	return releases, nil
}

func (s *Site) release(row row) *indexer.Release {
	details := s.baseURL + "details.php?id=" + string(row.ID)
	seeders := int(row.Seeders)
	rel := &indexer.Release{
		MinimumRatio:         indexer.Ptr(1.0),
		MinimumSeedTime:      indexer.Ptr(int64(96 * 60 * 60)),
		DownloadVolumeFactor: indexer.Ptr(1.0),
		UploadVolumeFactor:   indexer.Ptr(1.0),
		GUID:                 details,
		InfoURL:              details,
		DownloadURL:          s.baseURL + "download.php?id=" + string(row.ID) + "&apikey=" + s.settings.APIKey,
		Title:                string(row.Name),
		Categories:           indexer.IDs(s.caps.Categories.MapTracker(strconv.Itoa(int(row.Category)))),
		PublishDate:          time.Unix(int64(row.Added), 0),
		Size:                 indexer.Ptr(int64(row.Size)),
		Files:                indexer.Ptr(int(row.NumFiles)),
		Seeders:              &seeders,
		Peers:                indexer.Ptr(seeders + int(row.Leechers)),
		Grabs:                indexer.Ptr(int(row.Snatched)),
		IMDbID:               value(row.IMDbID),
		TVDBID:               value(row.TVDBID),
		TMDBID:               value(row.TMDBID),
	}
	if poster := string(row.Poster); strings.TrimSpace(poster) != "" {
		rel.PosterURL = s.baseURL + poster[1:]
	}
	var lines []string
	if strings.TrimSpace(string(row.Title)) != "" {
		lines = append(lines, "Title: "+string(row.Title))
	}
	if strings.TrimSpace(string(row.TitleOrigin)) != "" {
		lines = append(lines, "Original Title: "+string(row.TitleOrigin))
	}
	if year := value(row.Year); year > 0 {
		lines = append(lines, "Year: "+strconv.Itoa(year))
	}
	if pid := value(row.PID); pid > 0 {
		lines = append(lines, "Product-Link: "+s.baseURL+"product.php?pid="+strconv.Itoa(pid))
	}
	if len(row.Genres) > 0 {
		lines = append(lines, "Genres: "+join(row.Genres))
	}
	if len(row.Tags) > 0 {
		lines = append(lines, "Tags: "+join(row.Tags))
	}
	if strings.TrimSpace(string(row.ReleaseType)) != "" {
		lines = append(lines, "Release Type: "+string(row.ReleaseType))
	}
	rel.Description = strings.Join(lines, "\n")
	return rel
}

// value returns a nullable integer's value, 0 for null.
func value(n *jsonx.Int) int {
	if n == nil {
		return 0
	}
	return int(*n)
}

func join(values []jsonx.String) string {
	s := make([]string, len(values))
	for i, v := range values {
		s[i] = string(v)
	}
	return strings.Join(s, ", ")
}
