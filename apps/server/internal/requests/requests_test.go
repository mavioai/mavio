package requests

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/apps/server/internal/acquisition"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
	"github.com/mavioai/mavio/libs/metadata"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/store"
)

// title is a movie or series the fake metadata provider knows.
type title struct {
	name    string
	year    int
	rating  string
	tvdb    int
	seasons []int
	tags    []string
}

// fakeMetadata is a metadata provider and episode lister knowing titles
// by kind and TMDB ID.
type fakeMetadata struct {
	mu      sync.Mutex
	titles  map[string]title
	lookups int
	last    library.Lookup
}

func titleID(kind core.ItemKind, tmdb int) string { return string(kind) + ":" + strconv.Itoa(tmdb) }

func (f *fakeMetadata) Name() string { return "fake" }

func (f *fakeMetadata) lookup(l library.Lookup) (title, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lookups++
	f.last = l
	t, ok := f.titles[titleID(l.Kind, atoi(l.ExternalIDs[core.ProviderTMDB]))]
	return t, ok
}

func atoi(s string) int { n, _ := strconv.Atoi(s); return n }

func (f *fakeMetadata) Metadata(_ context.Context, l library.Lookup) (*metadata.Result, error) {
	t, ok := f.lookup(l)
	if !ok {
		return nil, nil
	}
	it := core.Item{
		Name: t.name, ProductionYear: t.year, OfficialRating: t.rating, Overview: "About " + t.name, Tags: t.tags,
		OriginalLanguage: "en", Studios: []string{"Studio " + t.name}, Tagline: "In " + l.Language,
		ExternalIDs: map[core.Provider]string{core.ProviderIMDb: "tt" + l.ExternalIDs[core.ProviderTMDB]},
	}
	if t.tvdb > 0 {
		it.ExternalIDs[core.ProviderTVDB] = strconv.Itoa(t.tvdb)
	}
	return &metadata.Result{
		Item: it, RemoteImages: []metadata.RemoteImage{{Kind: core.ImagePrimary, URL: "https://img/" + t.name}},
		People: []metadata.Person{{Name: "Lead", Kind: core.CreditActor, Role: "Hero"}},
		Facts: metadata.Facts{
			Budget: 1000, WatchProviders: []metadata.WatchProvider{{Name: "Stream"}},
			ProductionStatus: metadata.ProductionReleased, GenreRefs: []metadata.Ref{{ID: "18", Name: "Drama"}},
		},
	}, nil
}

func (f *fakeMetadata) Episodes(_ context.Context, l library.Lookup) (*library.EpisodeList, error) {
	t, ok := f.lookup(l)
	if !ok {
		return nil, nil
	}
	out := &library.EpisodeList{Provider: core.ProviderTMDB}
	for _, n := range t.seasons {
		aired := time.Date(2020+n, 1, 1, 0, 0, 0, 0, time.UTC)
		for e := 1; e <= 2; e++ {
			out.Episodes = append(out.Episodes, metadata.Result{Item: core.Item{
				Name: "Episode", ParentIndexNumber: new(n), IndexNumber: new(e), PremiereDate: new(aired.AddDate(0, 0, 7*(e-1))),
			}})
		}
	}
	return out, nil
}

// fakeAcquirer adds movies and series to the store as the acquisition
// runtime does: virtual, monitored, with seasons of two aired episodes.
type fakeAcquirer struct {
	t        *testing.T
	db       core.Store
	fail     error
	searches []core.ID
	seasons  []int
	series   []acquisition.NewSeries
	mu       sync.Mutex
}

func (f *fakeAcquirer) AddMovie(ctx context.Context, n acquisition.NewMovie) (core.Item, core.Wanted, error) {
	if f.fail != nil {
		return core.Item{}, core.Wanted{}, f.fail
	}
	it := core.Item{
		ID: core.NewID(), LibraryID: n.LibraryID, Kind: core.KindMovie, Name: "Movie " + strconv.Itoa(n.TmdbID), Virtual: true,
		Monitored: true, ExternalIDs: map[core.Provider]string{core.ProviderTMDB: strconv.Itoa(n.TmdbID)},
	}
	if err := f.db.Items().Upsert(ctx, it); err != nil {
		return core.Item{}, core.Wanted{}, err
	}
	w := core.Wanted{ItemID: it.ID, QualityProfileID: n.QualityProfileID, MinimumAvailability: n.MinimumAvailability}
	if err := f.db.Wanted().Put(ctx, &w); err != nil {
		return core.Item{}, core.Wanted{}, err
	}
	f.mu.Lock()
	f.searches = append(f.searches, it.ID)
	f.mu.Unlock()
	return it, w, nil
}

