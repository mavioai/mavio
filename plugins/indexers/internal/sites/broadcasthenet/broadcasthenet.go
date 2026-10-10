package broadcasthenet

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
	"github.com/mavioai/mavio/plugins/indexers/internal/jsonx"
)

var (
	links  = []string{"https://api.broadcasthe.net/"}
	legacy = []string{"http://api.broadcasthe.net/"}
)

// Definition is BroadcasTheNet.
var Definition = indexer.Definition{
	ID:          "broadcasthenet",
	Name:        "BroadcasTheNet",
	Description: "BroadcasTheNet (BTN) is an invite-only torrent tracker focused on TV shows",
	Protocol:    indexer.ProtocolTorrent,
	Privacy:     indexer.Private,
	Links:       links,
	LegacyLinks: legacy,
	Fields:      []indexer.Field{indexer.BaseURLField(links), indexer.APIKeyField},
	// The API takes 150 calls an hour.
	Limits: indexer.Limits{Queries: 150, Per: time.Hour},
	New:    newSite,
}

// Settings are BroadcasTheNet's settings.
type Settings struct {
	indexer.BaseSettings
	APIKey string `json:"api_key,omitempty"`
}

// Validate checks that the API key is set.
func (s Settings) Validate() error {
	if strings.TrimSpace(s.APIKey) == "" {
		return errors.New("api_key: must not be empty")
	}
	return nil
}

// Site is BroadcasTheNet.
type Site struct {
	settings Settings
	baseURL  string
	caps     *indexer.Capabilities
}

// pageSize is the results of a search by default.
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
	c.LimitsDefault, c.LimitsMax = pageSize, 1000
	c.Params[indexer.SearchTV] = []indexer.Param{indexer.ParamQ, indexer.ParamSeason, indexer.ParamEp, indexer.ParamTVDBID, indexer.ParamRID}
	c.Categories.Add("SD", indexer.TVSD, "SD")
	c.Categories.Add("720p", indexer.TVHD, "720p")
	c.Categories.Add("1080p", indexer.TVHD, "1080p")
	c.Categories.Add("1080i", indexer.TVHD, "1080i")
	c.Categories.Add("2160p", indexer.TVUHD, "2160p")
	c.Categories.Add("Portable Device", indexer.TVSD, "Portable Device")
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
		RateLimit:          5 * time.Second,
	}
}

// Capabilities implements indexer.Site.
func (s *Site) Capabilities(context.Context, *indexer.Indexer) (*indexer.Capabilities, error) {
	return s.caps, nil
}

// torrentQuery is the filter of a getTorrents call; empty fields are left
// out.
type torrentQuery struct {
	Category string `json:"Category,omitempty"`
	Name     string `json:"Name,omitempty"`
	Search   string `json:"Search,omitempty"`
	Tvdb     string `json:"Tvdb,omitempty"`
	Tvrage   string `json:"Tvrage,omitempty"`
}

// SearchRequests implements indexer.Site, as Prowlarr's
// BroadcastheNetRequestGenerator does: only TV and basic searches.
func (s *Site) SearchRequests(_ context.Context, _ *indexer.Indexer, q *indexer.Query) (*indexer.Chain, error) {
	var c indexer.Chain
	if q.Type != indexer.SearchTV && q.Type != indexer.SearchBasic {
		return &c, nil
	}
	results := q.Limit
	if results == 0 {
		results = s.caps.LimitsDefault
	}
	var p torrentQuery
	if strings.TrimSpace(q.Term) != "" {
		p.Search = strings.ReplaceAll(q.Term, " ", "%")
	}
	add := func(p torrentQuery) error {
		r, err := s.request(p, results, q.Offset)
		if err == nil {
			c.AddRequests(r)
		}
		return err
	}
	if q.Type == indexer.SearchBasic {
		return &c, add(p)
	}
	switch {
	case q.TVDBID > 0:
		p.Tvdb = strconv.Itoa(q.TVDBID)
	case q.RID > 0:
		p.Tvrage = strconv.Itoa(q.RID)
	}
	season := 0
	if q.Season != nil {
		season = *q.Season
	}
	episode, epErr := strconv.Atoi(strings.TrimSpace(q.Episode))
	_, termErr := strconv.Atoi(strings.TrimSpace(q.Term))
	var err error
	switch {
	case season > 0 && strings.TrimSpace(q.Episode) == "":
		p.Category, p.Name = "Season", fmt.Sprintf("Season %d%%", season)
		if err = add(p); err == nil {
			p.Category, p.Name = "Episode", fmt.Sprintf("S%02dE%%", season)
			err = add(p)
		}
	case dailyName(q) != "":
		p.Category, p.Name = "Episode", dailyName(q)
		err = add(p)
	case season > 0 && epErr == nil && episode > 0:
		p.Category, p.Name = "Episode", fmt.Sprintf("S%02dE%02d%%", season, episode)
		if episode < 100 {
			p.Name = fmt.Sprintf("S%02dE%%%02d%%", season, episode)
		}
		err = add(p)
	case strings.TrimSpace(q.Term) != "" && termErr == nil && (q.TVDBID > 0 || q.RID > 0):
		// IDs do not find episodes by absolute numbers.
	default:
		err = add(p)
	}
	return &c, err
}

