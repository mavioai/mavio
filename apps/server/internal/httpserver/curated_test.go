package httpserver_test

import (
	"net/http"
	"slices"
	"testing"

	"connectrpc.com/connect"

	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1/libraryv1connect"
	userv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1/userv1connect"
)

func TestCollectionsAndPlaylists(t *testing.T) {
	ctx := t.Context()
	url, s := startServer(t, nil)
	c := seedCatalog(t, s)
	token := signUp(t, url)
	collections := libraryv1connect.NewCollectionServiceClient(http.DefaultClient, url, withToken(token))
	playlists := libraryv1connect.NewPlaylistServiceClient(http.DefaultClient, url, withToken(token))
	items := libraryv1connect.NewItemServiceClient(http.DefaultClient, url, withToken(token))
	libraries := libraryv1connect.NewLibraryServiceClient(http.DefaultClient, url, withToken(token))
	children := func(client libraryv1connect.ItemServiceClient, parent string) []string {
		t.Helper()
		resp, err := client.ListItems(ctx, libraryv1.ListItemsRequest_builder{ParentId: &parent}.Build())
		if err != nil {
			t.Fatalf("ListItems: %v", err)
		}
		return names(resp.GetItems())
	}

	// A collection across libraries, in the collections library the server
	// creates.
	created, err := collections.CreateCollection(ctx, libraryv1.CreateCollectionRequest_builder{
		Name: new("Favourites"), ItemIds: []string{c.up.ID.String(), c.alien.ID.String(), c.series.ID.String(), c.up.ID.String()},
	}.Build())
	if err != nil {
		t.Fatal(err)
	}
	saga := created.GetCollection()
	if saga.GetKind() != libraryv1.ItemKind_ITEM_KIND_COLLECTION || saga.GetName() != "Favourites" {
		t.Errorf("collection = %v", saga)
	}
	if got := children(items, saga.GetId()); !slices.Equal(got, []string{"Up", "Alien", "Show"}) {
		t.Errorf("collection items = %q", got)
	}
	if _, err := collections.RemoveFromCollection(ctx, libraryv1.RemoveFromCollectionRequest_builder{Id: new(saga.GetId()), ItemIds: []string{c.alien.ID.String()}}.Build()); err != nil {
		t.Fatal(err)
	}
	if _, err := collections.AddToCollection(ctx, libraryv1.AddToCollectionRequest_builder{Id: new(saga.GetId()), ItemIds: []string{c.pilot.ID.String(), c.up.ID.String()}}.Build()); err != nil {
		t.Fatal(err)
	}
	if got := children(items, saga.GetId()); !slices.Equal(got, []string{"Up", "Show", "Pilot"}) {
		t.Errorf("collection after editing = %q", got)
	}
	byName := []*libraryv1.SortSpec{libraryv1.SortSpec_builder{Field: new(libraryv1.SortField_SORT_FIELD_NAME)}.Build()}
	resp, err := items.ListItems(ctx, libraryv1.ListItemsRequest_builder{ParentId: new(saga.GetId()), Sort: byName}.Build())
	if got := names(resp.GetItems()); err != nil || !slices.Equal(got, []string{"Pilot", "Show", "Up"}) {
		t.Errorf("collection by name = %q, %v", got, err)
	}
	renamed, err := collections.UpdateCollection(ctx, libraryv1.UpdateCollectionRequest_builder{Id: new(saga.GetId()), Name: new("All Time")}.Build())
	if err != nil || renamed.GetCollection().GetName() != "All Time" || renamed.GetCollection().GetSortName() != "All Time" {
		t.Errorf("renamed = %v, %v", renamed, err)
	}
	libs, err := libraries.ListLibraries(ctx, &libraryv1.ListLibrariesRequest{})
	if err != nil {
		t.Fatal(err)
	}
	var curated []*libraryv1.Library
	for _, lib := range libs.GetLibraries() {
		if k := lib.GetKind(); k == libraryv1.LibraryKind_LIBRARY_KIND_COLLECTIONS || k == libraryv1.LibraryKind_LIBRARY_KIND_PLAYLISTS {
			curated = append(curated, lib)
		}
	}
	if len(curated) != 1 || curated[0].GetName() != "Collections" || len(curated[0].GetPaths()) != 0 {
		t.Fatalf("curated libraries = %v", curated)
	}
	_, err = libraries.ScanLibrary(ctx, libraryv1.ScanLibraryRequest_builder{Id: new(curated[0].GetId())}.Build())
	wantCode(t, "scan the collections library", err, connect.CodeFailedPrecondition)
	_, err = libraries.CreateLibrary(ctx, libraryv1.CreateLibraryRequest_builder{Spec: libraryv1.LibrarySpec_builder{
		Name: new("More"), Kind: new(libraryv1.LibraryKind_LIBRARY_KIND_PLAYLISTS), Paths: []string{t.TempDir()},
	}.Build()}.Build())
	wantCode(t, "create a playlists library", err, connect.CodeInvalidArgument)
	_, err = collections.CreateCollection(ctx, libraryv1.CreateCollectionRequest_builder{Name: new("Nested"), ItemIds: []string{saga.GetId()}}.Build())
	wantCode(t, "collection in a collection", err, connect.CodeInvalidArgument)

	// A playlist: containers add their episodes in order, entries keep
	// duplicates and move.
	made, err := playlists.CreatePlaylist(ctx, libraryv1.CreatePlaylistRequest_builder{
		Name: new("Evening"), ItemIds: []string{c.series.ID.String(), c.alien.ID.String()},
	}.Build())
	if err != nil {
		t.Fatal(err)
	}
	mix := made.GetPlaylist()
	entries := func(client libraryv1connect.PlaylistServiceClient) ([]string, []string) {
		t.Helper()
		resp, err := client.ListPlaylistEntries(ctx, libraryv1.ListPlaylistEntriesRequest_builder{Id: new(mix.GetId())}.Build())
		if err != nil {
			t.Fatalf("ListPlaylistEntries: %v", err)
		}
		var names, ids []string
		for _, e := range resp.GetEntries() {
			names, ids = append(names, e.GetItem().GetName()), append(ids, e.GetId())
		}
		if int(resp.GetTotal()) != len(names) {
			t.Errorf("total = %d, want %d", resp.GetTotal(), len(names))
		}
		return names, ids
	}
	if got, _ := entries(playlists); !slices.Equal(got, []string{"Pilot", "Finale", "Alien"}) {
		t.Errorf("entries = %q", got)
	}
	added, err := playlists.AddToPlaylist(ctx, libraryv1.AddToPlaylistRequest_builder{Id: new(mix.GetId()), ItemIds: []string{c.alien.ID.String()}, Position: new(int32(0))}.Build())
	if err != nil || len(added.GetEntryIds()) != 1 {
		t.Fatalf("AddToPlaylist = %v, %v", added, err)
	}
	got, ids := entries(playlists)
	if !slices.Equal(got, []string{"Alien", "Pilot", "Finale", "Alien"}) || ids[0] != added.GetEntryIds()[0] {
		t.Errorf("after adding = %q", got)
	}
	if _, err := playlists.MovePlaylistEntry(ctx, libraryv1.MovePlaylistEntryRequest_builder{Id: new(mix.GetId()), EntryId: &ids[0], Position: new(int32(99))}.Build()); err != nil {
		t.Fatal(err)
	}
	if _, err := playlists.RemoveFromPlaylist(ctx, libraryv1.RemoveFromPlaylistRequest_builder{Id: new(mix.GetId()), EntryIds: []string{ids[3]}}.Build()); err != nil {
		t.Fatal(err)
	}
	if got, _ := entries(playlists); !slices.Equal(got, []string{"Pilot", "Finale", "Alien"}) {
		t.Errorf("after moving and removing = %q", got)
	}
	if got := children(items, mix.GetId()); !slices.Equal(got, []string{"Pilot", "Finale", "Alien"}) {
		t.Errorf("playlist items = %q", got)
	}
	_, err = playlists.AddToPlaylist(ctx, libraryv1.AddToPlaylistRequest_builder{Id: new(mix.GetId()), ItemIds: []string{mix.GetId()}}.Build())
	wantCode(t, "playlist in a playlist", err, connect.CodeInvalidArgument)

	// Another user sees neither the playlist nor, without access to it,
	// the collections library; their playlists are their own.
	users := userv1connect.NewUserServiceClient(http.DefaultClient, url, withToken(token))
	if _, err := users.CreateUser(ctx, userv1.CreateUserRequest_builder{
		Name: new("kid"), Password: new("pw"),
		Policy: userv1.UserPolicy_builder{LibraryIds: []string{c.shows.ID.String()}, MaxParentalRating: new(int32(12))}.Build(),
	}.Build()); err != nil {
		t.Fatal(err)
	}
	kidToken := login(t, url, "kid", "pw", "tablet")
	kidPlaylists := libraryv1connect.NewPlaylistServiceClient(http.DefaultClient, url, withToken(kidToken))
	kidItems := libraryv1connect.NewItemServiceClient(http.DefaultClient, url, withToken(kidToken))
	_, err = kidPlaylists.ListPlaylistEntries(ctx, libraryv1.ListPlaylistEntriesRequest_builder{Id: new(mix.GetId())}.Build())
	wantCode(t, "kid lists the admin's playlist", err, connect.CodeNotFound)
	_, err = kidItems.ListItems(ctx, libraryv1.ListItemsRequest_builder{ParentId: new(saga.GetId())}.Build())
	wantCode(t, "kid lists a collection", err, connect.CodeNotFound)
	_, err = libraryv1connect.NewCollectionServiceClient(http.DefaultClient, url, withToken(kidToken)).
		DeleteCollection(ctx, libraryv1.DeleteCollectionRequest_builder{Id: new(saga.GetId())}.Build())
	wantCode(t, "kid deletes a collection", err, connect.CodePermissionDenied)
	_, err = kidPlaylists.CreatePlaylist(ctx, libraryv1.CreatePlaylistRequest_builder{Name: new("Mine"), ItemIds: []string{c.alien.ID.String()}}.Build())
	wantCode(t, "kid adds a film it may not see", err, connect.CodeNotFound)
	// The season inherits the series' rating, above the kid's limit; the
	// pilot is rated for the kid.
	_, err = kidPlaylists.CreatePlaylist(ctx, libraryv1.CreatePlaylistRequest_builder{Name: new("Mine"), ItemIds: []string{c.season.ID.String()}}.Build())
	wantCode(t, "kid adds a rated season", err, connect.CodeNotFound)
	if _, err := kidPlaylists.CreatePlaylist(ctx, libraryv1.CreatePlaylistRequest_builder{Name: new("Mine"), ItemIds: []string{c.pilot.ID.String()}}.Build()); err != nil {
		t.Fatal(err)
	}
	mine, err := kidPlaylists.ListPlaylists(ctx, &libraryv1.ListPlaylistsRequest{})
	if err != nil || len(mine.GetPlaylists()) != 1 || mine.GetTotal() != 1 {
		t.Fatalf("kid's playlists = %v, %v", mine, err)
	}
	if got := children(kidItems, mine.GetPlaylists()[0].GetId()); !slices.Equal(got, []string{"Pilot"}) {
		t.Errorf("kid's playlist items = %q", got)
	}
	if theirs, err := playlists.ListPlaylists(ctx, &libraryv1.ListPlaylistsRequest{}); err != nil || len(theirs.GetPlaylists()) != 1 || theirs.GetPlaylists()[0].GetName() != "Evening" {
		t.Errorf("admin's playlists = %v, %v", theirs, err)
	}
	if all, err := items.ListItems(ctx, libraryv1.ListItemsRequest_builder{Kinds: []libraryv1.ItemKind{libraryv1.ItemKind_ITEM_KIND_PLAYLIST}}.Build()); err != nil || len(all.GetItems()) != 1 {
		t.Errorf("admin lists playlists = %v, %v", all, err)
	}
	if libs, err := libraryv1connect.NewLibraryServiceClient(http.DefaultClient, url, withToken(kidToken)).ListLibraries(ctx, &libraryv1.ListLibrariesRequest{}); err != nil ||
		len(libs.GetLibraries()) != 2 || !slices.ContainsFunc(libs.GetLibraries(), func(l *libraryv1.Library) bool { return l.GetKind() == libraryv1.LibraryKind_LIBRARY_KIND_PLAYLISTS }) {
		t.Errorf("kid's libraries = %v, %v", libs, err)
	}

	if _, err := playlists.DeletePlaylist(ctx, libraryv1.DeletePlaylistRequest_builder{Id: new(mix.GetId())}.Build()); err != nil {
		t.Fatal(err)
	}
	if _, err := collections.DeleteCollection(ctx, libraryv1.DeleteCollectionRequest_builder{Id: new(saga.GetId())}.Build()); err != nil {
		t.Fatal(err)
	}
	if _, err := items.GetItem(ctx, libraryv1.GetItemRequest_builder{Id: new(c.up.ID.String())}.Build()); err != nil {
		t.Errorf("collection items stay: %v", err)
	}
}
