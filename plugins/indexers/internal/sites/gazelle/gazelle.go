package gazelle

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
	"github.com/mavioai/mavio/plugins/indexers/internal/jsonx"
	"github.com/mavioai/mavio/plugins/indexers/internal/parse"
)

// site is how a Gazelle site differs from the others, as Prowlarr's
// subclasses of GazelleBase do.
type site struct {
	links, legacy []string
	// cookie is whether the site takes a cookie setting.
	cookie bool
	info   indexer.Info
	caps   func() *indexer.Capabilities
	// imdbInTags searches IMDb IDs as tags instead of catalogue numbers.
	imdbInTags bool
	// params adds the site's own parameters to searches.
	params func(s *Site, p *indexer.Params, q *indexer.Query)
	// movieParams replaces the parameters of movie searches.
	movieParams func(s *Site, q *indexer.Query) *indexer.Params
	// check replaces failing responses that are not OK or not JSON.
	check func(s *Site, ix *indexer.Indexer, r *indexer.Response) error
	// parse replaces reading the results of a successful response.
	parse func(s *Site, r *indexer.Response) ([]*indexer.Release, error)
	// cleanup cleans up the releases of searches further.
	cleanup func(releases []*indexer.Release, q *indexer.Query) []*indexer.Release
}

// Site is a Gazelle site, as Prowlarr's GazelleBase is.
type Site struct {
	site
	settings Settings
	baseURL  string
	caps     *indexer.Capabilities
}

func newSite(d site, s Settings) (*Site, error) {
	if err := s.validate(d.cookie); err != nil {
		return nil, err
	}
	if !d.cookie {
		s.Cookie = ""
	}
	return &Site{site: d, settings: s, baseURL: s.URL(d.links, d.legacy), caps: d.caps()}, nil
}

// definition returns the definition of a site.
func definition(d indexer.Definition, s site, extra ...indexer.Field) indexer.Definition {
	d.Protocol = indexer.ProtocolTorrent
	d.Privacy = indexer.Private
	d.Links, d.LegacyLinks = s.links, s.legacy
	d.Fields = fields(s.links, s.cookie, extra...)
	d.New = func(data []byte) (indexer.Site, error) {
		var settings Settings
		if err := indexer.DecodeSettings(data, &settings); err != nil {
			return nil, err
		}
		return newSite(s, settings)
	}
	return d
}

// baseInfo is what Gazelle sites are, unless they say otherwise.
var baseInfo = indexer.Info{
	Protocol:       indexer.ProtocolTorrent,
	Privacy:        indexer.Private,
	SupportsRSS:    true,
	SupportsSearch: true,
	PageSize:       50,
}

// Info implements indexer.Site.
func (s *Site) Info() indexer.Info { return s.info }

// Capabilities implements indexer.Site.
func (s *Site) Capabilities(context.Context, *indexer.Indexer) (*indexer.Capabilities, error) {
	return s.caps, nil
}

// SearchRequests implements indexer.Site, as Prowlarr's
// GazelleRequestGenerator does: one browse request of the API.
func (s *Site) SearchRequests(_ context.Context, _ *indexer.Indexer, q *indexer.Query) (*indexer.Chain, error) {
	var p *indexer.Params
	switch q.Type {
	case indexer.SearchMovie:
		if s.movieParams != nil {
			p = s.movieParams(s, q)
			break
		}
		p = s.BasicParams(q, q.SanitizedTerm())
		s.imdbParam(p, q)
	case indexer.SearchTV:
		p = s.BasicParams(q, q.SanitizedTVString())
		s.imdbParam(p, q)
	case indexer.SearchMusic:
		p = s.BasicParams(q, q.SanitizedTerm())
		if strings.TrimSpace(q.Artist) != "" && q.Artist != "VA" {
			p.Set("artistname", q.Artist)
		}
		if strings.TrimSpace(q.Album) != "" {
			p.Set("groupname", q.Album)
		}
		if strings.TrimSpace(q.Label) != "" {
			p.Set("recordlabel", q.Label)
		}
	default:
		p = s.BasicParams(q, q.SanitizedTerm())
	}
	r := indexer.NewRequest(s.baseURL+"ajax.php?"+p.Encode(), indexer.AcceptJSON)
	r.Cookies = s.cookies()
	var c indexer.Chain
	c.AddRequests(r)
	return &c, nil
}

func (s *Site) imdbParam(p *indexer.Params, q *indexer.Query) {
	if q.IMDbID == "" {
		return
	}
	key := "cataloguenumber"
	if s.imdbInTags {
		key = "taglist"
	}
	p.Set(key, q.FullIMDbID())
}

// BasicParams returns the parameters every search has: the term, with
// dots as spaces, the categories and the site's own.
func (s *Site) BasicParams(q *indexer.Query, term string) *indexer.Params {
	var p indexer.Params
	p.Set("action", "browse")
	p.Set("order_by", "time")
	p.Set("order_way", "desc")
	if strings.TrimSpace(term) != "" {
		p.Set("searchstr", strings.ReplaceAll(term, ".", " "))
	}
	for _, c := range s.caps.Categories.MapToTracker(q.Categories, false) {
		p.Set("filter_cat["+c+"]", "1")
	}
	if s.params != nil {
		s.params(s, &p, q)
	}
	return &p
}

