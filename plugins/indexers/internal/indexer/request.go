package indexer

import (
	"bytes"
	"iter"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"golang.org/x/text/encoding"
	"golang.org/x/text/transform"
)

// The Accept headers of the kinds of responses indexers give.
const (
	AcceptRSS  = "application/rss+xml, text/rss+xml, application/xml, text/xml"
	AcceptJSON = "application/json"
	AcceptHTML = "text/html"
)

// Request is a request to an indexer.
type Request struct {
	// GET when empty.
	Method string
	URL    string
	Header http.Header
	Body   []byte
	// Cookies sent besides the indexer's own.
	Cookies map[string]string
	// FollowRedirects follows redirects instead of returning them.
	FollowRedirects bool
	// AllowedStatus are error statuses the caller handles itself.
	AllowedStatus []int
	// RateLimit raises the indexer's minimum time between requests.
	RateLimit time.Duration
	// Timeout replaces the default of 100 seconds.
	Timeout time.Duration
	// Encoding decodes the response's text, replacing the indexer's.
	Encoding encoding.Encoding
	// State is the site's own data about the request, for reading its
	// response.
	State any
}

// NewRequest returns a GET request accepting the content type.
func NewRequest(rawURL, accept string) *Request {
	r := &Request{URL: rawURL, Header: http.Header{}}
	if accept != "" {
		r.Header.Set("Accept", accept)
	}
	return r
}

// SetForm makes the request a POST of a form.
func (r *Request) SetForm(form url.Values) {
	r.Method = http.MethodPost
	r.Body = []byte(form.Encode())
	if r.Header == nil {
		r.Header = http.Header{}
	}
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
}

// Response is an indexer's response.
type Response struct {
	Request *Request
	// The URL that answered, after redirects.
	URL        string
	StatusCode int
	Header     http.Header
	Body       []byte
	// Cookies are those the responses set, through redirects.
	Cookies map[string]string

	encoding encoding.Encoding
	content  *string
}

// Content returns the response's body as text, decoded from the
// request's or the indexer's encoding.
func (r *Response) Content() string {
	if r.content == nil {
		s := string(r.Body)
		if r.encoding != nil {
			if b, _, err := transform.Bytes(r.encoding.NewDecoder(), r.Body); err == nil {
				s = string(b)
			}
		}
		s = strings.TrimPrefix(s, "\uFEFF")
		r.content = &s
	}
	return *r.content
}

// SetContent replaces the response's text, as solvers of challenges do.
func (r *Response) SetContent(s string) {
	r.Body = []byte(s)
	r.content = &s
}

// IsRedirect reports whether the response redirects.
func (r *Response) IsRedirect() bool {
	switch r.StatusCode {
	case http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther, http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
		return true
	}
	return false
}

// RedirectURL returns where the response redirects, resolved against its
// URL; "" for none.
func (r *Response) RedirectURL() string {
	loc := r.Header.Get("Location")
	if loc == "" {
		return ""
	}
	return ResolveURL(r.URL, loc)
}

// HasError reports whether the status is an error the request does not
// handle itself.
func (r *Response) HasError() bool {
	if r.StatusCode < 400 {
		return false
	}
	for _, s := range r.Request.AllowedStatus {
		if s == r.StatusCode {
			return false
		}
	}
	return true
}

// HasContentType reports whether the response's content type contains
// the given one.
func (r *Response) HasContentType(t string) bool {
	return strings.Contains(strings.ToLower(r.Header.Get("Content-Type")), t)
}

// ResolveURL resolves a reference against a base URL; the reference as
// given when either does not parse.
func ResolveURL(base, ref string) string {
	ref = strings.TrimSpace(ref)
	b, err := url.Parse(base)
	if err != nil {
		return ref
	}
	r, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	return b.ResolveReference(r).String()
}

// Chain is the requests of a search: tiers of searches, each tried when
// the tiers before found nothing, each search a sequence of pages, fetched
// until one is not full.
type Chain struct {
	tiers [][]iter.Seq[*Request]
}

// Add adds a search to the last tier.
func (c *Chain) Add(pages iter.Seq[*Request]) {
	if pages == nil {
		return
	}
	if len(c.tiers) == 0 {
		c.tiers = [][]iter.Seq[*Request]{nil}
	}
	c.tiers[len(c.tiers)-1] = append(c.tiers[len(c.tiers)-1], pages)
}

// AddRequests adds a search of the requests given, one per page.
func (c *Chain) AddRequests(rs ...*Request) {
	c.Add(func(yield func(*Request) bool) {
		for _, r := range rs {
			if !yield(r) {
				return
			}
		}
	})
}

// AddTier starts a new tier, unless the last is empty.
func (c *Chain) AddTier() {
	if len(c.tiers) > 0 && len(c.tiers[len(c.tiers)-1]) == 0 {
		return
	}
	c.tiers = append(c.tiers, nil)
}

// Tiers returns the tiers.
func (c *Chain) Tiers() [][]iter.Seq[*Request] {
	return c.tiers
}

// First returns the first request of the first search, if any.
func (c *Chain) First() *Request {
	for _, tier := range c.tiers {
		for _, pages := range tier {
			for r := range pages {
				return r
			}
		}
	}
	return nil
}

// bodyReader returns a reader of the request's body, nil for none.
func (r *Request) bodyReader() *bytes.Reader {
	if len(r.Body) == 0 {
		return nil
	}
	return bytes.NewReader(r.Body)
}

var (
	cookiePair = regexp.MustCompile(`([^()<>@,;:\\"/\[\]?={}\s]+)=([^,;\\"\s]+)`)
	// cookieAttributes are the attributes of Set-Cookie headers, not
	// cookies.
	cookieAttributes = []string{"COMMENT", "COMMENTURL", "DISCORD", "DOMAIN", "EXPIRES", "MAX-AGE", "PATH", "PORT", "SECURE", "VERSION", "HTTPONLY", "SAMESITE"}
)

// ParseCookieHeader reads the cookies of a Cookie header, as Prowlarr's
// CookieUtil.CookieHeaderToDictionary does: malformed ones are skipped,
// and the last of the same name wins.
func ParseCookieHeader(header string) map[string]string {
	cookies := map[string]string{}
	for _, m := range cookiePair.FindAllStringSubmatch(header, -1) {
		if !slices.Contains(cookieAttributes, strings.ToUpper(m[1])) {
			cookies[m[1]] = m[2]
		}
	}
	return cookies
}