func (f *fakeAcquirer) AddSeries(ctx context.Context, n acquisition.NewSeries) (core.Item, core.Wanted, error) {
	if f.fail != nil {
		return core.Item{}, core.Wanted{}, f.fail
	}
	if n.Monitor != acquisition.MonitorNone {
		f.t.Errorf("AddSeries monitor = %v, want none", n.Monitor)
	}
	f.mu.Lock()
	f.series = append(f.series, n)
	f.mu.Unlock()
	series := core.Item{
		ID: core.NewID(), LibraryID: n.LibraryID, Kind: core.KindSeries, Name: "Series", Virtual: true, Monitored: true,
		ExternalIDs: map[core.Provider]string{core.ProviderTVDB: strconv.Itoa(n.TvdbID)},
	}
	if err := f.db.Items().Upsert(ctx, series); err != nil {
		return core.Item{}, core.Wanted{}, err
	}
	aired := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	for s := 1; s <= 3; s++ {
		season := core.Item{
			ID: core.NewID(), LibraryID: n.LibraryID, ParentID: series.ID, Kind: core.KindSeason, Name: "Season " + strconv.Itoa(s),
			Virtual: true, IndexNumber: new(s),
		}
		if err := f.db.Items().Upsert(ctx, season); err != nil {
			return core.Item{}, core.Wanted{}, err
		}
		for e := 1; e <= 2; e++ {
			ep := core.Item{
				ID: core.NewID(), LibraryID: n.LibraryID, ParentID: season.ID, Kind: core.KindEpisode,
				Name: "Episode", Virtual: true, ParentIndexNumber: new(s), IndexNumber: new(e), PremiereDate: &aired,
			}
			if err := f.db.Items().Upsert(ctx, ep); err != nil {
				return core.Item{}, core.Wanted{}, err
			}
		}
	}
	w := core.Wanted{ItemID: series.ID, QualityProfileID: n.QualityProfileID, SeasonFolder: n.SeasonFolder, MonitorNewItems: n.MonitorNewItems}
	if err := f.db.Wanted().Put(ctx, &w); err != nil {
		return core.Item{}, core.Wanted{}, err
	}
	return series, w, nil
}

func (f *fakeAcquirer) SetMonitored(ctx context.Context, ids []core.ID, monitored bool) error {
	for _, id := range ids {
		it, err := f.db.Items().Get(ctx, id)
		if err != nil {
			return err
		}
		it.Monitored = monitored
		if err := f.db.Items().Upsert(ctx, it); err != nil {
			return err
		}
	}
	return nil
}

func (f *fakeAcquirer) QueueSearch(_ context.Context, ids []core.ID, _ bool) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.searches = append(f.searches, ids...)
	return len(ids), nil
}

func (f *fakeAcquirer) QueueEpisodeSearch(_ context.Context, p acquisition.EpisodeSearchPayload) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seasons = append(f.seasons, p.SeasonNumber)
	return 1, nil
}

// recorder keeps the activities recorded.
type recorder struct {
	mu   sync.Mutex
	acts []core.Activity
}

func (r *recorder) Record(_ context.Context, a core.Activity) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.acts = append(r.acts, a)
}

// types returns the types of the activities of a user.
func (r *recorder) types(user core.ID) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []string
	for _, a := range r.acts {
		if a.UserID == user {
			out = append(out, a.Type)
		}
	}
	return out
}

type env struct {
	t        *testing.T
	ctx      context.Context
	db       core.Store
	svc      *Service
	acq      *fakeAcquirer
	meta     *fakeMetadata
	acts     *recorder
	settings core.RequestSettings
	clock    time.Time
	movies   core.Library
	shows    core.Library
	admin    core.User
}

func newEnv(t *testing.T) *env {
	t.Helper()
	ctx := t.Context()
	db, err := store.Open(ctx, "sqlite:"+filepath.Join(t.TempDir(), "mavio.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	e := &env{
		t: t, ctx: ctx, db: db, acq: &fakeAcquirer{t: t, db: db}, acts: &recorder{},
		clock: time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC),
		meta: &fakeMetadata{titles: map[string]title{
			titleID(core.KindMovie, 1):  {name: "Family Film", year: 2001, rating: "PG"},
			titleID(core.KindMovie, 2):  {name: "Grim Film", year: 2002, rating: "R"},
			titleID(core.KindMovie, 3):  {name: "Unrated Film", year: 2003},
			titleID(core.KindMovie, 4):  {name: "Fourth Film", year: 2004, rating: "G"},
			titleID(core.KindMovie, 5):  {name: "Fifth Film", year: 2005, rating: "G"},
			titleID(core.KindSeries, 9): {name: "Show", year: 2020, rating: "TV-PG", tvdb: 90, seasons: []int{0, 1, 2, 3}},
		}},
	}
	e.movies = core.Library{ID: core.NewID(), Name: "Movies", Kind: core.LibraryMovies, Paths: []string{t.TempDir()}}
	e.shows = core.Library{ID: core.NewID(), Name: "Shows", Kind: core.LibraryShows, Paths: []string{t.TempDir()}}
	for _, l := range []*core.Library{&e.movies, &e.shows} {
		if err := db.Libraries().Create(ctx, l); err != nil {
			t.Fatal(err)
		}
	}
	profile := func(kind core.ItemKind) core.ID {
		p := core.QualityProfile{ID: core.NewID(), Kind: kind, Name: string(kind), Cutoff: 7, Items: []core.QualityProfileItem{{Quality: 7, Allowed: true}}}
		if err := db.QualityProfiles().Put(ctx, &p); err != nil {
			t.Fatal(err)
		}
		return p.ID
	}
	e.settings = core.DefaultRequestSettings()
	e.settings.MovieLibraryID, e.settings.MovieQualityProfileID = e.movies.ID, profile(core.KindMovie)
	e.settings.SeriesLibraryID, e.settings.SeriesQualityProfileID = e.shows.ID, profile(core.KindSeries)
	e.svc = &Service{
		Store: db, Acquisition: e.acq, Settings: func() core.RequestSettings { return e.settings },
		Metadata: func() []library.Provider { return []library.Provider{e.meta} },
		Episodes: func() []library.EpisodeLister { return []library.EpisodeLister{e.meta} },
		Activity: e.acts, Now: func() time.Time { return e.clock },
	}
	e.admin = e.user("admin", func(u *core.User) { u.Kind = core.UserAdmin })
	return e
}

