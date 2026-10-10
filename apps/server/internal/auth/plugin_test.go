package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store"
)

func TestPluginRequests(t *testing.T) {
	ctx := t.Context()
	s, err := store.Open(ctx, "sqlite:"+filepath.Join(t.TempDir(), "mavio.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	for _, u := range []core.User{
		{Name: "alice", PasswordHash: HashPassword("pw")},
		{Name: "bob", PasswordHash: HashPassword("pw"), Disabled: true},
	} {
		if err := s.Users().Create(ctx, &u); err != nil {
			t.Fatal(err)
		}
	}

	i := NewInterceptor(s, "/test.v1.Public/Get").
		RequireUser("mavio.user.v1.UserDataService", "/mavio.user.v1.UserService/GetCurrentUser").
		RequireSession("mavio.session.v1.EventService")
	scopes := map[string]bool{
		"mavio.library.v1.ItemService":  true, // read only
		"mavio.user.v1.UserDataService": false,
		"mavio.user.v1.UserService":     false,
		"mavio.session.v1.EventService": false,
	}
	allows := func(procedure string, readOnly bool) bool {
		for service, readOnlyScope := range scopes {
			if len(procedure) > len(service)+2 && procedure[1:len(service)+1] == service && (!readOnlyScope || readOnly) {
				return true
			}
		}
		return false
	}

	tests := []struct {
		name      string
		actAs     bool
		procedure string
		readOnly  bool
		user      string
		want      connect.Code // 0 for success
		wantUser  string
	}{
		{"read in scope", false, "/mavio.library.v1.ItemService/GetItem", true, "", 0, "plugin org.example.p"},
		{"write beyond read scope", false, "/mavio.library.v1.ItemService/UpdateItem", false, "", connect.CodePermissionDenied, ""},
		{"service out of scope", false, "/mavio.library.v1.LibraryService/ListLibraries", true, "", connect.CodePermissionDenied, ""},
		{"public still needs scope", false, "/test.v1.Public/Get", true, "", connect.CodePermissionDenied, ""},
		{"admin method as itself", false, "/mavio.user.v1.UserService/ListUsers", true, "", 0, "plugin org.example.p"},
		{"user state as itself", false, "/mavio.user.v1.UserDataService/GetUserData", true, "", connect.CodeFailedPrecondition, ""},
		{"current user as itself", false, "/mavio.user.v1.UserService/GetCurrentUser", true, "", connect.CodeFailedPrecondition, ""},
		{"user without act-as", false, "/mavio.user.v1.UserDataService/GetUserData", true, "alice", connect.CodePermissionDenied, ""},
		{"act as user", true, "/mavio.user.v1.UserDataService/GetUserData", true, "ALICE", 0, "alice"},
		{"act as disabled user", true, "/mavio.user.v1.UserDataService/GetUserData", true, "bob", connect.CodeUnauthenticated, ""},
		{"act as unknown user", true, "/mavio.user.v1.UserDataService/GetUserData", true, "carol", connect.CodeUnauthenticated, ""},
		{"session procedure", true, "/mavio.session.v1.EventService/Subscribe", false, "alice", connect.CodeFailedPrecondition, ""},
	}
	for _, tt := range tests {
		g := Plugin{ID: "org.example.p", Allows: allows, ActAsUsers: tt.actAs}
		h := http.Header{}
		if tt.user != "" {
			h.Set(UserHeader, tt.user)
		}
		spec := connect.Spec{Procedure: tt.procedure}
		if tt.readOnly {
			spec.IdempotencyLevel = connect.IdempotencyNoSideEffects
		}
		got, err := i.authenticate(context.WithValue(ctx, pluginKey{}, g), spec, h)
		if code := connect.CodeOf(err); err != nil && code != tt.want || err == nil && tt.want != 0 {
			t.Errorf("%s: err = %v, want code %v", tt.name, err, tt.want)
			continue
		}
		if err != nil {
			continue
		}
		p, _ := FromContext(got)
		if p.User.Name != tt.wantUser || p.Plugin != g.ID || !p.Session.ID.IsZero() {
			t.Errorf("%s: principal = %+v, want user %q of plugin %s", tt.name, p, tt.wantUser, g.ID)
		}
		if tt.user == "" && (!p.User.Admin || !p.User.ID.IsZero()) {
			t.Errorf("%s: plugin principal = %+v, want an administrator without ID", tt.name, p.User)
		}
	}
}

func TestPluginHandlerDropsCredentials(t *testing.T) {
	g := Plugin{ID: "org.example.p", Allows: func(string, bool) bool { return true }}
	var seen *http.Request
	h := PluginHandler(g, http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { seen = r }))
	req := httptest.NewRequest(http.MethodPost, "/x", nil)
	req.Header.Set("Authorization", "Bearer someone-elses")
	req.Header.Set("Cookie", "a=b")
	h.ServeHTTP(httptest.NewRecorder(), req)
	if seen.Header.Get("Authorization") != "" || seen.Header.Get("Cookie") != "" {
		t.Errorf("credentials reached the handler: %v", seen.Header)
	}
	if got, ok := seen.Context().Value(pluginKey{}).(Plugin); !ok || got.ID != g.ID {
		t.Errorf("plugin in context = %+v, %v", got, ok)
	}
}
