package avistaz

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
	"sync"
	"time"

	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
	"github.com/mavioai/mavio/plugins/indexers/internal/jsonx"
	"github.com/mavioai/mavio/plugins/indexers/internal/parse"
)

// Settings are an AvistaZ site's settings.
type Settings struct {
	indexer.BaseSettings
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	// PID is the account's PID, found on its profile page.
	PID           string `json:"pid,omitempty"`
	FreeleechOnly bool   `json:"freeleech_only,omitempty"`
}

// Validate checks that the user name, password and PID are set.
func (s Settings) Validate() error {
	var errs []error
	for _, f := range []struct{ name, v string }{{"username", s.Username}, {"password", s.Password}, {"pid", s.PID}} {
		if strings.TrimSpace(f.v) == "" {
			errs = append(errs, fmt.Errorf("%s: must not be empty", f.name))
		}
	}
	return errors.Join(errs...)
}

// site is how an AvistaZ site differs from the others.
type site struct {
	links []string
	caps  func() *indexer.Capabilities
	// params replaces the parameters every search has.
	params func(s *Site, q *indexer.Query, genre string) *query
	// episode returns the episode part of TV search terms; the query's
	// episode string when nil.
	episode func(q *indexer.Query) string
	// categories maps a result's categories; by its type and video
	// quality when nil.
	categories func(s *Site, r *result) ([]int, error)
	// title returns a result's title; its file name when nil.
	title func(r *result) string
}

// Site is an AvistaZ site, as Prowlarr's AvistazBase is.
type Site struct {
	site
	settings Settings
	baseURL  string
	caps     *indexer.Capabilities

	mu    sync.Mutex
	token string
}

// pageSize is the most results of a search.
const pageSize = 50

func definition(d indexer.Definition, s site) indexer.Definition {
	d.Protocol = indexer.ProtocolTorrent
	d.Privacy = indexer.Private
	d.Links = s.links
	d.Fields = []indexer.Field{
		indexer.BaseURLField(s.links),
		{Name: "username", Label: "Username", Type: indexer.FieldText, HelpText: "Site Username"},
		{Name: "password", Label: "Password", Type: indexer.FieldText, Secret: true, HelpText: "Site Password"},
		{Name: "pid", Label: "PID", Type: indexer.FieldText, Secret: true, HelpText: "PID from My Account or My Profile page"},
		{Name: "freeleech_only", Label: "Freeleech Only", Type: indexer.FieldCheckbox, HelpText: "Search freeleech releases only"},
	}
	d.New = func(data []byte) (indexer.Site, error) {
		var settings Settings
		if err := indexer.DecodeSettings(data, &settings); err != nil {
			return nil, err
		}
		if err := settings.Validate(); err != nil {
			return nil, err
		}
		return &Site{site: s, settings: settings, baseURL: settings.URL(s.links, nil), caps: s.caps()}, nil
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
		RateLimit:      6 * time.Second,
	}
}

// Capabilities implements indexer.Site.
func (s *Site) Capabilities(context.Context, *indexer.Indexer) (*indexer.Capabilities, error) {
	return s.caps, nil
}

// query is a search's parameters, in order and repeatable.
type query []struct{ key, value string }

func (q *query) add(k, v string) { *q = append(*q, struct{ key, value string }{k, v}) }

// baseCapabilities returns the capabilities of AvistaZ's movie and TV
// sites: their movie and TV types.
func baseCapabilities(tv, movie []indexer.Param) *indexer.Capabilities {
	c := indexer.NewCapabilities()
	c.LimitsDefault, c.LimitsMax = pageSize, pageSize
	c.Params[indexer.SearchTV] = tv
	c.Params[indexer.SearchMovie] = movie
	for _, m := range []struct {
		tracker string
		c       *indexer.Category
	}{
		{"1", indexer.Movies},
		{"1", indexer.MoviesUHD},
		{"1", indexer.MoviesHD},
		{"1", indexer.MoviesSD},
		{"2", indexer.TV},
		{"2", indexer.TVUHD},
		{"2", indexer.TVHD},
		{"2", indexer.TVSD},
	} {
		c.Categories.Add(m.tracker, m.c, "")
	}
	return c
}

// limit returns the number of results to ask for.
func limit(q *indexer.Query) string {
	l := pageSize
	if q.Limit > 0 {
		l = min(l, q.Limit)
	}
	return strconv.Itoa(l)
}

