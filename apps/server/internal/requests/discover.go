package requests

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"connectrpc.com/connect"
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

// DiscoverPage is a page of a catalog or of search results. Pages are
// the provider's: TotalPages is exact, TotalResults the provider's total
// less the titles left out of this page, Hidden, since the titles left
// out of other pages are not known without reading them; Seerr leaves
// none out, and so does not tell.
type DiscoverPage struct {
	Titles                         []DiscoveredTitle
	Page, TotalPages, TotalResults int
	// Hidden counts the titles of this page left out: adult, above the
	// user's rating limit, or unrated when the user's policy blocks them.
	Hidden int
	// PluginID is the discovery provider that searched.
	PluginID string
}

// Discover lists a page of a catalog, marked with what the server has of
// each title, as Seerr's /discover routes. The user's rating limit
// applies: catalogs filtering by certification are asked for the
// certifications of the server's country within it when the caller
// filters by certification or the user's policy blocks unrated titles,
// since asking for certifications leaves the unrated titles out; of other
// pages, the titles the metadata providers rate above the limit are left
// out, as are unrated ones when the policy blocks them. Adult titles are
// never listed.
func (s *Service) Discover(ctx context.Context, caller core.User, catalogID string, req *pluginv1.DiscoverRequest) (DiscoverPage, error) {
	p, c, err := s.catalog(ctx, catalogID)
	if err != nil {
		return DiscoverPage{}, err
	}
	country, _ := s.region()
	req.SetCatalog(c.GetId())
	req.SetIncludeAdult(false)
	restricted := caller.EffectiveAccess().MaxParentalRating != nil
	filtered := false
	if slices.Contains(c.GetFilters(), pluginv1.DiscoverFilter_DISCOVER_FILTER_CERTIFICATION) {
		certs := req.GetCertifications()
		if restricted {
			allowed := Certifications(&caller, country)
			switch {
			case len(certs) > 0:
				certs = slices.DeleteFunc(slices.Clone(certs), func(c string) bool { return !slices.Contains(allowed, c) })
				if len(certs) == 0 {
					return DiscoverPage{Page: max(1, int(req.GetPage())), PluginID: p.ID}, nil
				}
				filtered = true
			case caller.EffectiveAccess().BlockUnrated:
				certs, filtered = allowed, true
			}
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
	return s.page(ctx, &caller, p.ID, res.GetTitles(), int(res.GetPage()), int(res.GetTotalPages()), int(res.GetTotalResults()), restricted && !filtered)
}

// Search finds titles by name through a discovery provider, as Seerr's
// GET /search (server/routes/search.ts): the one named, else the first
// that searches. Titles are marked and left out as Discover's.
func (s *Service) Search(ctx context.Context, caller core.User, pluginID string, req *pluginv1.SearchTitlesRequest) (DiscoverPage, error) {
	req.SetIncludeAdult(false)
	for _, p := range s.discoveryProviders() {
		if pluginID != "" && p.ID != pluginID {
			continue
		}
		res, err := p.Client.SearchTitles(ctx, req)
		switch {
		case connect.CodeOf(err) == connect.CodeUnimplemented:
			continue
		case err != nil:
			return DiscoverPage{}, fmt.Errorf("search %s: %w", p.ID, err)
		}
		return s.page(ctx, &caller, p.ID, res.GetTitles(), int(res.GetPage()), int(res.GetTotalPages()), int(res.GetTotalResults()),
			caller.EffectiveAccess().MaxParentalRating != nil)
	}
	return DiscoverPage{}, fmt.Errorf("discovery provider %q searching titles: %w", pluginID, core.ErrNotFound)
}

// page marks a provider's page of titles, leaving out adult titles and,
// when rate, those the user's rating limit does not allow.
func (s *Service) page(ctx context.Context, u *core.User, pluginID string, titles []*pluginv1.DiscoveredTitle, page, pages, total int, rate bool) (DiscoverPage, error) {
	out := DiscoverPage{Page: page, TotalPages: pages, PluginID: pluginID}
	for _, t := range titles {
		if t.GetAdult() {
			out.Hidden++
			continue
		}
		d := DiscoveredTitle{Title: t, Kind: core.KindMovie}
		if t.GetMediaKind() == pluginv1.MediaKind_MEDIA_KIND_SERIES {
			d.Kind = core.KindSeries
		}
		d.TmdbID, _ = strconv.Atoi(t.GetExternalIds()[string(core.ProviderTMDB)])
		out.Titles = append(out.Titles, d)
	}
	if rate {
		n := len(out.Titles)
		var err error
		if out.Titles, err = s.rated(ctx, u, out.Titles); err != nil {
			return DiscoverPage{}, err
		}
		out.Hidden += n - len(out.Titles)
	}
	out.TotalResults = max(0, total-out.Hidden)
	if err := s.mark(ctx, u, out.Titles); err != nil {
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
			rating, country := "", ""
			if d.TmdbID > 0 {
				t, err := s.Title(gctx, d.Kind, d.TmdbID)
				switch {
				case gctx.Err() != nil:
					return gctx.Err()
				case err == nil:
					rating, country = t.Rating, t.Country
				}
			}
			if country == "" {
				country, _ = s.region()
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
	for i := range titles {
		d := &titles[i]
		if d.TmdbID <= 0 {
			continue
		}
		x := indexes[d.Kind]
		if x == nil {
			idx, err := s.index(ctx, d.Kind)
			if err != nil {
				return err
			}
			x = &idx
			indexes[d.Kind] = x
		}
		tvdb, _ := strconv.Atoi(d.Title.GetExternalIds()[string(core.ProviderTVDB)])
		st, err := s.titleState(ctx, u, x, d.Kind, d.TmdbID, tvdb)
		if err != nil {
			return err
		}
		d.Status, d.ItemID, d.Requestable = st.status, st.itemID, st.requestable
		if len(st.active) > 0 {
			d.RequestID, d.RequestStatus = st.active[0].ID, st.active[0].Status
		}
	}
	return nil
}

// titleState is what the server has of a title.
type titleState struct {
	status MediaStatus
	itemID core.ID
	// all are the title's requests, active those pending, approved or
	// failed, newest first.
	all, active []core.Request
	// series is what the libraries have of a series.
	series seriesState
	// blocked is a title on the blocklist.
	blocked     bool
	requestable bool
}

// titleState finds what the server has of a title, as Seerr's media
// statuses: what the libraries have of it, requested when they have
// nothing and a request is active, deleted when they have nothing and a
// request was completed, blocklisted whatever they have when it is on the
// blocklist.
func (s *Service) titleState(ctx context.Context, u *core.User, x *mediaIndex, kind core.ItemKind, tmdbID, tvdbID int) (titleState, error) {
	var st titleState
	// The title's requests, of any status, know the TVDB ID of series
	// whose items may lack their TMDB ID.
	all, err := s.Store.Requests().List(ctx, core.RequestQuery{Kind: kind, TmdbID: tmdbID})
	if err != nil {
		return titleState{}, err
	}
	st.all = all.Items
	completed := false
	for _, r := range all.Items {
		if tvdbID == 0 {
			tvdbID = r.TvdbID
		}
		if r.Status.Active() {
			st.active = append(st.active, r)
		}
		completed = completed || r.Status == core.RequestCompleted
	}
	m := x.find(tmdbID, tvdbID)
	if kind == core.KindMovie {
		if st.status, st.itemID, err = s.movieStatus(ctx, m); err != nil {
			return titleState{}, err
		}
	} else {
		if st.series, err = s.seriesState(ctx, m); err != nil {
			return titleState{}, err
		}
		st.status, st.itemID = st.series.status(), st.series.itemID
	}
	switch {
	case st.status != StatusNone:
	case len(st.active) > 0:
		st.status = StatusRequested
	case completed:
		st.status = StatusDeleted
	}
	if st.blocked, err = s.isBlocked(ctx, kind, tmdbID); err != nil {
		return titleState{}, err
	}
	if st.blocked {
		st.status = StatusBlocklisted
	}
	st.requestable = mayRequest(u, kind) && (st.status == StatusNone || st.status == StatusDeleted ||
		kind == core.KindSeries && st.status != StatusAvailable && st.status != StatusBlocklisted)
	return st, nil
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