// dailyName returns the name filter of daily episodes, whose season is a
// year and episode a month and day; "" for others.
func dailyName(q *indexer.Query) string {
	if q.Season == nil {
		return ""
	}
	d, err := time.Parse("2006 01/02", fmt.Sprintf("%d %s", *q.Season, q.Episode))
	if err != nil {
		return ""
	}
	return d.Format("2006.01.02") + "%"
}

func (s *Site) request(p torrentQuery, results, offset int) (*indexer.Request, error) {
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"method":  "getTorrents",
		"params":  []any{s.settings.APIKey, p, results, offset},
		"id":      strings.ToLower(rand.Text()[:8]),
	})
	if err != nil {
		return nil, err
	}
	r := indexer.NewRequest(s.baseURL, "application/json-rpc, application/json")
	r.Method = http.MethodPost
	r.Body = body
	r.Header.Set("Content-Type", "application/json")
	r.AllowedStatus = []int{http.StatusServiceUnavailable}
	return r, nil
}

// torrent is a release of a search.
type torrent struct {
	GroupID     jsonx.Int    `json:"GroupID"`
	TorrentID   jsonx.Int    `json:"TorrentID"`
	Category    jsonx.String `json:"Category"`
	Snatched    *jsonx.Int   `json:"Snatched"`
	Seeders     *jsonx.Int   `json:"Seeders"`
	Leechers    *jsonx.Int   `json:"Leechers"`
	Container   jsonx.String `json:"Container"`
	Resolution  jsonx.String `json:"Resolution"`
	Origin      jsonx.String `json:"Origin"`
	ReleaseName jsonx.String `json:"ReleaseName"`
	Size        jsonx.Int64  `json:"Size"`
	Time        jsonx.Int64  `json:"Time"`
	TvdbID      *jsonx.Int   `json:"TvdbID"`
	TvrageID    *jsonx.Int   `json:"TvrageID"`
	ImdbID      jsonx.String `json:"ImdbID"`
	InfoHash    jsonx.String `json:"InfoHash"`
	DownloadURL jsonx.String `json:"DownloadURL"`
}

var (
	protocolPrefix = regexp.MustCompile(`^https?:`)
	h265           = regexp.MustCompile(`\b(H\.?265)\b`)
	h264           = regexp.MustCompile(`\b(H\.?264)\b`)
)

const callLimit = "Cannot do more than 150 API requests per hour."

