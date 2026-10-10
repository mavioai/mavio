package torznab

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
	"github.com/mavioai/mavio/plugins/indexers/internal/sites/newznab"
)

// AttrNamespace is the namespace of Torznab's attr elements.
const AttrNamespace = "http://torznab.com/schemas/2015/feed"

// Site is a Torznab indexer.
type Site struct {
	settings newznab.Settings
	caps     newznab.CapsProvider
}

// New returns a Torznab indexer, failing for invalid settings; no
// Torznab indexer needs an API key.
func New(s newznab.Settings) (*Site, error) {
	if err := newznab.ValidateSettings(s, nil); err != nil {
		return nil, err
	}
	return &Site{settings: s}, nil
}

// Info implements indexer.Site.
func (s *Site) Info() indexer.Info {
	return newznab.Info(indexer.ProtocolTorrent, &s.caps)
}

// Capabilities implements indexer.Site, reading the caps.
func (s *Site) Capabilities(ctx context.Context, ix *indexer.Indexer) (*indexer.Capabilities, error) {
	return s.caps.Get(ctx, ix, s.settings)
}

// SearchRequests implements indexer.Site.
func (s *Site) SearchRequests(ctx context.Context, ix *indexer.Indexer, q *indexer.Query) (*indexer.Chain, error) {
	caps, err := s.caps.Get(ctx, ix, s.settings)
	if err != nil {
		return nil, err
	}
	return newznab.SearchRequests(s.settings, caps, q), nil
}

// ParseResponse implements indexer.Site.
func (s *Site) ParseResponse(ctx context.Context, ix *indexer.Indexer, r *indexer.Response) ([]*indexer.Release, error) {
	caps, err := s.caps.Get(ctx, ix, s.settings)
	if err != nil {
		return nil, err
	}
	return NewParser(caps).Parse(ctx, r)
}

// TestSite implements indexer.SiteTester.
func (s *Site) TestSite(ctx context.Context, ix *indexer.Indexer) error {
	caps, err := s.caps.Get(ctx, ix, s.settings)
	if err != nil {
		return err
	}
	return newznab.CheckSearchable(caps)
}

// CheckError returns the error a Torznab response reports, if any.
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
		return fmt.Errorf("%w: invalid API key", indexer.ErrAuth)
	}
	if !strings.Contains(r.Request.URL, "apikey=") && msg == "Missing parameter" {
		return fmt.Errorf("%w: indexer requires an API key", indexer.ErrAuth)
	}
	if msg == "Request limit reached" {
		return &indexer.RateLimitError{Message: "API limit reached"}
	}
	return indexer.Errorf("Torznab error detected: %s", msg)
}

// Parser reads Torznab responses, as Prowlarr's TorznabRssParser does.
type Parser struct {
	*indexer.RSSParser
	// Caps map the indexer's categories.
	Caps *indexer.Capabilities
}

// NewParser returns a parser mapping categories with the capabilities.
func NewParser(caps *indexer.Capabilities) *Parser {
	p := indexer.NewTorrentRSSParser()
	p.UseEnclosureURL = true
	return &Parser{RSSParser: p, Caps: caps}
}

// Parse reads a response's releases.
func (p *Parser) Parse(ctx context.Context, r *indexer.Response) ([]*indexer.Release, error) {
	doc, err := newznab.LoadResponse(r)
	if err != nil {
		return nil, err
	}
	if err := CheckError(doc, r); err != nil {
		return nil, err
	}
	items := indexer.Items(doc)
	if !newznab.CheckEnclosures(ctx, r, items, indexer.TorrentEnclosureTypes, indexer.UsenetEnclosureTypes, "Newznab") {
		return nil, nil
	}
	var releases []*indexer.Release
	for _, n := range items {
		it := &indexer.Item{Node: n, Base: r.URL}
		rel, err := p.ReadItem(it, p)
		if errors.Is(err, indexer.ErrUnsupportedFeed) {
			return nil, err
		}
		if err != nil {
			continue
		}
		p.readAttrs(rel, it)
		releases = append(releases, rel)
	}
	return releases, nil
}

