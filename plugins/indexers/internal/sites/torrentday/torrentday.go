package torrentday

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
	"github.com/mavioai/mavio/plugins/indexers/internal/jsonx"
	"github.com/mavioai/mavio/plugins/indexers/internal/parse"
)

var links = []string{
	"https://tday.love/",
	"https://torrentday.cool/",
	"https://secure.torrentday.com/",
	"https://classic.torrentday.com/",
	"https://www.torrentday.com/",
	"https://www.torrentday.me/",
	"https://torrentday.it/",
	"https://td.findnemo.net/",
	"https://td.getcrazy.me/",
	"https://td.venom.global/",
	"https://td.workisboring.net/",
	"https://tday.findnemo.net/",
	"https://tday.getcrazy.me/",
	"https://tday.venom.global/",
	"https://tday.workisboring.net/",
}

// Definition is TorrentDay.
var Definition = indexer.Definition{
	ID:          "torrentday",
	Name:        "TorrentDay",
	Description: "TorrentDay (TD) is a Private site for TV / MOVIES / GENERAL",
	Protocol:    indexer.ProtocolTorrent,
	Privacy:     indexer.Private,
	Links:       links,
	Fields: []indexer.Field{
		indexer.BaseURLField(links),
		{Name: "cookie", Label: "Cookie", Type: indexer.FieldText, Secret: true, HelpText: "Site Cookie"},
		{Name: "freeleech_only", Label: "FreeLeech Only", Type: indexer.FieldCheckbox, HelpText: "Search Freeleech torrents only"},
	},
	New: newSite,
}

// Settings are TorrentDay's settings.
type Settings struct {
	indexer.BaseSettings
	Cookie        string `json:"cookie,omitempty"`
	FreeleechOnly bool   `json:"freeleech_only,omitempty"`
}

// Validate checks that the cookie is set.
func (s Settings) Validate() error {
	if strings.TrimSpace(s.Cookie) == "" {
		return errors.New("cookie: must not be empty")
	}
	return nil
}

// Site is TorrentDay.
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

	c.Categories.Add("25", indexer.MoviesSD, "Movies/480p")
	c.Categories.Add("96", indexer.MoviesUHD, "Movie/4K")
	c.Categories.Add("11", indexer.MoviesBluRay, "Movies/Bluray")
	c.Categories.Add("5", indexer.MoviesBluRay, "Movies/Bluray-Full")
	c.Categories.Add("103", indexer.MoviesSD, "Movies/Cam")
	c.Categories.Add("3", indexer.MoviesDVD, "Movies/DVD-R")
	c.Categories.Add("21", indexer.MoviesSD, "Movies/MP4")
	c.Categories.Add("22", indexer.MoviesForeign, "Movies/Non-English")
	c.Categories.Add("13", indexer.Movies, "Movies/Packs")
	c.Categories.Add("44", indexer.MoviesSD, "Movies/SD/x264")
	c.Categories.Add("48", indexer.Movies, "Movies/x265")
	c.Categories.Add("1", indexer.MoviesSD, "Movies/XviD")

	c.Categories.Add("24", indexer.TVSD, "TV/480p")
	c.Categories.Add("104", indexer.TVUHD, "TV/4K")
	c.Categories.Add("32", indexer.TVHD, "TV/Bluray")
	c.Categories.Add("31", indexer.TVSD, "TV/DVD-R")
	c.Categories.Add("33", indexer.TVSD, "TV/DVD-Rip")
	c.Categories.Add("46", indexer.TVSD, "TV/Mobile")
	c.Categories.Add("82", indexer.TVForeign, "TV/Non-English")
	c.Categories.Add("14", indexer.TV, "TV/Packs")
	c.Categories.Add("26", indexer.TVSD, "TV/SD/x264")
	c.Categories.Add("7", indexer.TVHD, "TV/x264")
	c.Categories.Add("34", indexer.TVx265, "TV/x265")
	c.Categories.Add("2", indexer.TVSD, "TV/XviD")

	c.Categories.Add("10", indexer.ConsoleNDS, "Nintendo")
	c.Categories.Add("4", indexer.PCGames, "PC/Games")
	c.Categories.Add("18", indexer.ConsolePS3, "PS")
	c.Categories.Add("8", indexer.ConsolePSP, "PSP")
	c.Categories.Add("9", indexer.ConsoleXBox, "Xbox")

	c.Categories.Add("17", indexer.AudioMP3, "Music/Audio")
	c.Categories.Add("27", indexer.Audio, "Music/Flac")
	c.Categories.Add("23", indexer.AudioForeign, "Music/Non-English")
	c.Categories.Add("41", indexer.Audio, "Music/Packs")
	c.Categories.Add("16", indexer.AudioVideo, "Music/Video")

	c.Categories.Add("29", indexer.TVAnime, "Anime")
	c.Categories.Add("42", indexer.AudioAudiobook, "Audio Books")
	c.Categories.Add("20", indexer.Books, "Books")
	c.Categories.Add("102", indexer.BooksForeign, "Books/Non-English")
	c.Categories.Add("30", indexer.TVDocumentary, "Documentary")
	c.Categories.Add("95", indexer.TVDocumentary, "Educational")
	c.Categories.Add("47", indexer.Other, "Fonts")
	c.Categories.Add("43", indexer.PCMac, "Mac")
	c.Categories.Add("45", indexer.AudioOther, "Podcast")
	c.Categories.Add("28", indexer.PC, "Softwa/Packs")
	c.Categories.Add("12", indexer.PC, "Software")

	c.Categories.Add("19", indexer.XXX, "XXX/0Day")
	c.Categories.Add("6", indexer.XXX, "XXX/Movies")
	c.Categories.Add("15", indexer.XXXPack, "XXX/Packs")
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
// TorrentDayRequestGenerator does: the site's JSON listing, of all its
// categories when the query has none of them.
func (s *Site) SearchRequests(_ context.Context, _ *indexer.Indexer, q *indexer.Query) (*indexer.Chain, error) {
	term, imdb := q.Term, ""
	switch q.Type {
	case indexer.SearchMovie:
		imdb = q.FullIMDbID()
	case indexer.SearchTV:
		term, imdb = q.SanitizedTVString(), q.FullIMDbID()
	}
	cats := s.caps.Categories.MapToTracker(q.Categories, false)
	if len(cats) == 0 {
		cats = s.caps.Categories.TrackerCategories()
	}
	u := s.baseURL + "t.json?" + strings.Join(cats, ";")
	if s.settings.FreeleechOnly {
		u += ";free"
	}
	u += ";q="
	if strings.TrimSpace(imdb) != "" {
		u += imdb + indexer.URLEncode(" ")
	}
	u += indexer.URLEncode(term)
	r := indexer.NewRequest(u, indexer.AcceptRSS)
	r.Cookies = s.cookies()
	var c indexer.Chain
	c.AddRequests(r)
	return &c, nil
}