// cookies returns the cookie setting's cookies; nil when unset.
func (s *Site) cookies() map[string]string {
	if strings.TrimSpace(s.settings.Cookie) == "" {
		return nil
	}
	return indexer.ParseCookieHeader(s.settings.Cookie)
}

// LoginNeeded implements indexer.LoginSite: never with a cookie setting,
// else when the site redirects or says the credentials are bad.
func (s *Site) LoginNeeded(r *indexer.Response) bool {
	if s.cookies() != nil {
		return false
	}
	c := r.Content()
	return r.IsRedirect() || strings.Contains(c, `"bad credentials"`) || strings.Contains(c, `"groupName":"wrong-creds"`)
}

// Login implements indexer.LoginSite: it posts the user name and password
// to login.php and keeps the cookies set, for 30 days.
func (s *Site) Login(ctx context.Context, ix *indexer.Indexer) error {
	loginURL := s.baseURL + "login.php"
	r := indexer.NewRequest(loginURL, indexer.AcceptJSON)
	r.Header.Set("Referer", loginURL)
	var form indexer.Params
	form.Set("username", s.settings.Username)
	form.Set("password", s.settings.Password)
	form.Set("keeplogged", "1")
	r.Method = http.MethodPost
	r.Body = []byte(form.Encode())
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := ix.Do(ctx, r)
	if err != nil {
		return fmt.Errorf("gazelle login: %w", err)
	}
	if resp.HasError() {
		return fmt.Errorf("gazelle login: %w", &indexer.HTTPError{Response: resp})
	}
	ix.SetCookies(resp.Cookies, time.Time{})
	return nil
}

// ParseResponse implements indexer.Site, as Prowlarr's GazelleParser
// does.
func (s *Site) ParseResponse(_ context.Context, ix *indexer.Indexer, r *indexer.Response) ([]*indexer.Release, error) {
	check := checkResponse
	if s.check != nil {
		check = s.check
	}
	if err := check(s, ix, r); err != nil {
		return nil, err
	}
	if s.parse != nil {
		return s.parse(s, r)
	}
	var resp response[group]
	if err := json.Unmarshal([]byte(r.Content()), &resp); err != nil {
		return nil, fmt.Errorf("gazelle: %w", err)
	}
	if resp.Status != "success" || resp.Response == nil {
		return nil, nil
	}
	var releases []*indexer.Release
	for _, g := range resp.Response.Results {
		poster := s.posterURL(string(g.Cover))
		if g.Torrents == nil {
			rel, err := s.groupRelease(&g, poster)
			if err != nil {
				return nil, err
			}
			if rel != nil {
				releases = append(releases, rel)
			}
			continue
		}
		for _, t := range g.Torrents {
			freeleech := bool(t.IsFreeLeech || t.IsNeutralLeech || t.IsPersonalFreeLeech)
			if s.settings.UseFreeleechToken == TokenRequired && !bool(t.CanUseToken) && !freeleech {
				continue
			}
			title := fmt.Sprintf("%s - %s (%s) [%s %s] [%s]", g.Artist, g.GroupName, g.GroupYear, t.Format, t.Encoding, t.Media)
			if t.HasCue {
				title += " [Cue]"
			}
			rel, err := s.release(&g, &t.torrentStats, freeleech, html.UnescapeString(title), t.Time.Time, t.Category)
			if err != nil {
				return nil, err
			}
			rel.Scene = bool(t.Scene)
			rel.PosterURL = poster
			releases = append(releases, rel)
		}
	}
	SortByDate(releases)
	return releases, nil
}

// checkResponse fails responses that are not OK or not JSON, forgetting
// the indexer's cookies.
func checkResponse(_ *Site, ix *indexer.Indexer, r *indexer.Response) error {
	if r.StatusCode != http.StatusOK {
		ix.SetCookies(nil, time.Time{})
		return statusError(r)
	}
	if !r.HasContentType(indexer.AcceptJSON) {
		ix.SetCookies(nil, time.Time{})
		return contentTypeError(r)
	}
	return nil
}

// statusError is the error of a response that is not OK, with the
// site's error message.
func statusError(r *indexer.Response) error {
	var e errorResponse
	msg := "Check the logs for more information."
	if json.Unmarshal([]byte(r.Content()), &e) == nil && e.Error != "" {
		msg = e.Error
	}
	return indexer.Errorf("Unexpected response status %d code from indexer request: %s", r.StatusCode, msg)
}

func contentTypeError(r *indexer.Response) error {
	return indexer.Errorf("Unexpected response header %s from indexer request, expected %s", r.Header.Get("Content-Type"), indexer.AcceptJSON)
}

