package headphones

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
	"github.com/mavioai/mavio/plugins/indexers/internal/sites/newznab"
)

var links = []string{"https://indexer.codeshy.com"}

const (
	// apiPath is the path of the Newznab API under the base URL.
	apiPath = "/api"
	// apiKey is the API key all of Headphones' users share; they are told
	// apart by their credentials.
	apiKey = "964d601959918a578a670984bdee9357"
)

// Definition is Headphones VIP.
var Definition = indexer.Definition{
	ID:          "headphones",
	Name:        "Headphones VIP",
	Description: "A Private Usenet indexer for music",
	Protocol:    indexer.ProtocolUsenet,
	Privacy:     indexer.Private,
	Links:       links,
	Fields: []indexer.Field{
		indexer.BaseURLField(links),
		{Name: "username", Label: "Username", Type: indexer.FieldText, HelpText: "Site Username"},
		{Name: "password", Label: "Password", Type: indexer.FieldText, HelpText: "Site Password", Secret: true},
	},
	New: newSite,
}

// Site is Headphones VIP.
type Site struct {
	settings indexer.UserPassSettings
	caps     *indexer.Capabilities
}

func newSite(data []byte) (indexer.Site, error) {
	var settings indexer.UserPassSettings
	if err := indexer.DecodeSettings(data, &settings); err != nil {
		return nil, err
	}
	if err := settings.Validate(); err != nil {
		return nil, err
	}
	c := indexer.NewCapabilities()
	c.Params[indexer.SearchMusic] = []indexer.Param{indexer.ParamQ}
	c.Categories.Add("3000", indexer.Audio, "")
	c.Categories.Add("3010", indexer.AudioMP3, "")
	c.Categories.Add("3040", indexer.AudioLossless, "")
	return &Site{settings: settings, caps: c}, nil
}

// Info implements indexer.Site.
func (s *Site) Info() indexer.Info {
	return indexer.Info{
		Protocol:           indexer.ProtocolUsenet,
		Privacy:            indexer.Private,
		SupportsRSS:        true,
		SupportsSearch:     true,
		SupportsPagination: true,
	}
}

// Capabilities implements indexer.Site.
func (s *Site) Capabilities(context.Context, *indexer.Indexer) (*indexer.Capabilities, error) {
	return s.caps, nil
}

// SearchRequests implements indexer.Site, as Prowlarr's
// HeadphonesRequestGenerator does: music and basic searches by term
// only, through the API's basic search; the categories are sent as the
// query gives them.
func (s *Site) SearchRequests(_ context.Context, _ *indexer.Indexer, q *indexer.Query) (*indexer.Chain, error) {
	var c indexer.Chain
	if q.Type != indexer.SearchBasic && q.Type != indexer.SearchMusic {
		return &c, nil
	}
	var p indexer.Params
	// Music searches take no artist nor album, so they fall back to a
	// basic search as well.
	if strings.TrimSpace(q.Term) != "" && s.caps.Available(indexer.SearchBasic) {
		p.Set("q", strings.ReplaceAll(q.Term, "+", "%20"))
	}
	u := strings.TrimRight(s.settings.URL(links, nil), "/") + strings.TrimRight(apiPath, "/") + "?t=search&extended=1"
	var cats []string
	for _, id := range q.Categories {
		if v := strconv.Itoa(id); !slices.Contains(cats, v) {
			cats = append(cats, v)
		}
	}
	if len(cats) > 0 {
		u += "&cat=" + strings.Join(cats, ",")
	}
	u += "&apikey=" + apiKey
	if q.Limit > 0 {
		p.Set("limit", strconv.Itoa(q.Limit))
	}
	p.Set("offset", strconv.Itoa(q.Offset))
	r := indexer.NewRequest(u+"&"+p.Encode(), indexer.AcceptRSS)
	s.authorize(r)
	c.AddRequests(r)
	return &c, nil
}

// authorize sends the credentials with a request, as basic
// authentication.
func (s *Site) authorize(r *indexer.Request) {
	auth := s.settings.Username + ":" + s.settings.Password
	r.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(auth)))
}

// DownloadRequest implements indexer.DownloadSite: NZBs are downloaded
// with the credentials.
func (s *Site) DownloadRequest(_ context.Context, _ *indexer.Indexer, link string) (*indexer.Request, error) {
	r := indexer.NewRequest(link, "")
	s.authorize(r)
	return r, nil
}

