package indexer

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/dlclark/regexp2"
)

// The enclosure types of NZBs, torrents and magnet links.
const (
	NZBEnclosureType     = "application/x-nzb"
	TorrentEnclosureType = "application/x-bittorrent"
	MagnetEnclosureType  = "application/x-bittorrent;x-scheme-handler/magnet"
)

var (
	// UsenetEnclosureTypes are the enclosure types of Usenet releases.
	UsenetEnclosureTypes = []string{NZBEnclosureType}
	// TorrentEnclosureTypes are the enclosure types of torrents.
	TorrentEnclosureTypes = []string{TorrentEnclosureType, MagnetEnclosureType}
)

// RSSParser reads releases from RSS feeds, as Prowlarr's RssParser and,
// with Torrent set, TorrentRssParser do. Newznab and Torznab override what
// they read differently through ItemReader.
type RSSParser struct {
	// UseGUIDInfoURL takes the guid element as the info URL.
	UseGUIDInfoURL bool
	// UseEnclosureURL takes the enclosure as the download URL.
	UseEnclosureURL bool
	// UseEnclosureLength takes the enclosure's length as the size.
	UseEnclosureLength bool
	// ParseSizeInDescription reads "1.3 GB" in the description as the
	// size.
	ParseSizeInDescription bool
	// PreferredEnclosureTypes are the enclosure types taken, in order;
	// any single enclosure when empty.
	PreferredEnclosureTypes []string

	// Torrent reads torrents' hashes, magnet links, seeders and peers.
	Torrent bool
	// CalculatePeersAsSum counts the seeders among the peers.
	CalculatePeersAsSum bool
	// ParseSeedersInDescription reads "Seeders: 3" and the like in the
	// description.
	ParseSeedersInDescription bool
	// The elements holding the info hash, peers, seeders, size and magnet
	// link, found at any depth; none when "".
	InfoHashElement, PeersElement, SeedsElement, SizeElement, MagnetElement string
	// DefaultReleaseSize, in MiB, is the size of releases without one.
	DefaultReleaseSize float64
	// DefaultReleaseSeeders are the seeders and peers of releases without
	// them.
	DefaultReleaseSeeders *int
}

// NewTorrentRSSParser returns a parser of torrent feeds, preferring
// torrent enclosures.
func NewTorrentRSSParser() *RSSParser {
	return &RSSParser{Torrent: true, PreferredEnclosureTypes: TorrentEnclosureTypes}
}

// Item is an item of a feed.
type Item struct {
	*Node
	// The URL of the feed, which relative URLs are resolved against.
	Base string
}

// ItemReader reads the fields of items; RSSParser reads them as RSS has
// them, and feeds with more embed it and replace some.
type ItemReader interface {
	GUID(it *Item) string
	Title(it *Item) string
	Description(it *Item) string
	PublishDate(it *Item) (time.Time, error)
	DownloadURL(it *Item) string
	InfoURL(it *Item) string
	CommentsURL(it *Item) string
	Categories(it *Item) []int
	Languages(it *Item) []string
	Size(it *Item) (int64, error)
	InfoHash(it *Item) string
	MagnetURL(it *Item) string
	Seeders(it *Item) (*int, error)
	Peers(it *Item) (*int, error)
}

// CheckResponse fails for statuses but 200 and for HTML answering a
// request that did not accept it, as RssParser.PreProcess does.
func CheckResponse(r *Response) error {
	if r.StatusCode != http.StatusOK {
		return Errorf("indexer API call resulted in an unexpected status code [%d]", r.StatusCode)
	}
	if r.HasContentType("text/html") && !strings.Contains(r.Request.Header.Get("Accept"), "text/html") {
		return Errorf("indexer responded with html content; the site is likely blocked or unavailable")
	}
	return nil
}

// Parse reads a feed's releases.
func (p *RSSParser) Parse(r *Response) ([]*Release, error) {
	if err := CheckResponse(r); err != nil {
		return nil, err
	}
	doc, err := LoadXML(r.Content())
	if err != nil {
		return nil, err
	}
	return p.ParseItems(r, Items(doc), p)
}

// ParseItems reads releases from items with a reader. Items that fail are
// skipped, but for unsupported feeds.
func (p *RSSParser) ParseItems(r *Response, items []*Node, reader ItemReader) ([]*Release, error) {
	var releases []*Release
	for _, n := range items {
		rel, err := p.ReadItem(&Item{Node: n, Base: r.URL}, reader)
		if errors.Is(err, ErrUnsupportedFeed) {
			return nil, err
		}
		if err == nil && rel != nil {
			releases = append(releases, rel)
		}
	}
	return releases, nil
}