// basicParams returns the parameters every search has, as Prowlarr's
// AvistazRequestGenerator does: the type, paging, freeleech, the genre,
// and video qualities for queries of only some.
func basicParams(s *Site, q *indexer.Query, genre string) *query {
	trackers := slices.Compact(s.caps.Categories.MapToTracker(q.Categories, false))
	typ := "0"
	if len(trackers) == 1 {
		typ = trackers[0]
	}
	p := &query{}
	p.add("in", "1")
	p.add("type", typ)
	p.add("limit", limit(q))
	if q.Limit > 0 && q.Offset > 0 {
		p.add("page", strconv.Itoa(q.Offset/q.Limit+1))
	}
	if s.settings.FreeleechOnly {
		p.add("discount[]", "1")
	}
	if strings.TrimSpace(genre) != "" {
		p.add("tags", genre)
	}
	has := func(cs ...*indexer.Category) bool {
		return slices.ContainsFunc(cs, func(c *indexer.Category) bool { return slices.Contains(q.Categories, c.ID) })
	}
	if !has(indexer.Movies, indexer.TV, indexer.Audio) {
		if has(indexer.MoviesUHD, indexer.TVUHD) {
			p.add("video_quality[]", "6")
		}
		if has(indexer.MoviesHD, indexer.TVHD) {
			p.add("video_quality[]", "2")
			p.add("video_quality[]", "7")
			p.add("video_quality[]", "3")
		}
		if has(indexer.MoviesSD, indexer.TVSD) {
			p.add("video_quality[]", "1")
		}
	}
	return p
}

// SearchRequests implements indexer.Site.
func (s *Site) SearchRequests(_ context.Context, ix *indexer.Indexer, q *indexer.Query) (*indexer.Chain, error) {
	params := s.params
	if params == nil {
		params = basicParams
	}
	var c indexer.Chain
	switch q.Type {
	case indexer.SearchBook:
		return &c, nil
	case indexer.SearchMovie:
		p := params(s, q, q.Genre)
		switch {
		case strings.TrimSpace(q.IMDbID) != "":
			p.add("imdb", q.FullIMDbID())
		case q.TMDBID != 0:
			p.add("tmdb", strconv.Itoa(q.TMDBID))
		default:
			p.add("search", strings.TrimSpace(q.SanitizedTerm()))
		}
		c.AddRequests(s.request(ix, p))
	case indexer.SearchTV:
		p := params(s, q, q.Genre)
		episode := q.EpisodeString()
		if s.episode != nil {
			episode = s.episode(q)
		}
		switch {
		case strings.TrimSpace(q.IMDbID) != "":
			p.add("imdb", q.FullIMDbID())
			p.add("search", strings.TrimSpace(episode))
		case q.TVDBID != 0:
			p.add("tvdb", strconv.Itoa(q.TVDBID))
			p.add("search", strings.TrimSpace(episode))
		default:
			p.add("search", strings.TrimSpace(q.SanitizedTerm()+" "+episode))
		}
		c.AddRequests(s.request(ix, p))
	default:
		p := params(s, q, "")
		p.add("search", strings.TrimSpace(q.SanitizedTerm()))
		c.AddRequests(s.request(ix, p))
	}
	return &c, nil
}

func (s *Site) request(ix *indexer.Indexer, p *query) *indexer.Request {
	parts := make([]string, len(*p))
	for i, e := range *p {
		parts[i] = e.key + "=" + indexer.URLEncode(e.value)
	}
	r := indexer.NewRequest(s.baseURL+"api/v1/jackett/torrents?"+strings.Join(parts, "&"), indexer.AcceptJSON)
	// Prowlarr's IndexerRequest of a URL follows redirects.
	r.FollowRedirects = true
	r.Header.Set("Authorization", "Bearer "+s.getToken(ix))
	r.AllowedStatus = []int{http.StatusNotFound}
	return r
}

// tokenState is the key of the token kept with the indexer's state.
const tokenState = "token"

// getToken returns the API token: the one got last, else the one kept
// with the indexer's state when ix is given, as Prowlarr keeps the token
// with the indexer's settings.
func (s *Site) getToken(ix *indexer.Indexer) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.token == "" && ix != nil && ix.Statuses != nil {
		s.token = ix.State(tokenState)
	}
	return s.token
}

// LoginNeeded implements indexer.LoginSite: on HTTP 401 and 412.
func (s *Site) LoginNeeded(r *indexer.Response) bool {
	return r.StatusCode == http.StatusUnauthorized || r.StatusCode == http.StatusPreconditionFailed
}