func (e *env) user(name string, edit func(*core.User)) core.User {
	e.t.Helper()
	u := core.User{ID: core.NewID(), Name: name, Kind: core.UserMember, AuthProvider: "test", Policy: core.UserPolicy{Requests: e.settings.DefaultPermissions}}
	if edit != nil {
		edit(&u)
	}
	if err := e.db.Users().Create(e.ctx, &u); err != nil {
		e.t.Fatal(err)
	}
	return u
}

func (e *env) movie(u core.User, tmdb int) (core.Request, error) {
	return e.svc.Create(e.ctx, u, NewRequest{Kind: core.KindMovie, TmdbID: tmdb})
}

func (e *env) must(r core.Request, err error) core.Request {
	e.t.Helper()
	if err != nil {
		e.t.Fatal(err)
	}
	return r
}

func (e *env) get(id core.ID) core.Request {
	e.t.Helper()
	r, err := e.db.Requests().Get(e.ctx, id)
	if err != nil {
		e.t.Fatal(err)
	}
	return r
}

// giveFiles gives the movie item, or the episodes of seasons of a series
// item, their files.
func (e *env) giveFiles(itemID core.ID, seasons ...int) {
	e.t.Helper()
	it, err := e.db.Items().Get(e.ctx, itemID)
	if err != nil {
		e.t.Fatal(err)
	}
	if it.Kind == core.KindMovie {
		it.Virtual, it.Path = false, "/movies/"+it.Name+".mkv"
		if err := e.db.Items().Upsert(e.ctx, it); err != nil {
			e.t.Fatal(err)
		}
		return
	}
	for ep, err := range e.db.Items().Walk(e.ctx, core.ItemQuery{ParentID: it.ID, Recursive: true, Kinds: []core.ItemKind{core.KindEpisode}, IncludeVirtual: true}) {
		if err != nil {
			e.t.Fatal(err)
		}
		if slices.Contains(seasons, *ep.ParentIndexNumber) {
			ep.Virtual, ep.Path = false, "/shows/"+ep.ID.String()+".mkv"
			if err := e.db.Items().Upsert(e.ctx, ep); err != nil {
				e.t.Fatal(err)
			}
		}
	}
}

func score(t *testing.T, rating string) *int {
	t.Helper()
	s, ok := metadata.RatingScore(rating, "US")
	if !ok {
		t.Fatalf("no score of %s", rating)
	}
	return &s
}

