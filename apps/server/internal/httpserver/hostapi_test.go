package httpserver_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/apps/server/internal/auth"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/plugin/manifest"
	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1/libraryv1connect"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	userv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1/userv1connect"
)

// TestHostAPI calls the handler tree as a plugin does through the host API:
// within its manifest's scopes, as itself or as a user.
func TestHostAPI(t *testing.T) {
	ctx := t.Context()
	h, _ := newHandler(t, nil)
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	adminToken := signUp(t, srv.URL)

	m := pluginv1.Manifest_builder{
		Id: new("org.example.p"),
		Permissions: pluginv1.Permissions_builder{
			Api: []string{
				"mavio.library.v1.LibraryService:read", "mavio.user.v1.UserService", "mavio.user.v1.UserDataService",
			},
			ActAsUsers: new(true),
		}.Build(),
	}.Build()
	grant := auth.Plugin{
		ID:         m.GetId(),
		Allows:     func(p string, readOnly bool) bool { return manifest.AllowsProcedure(m, p, readOnly) },
		ActAsUsers: true,
	}
	plugin := httptest.NewServer(auth.PluginHandler(grant, h))
	t.Cleanup(plugin.Close)

	code := func(err error) connect.Code {
		if err == nil {
			return 0
		}
		return connect.CodeOf(err)
	}
	asUser := func(name string) connect.ClientOption {
		return connect.WithInterceptors(connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
			return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
				req.Header().Set(auth.UserHeader, name)
				return next(ctx, req)
			}
		}))
	}
	libraries := libraryv1connect.NewLibraryServiceClient(http.DefaultClient, plugin.URL)
	users := userv1connect.NewUserServiceClient(http.DefaultClient, plugin.URL)
	// A token the plugin sends is ignored: it still acts as itself.
	usersWithToken := userv1connect.NewUserServiceClient(http.DefaultClient, plugin.URL, withToken(adminToken))
	userData := userv1connect.NewUserDataServiceClient(http.DefaultClient, plugin.URL)
	asAdmin := userv1connect.NewUserDataServiceClient(http.DefaultClient, plugin.URL, asUser("ADMIN"))
	itemIDs := &userv1.GetUserDataRequest{}
	itemIDs.SetItemIds([]string{core.NewID().String()})

	calls := []struct {
		name string
		call func() error
		want connect.Code
	}{
		{"read scope", func() error { _, err := libraries.ListLibraries(ctx, &libraryv1.ListLibrariesRequest{}); return err }, 0},
		{"write beyond read scope", func() error {
			_, err := libraries.DeleteLibrary(ctx, libraryv1.DeleteLibraryRequest_builder{Id: new(core.NewID().String())}.Build())
			return err
		}, connect.CodePermissionDenied},
		{"service out of scope", func() error {
			_, err := libraryv1connect.NewItemServiceClient(http.DefaultClient, plugin.URL).ListItems(ctx, &libraryv1.ListItemsRequest{})
			return err
		}, connect.CodePermissionDenied},
		{"administration as itself", func() error { _, err := users.ListUsers(ctx, &userv1.ListUsersRequest{}); return err }, 0},
		{"own state as itself", func() error { _, err := userData.GetUserData(ctx, itemIDs); return err }, connect.CodeFailedPrecondition},
		{"current user with a token", func() error {
			_, err := usersWithToken.GetCurrentUser(ctx, &userv1.GetCurrentUserRequest{})
			return err
		}, connect.CodeFailedPrecondition},
		{"own state as a user", func() error { _, err := asAdmin.GetUserData(ctx, itemIDs); return err }, 0},
		{"unknown user", func() error {
			_, err := userv1connect.NewUserDataServiceClient(http.DefaultClient, plugin.URL, asUser("nobody")).GetUserData(ctx, itemIDs)
			return err
		}, connect.CodeUnauthenticated},
	}
	for _, c := range calls {
		if got := code(c.call()); got != c.want {
			t.Errorf("%s: code %v, want %v", c.name, got, c.want)
		}
	}
	me, err := userv1connect.NewUserServiceClient(http.DefaultClient, plugin.URL, asUser("admin")).GetCurrentUser(ctx, &userv1.GetCurrentUserRequest{})
	if err != nil || me.GetUser().GetName() != "admin" {
		t.Errorf("GetCurrentUser as admin = %v, %v", me, err)
	}

	// Plain HTTP routes check the scopes too.
	resp, err := http.Get(plugin.URL + "/system/backups/none.zip")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("backup download without the scope: %d, want 403", resp.StatusCode)
	}
}
