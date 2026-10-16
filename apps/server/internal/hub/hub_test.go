package hub

import (
	"errors"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store"
)

var now = time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)

// env is a films and a shows library with a user who watched some of
// them.
type env struct {
	store        *store.Store
	svc          *Service
	user         core.User
	films, shows core.Library
	items        map[string]core.Item
}

func newEnv(t *testing.T) *env {
	t.Helper()
	ctx := t.Context()
	s, err := store.Open(ctx, "sqlite:"+filepath.Join(t.TempDir(), "mavio.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	e := &env{store: s, svc: New(s, nil), items: map[string]core.Item{}}
	e.svc.Now = func() time.Time { return now }
	e.user = core.User{Name: "viewer", Kind: core.UserMember}
	if err := s.Users().Create(ctx, &e.user); err != nil {
		t.Fatal(err)
	}
	e.films = core.Library{Name: "Films", Kind: core.LibraryMovies, Paths: []string{t.TempDir()}}
	e.shows = core.Library{Name: "Shows", Kind: core.LibraryShows, Paths: []string{t.TempDir()}}
	for _, lib := range []*core.Library{&e.shows, &e.films} {
		if err := s.Libraries().Create(ctx, lib); err != nil {
			t.Fatal(err)
		}
	}
	e.add(t, e.films, core.NilID, core.KindMovie, "Alpha", 8, "Drama")
	e.add(t, e.films, core.NilID, core.KindMovie, "Beta", 9, "Drama")
	e.add(t, e.films, core.NilID, core.KindMovie, "Gamma", 5, "Comedy")
	for _, name := range []string{"Saga", "Tale"} {
		series := e.add(t, e.shows, core.NilID, core.KindSeries, name, 7)
		season := e.add(t, e.shows, series.ID, core.KindSeason, name+" S1", 0)
		for n := 1; n <= 2; n++ {
			ep := e.add(t, e.shows, season.ID, core.KindEpisode, name+" E"+string(rune('0'+n)), 0)
			ep.ParentIndexNumber, ep.IndexNumber = new(1), new(n)
			if err := s.Items().Upsert(ctx, ep); err != nil {
				t.Fatal(err)
			}
		}
	}
	e.play(t, "Alpha", 10*24*time.Hour, true, 0)
	e.play(t, "Saga E1", 24*time.Hour, true, 0)
	// Begun before the plays that seed hubs.
	e.play(t, "Gamma", 100*24*time.Hour, false, 20*time.Minute)
	return e
}

func (e *env) add(t *testing.T, lib core.Library, parent core.ID, kind core.ItemKind, name string, rating float64, genres ...string) core.Item {
	t.Helper()
	added := now.Add(-time.Duration(100-len(e.items)) * time.Hour)
	it := core.Item{
		ID: core.NewID(), LibraryID: lib.ID, ParentID: parent, Kind: kind, Name: name, Path: lib.Paths[0] + "/" + name,
		CommunityRating: rating, Genres: genres, DateAdded: added, PremiereDate: &added,
	}
	if err := e.store.Items().Upsert(t.Context(), it); err != nil {
		t.Fatal(err)
	}
	e.items[name] = it
	return it
}

// play records the user's state of an item played ago.
func (e *env) play(t *testing.T, name string, ago time.Duration, played bool, position time.Duration) {
	t.Helper()
	at := now.Add(-ago)
	d := core.UserData{UserID: e.user.ID, ItemID: e.items[name].ID, Played: played, Position: position, LastPlayedAt: &at}
	if err := e.store.UserData().Put(t.Context(), &d); err != nil {
		t.Fatal(err)
	}
}

// summary describes hubs as their IDs, seeds and item names.
func summary(hubs []Hub) [][]string {
	out := [][]string{}
	for _, h := range hubs {
		row := []string{h.ID, h.SeedName}
		for _, it := range h.Items {
			row = append(row, it.Name)
		}
		out = append(out, row)
	}
	return out
}

func equal(a, b [][]string) bool { return slices.EqualFunc(a, b, slices.Equal) }

func TestHome(t *testing.T) {
	e := newEnv(t)
	ctx := t.Context()
	films, shows := e.films.ID.String(), e.shows.ID.String()
	hubs, err := e.svc.Home(ctx, &e.user, 10)
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"continue_watching", "", "Saga E2", "Gamma"},
		{"recently_added/" + films, "", "Gamma", "Beta", "Alpha"},
		{"because_you_watched/" + films, "Alpha", "Beta"},
		{"top_unplayed/" + films, "", "Beta", "Gamma"},
		{"recently_added/" + shows, "", "Tale", "Saga"},
		{"start_watching/" + shows, "", "Tale"},
	}
	if got := summary(hubs); !equal(got, want) {
		t.Errorf("got = %q,\nwant = %q", got, want)
	}

	// The user pins the shows only and hides one of their hubs.
	layout, err := e.svc.SetLayout(ctx, e.user.ID, core.HomeLayout{
		Pinned: true, Libraries: []core.ID{e.shows.ID}, HiddenHubs: []string{"recently_added/" + shows, "continue_watching"},
	})
	if err != nil {
		t.Fatal(err)
	}
	e.user.Home = layout
	hubs, err = e.svc.Home(ctx, &e.user, 10)
	if err != nil {
		t.Fatal(err)
	}
	want = [][]string{{"continue_watching", "", "Saga E2", "Gamma"}, {"start_watching/" + shows, "", "Tale"}}
	if got := summary(hubs); !equal(got, want) {
		t.Errorf("pinned: got = %q, want = %q", got, want)
	}

	// The administrator chooses the films' hubs.
	e.films.Hubs = core.LibraryHubs{Home: []core.HubKind{core.HubTopRated}}
	if err := e.store.Libraries().Update(ctx, &e.films); err != nil {
		t.Fatal(err)
	}
	if e.user.Home, err = e.svc.SetLayout(ctx, e.user.ID, core.HomeLayout{Pinned: true, Libraries: []core.ID{e.films.ID}}); err != nil {
		t.Fatal(err)
	}
	hubs, err = e.svc.Home(ctx, &e.user, 2)
	if err != nil {
		t.Fatal(err)
	}
	want = [][]string{{"continue_watching", "", "Saga E2", "Gamma"}, {"top_rated/" + films, "", "Beta", "Alpha"}}
	if got := summary(hubs); !equal(got, want) || !hubs[1].More {
		t.Errorf("chosen: got = %q, more %v; want = %q, more", got, hubs[1].More, want)
	}
}

