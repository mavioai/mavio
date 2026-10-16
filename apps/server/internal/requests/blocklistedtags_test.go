package requests

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/libs/core"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

// tagPage is what the fake keyword catalog lists of a keyword and kind:
// the pages and the titles of each page.
type tagPage struct {
	pages  int
	titles func(page int) []int
}

// fakeTags is a discovery provider with a popular catalog filtering by
// keyword, sorting by popularity and rating, and a collection catalog.
type fakeTags struct {
	mu        sync.Mutex
	keywords  map[string]map[pluginv1.MediaKind]tagPage
	parts     map[string][]int
	fail      func(*pluginv1.DiscoverRequest) bool
	requests  []string
	noCollect bool
}

func (f *fakeTags) SearchTitles(context.Context, *pluginv1.SearchTitlesRequest) (*pluginv1.SearchTitlesResponse, error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("no search"))
}

func (f *fakeTags) GetPerson(context.Context, *pluginv1.GetPersonRequest) (*pluginv1.GetPersonResponse, error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("no people"))
}

func (f *fakeTags) GetCollection(context.Context, *pluginv1.GetCollectionRequest) (*pluginv1.GetCollectionResponse, error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("no collections"))
}

func (f *fakeTags) ListCatalogs(context.Context, *pluginv1.ListCatalogsRequest) (*pluginv1.ListCatalogsResponse, error) {
	catalogs := []*pluginv1.Catalog{
		pluginv1.Catalog_builder{
			Id: new("trending"), Kind: new(pluginv1.CatalogKind_CATALOG_KIND_TRENDING),
			MediaKinds: []pluginv1.MediaKind{pluginv1.MediaKind_MEDIA_KIND_MOVIE, pluginv1.MediaKind_MEDIA_KIND_SERIES},
		}.Build(),
		pluginv1.Catalog_builder{
			Id: new("popular"), Kind: new(pluginv1.CatalogKind_CATALOG_KIND_POPULAR),
			MediaKinds: []pluginv1.MediaKind{pluginv1.MediaKind_MEDIA_KIND_MOVIE, pluginv1.MediaKind_MEDIA_KIND_SERIES},
			Filters:    []pluginv1.DiscoverFilter{pluginv1.DiscoverFilter_DISCOVER_FILTER_KEYWORD},
			Sorts:      []pluginv1.DiscoverSort{pluginv1.DiscoverSort_DISCOVER_SORT_POPULARITY, pluginv1.DiscoverSort_DISCOVER_SORT_RATING},
		}.Build(),
	}
	if !f.noCollect {
		catalogs = append(catalogs, pluginv1.Catalog_builder{
			Id: new("collection"), Kind: new(pluginv1.CatalogKind_CATALOG_KIND_COLLECTION),
			MediaKinds: []pluginv1.MediaKind{pluginv1.MediaKind_MEDIA_KIND_MOVIE},
		}.Build())
	}
	return pluginv1.ListCatalogsResponse_builder{Catalogs: catalogs}.Build(), nil
}

func (f *fakeTags) Discover(_ context.Context, req *pluginv1.DiscoverRequest) (*pluginv1.DiscoverResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if req.GetCatalog() == "collection" {
		var titles []*pluginv1.DiscoveredTitle
		for _, id := range f.parts[req.GetExternalIds()["tmdb_collection"]] {
			titles = append(titles, discovered(pluginv1.MediaKind_MEDIA_KIND_MOVIE, id, false))
		}
		return pluginv1.DiscoverResponse_builder{Titles: titles, Page: new(int32(1)), TotalPages: new(int32(1))}.Build(), nil
	}
	if req.GetCatalog() != "popular" || len(req.GetKeywords()) != 1 {
		return nil, fmt.Errorf("unexpected request %v", req)
	}
	kind := req.GetMediaKind()
	f.requests = append(f.requests, fmt.Sprintf("%s %s p%d %s asc=%v", req.GetKeywords()[0], kind, req.GetPage(), req.GetSort(), req.GetAscending()))
	if f.fail != nil && f.fail(req) {
		return nil, errors.New("TMDB is down")
	}
	tp := f.keywords[req.GetKeywords()[0]][kind]
	out := pluginv1.DiscoverResponse_builder{Page: new(req.GetPage()), TotalPages: new(int32(tp.pages))}.Build()
	if tp.titles != nil && int(req.GetPage()) <= tp.pages {
		for _, id := range tp.titles(int(req.GetPage())) {
			out.SetTitles(append(out.GetTitles(), discovered(kind, id, false)))
		}
	}
	return out, nil
}

func (f *fakeTags) ListFilterValues(_ context.Context, req *pluginv1.ListFilterValuesRequest) (*pluginv1.ListFilterValuesResponse, error) {
	out := &pluginv1.ListFilterValuesResponse{}
	for _, id := range req.GetIds() {
		if _, ok := f.keywords[id]; ok && req.GetFilter() == pluginv1.DiscoverFilter_DISCOVER_FILTER_KEYWORD {
			out.SetValues(append(out.GetValues(), pluginv1.FilterValue_builder{Id: new(id), Name: new("kw" + id)}.Build()))
		}
	}
	return out, nil
}

