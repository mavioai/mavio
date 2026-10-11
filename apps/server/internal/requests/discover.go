package requests

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/sync/errgroup"

	"github.com/mavioai/mavio/libs/core"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1/pluginv1connect"
)

// DiscoveryProvider is a discovery provider plugin.
type DiscoveryProvider struct {
	ID     string
	Client pluginv1connect.DiscoveryProviderServiceClient
}

// Catalog is a catalog of a discovery provider.
type Catalog struct {
	// ID is "<plugin>/<catalog>".
	ID       string
	PluginID string
	Catalog  *pluginv1.Catalog
}

// lookupConcurrency bounds the titles looked up at once to rate them.
const lookupConcurrency = 8

func (s *Service) discoveryProviders() []DiscoveryProvider {
	if s.Discovery == nil {
		return nil
	}
	return s.Discovery()
}

// Catalogs lists the catalogs of every discovery provider, as Seerr's
// discover sliders; a provider failing is left out.
func (s *Service) Catalogs(ctx context.Context, language string) ([]Catalog, error) {
	var out []Catalog
	for _, p := range s.discoveryProviders() {
		res, err := p.Client.ListCatalogs(ctx, pluginv1.ListCatalogsRequest_builder{Language: &language}.Build())
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			s.logger().WarnContext(ctx, "couldn't list catalogs", "plugin", p.ID, "err", err)
			continue
		}
		for _, c := range res.GetCatalogs() {
			out = append(out, Catalog{ID: p.ID + "/" + c.GetId(), PluginID: p.ID, Catalog: c})
		}
	}
	return out, nil
}

// catalog finds a catalog by its merged ID.
func (s *Service) catalog(ctx context.Context, id string) (DiscoveryProvider, *pluginv1.Catalog, error) {
	plugin, local, ok := strings.Cut(id, "/")
	if !ok || plugin == "" || local == "" {
		return DiscoveryProvider{}, nil, fmt.Errorf("%w: catalog %q is not <plugin>/<catalog>", core.ErrInvalid, id)
	}
	i := slices.IndexFunc(s.discoveryProviders(), func(p DiscoveryProvider) bool { return p.ID == plugin })
	if i < 0 {
		return DiscoveryProvider{}, nil, fmt.Errorf("discovery provider %s: %w", plugin, core.ErrNotFound)
	}
	p := s.discoveryProviders()[i]
	res, err := p.Client.ListCatalogs(ctx, pluginv1.ListCatalogsRequest_builder{}.Build())
	if err != nil {
		return DiscoveryProvider{}, nil, fmt.Errorf("list catalogs of %s: %w", plugin, err)
	}
	for _, c := range res.GetCatalogs() {
		if c.GetId() == local {
			return p, c, nil
		}
	}
	return DiscoveryProvider{}, nil, fmt.Errorf("catalog %s: %w", id, core.ErrNotFound)
}

// DiscoveredTitle is a title of a catalog, marked with what the server
// has of it.
type DiscoveredTitle struct {
	Title *pluginv1.DiscoveredTitle
	// Kind is core.KindMovie or core.KindSeries; TmdbID 0 when the
	// provider gives none.
	Kind   core.ItemKind
	TmdbID int
	Status MediaStatus
	// ItemID is its movie or series in a library, if any.
	ItemID core.ID
	// Request is its latest active request, if any.
	RequestID     core.ID
	RequestStatus core.RequestStatus
	// Requestable is whether the user may request it now.
	Requestable bool
}

// DiscoverPage is a page of a catalog.
type DiscoverPage struct {
	Titles                         []DiscoveredTitle
	Page, TotalPages, TotalResults int
}