// cookies returns the cookies of the settings, which every request
// carries.
func (s *Site) cookies() map[string]string {
	return indexer.ParseCookieHeader(s.settings.Cookie)
}

// DownloadRequest implements indexer.DownloadSite: downloads carry the
// settings' cookies.
func (s *Site) DownloadRequest(_ context.Context, _ *indexer.Indexer, link string) (*indexer.Request, error) {
	r := indexer.NewRequest(link, "")
	r.Cookies = s.cookies()
	return r, nil
}

// row is a release of a listing.
type row struct {
	Name               jsonx.String `json:"name"`
	ID                 jsonx.Int64  `json:"t"`
	Category           jsonx.String `json:"c"`
	Seeders            jsonx.Int    `json:"seeders"`
	Leechers           jsonx.Int    `json:"leechers"`
	IMDbID             jsonx.String `json:"imdb-id"`
	DownloadMultiplier *jsonx.Float `json:"download-multiplier"`
	CTime              jsonx.Int64  `json:"ctime"`
	Size               jsonx.Int64  `json:"size"`
	Files              jsonx.Int    `json:"files"`
	Completed          jsonx.Int    `json:"completed"`
}

// ParseResponse implements indexer.Site, as Prowlarr's TorrentDayParser
// does. Redirects are to the login page when the cookie expired.
func (s *Site) ParseResponse(ctx context.Context, _ *indexer.Indexer, r *indexer.Response) ([]*indexer.Release, error) {
	if r.IsRedirect() {
		to := r.RedirectURL()
		slog.WarnContext(ctx, "redirected from indexer request", "url", to)
		if strings.Contains(strings.ToLower(to), "/login.php") {
			return nil, indexer.Errorf("We are being redirected to the login page. Most likely your session expired or was killed. Recheck your cookie and try testing the indexer.")
		}
		return nil, indexer.Errorf("Redirected to %s from indexer request", to)
	}
	var rows []row
	if err := json.Unmarshal([]byte(r.Content()), &rows); err != nil {
		return nil, fmt.Errorf("torrentday: %w", err)
	}
	releases := make([]*indexer.Release, 0, len(rows))
	for _, row := range rows {
		id := strconv.FormatInt(int64(row.ID), 10)
		details := s.baseURL + "details.php?id=" + id
		factor := 1.0
		if row.DownloadMultiplier != nil {
			factor = float64(*row.DownloadMultiplier)
		}
		rel := &indexer.Release{
			Title:                string(row.Name),
			GUID:                 details,
			DownloadURL:          s.baseURL + "download.php/" + id + "/" + id + ".torrent",
			InfoURL:              details,
			PublishDate:          time.Unix(int64(row.CTime), 0).UTC(),
			Categories:           indexer.IDs(s.caps.Categories.MapTracker(string(row.Category))),
			Size:                 indexer.Ptr(int64(row.Size)),
			Files:                indexer.Ptr(int(row.Files)),
			Grabs:                indexer.Ptr(int(row.Completed)),
			Seeders:              indexer.Ptr(int(row.Seeders)),
			Peers:                indexer.Ptr(int(row.Seeders + row.Leechers)),
			DownloadVolumeFactor: indexer.Ptr(factor),
			UploadVolumeFactor:   indexer.Ptr(1.0),
			MinimumRatio:         indexer.Ptr(1.0),
			MinimumSeedTime:      indexer.Ptr(int64(259200)), // 72 hours
		}
		if id, ok := parse.IMDbID(string(row.IMDbID)); ok {
			rel.IMDbID = id
		}
		releases = append(releases, rel)
	}
	return releases, nil
}
