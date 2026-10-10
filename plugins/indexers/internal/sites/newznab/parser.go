package newznab

import (
	"context"
	"errors"
	"log/slog"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
)

// AttrNamespace is the namespace of Newznab's attr elements.
const AttrNamespace = "http://www.newznab.com/DTD/2010/feeds/attributes/"

// Parser reads Newznab responses, as Prowlarr's NewznabRssParser does.
type Parser struct {
	*indexer.RSSParser
	// Caps map the indexer's categories.
	Caps *indexer.Capabilities
}

// NewParser returns a parser mapping categories with the capabilities.
func NewParser(caps *indexer.Capabilities) *Parser {
	return &Parser{
		RSSParser: &indexer.RSSParser{UseEnclosureURL: true, PreferredEnclosureTypes: indexer.UsenetEnclosureTypes},
		Caps:      caps,
	}
}

// LoadResponse checks a Newznab or Torznab response and loads its XML;
// HTTP errors with XML bodies are read for the error they report.
func LoadResponse(r *indexer.Response) (*indexer.Document, error) {
	if r.HasError() && !r.HasContentType("xml") {
		if err := indexer.CheckResponse(r); err != nil {
			return nil, err
		}
	}
	return indexer.LoadXML(r.Content())
}

// Parse reads a response's releases.
func (p *Parser) Parse(ctx context.Context, r *indexer.Response) ([]*indexer.Release, error) {
	doc, err := LoadResponse(r)
	if err != nil {
		return nil, err
	}
	if err := CheckError(doc, r); err != nil {
		return nil, err
	}
	items := indexer.Items(doc)
	if !CheckEnclosures(ctx, r, items, indexer.UsenetEnclosureTypes, indexer.TorrentEnclosureTypes, "Torznab") {
		return nil, nil
	}
	var releases []*indexer.Release
	for _, n := range items {
		if !hasNZBEnclosure(n) {
			continue
		}
		it := &indexer.Item{Node: n, Base: r.URL}
		rel, err := p.ReadItem(it, p)
		if errors.Is(err, indexer.ErrUnsupportedFeed) {
			return nil, err
		}
		if err != nil {
			continue
		}
		ReadAttrs(rel, it, AttrNamespace)
		releases = append(releases, rel)
	}
	return releases, nil
}

// CheckEnclosures reports whether the items' enclosures may be of the
// protocol: false, with a warning, when none is of its types and some are
// of the other protocol's, whose API is named.
func CheckEnclosures(ctx context.Context, r *indexer.Response, items []*indexer.Node, types, otherTypes []string, other string) bool {
	var found []string
	for _, n := range items {
		for _, e := range indexer.Enclosures(n) {
			if !slices.Contains(found, e.Type) {
				found = append(found, e.Type)
			}
		}
	}
	if len(found) == 0 || slices.ContainsFunc(found, func(t string) bool { return slices.Contains(types, t) }) {
		return true
	}
	if slices.ContainsFunc(found, func(t string) bool { return slices.Contains(otherTypes, t) }) {
		slog.WarnContext(ctx, "feed has enclosures of the other protocol; is it a "+other+" indexer?", "url", r.Request.URL, "want", types[0], "found", found[0])
		return false
	}
	slog.WarnContext(ctx, "feed has no enclosures of the protocol", "url", r.Request.URL, "want", types[0], "found", found[0])
	return true
}

// hasNZBEnclosure reports whether an item has an enclosure of an NZB or
// of no type.
func hasNZBEnclosure(n *indexer.Node) bool {
	return nzbEnclosure(n) != nil
}

func nzbEnclosure(n *indexer.Node) *indexer.Node {
	for _, e := range n.Elements("enclosure") {
		if t, ok := e.Attr("type"); !ok || t == indexer.NZBEnclosureType {
			return e
		}
	}
	return nil
}

// Attrs returns the values of an item's attr elements of the namespace
// named key, in any case.
func Attrs(it *indexer.Item, space, key string) []string {
	var out []string
	for _, e := range it.ElementsNS(space, "attr") {
		if name, _ := e.Attr("name"); strings.EqualFold(name, key) {
			if v, ok := e.Attr("value"); ok {
				out = append(out, v)
			}
		}
	}
	return out
}

// Attr returns the value of an item's first attr element of the
// namespace named key, "" for none.
func Attr(it *indexer.Item, space, key string) string {
	if vs := Attrs(it, space, key); len(vs) > 0 {
		return vs[0]
	}
	return ""
}

// intAttrOf returns the first of the attributes that is a number.
func intAttrOf(it *indexer.Item, space string, keys ...string) (int, bool) {
	for _, k := range keys {
		if n, err := strconv.Atoi(strings.TrimSpace(Attr(it, space, k))); err == nil {
			return n, true
		}
	}
	return 0, false
}

