package home

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"log/slog"
	"slices"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/mavioai/mavio/apps/server/internal/browse"
	"github.com/mavioai/mavio/apps/server/internal/requests"
	"github.com/mavioai/mavio/libs/core"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

// The display preferences holding a user's choices of Home: Pinned lists
// the IDs of the libraries shown, in order, and Hidden the IDs of the hubs
// hidden, both comma separated.
const (
	Client = "mavio"
	View   = "home"
	Pinned = "libraries"
	Hidden = "hidden"
)

const (
	// DefaultLimit is the number of items listed of a hub by default.
	DefaultLimit = 20
	// seedWindow is how far back the plays seeding hubs go.
	seedWindow = 90 * 24 * time.Hour
	// continueWindow is how far back continue watching goes.
	continueWindow = 365 * 24 * time.Hour
	// Rediscover lists items last played longer ago than rediscoverAge,
	// series longer ago than rediscoverSeriesAge.
	rediscoverAge       = 365 * 24 * time.Hour
	rediscoverSeriesAge = 182 * 24 * time.Hour
	// requestWindow is how long a request that became available is listed.
	requestWindow = 30 * 24 * time.Hour
	// people is how many of a seed's actors or directors are drawn from.
	people = 5
	// concurrency bounds the hubs filled at once.
	concurrency = 4
)

// Hub is a row of items.
type Hub struct {
	// ID is the kind of a hub spanning the libraries, otherwise
	// "<kind>/<library ID>".
	ID   string
	Kind core.HubKind
	// Library is nil for hubs spanning the libraries.
	Library *core.Library
	// SeedID and SeedName are what a seeded hub is drawn from: an item, a
	// person, or a genre (SeedName only).
	SeedID   core.ID
	SeedName string
	Items    []core.Item
	// Titles are trending's discovered titles.
	Titles []requests.DiscoveredTitle
	// More reports whether the hub has items after these.
	More bool
}

// Service builds hubs.
type Service struct {
	store core.Store
	// requests lists trending titles; nil lists none.
	requests *requests.Service
	browser  *browse.Browser
	// Now returns the current time; nil uses time.Now.
	Now func() time.Time
	// Logger reports the failures of discovery; nil uses slog.Default.
	Logger *slog.Logger
}

// New returns a Service over store; reqs lists trending titles and may be
// nil.
func New(store core.Store, reqs *requests.Service) *Service {
	return &Service{store: store, requests: reqs, browser: browse.New(store)}
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Service) logger() *slog.Logger {
	if s.Logger != nil {
		return s.Logger
	}
	return slog.Default()
}

// spec names a hub.
type spec struct {
	kind core.HubKind
	lib  *core.Library
}

func (sp spec) id() string {
	if sp.lib == nil {
		return string(sp.kind)
	}
	return string(sp.kind) + "/" + sp.lib.ID.String()
}

// Home lists the hubs of the user's Home with their first limit items:
// continue watching and the requests that became available, the Home
// hubs of each pinned library, then trending titles. Hidden hubs and
// empty ones are left out; continue watching cannot be hidden.
func (s *Service) Home(ctx context.Context, u *core.User, limit int) ([]Hub, error) {
	prefs, err := s.store.DisplayPreferences().Get(ctx, u.ID, Client, View)
	if err != nil && !errors.Is(err, core.ErrNotFound) {
		return nil, fmt.Errorf("get home preferences: %w", err)
	}
	libs, err := s.libraries(ctx, u, prefs.Values)
	if err != nil {
		return nil, err
	}
	specs := []spec{{kind: core.HubContinueWatching}, {kind: core.HubAvailableRequests}}
	for _, lib := range libs {
		home, _ := lib.Hubs.Resolve(lib.Kind)
		for _, k := range home {
			specs = append(specs, spec{k, lib})
		}
	}
	specs = append(specs, spec{kind: core.HubTrending})
	hidden := split(prefs.Values[Hidden])
	specs = slices.DeleteFunc(specs, func(sp spec) bool {
		return sp.kind != core.HubContinueWatching && slices.Contains(hidden, sp.id())
	})
	return s.fillAll(ctx, u, specs, limit)
}

// Recommended lists the hubs of a library's Recommended tab with their
// first limit items, leaving out empty ones. It returns core.ErrNotFound
// for a library the user may not access.
func (s *Service) Recommended(ctx context.Context, u *core.User, libraryID core.ID, limit int) ([]Hub, error) {
	lib, err := s.library(ctx, u, libraryID)
	if err != nil {
		return nil, err
	}
	_, recommended := lib.Hubs.Resolve(lib.Kind)
	specs := make([]spec, len(recommended))
	for i, k := range recommended {
		specs[i] = spec{k, lib}
	}
	return s.fillAll(ctx, u, specs, limit)
}

