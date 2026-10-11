package filelist

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
	"github.com/mavioai/mavio/plugins/indexers/internal/jsonx"
)

var (
	links  = []string{"https://filelist.io/", "https://thefl.org/"}
	legacy = []string{"https://filelist.io", "https://filelist.ro/", "http://filelist.ro/", "http://flro.org/", "https://flro.org/"}
)

// Definition is FileList.io.
var Definition = indexer.Definition{
	ID:          "filelist",
	Name:        "FileList.io",
	Description: "FileList (FL) is a ROMANIAN Private Torrent Tracker for 0DAY / GENERAL",
	Protocol:    indexer.ProtocolTorrent,
	Privacy:     indexer.Private,
	Links:       links,
	LegacyLinks: legacy,
	Fields: []indexer.Field{
		indexer.BaseURLField(links),
		{Name: "username", Label: "Username", Type: indexer.FieldText, HelpText: "Site username"},
		{Name: "passkey", Label: "Passkey", Type: indexer.FieldText, Secret: true, HelpText: "Site Passkey"},
		{Name: "freeleech_only", Label: "Freeleech Only", Type: indexer.FieldCheckbox, HelpText: "Search freeleech releases only"},
	},
	// The API takes 150 calls an hour.
	Limits: indexer.Limits{Queries: 150, Per: time.Hour},
	New:    newSite,
}

// Settings are FileList.io's settings.
type Settings struct {
	indexer.BaseSettings
	Username      string `json:"username,omitempty"`
	Passkey       string `json:"passkey,omitempty"`
	FreeleechOnly bool   `json:"freeleech_only,omitempty"`
}

// Validate checks that the user name and passkey are set.
func (s Settings) Validate() error {
	var errs []error
	if strings.TrimSpace(s.Username) == "" {
		errs = append(errs, errors.New("username: must not be empty"))
	}
	if strings.TrimSpace(s.Passkey) == "" {
		errs = append(errs, errors.New("passkey: must not be empty"))
	}
	return errors.Join(errs...)
}

// Site is FileList.io.
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
	c.Params[indexer.SearchTV] = []indexer.Param{indexer.ParamQ, indexer.ParamIMDbID, indexer.ParamSeason, indexer.ParamEp}
	c.Params[indexer.SearchMovie] = []indexer.Param{indexer.ParamQ, indexer.ParamIMDbID}
	c.Params[indexer.SearchMusic] = []indexer.Param{indexer.ParamQ}
	c.Params[indexer.SearchBook] = []indexer.Param{indexer.ParamQ}
	c.Flags = []indexer.Flag{indexer.FlagInternal, indexer.FlagFreeleech}
	c.Categories.Add("24", indexer.TVAnime, "Anime")
	c.Categories.Add("11", indexer.Audio, "Music")
	c.Categories.Add("29", indexer.Books, "Books")
	c.Categories.Add("30", indexer.Other, "Courses")
	c.Categories.Add("15", indexer.TV, "Cartoons")
	c.Categories.Add("18", indexer.Other, "Various")
	c.Categories.Add("16", indexer.Books, "Docs")
	c.Categories.Add("25", indexer.Movies3D, "Movies 3D")
	c.Categories.Add("6", indexer.MoviesUHD, "Movies 4K")
	c.Categories.Add("26", indexer.MoviesBluRay, "Movies 4K Blu-Ray")
	c.Categories.Add("20", indexer.MoviesBluRay, "Movies Blu-Ray")
	c.Categories.Add("2", indexer.MoviesDVD, "Movies DVD")
	c.Categories.Add("3", indexer.MoviesForeign, "Movies DVD-RO")
	c.Categories.Add("4", indexer.MoviesHD, "Movies HD")
	c.Categories.Add("19", indexer.MoviesForeign, "Movies HD-RO")
	c.Categories.Add("1", indexer.MoviesSD, "Movies SD")
	c.Categories.Add("5", indexer.AudioLossless, "FLAC")
	c.Categories.Add("10", indexer.Console, "Games Console")
	c.Categories.Add("9", indexer.PCGames, "Games PC")
	c.Categories.Add("31", indexer.TVForeign, "K-Drama")
	c.Categories.Add("17", indexer.PC, "Linux")
	c.Categories.Add("22", indexer.PCMobileOther, "Mobile")
	c.Categories.Add("8", indexer.PC, "Apps")
	c.Categories.Add("28", indexer.MoviesForeign, "RO Dubbed")
	c.Categories.Add("28", indexer.TVForeign, "RO Dubbed")
	c.Categories.Add("27", indexer.TVUHD, "TV-Series 4K")
	c.Categories.Add("21", indexer.TVHD, "TV-Series HD")
	c.Categories.Add("23", indexer.TVSD, "TV-Series SD")
	c.Categories.Add("13", indexer.TVSport, "Sport")
	c.Categories.Add("12", indexer.AudioVideo, "Videoclips")
	c.Categories.Add("7", indexer.XXX, "XXX")
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

