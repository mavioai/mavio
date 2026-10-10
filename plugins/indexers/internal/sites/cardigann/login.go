package cardigann

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"

	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
)

// The methods of definitions, in lower case.
const (
	methodGet  = "get"
	methodPost = "post"
)

// cookieLife is how long the cookies of logins are kept.
const cookieLife = 30 * 24 * time.Hour

// parseCookieHeader parses a Cookie header's cookies, as Prowlarr's
// CookieUtil.CookieHeaderToDictionary does.
func parseCookieHeader(header string) map[string]string {
	cookies := map[string]string{}
	for part := range strings.SplitSeq(header, ";") {
		k, v, ok := strings.Cut(strings.TrimSpace(part), "=")
		if ok && k != "" {
			cookies[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	return cookies
}

// settingsCookie returns the cookies of the cookie setting; nil when unset.
func (e *engine) settingsCookie() map[string]string {
	s, _ := e.settings.Fields["cookie"].(string)
	if s == "" {
		return nil
	}
	return parseCookieHeader(s)
}

// loginHeaders returns the headers of login requests: the login's, else
// the search's.
func (e *engine) loginHeaders(vars Vars) (map[string]string, error) {
	h := e.def.Search.Headers
	if e.def.Login != nil && e.def.Login.Headers != nil {
		h = e.def.Login.Headers
	}
	return e.headers(h, vars)
}

// newRequest returns a request with the headers and a referer.
func newRequest(method, rawURL string, headers map[string]string, referer string) *indexer.Request {
	r := indexer.NewRequest(rawURL, "")
	r.Method = method
	setHeaders(r, headers)
	if referer != "" {
		r.Header.Set("Referer", referer)
	}
	return r
}

// login logs in as Prowlarr's DoLogin does, keeping the cookies.
func (e *engine) login(ctx context.Context, ix *indexer.Indexer) error {
	login := e.def.Login
	if login == nil {
		return nil
	}
	vars, err := e.baseVariables()
	if err != nil {
		return err
	}
	headers, err := e.loginHeaders(vars)
	if err != nil {
		return err
	}
	path, err := e.applyTemplate(login.Path, vars, nil)
	if err != nil {
		return err
	}
	inputs := func() ([]Entry[string], error) {
		var out []Entry[string]
		for _, in := range login.Inputs {
			v, err := e.applyTemplate(in.Value, nil, nil)
			if err != nil {
				return nil, err
			}
			out = append(out, Entry[string]{in.Key, v})
		}
		return out, nil
	}

	var resp *indexer.Response
	var sent map[string]string
	switch login.Method {
	case methodPost:
		pairs, err := inputs()
		if err != nil {
			return err
		}
		loginURL, err := e.resolvePath(path, "")
		if err != nil {
			return err
		}
		ix.SetCookies(map[string]string{}, time.Time{})
		r := newRequest(http.MethodPost, loginURL, headers, e.siteLink())
		e.setForm(r, uniqueKeys(pairs))
		r.FollowRedirects = true
		r.AllowedStatus = allStatuses
		if login.Cookies != nil {
			r.Cookies = parseCookieHeader(strings.Join(login.Cookies, "; "))
		}
		sent = r.Cookies
		if resp, err = e.do(ctx, ix, r); err != nil {
			return err
		}
	case "form":
		if resp, sent, err = e.formLogin(ctx, ix, path, vars, headers); err != nil {
			return err
		}
	case "cookie":
		ix.SetCookies(e.settingsCookie(), e.now().Add(cookieLife))
		return nil
	case methodGet, "oneurl":
		link := path
		if login.Method == methodGet {
			pairs, err := inputs()
			if err != nil {
				return err
			}
			link += "?" + nameValuesQuery(pairs)
		} else {
			oneURL, ok := login.Inputs.Get("oneurl")
			if !ok {
				return fmt.Errorf("the given key '%s' was not present in the dictionary", "oneurl")
			}
			v, err := e.applyTemplate(oneURL, nil, nil)
			if err != nil {
				return err
			}
			link += v
		}
		loginURL, err := e.resolvePath(link, "")
		if err != nil {
			return err
		}
		ix.SetCookies(map[string]string{}, time.Time{})
		r := newRequest(http.MethodGet, loginURL, headers, e.siteLink())
		r.AllowedStatus = allStatuses
		if resp, err = e.do(ctx, ix, r); err != nil {
			return err
		}
	default:
		return fmt.Errorf("login method %s not implemented", login.Method)
	}

	cookies := maps.Clone(sent)
	if cookies == nil {
		cookies = map[string]string{}
	}
	maps.Copy(cookies, resp.Cookies)
	if err := e.checkForError(resp, login.Error); err != nil {
		return err
	}
	ix.SetCookies(cookies, e.now().Add(cookieLife))
	return nil
}

// allStatuses are the error statuses logins read themselves.
var allStatuses = func() []int {
	var s []int
	for c := 400; c < 600; c++ {
		s = append(s, c)
	}
	return s
}()

// uniqueKeys keeps the last value of each key, in the order keys first
// appear, as a dictionary does.
func uniqueKeys(pairs []Entry[string]) []Entry[string] {
	var out []Entry[string]
	index := map[string]int{}
	for _, p := range pairs {
		if i, ok := index[p.Key]; ok {
			out[i].Value = p.Value
			continue
		}
		index[p.Key] = len(out)
		out = append(out, p)
	}
	return out
}

// nameValuesQuery writes a query string of a NameValueCollection: values of
// repeated keys joined by commas, keys as they are.
func nameValuesQuery(pairs []Entry[string]) string {
	var n nameValues
	for _, p := range pairs {
		n.add(p.Key, p.Value)
	}
	return queryString(n, nil, "&")
}

// landingPage fetches the login form's page, as Prowlarr's
// GetConfigurationForSetup does, failing when it asks for a captcha.
func (e *engine) landingPage(ctx context.Context, ix *indexer.Indexer, loginURL string, headers map[string]string) (*html.Node, map[string]string, error) {
	login := e.def.Login
	r := newRequest(http.MethodGet, loginURL, headers, e.siteLink())
	r.FollowRedirects = e.def.FollowRedirect
	if login.Cookies != nil {
		r.Cookies = parseCookieHeader(strings.Join(login.Cookies, "; "))
	}
	resp, err := e.do(ctx, ix, r)
	if err != nil {
		return nil, nil, err
	}
	cookies := maps.Clone(r.Cookies)
	if cookies == nil {
		cookies = map[string]string{}
	}
	maps.Copy(cookies, resp.Cookies)
	doc, err := parseHTML(resp.Content())
	if err != nil {
		return nil, nil, indexer.Errorf("Login failed: Invalid document received from %s", loginURL)
	}
	if c := login.Captcha; c != nil {
		if c.Type != "image" {
			return nil, nil, fmt.Errorf("captcha type %q is not implemented", c.Type)
		}
		n, err := e.sel.query(doc, c.Selector)
		if err != nil {
			return nil, nil, err
		}
		if n != nil {
			return nil, nil, indexer.Errorf("Found captcha during automatic login, aborting")
		}
		e.log.Debug("cardigann: no captcha image found", "definition", e.def.ID)
	}
	return doc, cookies, nil
}

// formLogin fills and posts the login page's form.
func (e *engine) formLogin(ctx context.Context, ix *indexer.Indexer, path string, vars Vars, headers map[string]string) (*indexer.Response, map[string]string, error) {
	login := e.def.Login
	loginURL, err := e.resolvePath(path, "")
	if err != nil {
		return nil, nil, err
	}
	doc, cookies, err := e.landingPage(ctx, ix, loginURL, headers)
	if err != nil {
		return nil, nil, err
	}
	formSelector := login.Form
	if formSelector == "" {
		formSelector = "form"
	}
	form, err := e.sel.query(doc, formSelector)
	if err != nil {
		return nil, nil, err
	}
	if form == nil {
		return nil, nil, indexer.Errorf("Login failed: No form found on %s using form selector %s", loginURL, formSelector)
	}
	fields, err := e.sel.queryAll(form, "input")
	if err != nil {
		return nil, nil, err
	}
	submit, _ := attribute(form, "action")
	if login.SubmitPath != "" {
		submit = login.SubmitPath
	}

	var pairs []Entry[string]
	for _, in := range fields {
		name, ok := attribute(in, "name")
		if !ok {
			continue
		}
		if _, disabled := attribute(in, "disabled"); disabled {
			continue
		}
		t, _ := attribute(in, "type")
		if t = strings.ToLower(t); t == "checkbox" || t == "radio" {
			if _, checked := attribute(in, "checked"); !checked {
				continue
			}
		}
		v, _ := attribute(in, "value")
		pairs = append(pairs, Entry[string]{name, v})
	}
	inputName := func(selector string) (string, error) {
		n, err := e.sel.query(doc, selector)
		if err != nil {
			return "", err
		}
		if n == nil {
			return "", indexer.Errorf("Login failed: No input found using selector %s", selector)
		}
		name, _ := attribute(n, "name")
		return name, nil
	}
	for _, in := range login.Inputs {
		v, err := e.applyTemplate(in.Value, nil, nil)
		if err != nil {
			return nil, nil, err
		}
		key := in.Key
		if login.Selectors {
			if key, err = inputName(in.Key); err != nil {
				return nil, nil, err
			}
		}
		pairs = append(pairs, Entry[string]{key, v})
	}
	root := documentElement(doc)
	selectorInputs := func(blocks Ordered[SelectorBlock], what string) ([]Entry[string], error) {
		var out []Entry[string]
		for i := range blocks {
			b := &blocks[i]
			v, ok, err := e.handleSelector(&b.Value, root, nil, !b.Value.Optional)
			if err != nil {
				selector := ""
				if b.Value.Selector != nil {
					selector = *b.Value.Selector
				}
				return nil, fmt.Errorf("error while parsing %s=%s, selector=%s: %w", what, b.Key, selector, err)
			}
			if b.Value.Optional && !ok {
				continue
			}
			out = append(out, Entry[string]{b.Key, v})
		}
		return out, nil
	}
	more, err := selectorInputs(login.SelectorInputs, "selector input")
	if err != nil {
		return nil, nil, err
	}
	pairs = append(pairs, more...)
	query, err := selectorInputs(login.GetSelectorInputs, "get selector input")
	if err != nil {
		return nil, nil, err
	}
	if len(query) > 0 {
		var n nameValues
		for _, q := range query {
			n.set(q.Key, q.Value)
		}
		submit += "?" + queryString(n, nil, "&")
	}
	submitURL, err := e.resolvePath(submit, loginURL)
	if err != nil {
		return nil, nil, err
	}

	// simpleCaptcha is solved by choosing its one image.
	if n, err := e.sel.query(doc, `script[src*="simpleCaptcha"]`); err != nil {
		return nil, nil, err
	} else if n != nil {
		captchaURL, err := e.resolvePath("simpleCaptcha.php?numImages=1", "")
		if err != nil {
			return nil, nil, err
		}
		r := newRequest(http.MethodGet, captchaURL, headers, loginURL)
		r.Cookies = cookies
		resp, err := e.do(ctx, ix, r)
		if err != nil {
			return nil, nil, err
		}
		var captcha struct {
			Images []struct {
				Hash json.RawMessage `json:"hash"`
			} `json:"images"`
		}
		if err := json.Unmarshal([]byte(resp.Content()), &captcha); err != nil {
			return nil, nil, fmt.Errorf("simpleCaptcha: %w", err)
		}
		if len(captcha.Images) == 0 {
			return nil, nil, fmt.Errorf("simpleCaptcha: no images")
		}
		hash := string(captcha.Images[0].Hash)
		if s, err := strconv.Unquote(hash); err == nil {
			hash = s
		}
		pairs = append(pairs, Entry[string]{"captchaSelection", hash}, Entry[string]{"submitme", "X"})
	}
	if c := login.Captcha; c != nil {
		if text, _ := e.settings.Fields["CAPTCHA"].(string); !isBlank(text) {
			key := c.Input
			if login.Selectors {
				if key, err = inputName(c.Input); err != nil {
					return nil, nil, err
				}
			}
			pairs = append(pairs, Entry[string]{key, text})
		}
	}
	pairs = uniqueKeys(pairs)

	r := newRequest(http.MethodPost, submitURL, headers, loginURL)
	r.FollowRedirects = true
	r.Cookies = cookies
	if enctype, _ := attribute(form, "enctype"); enctype == "multipart/form-data" {
		boundary := "---------------------------" + strconv.FormatInt(e.now().UnixMicro()/100, 10)
		parts := make([]string, 0, len(pairs)+1)
		for _, p := range pairs {
			parts = append(parts, "--"+boundary+"\r\nContent-Disposition: form-data; name=\""+p.Key+"\"\r\n\r\n"+p.Value)
		}
		parts = append(parts, "--"+boundary+"--")
		r.Header.Set("Referer", e.siteLink())
		r.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)
		r.Body = encode(strings.Join(parts, "\r\n"), e.encoding)
	} else {
		e.setForm(r, pairs)
		r.AllowedStatus = allStatuses
	}
	resp, err := e.do(ctx, ix, r)
	if err != nil {
		return nil, nil, err
	}
	return resp, cookies, nil
}

// checkForError fails for 401 responses and pages with an error block's
// element, as Prowlarr's CheckForError does.
func (e *engine) checkForError(r *indexer.Response, blocks []ErrorBlock) error {
	if r.StatusCode == http.StatusUnauthorized {
		return &indexer.HTTPError{Response: r}
	}
	if blocks == nil {
		return nil
	}
	doc, err := parseHTML(r.Content())
	if err != nil {
		return err
	}
	for i := range blocks {
		b := &blocks[i]
		n, err := e.sel.query(doc, b.Selector)
		if err != nil {
			return err
		}
		if n == nil {
			continue
		}
		msg := textContent(n)
		if b.Message != nil {
			if msg, _, err = e.handleSelector(b.Message, documentElement(doc), nil, true); err != nil {
				return err
			}
		}
		return indexer.Errorf("Error: %s", strings.TrimSpace(msg))
	}
	return nil
}

// loginNeeded reports whether a response asks to log in, as Prowlarr's
// CheckIfLoginIsNeeded does.
func (e *engine) loginNeeded(r *indexer.Response) bool {
	login := e.def.Login
	if login == nil {
		return false
	}
	if r.IsRedirect() {
		if hint := e.redirectDomainHint(r.Request.URL, r.RedirectURL()); hint != "" {
			e.log.Warn("cardigann: got redirected to another domain, try changing the indexer URL to "+hint, "definition", e.def.ID)
		}
		return true
	}
	if r.StatusCode >= 400 {
		return true
	}
	ct := r.Header.Get("Content-Type")
	if login.Test != nil && login.Test.Selector != "" && (ct == "" || strings.Contains(ct, "text/html")) {
		doc, err := parseHTML(r.Content())
		if err != nil {
			return true
		}
		nodes, err := e.sel.queryAll(doc, login.Test.Selector)
		if err != nil || len(nodes) == 0 {
			return true
		}
	}
	return false
}

// redirectDomainHint returns the site a request to the site's host was
// redirected to, when it is another; "" otherwise.
func (e *engine) redirectDomainHint(requestURL, redirectURL string) string {
	site, err1 := url.Parse(e.siteLink())
	req, err2 := url.Parse(requestURL)
	to, err3 := url.Parse(redirectURL)
	if err1 != nil || err2 != nil || err3 != nil {
		return ""
	}
	if strings.HasPrefix(req.Hostname(), site.Hostname()) && !strings.HasPrefix(to.Hostname(), site.Hostname()) {
		return to.Scheme + "://" + to.Hostname() + "/"
	}
	return ""
}

// do sends a request of the site's, failing for error statuses the
// request does not allow.
func (e *engine) do(ctx context.Context, ix *indexer.Indexer, r *indexer.Request) (*indexer.Response, error) {
	resp, err := ix.Do(ctx, r)
	if err != nil {
		return nil, err
	}
	if resp.HasError() {
		return nil, &indexer.HTTPError{Response: resp}
	}
	return resp, nil
}
