package nebulance

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
	"github.com/mavioai/mavio/plugins/indexers/internal/jsonx"
	"github.com/mavioai/mavio/plugins/indexers/internal/parse"
)

var links = []string{"https://nebulance.io/"}

// Definition is Nebulance.
var Definition = indexer.Definition{
	ID:          "nebulance",
	Name:        "Nebulance",
	Description: "Nebulance (NBL) is a ratioless Private Torrent Tracker for TV",
	Language:    "en-US",
	Protocol:    indexer.ProtocolTorrent,
	Privacy:     indexer.Private,
	Links:       links,
	Fields: []indexer.Field{
		indexer.BaseURLField(links),
		{Name: "api_key", Label: "API Key", Type: indexer.FieldText, Secret: true, HelpText: "API Key from User Settings > Api Keys. Key must have List and Download permissions"},
	},
	New: newSite,
}

// Settings are Nebulance's settings.
type Settings struct {
	indexer.BaseSettings
	APIKey string `json:"api_key,omitempty"`
}

// Site is Nebulance.
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
	return &Site{settings: settings, baseURL: settings.URL(links, nil), caps: capabilities()}, nil
}

func capabilities() *indexer.Capabilities {
	c := indexer.NewCapabilities()
	c.Params[indexer.SearchTV] = []indexer.Param{indexer.ParamQ, indexer.ParamSeason, indexer.ParamEp, indexer.ParamIMDbID, indexer.ParamTVMazeID}
	c.RawSearch = true
	c.Categories.Add("1", indexer.TV, "")
	c.Categories.Add("2", indexer.TVSD, "")
	c.Categories.Add("3", indexer.TVHD, "")
	c.Categories.Add("4", indexer.TVUHD, "")
	return c
}