// Get lists a page of a hub's items. It returns core.ErrNotFound for an
// unknown hub or one of a library the user may not access.
func (s *Service) Get(ctx context.Context, u *core.User, id string, limit, offset int) (Hub, error) {
	sp, err := s.parse(ctx, u, id)
	if err != nil {
		return Hub{}, err
	}
	return s.fill(ctx, u, sp, limit, offset)
}

// parse reads a hub's ID.
func (s *Service) parse(ctx context.Context, u *core.User, id string) (spec, error) {
	notFound := fmt.Errorf("%w: hub %q", core.ErrNotFound, id)
	kind, libID, scoped := strings.Cut(id, "/")
	sp := spec{kind: core.HubKind(kind)}
	switch {
	case !sp.kind.Valid():
		return spec{}, notFound
	case !scoped:
		if !sp.kind.Global() && sp.kind != core.HubContinueWatching {
			return spec{}, notFound
		}
		return sp, nil
	case sp.kind.Global():
		return spec{}, notFound
	}
	lid, err := core.ParseID(libID)
	if err != nil {
		return spec{}, notFound
	}
	if sp.lib, err = s.library(ctx, u, lid); err != nil {
		return spec{}, err
	}
	if !slices.Contains(sp.lib.Kind.HubKinds(), sp.kind) {
		return spec{}, notFound
	}
	return sp, nil
}

// library returns a library the user may access.
func (s *Service) library(ctx context.Context, u *core.User, id core.ID) (*core.Library, error) {
	if !u.Policy.CanAccessLibrary(id) {
		return nil, fmt.Errorf("%w: library %s", core.ErrNotFound, id)
	}
	lib, err := s.store.Libraries().Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get library: %w", err)
	}
	return &lib, nil
}

// libraries returns the libraries pinned to the user's Home: those named
// by the preferences in their order, or every library they may access in
// name order.
func (s *Service) libraries(ctx context.Context, u *core.User, prefs map[string]string) ([]*core.Library, error) {
	all, err := s.store.Libraries().List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list libraries: %w", err)
	}
	all = slices.DeleteFunc(all, func(l core.Library) bool { return !u.Policy.CanAccessLibrary(l.ID) })
	var out []*core.Library
	if pinned, ok := prefs[Pinned]; ok {
		for _, id := range split(pinned) {
			if i := slices.IndexFunc(all, func(l core.Library) bool { return l.ID.String() == id }); i >= 0 {
				out = append(out, &all[i])
			}
		}
		return out, nil
	}
	slices.SortStableFunc(all, func(a, b core.Library) int { return cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)) })
	for i := range all {
		out = append(out, &all[i])
	}
	return out, nil
}

