package indexer

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"strings"
)

// Download downloads a release's torrent or NZB; for magnet links, the
// link itself. Redirects to magnet links are followed.
func (ix *Indexer) Download(ctx context.Context, link string) ([]byte, error) {
	if err := ix.checkLimit(ix.Limits.Grabs, ix.Statuses.Grabs, "grab"); err != nil {
		return nil, err
	}
	data, err := ix.download(ctx, link)
	var he *HTTPError
	if dl, ok := ix.Site.(DownloadRelogger); ok && errors.As(err, &he) && he.Response.StatusCode == http.StatusUnauthorized {
		if err := dl.Login(ctx, ix); err != nil {
			return nil, err
		}
		data, err = ix.download(ctx, link)
	}
	if err != nil {
		return nil, err
	}
	if dr, ok := ix.Site.(DownloadRetrier); ok {
		if retry, ok := dr.RetryDownload(link, data); ok {
			if data, err = ix.download(ctx, retry); err != nil {
				return nil, err
			}
		}
	}
	ix.Statuses.RecordGrab(ix.ID)
	return data, nil
}

func (ix *Indexer) download(ctx context.Context, link string) ([]byte, error) {
	var r *Request
	if ds, ok := ix.Site.(DownloadSite); ok {
		var err error
		if r, err = ds.DownloadRequest(ctx, ix, link); err != nil {
			return nil, err
		}
	} else {
		r = NewRequest(link, "")
	}
	if strings.HasPrefix(r.URL, "magnet:") {
		if err := ValidateMagnet(r.URL); err != nil {
			return nil, err
		}
		return []byte(r.URL), nil
	}
	req := *r
	req.FollowRedirects = false
	req.Cookies = maps.Clone(r.Cookies)
	if req.Cookies == nil {
		req.Cookies = ix.Cookies()
	}
	s := ix.session()
	resp, err := s.Do(ctx, &req)
	for redirects := 0; err == nil && resp.IsRedirect(); redirects++ {
		next := resp.RedirectURL()
		if next == "" {
			return nil, fmt.Errorf("download %s: redirected without a location", link)
		}
		if strings.HasPrefix(next, "magnet:") {
			return ix.download(ctx, next)
		}
		if redirects == maxRedirects-1 {
			return nil, fmt.Errorf("download %s: redirected more than %d times", link, maxRedirects)
		}
		req.URL = next
		resp, err = s.Do(ctx, &req)
	}
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", link, err)
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("download %s: %w", link, ErrReleaseUnavailable)
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("download %s: %w", link, &RateLimitError{RetryAfter: retryAfter(resp, ix.now())})
	}
	if resp.HasError() {
		return nil, fmt.Errorf("download %s: %w", link, &HTTPError{Response: resp})
	}
	if resp.HasContentType("text/html") {
		return nil, fmt.Errorf("download %s: the site answered with a page", link)
	}
	if ix.Site.Info().Protocol == ProtocolUsenet {
		if err := ValidateNZB(resp.Body); err != nil {
			return nil, fmt.Errorf("download %s: %w", link, err)
		}
	}
	if v, ok := ix.Site.(DownloadValidator); ok {
		if err := v.ValidateDownload(resp.Body); err != nil {
			return nil, fmt.Errorf("download %s: %w", link, err)
		}
	}
	return resp.Body, nil
}

// ValidateMagnet checks that a magnet link names a torrent by its hash.
func ValidateMagnet(link string) error {
	u, err := url.Parse(link)
	if err != nil || u.Scheme != "magnet" {
		return fmt.Errorf("invalid magnet link %q", link)
	}
	for _, xt := range u.Query()["xt"] {
		if strings.HasPrefix(xt, "urn:btih:") || strings.HasPrefix(xt, "urn:btmh:") {
			return nil
		}
	}
	return fmt.Errorf("magnet link %q has no info hash", link)
}

// Test checks that the indexer answers: it fetches the first page of a
// search for recent releases, or for "test" when the site lists none.
func (ix *Indexer) Test(ctx context.Context) error {
	if tp, ok := ix.Site.(TestPreparer); ok {
		if err := tp.PrepareTest(ctx, ix); err != nil {
			return err
		}
	}
	q := &Query{Type: SearchBasic}
	if !ix.Site.Info().SupportsRSS {
		q.Term = "test"
	}
	chain, err := ix.Site.SearchRequests(ctx, ix, q)
	if err != nil {
		return err
	}
	first := chain.First()
	if first == nil {
		return errors.New("no feed to query; check the indexer's categories")
	}
	releases, err := ix.fetchPage(ctx, first)
	if err != nil {
		var he *HTTPError
		if errors.As(err, &he) && he.ServerError() {
			return fmt.Errorf("the indexer's server is unavailable, try again later: %w", err)
		}
		return err
	}
	if len(releases) == 0 {
		return errors.New("query successful, but the indexer returned no results; check its categories and settings")
	}
	if st, ok := ix.Site.(SiteTester); ok {
		return st.TestSite(ctx, ix)
	}
	return nil
}
