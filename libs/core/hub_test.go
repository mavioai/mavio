package core_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/mavioai/mavio/libs/core"
)

func TestLibraryHubs(t *testing.T) {
	tests := []struct {
		name string
		kind core.LibraryKind
		hubs core.LibraryHubs
		ok   bool
	}{
		{"defaults", core.LibraryMovies, core.LibraryHubs{}, true},
		{"none", core.LibraryShows, core.LibraryHubs{Home: []core.HubKind{}, Recommended: []core.HubKind{}}, true},
		{"chosen", core.LibraryShows, core.LibraryHubs{Home: []core.HubKind{core.HubStartWatching}, Recommended: []core.HubKind{core.HubContinueWatching, core.HubRecentlyAired}}, true},
		{"not offered", core.LibraryMusic, core.LibraryHubs{Recommended: []core.HubKind{core.HubTopRated}}, false},
		{"global", core.LibraryMovies, core.LibraryHubs{Home: []core.HubKind{core.HubTrending}}, false},
		{"twice", core.LibraryMovies, core.LibraryHubs{Home: []core.HubKind{core.HubTopRated, core.HubTopRated}}, false},
		{"continue watching on Home", core.LibraryMovies, core.LibraryHubs{Home: []core.HubKind{core.HubContinueWatching}}, false},
		{"unknown", core.LibraryMovies, core.LibraryHubs{Home: []core.HubKind{"popular"}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lib := core.Library{Name: "L", Kind: tt.kind, Paths: []string{"/media"}, Hubs: tt.hubs}
			err := lib.Validate()
			if (err == nil) != tt.ok || (err != nil && !errors.Is(err, core.ErrInvalid)) {
				t.Errorf("got = %v, want ok %v", err, tt.ok)
			}
		})
	}
}

func TestLibraryHubsResolve(t *testing.T) {
	home, recommended := core.LibraryHubs{}.Resolve(core.LibraryMovies)
	if !slices.Equal(home, []core.HubKind{core.HubRecentlyAdded, core.HubBecauseYouWatched, core.HubTopUnplayed}) ||
		!slices.Equal(recommended, core.LibraryMovies.HubKinds()) {
		t.Errorf("got = %v, %v; want the movie defaults", home, recommended)
	}
	home, recommended = core.LibraryHubs{Home: []core.HubKind{}}.Resolve(core.LibraryCollections)
	if home == nil || len(home) != 0 || recommended != nil {
		t.Errorf("got = %v, %v; want no hubs", home, recommended)
	}
	for _, k := range core.HubKinds {
		if !k.Valid() {
			t.Errorf("got = invalid, want %q valid", k)
		}
	}
}
