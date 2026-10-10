package indexer

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"math/rand/v2"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"

	"golang.org/x/text/encoding"
)

// Info is what an indexer site is, besides its capabilities.
type Info struct {
	Protocol Protocol
	Privacy  Privacy
	// SupportsRSS is whether the site lists recent releases for queries
	// without terms.
	SupportsRSS bool
	// SupportsSearch is whether the site takes search terms.
	SupportsSearch bool
	// SupportsPagination is whether the site takes offsets.
	SupportsPagination bool
	// PageSize is the number of releases of a full page, after which the
	// next page is fetched; 0 fetches only the first, 1 takes the size of
	// the first page.
	PageSize int
	// RateLimit is the minimum time between requests; 2 seconds when 0.
	RateLimit time.Duration
	// Encoding decodes the site's pages; UTF-8 or the response's charset
	// when nil.
	Encoding encoding.Encoding
}

// Site is an indexer site: how it is searched and its responses read, as
// Prowlarr's request generators and parsers are.
type Site interface {
	Info() Info
	// Capabilities returns what the site can be searched by; sites that
	// ask the indexer for them do so on first use.
	Capabilities(ctx context.Context, ix *Indexer) (*Capabilities, error)
	SearchRequests(ctx context.Context, ix *Indexer, q *Query) (*Chain, error)
	ParseResponse(ctx context.Context, ix *Indexer, r *Response) ([]*Release, error)
}

// LoginSite is a site that needs logging in.
type LoginSite interface {
	Site
	// LoginNeeded reports whether a response asks to log in; Indexer
	// takes HTTP 401 as asking when the site does not implement it.
	LoginNeeded(r *Response) bool
	// Login logs in, keeping the cookies with SetCookies.
	Login(ctx context.Context, ix *Indexer) error
}

// DownloadSite is a site whose releases are downloaded by other requests
// than GETs of their download URLs.
type DownloadSite interface {
	Site
	DownloadRequest(ctx context.Context, ix *Indexer, link string) (*Request, error)
}

// SiteTester is a site with more to check when the indexer is tested.
type SiteTester interface {
	Site
	// TestSite checks the site after its first page was fetched.
	TestSite(ctx context.Context, ix *Indexer) error
}

// ReleaseCleaner is a site that cleans up its releases further after a
// search.
type ReleaseCleaner interface {
	Site
	CleanupReleases(releases []*Release, q *Query) []*Release
}

// DownloadValidator checks a site's downloads.
type DownloadValidator interface {
	ValidateDownload(data []byte) error
}

// Limits are the most queries and grabs an indexer is used for, per hour
// or per day; none when 0.
type Limits struct {
	Queries, Grabs int
	// Per is time.Hour or 24 hours; a day when 0.
	Per time.Duration
}

// cookieLife is how long the cookies of successful searches are kept.
const cookieLife = 30 * 24 * time.Hour

// Indexer is a configured indexer: a site with an ID, its session and
// status, searched as Prowlarr's HttpIndexerBase searches.
type Indexer struct {
	ID       string
	Name     string
	Priority int
	Site     Site
	Session  *Session
	Statuses *Statuses
	Limits   Limits
	// Now is the clock; time.Now when nil.
	Now func() time.Time
}

func (ix *Indexer) now() time.Time {
	if ix.Now != nil {
		return ix.Now()
	}
	return time.Now()
}

func (ix *Indexer) rateLimit() time.Duration {
	if r := ix.Site.Info().RateLimit; r > 0 {
		return r
	}
	return 2 * time.Second
}

// Cookies returns the indexer's cookies.
func (ix *Indexer) Cookies() map[string]string {
	return ix.Statuses.Cookies(ix.ID)
}

// SetCookies keeps the indexer's cookies until expire; for 30 days when
// expire is zero.
func (ix *Indexer) SetCookies(cookies map[string]string, expire time.Time) {
	if expire.IsZero() {
		expire = ix.now().Add(cookieLife)
	}
	ix.Statuses.SetCookies(ix.ID, cookies, expire)
}

