package mteamtp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
	"github.com/mavioai/mavio/plugins/indexers/internal/jsonx"
)

var links = []string{
	"https://kp.m-team.cc/",
	"https://tp.m-team.cc/",
	"https://pt.m-team.cc/",
}

// UserAgent is the User-Agent the site's API is sent.
const UserAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/87.0.4280.88 Safari/537.35"

// Definition is M-Team - TP.
var Definition = indexer.Definition{
	ID:          "mteamtp",
	Name:        "M-Team - TP",
	Description: "M-Team TP (MTTP) is a CHINESE Private Torrent Tracker for HD MOVIES / TV / 3X",
	Language:    "zh-CN",
	Protocol:    indexer.ProtocolTorrent,
	Privacy:     indexer.Private,
	Links:       links,
	Fields: []indexer.Field{
		indexer.BaseURLField(links),
		{Name: "api_key", Label: "API Key", Type: indexer.FieldText, Secret: true, HelpText: "API Key from the Site (Found in User Control Panel => Laboratory)"},
		{Name: "freeleech_only", Label: "Freeleech Only", Type: indexer.FieldCheckbox, HelpText: "Search freeleech releases only"},
	},
	New: newSite,
}

// Settings are M-Team - TP's settings.
type Settings struct {
	indexer.BaseSettings
	APIKey        string `json:"api_key,omitempty"`
	FreeleechOnly bool   `json:"freeleech_only,omitempty"`
}

// Validate checks that the API key is set.
func (s Settings) Validate() error {
	if strings.TrimSpace(s.APIKey) == "" {
		return errors.New("api_key: must not be empty")
	}
	return nil
}

// Site is M-Team - TP.
type Site struct {
	settings Settings
	baseURL  string
	// apiURL is the address of the site's API.
	apiURL string
	caps   *indexer.Capabilities
}

func newSite(data []byte) (indexer.Site, error) {
	var settings Settings
	if err := indexer.DecodeSettings(data, &settings); err != nil {
		return nil, err
	}
	if err := settings.Validate(); err != nil {
		return nil, err
	}
	base := settings.URL(links, nil)
	return &Site{settings: settings, baseURL: base, apiURL: apiURL(base), caps: capabilities()}, nil
}

// apiURL returns the address of the API of a site's address: its host
// after the first label, under "api.".
func apiURL(base string) string {
	return "https://api." + base[strings.IndexByte(base, '.')+1:]
}

// adultCategories are the tracker's categories searched in the adult
// mode.
var adultCategories = []string{"410", "424", "437", "431", "429", "430", "426", "432", "436", "440", "425", "433", "411", "412", "413"}

