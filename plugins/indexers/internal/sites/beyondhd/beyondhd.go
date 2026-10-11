package beyondhd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
	"github.com/mavioai/mavio/plugins/indexers/internal/jsonx"
	"github.com/mavioai/mavio/plugins/indexers/internal/parse"
)

var links = []string{"https://beyond-hd.me/"}

// searchTypes are the labels of the release types searches can be
// limited to, by their value in the settings.
var searchTypes = []string{
	"UHD 100", "UHD 66", "UHD 50", "UHD Remux", "BD 50", "BD 25", "BD Remux",
	"2160p", "1080p", "1080i", "720p", "576p", "540p",
	"DVD 9", "DVD 5", "DVD Remux", "480p", "Other",
}

func searchTypeOptions() []indexer.Option {
	opts := make([]indexer.Option, len(searchTypes))
	for i, l := range searchTypes {
		opts[i] = indexer.Option{Value: i, Label: l}
	}
	return opts
}

// Definition is BeyondHD.
var Definition = indexer.Definition{
	ID:          "beyondhd",
	Name:        "BeyondHD",
	Description: "BeyondHD (BHD) is a Private Torrent Tracker for HD MOVIES / TV",
	Protocol:    indexer.ProtocolTorrent,
	Privacy:     indexer.Private,
	Links:       links,
	Fields: []indexer.Field{
		indexer.BaseURLField(links),
		{Name: "api_key", Label: "API Key", Type: indexer.FieldText, Secret: true, HelpText: "API Key from the Site (Found in My Security => API Key)"},
		{Name: "rss_key", Label: "RSS Key", Type: indexer.FieldText, Secret: true, HelpText: "RSS Key from the Site (Found in My Security => RSS Key)"},
		{Name: "freeleech_only", Label: "Freeleech Only", Type: indexer.FieldCheckbox, HelpText: "Search freeleech releases only"},
		{Name: "limited_only", Label: "Limited Only", Type: indexer.FieldCheckbox, HelpText: "Search freeleech only (Limited UL)"},
		{Name: "refund_only", Label: "Refund Only", Type: indexer.FieldCheckbox, HelpText: "Search refund only"},
		{Name: "rewind_only", Label: "Rewind Only", Type: indexer.FieldCheckbox, HelpText: "Search rewind only"},
		{
			Name: "search_types", Label: "Search Types", Type: indexer.FieldMultiSelect, Advanced: true,
			HelpText: "Select the types of releases that you are interested in. If none selected, all options are used.",
			Options:  searchTypeOptions(),
		},
	},
	New: newSite,
}

// Settings are BeyondHD's settings.
type Settings struct {
	indexer.BaseSettings
	APIKey        string `json:"api_key,omitempty"`
	RSSKey        string `json:"rss_key,omitempty"`
	FreeleechOnly bool   `json:"freeleech_only,omitempty"`
	LimitedOnly   bool   `json:"limited_only,omitempty"`
	RefundOnly    bool   `json:"refund_only,omitempty"`
	RewindOnly    bool   `json:"rewind_only,omitempty"`
	// SearchTypes are indexes of searchTypes.
	SearchTypes []int `json:"search_types,omitempty"`
}

// Validate checks that the API and RSS keys are set and 32 characters
// long.
func (s Settings) Validate() error {
	var errs []error
	for _, f := range []struct{ name, v string }{{"api_key", s.APIKey}, {"rss_key", s.RSSKey}} {
		switch {
		case strings.TrimSpace(f.v) == "":
			errs = append(errs, fmt.Errorf("%s: must not be empty", f.name))
		case len([]rune(f.v)) != 32:
			errs = append(errs, fmt.Errorf("%s: must be 32 characters", f.name))
		}
	}
	return errors.Join(errs...)
}

// Site is BeyondHD.
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
	c.Params[indexer.SearchMovie] = []indexer.Param{indexer.ParamQ, indexer.ParamIMDbID, indexer.ParamTMDBID}
	c.Flags = []indexer.Flag{indexer.FlagInternal, indexer.FlagExclusive}
	c.Categories.Add("1", indexer.Movies, "Movies")
	c.Categories.Add("2", indexer.TV, "TV")
	return c
}

// pageSize is the most results of a search.
const pageSize = 100

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

// body is a search of the API, its fields in the order Prowlarr writes
// them.
type body struct {
	Action     string   `json:"action"`
	RSSKey     string   `json:"rsskey"`
	Freeleech  int      `json:"freeleech,omitempty"`
	Limited    int      `json:"limited,omitempty"`
	Refund     int      `json:"refund,omitempty"`
	Rewind     int      `json:"rewind,omitempty"`
	IMDbID     string   `json:"imdb_id,omitempty"`
	TMDBID     string   `json:"tmdb_id,omitempty"`
	Search     string   `json:"search,omitempty"`
	Categories []int    `json:"categories,omitempty"`
	Types      []string `json:"types,omitempty"`
	Page       int      `json:"page,omitempty"`
}