// Do sends a request of the site's, such as logging in, as is: with the
// site's rate limit and encoding, a 15 second timeout unless the request
// sets one, and no cookies but the request's.
func (ix *Indexer) Do(ctx context.Context, r *Request) (*Response, error) {
	req := *r
	if req.Timeout == 0 {
		req.Timeout = 15 * time.Second
	}
	return ix.session().Do(ctx, &req)
}

func (ix *Indexer) session() *Session {
	s := *ix.Session
	info := ix.Site.Info()
	s.RateLimit = max(s.RateLimit, ix.rateLimit())
	if info.Encoding != nil {
		s.Encoding = info.Encoding
	}
	return &s
}

// Search searches the indexer: the requests of each tier of the query's
// search until a tier finds releases, each search page by page until a
// page is not full. Failures leave the indexer out for a while.
func (ix *Indexer) Search(ctx context.Context, q *Query) ([]*Release, error) {
	if until, failure, ok := ix.Statuses.Disabled(ix.ID); ok {
		return nil, fmt.Errorf("%w until %s: %s", ErrDisabled, until.Format(time.RFC3339), failure)
	}
	if err := ix.checkLimit(ix.Limits.Queries, ix.Statuses.Queries, "query"); err != nil {
		return nil, err
	}
	info := ix.Site.Info()
	if (!info.SupportsSearch && !info.SupportsRSS) || (!info.SupportsPagination && q.Offset > 0) {
		return nil, nil
	}
	caps, err := ix.Site.Capabilities(ctx, ix)
	if err != nil {
		return nil, ix.fail(ctx, err)
	}
	if !supports(caps, q) {
		return nil, nil
	}
	ix.Statuses.RecordQuery(ix.ID)
	chain, err := ix.Site.SearchRequests(ctx, ix, q)
	if err != nil {
		return nil, ix.fail(ctx, err)
	}
	var releases []*Release
	for _, tier := range chain.Tiers() {
		for _, pages := range tier {
			pageSize := info.PageSize
			for r := range pages {
				page, err := ix.fetchPage(ctx, r)
				if err != nil {
					return nil, ix.fail(ctx, err)
				}
				if pageSize == 1 {
					pageSize = len(page)
				}
				for _, rel := range page {
					if validRelease(rel, q.Interactive) {
						releases = append(releases, rel)
					}
				}
				if pageSize == 0 || len(page) < pageSize {
					break
				}
			}
		}
		if len(releases) > 0 {
			break
		}
	}
	ix.Statuses.RecordSuccess(ix.ID)
	releases = ix.cleanup(releases)
	if rc, ok := ix.Site.(ReleaseCleaner); ok {
		releases = rc.CleanupReleases(releases, q)
	}
	return releases, nil
}

func (ix *Indexer) checkLimit(limit int, count func(string, time.Duration) (int, time.Time), what string) error {
	if limit <= 0 {
		return nil
	}
	per := ix.Limits.Per
	if per <= 0 {
		per = 24 * time.Hour
	}
	if n, free := count(ix.ID, per); n >= limit {
		return fmt.Errorf("%w: %d of %d %s limit until %s", ErrLimitReached, n, limit, what, free.Format(time.RFC3339))
	}
	return nil
}

// supports reports whether the indexer takes all the parameters the query
// sets, as Prowlarr's Fetch overloads check.
func supports(c *Capabilities, q *Query) bool {
	if q.Type == SearchBasic {
		return true
	}
	set := map[Param]bool{
		ParamIMDbID:    q.IMDbID != "",
		ParamTMDBID:    q.TMDBID != 0,
		ParamTVDBID:    q.TVDBID != 0,
		ParamRID:       q.RID != 0,
		ParamTVMazeID:  q.TVMazeID != 0,
		ParamTraktID:   q.TraktID != 0,
		ParamDoubanID:  q.DoubanID != 0,
		ParamGenre:     q.Genre != "",
		ParamYear:      q.Year != 0,
		ParamArtist:    q.Artist != "",
		ParamAlbum:     q.Album != "",
		ParamLabel:     q.Label != "",
		ParamTrack:     q.Track != "",
		ParamAuthor:    q.Author != "",
		ParamTitle:     q.Title != "",
		ParamPublisher: q.Publisher != "",
	}
	for _, p := range searchTypeParams[q.Type] {
		if set[p] && !c.Has(q.Type, p) {
			return false
		}
	}
	return true
}