// SearchRequests implements indexer.Site, as Prowlarr's
// FileListRequestGenerator does.
func (s *Site) SearchRequests(_ context.Context, _ *indexer.Indexer, q *indexer.Query) (*indexer.Chain, error) {
	var p indexer.Params
	var c indexer.Chain
	term := strings.TrimSpace(q.SanitizedTerm())
	hasTerm := strings.TrimSpace(q.Term) != ""
	imdb := ""
	if q.Type == indexer.SearchTV || q.Type == indexer.SearchMovie {
		imdb = strings.TrimSpace(q.IMDbID)
	}
	switch {
	case q.Type == indexer.SearchTV && (imdb != "" || hasTerm):
		p.Set("action", "search-torrents")
		if d, ok := dailyDate(q); ok {
			if imdb != "" {
				// Daily episodes are not searched by IDs.
				return &c, nil
			}
			term = strings.TrimSpace(term + " " + d.Format("2006.01.02"))
		} else {
			if q.Season != nil {
				p.Set("season", strconv.Itoa(*q.Season))
			}
			if strings.TrimSpace(q.Episode) != "" {
				p.Set("episode", q.Episode)
			}
		}
		if imdb != "" {
			p.Set("type", "imdb")
			p.Set("query", q.FullIMDbID())
		} else {
			p.Set("type", "name")
			p.Set("query", term)
		}
	case imdb != "":
		p.Set("action", "search-torrents")
		p.Set("type", "imdb")
		p.Set("query", q.FullIMDbID())
	case hasTerm:
		p.Set("action", "search-torrents")
		p.Set("type", "name")
		p.Set("query", term)
	default:
		p.Set("action", "latest-torrents")
	}
	if len(q.Categories) > 0 {
		p.Set("category", strings.Join(s.caps.Categories.MapToTracker(q.Categories, false), ","))
	}
	if s.settings.FreeleechOnly {
		p.Set("freeleech", "1")
	}
	r := indexer.NewRequest(strings.TrimRight(s.baseURL, "/")+"/api.php?"+p.Encode(), indexer.AcceptJSON)
	// Prowlarr's IndexerRequest of a URL follows redirects.
	r.FollowRedirects = true
	auth := strings.TrimSpace(s.settings.Username) + ":" + strings.TrimSpace(s.settings.Passkey)
	r.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(auth)))
	c.AddRequests(r)
	return &c, nil
}

// dailyDate returns the date of a daily episode, whose season is a year
// and episode a month and day.
func dailyDate(q *indexer.Query) (time.Time, bool) {
	if q.Season == nil {
		return time.Time{}, false
	}
	d, err := time.Parse("2006 01/02", fmt.Sprintf("%d %s", *q.Season, q.Episode))
	return d, err == nil
}

// CleanupReleases implements indexer.ReleaseCleaner: the site searches
// loosely, so releases without the query's terms are dropped.
func (s *Site) CleanupReleases(releases []*indexer.Release, q *indexer.Query) []*indexer.Release {
	return indexer.FilterByQuery(releases, q)
}

// torrent is a release of a search.
type torrent struct {
	ID               jsonx.Int64  `json:"id"`
	Name             jsonx.String `json:"name"`
	Size             jsonx.Int64  `json:"size"`
	Leechers         jsonx.Int    `json:"leechers"`
	Seeders          jsonx.Int    `json:"seeders"`
	TimesCompleted   jsonx.Int    `json:"times_completed"`
	Files            jsonx.Int    `json:"files"`
	IMDbID           jsonx.String `json:"imdb"`
	Internal         jsonx.Bool   `json:"internal"`
	Freeleech        jsonx.Bool   `json:"freeleech"`
	DoubleUp         jsonx.Bool   `json:"doubleup"`
	UploadDate       jsonx.String `json:"upload_date"`
	Category         jsonx.String `json:"category"`
	SmallDescription jsonx.String `json:"small_description"`
}

