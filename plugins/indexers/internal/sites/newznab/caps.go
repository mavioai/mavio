package newznab

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
)

// capsLife is how long an indexer's capabilities are kept.
const capsLife = 7 * 24 * time.Hour

// CapsProvider reads an indexer's capabilities from its caps and keeps
// them for a week, as Prowlarr's NewznabCapabilitiesProvider does.
type CapsProvider struct {
	// fetching serializes reads of the caps; mu guards what is kept.
	fetching sync.Mutex
	mu       sync.Mutex
	caps     *indexer.Capabilities
	fetched  time.Time
}

// Get returns the capabilities, reading them when not kept.
func (p *CapsProvider) Get(ctx context.Context, ix *indexer.Indexer, s Settings) (*indexer.Capabilities, error) {
	now := time.Now
	if ix.Now != nil {
		now = ix.Now
	}
	p.fetching.Lock()
	defer p.fetching.Unlock()
	p.mu.Lock()
	caps, fetched := p.caps, p.fetched
	p.mu.Unlock()
	if caps != nil && now().Sub(fetched) < capsLife {
		return caps, nil
	}
	caps, err := fetchCapabilities(ctx, ix, s)
	if err != nil {
		return nil, err
	}
	p.mu.Lock()
	p.caps, p.fetched = caps, now()
	p.mu.Unlock()
	return caps, nil
}

// Set keeps capabilities read at a time, as stored capabilities are.
func (p *CapsProvider) Set(caps *indexer.Capabilities, fetched time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.caps, p.fetched = caps, fetched
}

// Cached returns the capabilities kept, nil when there are none.
func (p *CapsProvider) Cached() *indexer.Capabilities {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.caps
}

func fetchCapabilities(ctx context.Context, ix *indexer.Indexer, s Settings) (*indexer.Capabilities, error) {
	u := s.apiURL() + "?t=caps"
	if strings.TrimSpace(s.APIKey) != "" {
		u += "&apikey=" + s.APIKey
	}
	r := indexer.NewRequest(u, indexer.AcceptRSS)
	r.FollowRedirects = true
	resp, err := ix.Do(ctx, r)
	if err != nil {
		return nil, fmt.Errorf("get capabilities of %s: %w", s.BaseURL, err)
	}
	if resp.HasError() && !resp.HasContentType("xml") {
		return nil, &indexer.HTTPError{Response: resp}
	}
	caps, err := ParseCapabilities(resp)
	if errors.Is(err, errUnexpectedData) {
		// Prowlarr keeps the defaults when the caps hold what it cannot
		// read.
		slog.DebugContext(ctx, "unexpected Newznab capabilities", "indexer", ix.Name, "error", err)
		return indexer.NewCapabilities(), nil
	}
	return caps, err
}

// errUnexpectedData is returned for caps whose values do not parse.
var errUnexpectedData = errors.New("unexpected data")

// ParseCapabilities reads capabilities from a caps response.
func ParseCapabilities(r *indexer.Response) (*indexer.Capabilities, error) {
	doc, err := indexer.LoadXML(r.Content())
	if err != nil {
		return nil, fmt.Errorf("capabilities: %w", err)
	}
	if err := CheckError(doc, r); err != nil {
		return nil, err
	}
	root := doc.Root
	if root == nil || root.Name.Local != "caps" || root.Name.Space != "" {
		return nil, errors.New("capabilities: unexpected XML")
	}
	caps := indexer.NewCapabilities()
	if limits := root.Element("limits"); limits != nil {
		if caps.LimitsDefault, err = intAttr(limits, "default"); err != nil {
			return nil, err
		}
		if caps.LimitsMax, err = intAttr(limits, "max"); err != nil {
			return nil, err
		}
	}
	if searching := root.Element("searching"); searching != nil {
		for _, s := range []struct {
			element string
			t       indexer.SearchType
		}{
			{"search", indexer.SearchBasic},
			{"movie-search", indexer.SearchMovie},
			{"tv-search", indexer.SearchTV},
			{"audio-search", indexer.SearchMusic},
			{"book-search", indexer.SearchBook},
		} {
			readSearch(caps, searching.Element(s.element), s.t)
		}
	}
	if cats := root.Element("categories"); cats != nil {
		if err := readCategories(caps, cats); err != nil {
			return nil, err
		}
	}
	return caps, nil
}