// fetchPage fetches a page of a search and parses it.
func (ix *Indexer) fetchPage(ctx context.Context, r *Request) ([]*Release, error) {
	resp, err := ix.fetch(ctx, r)
	if err != nil {
		return nil, err
	}
	return ix.Site.ParseResponse(ctx, ix, resp)
}

// fetch sends a search's request with the indexer's cookies, retrying
// server failures twice and logging in when the response asks to.
func (ix *Indexer) fetch(ctx context.Context, r *Request) (*Response, error) {
	s := ix.session()
	req := *r
	req.Cookies = maps.Clone(r.Cookies)
	if req.Cookies == nil {
		req.Cookies = map[string]string{}
	}
	maps.Copy(req.Cookies, ix.Cookies())

	resp, err := ix.retry(ctx, s, &req)
	if err != nil {
		return nil, err
	}
	if ls, ok := ix.Site.(LoginSite); ok && ls.LoginNeeded(resp) || !ok && resp.StatusCode == http.StatusUnauthorized {
		if ok {
			if err := ls.Login(ctx, ix); err != nil {
				return nil, err
			}
		}
		req.Cookies = maps.Clone(r.Cookies)
		if req.Cookies == nil {
			req.Cookies = map[string]string{}
		}
		maps.Copy(req.Cookies, ix.Cookies())
		if resp, err = s.Do(ctx, &req); err != nil {
			return nil, err
		}
	}
	if IsCloudflareProtected(resp) {
		return nil, fmt.Errorf("%s: %w", req.URL, ErrCloudflare)
	}
	if resp.HasError() {
		if resp.StatusCode == http.StatusTooManyRequests {
			return nil, &RateLimitError{RetryAfter: retryAfter(resp, ix.now())}
		}
		if resp.StatusCode >= 500 {
			return nil, &HTTPError{Response: resp}
		}
	}
	maps.Copy(req.Cookies, resp.Cookies)
	ix.SetCookies(req.Cookies, time.Time{})
	return resp, nil
}

// retry sends a request, sending it again twice on server failures and
// timeouts, after exponentially longer delays from the rate limit.
func (ix *Indexer) retry(ctx context.Context, s *Session, r *Request) (*Response, error) {
	delay := s.RateLimit
	for attempt := 0; ; attempt++ {
		resp, err := s.Do(ctx, r)
		if err != nil || attempt == 2 || (resp.StatusCode < 500 && resp.StatusCode != http.StatusRequestTimeout) {
			return resp, err
		}
		// Jitter as Polly's, between half and one and a half the delay.
		if err := sleep(ctx, time.Duration(float64(delay)*(0.5+rand.Float64()))); err != nil {
			return nil, err
		}
		delay *= 2
	}
}