// Login implements indexer.LoginSite: it gets a new token for the API.
// The token is kept with the indexer's state until the site rejects it.
func (s *Site) Login(ctx context.Context, ix *indexer.Indexer) error {
	token, err := s.newToken(ctx, ix)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.token = token
	s.mu.Unlock()
	if ix.Statuses != nil {
		ix.SetState(tokenState, token)
	}
	return nil
}

func (s *Site) newToken(ctx context.Context, ix *indexer.Indexer) (string, error) {
	var form indexer.Params
	form.Set("username", s.settings.Username)
	form.Set("password", s.settings.Password)
	form.Set("pid", strings.TrimSpace(s.settings.PID))
	r := indexer.NewRequest(s.baseURL+"api/v1/jackett/auth", indexer.AcceptJSON)
	r.Method = http.MethodPost
	r.Body = []byte(form.Encode())
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := ix.Do(ctx, r)
	if err != nil {
		return "", fmt.Errorf("avistaz login: %w", err)
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusUnprocessableEntity {
		return "", fmt.Errorf("%w: %s", indexer.ErrAuth, errorMessage(resp, "Unauthorized request to indexer"))
	}
	if resp.HasError() {
		return "", fmt.Errorf("avistaz login: %w", &indexer.HTTPError{Response: resp})
	}
	var auth struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal([]byte(resp.Content()), &auth); err != nil {
		return "", errors.New("invalid response from AvistaZ, the response is not valid JSON")
	}
	return auth.Token, nil
}

// errorMessage returns the message of an API error; def when none.
func errorMessage(r *indexer.Response, def string) string {
	var e struct {
		Message string `json:"message"`
	}
	if json.Unmarshal([]byte(r.Content()), &e) == nil && e.Message != "" {
		return e.Message
	}
	return def
}

// ModifyRequest implements indexer.RequestModifier: requests sent again
// after logging in carry the new token.
func (s *Site) ModifyRequest(r *indexer.Request) {
	r.Header.Set("Authorization", "Bearer "+s.getToken(nil))
}

// TestSite implements indexer.SiteTester: the credentials get a token.
func (s *Site) TestSite(ctx context.Context, ix *indexer.Indexer) error {
	_, err := s.newToken(ctx, ix)
	return err
}

// result is a release of a search.
type result struct {
	URL      jsonx.String            `json:"url"`
	Download jsonx.String            `json:"download"`
	Category map[string]jsonx.String `json:"category"`
	MovieTV  *struct {
		TMDB jsonx.String `json:"tmdb"`
		TVDB jsonx.String `json:"tvdb"`
		IMDb jsonx.String `json:"imdb"`
	} `json:"movie_tv"`
	CreatedAtISO   jsonx.String `json:"created_at_iso"`
	FileName       jsonx.String `json:"file_name"`
	ReleaseTitle   jsonx.String `json:"release_title"`
	InfoHash       jsonx.String `json:"info_hash"`
	Leech          *jsonx.Int   `json:"leech"`
	Completed      *jsonx.Int   `json:"completed"`
	Seed           *jsonx.Int   `json:"seed"`
	FileSize       *jsonx.Int64 `json:"file_size"`
	FileCount      *jsonx.Int   `json:"file_count"`
	DownloadFactor *jsonx.Float `json:"download_multiply"`
	UploadFactor   *jsonx.Float `json:"upload_multiply"`
	VideoQuality   jsonx.String `json:"video_quality"`
	Type           jsonx.String `json:"type"`
	Format         jsonx.String `json:"format"`
	Audio          []language   `json:"audio"`
	Subtitle       []language   `json:"subtitle"`
}

type language struct {
	Language jsonx.String `json:"language"`
}

// ParseResponse implements indexer.Site, as Prowlarr's AvistazParserBase
// does.
func (s *Site) ParseResponse(_ context.Context, _ *indexer.Indexer, r *indexer.Response) ([]*indexer.Release, error) {
	switch {
	case r.StatusCode == http.StatusNotFound:
		return nil, nil
	case r.StatusCode == http.StatusTooManyRequests:
		return nil, &indexer.RateLimitError{Message: "API Request Limit Reached"}
	case r.StatusCode == http.StatusUnauthorized:
		return nil, fmt.Errorf("%w: %s", indexer.ErrAuth, errorMessage(r, "Unauthorized request to indexer"))
	case r.StatusCode != http.StatusOK:
		return nil, indexer.Errorf("Unexpected response status %d code from indexer request", r.StatusCode)
	case !r.HasContentType(indexer.AcceptJSON):
		return nil, indexer.Errorf("Unexpected response header %s from indexer request, expected %s", r.Header.Get("Content-Type"), indexer.AcceptJSON)
	}
	var resp struct {
		Data []result `json:"data"`
	}
	if err := json.Unmarshal([]byte(r.Content()), &resp); err != nil {
		return nil, fmt.Errorf("avistaz: %w", err)
	}
	releases := make([]*indexer.Release, 0, len(resp.Data))
	for i := range resp.Data {
		rel, err := s.release(&resp.Data[i])
		if err != nil {
			return nil, err
		}
		releases = append(releases, rel)
	}
	slices.SortStableFunc(releases, func(a, b *indexer.Release) int { return b.PublishDate.Compare(a.PublishDate) })
	return releases, nil
}

func (s *Site) release(row *result) (*indexer.Release, error) {
	categories := s.categories
	if categories == nil {
		categories = typeCategories
	}
	cats, err := categories(s, row)
	if err != nil {
		return nil, err
	}
	date, err := indexer.ParseDate(string(row.CreatedAtISO))
	if err != nil {
		return nil, fmt.Errorf("avistaz: %w", err)
	}
	title := string(row.FileName)
	if s.title != nil {
		title = s.title(row)
	}
	rel := &indexer.Release{
		GUID:            string(row.URL),
		InfoURL:         string(row.URL),
		Title:           title,
		DownloadURL:     string(row.Download),
		Protocol:        indexer.ProtocolTorrent,
		Categories:      cats,
		InfoHash:        string(row.InfoHash),
		PublishDate:     date,
		MinimumRatio:    indexer.Ptr(1.0),
		MinimumSeedTime: indexer.Ptr(int64(259200)),
	}
	if row.FileSize != nil {
		rel.Size = indexer.Ptr(int64(*row.FileSize))
	}
	if row.FileCount != nil {
		rel.Files = indexer.Ptr(int(*row.FileCount))
	}
	if row.Completed != nil {
		rel.Grabs = indexer.Ptr(int(*row.Completed))
	}
	if row.Seed != nil {
		rel.Seeders = indexer.Ptr(int(*row.Seed))
		if row.Leech != nil {
			rel.Peers = indexer.Ptr(int(*row.Leech + *row.Seed))
		}
	}
	if row.DownloadFactor != nil {
		rel.DownloadVolumeFactor = indexer.Ptr(float64(*row.DownloadFactor))
	}
	if row.UploadFactor != nil {
		rel.UploadVolumeFactor = indexer.Ptr(float64(*row.UploadFactor))
	}
	for _, a := range row.Audio {
		rel.Languages = append(rel.Languages, string(a.Language))
	}
	for _, a := range row.Subtitle {
		rel.Subtitles = append(rel.Subtitles, string(a.Language))
	}
	if row.FileSize != nil && *row.FileSize > 0 {
		gb := float64(*row.FileSize) / (1024 * 1024 * 1024)
		if gb > 50 {
			rel.MinimumSeedTime = indexer.Ptr(int64(100*math.Log(gb)-219.2023) * 3600)
		} else {
			rel.MinimumSeedTime = indexer.Ptr(259200 + int64(gb*7200))
		}
	}
	if m := row.MovieTV; m != nil {
		if id, ok := parse.IMDbID(string(m.IMDb)); ok {
			rel.IMDbID = id
		}
		rel.TMDBID = coerceInt(string(m.TMDB))
		rel.TVDBID = coerceInt(string(m.TVDB))
	}
	return rel, nil
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

// typeCategories maps a result's type and video quality.
func typeCategories(_ *Site, r *result) ([]int, error) {
	q := string(r.VideoQuality)
	hd := q == "1080p" || q == "1080i" || q == "720p"
	switch strings.ToUpper(string(r.Type)) {
	case "MOVIE":
		switch {
		case hd:
			return []int{indexer.MoviesHD.ID}, nil
		case q == "2160p":
			return []int{indexer.MoviesUHD.ID}, nil
		}
		return []int{indexer.MoviesSD.ID}, nil
	case "TV-SHOW":
		switch {
		case hd:
			return []int{indexer.TVHD.ID}, nil
		case q == "2160p":
			return []int{indexer.TVUHD.ID}, nil
		}
		return []int{indexer.TVSD.ID}, nil
	case "MUSIC":
		return []int{indexer.Audio.ID}, nil
	}
	return nil, fmt.Errorf("error parsing Avistaz category type %q", r.Type)
}
