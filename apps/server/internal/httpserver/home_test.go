package httpserver_test

import (
	"net/http"
	"slices"
	"testing"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/libs/core"
	homev1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/home/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/home/v1/homev1connect"
	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1/libraryv1connect"
	userv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1/userv1connect"
)

func TestHomeHubs(t *testing.T) {
	ctx := t.Context()
	url, s := startServer(t, nil)
	token := signUp(t, url)
	libraries := libraryv1connect.NewLibraryServiceClient(http.DefaultClient, url, withToken(token))
	created, err := libraries.CreateLibrary(ctx, libraryv1.CreateLibraryRequest_builder{Spec: libraryv1.LibrarySpec_builder{
		Name: new("Films"), Kind: new(libraryv1.LibraryKind_LIBRARY_KIND_MOVIES), Paths: []string{t.TempDir()},
		Hubs: libraryv1.LibraryHubs_builder{
			Home: libraryv1.HubList_builder{Kinds: []libraryv1.HubKind{libraryv1.HubKind_HUB_KIND_TOP_RATED}}.Build(),
		}.Build(),
	}.Build()}.Build())
	if err != nil {
		t.Fatalf("CreateLibrary: %v", err)
	}
	films := created.GetLibrary()
	if got := films.GetHubs().GetHome().GetKinds(); !slices.Equal(got, []libraryv1.HubKind{libraryv1.HubKind_HUB_KIND_TOP_RATED}) || films.GetHubs().HasRecommended() {
		t.Errorf("hubs = %v, recommended %v; want top rated on Home, the default tab", got, films.GetHubs().HasRecommended())
	}
	_, err = libraries.CreateLibrary(ctx, libraryv1.CreateLibraryRequest_builder{Spec: libraryv1.LibrarySpec_builder{
		Name: new("Music"), Kind: new(libraryv1.LibraryKind_LIBRARY_KIND_MUSIC), Paths: []string{t.TempDir()},
		Hubs: libraryv1.LibraryHubs_builder{
			Home: libraryv1.HubList_builder{Kinds: []libraryv1.HubKind{libraryv1.HubKind_HUB_KIND_TOP_RATED}}.Build(),
		}.Build(),
	}.Build()}.Build())
	wantCode(t, "a hub music libraries lack", err, connect.CodeInvalidArgument)

	lib := core.MustParseID(films.GetId())
	paths := t.TempDir()
	for i, name := range []string{"Low", "High"} {
		it := core.Item{ID: core.NewID(), LibraryID: lib, Kind: core.KindMovie, Name: name, Path: paths + "/" + name, CommunityRating: float64(5 + i)}
		if err := s.Items().Upsert(ctx, it); err != nil {
			t.Fatal(err)
		}
	}
	home := homev1connect.NewHomeServiceClient(http.DefaultClient, url, withToken(token))
	resp, err := home.ListHubs(ctx, homev1.ListHubsRequest_builder{ItemLimit: new(int32(1))}.Build())
	if err != nil {
		t.Fatalf("ListHubs: %v", err)
	}
	hubID := "top_rated/" + films.GetId()
	if hubs := resp.GetHubs(); len(hubs) != 1 || hubs[0].GetId() != hubID || hubs[0].GetLibraryName() != "Films" ||
		hubs[0].GetKind() != libraryv1.HubKind_HUB_KIND_TOP_RATED || !slices.Equal(names(hubs[0].GetItems()), []string{"High"}) || !hubs[0].GetMore() {
		t.Errorf("hubs = %v, want top rated with High and more", hubs)
	}
	got, err := home.GetHub(ctx, homev1.GetHubRequest_builder{Id: &hubID, Offset: new(int32(1))}.Build())
	if err != nil {
		t.Fatalf("GetHub: %v", err)
	}
	if !slices.Equal(names(got.GetHub().GetItems()), []string{"Low"}) || got.GetHub().GetMore() {
		t.Errorf("second page = %v, want Low", got.GetHub())
	}
	tab, err := home.ListHubs(ctx, homev1.ListHubsRequest_builder{LibraryId: new(films.GetId())}.Build())
	if err != nil {
		t.Fatalf("ListHubs of the tab: %v", err)
	}
	if len(tab.GetHubs()) < 3 {
		t.Errorf("tab hubs = %d, want the default hubs with items", len(tab.GetHubs()))
	}

	users := userv1connect.NewUserServiceClient(http.DefaultClient, url, withToken(token))
	if _, err := users.CreateUser(ctx, userv1.CreateUserRequest_builder{
		Name: new("kid"), Password: new("pw"), Policy: userv1.UserPolicy_builder{LibraryIds: []string{}}.Build(),
	}.Build()); err != nil {
		t.Fatal(err)
	}
	asKid := homev1connect.NewHomeServiceClient(http.DefaultClient, url, withToken(login(t, url, "kid", "pw", "tablet")))
	_, err = asKid.GetHub(ctx, homev1.GetHubRequest_builder{Id: &hubID}.Build())
	wantCode(t, "kid opens a hub of another library", err, connect.CodeNotFound)
	_, err = asKid.ListHubs(ctx, homev1.ListHubsRequest_builder{LibraryId: new(films.GetId())}.Build())
	wantCode(t, "kid lists another library's tab", err, connect.CodeNotFound)
}