// readSearch reads the parameters of a search type; unknown parameters
// are ignored.
func readSearch(caps *indexer.Capabilities, e *indexer.Node, t indexer.SearchType) {
	if available, _ := e.Attr("available"); available != "yes" {
		caps.Params[t] = nil
		return
	}
	params, ok := e.Attr("supportedParams")
	if !ok {
		caps.Params[t] = []indexer.Param{indexer.ParamQ}
		return
	}
	for p := range strings.SplitSeq(params, ",") {
		// Unknown and repeated parameters are skipped.
		_ = caps.AddParams(t, []string{strings.TrimSpace(p)})
	}
	if t == indexer.SearchBasic {
		engine, _ := e.Attr("searchEngine")
		caps.RawSearch = engine == "raw"
	}
}

// readCategories maps the indexer's categories to standard ones by name
// or ID.
func readCategories(caps *indexer.Capabilities, cats *indexer.Node) error {
	parents := indexer.ParentCategories()
	all := indexer.StandardCategories()
	find := func(cs []*indexer.Category, match func(c *indexer.Category) bool) *indexer.Category {
		for _, c := range cs {
			if match(c) {
				return c
			}
		}
		return nil
	}
	for _, cat := range cats.Elements("category") {
		parentName, _ := cat.Attr("name")
		parentID, err := intAttr(cat, "id")
		if err != nil {
			return err
		}
		lowerParent := strings.ToLower(parentName)
		mapped := find(parents, func(c *indexer.Category) bool { return strings.EqualFold(c.Name, parentName) })
		if mapped == nil {
			// Subcategories listed as parents.
			mapped = find(all, func(c *indexer.Category) bool {
				return c.ID == parentID && strings.Contains(strings.ToLower(c.Name), lowerParent)
			})
		}
		if mapped == nil {
			mapped = find(parents, func(c *indexer.Category) bool { return strings.Contains(lowerParent, strings.ToLower(c.Name)) })
		}
		if mapped == nil {
			mapped = find(parents, func(c *indexer.Category) bool { return c.ID == parentID })
		}
		if mapped == nil {
			mapped = indexer.Other
		}
		for _, sub := range cat.Elements("subcat") {
			subName, _ := sub.Attr("name")
			subID, err := intAttr(sub, "id")
			if err != nil {
				return err
			}
			name := strings.ToLower(mapped.Name + "/" + subName)
			mappedSub := find(all, func(c *indexer.Category) bool { return strings.ToLower(c.Name) == name })
			if mappedSub == nil {
				mappedSub = find(all, func(c *indexer.Category) bool { return c.ID == subID })
			}
			if mappedSub == nil && mapped.ID != indexer.Other.ID {
				other := strings.ToLower(mapped.Name) + "/other"
				mappedSub = find(all, func(c *indexer.Category) bool { return strings.ToLower(c.Name) == other })
			}
			if mappedSub == nil {
				mappedSub = indexer.OtherMisc
			}
			caps.Categories.Add(strconv.Itoa(subID), mappedSub, parentName+"/"+subName)
		}
		caps.Categories.Add(strconv.Itoa(parentID), mapped, parentName)
	}
	return nil
}

func intAttr(n *indexer.Node, name string) (int, error) {
	v, ok := n.Attr(name)
	if !ok {
		return 0, fmt.Errorf("%w: %s has no %s", errUnexpectedData, n.Name.Local, name)
	}
	i, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return 0, fmt.Errorf("%w: %s %s %q", errUnexpectedData, n.Name.Local, name, v)
	}
	return i, nil
}

// CheckError returns the error a Newznab response reports, if any, as
// Prowlarr's NewznabRssParser.CheckError does.
func CheckError(doc *indexer.Document, r *indexer.Response) error {
	errs := doc.Root.Descendants("error")
	if doc.Root != nil && strings.EqualFold(doc.Root.Name.Local, "error") {
		errs = append([]*indexer.Node{doc.Root}, errs...)
	}
	if len(errs) == 0 {
		return nil
	}
	codeValue, _ := errs[0].Attr("code")
	msg, _ := errs[0].Attr("description")
	if code, err := strconv.Atoi(strings.TrimSpace(codeValue)); err == nil && code >= 100 && code <= 199 {
		return fmt.Errorf("%w: %s", indexer.ErrAuth, msg)
	}
	if !strings.Contains(r.Request.URL, "apikey=") && (msg == "Missing parameter" || strings.Contains(msg, "apikey")) {
		return fmt.Errorf("%w: indexer requires an API key", indexer.ErrAuth)
	}
	if msg == "Request limit reached" {
		return &indexer.RateLimitError{Message: "API limit reached"}
	}
	return indexer.Errorf("Newznab error detected: %s", msg)
}
