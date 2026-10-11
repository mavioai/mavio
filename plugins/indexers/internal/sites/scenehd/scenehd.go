package scenehd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
	"github.com/mavioai/mavio/plugins/indexers/internal/jsonx"
	"github.com/mavioai/mavio/plugins/indexers/internal/parse"
)

var links = []string{"https://scenehd.org/"}

// Definition is SceneHD.
var Definition = indexer.Definition{
	ID:          "scenehd",
	Name:        "SceneHD",
	Description: "SceneHD is Private site for HD TV / MOVIES",
	Language:    "en-US",
	Protocol:    indexer.ProtocolTorrent,
	Privacy:     indexer.Private,
	Links:       links,
	Fields: []indexer.Field{
		indexer.BaseURLField(links),
		{Name: "passkey", Label: "Passkey", HelpText: "Site Passkey", Type: indexer.FieldText},
	},
	New: newSite,
}

// Settings are SceneHD's settings.
type Settings struct {
	indexer.BaseSettings
	Passkey string `json:"passkey,omitempty"`
}

// Validate checks that the passkey has 32 characters.
func (s Settings) Validate() error {
	if strings.TrimSpace(s.Passkey) == "" {
		return errors.New("passkey: must not be empty")
	}
	if n := len([]rune(s.Passkey)); n != 32 {
		return fmt.Errorf("passkey: must be 32 characters long, not %d", n)
	}
	return nil
}

// Site is SceneHD.
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
	c.Categories.Add("2", indexer.MoviesUHD, "Movie/2160")
	c.Categories.Add("1", indexer.MoviesHD, "Movie/1080")
	c.Categories.Add("4", indexer.MoviesHD, "Movie/720")
	c.Categories.Add("8", indexer.MoviesBluRay, "Movie/BD5/9")
	c.Categories.Add("6", indexer.TVUHD, "TV/2160")
	c.Categories.Add("5", indexer.TVHD, "TV/1080")
	c.Categories.Add("7", indexer.TVHD, "TV/720")
	c.Categories.Add("22", indexer.MoviesBluRay, "Bluray/Complete")
	c.Categories.Add("10", indexer.XXX, "XXX")
	c.Categories.Add("16", indexer.MoviesOther, "Subpacks")
	c.Categories.Add("13", indexer.AudioVideo, "MVID")
	c.Categories.Add("9", indexer.Other, "Other")
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

// SearchRequests implements indexer.Site: one search of the API, by the
// IMDb ID and the term.
func (s *Site) SearchRequests(_ context.Context, _ *indexer.Indexer, q *indexer.Query) (*indexer.Chain, error) {
	var term, imdb string
	switch q.Type {
	case indexer.SearchMovie:
		term, imdb = q.SanitizedTerm(), q.FullIMDbID()
	case indexer.SearchTV:
		term, imdb = q.SanitizedTVString(), q.FullIMDbID()
	default:
		term = q.SanitizedTerm()
	}
	var search []string
	for _, v := range []string{imdb, term} {
		if strings.TrimSpace(v) != "" {
			search = append(search, v)
		}
	}
	var p indexer.Params
	p.Set("api", "")
	p.Set("passkey", s.settings.Passkey)
	p.Set("search", strings.Join(search, " "))
	if len(q.Categories) > 0 {
		var cats []string
		for _, c := range s.caps.Categories.MapToTracker(q.Categories, false) {
			if !slices.Contains(cats, c) {
				cats = append(cats, c)
			}
		}
		p.Set("cat", strings.Join(cats, ","))
	}
	r := indexer.NewRequest(strings.TrimRight(s.baseURL, "/")+"/browse.php?"+p.Encode(), indexer.AcceptJSON)
	r.FollowRedirects = true
	var c indexer.Chain
	c.AddRequests(r)
	return &c, nil
}

// torrent is a torrent of the API.
type torrent struct {
	ID             jsonx.Int64  `json:"id"`
	Name           jsonx.String `json:"name"`
	Added          jsonx.String `json:"added"`
	IsFreeleech    jsonx.Int    `json:"is_freeleech"`
	Category       jsonx.String `json:"category"`
	Size           jsonx.Int64  `json:"size"`
	TimesCompleted jsonx.Int    `json:"times_completed"`
	NumFiles       jsonx.Int    `json:"numfiles"`
	Seeders        jsonx.Int    `json:"seeders"`
	Leechers       jsonx.Int    `json:"leechers"`
	IMDbID         jsonx.String `json:"imdbid"`
}

// ParseResponse implements indexer.Site, as Prowlarr's SceneHDParser
// does.
func (s *Site) ParseResponse(_ context.Context, _ *indexer.Indexer, r *indexer.Response) ([]*indexer.Release, error) {
	content := r.Content()
	if strings.Contains(content, "User not found or passkey not set") {
		return nil, fmt.Errorf("%w: %s", indexer.ErrAuth, "The passkey is invalid. Check the indexer configuration.")
	}
	var torrents []torrent
	if err := json.Unmarshal([]byte(content), &torrents); err != nil {
		return nil, fmt.Errorf("scenehd: %w", err)
	}
	releases := make([]*indexer.Release, 0, len(torrents))
	for _, t := range torrents {
		id := strconv.FormatInt(int64(t.ID), 10)
		details := absoluteURI(s.baseURL + "details.php?id=" + id)
		date, err := time.ParseInLocation(time.DateTime, string(t.Added), time.Local)
		if err != nil {
			return nil, fmt.Errorf("scenehd: %w", err)
		}
		dvf := 1.0
		if t.IsFreeleech == 1 {
			dvf = 0
		}
		rel := &indexer.Release{
			Title:                string(t.Name),
			DownloadURL:          absoluteURI(s.baseURL + "download.php?id=" + id + "&passkey=" + s.settings.Passkey),
			InfoURL:              details,
			GUID:                 details,
			Categories:           indexer.IDs(s.caps.Categories.MapTracker(string(t.Category))),
			PublishDate:          date,
			Size:                 indexer.Ptr(int64(t.Size)),
			Grabs:                indexer.Ptr(int(t.TimesCompleted)),
			Files:                indexer.Ptr(int(t.NumFiles)),
			Seeders:              indexer.Ptr(int(t.Seeders)),
			Peers:                indexer.Ptr(int(t.Leechers) + int(t.Seeders)),
			MinimumRatio:         indexer.Ptr(1.0),
			MinimumSeedTime:      indexer.Ptr(int64(0)),
			DownloadVolumeFactor: &dvf,
			UploadVolumeFactor:   indexer.Ptr(1.0),
		}
		if id, ok := parse.IMDbID(string(t.IMDbID)); ok {
			rel.IMDbID = id
		}
		releases = append(releases, rel)
	}
	return releases, nil
}

// absoluteURI normalizes a URL as .NET's Uri.AbsoluteUri does, escaping
// what must be; the URL as it is when it does not parse.
func absoluteURI(s string) string {
	u, err := url.Parse(s)
	if err != nil {
		return s
	}
	return u.String()
}
