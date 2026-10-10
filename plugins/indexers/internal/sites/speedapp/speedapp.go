package speedapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
	"github.com/mavioai/mavio/plugins/indexers/internal/jsonx"
	"github.com/mavioai/mavio/plugins/indexers/internal/parse"
)

// Settings are a SpeedApp site's settings.
type Settings struct {
	indexer.BaseSettings
	Email    string `json:"email,omitempty"`
	Password string `json:"password,omitempty"`
}

// Validate checks that the email and password are set.
func (s Settings) Validate() error {
	var errs []error
	if strings.TrimSpace(s.Email) == "" {
		errs = append(errs, errors.New("email: must not be empty"))
	}
	if strings.TrimSpace(s.Password) == "" {
		errs = append(errs, errors.New("password: must not be empty"))
	}
	return errors.Join(errs...)
}

// site is how a SpeedApp site differs from the others.
type site struct {
	links, legacy []string
	categories    func(c *indexer.Categories)
	// rateLimit is the time between requests; 2 seconds when 0.
	rateLimit time.Duration
	// minimumSeedTime is in seconds.
	minimumSeedTime int64
}

// Site is a SpeedApp site, as Prowlarr's SpeedAppBase is.
type Site struct {
	site
	settings Settings
	baseURL  string
	caps     *indexer.Capabilities

	mu    sync.Mutex
	token string
}

// pageSize is the most results of a search.
const pageSize = 100

func definition(d indexer.Definition, s site) indexer.Definition {
	d.Protocol = indexer.ProtocolTorrent
	d.Privacy = indexer.Private
	d.Links = s.links
	d.LegacyLinks = s.legacy
	d.Fields = []indexer.Field{
		indexer.BaseURLField(s.links),
		{Name: "email", Label: "Email", Type: indexer.FieldText, HelpText: "Site Email"},
		{Name: "password", Label: "Password", Type: indexer.FieldText, Secret: true, HelpText: "Site Password"},
	}
	d.New = func(data []byte) (indexer.Site, error) {
		var settings Settings
		if err := indexer.DecodeSettings(data, &settings); err != nil {
			return nil, err
		}
		if err := settings.Validate(); err != nil {
			return nil, err
		}
		c := indexer.NewCapabilities()
		c.LimitsDefault, c.LimitsMax = pageSize, pageSize
		c.Params[indexer.SearchTV] = []indexer.Param{indexer.ParamQ, indexer.ParamIMDbID, indexer.ParamSeason, indexer.ParamEp}
		c.Params[indexer.SearchMovie] = []indexer.Param{indexer.ParamQ, indexer.ParamIMDbID}
		c.Params[indexer.SearchMusic] = []indexer.Param{indexer.ParamQ}
		c.Params[indexer.SearchBook] = []indexer.Param{indexer.ParamQ}
		s.categories(&c.Categories)
		return &Site{site: s, settings: settings, baseURL: settings.URL(s.links, s.legacy), caps: c}, nil
	}
	return d
}

// Info implements indexer.Site.
func (s *Site) Info() indexer.Info {
	return indexer.Info{
		Protocol:       indexer.ProtocolTorrent,
		Privacy:        indexer.Private,
		SupportsRSS:    true,
		SupportsSearch: true,
		PageSize:       pageSize,
		RateLimit:      s.rateLimit,
	}
}

// Capabilities implements indexer.Site.
func (s *Site) Capabilities(context.Context, *indexer.Indexer) (*indexer.Capabilities, error) {
	return s.caps, nil
}

// SearchRequests implements indexer.Site, as Prowlarr's
// SpeedAppRequestGenerator does.
func (s *Site) SearchRequests(_ context.Context, _ *indexer.Indexer, q *indexer.Query) (*indexer.Chain, error) {
	var p indexer.Params
	l := pageSize
	if q.Limit > 0 {
		l = min(l, q.Limit)
	}
	p.Set("itemsPerPage", strconv.Itoa(l))
	p.Set("sort", "torrent.createdAt")
	p.Set("direction", "desc")
	if q.Limit > 0 && q.Offset > 0 {
		p.Set("page", strconv.Itoa(q.Offset/q.Limit+1))
	}
	imdb := ""
	if q.Type == indexer.SearchMovie || q.Type == indexer.SearchTV {
		imdb = q.FullIMDbID()
	}
	if strings.TrimSpace(imdb) != "" {
		p.Set("imdbId", imdb)
	} else {
		p.Set("search", q.SanitizedTerm())
	}
	if q.Type == indexer.SearchTV {
		if q.Season != nil {
			p.Set("season", strconv.Itoa(*q.Season))
		}
		if q.Episode != "" {
			p.Set("episode", q.Episode)
		}
	}
	parts := []string{p.Encode()}
	for _, c := range s.caps.Categories.MapToTracker(q.Categories, false) {
		parts = append(parts, "categories[]="+indexer.URLEncode(c))
	}
	r := indexer.NewRequest(s.baseURL+"api/torrent?"+strings.Join(parts, "&"), indexer.AcceptJSON)
	s.ModifyRequest(r)
	var c indexer.Chain
	c.AddRequests(r)
	return &c, nil
}

func (s *Site) getToken() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.token
}

// LoginNeeded implements indexer.LoginSite: without a token, or on HTTP
// 401.
func (s *Site) LoginNeeded(r *indexer.Response) bool {
	return s.getToken() == "" || r.StatusCode == http.StatusUnauthorized
}