// ReadItem reads a release from an item with a reader.
func (p *RSSParser) ReadItem(it *Item, reader ItemReader) (*Release, error) {
	rel := &Release{
		GUID:        reader.GUID(it),
		Title:       reader.Title(it),
		Description: reader.Description(it),
		DownloadURL: reader.DownloadURL(it),
		InfoURL:     reader.InfoURL(it),
		CommentsURL: reader.CommentsURL(it),
		Categories:  reader.Categories(it),
		Languages:   reader.Languages(it),
	}
	var err error
	if rel.PublishDate, err = reader.PublishDate(it); err != nil {
		return nil, err
	}
	size, err := reader.Size(it)
	if err != nil {
		return nil, fmt.Errorf("unable to parse size from %s: %w", rel.Title, err)
	}
	rel.Size = &size
	if !p.Torrent {
		return rel, nil
	}
	rel.Protocol = ProtocolTorrent
	rel.InfoHash = reader.InfoHash(it)
	rel.MagnetURL = reader.MagnetURL(it)
	if rel.Seeders, err = reader.Seeders(it); err != nil {
		return nil, err
	}
	if rel.Peers, err = reader.Peers(it); err != nil {
		return nil, err
	}
	if p.CalculatePeersAsSum {
		if rel.Peers != nil && rel.Seeders != nil {
			rel.Peers = Ptr(*rel.Peers + *rel.Seeders)
		} else {
			rel.Peers = nil
		}
	}
	return rel, nil
}

// Items returns the items of a feed's channel.
func Items(doc *Document) []*Node {
	return doc.Root.Element("channel").Elements("item")
}

// GUID returns the guid element, a random one when missing.
func (p *RSSParser) GUID(it *Item) string {
	if g := it.Element("guid"); g != nil {
		return g.Value()
	}
	return newGUID()
}

func newGUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// Title returns the title element, "Unknown" when missing.
func (p *RSSParser) Title(it *Item) string { return it.ChildValue("title", "Unknown") }

// Description returns the description element.
func (p *RSSParser) Description(it *Item) string { return it.ChildValue("description", "") }

// PublishDate returns the pubDate element, which feeds must have.
func (p *RSSParser) PublishDate(it *Item) (time.Time, error) {
	s := it.ChildValue("pubDate", "")
	if strings.TrimSpace(s) == "" {
		return time.Time{}, fmt.Errorf("%w: each item in the RSS feed must have a pubDate element with a valid publish date", ErrUnsupportedFeed)
	}
	return ParseDate(s)
}

// DownloadURL returns the enclosure's URL or the link element.
func (p *RSSParser) DownloadURL(it *Item) string {
	if p.UseEnclosureURL {
		if e := p.Enclosure(Enclosures(it.Node), true); e != nil {
			return it.URL(e.URL)
		}
		return ""
	}
	return it.URL(it.ChildValue("link", ""))
}

// InfoURL returns the guid element when it is the info URL.
func (p *RSSParser) InfoURL(it *Item) string {
	if p.UseGUIDInfoURL {
		return it.URL(it.ChildValue("guid", ""))
	}
	return ""
}

// CommentsURL returns the comments element.
func (p *RSSParser) CommentsURL(it *Item) string { return it.URL(it.ChildValue("comments", "")) }

// Categories returns Other.
func (p *RSSParser) Categories(*Item) []int { return []int{Other.ID} }

// Languages returns none.
func (p *RSSParser) Languages(*Item) []string { return nil }

// Size returns the enclosure's length, the size in the description or of
// the size element, or the default.
func (p *RSSParser) Size(it *Item) (int64, error) {
	var size int64
	switch {
	case p.UseEnclosureLength:
		if e := p.Enclosure(Enclosures(it.Node), true); e != nil {
			size = e.Length
		}
	case p.ParseSizeInDescription && it.Element("description") != nil:
		size = ParseSize(it.Element("description").Value(), true)
	}
	if !p.Torrent {
		return size, nil
	}
	if size == 0 && p.SizeElement != "" {
		if n, err := single(it.Descendants(p.SizeElement)); err != nil {
			return 0, err
		} else if n != nil {
			size = ParseSize(n.Value(), true)
		}
	}
	if size == 0 && p.DefaultReleaseSize > 0 {
		return int64(p.DefaultReleaseSize * 1024 * 1024), nil
	}
	return size, nil
}