func TestBlocklistTags(t *testing.T) {
	e := newEnv(t)
	movie, series := pluginv1.MediaKind_MEDIA_KIND_MOVIE, pluginv1.MediaKind_MEDIA_KIND_SERIES
	f := &fakeTags{
		keywords: map[string]map[pluginv1.MediaKind]tagPage{
			// More pages than queries: every order, two pages each.
			"10": {
				movie:  {pages: 100, titles: func(page int) []int { return []int{page} }},
				series: {pages: 3, titles: func(page int) []int { return []int{1000 + page} }},
			},
			"11": {movie: {pages: 1, titles: func(int) []int { return []int{1} }}},
		},
		fail: func(r *pluginv1.DiscoverRequest) bool {
			return r.GetPage() == 2 && r.GetSort() == pluginv1.DiscoverSort_DISCOVER_SORT_RATING && r.GetAscending()
		},
	}
	e.svc.Discovery = func() []DiscoveryProvider { return []DiscoveryProvider{{ID: "tmdb", Client: f}} }
	var sleeps []time.Duration
	e.svc.Sleep = func(_ context.Context, d time.Duration) error { sleeps = append(sleeps, d); return nil }
	var dropped []string
	e.svc.DropBlocklistedTags = func(_ context.Context, tags []string) error { dropped = tags; return nil }
	e.settings.BlocklistedTags = []string{"tmdb/10", "tmdb/11", "tmdb/20", "gone/1"}
	e.settings.BlocklistedTagsLimit = 2

	// Movie 2 is blocklisted by hand, 99 by a tag it no longer has.
	repo := e.db.BlockedTitles()
	byHand := core.BlockedTitle{Kind: core.KindMovie, TmdbID: 2, Title: "By hand", UserID: e.admin.ID, CreatedAt: e.clock}
	for _, b := range []*core.BlockedTitle{&byHand, {Kind: core.KindMovie, TmdbID: 99, Tags: []string{"tmdb/10"}, CreatedAt: e.clock}} {
		if err := repo.Add(e.ctx, b); err != nil {
			t.Fatal(err)
		}
	}

	if err := e.svc.BlocklistTags(e.ctx); err != nil {
		t.Fatalf("BlocklistTags() = %v", err)
	}
	wantRequests := []string{
		"10 MEDIA_KIND_MOVIE p1 DISCOVER_SORT_POPULARITY asc=false", "10 MEDIA_KIND_MOVIE p1 DISCOVER_SORT_POPULARITY asc=true",
		"10 MEDIA_KIND_MOVIE p1 DISCOVER_SORT_RATING asc=false", "10 MEDIA_KIND_MOVIE p1 DISCOVER_SORT_RATING asc=true",
		"10 MEDIA_KIND_MOVIE p2 DISCOVER_SORT_POPULARITY asc=false", "10 MEDIA_KIND_MOVIE p2 DISCOVER_SORT_POPULARITY asc=true",
		"10 MEDIA_KIND_MOVIE p2 DISCOVER_SORT_RATING asc=false", "10 MEDIA_KIND_MOVIE p2 DISCOVER_SORT_RATING asc=true",
		// No more pages than queries: the catalog's own order.
		"10 MEDIA_KIND_SERIES p1 DISCOVER_SORT_POPULARITY asc=false",
		"10 MEDIA_KIND_SERIES p2 DISCOVER_SORT_UNSPECIFIED asc=false", "10 MEDIA_KIND_SERIES p3 DISCOVER_SORT_UNSPECIFIED asc=false",
		"11 MEDIA_KIND_MOVIE p1 DISCOVER_SORT_POPULARITY asc=false",
		"11 MEDIA_KIND_SERIES p1 DISCOVER_SORT_POPULARITY asc=false",
	}
	if !slices.Equal(f.requests, wantRequests) {
		t.Errorf("requests = %q, want = %q", f.requests, wantRequests)
	}
	// Every query but the failing one is followed by a pause.
	if len(sleeps) != len(wantRequests)-1 || sleeps[0] != tagQueryDelay {
		t.Errorf("sleeps = %v, want = %d of %v", sleeps, len(wantRequests)-1, tagQueryDelay)
	}
	if want := []string{"tmdb/20"}; !slices.Equal(dropped, want) {
		t.Errorf("dropped tags = %v, want = %v", dropped, want)
	}

	page, err := repo.List(e.ctx, core.BlockedTitleQuery{})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string][]string{}
	for _, b := range page.Items {
		got[string(b.Kind)+strconv.Itoa(b.TmdbID)] = b.Tags
	}
	want := map[string][]string{
		"movie1": {"tmdb/10", "tmdb/11"}, "movie2": nil,
		"series1001": {"tmdb/10"}, "series1002": {"tmdb/10"}, "series1003": {"tmdb/10"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("blocklist = %v, want = %v", got, want)
	}
	if b, err := repo.Get(e.ctx, core.KindMovie, 2); err != nil || !reflect.DeepEqual(b, byHand) {
		t.Errorf("title blocklisted by hand = %+v, %v, want = %+v", b, err, byHand)
	}

	// Without tags, the titles blocklisted by tags are taken off it, as
	// Seerr cleans the blocklist first.
	e.settings.BlocklistedTags = nil
	if err := e.svc.BlocklistTags(e.ctx); err != nil {
		t.Fatal(err)
	}
	if page, err := repo.List(e.ctx, core.BlockedTitleQuery{}); err != nil || page.Total != 1 || page.Items[0].TmdbID != 2 {
		t.Errorf("blocklist without tags = %+v, %v, want = the title blocklisted by hand", page, err)
	}

	// Cancelling stops the job, leaving the blocklist as it is.
	e.settings.BlocklistedTags = []string{"tmdb/10"}
	ctx, cancel := context.WithCancel(e.ctx)
	e.svc.Sleep = func(context.Context, time.Duration) error { cancel(); return context.Canceled }
	if err := e.svc.BlocklistTags(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("BlocklistTags(cancelled) = %v, want = context.Canceled", err)
	}
	if page, err := repo.List(e.ctx, core.BlockedTitleQuery{}); err != nil || page.Total != 1 {
		t.Errorf("blocklist after cancelling = %+v, %v, want = unchanged", page, err)
	}
}