// ParseResponse implements indexer.Site, as Prowlarr's
// BroadcastheNetParser does.
func (s *Site) ParseResponse(_ context.Context, _ *indexer.Indexer, r *indexer.Response) ([]*indexer.Release, error) {
	switch r.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized:
		return nil, fmt.Errorf("%w: API Key invalid or not authorized", indexer.ErrAuth)
	case http.StatusNotFound:
		return nil, indexer.Errorf("Indexer API call returned NotFound, the Indexer API may have changed.")
	case http.StatusServiceUnavailable:
		return nil, &indexer.RateLimitError{Message: callLimit}
	default:
		return nil, indexer.Errorf("Indexer API call returned an unexpected StatusCode [%d]", r.StatusCode)
	}
	if strings.Contains(r.Header.Get("Content-Type"), "text/html") {
		return nil, indexer.Errorf("Indexer responded with html content. Site is likely blocked or unavailable.")
	}
	content := r.Content()
	if strings.Contains(strings.ToLower(content), "call limit exceeded") {
		return nil, &indexer.RateLimitError{Message: callLimit}
	}
	if content == "Query execution was interrupted" {
		return nil, indexer.Errorf("Indexer API returned an internal server error")
	}
	var resp struct {
		Error  json.RawMessage `json:"error"`
		Result *struct {
			Torrents json.RawMessage `json:"torrents"`
			Results  jsonx.Int       `json:"results"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(content), &resp); err != nil {
		return nil, fmt.Errorf("broadcasthenet: %w", err)
	}
	if (len(resp.Error) > 0 && string(resp.Error) != "null") || resp.Result == nil {
		return nil, indexer.Errorf("Indexer API call returned an error [%s]", resp.Error)
	}
	if resp.Result.Results == 0 || len(resp.Result.Torrents) == 0 || string(resp.Result.Torrents) == "null" {
		return nil, nil
	}
	protocol := "https:"
	if u, err := url.Parse(r.Request.URL); err == nil && u.Scheme != "" {
		protocol = u.Scheme + ":"
	}
	torrents, err := inOrder(resp.Result.Torrents)
	if err != nil {
		return nil, fmt.Errorf("broadcasthenet: %w", err)
	}
	releases := make([]*indexer.Release, 0, len(torrents))
	for _, t := range torrents {
		releases = append(releases, s.release(t, protocol))
	}
	return releases, nil
}

// inOrder reads the torrents of an object keyed by their IDs, in the
// object's order.
func inOrder(data json.RawMessage) ([]torrent, error) {
	d := json.NewDecoder(bytes.NewReader(data))
	if _, err := d.Token(); err != nil {
		return nil, err
	}
	var out []torrent
	for d.More() {
		if _, err := d.Token(); err != nil {
			return nil, err
		}
		var t torrent
		if err := d.Decode(&t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}

func (s *Site) release(t torrent, protocol string) *indexer.Release {
	origin := strings.ToUpper(string(t.Origin))
	seedTime := int64(86400) // 24 hours for episodes
	if strings.ToUpper(string(t.Category)) == "SEASON" {
		seedTime = 432000 // 120 hours for seasons
	}
	rel := &indexer.Release{
		GUID:                 fmt.Sprintf("BTN-%d", t.TorrentID),
		InfoURL:              fmt.Sprintf("%s//broadcasthe.net/torrents.php?id=%d&torrentid=%d", protocol, t.GroupID, t.TorrentID),
		DownloadURL:          protocolPrefix.ReplaceAllLiteralString(string(t.DownloadURL), protocol),
		Title:                title(t),
		Protocol:             indexer.ProtocolTorrent,
		Categories:           indexer.IDs(s.caps.Categories.MapTracker(string(t.Resolution))),
		InfoHash:             string(t.InfoHash),
		Size:                 indexer.Ptr(int64(t.Size)),
		PublishDate:          time.Unix(int64(t.Time), 0).UTC(),
		Scene:                origin == "SCENE",
		DownloadVolumeFactor: indexer.Ptr(0.0),
		UploadVolumeFactor:   indexer.Ptr(1.0),
		MinimumRatio:         indexer.Ptr(1.0),
		MinimumSeedTime:      indexer.Ptr(seedTime),
	}
	if origin == "INTERNAL" {
		rel.Flags = []indexer.Flag{indexer.FlagInternal}
	}
	if t.Snatched != nil {
		rel.Grabs = indexer.Ptr(int(*t.Snatched))
	}
	if t.Seeders != nil {
		rel.Seeders = indexer.Ptr(int(*t.Seeders))
		if t.Leechers != nil {
			rel.Peers = indexer.Ptr(int(*t.Leechers + *t.Seeders))
		}
	}
	if t.TvdbID != nil && *t.TvdbID > 0 {
		rel.TVDBID = int(*t.TvdbID)
	}
	if t.TvrageID != nil && *t.TvrageID > 0 {
		rel.TVRageID = int(*t.TvrageID)
	}
	if id, err := strconv.Atoi(strings.TrimSpace(string(t.ImdbID))); err == nil {
		rel.IMDbID = id
	}
	if len(rel.Categories) == 0 {
		rel.Categories = []int{indexer.TV.ID}
	}
	return rel
}

// title returns a torrent's release name; Blu-rays and DVDs name their
// codecs as AVC and HEVC.
func title(t torrent) string {
	name := strings.ReplaceAll(string(t.ReleaseName), `\`, "")
	if c := strings.ToUpper(string(t.Container)); c == "M2TS" || c == "ISO" {
		name = h265.ReplaceAllString(name, "HEVC")
		name = h264.ReplaceAllString(name, "AVC")
	}
	return name
}