// SearchRequests implements indexer.Site, as Prowlarr's
// BeyondHDRequestGenerator does: a POST of the search to the API.
func (s *Site) SearchRequests(_ context.Context, _ *indexer.Indexer, q *indexer.Query) (*indexer.Chain, error) {
	term := q.SanitizedTerm()
	var imdb string
	var tmdb int
	switch q.Type {
	case indexer.SearchMovie:
		imdb, tmdb = q.FullIMDbID(), q.TMDBID
	case indexer.SearchTV:
		imdb = q.FullIMDbID()
		term = q.SanitizedTVString()
		if q.Season != nil && *q.Season > 0 && strings.TrimSpace(q.Episode) != "" {
			if d, err := time.Parse("2006 01/02", fmt.Sprintf("%d %s", *q.Season, q.Episode)); err == nil {
				term = q.SanitizedTerm() + " " + d.Format("2006-01-02")
			}
		}
	}
	b := body{Action: "search", RSSKey: s.settings.RSSKey}
	if s.settings.FreeleechOnly {
		b.Freeleech = 1
	}
	if s.settings.LimitedOnly {
		b.Limited = 1
	}
	if s.settings.RefundOnly {
		b.Refund = 1
	}
	if s.settings.RewindOnly {
		b.Rewind = 1
	}
	switch {
	case strings.TrimSpace(imdb) != "":
		b.IMDbID = imdb
	case tmdb > 0:
		b.TMDBID = "movie/" + strconv.Itoa(tmdb)
	}
	if strings.TrimSpace(term) != "" {
		b.Search = strings.TrimSpace(term)
	}
	for _, c := range s.caps.Categories.MapToTracker(q.Categories, false) {
		n, err := strconv.Atoi(c)
		if err != nil {
			return nil, fmt.Errorf("beyondhd: category %q: %w", c, err)
		}
		b.Categories = append(b.Categories, n)
	}
	for _, t := range s.settings.SearchTypes {
		if t >= 0 && t < len(searchTypes) && !slices.Contains(b.Types, searchTypes[t]) {
			b.Types = append(b.Types, searchTypes[t])
		}
	}
	if q.Limit > 0 && q.Offset > 0 {
		b.Page = q.Offset/q.Limit + 1
	}
	data, err := json.Marshal(b)
	if err != nil {
		return nil, err
	}
	r := indexer.NewRequest(s.baseURL+"api/torrents/"+s.settings.APIKey, indexer.AcceptJSON)
	// Prowlarr's IndexerRequest of a URL follows redirects.
	r.FollowRedirects = true
	r.Method = http.MethodPost
	r.Header.Set("Content-Type", "application/json")
	r.Body = data
	var c indexer.Chain
	c.AddRequests(r)
	return &c, nil
}

// CleanupReleases implements indexer.ReleaseCleaner: the site searches
// loosely, so releases not matching the query are dropped.
func (s *Site) CleanupReleases(releases []*indexer.Release, q *indexer.Query) []*indexer.Release {
	return indexer.FilterByQuery(releases, q)
}

// result is a release of a search.
type result struct {
	Name         jsonx.String `json:"name"`
	InfoHash     jsonx.String `json:"info_hash"`
	Category     jsonx.String `json:"category"`
	Type         jsonx.String `json:"type"`
	Size         jsonx.Int64  `json:"size"`
	Grabs        jsonx.Int    `json:"times_completed"`
	Seeders      jsonx.Int    `json:"seeders"`
	Leechers     jsonx.Int    `json:"leechers"`
	Audios       *string      `json:"audios"`
	Subtitles    *string      `json:"subtitles"`
	CreatedAt    jsonx.String `json:"created_at"`
	DownloadLink jsonx.String `json:"download_url"`
	InfoURL      jsonx.String `json:"url"`
	IMDbID       jsonx.String `json:"imdb_id"`
	TMDBID       jsonx.String `json:"tmdb_id"`
	Freeleech    jsonx.Bool   `json:"freeleech"`
	Promo25      jsonx.Bool   `json:"promo25"`
	Promo50      jsonx.Bool   `json:"promo50"`
	Promo75      jsonx.Bool   `json:"promo75"`
	Limited      jsonx.Bool   `json:"limited"`
	Exclusive    jsonx.Bool   `json:"exclusive"`
	Internal     jsonx.Bool   `json:"internal"`
}

