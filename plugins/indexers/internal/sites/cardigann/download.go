package cardigann

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
)

// cookies returns the cookies of the site's requests: the cookie
// setting's, else the indexer's.
func (e *engine) cookies(ix *indexer.Indexer) map[string]string {
	if c := e.settingsCookie(); c != nil {
		return c
	}
	return ix.Cookies()
}

// downloadHeaders returns the headers of download requests: the
// download's, else the search's.
func (e *engine) downloadHeaders(vars Vars) (map[string]string, error) {
	h := e.def.Search.Headers
	if e.def.Download != nil && e.def.Download.Headers != nil {
		h = e.def.Download.Headers
	}
	return e.headers(h, vars)
}

// downloadRequest returns the request downloading a release, as
// Prowlarr's DownloadRequest does: the link itself, or a link selected
// from its page, or a magnet link of the info hash on its page.
func (e *engine) downloadRequest(ctx context.Context, ix *indexer.Indexer, link string) (*indexer.Request, error) {
	cookies := e.cookies(ix)
	method := http.MethodGet
	vars, err := e.baseVariables()
	if err != nil {
		return nil, err
	}
	u, err := url.Parse(link)
	if err != nil {
		return nil, fmt.Errorf("invalid download link %q: %w", link, err)
	}
	addURIVariables(vars, u, ".DownloadUri")
	headers, err := e.downloadHeaders(vars)
	if err != nil {
		return nil, err
	}
	newDownload := func(rawURL string) *indexer.Request {
		r := newRequest(method, rawURL, headers, "")
		r.Cookies = cookies
		r.FollowRedirects = true
		return r
	}

	if d := e.def.Download; d != nil {
		page := newDownload(link)
		page.Method = http.MethodGet
		var resp *indexer.Response
		if b := d.Before; b != nil {
			block := b.RequestBlock
			if b.PathSelector != nil {
				if resp, err = e.do(ctx, ix, page); err != nil {
					return nil, err
				}
				path, ok, err := e.matchSelector(resp, b.PathSelector, vars, false)
				if err != nil {
					return nil, err
				}
				if !ok {
					return nil, errors.New("value cannot be null")
				}
				block.Path = path
			}
			if resp, err = e.handleRequest(ctx, ix, &block, vars, link, cookies); err != nil {
				return nil, err
			}
		}
		if d.Method == methodPost {
			method = http.MethodPost
		}

		switch {
		case d.InfoHash != nil:
			r, err := e.infoHashRequest(ctx, ix, d, page, resp, vars, link, newDownload)
			if err == nil {
				r.Method = method
				return r, nil
			}
			hash, title := "", ""
			if d.InfoHash.Hash != nil {
				hash = d.InfoHash.Hash.Selector
			}
			if d.InfoHash.Title != nil {
				title = d.InfoHash.Title.Selector
			}
			e.log.Error("cardigann: exception with infohash block", "definition", e.def.ID, "hash_selector", hash, "title_selector", title, "error", err)
		case len(d.Selectors) > 0:
			for i := range d.Selectors {
				s := &d.Selectors[i]
				r, err := e.selectorRequest(ctx, ix, d, s, page, &resp, vars, link, newDownload)
				if err != nil {
					e.log.Error("cardigann: an exception occurred while trying a download selector, retrying with the next", "definition", e.def.ID, "selector", s.Selector, "error", err)
					continue
				}
				if r == nil {
					continue
				}
				r.Method = method
				return r, nil
			}
			return nil, indexer.Errorf("Download selectors didn't match for %s", link)
		}
	}
	r := newDownload(link)
	r.Method = method
	return r, nil
}

// infoHashRequest returns a request of the magnet link of the info hash
// and title on the release's page.
func (e *engine) infoHashRequest(ctx context.Context, ix *indexer.Indexer, d *DownloadBlock, page *indexer.Request, resp *indexer.Response, vars Vars, link string, newDownload func(string) *indexer.Request) (*indexer.Request, error) {
	ih := d.InfoHash
	var err error
	if !ih.UseBeforeResponse || d.Before == nil || resp == nil {
		if resp, err = e.do(ctx, ix, page); err != nil {
			return nil, err
		}
	}
	if ih.Hash == nil || ih.Title == nil {
		return nil, errors.New("infohash block needs hash and title selectors")
	}
	hash, ok, err := e.matchSelector(resp, ih.Hash, vars, false)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("InfoHash selectors didn't match hash")
	}
	title, ok, err := e.matchSelector(resp, ih.Title, vars, false)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("InfoHash selectors didn't match title")
	}
	magnet, err := indexer.PublicMagnetLink(hash, title)
	if err != nil {
		return nil, err
	}
	torrentLink, err := e.resolvePath(magnet, link)
	if err != nil {
		return nil, err
	}
	return newDownload(torrentLink), nil
}

