package hdbits

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
	"github.com/mavioai/mavio/plugins/indexers/internal/jsonx"
	"github.com/mavioai/mavio/plugins/indexers/internal/parse"
)

var (
	links  = []string{"https://hdbits.org/"}
	legacy = []string{"https://hdbits.org"}
)

// The media of releases.
const (
	mediumBluray  = 1
	mediumEncode  = 3
	mediumCapture = 4
	mediumRemux   = 5
	mediumWebDL   = 6
)

// The categories of releases this package tells apart.
const (
	categoryTV          = 2
	categoryDocumentary = 3
	categoryXXX         = 7
)

// Definition is HDBits.
var Definition = indexer.Definition{
	ID:          "hdbits",
	Name:        "HDBits",
	Description: "Best HD Tracker",
	Protocol:    indexer.ProtocolTorrent,
	Privacy:     indexer.Private,
	Links:       links,
	LegacyLinks: legacy,
	Fields: []indexer.Field{
		indexer.BaseURLField(links),
		{Name: "username", Label: "Username", Type: indexer.FieldText, HelpText: "Site Username"},
		{Name: "api_key", Label: "Passkey", Type: indexer.FieldText, Secret: true, HelpText: "Passkey from the Security page of the site's profile"},
		{
			Name: "codecs", Label: "Codecs", Type: indexer.FieldMultiSelect, Advanced: true,
			HelpText: "If unspecified, all options are used.",
			Options:  []indexer.Option{{Value: 1, Label: "H.264"}, {Value: 2, Label: "MPEG-2"}, {Value: 3, Label: "VC-1"}, {Value: 4, Label: "XviD"}, {Value: 5, Label: "HEVC"}},
		},
		{
			Name: "mediums", Label: "Mediums", Type: indexer.FieldMultiSelect, Advanced: true,
			HelpText: "If unspecified, all options are used.",
			Options: []indexer.Option{
				{Value: mediumBluray, Label: "Blu-ray/HD DVD"},
				{Value: mediumEncode, Label: "Encode"},
				{Value: mediumCapture, Label: "Capture"},
				{Value: mediumRemux, Label: "Remux"},
				{Value: mediumWebDL, Label: "WEB-DL"},
			},
		},
		{
			Name: "origins", Label: "Origins", Type: indexer.FieldMultiSelect, Advanced: true,
			HelpText: "If unspecified, all options are used.",
			Options:  []indexer.Option{{Value: 0, Label: "Undefined"}, {Value: 1, Label: "Internal"}},
		},
		{Name: "freeleech_only", Label: "Freeleech Only", Type: indexer.FieldCheckbox, Advanced: true, HelpText: "Show freeleech releases only"},
		{Name: "use_filenames", Label: "Use Filenames", Type: indexer.FieldCheckbox, Default: true, HelpText: "Use torrent file names as release titles"},
	},
	New: newSite,
}

// Settings are HDBits' settings.
type Settings struct {
	indexer.BaseSettings
	Username      string `json:"username,omitempty"`
	APIKey        string `json:"api_key,omitempty"`
	Codecs        []int  `json:"codecs,omitempty"`
	Mediums       []int  `json:"mediums,omitempty"`
	Origins       []int  `json:"origins,omitempty"`
	FreeleechOnly bool   `json:"freeleech_only,omitempty"`
	// UseFilenames titles releases by their torrent files; true when nil.
	UseFilenames *bool `json:"use_filenames,omitempty"`
}

// Validate checks that the user name and passkey are set.
func (s Settings) Validate() error {
	var errs []error
	if strings.TrimSpace(s.Username) == "" {
		errs = append(errs, errors.New("username: must not be empty"))
	}
	if strings.TrimSpace(s.APIKey) == "" {
		errs = append(errs, errors.New("api_key: must not be empty"))
	}
	return errors.Join(errs...)
}

// Site is HDBits.
type Site struct {
	settings Settings
	baseURL  string
	caps     *indexer.Capabilities
}