func TestBlocklistedTagsJob(t *testing.T) {
	e := newEnv(t)
	if err := e.svc.Schedule(e.ctx); err != nil {
		t.Fatal(err)
	}
	jobs, err := e.db.Jobs().List(e.ctx, core.JobQuery{Kinds: []string{JobBlocklistedTags}})
	if err != nil || len(jobs.Items) != 1 || !jobs.Items[0].RunAt.Equal(e.clock.Add(BlocklistedTagsInterval)) {
		t.Fatalf("scheduled jobs = %+v, %v, want = one a week later", jobs, err)
	}
	next, err := e.svc.Handlers()[JobBlocklistedTags](e.ctx, jobs.Items[0])
	if err != nil || len(next) != 1 || next[0].Kind != JobBlocklistedTags || !next[0].RunAt.Equal(e.clock.Add(BlocklistedTagsInterval)) {
		t.Errorf("handler = %+v, %v, want = the next run", next, err)
	}
	if next, err := e.svc.Handlers()[JobBlocklistedTags](e.ctx, RunBlocklistedTagsJob(e.clock)); err != nil || len(next) != 0 {
		t.Errorf("handler on demand = %+v, %v, want = no next run", next, err)
	}
}

func TestBlockCollection(t *testing.T) {
	e := newEnv(t)
	f := &fakeTags{parts: map[string][]int{"10": {1, 2, 2}}}
	e.svc.Discovery = func() []DiscoveryProvider { return []DiscoveryProvider{{ID: "tmdb", Client: f}} }
	if _, err := e.svc.Block(e.ctx, e.admin, core.KindMovie, 2); err != nil {
		t.Fatal(err)
	}
	user := e.user("user", nil)
	if _, err := e.svc.BlockCollection(e.ctx, user, 10); !errors.Is(err, ErrPermission) {
		t.Errorf("BlockCollection by a user = %v, want = ErrPermission", err)
	}
	blocked, err := e.svc.BlockCollection(e.ctx, e.admin, 10)
	if err != nil || len(blocked) != 1 || blocked[0].TmdbID != 1 || blocked[0].Kind != core.KindMovie ||
		blocked[0].Title != "T1" || blocked[0].UserID != e.admin.ID {
		t.Fatalf("BlockCollection() = %+v, %v, want = movie 1", blocked, err)
	}
	if ok, err := e.svc.isBlocked(e.ctx, core.KindMovie, 1); err != nil || !ok {
		t.Errorf("movie 1 blocked = %v, %v", ok, err)
	}
	if err := e.svc.UnblockCollection(e.ctx, user, 10); !errors.Is(err, ErrPermission) {
		t.Errorf("UnblockCollection by a user = %v, want = ErrPermission", err)
	}
	if err := e.svc.UnblockCollection(e.ctx, e.admin, 10); err != nil {
		t.Fatal(err)
	}
	if page, err := e.db.BlockedTitles().List(e.ctx, core.BlockedTitleQuery{}); err != nil || page.Total != 0 {
		t.Errorf("blocklist after UnblockCollection = %+v, %v, want = empty", page, err)
	}
	f.noCollect = true
	if _, err := e.svc.BlockCollection(e.ctx, e.admin, 10); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("BlockCollection without collection catalogs = %v, want = ErrNotFound", err)
	}
}