// Login implements indexer.LoginSite: it gets a new token for the API.
// The token is kept until the site rejects it, but not across restarts.
func (s *Site) Login(ctx context.Context, ix *indexer.Indexer) error {
	token, err := s.newToken(ctx, ix)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.token = token
	s.mu.Unlock()
	return nil
}

func (s *Site) newToken(ctx context.Context, ix *indexer.Indexer) (string, error) {
	body, err := json.Marshal(struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}{s.settings.Email, s.settings.Password})
	if err != nil {
		return "", err
	}
	r := indexer.NewRequest(s.baseURL+"api/login", indexer.AcceptJSON)
	r.Method = http.MethodPost
	r.Body = body
	r.Header.Set("Content-Type", "application/json")
	resp, err := ix.Do(ctx, r)
	if err != nil {
		return "", fmt.Errorf("speedapp login: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", fmt.Errorf("speedapp login: %w", &indexer.HTTPError{Response: resp})
	}
	var auth struct {
		Token jsonx.String `json:"token"`
	}
	if err := json.Unmarshal([]byte(resp.Content()), &auth); err != nil {
		return "", fmt.Errorf("speedapp login: %w", err)
	}
	return string(auth.Token), nil
}

// ModifyRequest implements indexer.RequestModifier: requests carry the
// token.
func (s *Site) ModifyRequest(r *indexer.Request) {
	r.Header.Set("Authorization", "Bearer "+s.getToken())
}

// DownloadRequest implements indexer.DownloadSite: downloads carry the
// token, logging in first when there is none yet.
func (s *Site) DownloadRequest(ctx context.Context, ix *indexer.Indexer, link string) (*indexer.Request, error) {
	if s.getToken() == "" {
		if err := s.Login(ctx, ix); err != nil {
			return nil, err
		}
	}
	r := indexer.NewRequest(link, "")
	s.ModifyRequest(r)
	return r, nil
}

// TestSite implements indexer.SiteTester: the credentials get a token.
func (s *Site) TestSite(ctx context.Context, ix *indexer.Indexer) error {
	return s.Login(ctx, ix)
}

// CleanupReleases implements indexer.ReleaseCleaner: the site searches
// loosely, so releases without the query's terms are dropped.
func (s *Site) CleanupReleases(releases []*indexer.Release, q *indexer.Query) []*indexer.Release {
	return indexer.FilterByQuery(releases, q)
}

// torrent is a release of a search.
type torrent struct {
	DownloadFactor jsonx.Float  `json:"download_volume_factor"`
	UploadFactor   jsonx.Float  `json:"upload_volume_factor"`
	URL            jsonx.String `json:"url"`
	ID             jsonx.Int    `json:"id"`
	Name           jsonx.String `json:"name"`
	Category       struct {
		ID jsonx.Int `json:"id"`
	} `json:"category"`
	Size             jsonx.Int64  `json:"size"`
	CreatedAt        jsonx.Time   `json:"created_at"`
	TimesCompleted   jsonx.Int    `json:"times_completed"`
	Leechers         jsonx.Int    `json:"leechers"`
	Seeders          jsonx.Int    `json:"seeders"`
	ShortDescription jsonx.String `json:"short_description"`
	Poster           jsonx.String `json:"poster"`
	IMDbID           jsonx.String `json:"imdb_id"`
}

var requestTag = regexp.MustCompile(`(?i)\[REQUEST(ED)?\]`)

// ParseResponse implements indexer.Site, as Prowlarr's SpeedAppParser
// does.
func (s *Site) ParseResponse(_ context.Context, _ *indexer.Indexer, r *indexer.Response) ([]*indexer.Release, error) {
	if r.StatusCode != http.StatusOK {
		return nil, indexer.Errorf("Unexpected response status %d code from indexer request", r.StatusCode)
	}
	if !r.HasContentType(indexer.AcceptJSON) {
		return nil, indexer.Errorf("Unexpected response header %s from indexer request, expected %s", r.Header.Get("Content-Type"), indexer.AcceptJSON)
	}
	var torrents []torrent
	if err := json.Unmarshal([]byte(r.Content()), &torrents); err != nil {
		return nil, fmt.Errorf("speedapp: %w", err)
	}
	releases := make([]*indexer.Release, 0, len(torrents))
	for _, t := range torrents {
		rel := &indexer.Release{
			GUID:                 string(t.URL),
			Title:                strings.Trim(requestTag.ReplaceAllString(string(t.Name), ""), " ."),
			Description:          string(t.ShortDescription),
			Size:                 indexer.Ptr(int64(t.Size)),
			DownloadURL:          s.baseURL + "api/torrent/" + strconv.Itoa(int(t.ID)) + "/download",
			PosterURL:            string(t.Poster),
			InfoURL:              string(t.URL),
			Protocol:             indexer.ProtocolTorrent,
			Grabs:                indexer.Ptr(int(t.TimesCompleted)),
			PublishDate:          t.CreatedAt.Time,
			Categories:           indexer.IDs(s.caps.Categories.MapTracker(strconv.Itoa(int(t.Category.ID)))),
			Seeders:              indexer.Ptr(int(t.Seeders)),
			Peers:                indexer.Ptr(int(t.Leechers + t.Seeders)),
			MinimumRatio:         indexer.Ptr(1.0),
			MinimumSeedTime:      indexer.Ptr(s.minimumSeedTime),
			DownloadVolumeFactor: indexer.Ptr(float64(t.DownloadFactor)),
			UploadVolumeFactor:   indexer.Ptr(float64(t.UploadFactor)),
		}
		if id, ok := parse.IMDbID(string(t.IMDbID)); ok {
			rel.IMDbID = id
		}
		releases = append(releases, rel)
	}
	return releases, nil
}
