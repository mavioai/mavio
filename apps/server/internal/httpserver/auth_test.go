package httpserver_test

import (
	"crypto/ed25519"
	"crypto/sha256"
	"net/http"
	"testing"
	"time"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/apps/server/internal/auth"
	"github.com/mavioai/mavio/libs/core"
	authv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1/authv1connect"
	userv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1/userv1connect"
)

// device returns a test device whose key is derived from its ID.
func device(id string) *authv1.Device {
	return authv1.Device_builder{
		Id: &id, Name: new("Test " + id), Client: new("Mavio Test"), ClientVersion: new("1.0"),
		PublicKey: deviceKey(id).Public().(ed25519.PublicKey),
	}.Build()
}

// deviceKey returns the private key of device(id).
func deviceKey(id string) ed25519.PrivateKey {
	seed := sha256.Sum256([]byte(id))
	return ed25519.NewKeyFromSeed(seed[:])
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
	if err != nil || !info.GetSetupRequired() || info.GetMode() != authv1.AccountMode_ACCOUNT_MODE_UNSPECIFIED {
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
	if info, err := public.GetAuthInfo(ctx, &authv1.GetAuthInfoRequest{}); err != nil || info.GetSetupRequired() || info.GetMode() != authv1.AccountMode_ACCOUNT_MODE_STANDALONE {
		t.Errorf("GetAuthInfo after setup = %v, %v; want standalone", info, err)
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

func TestQuickConnect(t *testing.T) {
	ctx := t.Context()
	url := newServer(t)
	token := signUp(t, url)
	anon := authv1connect.NewAuthServiceClient(http.DefaultClient, url)
	signedIn := authv1connect.NewAuthServiceClient(http.DefaultClient, url, withToken(token))

	// The TV starts without credentials and shows the code.
	started, err := anon.StartQuickConnect(ctx, authv1.StartQuickConnectRequest_builder{Device: device("tv")}.Build())
	if err != nil || len(started.GetCode()) != 6 || started.GetSecret() == "" {
		t.Fatalf("StartQuickConnect = %v, %v", started, err)
	}
	state, err := anon.GetQuickConnectState(ctx, authv1.GetQuickConnectStateRequest_builder{Secret: new(started.GetSecret())}.Build())
	if err != nil || state.GetAuthorized() {
		t.Errorf("state = %v, %v; want pending", state, err)
	}
	_, err = anon.LoginWithQuickConnect(ctx, authv1.LoginWithQuickConnectRequest_builder{Secret: new(started.GetSecret())}.Build())
	wantCode(t, "login before authorizing", err, connect.CodeNotFound)
	_, err = anon.AuthorizeQuickConnect(ctx, authv1.AuthorizeQuickConnectRequest_builder{Code: new(started.GetCode())}.Build())
	wantCode(t, "authorize without signing in", err, connect.CodeUnauthenticated)
	_, err = signedIn.AuthorizeQuickConnect(ctx, authv1.AuthorizeQuickConnectRequest_builder{Code: new("12345")}.Build())
	wantCode(t, "malformed code", err, connect.CodeInvalidArgument)

	// The signed-in phone enters the code; the TV signs in as its user.
	authorized, err := signedIn.AuthorizeQuickConnect(ctx, authv1.AuthorizeQuickConnectRequest_builder{Code: new(started.GetCode())}.Build())
	if err != nil || authorized.GetDevice().GetId() != "tv" {
		t.Fatalf("AuthorizeQuickConnect = %v, %v", authorized, err)
	}
	if state, err := anon.GetQuickConnectState(ctx, authv1.GetQuickConnectStateRequest_builder{Secret: new(started.GetSecret())}.Build()); err != nil || !state.GetAuthorized() {
		t.Errorf("state = %v, %v; want authorized", state, err)
	}
	login, err := anon.LoginWithQuickConnect(ctx, authv1.LoginWithQuickConnectRequest_builder{Secret: new(started.GetSecret())}.Build())
	if err != nil || login.GetUser().GetName() != "admin" || login.GetSession().GetDevice().GetId() != "tv" {
		t.Fatalf("LoginWithQuickConnect = %v, %v", login, err)
	}
	me, err := userv1connect.NewUserServiceClient(http.DefaultClient, url, withToken(login.GetAccessToken())).
		GetCurrentUser(ctx, &userv1.GetCurrentUserRequest{})
	if err != nil || me.GetUser().GetName() != "admin" {
		t.Errorf("TV's user = %v, %v", me, err)
	}
	_, err = anon.LoginWithQuickConnect(ctx, authv1.LoginWithQuickConnectRequest_builder{Secret: new(started.GetSecret())}.Build())
	wantCode(t, "second login", err, connect.CodeNotFound)
}

func TestRefreshSession(t *testing.T) {
	ctx := t.Context()
	url, s := startServer(t, nil)
	public := authv1connect.NewAuthServiceClient(http.DefaultClient, url)
	first, err := public.CreateFirstUser(ctx, authv1.CreateFirstUserRequest_builder{
		Name: new("admin"), Password: new("secret"), Device: device("setup"),
	}.Build())
	if err != nil {
		t.Fatalf("CreateFirstUser: %v", err)
	}
	if exp := first.GetAccessTokenExpireTime().AsTime(); exp.Before(time.Now().Add(50*time.Minute)) || exp.After(time.Now().Add(time.Hour+time.Minute)) {
		t.Errorf("access token expires at %v, want in an hour", exp)
	}
	id := first.GetSession().GetId()
	refresh := func(key ed25519.PrivateKey) (*authv1.RefreshSessionResponse, error) {
		start, err := public.StartRefresh(ctx, authv1.StartRefreshRequest_builder{SessionId: &id}.Build())
		if err != nil {
			return nil, err
		}
		sessionID := core.MustParseID(id)
		sig := ed25519.Sign(key, auth.RefreshMessage(sessionID, start.GetNonce()))
		return public.RefreshSession(ctx, authv1.RefreshSessionRequest_builder{
			SessionId: &id, Nonce: start.GetNonce(), Signature: sig,
		}.Build())
	}

	got, err := refresh(deviceKey("setup"))
	if err != nil {
		t.Fatalf("RefreshSession: %v", err)
	}
	asSetup := authv1connect.NewAuthServiceClient(http.DefaultClient, url, withToken(got.GetAccessToken()))
	if _, err := asSetup.ListSessions(ctx, &authv1.ListSessionsRequest{}); err != nil {
		t.Errorf("ListSessions with the refreshed token: %v", err)
	}

	tests := []struct {
		name  string
		setup func()
		key   ed25519.PrivateKey
	}{
		{"another device's key", func() {}, deviceKey("phone")},
		{"idle session", func() {
			if err := s.AuthSessions().Touch(ctx, core.MustParseID(id), time.Now().AddDate(0, 0, -91)); err != nil {
				t.Fatal(err)
			}
		}, deviceKey("setup")},
		{"disabled user", func() {
			if err := s.AuthSessions().Touch(ctx, core.MustParseID(id), time.Now()); err != nil {
				t.Fatal(err)
			}
			u, err := s.Users().GetByName(ctx, "admin")
			if err != nil {
				t.Fatal(err)
			}
			u.Disabled = true
			if err := s.Users().Update(ctx, &u); err != nil {
				t.Fatal(err)
			}
		}, deviceKey("setup")},
	}
	for _, tt := range tests {
		tt.setup()
		if _, err := refresh(tt.key); connect.CodeOf(err) != connect.CodeUnauthenticated {
			t.Errorf("%s: err = %v, want unauthenticated", tt.name, err)
		}
	}

	// A nonce is used once.
	start, err := public.StartRefresh(ctx, authv1.StartRefreshRequest_builder{SessionId: &id}.Build())
	if err != nil {
		t.Fatal(err)
	}
	req := authv1.RefreshSessionRequest_builder{
		SessionId: &id, Nonce: start.GetNonce(), Signature: make([]byte, ed25519.SignatureSize),
	}.Build()
	_, _ = public.RefreshSession(ctx, req)
	req.SetSignature(ed25519.Sign(deviceKey("setup"), auth.RefreshMessage(core.MustParseID(id), start.GetNonce())))
	if _, err := public.RefreshSession(ctx, req); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Errorf("reused nonce: err = %v, want unauthenticated", err)
	}
	_, err = public.StartRefresh(ctx, authv1.StartRefreshRequest_builder{SessionId: new(core.NewID().String())}.Build())
	if connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Errorf("StartRefresh of an unknown session: err = %v, want unauthenticated", err)
	}
}
