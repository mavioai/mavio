package browse

import (
	"fmt"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store"
)

// env is a films, a shows and a music library with a user.
type env struct {
	store  *store.Store
	b      *Browser
	user   core.ID
	items  map[string]core.Item
	scope  Scope
	shows  core.Library
	clock  time.Time
	series []core.Item
}

func newEnv(t *testing.T) *env {
	t.Helper()
	ctx := t.Context()
	s, err := store.Open(ctx, "sqlite:"+filepath.Join(t.TempDir(), "mavio.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	e := &env{store: s, b: New(s), items: map[string]core.Item{}, clock: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	u := core.User{Name: "viewer", PasswordHash: "x"}
	if err := s.Users().Create(ctx, &u); err != nil {
		t.Fatal(err)
	}
	e.user = u.ID
	e.scope = Scope{UserID: u.ID}

	libs := map[core.LibraryKind]*core.Library{}
	for _, kind := range []core.LibraryKind{core.LibraryMovies, core.LibraryShows, core.LibraryMusic} {
		lib := &core.Library{Name: string(kind), Kind: kind, Paths: []string{t.TempDir()}}
		if err := s.Libraries().Create(ctx, lib); err != nil {
			t.Fatal(err)
		}
		libs[kind] = lib
	}
	e.shows = *libs[core.LibraryShows]
	// Added in this order, one hour apart: the film, then each series'
	// episodes, the album's tracks and a second film.
	e.add(t, *libs[core.LibraryMovies], core.NilID, core.KindMovie, "Alien")
	for _, name := range []string{"Lost", "Fargo"} {
		series := e.add(t, e.shows, core.NilID, core.KindSeries, name)
		e.series = append(e.series, series)
		season := e.add(t, e.shows, series.ID, core.KindSeason, name+" S1")
		for n := 1; n <= 3; n++ {
			ep := e.add(t, e.shows, season.ID, core.KindEpisode, fmt.Sprintf("%s E%d", name, n))
			ep.ParentIndexNumber, ep.IndexNumber = new(1), new(n)
			e.put(t, ep)
		}
	}
	album := e.add(t, *libs[core.LibraryMusic], core.NilID, core.KindMusicAlbum, "Bloom")
	for n := 1; n <= 2; n++ {
		e.add(t, *libs[core.LibraryMusic], album.ID, core.KindTrack, fmt.Sprintf("Track %d", n))
	}
	e.add(t, *libs[core.LibraryMovies], core.NilID, core.KindMovie, "Up")
	return e
}

func (e *env) add(t *testing.T, lib core.Library, parent core.ID, kind core.ItemKind, name string) core.Item {
	t.Helper()
	e.clock = e.clock.Add(time.Hour)
	it := core.Item{ID: core.NewID(), LibraryID: lib.ID, ParentID: parent, Kind: kind, Name: name, DateAdded: e.clock}
	e.put(t, it)
	return it
}

func (e *env) put(t *testing.T, it core.Item) {
	t.Helper()
	if err := e.store.Items().Upsert(t.Context(), it); err != nil {
		t.Fatal(err)
	}
	e.items[it.Name] = it
}

// play records the user's state of an item at hour h.
func (e *env) play(t *testing.T, name string, h int, played bool, position time.Duration) {
	t.Helper()
	at := time.Date(2026, 2, 1, h, 0, 0, 0, time.UTC)
	d := core.UserData{UserID: e.user, ItemID: e.items[name].ID, Played: played, Position: position, LastPlayedAt: &at}
	if err := e.store.UserData().Put(t.Context(), &d); err != nil {
		t.Fatal(err)
	}
}

func names(items []core.Item) []string {
	out := []string{}
	for _, it := range items {
		out = append(out, it.Name)
	}
	return out
}

func TestLatest(t *testing.T) {
	e := newEnv(t)
	ctx := t.Context()
	latest := func(scope Scope, q LatestQuery) []string {
		t.Helper()
		list, err := e.b.Latest(ctx, scope, q)
		if err != nil {
			t.Fatal(err)
		}
		return names(list)
	}
	e.play(t, "Up", 1, true, 0)
	tests := []struct {
		name string
		q    LatestQuery
		want []string
	}{
		{"ungrouped", LatestQuery{Limit: 4}, []string{"Up", "Track 2", "Track 1", "Fargo E3"}},
		{"grouped: albums and single-season series", LatestQuery{Group: true, Limit: 4}, []string{"Up", "Bloom", "Fargo", "Lost"}},
		{"kinds", LatestQuery{Group: true, Kinds: []core.ItemKind{core.KindMovie}, Limit: 5}, []string{"Up", "Alien"}},
		{"unplayed", LatestQuery{Group: true, Played: new(false), Limit: 1}, []string{"Bloom"}},
	}
	for _, tt := range tests {
		if got := latest(e.scope, tt.q); !slices.Equal(got, tt.want) {
			t.Errorf("%s: got = %q, want = %q", tt.name, got, tt.want)
		}
	}

	shows := e.scope
	shows.LibraryIDs = []core.ID{e.shows.ID}
	grouped := LatestQuery{Group: true, Limit: 2}
	// A day later, a second season of Lost: it shows as the season.
	e.clock = e.clock.Add(48 * time.Hour)
	season := e.add(t, e.shows, e.series[0].ID, core.KindSeason, "Lost S2")
	for n := 1; n <= 2; n++ {
		ep := e.add(t, e.shows, season.ID, core.KindEpisode, fmt.Sprintf("Lost S2E%d", n))
		ep.ParentIndexNumber, ep.IndexNumber = new(2), new(n)
		e.put(t, ep)
	}
	if got, want := latest(shows, grouped), []string{"Lost S2", "Fargo"}; !slices.Equal(got, want) {
		t.Errorf("new season: got = %q, want = %q", got, want)
	}
	// Another day later, one more Fargo episode: it shows as itself.
	e.clock = e.clock.Add(48 * time.Hour)
	ep := e.add(t, e.shows, e.items["Fargo S1"].ID, core.KindEpisode, "Fargo E4")
	ep.ParentIndexNumber, ep.IndexNumber = new(1), new(4)
	e.put(t, ep)
	if got, want := latest(shows, grouped), []string{"Fargo E4", "Lost S2"}; !slices.Equal(got, want) {
		t.Errorf("single new episode: got = %q, want = %q", got, want)
	}
	// A user who may not see the rated series sees none of it.
	lost := e.series[0]
	lost.ParentalRating = 17
	e.put(t, lost)
	limited := shows
	limited.MaxRating = 12
	// Fargo's older episodes belong to the same entry.
	if got, want := latest(limited, grouped), []string{"Fargo E4"}; !slices.Equal(got, want) {
		t.Errorf("rating limit: got = %q, want = %q", got, want)
	}
}

func TestNextUp(t *testing.T) {
	e := newEnv(t)
	ctx := t.Context()
	nextUp := func(scope Scope, q NextUpQuery) []string {
		t.Helper()
		list, err := e.b.NextUp(ctx, scope, q)
		if err != nil {
			t.Fatal(err)
		}
		return names(list)
	}
	if got := nextUp(e.scope, NextUpQuery{Limit: 10}); len(got) != 0 {
		t.Errorf("nothing played: got = %q", got)
	}

	e.play(t, "Lost E1", 1, true, 0)
	e.play(t, "Fargo E1", 2, true, 0)
	e.play(t, "Fargo E2", 3, false, 10*time.Minute) // being watched
	e.play(t, "Alien", 4, true, 0)
	tests := []struct {
		name  string
		scope Scope
		q     NextUpQuery
		want  []string
	}{
		{"most recently played series first, resumed left out", e.scope, NextUpQuery{Limit: 10}, []string{"Lost E2"}},
		{"resumable included", e.scope, NextUpQuery{Limit: 10, IncludeResumable: true}, []string{"Fargo E2", "Lost E2"}},
		{"page", e.scope, NextUpQuery{Limit: 1, Offset: 1, IncludeResumable: true}, []string{"Lost E2"}},
		{"since", e.scope, NextUpQuery{Limit: 10, IncludeResumable: true, Since: time.Date(2026, 2, 1, 2, 0, 0, 0, time.UTC)}, []string{"Fargo E2"}},
		{"one series", e.scope, NextUpQuery{SeriesID: e.series[0].ID, Limit: 10}, []string{"Lost E2"}},
		{"other libraries", Scope{UserID: e.user, LibraryIDs: []core.ID{e.items["Alien"].LibraryID}}, NextUpQuery{Limit: 10}, nil},
	}
	for _, tt := range tests {
		if got := nextUp(tt.scope, tt.q); !slices.Equal(got, tt.want) {
			t.Errorf("%s: got = %q, want = %q", tt.name, got, tt.want)
		}
	}

	// A rated series hides its unrated episodes from a limited user.
	lost := e.series[0]
	lost.ParentalRating = 17
	e.put(t, lost)
	if got := nextUp(Scope{UserID: e.user, MaxRating: 12}, NextUpQuery{Limit: 10, IncludeResumable: true}); !slices.Equal(got, []string{"Fargo E2"}) {
		t.Errorf("rating limit: got = %q", got)
	}
}
