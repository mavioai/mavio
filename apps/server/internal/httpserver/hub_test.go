package httpserver_test

import (
	"net/http"
	"slices"
	"testing"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/libs/core"
	hubv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/hub/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/hub/v1/hubv1connect"
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
	hubClient := hubv1connect.NewHubServiceClient(http.DefaultClient, url, withToken(token))
	resp, err := hubClient.ListHubs(ctx, hubv1.ListHubsRequest_builder{ItemLimit: new(int32(1))}.Build())
	if err != nil {
		t.Fatalf("ListHubs: %v", err)
	}
	hubID := "top_rated/" + films.GetId()
	if hubs := resp.GetHubs(); len(hubs) != 1 || hubs[0].GetId() != hubID || hubs[0].GetLibraryName() != "Films" ||
		hubs[0].GetKind() != libraryv1.HubKind_HUB_KIND_TOP_RATED || !slices.Equal(names(hubs[0].GetItems()), []string{"High"}) || !hubs[0].GetMore() {
		t.Errorf("hubs = %v, want top rated with High and more", hubs)
	}
	got, err := hubClient.GetHub(ctx, hubv1.GetHubRequest_builder{Id: &hubID, Offset: new(int32(1))}.Build())
	if err != nil {
		t.Fatalf("GetHub: %v", err)
	}
	if !slices.Equal(names(got.GetHub().GetItems()), []string{"Low"}) || got.GetHub().GetMore() {
		t.Errorf("second page = %v, want Low", got.GetHub())
	}
	tab, err := hubClient.ListHubs(ctx, hubv1.ListHubsRequest_builder{LibraryId: new(films.GetId())}.Build())
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
	asKid := hubv1connect.NewHubServiceClient(http.DefaultClient, url, withToken(login(t, url, "kid", "pw", "tablet")))
	_, err = asKid.GetHub(ctx, hubv1.GetHubRequest_builder{Id: &hubID}.Build())
	wantCode(t, "kid opens a hub of another library", err, connect.CodeNotFound)
	_, err = asKid.ListHubs(ctx, hubv1.ListHubsRequest_builder{LibraryId: new(films.GetId())}.Build())
	wantCode(t, "kid lists another library's tab", err, connect.CodeNotFound)

	// The user hides the films' hub; the kid may not pin the films.
	layout := hubv1.HomeLayout_builder{HiddenHubIds: []string{hubID}}.Build()
	if _, err := hubClient.UpdateHomeLayout(ctx, hubv1.UpdateHomeLayoutRequest_builder{Layout: layout}.Build()); err != nil {
		t.Fatalf("UpdateHomeLayout: %v", err)
	}
	if got, err := hubClient.GetHomeLayout(ctx, &hubv1.GetHomeLayoutRequest{}); err != nil || !slices.Equal(got.GetLayout().GetHiddenHubIds(), []string{hubID}) {
		t.Errorf("GetHomeLayout = %v, %v; want %s hidden", got, err, hubID)
	}
	if resp, err := hubClient.ListHubs(ctx, &hubv1.ListHubsRequest{}); err != nil || len(resp.GetHubs()) != 0 {
		t.Errorf("ListHubs with the hub hidden = %v, %v; want none", resp.GetHubs(), err)
	}
	_, err = asKid.UpdateHomeLayout(ctx, hubv1.UpdateHomeLayoutRequest_builder{Layout: hubv1.HomeLayout_builder{
		Pinned: new(true), LibraryIds: []string{films.GetId()},
	}.Build()}.Build())
	wantCode(t, "kid pins another library", err, connect.CodeInvalidArgument)
}