func capabilities() *indexer.Capabilities {
	c := indexer.NewCapabilities()
	c.Params[indexer.SearchTV] = []indexer.Param{indexer.ParamQ, indexer.ParamSeason, indexer.ParamEp, indexer.ParamIMDbID}
	c.Params[indexer.SearchMovie] = []indexer.Param{indexer.ParamQ, indexer.ParamIMDbID}
	c.Params[indexer.SearchMusic] = []indexer.Param{indexer.ParamQ}
	c.Params[indexer.SearchBook] = []indexer.Param{indexer.ParamQ}
	c.Categories.Add("401", indexer.MoviesSD, "Movie(電影)/SD")
	c.Categories.Add("419", indexer.MoviesHD, "Movie(電影)/HD")
	c.Categories.Add("420", indexer.MoviesDVD, "Movie(電影)/DVDiSo")
	c.Categories.Add("421", indexer.MoviesBluRay, "Movie(電影)/Blu-Ray")
	c.Categories.Add("439", indexer.MoviesHD, "Movie(電影)/Remux")
	c.Categories.Add("403", indexer.TVSD, "TV Series(影劇/綜藝)/SD")
	c.Categories.Add("402", indexer.TVHD, "TV Series(影劇/綜藝)/HD")
	c.Categories.Add("438", indexer.TVHD, "TV Series(影劇/綜藝)/BD")
	c.Categories.Add("435", indexer.TVSD, "TV Series(影劇/綜藝)/DVDiSo")
	c.Categories.Add("404", indexer.TVDocumentary, "紀錄教育")
	c.Categories.Add("434", indexer.Audio, "Music(無損)")
	c.Categories.Add("406", indexer.AudioVideo, "MV(演唱)")
	c.Categories.Add("423", indexer.PCGames, "PCGame(PC遊戲)")
	c.Categories.Add("448", indexer.ConsoleOther, "TvGame(TV遊戲)")
	c.Categories.Add("405", indexer.TVAnime, "Anime(動畫)")
	c.Categories.Add("453", indexer.TVAnime, "Anime(動畫)/BluRay")
	c.Categories.Add("407", indexer.TVSport, "Sports(運動)")
	c.Categories.Add("427", indexer.BooksEBook, "Study/Edu ebook(教育書面)")
	c.Categories.Add("422", indexer.PC0day, "Software(軟體)")
	c.Categories.Add("442", indexer.AudioAudiobook, "Study/Edu audio(教育音檔)")
	c.Categories.Add("451", indexer.MoviesOther, "教育影片")
	c.Categories.Add("409", indexer.Other, "Misc(其他)")
	// Adult.
	c.Categories.Add("410", indexer.XXX, "AV(有碼)/HD Censored")
	c.Categories.Add("424", indexer.XXXSD, "AV(有碼)/SD Censored")
	c.Categories.Add("437", indexer.XXXDVD, "AV(有碼)/DVDiSo Censored")
	c.Categories.Add("431", indexer.XXX, "AV(有碼)/Blu-Ray Censored")
	c.Categories.Add("429", indexer.XXX, "AV(無碼)/HD Uncensored")
	c.Categories.Add("430", indexer.XXXSD, "AV(無碼)/SD Uncensored")
	c.Categories.Add("426", indexer.XXXDVD, "AV(無碼)/DVDiSo Uncensored")
	c.Categories.Add("432", indexer.XXX, "AV(無碼)/Blu-Ray Uncensored")
	c.Categories.Add("436", indexer.XXX, "AV(網站)/0Day")
	c.Categories.Add("440", indexer.XXX, "AV(Gay)/HD")
	c.Categories.Add("425", indexer.XXX, "IV(寫真影集)/Video Collection")
	c.Categories.Add("433", indexer.XXXImageSet, "IV(寫真圖集)/Picture Collection")
	c.Categories.Add("411", indexer.XXX, "H-Game(遊戲)")
	c.Categories.Add("412", indexer.XXX, "H-Anime(動畫)")
	c.Categories.Add("413", indexer.XXX, "H-Comic(漫畫)")
	return c
}

// pageSize is the number of results a search asks for.
const pageSize = 100

// Info implements indexer.Site.
func (s *Site) Info() indexer.Info {
	return indexer.Info{
		Protocol:       indexer.ProtocolTorrent,
		Privacy:        indexer.Private,
		SupportsRSS:    true,
		SupportsSearch: true,
		PageSize:       pageSize,
		RateLimit:      5 * time.Second,
	}
}

// Capabilities implements indexer.Site.
func (s *Site) Capabilities(context.Context, *indexer.Indexer) (*indexer.Capabilities, error) {
	return s.caps, nil
}

// LoginNeeded implements indexer.LoginSite: the API never asks to log in,
// as Prowlarr's CheckIfLoginNeeded answers.
func (s *Site) LoginNeeded(*indexer.Response) bool { return false }

// Login implements indexer.LoginSite: there is nothing to log in to.
func (s *Site) Login(context.Context, *indexer.Indexer) error { return nil }

// The modes of searches.
const (
	modeNormal = "normal"
	modeAdult  = "adult"
)

// search is a search of the API, its fields in the order Prowlarr writes
// them.
type search struct {
	Mode       string   `json:"mode"`
	Categories []string `json:"categories"`
	Discount   string   `json:"discount,omitempty"`
	IMDb       string   `json:"imdb,omitempty"`
	Keyword    string   `json:"keyword,omitempty"`
	PageNumber int      `json:"pageNumber"`
	PageSize   int      `json:"pageSize"`
}

// SearchRequests implements indexer.Site, as Prowlarr's
// MTeamTpRequestGenerator does: basic searches search the normal and the
// adult modes.
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
	var c indexer.Chain
	c.Add(s.pages(modeNormal, q, term, imdb))
	if q.Type == indexer.SearchBasic {
		c.Add(s.pages(modeAdult, q, term, imdb))
	}
	return &c, nil
}