const pageSize = 100

func newSite(data []byte) (indexer.Site, error) {
	var settings Settings
	if err := indexer.DecodeSettings(data, &settings); err != nil {
		return nil, err
	}
	if err := settings.Validate(); err != nil {
		return nil, err
	}
	c := indexer.NewCapabilities()
	c.Params[indexer.SearchTV] = []indexer.Param{indexer.ParamQ, indexer.ParamSeason, indexer.ParamEp, indexer.ParamTVDBID}
	c.Params[indexer.SearchMovie] = []indexer.Param{indexer.ParamQ, indexer.ParamIMDbID}
	c.Flags = []indexer.Flag{indexer.FlagInternal, indexer.FlagExclusive}
	c.Categories.Add("1", indexer.Movies, "Movie")
	c.Categories.Add("2", indexer.TV, "TV")
	c.Categories.Add("3", indexer.TVDocumentary, "Documentary")
	c.Categories.Add("4", indexer.Audio, "Music")
	c.Categories.Add("5", indexer.TVSport, "Sport")
	c.Categories.Add("6", indexer.Audio, "Audio Track")
	c.Categories.Add("7", indexer.XXX, "XXX")
	c.Categories.Add("8", indexer.Other, "Misc/Demo")
	return &Site{settings: settings, baseURL: settings.URL(links, legacy), caps: c}, nil
}

// Info implements indexer.Site.
func (s *Site) Info() indexer.Info {
	return indexer.Info{
		Protocol:           indexer.ProtocolTorrent,
		Privacy:            indexer.Private,
		SupportsRSS:        true,
		SupportsSearch:     true,
		SupportsPagination: true,
		PageSize:           pageSize,
	}
}

// Capabilities implements indexer.Site.
func (s *Site) Capabilities(context.Context, *indexer.Indexer) (*indexer.Capabilities, error) {
	return s.caps, nil
}

// torrentQuery is the body of a search; nil fields are left out.
type torrentQuery struct {
	Username string    `json:"username"`
	Passkey  string    `json:"passkey"`
	Search   string    `json:"search,omitempty"`
	Category []int     `json:"category,omitempty"`
	Codec    []int     `json:"codec,omitempty"`
	Medium   []int     `json:"medium,omitempty"`
	Origin   []int     `json:"origin,omitempty"`
	IMDb     *imdbInfo `json:"imdb,omitempty"`
	TVDB     *tvdbInfo `json:"tvdb,omitempty"`
	Limit    int       `json:"limit"`
	Page     *int      `json:"page,omitempty"`
}

type imdbInfo struct {
	ID int `json:"id"`
}

type tvdbInfo struct {
	ID      int    `json:"id"`
	Season  *int   `json:"season,omitempty"`
	Episode string `json:"episode,omitempty"`
}

// torrentExt is the extension of torrent files, in any case.
var torrentExt = regexp.MustCompile(`(?i)\.torrent`)

// nonWord matches what .NET's \W does.
var nonWord = regexp.MustCompile(`[^\p{L}\p{Mn}\p{Nd}\p{Pc}]+`)