func (p *Parser) readAttrs(rel *indexer.Release, it *indexer.Item) {
	newznab.ReadAttrs(rel, it, AttrNamespace)
	if rel.IMDbID == 0 {
		// Torznab's imdbid is sometimes written with its "tt".
		if v := strings.TrimPrefix(attr(it, "imdbid"), "tt"); v != "" {
			rel.IMDbID, _ = strconv.Atoi(v)
		}
	}
	rel.DownloadVolumeFactor = indexer.Ptr(floatAttr(it, "downloadvolumefactor", 1))
	rel.UploadVolumeFactor = indexer.Ptr(floatAttr(it, "uploadvolumefactor", 1))
	for _, tag := range newznab.Attrs(it, AttrNamespace, "tag") {
		switch strings.ToLower(tag) {
		case "internal":
			if !rel.HasFlag(indexer.FlagInternal) {
				rel.Flags = append(rel.Flags, indexer.FlagInternal)
			}
		case "scene":
			rel.Scene = true
		}
	}
}

func attr(it *indexer.Item, key string) string {
	return strings.TrimSpace(newznab.Attr(it, AttrNamespace, key))
}

func floatAttr(it *indexer.Item, key string, def float64) float64 {
	if f, err := strconv.ParseFloat(attr(it, key), 32); err == nil {
		return f
	}
	return def
}

// Categories implements indexer.ItemReader.
func (p *Parser) Categories(it *indexer.Item) []int {
	return newznab.MapCategories(p.Caps, it, AttrNamespace)
}

// Languages implements indexer.ItemReader.
func (p *Parser) Languages(it *indexer.Item) []string {
	return newznab.Languages(it, AttrNamespace)
}

// InfoURL implements indexer.ItemReader: the comments URL without its
// fragment.
func (p *Parser) InfoURL(it *indexer.Item) string { return newznab.InfoURL(it) }

// Size implements indexer.ItemReader: the size attribute or the
// enclosure's length.
func (p *Parser) Size(it *indexer.Item) (int64, error) {
	return newznab.Size(p.RSSParser, it, AttrNamespace), nil
}

// DownloadURL implements indexer.ItemReader: the torrent enclosure's URL,
// or the first enclosure's.
func (p *Parser) DownloadURL(it *indexer.Item) string {
	u := p.RSSParser.DownloadURL(it)
	if newznab.IsAbsoluteURL(u) {
		return u
	}
	if e := it.Element("enclosure"); e != nil {
		v, _ := e.Attr("url")
		return it.URL(v)
	}
	return u
}

// InfoHash implements indexer.ItemReader: the infohash attribute.
func (p *Parser) InfoHash(it *indexer.Item) string { return attr(it, "infohash") }

// MagnetURL implements indexer.ItemReader: the magneturl attribute.
func (p *Parser) MagnetURL(it *indexer.Item) string { return attr(it, "magneturl") }

// Seeders implements indexer.ItemReader: the seeders attribute.
func (p *Parser) Seeders(it *indexer.Item) (*int, error) {
	if v := attr(it, "seeders"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("seeders %q: %w", v, err)
		}
		return &n, nil
	}
	return p.RSSParser.Seeders(it)
}

// Peers implements indexer.ItemReader: the peers attribute, or seeders
// and leechers.
func (p *Parser) Peers(it *indexer.Item) (*int, error) {
	if v := attr(it, "peers"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("peers %q: %w", v, err)
		}
		return &n, nil
	}
	if s, l := attr(it, "seeders"), attr(it, "leechers"); s != "" && l != "" {
		seeders, err1 := strconv.Atoi(s)
		leechers, err2 := strconv.Atoi(l)
		if err := errors.Join(err1, err2); err != nil {
			return nil, fmt.Errorf("seeders and leechers: %w", err)
		}
		return indexer.Ptr(seeders + leechers), nil
	}
	return p.RSSParser.Peers(it)
}