// ReadAttrs reads the IDs, grabs, files and poster of a Newznab or
// Torznab item.
func ReadAttrs(rel *indexer.Release, it *indexer.Item, space string) {
	ids := []struct {
		dst  *int
		keys []string
	}{
		{&rel.IMDbID, []string{"imdb", "imdbid"}},
		{&rel.TMDBID, []string{"tmdbid", "tmdb"}},
		{&rel.TVDBID, []string{"tvdbid", "tvdb"}},
		{&rel.TVMazeID, []string{"tvmazeid", "tvmaze"}},
		{&rel.TraktID, []string{"traktid", "trakt"}},
		{&rel.TVRageID, []string{"rageid"}},
	}
	for _, id := range ids {
		*id.dst, _ = intAttrOf(it, space, id.keys...)
	}
	if n, ok := intAttrOf(it, space, "grabs"); ok {
		rel.Grabs = &n
	}
	if n, ok := intAttrOf(it, space, "files"); ok {
		rel.Files = &n
	}
	rel.PosterURL = it.URL(Attr(it, space, "coverurl"))
}

// Categories implements indexer.ItemReader: the category attributes or
// elements, mapped by the capabilities.
func (p *Parser) Categories(it *indexer.Item) []int {
	return MapCategories(p.Caps, it, AttrNamespace)
}

// MapCategories maps the categories an item's attributes, or else its
// category elements, give.
func MapCategories(caps *indexer.Capabilities, it *indexer.Item, space string) []int {
	cats := Attrs(it, space, "category")
	if len(cats) == 0 {
		for _, e := range it.Elements("category") {
			cats = append(cats, e.Value())
		}
	}
	var out []int
	for _, c := range cats {
		for _, m := range caps.Categories.MapTracker(c) {
			out = append(out, m.ID)
		}
	}
	return out
}

// Languages implements indexer.ItemReader: the language attributes or
// elements, split at commas.
func (p *Parser) Languages(it *indexer.Item) []string {
	var out []string
	for _, l := range Languages(it, AttrNamespace) {
		for part := range strings.SplitSeq(l, ",") {
			if part = strings.TrimSpace(part); part != "" {
				out = append(out, part)
			}
		}
	}
	return out
}

// Languages returns the language attributes, or else elements, that are
// not blank.
func Languages(it *indexer.Item, space string) []string {
	ls := Attrs(it, space, "language")
	if len(ls) == 0 {
		for _, e := range it.Elements("language") {
			ls = append(ls, e.Value())
		}
	}
	return slices.DeleteFunc(ls, func(l string) bool { return strings.TrimSpace(l) == "" })
}

// InfoURL implements indexer.ItemReader: the comments URL without its
// fragment.
func (p *Parser) InfoURL(it *indexer.Item) string { return InfoURL(it) }

// InfoURL returns an item's comments URL without "#comments".
func InfoURL(it *indexer.Item) string {
	return it.URL(strings.TrimSuffix(it.ChildValue("comments", ""), "#comments"))
}

// Size implements indexer.ItemReader: the size attribute or the
// enclosure's length.
func (p *Parser) Size(it *indexer.Item) (int64, error) {
	return Size(p.RSSParser, it, AttrNamespace), nil
}

// Size returns an item's size attribute or its enclosure's length.
func Size(p *indexer.RSSParser, it *indexer.Item, space string) int64 {
	if n, err := strconv.ParseInt(strings.TrimSpace(Attr(it, space, "size")), 10, 64); err == nil {
		return n
	}
	if e := p.Enclosure(indexer.Enclosures(it.Node), true); e != nil {
		return e.Length
	}
	return 0
}

// PublishDate implements indexer.ItemReader: the usenetdate attribute or
// pubDate.
func (p *Parser) PublishDate(it *indexer.Item) (time.Time, error) {
	if d := Attr(it, AttrNamespace, "usenetdate"); strings.TrimSpace(d) != "" {
		return indexer.ParseDate(d)
	}
	return p.RSSParser.PublishDate(it)
}

// DownloadURL implements indexer.ItemReader: the NZB enclosure's URL.
func (p *Parser) DownloadURL(it *indexer.Item) string {
	u := p.RSSParser.DownloadURL(it)
	if IsAbsoluteURL(u) {
		return u
	}
	if e := nzbEnclosure(it.Node); e != nil {
		if v, ok := e.Attr("url"); ok {
			return v
		}
	}
	return u
}

// MagnetURL implements indexer.ItemReader; NZBs have none.
func (p *Parser) MagnetURL(*indexer.Item) string { return "" }

// InfoHash implements indexer.ItemReader; NZBs have none.
func (p *Parser) InfoHash(*indexer.Item) string { return "" }

// IsAbsoluteURL reports whether s is an absolute URL.
func IsAbsoluteURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && u.IsAbs() && u.Host != ""
}