// ParseResponse implements indexer.Site, as Prowlarr's BeyondHDParser
// does.
func (s *Site) ParseResponse(_ context.Context, _ *indexer.Indexer, r *indexer.Response) ([]*indexer.Release, error) {
	if r.StatusCode != http.StatusOK {
		return nil, indexer.Errorf("Unexpected response status %d code from indexer request", r.StatusCode)
	}
	content := r.Content()
	if r.HasContentType("text/html") {
		reason := "Site is likely blocked or unavailable."
		if strings.Contains(strings.ToLower(content), "site maintenance") {
			reason = "Site is under maintenance."
		}
		return nil, indexer.Errorf("Indexer responded with HTML content. %s", reason)
	}
	if !r.HasContentType(indexer.AcceptJSON) {
		return nil, indexer.Errorf("Unexpected response header %s from indexer request, expected %s", r.Header.Get("Content-Type"), indexer.AcceptJSON)
	}
	if strings.Contains(strings.ToLower(content), "invalid api key") {
		return nil, fmt.Errorf("API Key invalid or not authorized: %w", indexer.ErrAuth)
	}
	var resp struct {
		StatusCode    jsonx.Int    `json:"status_code"`
		StatusMessage jsonx.String `json:"status_message"`
		Results       []result     `json:"results"`
	}
	if err := json.Unmarshal([]byte(content), &resp); err != nil {
		return nil, fmt.Errorf("beyondhd: %w", err)
	}
	if resp.StatusCode == 0 {
		return nil, indexer.Errorf("Indexer Error: %s", resp.StatusMessage)
	}
	releases := make([]*indexer.Release, 0, len(resp.Results))
	for i := range resp.Results {
		row := &resp.Results[i]
		// Results not matching the freeleech or limited filters are
		// skipped.
		if s.settings.FreeleechOnly && !bool(row.Freeleech) || s.settings.LimitedOnly && !bool(row.Limited) {
			continue
		}
		rel, err := s.release(row)
		if err != nil {
			return nil, err
		}
		releases = append(releases, rel)
	}
	slices.SortStableFunc(releases, func(a, b *indexer.Release) int { return b.PublishDate.Compare(a.PublishDate) })
	return releases, nil
}

func (s *Site) release(row *result) (*indexer.Release, error) {
	date, err := indexer.ParseDate(string(row.CreatedAt))
	if err != nil {
		return nil, fmt.Errorf("beyondhd: %w", err)
	}
	details := string(row.InfoURL)
	factor := 1.0
	switch {
	case bool(row.Freeleech) || bool(row.Limited):
		factor = 0
	case bool(row.Promo75):
		factor = 0.25
	case bool(row.Promo50):
		factor = 0.5
	case bool(row.Promo25):
		factor = 0.75
	}
	rel := &indexer.Release{
		Title:                string(row.Name),
		DownloadURL:          string(row.DownloadLink),
		InfoHash:             string(row.InfoHash),
		InfoURL:              details,
		GUID:                 details,
		Categories:           indexer.IDs(s.caps.Categories.MapTrackerDesc(string(row.Category))),
		PublishDate:          date,
		Size:                 indexer.Ptr(int64(row.Size)),
		Grabs:                indexer.Ptr(int(row.Grabs)),
		Seeders:              indexer.Ptr(int(row.Seeders)),
		Peers:                indexer.Ptr(int(row.Leechers + row.Seeders)),
		DownloadVolumeFactor: indexer.Ptr(factor),
		UploadVolumeFactor:   indexer.Ptr(1.0),
		MinimumRatio:         indexer.Ptr(1.0),
		MinimumSeedTime:      indexer.Ptr(int64(172800)), // 48 hours
		Languages:            split(row.Audios),
		Subtitles:            split(row.Subtitles),
	}
	if id, ok := parse.IMDbID(string(row.IMDbID)); ok {
		rel.IMDbID = id
	}
	if bool(row.Internal) {
		rel.Flags = append(rel.Flags, indexer.FlagInternal)
	}
	if bool(row.Exclusive) {
		rel.Flags = append(rel.Flags, indexer.FlagExclusive)
	}
	// The site's TMDB IDs, such as "movie/603", can be odd.
	if t := string(row.TMDBID); strings.TrimSpace(t) != "" {
		if parts := strings.Split(t, "/"); len(parts) > 1 {
			rel.TMDBID = coerceInt(parts[1])
		}
	}
	return rel, nil
}

// split returns the entries of a list separated by commas, trimmed and
// without empty ones.
func split(s *string) []string {
	if s == nil {
		return []string{}
	}
	out := []string{}
	for v := range strings.SplitSeq(*s, ",") {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// coerceInt reads an integer as Prowlarr's TryCoerceInt does; 0 when it
// cannot.
func coerceInt(s string) int {
	if strings.TrimSpace(s) == "" {
		return 0
	}
	n, err := parse.Int(s)
	if err != nil || n > math.MaxInt32 || n < math.MinInt32 {
		return 0
	}
	return int(n)
}