func TestRecommended(t *testing.T) {
	e := newEnv(t)
	ctx := t.Context()
	films := e.films.ID.String()
	hubs, err := e.svc.Recommended(ctx, &e.user, e.films.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"continue_watching/" + films, "", "Gamma"},
		{"recently_added/" + films, "", "Gamma", "Beta", "Alpha"},
		{"recently_released/" + films, "", "Gamma", "Beta", "Alpha"},
		{"top_rated/" + films, "", "Beta", "Alpha", "Gamma"},
		{"top_unplayed/" + films, "", "Beta", "Gamma"},
		{"because_you_watched/" + films, "Alpha", "Beta"},
		{"more_in_genre/" + films, "Drama", "Beta"},
	}
	if got := summary(hubs); !equal(got, want) {
		t.Errorf("got = %q,\nwant = %q", got, want)
	}

	limited := e.user
	limited.Access.Libraries = []core.ID{e.films.ID}
	if _, err := e.svc.Recommended(ctx, &limited, e.shows.ID, 10); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("another library: got = %v, want not found", err)
	}
}

func TestGet(t *testing.T) {
	e := newEnv(t)
	ctx := t.Context()
	id := "top_unplayed/" + e.films.ID.String()
	for _, tt := range []struct {
		offset int
		want   []string
		more   bool
	}{{0, []string{"Beta"}, true}, {1, []string{"Gamma"}, false}} {
		h, err := e.svc.Get(ctx, &e.user, id, 1, tt.offset)
		if err != nil {
			t.Fatal(err)
		}
		if got := summary([]Hub{h})[0][2:]; !slices.Equal(got, tt.want) || h.More != tt.more {
			t.Errorf("offset %d: got = %q, more %v; want = %q, more %v", tt.offset, got, h.More, tt.want, tt.more)
		}
	}
	for _, id := range []string{
		"bogus", "recently_added", "trending/" + e.films.ID.String(), "recently_aired/" + e.films.ID.String(),
		"top_rated/not-an-id", "top_rated/" + core.NewID().String(),
	} {
		if _, err := e.svc.Get(ctx, &e.user, id, 1, 0); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("%q: got = %v, want not found", id, err)
		}
	}
}