// InfoHash returns the info hash element, or the magnet link's hash.
func (p *RSSParser) InfoHash(it *Item) string {
	if p.InfoHashElement != "" {
		if ns := it.Descendants(p.InfoHashElement); len(ns) > 0 {
			return ns[0].Value()
		}
		return ""
	}
	if m := p.MagnetURL(it); m != "" {
		return MagnetInfoHash(m)
	}
	return ""
}

// MagnetURL returns the magnet element, or the download URL when it is a
// magnet link.
func (p *RSSParser) MagnetURL(it *Item) string {
	var m string
	if p.MagnetElement != "" {
		if ns := it.Descendants(p.MagnetElement); len(ns) > 0 {
			m = ns[0].Value()
		}
	} else {
		m = p.DownloadURL(it)
	}
	if strings.HasPrefix(m, "magnet:") {
		return m
	}
	return ""
}

var (
	seedersRegex  = regexp2.MustCompile(`(Seeder)s?:\s+(?<value>\d+)|(?<value>\d+)\s+(seeder)s?`, regexp2.IgnoreCase)
	leechersRegex = regexp2.MustCompile(`(Leecher)s?:\s+(?<value>\d+)|(?<value>\d+)\s+(leecher)s?`, regexp2.IgnoreCase)
	peersRegex    = regexp2.MustCompile(`(Peer)s?:\s+(?<value>\d+)|(?<value>\d+)\s+(peer)s?`, regexp2.IgnoreCase)
)

func matchInt(re *regexp2.Regexp, s string) (int, bool) {
	m, err := re.FindStringMatch(s)
	if err != nil || m == nil {
		return 0, false
	}
	n, err := strconv.Atoi(m.GroupByName("value").String())
	return n, err == nil
}

// Seeders returns the seeders in the description or the seeds element.
func (p *RSSParser) Seeders(it *Item) (*int, error) {
	if desc := it.Element("description"); p.ParseSeedersInDescription && desc != nil {
		v := desc.Value()
		if s, ok := matchInt(seedersRegex, v); ok {
			return &s, nil
		}
		peers, okP := matchInt(peersRegex, v)
		leechers, okL := matchInt(leechersRegex, v)
		if okP && okL {
			return Ptr(peers - leechers), nil
		}
	}
	if p.SeedsElement != "" {
		n, err := single(it.Descendants(p.SeedsElement))
		if err != nil {
			return nil, err
		}
		if n != nil {
			v, err := strconv.Atoi(strings.TrimSpace(n.Value()))
			if err != nil {
				return nil, fmt.Errorf("seeders %q: %w", n.Value(), err)
			}
			return &v, nil
		}
	}
	return p.DefaultReleaseSeeders, nil
}

// Peers returns the peers in the description or the peers element.
func (p *RSSParser) Peers(it *Item) (*int, error) {
	if desc := it.Element("description"); p.ParseSeedersInDescription && desc != nil {
		v := desc.Value()
		if n, ok := matchInt(peersRegex, v); ok {
			return &n, nil
		}
		seeders, okS := matchInt(seedersRegex, v)
		leechers, okL := matchInt(leechersRegex, v)
		if okS && okL {
			return Ptr(seeders + leechers), nil
		}
	}
	if p.PeersElement != "" {
		n, err := single(it.Descendants(p.PeersElement))
		if err != nil {
			return nil, err
		}
		if n == nil {
			return nil, fmt.Errorf("no %s element", p.PeersElement)
		}
		v, err := strconv.Atoi(strings.TrimSpace(n.Value()))
		if err != nil {
			return nil, fmt.Errorf("peers %q: %w", n.Value(), err)
		}
		return &v, nil
	}
	return p.DefaultReleaseSeeders, nil
}

// single returns the only node, nil for none, failing for more, as
// SingleOrDefault does.
func single(ns []*Node) (*Node, error) {
	switch len(ns) {
	case 0:
		return nil, nil
	case 1:
		return ns[0], nil
	}
	return nil, fmt.Errorf("%d %s elements, want one", len(ns), ns[0].Name.Local)
}

// Enclosure is an enclosure of an item.
type Enclosure struct {
	URL, Type string
	Length    int64
}

// Enclosures returns an item's enclosures; those whose length does not
// parse are skipped.
func Enclosures(item *Node) []Enclosure {
	var out []Enclosure
	for _, e := range item.Elements("enclosure") {
		enc := Enclosure{}
		enc.URL, _ = e.Attr("url")
		enc.Type, _ = e.Attr("type")
		if l, ok := e.Attr("length"); ok && l != "" {
			n, err := strconv.ParseInt(strings.TrimSpace(l), 10, 64)
			if err != nil {
				continue
			}
			enc.Length = n
		}
		out = append(out, enc)
	}
	return out
}

