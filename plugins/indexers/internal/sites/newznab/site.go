package newznab

import (
	"context"
	"errors"
	"slices"

	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
)

// Site is a Newznab indexer.
type Site struct {
	settings Settings
	caps     CapsProvider
}

// New returns a Newznab indexer, failing for invalid settings.
func New(s Settings) (*Site, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	return &Site{settings: s}, nil
}

// Info implements indexer.Site; pages are as long as the capabilities
// say, 100 releases until they are read.
func (s *Site) Info() indexer.Info {
	return Info(indexer.ProtocolUsenet, &s.caps)
}

// Info returns the info of a Newznab or Torznab indexer.
func Info(protocol indexer.Protocol, caps *CapsProvider) indexer.Info {
	info := indexer.Info{
		Protocol:           protocol,
		Privacy:            indexer.Private,
		SupportsRSS:        true,
		SupportsSearch:     true,
		SupportsPagination: true,
		PageSize:           100,
	}
	if c := caps.Cached(); c != nil && c.LimitsDefault > 0 {
		info.PageSize = c.LimitsDefault
	}
	return info
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
	return SearchRequests(s.settings, caps, q), nil
}

// ParseResponse implements indexer.Site.
func (s *Site) ParseResponse(ctx context.Context, ix *indexer.Indexer, r *indexer.Response) ([]*indexer.Release, error) {
	caps, err := s.caps.Get(ctx, ix, s.settings)
	if err != nil {
		return nil, err
	}
	return NewParser(caps).Parse(ctx, r)
}

// TestSite implements indexer.SiteTester, checking that the indexer can
// be searched.
func (s *Site) TestSite(ctx context.Context, ix *indexer.Indexer) error {
	caps, err := s.caps.Get(ctx, ix, s.settings)
	if err != nil {
		return err
	}
	return CheckSearchable(caps)
}

// CheckSearchable fails for indexers that take none of the searches
// Prowlarr makes, as its TestCapabilities does.
func CheckSearchable(caps *indexer.Capabilities) error {
	takes := func(t indexer.SearchType, ps ...indexer.Param) bool {
		return slices.ContainsFunc(ps, func(p indexer.Param) bool { return caps.Has(t, p) })
	}
	switch {
	case caps.Has(indexer.SearchBasic, indexer.ParamQ),
		takes(indexer.SearchMovie, indexer.ParamQ, indexer.ParamIMDbID, indexer.ParamTMDBID, indexer.ParamTraktID),
		takes(indexer.SearchTV, indexer.ParamQ, indexer.ParamTVDBID, indexer.ParamIMDbID, indexer.ParamTMDBID, indexer.ParamRID) &&
			caps.Has(indexer.SearchTV, indexer.ParamSeason) && caps.Has(indexer.SearchTV, indexer.ParamEp),
		takes(indexer.SearchMusic, indexer.ParamQ, indexer.ParamArtist, indexer.ParamAlbum),
		takes(indexer.SearchBook, indexer.ParamQ, indexer.ParamAuthor, indexer.ParamTitle):
		return nil
	}
	return errors.New("this indexer does not support searching for tv, music, or movies; ask its staff to enable it")
}