func TestCreatePermissions(t *testing.T) {
	e := newEnv(t)
	noMovies := e.user("series-only", func(u *core.User) { u.Policy.Requests.Movies = false })
	other := e.user("other", nil)
	tests := []struct {
		name string
		user core.User
		n    NewRequest
		want error
	}{
		{"no movie permission", noMovies, NewRequest{Kind: core.KindMovie, TmdbID: 1}, ErrPermission},
		{"for another user", other, NewRequest{Kind: core.KindMovie, TmdbID: 1, For: noMovies.ID}, ErrPermission},
		{"ignoring quotas", other, NewRequest{Kind: core.KindMovie, TmdbID: 1, IgnoreQuota: true}, ErrPermission},
		{"unknown title", other, NewRequest{Kind: core.KindMovie, TmdbID: 404}, core.ErrNotFound},
		{"bad kind", other, NewRequest{Kind: core.KindEpisode, TmdbID: 1}, core.ErrInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := e.svc.Create(e.ctx, tt.user, tt.n)
			if !errors.Is(err, tt.want) {
				t.Errorf("Create() error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestCreateRatingLimit(t *testing.T) {
	e := newEnv(t)
	pg := e.user("pg", func(u *core.User) { u.Access.MaxParentalRating = score(t, "PG") })
	strict := e.user("strict", func(u *core.User) {
		u.Access.MaxParentalRating, u.Access.BlockUnrated = score(t, "PG"), true
	})
	tests := []struct {
		name string
		user core.User
		tmdb int
		want error
	}{
		{"within the limit", pg, 1, nil},
		{"above the limit", pg, 2, ErrRating},
		{"unrated passes", pg, 3, nil},
		{"unrated blocked", strict, 4, nil},
		{"unrated blocked by policy", strict, 3, ErrRating},
		{"administrators are not limited by others' policies", e.admin, 2, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := e.movie(tt.user, tt.tmdb)
			if tt.want == nil && err != nil || tt.want != nil && !errors.Is(err, tt.want) {
				t.Errorf("Create(%d) error = %v, want %v", tt.tmdb, err, tt.want)
			}
		})
	}
}

func TestCreatePendingAndAutoApproved(t *testing.T) {
	e := newEnv(t)
	u := e.user("user", nil)
	r := e.must(e.movie(u, 1))
	if r.Status != core.RequestPending || r.Title != "Family Film" || r.Year != 2001 || r.RequestedBy != u.ID {
		t.Errorf("Create() = %+v, want a pending request of Family Film by the user", r)
	}
	if got := e.acts.types(u.ID); !slices.Equal(got, []string{TypePending}) {
		t.Errorf("activities = %v, want [%s]", got, TypePending)
	}
	if len(e.acq.searches) != 0 {
		t.Errorf("searches = %v, want none before approval", e.acq.searches)
	}

	auto := e.user("auto", func(u *core.User) { u.Policy.Requests.AutoApproveMovies = true })
	r = e.must(e.movie(auto, 4))
	if r.Status != core.RequestApproved || r.ItemID.IsZero() {
		t.Errorf("Create() = %+v, want approved with an item", r)
	}
	if got := e.acts.types(auto.ID); !slices.Equal(got, []string{TypeAutoApproved}) {
		t.Errorf("activities = %v, want [%s]", got, TypeAutoApproved)
	}
	w, err := e.db.Wanted().Get(e.ctx, r.ItemID)
	if err != nil || w.QualityProfileID != e.settings.MovieQualityProfileID {
		t.Errorf("Wanted = %+v, %v, want the requests' quality profile", w, err)
	}
}

func TestCreateDuplicates(t *testing.T) {
	e := newEnv(t)
	u := e.user("user", nil)
	e.must(e.movie(u, 1))
	if _, err := e.movie(e.user("second", nil), 1); !errors.Is(err, core.ErrConflict) {
		t.Errorf("requesting a requested movie: error = %v, want ErrConflict", err)
	}
	// A wanted movie, then an available one, cannot be requested.
	r := e.must(e.svc.Create(e.ctx, e.admin, NewRequest{Kind: core.KindMovie, TmdbID: 4}))
	e.must(e.svc.Create(e.ctx, e.admin, NewRequest{Kind: core.KindMovie, TmdbID: 5}))
	if err := e.db.Requests().Delete(e.ctx, r.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.movie(u, 4); !errors.Is(err, core.ErrConflict) {
		t.Errorf("requesting a wanted movie: error = %v, want ErrConflict", err)
	}
	e.giveFiles(r.ItemID)
	if _, err := e.movie(u, 4); !errors.Is(err, core.ErrConflict) {
		t.Errorf("requesting an available movie: error = %v, want ErrConflict", err)
	}
	// Declined requests do not stand in the way.
	d := e.must(e.movie(u, 2))
	e.must(e.svc.Decline(e.ctx, e.admin, d.ID))
	e.must(e.movie(e.user("third", nil), 2))
}

func TestQuota(t *testing.T) {
	e := newEnv(t)
	e.settings.MovieQuota = core.Quota{Limit: 2, Days: 7}
	u := e.user("user", nil)
	first := e.must(e.movie(u, 1))
	e.must(e.movie(u, 2))
	if _, err := e.movie(u, 3); !errors.Is(err, ErrQuota) {
		t.Fatalf("third request in a week: error = %v, want ErrQuota", err)
	}
	q, err := e.svc.Quota(e.ctx, u, core.KindMovie)
	if err != nil || q != (Quota{Limit: 2, Days: 7, Used: 2, Remaining: 0, Restricted: true}) {
		t.Errorf("Quota() = %+v, %v", q, err)
	}
	// Declined requests free the quota.
	e.must(e.svc.Decline(e.ctx, e.admin, first.ID))
	third := e.must(e.movie(u, 3))
	// Requests older than the window do not count.
	e.clock = e.clock.Add(8 * 24 * time.Hour)
	e.must(e.movie(u, 4))
	if q, _ := e.svc.Quota(e.ctx, u, core.KindMovie); q.Used != 1 {
		t.Errorf("Quota().Used a week later = %d, want 1", q.Used)
	}
	// Users' own quotas win; administrators are unlimited and may ignore
	// quotas for others.
	own := e.user("own", func(u *core.User) { u.Policy.MovieQuota = &core.Quota{Limit: 1} })
	e.must(e.movie(own, 5))
	if _, err := e.movie(own, 3); !errors.Is(err, ErrQuota) && !errors.Is(err, core.ErrConflict) {
		t.Errorf("own quota: error = %v, want ErrQuota", err)
	}
	if err := e.db.Requests().Delete(e.ctx, third.ID); err != nil {
		t.Fatal(err)
	}
	r := e.must(e.svc.Create(e.ctx, e.admin, NewRequest{Kind: core.KindMovie, TmdbID: 3, For: own.ID, IgnoreQuota: true}))
	if !r.IgnoreQuota || r.RequestedBy != own.ID {
		t.Errorf("request for a user ignoring quotas = %+v", r)
	}
	if q, _ := e.svc.Quota(e.ctx, e.admin, core.KindMovie); q.Limit != 0 || q.Restricted {
		t.Errorf("administrator's Quota() = %+v, want unlimited", q)
	}
}

func TestSeriesSeasons(t *testing.T) {
	e := newEnv(t)
	e.settings.SeriesQuota = core.Quota{Limit: 4}
	u := e.user("user", nil)
	r := e.must(e.svc.Create(e.ctx, u, NewRequest{Kind: core.KindSeries, TmdbID: 9, Seasons: []int{0, 1, 1}}))
	if got := r.SeasonNumbers(); !slices.Equal(got, []int{1}) || r.TvdbID != 90 {
		t.Errorf("seasons = %v, TVDB %d, want [1] without specials, 90", got, r.TvdbID)
	}
	// All seasons leaves out those requested already.
	other := e.user("other", nil)
	r2 := e.must(e.svc.Create(e.ctx, other, NewRequest{Kind: core.KindSeries, TmdbID: 9, AllSeasons: true}))
	if got := r2.SeasonNumbers(); !slices.Equal(got, []int{2, 3}) {
		t.Errorf("all seasons = %v, want [2 3]", got)
	}
	if _, err := e.svc.Create(e.ctx, e.user("third", nil), NewRequest{Kind: core.KindSeries, TmdbID: 9, AllSeasons: true}); !errors.Is(err, core.ErrConflict) {
		t.Errorf("no seasons left: error = %v, want ErrConflict", err)
	}
	// Quotas count seasons.
	if _, err := e.svc.Update(e.ctx, other, r2.ID, []int{2, 3, 4, 5, 6}); !errors.Is(err, ErrQuota) {
		t.Errorf("Update past the quota: error = %v, want ErrQuota", err)
	}
	if _, err := e.svc.Update(e.ctx, other, r2.ID, []int{1, 2}); !errors.Is(err, core.ErrConflict) && err != nil {
		t.Errorf("Update() error = %v", err)
	}
	if updated := e.get(r2.ID); !slices.Equal(updated.SeasonNumbers(), []int{2}) {
		got := updated.SeasonNumbers()
		t.Errorf("updated seasons = %v, want [2]: season 1 is requested already", got)
	}
	if _, err := e.svc.Update(e.ctx, u, r2.ID, []int{3}); !errors.Is(err, ErrPermission) {
		t.Errorf("Update of another's request: error = %v, want ErrPermission", err)
	}
}

func TestApproveSeriesAndAvailability(t *testing.T) {
	e := newEnv(t)
	u := e.user("user", nil)
	r := e.must(e.svc.Create(e.ctx, u, NewRequest{Kind: core.KindSeries, TmdbID: 9, Seasons: []int{1, 2}}))
	if _, err := e.svc.Approve(e.ctx, u, r.ID); !errors.Is(err, ErrPermission) {
		t.Errorf("Approve by a user: error = %v, want ErrPermission", err)
	}
	r = e.must(e.svc.Approve(e.ctx, e.admin, r.ID))
	if r.Status != core.RequestApproved || r.ItemID.IsZero() {
		t.Fatalf("Approve() = %+v", r)
	}
	if !slices.Equal(e.acq.seasons, []int{1, 2}) {
		t.Errorf("season searches = %v, want [1 2]", e.acq.seasons)
	}
	var monitored []int
	for it, err := range e.db.Items().Walk(e.ctx, core.ItemQuery{ParentID: r.ItemID, Kinds: []core.ItemKind{core.KindSeason}, IncludeVirtual: true}) {
		if err != nil {
			t.Fatal(err)
		}
		if it.Monitored {
			monitored = append(monitored, *it.IndexNumber)
		}
	}
	slices.Sort(monitored)
	if !slices.Equal(monitored, []int{1, 2}) {
		t.Errorf("monitored seasons = %v, want [1 2]", monitored)
	}
	if _, err := e.svc.Approve(e.ctx, e.admin, r.ID); !errors.Is(err, ErrState) {
		t.Errorf("approving twice: error = %v, want ErrState", err)
	}
	// Season 3 is free; seasons 1 and 2 are wanted.
	if _, err := e.svc.Create(e.ctx, e.user("other", nil), NewRequest{Kind: core.KindSeries, TmdbID: 9, Seasons: []int{1}}); !errors.Is(err, core.ErrConflict) {
		t.Errorf("requesting a wanted season: error = %v, want ErrConflict", err)
	}

	e.giveFiles(r.ItemID, 1)
	e.svc.Imported(e.ctx, r.ItemID)
	r = e.get(r.ID)
	if r.Status != core.RequestApproved || r.Seasons[0].Status != core.RequestCompleted || r.Seasons[1].Status != core.RequestApproved {
		t.Errorf("after season 1 = %+v, want season 1 completed", r)
	}
	e.giveFiles(r.ItemID, 2)
	if err := e.svc.Reconcile(e.ctx); err != nil {
		t.Fatal(err)
	}
	if r = e.get(r.ID); r.Status != core.RequestCompleted {
		t.Errorf("after season 2 = %+v, want completed", r)
	}
	if got := e.acts.types(u.ID); !slices.Equal(got, []string{TypePending, TypeApproved, TypeAvailable}) {
		t.Errorf("activities = %v", got)
	}
}

func TestApproveMovieFailsAndRetries(t *testing.T) {
	e := newEnv(t)
	u := e.user("user", nil)
	r := e.must(e.movie(u, 1))
	e.acq.fail = errors.New("no root folder")
	r = e.must(e.svc.Approve(e.ctx, e.admin, r.ID))
	if r.Status != core.RequestFailed || r.Message == "" {
		t.Fatalf("Approve() with acquisition failing = %+v, want failed with a message", r)
	}
	if _, err := e.svc.Retry(e.ctx, u, r.ID); !errors.Is(err, ErrPermission) {
		t.Errorf("Retry by a user: error = %v, want ErrPermission", err)
	}
	e.acq.fail = nil
	r = e.must(e.svc.Retry(e.ctx, e.admin, r.ID))
	if r.Status != core.RequestApproved || r.ItemID.IsZero() || r.Message != "" {
		t.Fatalf("Retry() = %+v", r)
	}
	if _, err := e.svc.Retry(e.ctx, e.admin, r.ID); !errors.Is(err, ErrState) {
		t.Errorf("retrying an approved request: error = %v, want ErrState", err)
	}
	e.giveFiles(r.ItemID)
	e.svc.Imported(e.ctx, r.ItemID)
	if r = e.get(r.ID); r.Status != core.RequestCompleted {
		t.Errorf("after the import = %+v, want completed", r)
	}
	if got := e.acts.types(u.ID); !slices.Equal(got, []string{TypePending, TypeFailed, TypeApproved, TypeAvailable}) {
		t.Errorf("activities = %v", got)
	}
}

func TestDeleteAndList(t *testing.T) {
	e := newEnv(t)
	u, other := e.user("user", nil), e.user("other", nil)
	r := e.must(e.movie(u, 1))
	e.must(e.movie(other, 2))
	page, err := e.svc.List(e.ctx, u, core.RequestQuery{})
	if err != nil || page.Total != 1 || page.Items[0].ID != r.ID {
		t.Errorf("List() by a user = %+v, %v, want their own", page, err)
	}
	if _, err := e.svc.List(e.ctx, u, core.RequestQuery{RequestedBy: other.ID}); !errors.Is(err, ErrPermission) {
		t.Errorf("List() of another user: error = %v, want ErrPermission", err)
	}
	if page, _ := e.svc.List(e.ctx, e.admin, core.RequestQuery{}); page.Total != 2 {
		t.Errorf("List() by an administrator: total = %d, want 2", page.Total)
	}
	if _, err := e.svc.Get(e.ctx, other, r.ID); !errors.Is(err, ErrPermission) {
		t.Errorf("Get() of another's request: error = %v, want ErrPermission", err)
	}
	e.must(e.svc.Approve(e.ctx, e.admin, r.ID))
	if err := e.svc.Delete(e.ctx, u, r.ID); !errors.Is(err, ErrPermission) {
		t.Errorf("Delete of an approved request by its user: error = %v, want ErrPermission", err)
	}
	if err := e.svc.Delete(e.ctx, e.admin, r.ID); err != nil {
		t.Errorf("Delete by an administrator: %v", err)
	}
}

func TestConcurrentRequests(t *testing.T) {
	e := newEnv(t)
	users := make([]core.User, 8)
	for i := range users {
		users[i] = e.user("user"+strconv.Itoa(i), nil)
	}
	var wg sync.WaitGroup
	errs := make([]error, len(users))
	for i, u := range users {
		wg.Go(func() { _, errs[i] = e.movie(u, 1) })
	}
	wg.Wait()
	ok := 0
	for _, err := range errs {
		switch {
		case err == nil:
			ok++
		case !errors.Is(err, core.ErrConflict):
			t.Errorf("Create() error = %v", err)
		}
	}
	if ok != 1 {
		t.Errorf("%d requests of one movie succeeded, want 1", ok)
	}
}

func TestIssues(t *testing.T) {
	e := newEnv(t)
	reporter := e.user("reporter", func(u *core.User) { u.Policy.Requests.ReportIssues = true })
	other := e.user("other", func(u *core.User) { u.Policy.Requests.ReportIssues = true })
	silent := e.user("silent", nil)
	it := core.Item{ID: core.NewID(), LibraryID: e.movies.ID, Kind: core.KindMovie, Name: "Film", Path: "/movies/film.mkv"}
	if err := e.db.Items().Upsert(e.ctx, it); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.CreateIssue(e.ctx, silent, NewIssue{ItemID: it.ID, Type: core.IssueVideo, Message: "x"}); !errors.Is(err, ErrPermission) {
		t.Errorf("CreateIssue without permission: error = %v, want ErrPermission", err)
	}
	i, err := e.svc.CreateIssue(e.ctx, reporter, NewIssue{ItemID: it.ID, Type: core.IssueAudio, Message: "No sound"})
	if err != nil {
		t.Fatal(err)
	}
	if i.Status != core.IssueOpen || len(i.Comments) != 1 || i.Comments[0].Message != "No sound" {
		t.Errorf("CreateIssue() = %+v", i)
	}
	if _, err := e.svc.GetIssue(e.ctx, other, i.ID); !errors.Is(err, ErrPermission) {
		t.Errorf("GetIssue of another's issue: error = %v, want ErrPermission", err)
	}
	if _, err := e.svc.Comment(e.ctx, other, i.ID, "me too"); !errors.Is(err, ErrPermission) {
		t.Errorf("Comment on another's issue: error = %v, want ErrPermission", err)
	}
	if i, err = e.svc.Comment(e.ctx, e.admin, i.ID, "Looking"); err != nil || len(i.Comments) != 2 {
		t.Fatalf("Comment() = %+v, %v", i, err)
	}
	if i, err = e.svc.Resolve(e.ctx, reporter, i.ID); err != nil || i.Status != core.IssueResolved {
		t.Errorf("Resolve by its creator = %+v, %v", i, err)
	}
	if i, err = e.svc.Reopen(e.ctx, e.admin, i.ID); err != nil || i.Status != core.IssueOpen {
		t.Errorf("Reopen() = %+v, %v", i, err)
	}
	if err := e.svc.DeleteIssue(e.ctx, reporter, i.ID); !errors.Is(err, ErrPermission) {
		t.Errorf("DeleteIssue by its creator after comments: error = %v, want ErrPermission", err)
	}
	if err := e.svc.DeleteIssue(e.ctx, e.admin, i.ID); err != nil {
		t.Errorf("DeleteIssue by an administrator: %v", err)
	}
	got := e.acts.types(reporter.ID)
	for _, want := range []string{TypeIssueCommented, TypeIssueReopened} {
		if !slices.Contains(got, want) {
			t.Errorf("reporter's activities = %v, want %s", got, want)
		}
	}
}

// fakeDiscovery is a discovery provider of one catalog filtering by
// certification and one that does not.
type fakeDiscovery struct {
	mu         sync.Mutex
	titles     []*pluginv1.DiscoveredTitle
	last       *pluginv1.DiscoverRequest
	lastSearch *pluginv1.SearchTitlesRequest
	// noSearch answers searches unimplemented.
	noSearch bool
}

func (f *fakeDiscovery) SearchTitles(_ context.Context, req *pluginv1.SearchTitlesRequest) (*pluginv1.SearchTitlesResponse, error) {
	if f.noSearch {
		return nil, connect.NewError(connect.CodeUnimplemented, errors.New("no search"))
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastSearch = req
	return pluginv1.SearchTitlesResponse_builder{Titles: f.titles, Page: new(int32(1)), TotalPages: new(int32(3)), TotalResults: new(int32(60))}.Build(), nil
}

func (f *fakeDiscovery) ListCatalogs(context.Context, *pluginv1.ListCatalogsRequest) (*pluginv1.ListCatalogsResponse, error) {
	return pluginv1.ListCatalogsResponse_builder{Catalogs: []*pluginv1.Catalog{
		pluginv1.Catalog_builder{
			Id: new("popular"), Name: new("Popular"), Kind: new(pluginv1.CatalogKind_CATALOG_KIND_POPULAR),
			MediaKinds: []pluginv1.MediaKind{pluginv1.MediaKind_MEDIA_KIND_MOVIE},
			Filters:    []pluginv1.DiscoverFilter{pluginv1.DiscoverFilter_DISCOVER_FILTER_CERTIFICATION},
		}.Build(),
		pluginv1.Catalog_builder{
			Id: new("trending"), Name: new("Trending"), Kind: new(pluginv1.CatalogKind_CATALOG_KIND_TRENDING),
			MediaKinds: []pluginv1.MediaKind{pluginv1.MediaKind_MEDIA_KIND_UNSPECIFIED, pluginv1.MediaKind_MEDIA_KIND_MOVIE, pluginv1.MediaKind_MEDIA_KIND_SERIES},
		}.Build(),
	}}.Build(), nil
}

func (f *fakeDiscovery) Discover(_ context.Context, req *pluginv1.DiscoverRequest) (*pluginv1.DiscoverResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.last = req
	return pluginv1.DiscoverResponse_builder{Titles: f.titles, Page: new(int32(1)), TotalPages: new(int32(1)), TotalResults: new(int32(len(f.titles)))}.Build(), nil
}

func (f *fakeDiscovery) ListFilterValues(context.Context, *pluginv1.ListFilterValuesRequest) (*pluginv1.ListFilterValuesResponse, error) {
	return pluginv1.ListFilterValuesResponse_builder{Values: []*pluginv1.FilterValue{pluginv1.FilterValue_builder{Id: new("28"), Name: new("Action")}.Build()}}.Build(), nil
}

func discovered(kind pluginv1.MediaKind, tmdb int, adult bool) *pluginv1.DiscoveredTitle {
	return pluginv1.DiscoveredTitle_builder{
		MediaKind: new(kind), Name: new("T" + strconv.Itoa(tmdb)), Adult: new(adult),
		ExternalIds: map[string]string{"tmdb": strconv.Itoa(tmdb)},
	}.Build()
}

func TestDiscover(t *testing.T) {
	e := newEnv(t)
	movie, series := pluginv1.MediaKind_MEDIA_KIND_MOVIE, pluginv1.MediaKind_MEDIA_KIND_SERIES
	d := &fakeDiscovery{titles: []*pluginv1.DiscoveredTitle{
		discovered(movie, 1, false), discovered(movie, 2, false), discovered(movie, 3, false),
		discovered(movie, 4, false), discovered(movie, 66, true), discovered(series, 9, false),
	}}
	e.svc.Discovery = func() []DiscoveryProvider { return []DiscoveryProvider{{ID: "tmdb", Client: d}} }

	cs, err := e.svc.Catalogs(e.ctx, "")
	if err != nil || len(cs) != 2 || cs[0].ID != "tmdb/popular" || cs[1].ID != "tmdb/trending" {
		t.Fatalf("Catalogs() = %+v, %v", cs, err)
	}
	if v, err := e.svc.FilterValues(e.ctx, "tmdb/popular", &pluginv1.ListFilterValuesRequest{}); err != nil || len(v) != 1 {
		t.Errorf("FilterValues() = %v, %v", v, err)
	}
	if _, err := e.svc.Discover(e.ctx, e.admin, "nope/popular", &pluginv1.DiscoverRequest{}); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("Discover of an unknown provider: error = %v, want ErrNotFound", err)
	}

	pg := e.user("pg", func(u *core.User) { u.Access.MaxParentalRating = score(t, "PG") })
	// Title 1 is requested, 4 wanted with its file, the series wanted in
	// part.
	e.must(e.movie(pg, 1))
	avail := e.must(e.svc.Create(e.ctx, e.admin, NewRequest{Kind: core.KindMovie, TmdbID: 4}))
	e.giveFiles(avail.ItemID)
	sr := e.must(e.svc.Create(e.ctx, e.admin, NewRequest{Kind: core.KindSeries, TmdbID: 9, Seasons: []int{1}}))
	e.giveFiles(sr.ItemID, 1)

	// The certification catalog is asked for the certifications within
	// the limit when the user's policy blocks unrated titles, or of those
	// the caller asks for; else its titles are rated by the server, which
	// keeps the unrated ones.
	strict := e.user("strict", func(u *core.User) { u.Access.MaxParentalRating, u.Access.BlockUnrated = score(t, "PG"), true })
	if _, err := e.svc.Discover(e.ctx, strict, "tmdb/popular", pluginv1.DiscoverRequest_builder{MediaKind: new(movie)}.Build()); err != nil {
		t.Fatal(err)
	}
	if got := d.last.GetCertifications(); !slices.Contains(got, "PG") || slices.Contains(got, "R") || d.last.GetCertificationCountry() != "US" || d.last.GetIncludeAdult() {
		t.Errorf("certifications asked = %v in %q", got, d.last.GetCertificationCountry())
	}
	if _, err := e.svc.Discover(e.ctx, pg, "tmdb/popular", pluginv1.DiscoverRequest_builder{MediaKind: new(movie), Certifications: []string{"PG", "R"}}.Build()); err != nil {
		t.Fatal(err)
	}
	if got := d.last.GetCertifications(); !slices.Equal(got, []string{"PG"}) {
		t.Errorf("certifications asked of those given = %v, want [PG]", got)
	}
	page, err := e.svc.Discover(e.ctx, pg, "tmdb/popular", pluginv1.DiscoverRequest_builder{MediaKind: new(movie)}.Build())
	if err != nil {
		t.Fatal(err)
	}
	if len(d.last.GetCertifications()) > 0 || !slices.ContainsFunc(page.Titles, func(d DiscoveredTitle) bool { return d.TmdbID == 3 }) {
		t.Errorf("certifications asked when unrated titles are allowed = %v; unrated title kept = %v", d.last.GetCertifications(), page.Titles)
	}
	strictPage, err := e.svc.Discover(e.ctx, strict, "tmdb/trending", &pluginv1.DiscoverRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if slices.ContainsFunc(strictPage.Titles, func(d DiscoveredTitle) bool { return d.TmdbID == 3 }) {
		t.Errorf("unrated title listed to a user blocking them")
	}
	// Others' titles are looked up and those above the limit left out.
	page, err = e.svc.Discover(e.ctx, pg, "tmdb/trending", &pluginv1.DiscoverRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if d.last.HasCertificationCountry() || len(d.last.GetCertifications()) > 0 {
		t.Errorf("certifications asked of a catalog without the filter = %v", d.last.GetCertifications())
	}
	type mark struct {
		tmdb        int
		status      MediaStatus
		requested   bool
		requestable bool
	}
	var got []mark
	for _, d := range page.Titles {
		got = append(got, mark{d.TmdbID, d.Status, !d.RequestID.IsZero(), d.Requestable})
	}
	want := []mark{
		{1, StatusRequested, true, false},
		{3, StatusNone, false, true},
		{4, StatusAvailable, true, false},
		{9, StatusAvailable, true, false},
	}
	// The series is wanted in season 1 only, which has its files.
	if !slices.Equal(got, want) {
		t.Errorf("Discover() = %+v, want %+v", got, want)
	}
	// The adult title and the one rated R are left out of the page.
	if page.Hidden != 2 || page.TotalResults != 4 || page.TotalPages != 1 {
		t.Errorf("Discover() hidden = %d of %d results in %d pages, want 2 of 4 in 1", page.Hidden, page.TotalResults, page.TotalPages)
	}
}
