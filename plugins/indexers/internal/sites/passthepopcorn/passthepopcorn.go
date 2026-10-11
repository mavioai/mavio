package passthepopcorn

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
	"github.com/mavioai/mavio/plugins/indexers/internal/jsonx"
)

var links = []string{"https://passthepopcorn.me"}

// The flags of PassThePopcorn's releases.
const (
	// FlagGolden marks releases following Golden Popcorn quality rules.
	FlagGolden indexer.Flag = "golden"
	// FlagApproved marks releases approved by the site's staff.
	FlagApproved indexer.Flag = "approved"
)

// Definition is PassThePopcorn.
var Definition = indexer.Definition{
	ID:          "passthepopcorn",
	Name:        "PassThePopcorn",
	Description: "PassThePopcorn (PTP) is a Private site for MOVIES / TV",
	Protocol:    indexer.ProtocolTorrent,
	Privacy:     indexer.Private,
	Links:       links,
	Fields: []indexer.Field{
		indexer.BaseURLField(links),
		{Name: "api_user", Label: "API User", Type: indexer.FieldText, HelpText: "The API user from the security page of the site's profile"},
		{Name: "api_key", Label: "API Key", Type: indexer.FieldText, Secret: true, HelpText: "The API key from the security page of the site's profile"},
		{Name: "freeleech_only", Label: "Freeleech Only", Type: indexer.FieldCheckbox, HelpText: "Search freeleech releases only"},
		{Name: "golden_popcorn_only", Label: "Golden Popcorn Only", Type: indexer.FieldCheckbox, Advanced: true, HelpText: "Search Golden Popcorn releases only"},
	},
	// The API takes 150 calls an hour.
	Limits: indexer.Limits{Queries: 150, Per: time.Hour},
	New:    newSite,
}

// Settings are PassThePopcorn's settings.
type Settings struct {
	indexer.BaseSettings
	APIUser           string `json:"api_user,omitempty"`
	APIKey            string `json:"api_key,omitempty"`
	FreeleechOnly     bool   `json:"freeleech_only,omitempty"`
	GoldenPopcornOnly bool   `json:"golden_popcorn_only,omitempty"`
}

// Validate checks that the API user and key are set.
func (s Settings) Validate() error {
	var errs []error
	if strings.TrimSpace(s.APIUser) == "" {
		errs = append(errs, errors.New("api_user: must not be empty"))
	}
	if strings.TrimSpace(s.APIKey) == "" {
		errs = append(errs, errors.New("api_key: must not be empty"))
	}
	return errors.Join(errs...)
}

// Site is PassThePopcorn.
type Site struct {
	settings Settings
	baseURL  string
	caps     *indexer.Capabilities
}

const pageSize = 50

func newSite(data []byte) (indexer.Site, error) {
	var settings Settings
	if err := indexer.DecodeSettings(data, &settings); err != nil {
		return nil, err
	}
	if err := settings.Validate(); err != nil {
		return nil, err
	}
	c := indexer.NewCapabilities()
	c.LimitsDefault, c.LimitsMax = pageSize, pageSize
	c.Params[indexer.SearchMovie] = []indexer.Param{indexer.ParamQ, indexer.ParamIMDbID}
	c.Flags = []indexer.Flag{indexer.FlagFreeleech, FlagGolden, FlagApproved}
	c.Categories.Add("1", indexer.Movies, "Feature Film")
	c.Categories.Add("2", indexer.Movies, "Short Film")
	c.Categories.Add("3", indexer.Movies, "Miniseries")
	c.Categories.Add("4", indexer.Movies, "Stand-up Comedy")
	c.Categories.Add("5", indexer.Movies, "Live Performance")
	c.Categories.Add("6", indexer.Movies, "Movie Collection")
	return &Site{settings: settings, baseURL: settings.URL(links, nil), caps: c}, nil
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
		RateLimit:          4 * time.Second,
	}
}

// Capabilities implements indexer.Site.
func (s *Site) Capabilities(context.Context, *indexer.Indexer) (*indexer.Capabilities, error) {
	return s.caps, nil
}