// Info implements indexer.Site.
func (s *Site) Info() indexer.Info {
	return indexer.Info{
		Protocol:           indexer.ProtocolTorrent,
		Privacy:            indexer.Private,
		SupportsRSS:        true,
		SupportsSearch:     true,
		SupportsPagination: true,
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

// SearchRequests implements indexer.Site, as Prowlarr's
// NebulanceRequestGenerator does: only TV and basic searches, which the
// API takes with names of three characters or more.
func (s *Site) SearchRequests(ctx context.Context, _ *indexer.Indexer, q *indexer.Query) (*indexer.Chain, error) {
	var c indexer.Chain
	var p indexer.Params
	p.Set("action", "search")
	p.Set("age", ">0")
	term := strings.TrimSpace(q.SanitizedTerm())
	switch q.Type {
	case indexer.SearchTV:
		switch {
		case q.TVMazeID > 0:
			p.Set("tvmaze", strconv.Itoa(q.TVMazeID))
		case strings.TrimSpace(q.IMDbID) != "":
			p.Set("imdb", q.FullIMDbID())
		}
		if term != "" {
			p.Set("release", term)
		}
		if d, ok := dailyDate(q); ok {
			if term != "" {
				p.Set("name", term)
			}
			p.Set("release", d.Format("2006.01.02"))
		} else {
			if q.Season != nil {
				p.Set("season", strconv.Itoa(*q.Season))
			}
			if n, err := strconv.ParseInt(strings.TrimSpace(q.Episode), 10, 32); err == nil {
				p.Set("episode", strconv.FormatInt(n, 10))
			}
		}
		if (has(&p, "season") || has(&p, "episode")) && !has(&p, "name") && !has(&p, "release") && !has(&p, "tvmaze") && !has(&p, "imdb") {
			slog.WarnContext(ctx, "NBL API does not support season calls without name, series, id, imdb, tvmaze, or time keys.")
			return &c, nil
		}
		if tooShort(&p, "name") || tooShort(&p, "release") {
			slog.WarnContext(ctx, "NBL API does not support release calls that are 2 characters or fewer.")
			return &c, nil
		}
	case indexer.SearchBasic:
		if term != "" {
			p.Set("release", term)
		}
		if tooShort(&p, "release") {
			slog.DebugContext(ctx, "NBL API does not support release calls that are 2 characters or fewer.")
			return &c, nil
		}
	default:
		return &c, nil
	}
	p.Set("api_key", s.settings.APIKey)
	limit := q.Limit
	if limit == 0 {
		limit = 100
	}
	p.Set("per_page", strconv.Itoa(limit))
	if q.Limit > 0 && q.Offset > 0 {
		p.Set("page", strconv.Itoa(q.Offset/q.Limit))
	}
	r := indexer.NewRequest(s.baseURL+"api.php?"+p.Encode(), indexer.AcceptJSON)
	// Prowlarr's IndexerRequest of a URL follows redirects.
	r.FollowRedirects = true
	c.AddRequests(r)
	return &c, nil
}

// dailyDate returns the date of a daily episode, whose season is a year
// and episode a month and day.
func dailyDate(q *indexer.Query) (time.Time, bool) {
	if q.Season == nil || strings.TrimSpace(q.Episode) == "" {
		return time.Time{}, false
	}
	d, err := time.Parse("2006 01/02", fmt.Sprintf("%d %s", *q.Season, q.Episode))
	return d, err == nil
}

func has(p *indexer.Params, key string) bool {
	v, _ := p.Get(key)
	return strings.TrimSpace(v) != ""
}

// tooShort reports whether a parameter is set to one or two characters.
func tooShort(p *indexer.Params, key string) bool {
	v, _ := p.Get(key)
	n := len([]rune(v))
	return n > 0 && n < 3
}

// torrent is a release of a search.
type torrent struct {
	ReleaseTitle jsonx.String   `json:"rls_name"`
	Category     jsonx.String   `json:"cat"`
	Size         jsonx.Int64    `json:"size"`
	Seed         jsonx.Int      `json:"seed"`
	Leech        jsonx.Int      `json:"leech"`
	Snatch       jsonx.Int      `json:"snatch"`
	DownloadLink jsonx.String   `json:"download"`
	FileList     []string       `json:"file_list"`
	GroupName    jsonx.String   `json:"group_name"`
	TorrentID    jsonx.Int      `json:"group_id"`
	IMDbID       jsonx.String   `json:"imdb_id"`
	TVMazeID     *jsonx.Int     `json:"tvmaze_id"`
	PublishDate  jsonx.String   `json:"rls_utc"`
	Tags         []jsonx.String `json:"tags"`
}

// ParseResponse implements indexer.Site, as Prowlarr's NebulanceParser
// does.
func (s *Site) ParseResponse(_ context.Context, _ *indexer.Indexer, r *indexer.Response) ([]*indexer.Release, error) {
	if r.StatusCode != http.StatusOK {
		return nil, indexer.Errorf("Unexpected response status '%d' code from indexer request. Check the logs for more information.", r.StatusCode)
	}
	var resp struct {
		TotalResults jsonx.Int `json:"total_results"`
		Items        []torrent `json:"items"`
		Error        *struct {
			Message jsonx.String `json:"message"`
		} `json:"error"`
	}
	content := r.Content()
	if err := json.Unmarshal([]byte(content), &resp); err != nil {
		msg := err.Error()
		var rpc struct {
			Result *string `json:"result"`
		}
		if json.Unmarshal([]byte(content), &rpc) == nil && rpc.Result != nil {
			msg = *rpc.Result
		}
		return nil, indexer.Errorf("Unexpected response from indexer request: %s", msg)
	}
	if resp.Error != nil {
		return nil, indexer.Errorf("Indexer API call returned an error [%s]", resp.Error.Message)
	}
	if resp.TotalResults == 0 || len(resp.Items) == 0 {
		return nil, nil
	}
	releases := make([]*indexer.Release, 0, len(resp.Items))
	for i := range resp.Items {
		rel, err := s.release(&resp.Items[i])
		if err != nil {
			return nil, err
		}
		releases = append(releases, rel)
	}
	return releases, nil
}

func (s *Site) release(row *torrent) (*indexer.Release, error) {
	date, err := indexer.ParseDate(string(row.PublishDate))
	if err != nil {
		return nil, fmt.Errorf("nebulance: %w", err)
	}
	details := s.baseURL + "torrents.php?id=" + strconv.Itoa(int(row.TorrentID))
	title := string(row.ReleaseTitle)
	if strings.TrimSpace(title) == "" {
		title = string(row.GroupName)
	}
	// Seasons are seeded for 120 hours, episodes for 24.
	seedTime := int64(86400)
	if strings.ToUpper(string(row.Category)) == "SEASON" {
		seedTime = 432000
	}
	rel := &indexer.Release{
		GUID:        details,
		InfoURL:     details,
		DownloadURL: string(row.DownloadLink),
		Title:       strings.TrimSpace(title),
		Categories:  []int{tvCategory(string(row.ReleaseTitle)).ID},
		Size:        indexer.Ptr(int64(row.Size)),
		Files:       indexer.Ptr(len(row.FileList)),
		PublishDate: date,
		Grabs:       indexer.Ptr(int(row.Snatch)),
		Seeders:     indexer.Ptr(int(row.Seed)),
		Peers:       indexer.Ptr(int(row.Seed + row.Leech)),
		// The tracker is ratioless.
		MinimumRatio:         indexer.Ptr(0.0),
		MinimumSeedTime:      indexer.Ptr(seedTime),
		DownloadVolumeFactor: indexer.Ptr(0.0),
		UploadVolumeFactor:   indexer.Ptr(1.0),
	}
	for _, t := range row.Tags {
		if strings.EqualFold(string(t), "scene") {
			rel.Scene = true
		}
	}
	if strings.TrimSpace(string(row.IMDbID)) != "" {
		if id, ok := parse.IMDbID(string(row.IMDbID)); ok {
			rel.IMDbID = id
		}
	}
	if row.TVMazeID != nil && *row.TVMazeID > 0 {
		rel.TVMazeID = int(*row.TVMazeID)
	}
	return rel, nil
}

// DownloadRequest implements indexer.DownloadSite: downloads are sent
// without the site's cookies, which would redirect them to the login
// page.
func (s *Site) DownloadRequest(_ context.Context, _ *indexer.Indexer, link string) (*indexer.Request, error) {
	r := indexer.NewRequest(link, "")
	r.Cookies = map[string]string{}
	return r, nil
}