// Discover lists a page of a catalog, marked with what the server has of
// each title, as Seerr's /discover routes. The user's rating limit
// applies: catalogs filtering by certification list only the
// certifications of the requests' country within it; of other catalogs,
// the titles the metadata providers rate above it are left out, as are
// unrated ones when the user's policy blocks them. Adult titles are never
// listed.
func (s *Service) Discover(ctx context.Context, caller core.User, catalogID string, req *pluginv1.DiscoverRequest) (DiscoverPage, error) {
	p, c, err := s.catalog(ctx, catalogID)
	if err != nil {
		return DiscoverPage{}, err
	}
	kind := core.KindMovie
	if req.GetMediaKind() == pluginv1.MediaKind_MEDIA_KIND_SERIES {
		kind = core.KindSeries
	}
	country, _ := s.region(ctx, kind)
	req.SetCatalog(c.GetId())
	req.SetIncludeAdult(false)
	restricted := caller.Policy.MaxParentalRating != nil
	filtered := false
	if slices.Contains(c.GetFilters(), pluginv1.DiscoverFilter_DISCOVER_FILTER_CERTIFICATION) {
		certs := req.GetCertifications()
		if restricted {
			allowed := Certifications(&caller, country)
			if len(certs) > 0 {
				certs = slices.DeleteFunc(slices.Clone(certs), func(c string) bool { return !slices.Contains(allowed, c) })
				if len(certs) == 0 {
					return DiscoverPage{Page: max(1, int(req.GetPage()))}, nil
				}
			} else {
				certs = allowed
			}
			filtered = true
		}
		if len(certs) > 0 {
			req.SetCertificationCountry(country)
			req.SetCertifications(certs)
		}
	} else {
		req.ClearCertificationCountry()
		req.SetCertifications(nil)
	}
	res, err := p.Client.Discover(ctx, req)
	if err != nil {
		return DiscoverPage{}, fmt.Errorf("discover %s: %w", catalogID, err)
	}
	out := DiscoverPage{Page: int(res.GetPage()), TotalPages: int(res.GetTotalPages()), TotalResults: int(res.GetTotalResults())}
	for _, t := range res.GetTitles() {
		d := DiscoveredTitle{Title: t, Kind: core.KindMovie}
		if t.GetMediaKind() == pluginv1.MediaKind_MEDIA_KIND_SERIES {
			d.Kind = core.KindSeries
		}
		d.TmdbID, _ = strconv.Atoi(t.GetExternalIds()[string(core.ProviderTMDB)])
		if t.GetAdult() {
			continue
		}
		out.Titles = append(out.Titles, d)
	}
	if restricted && !filtered {
		if out.Titles, err = s.rated(ctx, &caller, out.Titles); err != nil {
			return DiscoverPage{}, err
		}
	}
	if err := s.mark(ctx, &caller, out.Titles); err != nil {
		return DiscoverPage{}, err
	}
	return out, nil
}

// rated leaves out the titles a user's rating limit does not allow, as
// the metadata providers rate them; titles they do not know are unrated.
func (s *Service) rated(ctx context.Context, u *core.User, titles []DiscoveredTitle) ([]DiscoveredTitle, error) {
	keep := make([]bool, len(titles))
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(lookupConcurrency)
	for i, d := range titles {
		g.Go(func() error {
			rating, country := "", "US"
			if d.TmdbID > 0 {
				t, err := s.Title(gctx, d.Kind, d.TmdbID)
				switch {
				case gctx.Err() != nil:
					return gctx.Err()
				case err == nil:
					rating, country = t.Rating, t.Country
				default:
					country, _ = s.region(gctx, d.Kind)
				}
			}
			keep[i] = RatingAllowed(u, rating, country)
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	out := titles[:0]
	for i, d := range titles {
		if keep[i] {
			out = append(out, d)
		}
	}
	return out, nil
}

// mark marks titles with what the libraries have of them and their
// requests.
func (s *Service) mark(ctx context.Context, u *core.User, titles []DiscoveredTitle) error {
	indexes := map[core.ItemKind]*mediaIndex{}
	index := func(kind core.ItemKind) (*mediaIndex, error) {
		if x := indexes[kind]; x != nil {
			return x, nil
		}
		x, err := s.index(ctx, kind)
		if err != nil {
			return nil, err
		}
		indexes[kind] = &x
		return &x, nil
	}
	for i := range titles {
		d := &titles[i]
		if d.TmdbID <= 0 {
			continue
		}
		x, err := index(d.Kind)
		if err != nil {
			return err
		}
		// The title's requests, of any status, know the TVDB ID of series
		// whose items may lack their TMDB ID.
		all, err := s.Store.Requests().List(ctx, core.RequestQuery{Kind: d.Kind, TmdbID: d.TmdbID})
		if err != nil {
			return err
		}
		tvdb, _ := strconv.Atoi(d.Title.GetExternalIds()[string(core.ProviderTVDB)])
		var active []core.Request
		for _, r := range all.Items {
			if tvdb == 0 {
				tvdb = r.TvdbID
			}
			if r.Status.Active() {
				active = append(active, r)
			}
		}
		m := x.find(d.TmdbID, tvdb)
		if d.Kind == core.KindMovie {
			if d.Status, d.ItemID, err = s.movieStatus(ctx, m); err != nil {
				return err
			}
		} else {
			st, err := s.seriesState(ctx, m)
			if err != nil {
				return err
			}
			d.Status, d.ItemID = st.status(), st.itemID
		}
		if len(active) > 0 {
			d.RequestID, d.RequestStatus = active[0].ID, active[0].Status
			if d.Status == StatusNone {
				d.Status = StatusRequested
			}
		}
		d.Requestable = mayRequest(u, d.Kind) && (d.Status == StatusNone || d.Kind == core.KindSeries && d.Status != StatusAvailable)
	}
	return nil
}

// FilterValues lists the values a catalog's filter takes.
func (s *Service) FilterValues(ctx context.Context, catalogID string, req *pluginv1.ListFilterValuesRequest) ([]*pluginv1.FilterValue, error) {
	p, _, err := s.catalog(ctx, catalogID)
	if err != nil {
		return nil, err
	}
	res, err := p.Client.ListFilterValues(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("list filter values of %s: %w", catalogID, err)
	}
	return res.GetValues(), nil
}