// romania is the zone of the site's dates, which have none.
var romania = time.FixedZone("+0300", 3*60*60)

// ParseResponse implements indexer.Site, as Prowlarr's FileListParser
// does.
func (s *Site) ParseResponse(_ context.Context, _ *indexer.Indexer, r *indexer.Response) ([]*indexer.Release, error) {
	if r.StatusCode != http.StatusOK {
		return nil, indexer.Errorf("Unexpected response status %d code from indexer request", r.StatusCode)
	}
	content := r.Content()
	if strings.HasPrefix(content, `{"error"`) {
		var e struct {
			Error string `json:"error"`
		}
		if json.Unmarshal([]byte(content), &e) == nil {
			return nil, indexer.Errorf("Unexpected response from indexer request: %s", e.Error)
		}
	}
	if !r.HasContentType(indexer.AcceptJSON) {
		return nil, indexer.Errorf("Unexpected response header %s from indexer request, expected %s", r.Header.Get("Content-Type"), indexer.AcceptJSON)
	}
	var rows []torrent
	if err := json.Unmarshal([]byte(content), &rows); err != nil {
		return nil, fmt.Errorf("filelist: %w", err)
	}
	releases := make([]*indexer.Release, 0, len(rows))
	for _, row := range rows {
		if s.settings.FreeleechOnly && !bool(row.Freeleech) {
			continue
		}
		rel, err := s.release(&row)
		if err != nil {
			return nil, err
		}
		releases = append(releases, rel)
	}
	return releases, nil
}

func (s *Site) release(row *torrent) (*indexer.Release, error) {
	date, err := time.ParseInLocation(time.DateTime, strings.TrimSpace(string(row.UploadDate)), romania)
	if err != nil {
		return nil, fmt.Errorf("filelist: %w", err)
	}
	id := strconv.FormatInt(int64(row.ID), 10)
	download := indexer.AddQueryParam(indexer.AddQueryParam(indexer.CombinePath(s.baseURL, "/download.php"), "id", id), "passkey", strings.TrimSpace(s.settings.Passkey))
	rel := &indexer.Release{
		GUID:                 "FileList-" + id,
		Title:                string(row.Name),
		Size:                 indexer.Ptr(int64(row.Size)),
		Categories:           indexer.IDs(s.caps.Categories.MapTrackerDesc(string(row.Category))),
		DownloadURL:          download,
		InfoURL:              indexer.AddQueryParam(indexer.CombinePath(s.baseURL, "/details.php"), "id", id),
		Protocol:             indexer.ProtocolTorrent,
		Seeders:              indexer.Ptr(int(row.Seeders)),
		Peers:                indexer.Ptr(int(row.Leechers + row.Seeders)),
		PublishDate:          date.UTC(),
		Description:          string(row.SmallDescription),
		Files:                indexer.Ptr(int(row.Files)),
		Grabs:                indexer.Ptr(int(row.TimesCompleted)),
		DownloadVolumeFactor: indexer.Ptr(1.0),
		UploadVolumeFactor:   indexer.Ptr(1.0),
		MinimumRatio:         indexer.Ptr(1.0),
		MinimumSeedTime:      indexer.Ptr(int64(172800)), // 48 hours
	}
	for g := range strings.SplitSeq(string(row.SmallDescription), ",") {
		if g = strings.TrimSpace(g); g != "" {
			rel.Genres = append(rel.Genres, g)
		}
	}
	if imdb := string(row.IMDbID); len(imdb) > 2 {
		if n, err := strconv.Atoi(strings.TrimLeft(imdb, "t")); err == nil {
			rel.IMDbID = n
		}
	}
	if bool(row.Internal) {
		rel.Flags = append(rel.Flags, indexer.FlagInternal)
	}
	if bool(row.Freeleech) {
		rel.DownloadVolumeFactor = indexer.Ptr(0.0)
	}
	if bool(row.DoubleUp) {
		rel.UploadVolumeFactor = indexer.Ptr(2.0)
	}
	return rel, nil
}
