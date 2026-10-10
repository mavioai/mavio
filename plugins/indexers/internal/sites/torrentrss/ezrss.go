package torrentrss

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
)

// ezrssNamespace is the namespace of EZRSS feeds' torrent elements.
const ezrssNamespace = "http://xmlns.ezrss.it/0.1/"

// ezrssParser reads EZRSS feeds, as Prowlarr's EzrssTorrentRssParser does.
type ezrssParser struct {
	*indexer.RSSParser
}

func newEzrssParser() *ezrssParser {
	p := indexer.NewTorrentRSSParser()
	p.UseGUIDInfoURL = true
	p.UseEnclosureURL = true
	return &ezrssParser{p}
}

func (p *ezrssParser) Parse(r *indexer.Response) ([]*indexer.Release, error) {
	if r.HasError() {
		if err := indexer.CheckResponse(r); err != nil {
			return nil, err
		}
	}
	doc, err := indexer.LoadXML(r.Content())
	if err != nil {
		return nil, err
	}
	items := indexer.Items(doc)
	if len(items) == 1 && p.Title(&indexer.Item{Node: items[0]}) == "No items exist - Try again later" {
		return nil, indexer.Errorf("no results were found")
	}
	return p.ParseItems(r, items, p)
}

func (p *ezrssParser) Size(it *indexer.Item) (int64, error) {
	n, err := singleDescendant(it, "contentLength")
	if err != nil || n == nil {
		return p.RSSParser.Size(it)
	}
	return strconv.ParseInt(strings.TrimSpace(n.Value()), 10, 64)
}

func (p *ezrssParser) InfoHash(it *indexer.Item) string {
	n, _ := singleDescendant(it, "infoHash")
	return n.Value()
}

func (p *ezrssParser) MagnetURL(it *indexer.Item) string {
	n, _ := singleDescendant(it, "magnetURI")
	return n.Value()
}

func (p *ezrssParser) Seeders(it *indexer.Item) (*int, error) {
	return p.count(it, "seeds", p.RSSParser.Seeders)
}

func (p *ezrssParser) Peers(it *indexer.Item) (*int, error) {
	return p.count(it, "peers", p.RSSParser.Peers)
}

func (p *ezrssParser) count(it *indexer.Item, name string, def func(*indexer.Item) (*int, error)) (*int, error) {
	n, err := singleDescendant(it, name)
	if err != nil {
		return nil, err
	}
	if n == nil {
		return def(it)
	}
	v, err := strconv.Atoi(strings.TrimSpace(n.Value()))
	if err != nil {
		return nil, fmt.Errorf("%s %q: %w", name, n.Value(), err)
	}
	return &v, nil
}

func singleDescendant(it *indexer.Item, name string) (*indexer.Node, error) {
	ns := it.Descendants(name)
	switch len(ns) {
	case 0:
		return nil, nil
	case 1:
		return ns[0], nil
	}
	return nil, fmt.Errorf("%d %s elements, want one", len(ns), name)
}

// isEZTVFeed reports whether a feed is an EZRSS one, by its namespaces or
// DTD.
func isEZTVFeed(doc *indexer.Document) bool {
	if doc.Root.NamespaceOfPrefix("torrent") == ezrssNamespace {
		return true
	}
	for _, d := range doc.Directives {
		if strings.HasPrefix(d, "DOCTYPE") && strings.Contains(d, `"`+ezrssNamespace+`dtd/"`) {
			return true
		}
	}
	// An element whose default namespace is EZRSS's has one declaring it.
	for _, n := range append([]*indexer.Node{doc.Root}, doc.Root.Descendants("")...) {
		if v, ok := n.Attr("xmlns"); ok && v == ezrssNamespace {
			return true
		}
	}
	return false
}