// SearchRequests implements indexer.Site, as Prowlarr's
// PassThePopcornRequestGenerator does.
func (s *Site) SearchRequests(_ context.Context, _ *indexer.Indexer, q *indexer.Query) (*indexer.Chain, error) {
	var c indexer.Chain
	term := q.SanitizedTerm()
	switch q.Type {
	case indexer.SearchMusic, indexer.SearchBook:
		return &c, nil
	case indexer.SearchMovie, indexer.SearchTV:
		switch {
		case strings.TrimSpace(q.IMDbID) != "":
			term = q.FullIMDbID()
		case q.Type == indexer.SearchTV:
			term = q.SanitizedTVString()
		}
	}
	var p indexer.Params
	p.Set("action", "advanced")
	p.Set("json", "noredirect")
	p.Set("grouping", "0")
	p.Set("order_by", "time")
	p.Set("order_way", "desc")
	p.Set("searchstr", term)
	if s.settings.FreeleechOnly {
		p.Set("freetorrent", "1")
	}
	if s.settings.GoldenPopcornOnly {
		p.Set("scene", "2")
	}
	if q.IsRSS() {
		for _, cat := range s.caps.Categories.MapToTracker(q.Categories, false) {
			p.Set("filter_cat["+cat+"]", "1")
		}
	}
	if q.Limit > 0 && q.Offset > 0 {
		p.Set("page", strconv.Itoa(q.Offset/q.Limit+1))
	}
	r := indexer.NewRequest(strings.TrimRight(strings.TrimSpace(s.baseURL), "/")+"/torrents.php?"+p.Encode(), indexer.AcceptJSON)
	// Prowlarr's IndexerRequest of a URL follows redirects.
	r.FollowRedirects = true
	s.authorize(r)
	r.AllowedStatus = []int{http.StatusTooManyRequests}
	c.AddRequests(r)
	return &c, nil
}

func (s *Site) authorize(r *indexer.Request) {
	r.Header.Set("ApiUser", s.settings.APIUser)
	r.Header.Set("ApiKey", s.settings.APIKey)
}

// DownloadRequest implements indexer.DownloadSite: downloads carry the API
// user and key.
func (s *Site) DownloadRequest(_ context.Context, _ *indexer.Indexer, link string) (*indexer.Request, error) {
	r := indexer.NewRequest(link, "")
	s.authorize(r)
	return r, nil
}

type movie struct {
	GroupID    jsonx.String   `json:"GroupId"`
	CategoryID jsonx.String   `json:"CategoryId"`
	Year       jsonx.Int      `json:"Year"`
	Cover      jsonx.String   `json:"Cover"`
	Tags       []jsonx.String `json:"Tags"`
	IMDbID     jsonx.String   `json:"ImdbId"`
	Torrents   []torrent      `json:"Torrents"`
}

type torrent struct {
	ID            jsonx.Int    `json:"Id"`
	Scene         jsonx.Bool   `json:"Scene"`
	Size          jsonx.Int64  `json:"Size"`
	UploadTime    jsonx.String `json:"UploadTime"`
	Snatched      jsonx.Int    `json:"Snatched"`
	Seeders       jsonx.Int    `json:"Seeders"`
	Leechers      jsonx.Int    `json:"Leechers"`
	ReleaseName   jsonx.String `json:"ReleaseName"`
	Checked       jsonx.Bool   `json:"Checked"`
	GoldenPopcorn jsonx.Bool   `json:"GoldenPopcorn"`
	FreeleechType jsonx.String `json:"FreeleechType"`
}

