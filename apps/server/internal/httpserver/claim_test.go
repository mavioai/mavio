package httpserver_test

import (
	"crypto/ed25519"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/apps/server/internal/cloud"
	"github.com/mavioai/mavio/apps/server/internal/cloud/cloudtest"
	"github.com/mavioai/mavio/apps/server/internal/httpserver"
	"github.com/mavioai/mavio/apps/server/internal/identity"
	authv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1/authv1connect"
	cloudv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/cloud/v1"
)

// claimer has the handler sign Mavio accounts in through the simulated
// mavio.dev, with its identity key.
func claimer(o *httpserver.Options, dev *cloudtest.Server, key *identity.Key) {
	o.SignIn = &cloud.Claimer{Store: o.Store, Client: cloud.NewClient(dev.URL, key, nil)}
}

// startSignIn starts signing in through StartSignIn and returns the
// authorization request the server pushed, with its ID.
func startSignIn(t *testing.T, public authv1connect.AuthServiceClient, dev *cloudtest.Server) (string, *cloudv1.CreateAuthorizationRequestRequest) {
	t.Helper()
	resp, err := public.StartSignIn(t.Context(), authv1.StartSignInRequest_builder{
		RedirectUri: new("http://192.168.1.10:8686/auth/callback"), CodeChallenge: new(strings.Repeat("c", 43)),
		DeviceKey: device("browser").GetPublicKey(), State: new("state"),
	}.Build())
	if err != nil {
		t.Fatalf("StartSignIn: %v", err)
	}
	u, err := url.Parse(resp.GetUrl())
	if err != nil {
		t.Fatal(err)
	}
	id := u.Query().Get("request_uri")
	r, ok := dev.AuthorizationRequest(id)
	if !ok || r.GetRedirectUri() != "http://192.168.1.10:8686/auth/callback" || r.GetState() != "state" {
		t.Fatalf("authorization request %q = %v, %v; want the web UI's redirect URI and state", id, r, ok)
	}
	return id, r
}

// claim claims the server as the web UI does: it signs in through
// StartSignIn, has account approve the request at the simulated
// mavio.dev, and signs the account in with an assertion, which completes
// the claim.
func claim(t *testing.T, public authv1connect.AuthServiceClient, dev *cloudtest.Server, key *identity.Key, account *cloudv1.Account) {
	t.Helper()
	ctx := t.Context()
	id, r := startSignIn(t, public, dev)
	if !r.GetClaim() {
		t.Fatalf("authorization request = %v, want one claiming the server", r)
	}
	dev.ApproveAuthorizationRequest(id, account)
	d := device("browser")
	token := dev.Assertion(key.ID(), account.GetId(), d.GetPublicKey())
	// A server without an assertion verifier claims itself all the same.
	_, _ = public.SignInWithAssertion(ctx, authv1.SignInWithAssertionRequest_builder{
		Assertion: &token, Device: d, Signature: ed25519.Sign(deviceKey("browser"), cloud.AssertionMessage(token)),
	}.Build())
	info, err := public.GetAuthInfo(ctx, &authv1.GetAuthInfoRequest{})
	if err != nil || info.GetSetupRequired() {
		t.Fatalf("GetAuthInfo after signing in = %v, %v; want the server claimed", info, err)
	}
}

func TestStartSignIn(t *testing.T) {
	ctx := t.Context()
	dev := cloudtest.New(t)
	key := identity.Generate()
	h, _ := newHandler(t, nil, func(o *httpserver.Options) { claimer(o, dev, key) })
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	public := authv1connect.NewAuthServiceClient(http.DefaultClient, srv.URL)

	claim(t, public, dev, key, cloudv1.Account_builder{Id: new("acct-1"), UserName: new("alice")}.Build())
	info, err := public.GetAuthInfo(ctx, &authv1.GetAuthInfoRequest{})
	if err != nil || info.GetSetupRequired() || info.GetMode() != authv1.AccountMode_ACCOUNT_MODE_MAVIO_DEV {
		t.Errorf("GetAuthInfo after claiming = %v, %v; want mavio.dev mode, set up", info, err)
	}
	if _, r := startSignIn(t, public, dev); r.GetClaim() {
		t.Errorf("authorization request of a claimed server = %v, want one that does not claim it", r)
	}
}

func TestStartSignInStandalone(t *testing.T) {
	dev := cloudtest.New(t)
	h, _ := newHandler(t, nil, func(o *httpserver.Options) { claimer(o, dev, identity.Generate()) })
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	public := authv1connect.NewAuthServiceClient(http.DefaultClient, srv.URL)
	if _, err := public.CreateFirstUser(t.Context(), authv1.CreateFirstUserRequest_builder{
		Name: new("admin"), Password: new("password"), Device: device("browser"),
	}.Build()); err != nil {
		t.Fatal(err)
	}
	_, err := public.StartSignIn(t.Context(), authv1.StartSignInRequest_builder{
		RedirectUri: new("http://localhost:8686/auth/callback"), CodeChallenge: new(strings.Repeat("c", 43)),
		DeviceKey: device("browser").GetPublicKey(),
	}.Build())
	wantCode(t, "StartSignIn of a standalone server", err, connect.CodeFailedPrecondition)
}

func TestStartSignInUnavailable(t *testing.T) {
	h, _ := newHandler(t, nil, func(o *httpserver.Options) {
		o.SignIn = &cloud.Claimer{Store: o.Store, Client: cloud.NewClient("http://127.0.0.1:1/api", identity.Generate(), nil)}
	})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	_, err := authv1connect.NewAuthServiceClient(http.DefaultClient, srv.URL).StartSignIn(t.Context(), authv1.StartSignInRequest_builder{
		RedirectUri: new("http://localhost:8686/auth/callback"), CodeChallenge: new(strings.Repeat("c", 43)),
		DeviceKey: device("browser").GetPublicKey(),
	}.Build())
	wantCode(t, "StartSignIn with mavio.dev unreachable", err, connect.CodeUnavailable)
}
