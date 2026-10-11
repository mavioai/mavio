package torrentrss

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
)

// Settings are a Torrent RSS feed's settings.
type Settings struct {
	BaseURL string `json:"base_url"`
	// Cookie is sent with requests, as a Cookie header.
	Cookie string `json:"cookie,omitempty"`
	// AllowZeroSize accepts feeds without the sizes of releases.
	AllowZeroSize bool `json:"allow_zero_size,omitempty"`
	// DefaultReleaseSize, in MiB, is the size of releases without one.
	DefaultReleaseSize float64 `json:"default_release_size,omitempty"`
}

// ParserSettings are how a feed is read, as detected.
type ParserSettings struct {
	UseEZTVFormat             bool
	ParseSeedersInDescription bool
	UseEnclosureURL           bool
	UseEnclosureLength        bool
	ParseSizeInDescription    bool
	SizeElementName           string
}

// detectionLife is how long detected parser settings are kept.
const detectionLife = 7 * 24 * time.Hour

// Site is a Torrent RSS feed.
type Site struct {
	settings Settings
	// detect detects how to read a response; Detect when nil.
	detect func(r *indexer.Response, s Settings) (*ParserSettings, error)

	mu       sync.Mutex
	parser   *ParserSettings
	detected time.Time
}

// New returns a feed.
func New(s Settings) (*Site, error) {
	if !strings.HasPrefix(s.BaseURL, "http://") && !strings.HasPrefix(s.BaseURL, "https://") {
		return nil, fmt.Errorf("feed URL %q is not an HTTP URL", s.BaseURL)
	}
	return &Site{settings: s}, nil
}

// Info implements indexer.Site.
func (s *Site) Info() indexer.Info {
	return indexer.Info{Protocol: indexer.ProtocolTorrent, Privacy: indexer.Public, SupportsRSS: true}
}

// Capabilities implements indexer.Site.
func (s *Site) Capabilities(context.Context, *indexer.Indexer) (*indexer.Capabilities, error) {
	c := indexer.NewCapabilities()
	for _, t := range []indexer.SearchType{indexer.SearchTV, indexer.SearchMovie, indexer.SearchMusic, indexer.SearchBook} {
		if err := c.AddParams(t, []string{"q"}); err != nil {
			return nil, err
		}
	}
	c.Categories.Add("1", indexer.Other, "")
	return c, nil
}

// SearchRequests implements indexer.Site: the feed, for queries of recent
// releases only.
func (s *Site) SearchRequests(_ context.Context, _ *indexer.Indexer, q *indexer.Query) (*indexer.Chain, error) {
	var c indexer.Chain
	if q.IsRSS() {
		r := indexer.NewRequest(strings.TrimRight(strings.TrimSpace(s.settings.BaseURL), "/"), indexer.AcceptRSS)
		// Prowlarr's IndexerRequest of a URL follows redirects.
		r.FollowRedirects = true
		r.Cookies = ParseCookies(s.settings.Cookie)
		c.AddRequests(r)
	}
	return &c, nil
}

// ParseResponse implements indexer.Site, detecting how to read the feed
// on the first response.
func (s *Site) ParseResponse(_ context.Context, ix *indexer.Indexer, r *indexer.Response) ([]*indexer.Release, error) {
	ps, err := s.parserSettings(ix, r)
	if err != nil {
		return nil, err
	}
	return s.Parser(ps).Parse(r)
}

func (s *Site) parserSettings(ix *indexer.Indexer, r *indexer.Response) (*ParserSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if ix != nil && ix.Now != nil {
		now = ix.Now()
	}
	if s.parser != nil && now.Sub(s.detected) < detectionLife {
		return s.parser, nil
	}
	detect := s.detect
	if detect == nil {
		detect = Detect
	}
	ps, err := detect(r, s.settings)
	if err != nil {
		return nil, err
	}
	if ps == nil {
		return nil, fmt.Errorf("%w: could not parse feed from %s", indexer.ErrUnsupportedFeed, s.settings.BaseURL)
	}
	s.parser, s.detected = ps, now
	return ps, nil
}

// parser reads a feed.
type parser interface {
	Parse(r *indexer.Response) ([]*indexer.Release, error)
}

// Parser returns the parser of feeds read with the settings, as
// Prowlarr's TorrentRssParserFactory does.
func (s *Site) Parser(ps *ParserSettings) parser {
	if ps.UseEZTVFormat {
		return newEzrssParser()
	}
	p := indexer.NewTorrentRSSParser()
	p.ParseSeedersInDescription = ps.ParseSeedersInDescription
	p.UseEnclosureURL = ps.UseEnclosureURL
	p.UseEnclosureLength = ps.UseEnclosureLength
	p.ParseSizeInDescription = ps.ParseSizeInDescription
	p.SizeElement = ps.SizeElementName
	p.DefaultReleaseSize = s.settings.DefaultReleaseSize
	p.DefaultReleaseSeeders = indexer.Ptr(1)
	return p
}

// ParseCookies parses a Cookie header's cookies.
func ParseCookies(header string) map[string]string {
	cookies := map[string]string{}
	for part := range strings.SplitSeq(header, ";") {
		k, v, ok := strings.Cut(strings.TrimSpace(part), "=")
		if ok && k != "" {
			cookies[k] = v
		}
	}
	return cookies
}