// pages returns the requests of a search in a mode: none in the adult
// mode without adult categories, nor in the normal mode with only them.
func (s *Site) pages(mode string, q *indexer.Query, term, imdb string) iter.Seq[*indexer.Request] {
	mapped := s.caps.Categories.MapToTracker(q.Categories, false)
	var adult, normal []string
	for _, c := range mapped {
		if slices.Contains(adultCategories, c) {
			adult = append(adult, c)
		} else {
			normal = append(normal, c)
		}
	}
	return func(yield func(*indexer.Request) bool) {
		switch {
		case mode == modeAdult && len(adult) > 0:
			yield(s.request(mode, adult, term, imdb))
		case mode == modeNormal && (len(mapped) == 0 || len(normal) > 0):
			yield(s.request(mode, normal, term, imdb))
		}
	}
}

func (s *Site) request(mode string, categories []string, term, imdb string) *indexer.Request {
	body := search{Mode: mode, Categories: categories, PageNumber: 1, PageSize: pageSize}
	if body.Categories == nil {
		body.Categories = []string{}
	}
	if strings.TrimSpace(imdb) != "" {
		body.IMDb = strings.TrimSpace(imdb)
	}
	if strings.TrimSpace(term) != "" {
		body.Keyword = strings.TrimSpace(term)
	}
	if s.settings.FreeleechOnly {
		body.Discount = "FREE"
	}
	// Marshalling strings, ints and slices of strings cannot fail.
	data, _ := json.Marshal(body)
	r := indexer.NewRequest(indexer.CombinePath(s.apiURL, "/api/torrent/search"), indexer.AcceptJSON)
	r.Method = http.MethodPost
	r.Header.Set("x-api-key", s.settings.APIKey)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("User-Agent", UserAgent)
	r.Body = data
	return r
}

// torrent is a release of a search.
type torrent struct {
	ID          jsonx.String `json:"id"`
	Name        jsonx.String `json:"name"`
	Description jsonx.String `json:"smallDescr"`
	Category    jsonx.String `json:"category"`
	NumFiles    jsonx.String `json:"numfiles"`
	IMDb        jsonx.String `json:"imdb"`
	Size        jsonx.String `json:"size"`
	Status      *struct {
		CreatedDate    *jsonx.String `json:"createdDate"`
		Discount       jsonx.String  `json:"discount"`
		TimesCompleted jsonx.String  `json:"timesCompleted"`
		Seeders        jsonx.String  `json:"seeders"`
		Leechers       jsonx.String  `json:"leechers"`
	} `json:"status"`
}

// ParseResponse implements indexer.Site, as Prowlarr's MTeamTpParser
// does.
func (s *Site) ParseResponse(_ context.Context, _ *indexer.Indexer, r *indexer.Response) ([]*indexer.Release, error) {
	if r.StatusCode != http.StatusOK {
		return nil, indexer.Errorf("Unexpected response status %d code from indexer request", r.StatusCode)
	}
	if !r.HasContentType(indexer.AcceptJSON) {
		return nil, indexer.Errorf("Unexpected response header %s from indexer request, expected %s", r.Header.Get("Content-Type"), indexer.AcceptJSON)
	}
	var resp *struct {
		Data *struct {
			Torrents []torrent `json:"data"`
		} `json:"data"`
		Message jsonx.String `json:"message"`
	}
	if err := json.Unmarshal([]byte(r.Content()), &resp); err != nil {
		return nil, indexer.Errorf("Invalid response received from M-Team, not a valid JSON")
	}
	if resp == nil || resp.Data == nil || resp.Data.Torrents == nil {
		if resp != nil && strings.TrimSpace(string(resp.Message)) != "" && strings.ToUpper(string(resp.Message)) != "SUCCESS" {
			return nil, indexer.Errorf("Invalid response received from M-Team. Response from API: %s", resp.Message)
		}
		return nil, nil
	}
	releases := make([]*indexer.Release, 0, len(resp.Data.Torrents))
	for i := range resp.Data.Torrents {
		rel, err := s.release(&resp.Data.Torrents[i])
		if err != nil {
			return nil, err
		}
		releases = append(releases, rel)
	}
	slices.SortStableFunc(releases, func(a, b *indexer.Release) int { return b.PublishDate.Compare(a.PublishDate) })
	return releases, nil
}