// retryAfter returns how long a 429 response asks to wait, 0 when it does
// not tell.
func retryAfter(r *Response, now time.Time) time.Duration {
	v := strings.TrimSpace(r.Header.Get("Retry-After"))
	if v == "" {
		return 0
	}
	var secs int
	if _, err := fmt.Sscanf(v, "%d", &secs); err == nil && fmt.Sprint(secs) == v {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil && t.After(now) {
		return t.Sub(now)
	}
	return 0
}

// minimumBackoff is how long rate limited indexers are left out for when
// they do not tell.
const minimumBackoff = time.Hour

// fail records a failure of the indexer, by kind, and returns it.
func (ix *Indexer) fail(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return err
	}
	var rl *RateLimitError
	var dns *net.DNSError
	var op *net.OpError
	switch {
	case errors.As(err, &rl):
		backoff := rl.RetryAfter
		if backoff <= 0 {
			backoff = minimumBackoff
		}
		ix.Statuses.RecordFailure(ix.ID, backoff, err.Error())
	case errors.As(err, &dns), errors.As(err, &op) && op.Op == "dial":
		ix.Statuses.RecordConnectionFailure(ix.ID, err.Error())
	default:
		ix.Statuses.RecordFailure(ix.ID, 0, err.Error())
	}
	return fmt.Errorf("indexer %s: %w", ix.Name, err)
}

// validRelease reports whether a release is complete enough to list: it
// has a title and, unless someone is choosing, a size and categories.
func validRelease(r *Release, interactive bool) bool {
	if strings.TrimSpace(r.Title) == "" {
		return false
	}
	return interactive || (r.Size != nil && len(r.Categories) > 0)
}

// cleanup fills in releases' GUIDs and flags, and drops repeated GUIDs, as
// Prowlarr's CleanupReleases does.
func (ix *Indexer) cleanup(releases []*Release) []*Release {
	protocol := ix.Site.Info().Protocol
	seen := map[string]bool{}
	out := releases[:0]
	for _, r := range releases {
		if strings.TrimSpace(r.GUID) == "" {
			switch {
			case strings.TrimSpace(r.DownloadURL) != "":
				r.GUID = r.DownloadURL
			case protocol == ProtocolTorrent && strings.TrimSpace(r.MagnetURL) != "":
				r.GUID = r.MagnetURL
			case strings.TrimSpace(r.InfoURL) != "":
				r.GUID = r.InfoURL
			}
		}
		r.Protocol = protocol
		if protocol == ProtocolTorrent {
			addFlags(r)
		}
		if seen[r.GUID] {
			continue
		}
		seen[r.GUID] = true
		out = append(out, r)
	}
	return out
}

func addFlags(r *Release) {
	add := func(f Flag) {
		if !r.HasFlag(f) {
			r.Flags = append(r.Flags, f)
		}
	}
	if f := r.DownloadVolumeFactor; f != nil {
		switch *f {
		case 0:
			add(FlagFreeleech)
		case 0.5:
			add(FlagHalfLeech)
		}
	}
	if f := r.UploadVolumeFactor; f != nil {
		switch *f {
		case 0:
			add(FlagNeutralLeech)
		case 2:
			add(FlagDoubleUpload)
		}
	}
	if r.Scene {
		add(FlagScene)
	}
}

var (
	nonWord     = regexp.MustCompile(`[^\p{L}\p{N}_]+`)
	commonWords = []string{"and", "the", "an", "of"}
)

// FilterByQuery keeps the releases whose title or description has the
// query's terms: one of a single term, or two of more, as Prowlarr's
// FilterReleasesByQuery does for sites that search loosely.
func FilterByQuery(releases []*Release, q *Query) []*Release {
	if q.IsRSS() || q.IsIDSearch() {
		return releases
	}
	var terms []string
	for _, t := range nonWord.Split(q.Term, -1) {
		if len([]rune(t)) > 1 && !containsFold(commonWords, t) {
			terms = append(terms, strings.ToLower(t))
		}
	}
	var out []*Release
	for _, r := range releases {
		title, desc := strings.ToLower(r.Title), strings.ToLower(r.Description)
		n := 0
		for _, t := range terms {
			if strings.Contains(title, t) || strings.Contains(desc, t) {
				n++
			}
		}
		if (len(terms) > 1 && n > 1) || (len(terms) <= 1 && n > 0) {
			out = append(out, r)
		}
	}
	return out
}

func containsFold(list []string, s string) bool {
	for _, v := range list {
		if strings.EqualFold(v, s) {
			return true
		}
	}
	return false
}