// validSizeThreshold is the size below which releases are taken not to
// have their content's size.
const validSizeThreshold = 2 * 1024 * 1024

// Detect detects how to read a feed from a response of it, as Prowlarr's
// TorrentRssSettingsDetector does; nil when it cannot.
func Detect(r *indexer.Response, s Settings) (*ParserSettings, error) {
	doc, err := indexer.LoadXML(r.Content())
	if err != nil {
		return nil, fmt.Errorf("%w: %w", indexer.ErrUnsupportedFeed, err)
	}
	if isEZTVFeed(doc) {
		releases, err := parse(newEzrssParser(), r)
		if err == nil {
			err = errors.Join(validateReleases(releases), validateSizes(releases, s))
		}
		if err == nil {
			return &ParserSettings{UseEZTVFormat: true}, nil
		}
	}
	return detectGeneric(doc, r, s)
}

func detectGeneric(doc *indexer.Document, r *indexer.Response, s Settings) (*ParserSettings, error) {
	p := indexer.NewTorrentRSSParser()
	p.UseEnclosureURL, p.UseEnclosureLength, p.ParseSeedersInDescription = true, true, true
	items := indexer.Items(doc)
	if len(items) == 0 {
		return nil, fmt.Errorf("%w: empty feed, cannot check if feed is parsable", indexer.ErrUnsupportedFeed)
	}
	ps := &ParserSettings{UseEnclosureURL: true, UseEnclosureLength: true, ParseSeedersInDescription: true}
	if items[0].Element("enclosure") == nil {
		p.UseEnclosureURL, ps.UseEnclosureURL = false, false
	}
	releases, err := parseValid(p, r)
	if err != nil {
		return nil, err
	}
	if !anySeeders(releases) {
		p.ParseSeedersInDescription, ps.ParseSeedersInDescription = false, false
	}
	if !anySmall(releases) {
		return ps, nil
	}
	p.UseEnclosureLength, ps.UseEnclosureLength = false, false
	for _, name := range []string{"size", "Size"} {
		p.SizeElement, ps.SizeElementName = name, name
		if releases, err = parseValid(p, r); err != nil {
			return nil, err
		}
		if !anySmall(releases) {
			return ps, nil
		}
	}
	p.SizeElement, ps.SizeElementName = "", ""
	p.ParseSizeInDescription, ps.ParseSizeInDescription = true, true
	if releases, err = parseValid(p, r); err != nil {
		return nil, err
	}
	valid := 0
	for _, rel := range releases {
		if *rel.Size >= validSizeThreshold {
			valid++
		}
	}
	if valid > len(releases)/2 {
		return ps, nil
	}
	p.ParseSizeInDescription, ps.ParseSizeInDescription = false, false
	if releases, err = parseValid(p, r); err != nil {
		return nil, err
	}
	if err := validateSizes(releases, s); err != nil {
		return nil, err
	}
	return ps, nil
}

func parse(p parser, r *indexer.Response) ([]*indexer.Release, error) {
	releases, err := p.Parse(r)
	if err != nil {
		if errors.Is(err, indexer.ErrUnsupportedFeed) {
			return nil, err
		}
		return nil, fmt.Errorf("%w: unable to parse indexer: %w", indexer.ErrUnsupportedFeed, err)
	}
	return releases, nil
}

func parseValid(p parser, r *indexer.Response) ([]*indexer.Release, error) {
	releases, err := parse(p, r)
	if err != nil {
		return nil, err
	}
	return releases, validateReleases(releases)
}

func anySeeders(releases []*indexer.Release) bool {
	for _, r := range releases {
		if r.Seeders != nil {
			return true
		}
	}
	return false
}

func anySmall(releases []*indexer.Release) bool {
	for _, r := range releases {
		if *r.Size < validSizeThreshold {
			return true
		}
	}
	return false
}

func validateReleases(releases []*indexer.Release) error {
	if len(releases) == 0 {
		return fmt.Errorf("%w: empty feed, cannot check if feed is parsable", indexer.ErrUnsupportedFeed)
	}
	guids := map[string]bool{}
	for _, r := range releases {
		if strings.TrimSpace(r.Title) == "" {
			return fmt.Errorf("%w: feed contains releases without title", indexer.ErrUnsupportedFeed)
		}
		if !validDownloadURL(r.DownloadURL) {
			return fmt.Errorf("%w: failed to find a valid download url in the feed", indexer.ErrUnsupportedFeed)
		}
		if guids[r.GUID] {
			return fmt.Errorf("%w: feed contains releases with same guid, rejecting malformed rss feed", indexer.ErrUnsupportedFeed)
		}
		guids[r.GUID] = true
	}
	return nil
}

func validateSizes(releases []*indexer.Release, s Settings) error {
	for _, r := range releases {
		if *r.Size == 0 && !s.AllowZeroSize {
			return fmt.Errorf("%w: feed doesn't contain the release content size", indexer.ErrUnsupportedFeed)
		}
	}
	for _, r := range releases {
		if *r.Size != 0 && *r.Size < validSizeThreshold {
			return fmt.Errorf("%w: size of one or more releases lower than 2 MB, feed must contain release content size", indexer.ErrUnsupportedFeed)
		}
	}
	return nil
}

func validDownloadURL(u string) bool {
	return strings.HasPrefix(u, "magnet:") || strings.HasPrefix(u, "http:") || strings.HasPrefix(u, "https:")
}
