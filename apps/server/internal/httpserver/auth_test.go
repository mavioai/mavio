package httpserver_test

import (
	"net/http"
	"testing"

	"connectrpc.com/connect"

	authv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1/authv1connect"
)

func device(id string) *authv1.Device {
	return authv1.Device_builder{Id: &id, Name: new("Test " + id), Client: new("Mavio Test"), ClientVersion: new("1.0")}.Build()
}

// signUp creates the first user, "admin" with password "secret", and
// returns its token.
func signUp(t *testing.T, url string) string {
	t.Helper()
	resp, err := authv1connect.NewAuthServiceClient(http.DefaultClient, url).CreateFirstUser(t.Context(),
		authv1.CreateFirstUserRequest_builder{Name: new("admin"), Password: new("secret"), Device: device("setup")}.Build())
	if err != nil {
		t.Fatalf("CreateFirstUser: %v", err)
	}
	return resp.GetAccessToken()
}

func TestAuthFlow(t *testing.T) {
	ctx := t.Context()
	url := newServer(t)
	public := authv1connect.NewAuthServiceClient(http.DefaultClient, url)
	wantCode := func(what string, err error, want connect.Code) {
		t.Helper()
		if got := connect.CodeOf(err); err == nil || got != want {
			t.Errorf("%s: err = %v, want code %v", what, err, want)
		}
	}

	info, err := public.GetAuthInfo(ctx, &authv1.GetAuthInfoRequest{})
	if err != nil || !info.GetSetupRequired() {
		t.Fatalf("GetAuthInfo before setup = %v, %v; want setup required", info, err)
	}

	first, err := public.CreateFirstUser(ctx, authv1.CreateFirstUserRequest_builder{
		Name: new("admin"), Password: new("secret"), Device: device("setup"),
	}.Build())
	if err != nil {
		t.Fatalf("CreateFirstUser: %v", err)
	}
	if u := first.GetUser(); u.GetName() != "admin" || !u.GetAdmin() || !u.GetPolicy().GetAllLibraries() || !first.GetSession().GetCurrent() {
		t.Errorf("CreateFirstUser = %v", first)
	}
	_, err = public.CreateFirstUser(ctx, authv1.CreateFirstUserRequest_builder{
		Name: new("other"), Password: new("x"), Device: device("setup"),
	}.Build())
	wantCode("second CreateFirstUser", err, connect.CodeFailedPrecondition)
	if info, err := public.GetAuthInfo(ctx, &authv1.GetAuthInfoRequest{}); err != nil || info.GetSetupRequired() {
		t.Errorf("GetAuthInfo after setup = %v, %v; want no setup", info, err)
	}

	login := func(name, password, deviceID string) (*authv1.LoginResponse, error) {
		return public.Login(ctx, authv1.LoginRequest_builder{Name: &name, Password: &password, Device: device(deviceID)}.Build())
	}
	_, err = login("admin", "wrong", "phone")
	wantCode("wrong password", err, connect.CodeUnauthenticated)
	_, err = login("nobody", "secret", "phone")
	wantCode("unknown user", err, connect.CodeUnauthenticated)
	_, err = public.Login(ctx, authv1.LoginRequest_builder{Name: new("admin"), Password: new("secret")}.Build())
	wantCode("login without a device", err, connect.CodeInvalidArgument)

	phone, err := login("ADMIN", "secret", "phone")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if !phone.GetUser().HasLastLoginTime() || phone.GetAccessToken() == first.GetAccessToken() {
		t.Errorf("Login = %v", phone)
	}
	asPhone := authv1connect.NewAuthServiceClient(http.DefaultClient, url, withToken(phone.GetAccessToken()))
	asSetup := authv1connect.NewAuthServiceClient(http.DefaultClient, url, withToken(first.GetAccessToken()))

	list, err := asPhone.ListSessions(ctx, &authv1.ListSessionsRequest{})
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	current := map[string]bool{}
	for _, s := range list.GetSessions() {
		current[s.GetDevice().GetId()] = s.GetCurrent()
	}
	if len(current) != 2 || !current["phone"] || current["setup"] {
		t.Errorf("ListSessions = %v; want the phone current and the setup device", list)
	}

	// Signing in again from the phone revokes its old token.
	again, err := login("admin", "secret", "phone")
	if err != nil {
		t.Fatal(err)
	}
	_, err = asPhone.ListSessions(ctx, &authv1.ListSessionsRequest{})
	wantCode("replaced token", err, connect.CodeUnauthenticated)
	asPhone = authv1connect.NewAuthServiceClient(http.DefaultClient, url, withToken(again.GetAccessToken()))

	_, err = asPhone.RevokeSession(ctx, authv1.RevokeSessionRequest_builder{Id: new("01890a5d-ac96-774b-bcce-b302099a8057")}.Build())
	wantCode("revoke unknown session", err, connect.CodeNotFound)
	if _, err := asPhone.RevokeSession(ctx, authv1.RevokeSessionRequest_builder{Id: new(first.GetSession().GetId())}.Build()); err != nil {
		t.Fatalf("RevokeSession: %v", err)
	}
	_, err = asSetup.ListSessions(ctx, &authv1.ListSessionsRequest{})
	wantCode("revoked token", err, connect.CodeUnauthenticated)

	if _, err := asPhone.Logout(ctx, &authv1.LogoutRequest{}); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	_, err = asPhone.ListSessions(ctx, &authv1.ListSessionsRequest{})
	wantCode("after Logout", err, connect.CodeUnauthenticated)
	_, err = public.Logout(ctx, &authv1.LogoutRequest{})
	wantCode("Logout without a token", err, connect.CodeUnauthenticated)
}