// groupRelease returns the release of a group listed as a torrent.
func (s *Site) groupRelease(g *group, poster string) (*indexer.Release, error) {
	freeleech := bool(g.IsFreeLeech || g.IsNeutralLeech || g.IsPersonalFreeLeech)
	if s.settings.UseFreeleechToken == TokenRequired && !bool(g.CanUseToken) && !freeleech {
		return nil, nil
	}
	var date time.Time
	if n, err := strconv.ParseInt(string(g.GroupTime), 10, 64); err == nil {
		date = time.Unix(n, 0).UTC()
	} else {
		d, err := parse.Fuzzy(time.Now(), string(g.GroupTime), "")
		if err != nil {
			return nil, err
		}
		date = d.UTC()
	}
	rel, err := s.release(g, &g.torrentStats, freeleech, html.UnescapeString(string(g.GroupName)), date, g.Category)
	if err != nil {
		return nil, err
	}
	rel.PosterURL = poster
	return rel, nil
}

// release returns the release of a torrent.
func (s *Site) release(g *group, t *torrentStats, freeleech bool, title string, date time.Time, category *jsonx.String) (*indexer.Release, error) {
	size, err := strconv.ParseInt(string(t.Size), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("gazelle: size %q: %w", t.Size, err)
	}
	seeders, err := strconv.Atoi(string(t.Seeders))
	if err != nil {
		return nil, fmt.Errorf("gazelle: seeders %q: %w", t.Seeders, err)
	}
	leechers, err := strconv.Atoi(string(t.Leechers))
	if err != nil {
		return nil, fmt.Errorf("gazelle: leechers %q: %w", t.Leechers, err)
	}
	info := s.InfoURL(string(g.GroupID), int(t.TorrentID))
	rel := &indexer.Release{
		GUID:                 info,
		InfoURL:              info,
		DownloadURL:          s.DownloadURL(int(t.TorrentID), bool(t.CanUseToken) && !freeleech),
		Title:                title,
		Protocol:             indexer.ProtocolTorrent,
		Size:                 &size,
		Files:                indexer.Ptr(int(t.FileCount)),
		Seeders:              &seeders,
		Peers:                indexer.Ptr(seeders + leechers),
		PublishDate:          date,
		Categories:           s.categories(category),
		DownloadVolumeFactor: indexer.Ptr(volumeFactor(freeleech)),
		UploadVolumeFactor:   indexer.Ptr(volumeFactor(bool(t.IsNeutralLeech))),
	}
	if t.Snatches != nil {
		rel.Grabs = indexer.Ptr(int(*t.Snatches))
	}
	return rel, nil
}

// categories maps a torrent's category, by description; the site's
// category "1" when it has none.
func (s *Site) categories(category *jsonx.String) []int {
	if category == nil || strings.Contains(string(*category), "Select Category") {
		return indexer.IDs(s.caps.Categories.MapTracker("1"))
	}
	return indexer.IDs(s.caps.Categories.MapTrackerDesc(string(*category)))
}

func volumeFactor(free bool) float64 {
	if free {
		return 0
	}
	return 1
}

// DownloadURL returns the link downloading a torrent, using a token when
// the settings say so and one can be used.
func (s *Site) DownloadURL(id int, canUseToken bool) string {
	u := indexer.AddQueryParam(indexer.AddQueryParam(indexer.CombinePath(s.baseURL, "/torrents.php"), "action", "download"), "id", strconv.Itoa(id))
	if s.settings.UseFreeleechToken != TokenNever && canUseToken {
		u = indexer.AddQueryParam(u, "usetoken", "1")
	}
	return u
}

// InfoURL returns the page of a torrent.
func (s *Site) InfoURL(groupID string, id int) string {
	return indexer.AddQueryParam(indexer.AddQueryParam(indexer.CombinePath(s.baseURL, "/torrents.php"), "id", groupID), "torrentid", strconv.Itoa(id))
}

func (s *Site) posterURL(cover string) string {
	if cover == "" {
		return ""
	}
	if strings.HasPrefix(cover, "http") {
		return cover
	}
	return indexer.CombinePath(s.baseURL, cover)
}

// SortByDate sorts releases newest first.
func SortByDate(releases []*indexer.Release) {
	slices.SortStableFunc(releases, func(a, b *indexer.Release) int { return b.PublishDate.Compare(a.PublishDate) })
}

// CleanupReleases implements indexer.ReleaseCleaner.
func (s *Site) CleanupReleases(releases []*indexer.Release, q *indexer.Query) []*indexer.Release {
	if s.cleanup == nil {
		return releases
	}
	return s.cleanup(releases, q)
}

// tokenErrors are what sites answer downloads using freeleech tokens
// that cannot be used with.
var tokenErrors = []string{
	"You do not have any freeleech tokens left.",
	"You do not have enough freeleech tokens",
	"This torrent is too large.",
	"You cannot use tokens here",
}

// RetryDownload implements indexer.DownloadRetrier: when a token is
// preferred but cannot be used, the torrent is downloaded without.
func (s *Site) RetryDownload(link string, data []byte) (string, bool) {
	if s.settings.UseFreeleechToken != TokenPreferred || len(data) == 0 || data[0] == 'd' || !strings.Contains(link, "usetoken=1") {
		return "", false
	}
	page := string(data)
	if !slices.ContainsFunc(tokenErrors, func(e string) bool { return strings.Contains(page, e) }) {
		return "", false
	}
	return indexer.RemoveQueryParam(link, "usetoken"), true
}