// ParseResponse implements indexer.Site, as Prowlarr's
// PassThePopcornParser does.
func (s *Site) ParseResponse(_ context.Context, _ *indexer.Indexer, r *indexer.Response) ([]*indexer.Release, error) {
	switch {
	case r.StatusCode == http.StatusOK:
	case r.StatusCode == http.StatusForbidden:
		return nil, &indexer.RateLimitError{Message: "PTP Query Limit Reached. Please try again later."}
	case r.StatusCode == http.StatusTooManyRequests && strings.Contains(r.Content(), "We are not a TV indexer"):
		return nil, indexer.Errorf("Invalid indexer request: We are not a TV indexer")
	case r.StatusCode == http.StatusTooManyRequests:
		return nil, &indexer.RateLimitError{Message: "PTP Request Limit Reached. Please try again later."}
	default:
		return nil, indexer.Errorf("Unexpected response status %d code from indexer request", r.StatusCode)
	}
	if !r.HasContentType(indexer.AcceptJSON) {
		return nil, indexer.Errorf("Unexpected response header %s from indexer request, expected %s", r.Header.Get("Content-Type"), indexer.AcceptJSON)
	}
	var resp struct {
		TotalResults jsonx.String `json:"TotalResults"`
		Movies       []movie      `json:"Movies"`
	}
	if err := json.Unmarshal([]byte(r.Content()), &resp); err != nil {
		return nil, fmt.Errorf("passthepopcorn: %w", err)
	}
	if resp.TotalResults == "0" || strings.TrimSpace(string(resp.TotalResults)) == "" {
		return nil, nil
	}
	var releases []*indexer.Release
	for _, m := range resp.Movies {
		for _, t := range m.Torrents {
			rel, err := s.release(&m, &t)
			if err != nil {
				return nil, err
			}
			if s.settings.FreeleechOnly && *rel.DownloadVolumeFactor != 0 {
				continue
			}
			releases = append(releases, rel)
		}
	}
	return releases, nil
}

func (s *Site) release(m *movie, t *torrent) (*indexer.Release, error) {
	date, err := time.ParseInLocation(time.DateTime, strings.TrimSpace(string(t.UploadTime)), time.UTC)
	if err != nil {
		return nil, fmt.Errorf("passthepopcorn: %w", err)
	}
	id := strconv.Itoa(int(t.ID))
	down, up := 1.0, 1.0
	switch strings.ToUpper(string(t.FreeleechType)) {
	case "FREELEECH":
		down = 0
	case "NEUTRAL LEECH":
		down, up = 0, 0
	case "HALF LEECH":
		down = 0.5
	}
	torrents := indexer.CombinePath(s.baseURL, "/torrents.php")
	rel := &indexer.Release{
		GUID:                 "PassThePopcorn-" + id,
		Title:                string(t.ReleaseName),
		Year:                 int(m.Year),
		InfoURL:              indexer.AddQueryParam(indexer.AddQueryParam(torrents, "id", string(m.GroupID)), "torrentid", id),
		DownloadURL:          indexer.AddQueryParam(indexer.AddQueryParam(torrents, "action", "download"), "id", id),
		Protocol:             indexer.ProtocolTorrent,
		Categories:           indexer.IDs(s.caps.Categories.MapTracker(string(m.CategoryID))),
		Size:                 indexer.Ptr(int64(t.Size)),
		Grabs:                indexer.Ptr(int(t.Snatched)),
		Seeders:              indexer.Ptr(int(t.Seeders)),
		Peers:                indexer.Ptr(int(t.Leechers + t.Seeders)),
		PublishDate:          date,
		Scene:                bool(t.Scene),
		DownloadVolumeFactor: indexer.Ptr(down),
		UploadVolumeFactor:   indexer.Ptr(up),
		MinimumRatio:         indexer.Ptr(1.0),
		MinimumSeedTime:      indexer.Ptr(int64(345600)),
		PosterURL:            posterURL(string(m.Cover)),
	}
	if strings.TrimSpace(string(m.IMDbID)) != "" {
		n, err := strconv.Atoi(strings.TrimSpace(string(m.IMDbID)))
		if err != nil {
			return nil, fmt.Errorf("passthepopcorn: IMDb ID: %w", err)
		}
		rel.IMDbID = n
	}
	if t.GoldenPopcorn {
		rel.Flags = append(rel.Flags, FlagGolden)
	}
	if t.Checked {
		rel.Flags = append(rel.Flags, FlagApproved)
	}
	for _, g := range m.Tags {
		rel.Genres = append(rel.Genres, string(g))
	}
	return rel, nil
}

// posterURL returns a cover's address if it is an absolute HTTP one.
func posterURL(cover string) string {
	u, err := url.Parse(strings.TrimSpace(cover))
	if err != nil || u.Host == "" || !slices.Contains([]string{"http", "https"}, u.Scheme) {
		return ""
	}
	return u.String()
}