// Enclosure returns the enclosure of the first preferred type or, with
// none preferred or when not enforcing them, the only enclosure.
func (p *RSSParser) Enclosure(encs []Enclosure, enforceType bool) *Enclosure {
	if len(encs) == 0 {
		return nil
	}
	if len(p.PreferredEnclosureTypes) > 0 {
		for _, t := range p.PreferredEnclosureTypes {
			for i := range encs {
				if encs[i].Type == t {
					return &encs[i]
				}
			}
		}
		if enforceType {
			return nil
		}
	}
	if len(encs) == 1 {
		return &encs[0]
	}
	return nil
}

// URL resolves a URL of the item against the feed's; "" for none.
func (it *Item) URL(v string) string {
	if strings.TrimSpace(v) == "" {
		return ""
	}
	return ResolveURL(it.Base, v)
}

var sizeRegex = regexp2.MustCompile(`(?<value>(?<!\.\d*)(?:\d+,)*\d+(?:\.\d{1,3})?)\W?(?<unit>[KMG]i?B)(?![\w/])`, regexp2.IgnoreCase)

// ParseSize reads a size such as "1.3 GB", with decimal or, when binary,
// binary prefixes; KiB, MiB and GiB are always binary. 0 when there is
// none.
func ParseSize(s string, binary bool) int64 {
	if strings.TrimSpace(s) == "" {
		return 0
	}
	if allDigits(s) {
		n, _ := strconv.ParseInt(s, 10, 64)
		return n
	}
	m, err := sizeRegex.FindStringMatch(s)
	if err != nil || m == nil {
		return 0
	}
	v, err := strconv.ParseFloat(strings.ReplaceAll(m.GroupByName("value").String(), ",", ""), 64)
	if err != nil {
		return 0
	}
	unit := strings.ToLower(m.GroupByName("unit").String())
	power := map[byte]float64{'k': 1, 'm': 2, 'g': 3}[unit[0]]
	base := 1000.0
	if binary || strings.Contains(unit, "i") {
		base = 1024
	}
	return int64(math.RoundToEven(v * math.Pow(base, power)))
}

func allDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

var trailingZone = regexp.MustCompile(`\s[A-Z]{2,4}$`)

// dateLayouts are the layouts ParseDate tries, those of RSS and the
// common ones .NET's DateTime.Parse reads.
var dateLayouts = []string{
	time.RFC1123Z,
	time.RFC1123,
	"Mon, 2 Jan 2006 15:04:05 -0700",
	"Mon, 2 Jan 2006 15:04:05 MST",
	"2 Jan 2006 15:04:05 -0700",
	time.RFC3339Nano,
	"2006-01-02T15:04:05.999999999-0700",
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05 -0700",
	"2006-01-02 15:04:05",
	"Mon, 2 Jan 2006 15:04:05",
	"Mon, 02 Jan 2006 15:04:05",
	"2 Jan 2006 15:04:05",
	"01/02/2006 15:04:05",
	"2006-01-02",
}

// ParseDate reads an item's date, in UTC when it has no zone; unknown
// zone names are dropped, as Prowlarr's ParseDate does.
func ParseDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	for _, try := range []string{s, trailingZone.ReplaceAllString(s, "")} {
		for _, layout := range dateLayouts {
			if t, err := time.ParseInLocation(layout, try, time.UTC); err == nil {
				return t.UTC(), nil
			}
		}
	}
	return time.Time{}, fmt.Errorf("unable to parse date %q", s)
}

// MagnetInfoHash returns the info hash of a magnet link in upper case hex,
// as MonoTorrent writes it; "" for none.
func MagnetInfoHash(link string) string {
	for xt := range strings.SplitSeq(link[strings.IndexByte(link, '?')+1:], "&") {
		v, ok := strings.CutPrefix(xt, "xt=urn:btih:")
		if !ok {
			continue
		}
		switch len(v) {
		case 40:
			return strings.ToUpper(v)
		case 32:
			return base32ToHex(v)
		}
	}
	return ""
}

func base32ToHex(v string) string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"
	var bits, n uint64
	var out strings.Builder
	for _, r := range strings.ToUpper(v) {
		i := strings.IndexRune(alphabet, r)
		if i < 0 {
			return ""
		}
		bits = bits<<5 | uint64(i)
		n += 5
		for n >= 4 {
			n -= 4
			out.WriteByte("0123456789ABCDEF"[bits>>n&0xf])
		}
	}
	return out.String()
}