func TestSeedChangesDaily(t *testing.T) {
	e := newEnv(t)
	ctx := t.Context()
	e.play(t, "Beta", 3*24*time.Hour, true, 0)
	id := "because_you_watched/" + e.films.ID.String()
	seeds := map[string]bool{}
	for day := range 10 {
		e.svc.Now = func() time.Time { return now.Add(time.Duration(day) * 24 * time.Hour) }
		h, err := e.svc.Get(ctx, &e.user, id, 10, 0)
		if err != nil {
			t.Fatal(err)
		}
		again, err := e.svc.Get(ctx, &e.user, id, 10, 0)
		if err != nil {
			t.Fatal(err)
		}
		if h.SeedName != again.SeedName {
			t.Errorf("day %d: got seeds %q and %q, want one", day, h.SeedName, again.SeedName)
		}
		seeds[h.SeedName] = true
	}
	if !seeds["Alpha"] || !seeds["Beta"] {
		t.Errorf("got seeds %v, want both played films", seeds)
	}
}

func TestAvailableRequests(t *testing.T) {
	e := newEnv(t)
	ctx := t.Context()
	for _, r := range []core.Request{
		{Kind: core.KindMovie, TmdbID: 1, Title: "Beta", ItemID: e.items["Beta"].ID, Status: core.RequestCompleted, RequestedBy: e.user.ID},
		{Kind: core.KindMovie, TmdbID: 2, Title: "Gamma", ItemID: e.items["Gamma"].ID, Status: core.RequestApproved, RequestedBy: e.user.ID},
	} {
		if err := e.store.Requests().Put(ctx, &r); err != nil {
			t.Fatal(err)
		}
	}
	e.svc.Now = time.Now
	h, err := e.svc.Get(ctx, &e.user, "available_requests", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := summary([]Hub{h})[0][2:], []string{"Beta"}; !slices.Equal(got, want) {
		t.Errorf("got = %q, want = %q", got, want)
	}
	e.svc.Now = func() time.Time { return time.Now().Add(31 * 24 * time.Hour) }
	if h, err := e.svc.Get(ctx, &e.user, "available_requests", 10, 0); err != nil || len(h.Items) != 0 {
		t.Errorf("a month later: got = %d items, %v; want none", len(h.Items), err)
	}
}

func TestSetLayout(t *testing.T) {
	e := newEnv(t)
	ctx := t.Context()
	if _, err := e.svc.SetLayout(ctx, e.user.ID, core.HomeLayout{Pinned: true, Libraries: []core.ID{core.NewID()}}); !errors.Is(err, core.ErrInvalid) {
		t.Errorf("unknown library: got = %v, want = %v", err, core.ErrInvalid)
	}
	// Unpinned layouts keep no libraries.
	got, err := e.svc.SetLayout(ctx, e.user.ID, core.HomeLayout{Libraries: []core.ID{e.films.ID}, HiddenHubs: []string{"trending"}})
	if err != nil || got.Pinned || got.Libraries != nil || !slices.Equal(got.HiddenHubs, []string{"trending"}) {
		t.Errorf("unpinned: got = %+v, %v; want = hidden trending", got, err)
	}
	stored, err := e.store.Users().Get(ctx, e.user.ID)
	if err != nil || !slices.Equal(stored.Home.HiddenHubs, []string{"trending"}) {
		t.Errorf("stored: got = %+v, %v; want = hidden trending", stored.Home, err)
	}
}