// selectorRequest returns a request of the link a download selector
// selects; nil when it selects none or, tested, not a torrent.
func (e *engine) selectorRequest(ctx context.Context, ix *indexer.Indexer, d *DownloadBlock, s *SelectorField, page *indexer.Request, resp **indexer.Response, vars Vars, link string, newDownload func(string) *indexer.Request) (*indexer.Request, error) {
	if _, err := e.applyTemplate(s.Selector, vars, nil); err != nil {
		return nil, err
	}
	if !s.UseBeforeResponse || d.Before == nil || *resp == nil {
		r, err := e.do(ctx, ix, page)
		if err != nil {
			return nil, err
		}
		*resp = r
	}
	href, ok, err := e.matchSelector(*resp, s, vars, true)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	torrentLink, err := e.resolvePath(href, link)
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(torrentLink, "magnet:") && e.def.testsLinks() {
		test := newDownload(torrentLink)
		test.Method = http.MethodGet
		r, err := e.do(ctx, ix, test)
		if err != nil {
			return nil, err
		}
		*resp = r
		if c := r.Content(); len(c) >= 1 && c[0] != 'd' {
			e.log.Debug("cardigann: a download selector's torrent file is invalid, retrying with the next", "definition", e.def.ID, "selector", s.Selector)
			return nil, nil
		}
	}
	return newDownload(torrentLink), nil
}

// handleRequest sends a request block's request, as Prowlarr's
// HandleRequest does.
func (e *engine) handleRequest(ctx context.Context, ix *indexer.Indexer, b *RequestBlock, vars Vars, referer string, cookies map[string]string) (*indexer.Response, error) {
	path, err := e.applyTemplate(b.Path, vars, nil)
	if err != nil {
		return nil, err
	}
	link, err := e.resolvePath(path, "")
	if err != nil {
		return nil, err
	}
	post := strings.EqualFold(b.Method, "post")
	var query nameValues
	var form []Entry[string]
	for _, in := range b.Inputs {
		v, err := e.applyTemplate(in.Value, vars, nil)
		if err != nil {
			return nil, err
		}
		if post {
			if _, dup := Ordered[string](form).Get(in.Key); dup {
				return nil, fmt.Errorf("an item with the same key has already been added: %s", in.Key)
			}
			form = append(form, Entry[string]{in.Key, v})
		} else {
			query.add(in.Key, v)
		}
	}
	if len(query) > 0 {
		if !strings.Contains(link, "?") {
			link += "?"
		}
		link += queryString(query, e.encoding, b.separator())
	}
	headers, err := e.downloadHeaders(vars)
	if err != nil {
		return nil, err
	}
	r := newRequest(http.MethodGet, link, headers, referer)
	r.Cookies = cookies
	if post {
		e.setForm(r, form)
	}
	return e.do(ctx, ix, r)
}

// matchSelector selects a value from a page, as Prowlarr's MatchSelector
// does; ok is false when the selector matches nothing.
func (e *engine) matchSelector(r *indexer.Response, s *SelectorField, vars Vars, debug bool) (string, bool, error) {
	selector, err := e.applyTemplate(s.Selector, vars, nil)
	if err != nil {
		return "", false, err
	}
	doc, err := parseHTML(r.Content())
	if err != nil {
		return "", false, err
	}
	n, err := e.sel.query(doc, selector)
	if err != nil {
		return "", false, err
	}
	if n == nil {
		e.log.Debug("cardigann: selector could not match any elements", "definition", e.def.ID, "selector", selector)
		return "", false, nil
	}
	if debug {
		e.log.Debug("cardigann: download selector matched", "definition", e.def.ID, "selector", selector, "element", outerHTML(n))
	}
	var v string
	if s.Attribute != nil {
		a, ok := attribute(n, *s.Attribute)
		if !ok {
			return "", false, fmt.Errorf("attribute %q is not set for element %s", *s.Attribute, outerHTML(n))
		}
		v = a
	} else {
		v = textContent(n)
	}
	v, err = e.applyFilters(v, s.Filters, vars)
	if err != nil {
		return "", false, err
	}
	return v, true, nil
}
