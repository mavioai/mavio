package cardigann

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
)

// Site is an indexer site a definition describes.
type Site struct {
	e    *engine
	caps *indexer.Capabilities
}

// New returns the site of a definition with the settings, failing for
// definitions it cannot read.
func New(def *Definition, s Settings, log *slog.Logger) (*Site, error) {
	e, err := newEngine(def, s, nil, log)
	if err != nil {
		return nil, err
	}
	caps, err := capabilities(def, e)
	if err != nil {
		return nil, fmt.Errorf("definition %s: %w", def.ID, err)
	}
	return &Site{e: e, caps: caps}, nil
}

// capabilities returns a definition's capabilities, as Prowlarr's
// ParseCardigannSearchModes reads its modes.
func capabilities(def *Definition, e *engine) (*indexer.Capabilities, error) {
	caps := &indexer.Capabilities{
		LimitsDefault: 100, LimitsMax: 100,
		Params:     map[indexer.SearchType][]indexer.Param{},
		RawSearch:  def.Caps.AllowRawSearch,
		Categories: e.categories,
	}
	modes := def.Caps.Modes
	if len(modes) == 0 {
		return nil, fmt.Errorf("at least one search mode is required")
	}
	if _, ok := modes.Get("search"); !ok {
		return nil, fmt.Errorf("the search mode 'search' is mandatory")
	}
	types := map[string]indexer.SearchType{
		"tv-search":    indexer.SearchTV,
		"movie-search": indexer.SearchMovie,
		"music-search": indexer.SearchMusic,
		"book-search":  indexer.SearchBook,
	}
	for _, m := range modes {
		if m.Key == "search" {
			if len(m.Value) != 1 || m.Value[0] != "q" {
				return nil, fmt.Errorf("in search mode 'search' only 'q' parameter is supported and it's mandatory")
			}
			if err := caps.AddParams(indexer.SearchBasic, m.Value); err != nil {
				return nil, err
			}
			continue
		}
		t, ok := types[m.Key]
		if !ok {
			return nil, fmt.Errorf("unsupported search mode: %s", m.Key)
		}
		if err := caps.AddParams(t, m.Value); err != nil {
			return nil, err
		}
	}
	return caps, nil
}

// Info implements indexer.Site. Each search path is a page, so that all
// of them are searched.
func (s *Site) Info() indexer.Info {
	privacy := indexer.SemiPrivate
	switch s.e.def.Type {
	case "private":
		privacy = indexer.Private
	case "public":
		privacy = indexer.Public
	}
	var rate time.Duration
	if d := s.e.def.RequestDelay; d != nil && *d > 2 {
		rate = time.Duration(*d * float64(time.Second))
	}
	return indexer.Info{
		Protocol:           indexer.ProtocolTorrent,
		Privacy:            privacy,
		SupportsRSS:        true,
		SupportsSearch:     true,
		SupportsPagination: true,
		PageSize:           1,
		RateLimit:          rate,
		Encoding:           s.e.encoding,
	}
}

// Capabilities implements indexer.Site.
func (s *Site) Capabilities(context.Context, *indexer.Indexer) (*indexer.Capabilities, error) {
	return s.caps, nil
}

// SearchRequests implements indexer.Site.
func (s *Site) SearchRequests(_ context.Context, ix *indexer.Indexer, q *indexer.Query) (*indexer.Chain, error) {
	requests, err := s.e.searchRequests(q)
	if err != nil {
		return nil, err
	}
	if c := s.e.settingsCookie(); c != nil {
		for _, r := range requests {
			r.Cookies = c
		}
	}
	var chain indexer.Chain
	chain.AddRequests(requests...)
	return &chain, nil
}

// ParseResponse implements indexer.Site.
func (s *Site) ParseResponse(_ context.Context, _ *indexer.Indexer, r *indexer.Response) ([]*indexer.Release, error) {
	return s.e.parseResponse(r)
}

// CleanupReleases implements indexer.ReleaseCleaner: definitions with
// the andmatch rows filter keep the releases matching the query.
func (s *Site) CleanupReleases(releases []*indexer.Release, q *indexer.Query) []*indexer.Release {
	if s.e.matchesQuery() {
		return indexer.FilterByQuery(releases, q)
	}
	return releases
}

// LoginNeeded implements indexer.LoginSite.
func (s *Site) LoginNeeded(r *indexer.Response) bool {
	return s.e.loginNeeded(r)
}

// Login implements indexer.LoginSite.
func (s *Site) Login(ctx context.Context, ix *indexer.Indexer) error {
	return s.e.login(ctx, ix)
}

// DownloadRequest implements indexer.DownloadSite.
func (s *Site) DownloadRequest(ctx context.Context, ix *indexer.Indexer, link string) (*indexer.Request, error) {
	return s.e.downloadRequest(ctx, ix, link)
}
