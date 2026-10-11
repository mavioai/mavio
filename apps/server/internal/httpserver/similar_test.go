package httpserver_test

import (
	"net/http"
	"slices"
	"testing"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/libs/core"
	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1/libraryv1connect"
	userv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1/userv1connect"
)

func TestListSimilarItems(t *testing.T) {
	ctx := t.Context()
	url, s := startServer(t, nil)
	films := core.Library{Name: "Films", Kind: core.LibraryMovies, Paths: []string{t.TempDir()}}
	shows := core.Library{Name: "Shows", Kind: core.LibraryShows, Paths: []string{t.TempDir()}}
	for _, lib := range []*core.Library{&films, &shows} {
		if err := s.Libraries().Create(ctx, lib); err != nil {
			t.Fatal(err)
		}
	}
	item := func(lib core.Library, parent core.ID, kind core.ItemKind, name string, genres ...string) core.Item {
		return core.Item{ID: core.NewID(), LibraryID: lib.ID, ParentID: parent, Kind: kind, Name: name, Path: lib.Paths[0] + "/" + name, Genres: genres}
	}
	seed := item(films, core.NilID, core.KindMovie, "Seed", "Drama", "Crime")
	close := item(films, core.NilID, core.KindMovie, "Close", "Drama", "Crime")
	far := item(films, core.NilID, core.KindMovie, "Far", "Drama")
	rated := item(films, core.NilID, core.KindMovie, "Rated", "Drama", "Crime")
	rated.ParentalRating = new(17)
	unrelated := item(films, core.NilID, core.KindMovie, "Unrelated", "Animation")
	series := item(shows, core.NilID, core.KindSeries, "Series", "Drama")
	season := item(shows, series.ID, core.KindSeason, "Season 1")
	episode := item(shows, season.ID, core.KindEpisode, "Episode")
	other := item(shows, core.NilID, core.KindSeries, "Other Series", "Drama")
	if err := s.Items().Upsert(ctx, seed, close, far, rated, unrelated, series, season, episode, other); err != nil {
		t.Fatal(err)
	}

	token := signUp(t, url)
	asAdmin := libraryv1connect.NewItemServiceClient(http.DefaultClient, url, withToken(token))
	similar := func(c libraryv1connect.ItemServiceClient, id core.ID) []string {
		t.Helper()
		resp, err := c.ListSimilarItems(ctx, libraryv1.ListSimilarItemsRequest_builder{Id: new(id.String())}.Build())
		if err != nil {
			t.Fatalf("ListSimilarItems: %v", err)
		}
		return names(resp.GetItems())
	}
	// Ties are broken by ID, so compare the scores' order only.
	if got := similar(asAdmin, seed.ID); len(got) != 3 || !slices.Contains(got[:2], "Close") || !slices.Contains(got[:2], "Rated") || got[2] != "Far" {
		t.Errorf("similar to the seed = %q, want = Close and Rated, then Far", got)
	}
	if got, want := similar(asAdmin, episode.ID), []string{"Other Series"}; !slices.Equal(got, want) {
		t.Errorf("similar to an episode = %q, want = %q", got, want)
	}

	users := userv1connect.NewUserServiceClient(http.DefaultClient, url, withToken(token))
	if _, err := users.CreateUser(ctx, userv1.CreateUserRequest_builder{
		Name: new("kid"), Password: new("pw"),
		Policy: userv1.UserPolicy_builder{LibraryIds: []string{films.ID.String(), shows.ID.String()}, MaxParentalRating: new(int32(12))}.Build(),
	}.Build()); err != nil {
		t.Fatal(err)
	}
	asKid := libraryv1connect.NewItemServiceClient(http.DefaultClient, url, withToken(login(t, url, "kid", "pw", "tablet")))
	if got, want := similar(asKid, seed.ID), []string{"Close", "Far"}; !slices.Equal(got, want) {
		t.Errorf("kid's similar = %q, want = %q", got, want)
	}
	_, err := asKid.ListSimilarItems(ctx, libraryv1.ListSimilarItemsRequest_builder{Id: new(rated.ID.String())}.Build())
	wantCode(t, "kid lists items similar to a rated film", err, connect.CodeNotFound)
}