// SearchRequests implements indexer.Site, as Prowlarr's
// HDBitsRequestGenerator does.
func (s *Site) SearchRequests(_ context.Context, _ *indexer.Indexer, q *indexer.Query) (*indexer.Chain, error) {
	var c indexer.Chain
	var p torrentQuery
	hasTerm := strings.TrimSpace(q.Term) != ""
	imdb, _ := parse.IMDbID(q.IMDbID)
	switch q.Type {
	case indexer.SearchMusic, indexer.SearchBook:
		return &c, nil
	case indexer.SearchMovie:
		if imdb == 0 && hasTerm {
			p.Search = strings.TrimSpace(nonWord.ReplaceAllString(q.SanitizedTerm(), " "))
		}
	case indexer.SearchTV:
		if q.TVDBID == 0 && imdb == 0 && hasTerm {
			p.Search = q.SanitizedTVString()
		}
		if q.TVDBID != 0 {
			p.TVDB = &tvdbInfo{ID: q.TVDBID}
			if d, ok := dailyDate(q); ok {
				p.Search = d.Format(time.DateOnly)
			} else {
				p.TVDB.Season, p.TVDB.Episode = q.Season, q.Episode
			}
		}
	default:
		imdb = 0
		if hasTerm {
			p.Search = q.SanitizedTerm()
		}
	}
	if imdb != 0 {
		p.IMDb = &imdbInfo{ID: imdb}
	}
	r, err := s.request(&p, q)
	if err != nil {
		return nil, err
	}
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

func (s *Site) request(p *torrentQuery, q *indexer.Query) (*indexer.Request, error) {
	p.Username, p.Passkey = s.settings.Username, s.settings.APIKey
	p.Codec, p.Medium, p.Origin = s.settings.Codecs, s.settings.Mediums, s.settings.Origins
	for _, t := range s.caps.Categories.MapToTracker(q.Categories, false) {
		n, err := strconv.Atoi(t)
		if err != nil {
			return nil, err
		}
		p.Category = append(p.Category, n)
	}
	p.Limit = pageSize
	if q.Limit > 0 && q.Offset > 0 {
		p.Page = indexer.Ptr(q.Offset / q.Limit)
	}
	body, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	r := indexer.NewRequest(indexer.CombinePath(s.baseURL, "/api/torrents"), indexer.AcceptJSON)
	r.Method = http.MethodPost
	r.Header.Set("Content-Type", "application/json")
	r.Body = body
	return r, nil
}

// result is a release of a search.
type result struct {
	ID             jsonx.String `json:"id"`
	Hash           jsonx.String `json:"hash"`
	Leechers       jsonx.Int    `json:"leechers"`
	Seeders        jsonx.Int    `json:"seeders"`
	Name           jsonx.String `json:"name"`
	TimesCompleted jsonx.Int64  `json:"times_completed"`
	Size           jsonx.Int64  `json:"size"`
	Added          jsonx.Time   `json:"added"`
	NumFiles       jsonx.Int64  `json:"numfiles"`
	FileName       jsonx.String `json:"filename"`
	Freeleech      jsonx.String `json:"freeleech"`
	TypeCategory   jsonx.Int    `json:"type_category"`
	TypeMedium     jsonx.Int    `json:"type_medium"`
	TypeOrigin     jsonx.Int    `json:"type_origin"`
	TypeExclusive  jsonx.Int    `json:"type_exclusive"`
	IMDb           *struct {
		ID   jsonx.Int  `json:"id"`
		Year *jsonx.Int `json:"year"`
	} `json:"imdb"`
	TVDB *struct {
		ID *jsonx.Int `json:"id"`
	} `json:"tvdb"`
}

// The API's statuses of failed logins.
const (
	statusAuthDataMissing = 4
	statusAuthFailed      = 5
)

// ParseResponse implements indexer.Site, as Prowlarr's HDBitsParser does.
func (s *Site) ParseResponse(_ context.Context, _ *indexer.Indexer, r *indexer.Response) ([]*indexer.Release, error) {
	if r.StatusCode == http.StatusForbidden {
		return nil, &indexer.RateLimitError{Message: "HDBits Query Limit Reached. Please try again later."}
	}
	if r.StatusCode != http.StatusOK {
		return nil, indexer.Errorf("Unexpected response status %d code from indexer request", r.StatusCode)
	}
	var resp struct {
		Status  *jsonx.Int      `json:"status"`
		Message jsonx.String    `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal([]byte(r.Content()), &resp); err != nil {
		return nil, fmt.Errorf("hdbits: %w", err)
	}
	if resp.Status == nil {
		return nil, indexer.Errorf("HDBits API response has no status")
	}
	if *resp.Status != 0 {
		err := indexer.Errorf("HDBits API request returned status code %d: %s", *resp.Status, resp.Message)
		if *resp.Status == statusAuthDataMissing || *resp.Status == statusAuthFailed {
			return nil, fmt.Errorf("%w: %w", indexer.ErrAuth, err)
		}
		return nil, err
	}
	if !strings.HasPrefix(strings.TrimSpace(string(resp.Data)), "[") {
		return nil, indexer.Errorf("Indexer API call response missing result data")
	}
	var results []result
	if err := json.Unmarshal(resp.Data, &results); err != nil {
		return nil, fmt.Errorf("hdbits: %w", err)
	}
	releases := make([]*indexer.Release, 0, len(results))
	for i := range results {
		row := &results[i]
		if s.settings.FreeleechOnly && row.Freeleech != "yes" {
			continue
		}
		releases = append(releases, s.release(row))
	}
	return releases, nil
}

func (s *Site) release(row *result) *indexer.Release {
	id := string(row.ID)
	rel := &indexer.Release{
		GUID:                 "HDBits-" + id,
		Title:                s.title(row),
		Size:                 indexer.Ptr(int64(row.Size)),
		Categories:           indexer.IDs(s.caps.Categories.MapTracker(strconv.Itoa(int(row.TypeCategory)))),
		InfoHash:             string(row.Hash),
		DownloadURL:          indexer.AddQueryParam(indexer.AddQueryParam(indexer.CombinePath(s.baseURL, "/download.php"), "id", id), "passkey", s.settings.APIKey),
		InfoURL:              indexer.AddQueryParam(indexer.CombinePath(s.baseURL, "/details.php"), "id", id),
		Protocol:             indexer.ProtocolTorrent,
		Seeders:              indexer.Ptr(int(row.Seeders)),
		Grabs:                indexer.Ptr(int(row.TimesCompleted)),
		Files:                indexer.Ptr(int(row.NumFiles)),
		Peers:                indexer.Ptr(int(row.Leechers + row.Seeders)),
		PublishDate:          row.Added.UTC(),
		DownloadVolumeFactor: indexer.Ptr(downloadFactor(row)),
		UploadVolumeFactor:   indexer.Ptr(1.0),
	}
	if row.TypeCategory == categoryXXX {
		rel.UploadVolumeFactor = indexer.Ptr(0.0)
	}
	if row.IMDb != nil {
		rel.IMDbID = int(row.IMDb.ID)
		if row.IMDb.Year != nil {
			rel.Year = int(*row.IMDb.Year)
		}
	}
	if row.TVDB != nil && row.TVDB.ID != nil {
		rel.TVDBID = int(*row.TVDB.ID)
	}
	if row.TypeOrigin == 1 {
		rel.Flags = append(rel.Flags, indexer.FlagInternal)
	}
	if row.TypeExclusive == 1 {
		rel.Flags = append(rel.Flags, indexer.FlagExclusive)
	}
	return rel
}

// title returns the torrent file's name as the title, unless the settings
// say not to, or the release is XXX or a full disc.
func (s *Site) title(row *result) string {
	useFilenames := s.settings.UseFilenames == nil || *s.settings.UseFilenames
	if row.TypeCategory != categoryXXX && row.TypeMedium != mediumBluray && useFilenames && strings.TrimSpace(string(row.FileName)) != "" {
		return torrentExt.ReplaceAllLiteralString(string(row.FileName), "")
	}
	return string(row.Name)
}

// downloadFactor returns the share of a release's download counted.
func downloadFactor(row *result) float64 {
	switch {
	case row.Freeleech == "yes":
		return 0
	case row.TypeCategory == categoryXXX:
		// 100% neutral leech: all XXX content.
		return 0
	case row.TypeMedium == mediumBluray || row.TypeMedium == mediumRemux || row.TypeMedium == mediumCapture,
		row.TypeOrigin == 1, row.TypeCategory == categoryTV, row.TypeCategory == categoryDocumentary:
		// 50% free leech: full discs, remuxes, captures, internal
		// encodes, TV and documentaries.
		return 0.5
	}
	return 1
}