// chinaZone is the zone of the site's dates, which have none.
var chinaZone = time.FixedZone("+0800", 8*60*60)

func (s *Site) release(t *torrent) (*indexer.Release, error) {
	if t.Status == nil {
		return nil, fmt.Errorf("mteamtp: torrent %s has no status", t.ID)
	}
	var n [6]int64
	for i, f := range []struct {
		name, v string
		bits    int
	}{
		{"id", string(t.ID), 32},
		{"numfiles", string(t.NumFiles), 32},
		{"size", string(t.Size), 64},
		{"timesCompleted", string(t.Status.TimesCompleted), 32},
		{"seeders", string(t.Status.Seeders), 32},
		{"leechers", string(t.Status.Leechers), 32},
	} {
		v, err := strconv.ParseInt(strings.TrimSpace(f.v), 10, f.bits)
		if err != nil {
			return nil, fmt.Errorf("mteamtp: %s: %w", f.name, err)
		}
		n[i] = v
	}
	id := strconv.FormatInt(n[0], 10)
	info := strings.TrimRight(s.baseURL, "/") + "/detail/" + id
	download, upload := 1.0, 1.0
	switch strings.ToUpper(string(t.Status.Discount)) {
	case "FREE":
		download = 0
	case "_2X_FREE":
		download, upload = 0, 2
	case "PERCENT_50":
		download = 0.5
	case "_2X_PERCENT_50":
		download, upload = 0.5, 2
	case "PERCENT_70":
		download = 0.3
	}
	rel := &indexer.Release{
		GUID:                 info,
		Title:                cleanTitle(string(t.Name)),
		InfoURL:              info,
		DownloadURL:          indexer.AddQueryParam(indexer.CombinePath(s.apiURL, "/api/torrent/genDlToken"), "id", id),
		Categories:           indexer.IDs(s.caps.Categories.MapTracker(string(t.Category))),
		Description:          string(t.Description),
		Files:                indexer.Ptr(int(n[1])),
		Size:                 indexer.Ptr(n[2]),
		Grabs:                indexer.Ptr(int(n[3])),
		Seeders:              indexer.Ptr(int(n[4])),
		Peers:                indexer.Ptr(int(n[4] + n[5])),
		DownloadVolumeFactor: indexer.Ptr(download),
		UploadVolumeFactor:   indexer.Ptr(upload),
		MinimumRatio:         indexer.Ptr(1.0),
		MinimumSeedTime:      indexer.Ptr(int64(172800)), // 2 days
	}
	if c := t.Status.CreatedDate; c != nil {
		if d, err := time.ParseInLocation(time.DateTime, string(*c), chinaZone); err == nil {
			rel.PublishDate = d.UTC()
		}
	}
	return rel, nil
}

var spaces = regexp.MustCompile(`\s+`)

// cleanTitle collapses the runs of white space of a title.
func cleanTitle(title string) string {
	return strings.TrimSpace(spaces.ReplaceAllString(title, " "))
}

// DownloadRequest implements indexer.DownloadSite: the release's link
// asks the API for a link with a token, which downloads the torrent.
func (s *Site) DownloadRequest(ctx context.Context, ix *indexer.Indexer, link string) (*indexer.Request, error) {
	r := indexer.NewRequest(link, indexer.AcceptJSON)
	r.Method = http.MethodPost
	r.Header.Set("x-api-key", s.settings.APIKey)
	r.Header.Set("User-Agent", UserAgent)
	resp, err := ix.Do(ctx, r)
	if err != nil {
		return nil, fmt.Errorf("mteamtp: download token: %w", err)
	}
	if resp.HasError() {
		return nil, fmt.Errorf("mteamtp: download token: %w", &indexer.HTTPError{Response: resp})
	}
	var token struct {
		Data jsonx.String `json:"data"`
	}
	if err := json.Unmarshal([]byte(resp.Content()), &token); err != nil {
		return nil, errors.New("invalid response received from M-Team, not a valid JSON")
	}
	if strings.TrimSpace(string(token.Data)) == "" {
		return nil, fmt.Errorf("unable to find download link for: %s", link)
	}
	dl := indexer.NewRequest(string(token.Data), "")
	dl.Header.Set("User-Agent", UserAgent)
	// The link carries the token; the site's cookies are not sent.
	dl.Cookies = map[string]string{}
	return dl, nil
}