// ParseResponse implements indexer.Site, as Prowlarr's
// HeadphonesRssParser does.
func (s *Site) ParseResponse(ctx context.Context, _ *indexer.Indexer, r *indexer.Response) ([]*indexer.Release, error) {
	doc, err := newznab.LoadResponse(r)
	if err != nil {
		return nil, err
	}
	if err := checkError(doc, r); err != nil {
		return nil, err
	}
	p := &parser{
		RSSParser: &indexer.RSSParser{UseEnclosureURL: true, PreferredEnclosureTypes: indexer.UsenetEnclosureTypes},
		caps:      s.caps,
	}
	items := indexer.Items(doc)
	var releases []*indexer.Release
	for _, n := range items {
		it := &indexer.Item{Node: n, Base: r.URL}
		rel, err := p.ReadItem(it, p)
		if errors.Is(err, indexer.ErrUnsupportedFeed) {
			return nil, err
		}
		// Items whose download URL is relative and that have no enclosure
		// fail in Prowlarr.
		if err != nil || !newznab.IsAbsoluteURL(rel.DownloadURL) && it.Element("enclosure") == nil {
			continue
		}
		rel.IMDbID = intAttr(it, "imdb")
		rel.Grabs = indexer.Ptr(intAttr(it, "grabs"))
		rel.Files = indexer.Ptr(intAttr(it, "files"))
		releases = append(releases, rel)
	}
	if !newznab.CheckEnclosures(ctx, r, items, indexer.UsenetEnclosureTypes, indexer.TorrentEnclosureTypes, "Torznab") {
		return nil, nil
	}
	return releases, nil
}

// checkError returns the error a response reports, if any, as
// HeadphonesRssParser.CheckError does.
func checkError(doc *indexer.Document, r *indexer.Response) error {
	errs := doc.Root.Descendants("error")
	if doc.Root != nil && strings.EqualFold(doc.Root.Name.Local, "error") {
		errs = append([]*indexer.Node{doc.Root}, errs...)
	}
	if len(errs) == 0 {
		return nil
	}
	codeValue, _ := errs[0].Attr("code")
	msg, _ := errs[0].Attr("description")
	code, err := strconv.Atoi(strings.TrimSpace(codeValue))
	if err != nil {
		return fmt.Errorf("headphones: error code %q: %w", codeValue, err)
	}
	if code >= 100 && code <= 199 {
		return fmt.Errorf("%w: %s", indexer.ErrAuth, msg)
	}
	if !strings.Contains(r.Request.URL, "apikey=") && (msg == "Missing parameter" || strings.Contains(msg, "apikey")) {
		return fmt.Errorf("%w: Indexer requires an API key", indexer.ErrAuth)
	}
	if msg == "Request limit reached" {
		return &indexer.RateLimitError{Message: "API limit reached"}
	}
	return indexer.Errorf("%s", msg)
}

// intAttr returns a Newznab attribute's number, 0 when it has none.
func intAttr(it *indexer.Item, key string) int {
	n, err := strconv.Atoi(newznab.Attr(it, newznab.AttrNamespace, key))
	if err != nil {
		return 0
	}
	return n
}

// parser reads the items of Headphones' feeds as Newznab items, but for
// their categories, given by attributes only.
type parser struct {
	*indexer.RSSParser
	caps *indexer.Capabilities
}

// Categories implements indexer.ItemReader: the category attributes that
// are numbers, mapped.
func (p *parser) Categories(it *indexer.Item) []int {
	var out []int
	for _, c := range newznab.Attrs(it, newznab.AttrNamespace, "category") {
		n, err := strconv.Atoi(c)
		if err != nil {
			continue
		}
		out = append(out, indexer.IDs(p.caps.Categories.MapTracker(strconv.Itoa(n)))...)
	}
	return out
}

// Languages implements indexer.ItemReader: the language attributes, or
// else elements, that are not blank.
func (p *parser) Languages(it *indexer.Item) []string {
	return newznab.Languages(it, newznab.AttrNamespace)
}

// InfoURL implements indexer.ItemReader: the comments URL without its
// fragment.
func (p *parser) InfoURL(it *indexer.Item) string { return newznab.InfoURL(it) }

// Size implements indexer.ItemReader: the size attribute or the
// enclosure's length.
func (p *parser) Size(it *indexer.Item) (int64, error) {
	return newznab.Size(p.RSSParser, it, newznab.AttrNamespace), nil
}

// PublishDate implements indexer.ItemReader: the usenetdate attribute or
// pubDate.
func (p *parser) PublishDate(it *indexer.Item) (time.Time, error) {
	if d := newznab.Attr(it, newznab.AttrNamespace, "usenetdate"); strings.TrimSpace(d) != "" {
		return indexer.ParseDate(d)
	}
	return p.RSSParser.PublishDate(it)
}

// DownloadURL implements indexer.ItemReader: the NZB enclosure's URL, else
// the first enclosure's.
func (p *parser) DownloadURL(it *indexer.Item) string {
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