// fillAll fills hubs concurrently, leaving out empty ones.
func (s *Service) fillAll(ctx context.Context, u *core.User, specs []spec, limit int) ([]Hub, error) {
	hubs := make([]Hub, len(specs))
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(concurrency)
	for i, sp := range specs {
		g.Go(func() error {
			h, err := s.fill(gctx, u, sp, limit, 0)
			hubs[i] = h
			return err
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	return slices.DeleteFunc(hubs, func(h Hub) bool { return len(h.Items) == 0 && len(h.Titles) == 0 }), nil
}

// fill lists a page of a hub's items.
func (s *Service) fill(ctx context.Context, u *core.User, sp spec, limit, offset int) (Hub, error) {
	limit = cmp.Or(limit, DefaultLimit)
	h := Hub{ID: sp.id(), Kind: sp.kind, Library: sp.lib}
	n := limit + 1 // one more tells whether there are more
	now := s.now()
	scope := browse.Scope{
		UserID: u.ID, LibraryIDs: u.Policy.Libraries,
		MaxRating: u.Access().MaxParentalRating, SkipUnrated: u.Access().BlockUnrated,
	}
	var kinds []core.ItemKind
	if sp.lib != nil {
		scope.LibraryIDs, kinds = []core.ID{sp.lib.ID}, titleKinds(sp.lib.Kind)
	}
	q := core.ItemQuery{
		LibraryIDs: scope.LibraryIDs, MaxRating: scope.MaxRating, SkipUnrated: scope.SkipUnrated, UserID: u.ID,
		Kinds: kinds, Limit: n, Offset: offset,
	}
	var items []core.Item
	var err error
	switch sp.kind {
	case core.HubContinueWatching:
		items, err = s.browser.ContinueWatching(ctx, scope, browse.ContinueQuery{Since: now.Add(-continueWindow), Limit: n, Offset: offset})
	case core.HubRecentlyAdded:
		items, err = s.browser.Latest(ctx, scope, browse.LatestQuery{Group: true, Limit: offset + n})
		items = items[min(offset, len(items)):]
	case core.HubRecentlyReleased:
		q.Sort = []core.SortSpec{{Field: core.SortPremiereDate, Desc: true}}
		items, err = s.query(ctx, q)
	case core.HubRecentlyAired:
		q.Kinds = []core.ItemKind{core.KindEpisode}
		q.Sort = []core.SortSpec{{Field: core.SortPremiereDate, Desc: true}}
		items, err = s.query(ctx, q)
	case core.HubTopRated, core.HubTopUnplayed:
		if sp.kind == core.HubTopUnplayed {
			q.Played = new(false)
		}
		q.Sort = []core.SortSpec{{Field: core.SortCommunityRating, Desc: true}}
		items, err = s.query(ctx, q)
	case core.HubStartWatching:
		q.Kinds, q.Started = []core.ItemKind{core.KindSeries}, new(false)
		q.Sort = []core.SortSpec{{Field: core.SortDateAdded, Desc: true}}
		items, err = s.query(ctx, q)
	case core.HubRediscover:
		age := rediscoverAge
		if sp.lib.Kind == core.LibraryShows {
			age = rediscoverSeriesAge
		}
		q.PlayedBefore = now.Add(-age)
		q.Sort = []core.SortSpec{{Field: core.SortCommunityRating, Desc: true}}
		items, err = s.query(ctx, q)
	case core.HubBecauseYouWatched, core.HubBecauseYouLike, core.HubMoreInGenre, core.HubStarring, core.HubDirectedBy:
		items, err = s.seeded(ctx, u, sp, &h, q)
	case core.HubAvailableRequests:
		items, err = s.availableRequests(ctx, u, now, n, offset)
	case core.HubTrending:
		titles := s.trending(ctx, u)
		h.Titles = titles[min(offset, len(titles)):]
		h.Titles, h.More = h.Titles[:min(limit, len(h.Titles))], len(h.Titles) > limit
		return h, nil
	}
	if err != nil {
		return Hub{}, fmt.Errorf("fill hub %s: %w", h.ID, err)
	}
	h.Items, h.More = items[:min(limit, len(items))], len(items) > limit
	return h, nil
}

func (s *Service) query(ctx context.Context, q core.ItemQuery) ([]core.Item, error) {
	page, err := s.store.Items().Query(ctx, q)
	if err != nil {
		return nil, err
	}
	return page.Items, nil
}

// titleKinds are the kinds of the titles a library of kind k lists in its
// hubs.
func titleKinds(k core.LibraryKind) []core.ItemKind {
	switch k {
	case core.LibraryMovies:
		return []core.ItemKind{core.KindMovie}
	case core.LibraryShows:
		return []core.ItemKind{core.KindSeries}
	case core.LibraryMusicVideos:
		return []core.ItemKind{core.KindMusicVideo}
	case core.LibraryHomeVideos:
		return []core.ItemKind{core.KindVideo}
	case core.LibraryMixed:
		return []core.ItemKind{core.KindMovie, core.KindSeries, core.KindVideo}
	case core.LibraryMusic:
		return []core.ItemKind{core.KindMusicAlbum}
	case core.LibraryBooks:
		return []core.ItemKind{core.KindBook, core.KindAudioBook}
	case core.LibraryPhotos:
		return []core.ItemKind{core.KindPhotoAlbum}
	}
	return nil
}

// seeded lists the items of a seeded hub, drawing its seed for the day:
// an item the user played in the last 90 days, or one they marked as a
// favorite, and of that item a genre, actor or director.
func (s *Service) seeded(ctx context.Context, u *core.User, sp spec, h *Hub, q core.ItemQuery) ([]core.Item, error) {
	cq := q
	cq.Limit, cq.Offset = core.MaxPageSize, 0
	cq.Sort = []core.SortSpec{{Field: core.SortName}}
	if sp.kind == core.HubBecauseYouLike {
		cq.Favorite = new(true)
	} else {
		cq.PlayedAfter = s.now().Add(-seedWindow)
	}
	candidates, err := s.query(ctx, cq)
	if err != nil || len(candidates) == 0 {
		return nil, err
	}
	pick := s.picker(u.ID, sp)
	seed := candidates[pick(len(candidates))]
	q.Played = new(false)
	switch sp.kind {
	case core.HubBecauseYouWatched, core.HubBecauseYouLike:
		h.SeedID, h.SeedName = seed.ID, seed.Name
		q.SimilarTo, q.Kinds = seed.ID, []core.ItemKind{seed.Kind}
	case core.HubMoreInGenre:
		if len(seed.Genres) == 0 {
			return nil, nil
		}
		h.SeedName = seed.Genres[pick(len(seed.Genres))]
		q.Genres = []string{h.SeedName}
		q.Sort = []core.SortSpec{{Field: core.SortCommunityRating, Desc: true}}
	case core.HubStarring, core.HubDirectedBy:
		role := core.CreditActor
		if sp.kind == core.HubDirectedBy {
			role = core.CreditDirector
		}
		credits, err := s.store.People().CreditsForItem(ctx, seed.ID)
		if err != nil {
			return nil, err
		}
		credits = slices.DeleteFunc(credits, func(c core.Credit) bool { return c.Kind != role })
		slices.SortStableFunc(credits, func(a, b core.Credit) int { return cmp.Compare(a.Order, b.Order) })
		if len(credits) == 0 {
			return nil, nil
		}
		person, err := s.store.People().Get(ctx, credits[pick(min(people, len(credits)))].PersonID)
		if err != nil {
			return nil, err
		}
		h.SeedID, h.SeedName = person.ID, person.Name
		q.PersonID = person.ID
		q.Sort = []core.SortSpec{{Field: core.SortCommunityRating, Desc: true}}
	}
	return s.query(ctx, q)
}

// picker returns a function choosing one of n things, the same all day for
// a user's hub and different from day to day.
func (s *Service) picker(user core.ID, sp spec) func(n int) int {
	day := s.now().UTC().Format(time.DateOnly)
	return func(n int) int {
		f := fnv.New64a()
		fmt.Fprintf(f, "%s|%s|%s|%d", user, day, sp.id(), n)
		return int(f.Sum64() % uint64(n))
	}
}

// availableRequests lists the items of the user's requests that became
// available in the last 30 days, most recent first.
func (s *Service) availableRequests(ctx context.Context, u *core.User, now time.Time, limit, offset int) ([]core.Item, error) {
	page, err := s.store.Requests().List(ctx, core.RequestQuery{
		RequestedBy: u.ID, Statuses: []core.RequestStatus{core.RequestCompleted}, SortByModified: true,
	})
	if err != nil {
		return nil, err
	}
	var items []core.Item
	for _, r := range page.Items {
		if r.UpdatedAt.Before(now.Add(-requestWindow)) {
			break
		}
		if r.ItemID.IsZero() {
			continue
		}
		it, err := s.store.Items().Get(ctx, r.ItemID)
		switch {
		case errors.Is(err, core.ErrNotFound):
			continue
		case err != nil:
			return nil, err
		}
		if u.CanAccess(&it) && !slices.ContainsFunc(items, func(o core.Item) bool { return o.ID == it.ID }) {
			items = append(items, it)
		}
	}
	items = items[min(offset, len(items)):]
	return items[:min(limit, len(items))], nil
}

// trending lists the titles of the first trending catalog of discovery
// that the server does not have, most popular first. Discovery failing
// lists none.
func (s *Service) trending(ctx context.Context, u *core.User) []requests.DiscoveredTitle {
	if s.requests == nil {
		return nil
	}
	titles, err := s.discoverTrending(ctx, u)
	if err != nil {
		if ctx.Err() == nil {
			s.logger().WarnContext(ctx, "couldn't list trending titles", "err", err)
		}
		return nil
	}
	return titles
}

func (s *Service) discoverTrending(ctx context.Context, u *core.User) ([]requests.DiscoveredTitle, error) {
	catalogs, err := s.requests.Catalogs(ctx, "")
	if err != nil {
		return nil, err
	}
	i := slices.IndexFunc(catalogs, func(c requests.Catalog) bool {
		return c.Catalog.GetKind() == pluginv1.CatalogKind_CATALOG_KIND_TRENDING
	})
	if i < 0 {
		return nil, nil
	}
	c := catalogs[i]
	// A catalog mixing movies and series lists both at once; others list
	// one kind per request.
	kinds := c.Catalog.GetMediaKinds()
	if len(kinds) == 0 || slices.Contains(kinds, pluginv1.MediaKind_MEDIA_KIND_UNSPECIFIED) {
		kinds = []pluginv1.MediaKind{pluginv1.MediaKind_MEDIA_KIND_UNSPECIFIED}
	}
	var titles []requests.DiscoveredTitle
	for _, k := range kinds {
		page, err := s.requests.Discover(ctx, *u, c.ID, pluginv1.DiscoverRequest_builder{MediaKind: &k}.Build())
		if err != nil {
			return nil, err
		}
		for _, t := range page.Titles {
			if t.Status != requests.StatusAvailable {
				titles = append(titles, t)
			}
		}
	}
	slices.SortStableFunc(titles, func(a, b requests.DiscoveredTitle) int {
		return cmp.Compare(b.Title.GetPopularity(), a.Title.GetPopularity())
	})
	return titles, nil
}

// split reads a comma-separated list.
func split(list string) []string {
	var out []string
	for v := range strings.SplitSeq(list, ",") {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}
