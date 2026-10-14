package httpserver_test

import (
	"crypto/ed25519"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/apps/server/internal/cloud"
	"github.com/mavioai/mavio/apps/server/internal/cloud/cloudtest"
	"github.com/mavioai/mavio/apps/server/internal/httpserver"
	"github.com/mavioai/mavio/apps/server/internal/identity"
	"github.com/mavioai/mavio/libs/core"
	authv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1/authv1connect"
	cloudv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/cloud/v1"
	userv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1/userv1connect"
)

func TestMavioDevMode(t *testing.T) {
	ctx := t.Context()
	dev := cloudtest.New(t)
	key := identity.Generate()
	h, st := newHandler(t, nil, func(o *httpserver.Options) {
		o.Claim = (&cloud.Claimer{Store: o.Store, Client: cloud.NewClient(dev.URL, key, nil)}).Claim
		o.Assertions = cloud.NewVerifier(dev.URL, key.ID(), cloud.NewKeySet(dev.URL, nil))
		o.MavioDevURL = dev.URL
	})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	public := authv1connect.NewAuthServiceClient(http.DefaultClient, srv.URL)
	signIn := func(account, deviceID string) (*authv1.SignInWithAssertionResponse, error) {
		d := device(deviceID)
		token := dev.Assertion(key.ID(), account, d.GetPublicKey())
		return public.SignInWithAssertion(ctx, authv1.SignInWithAssertionRequest_builder{
			Assertion: &token, Device: d, Signature: ed25519.Sign(deviceKey(deviceID), cloud.AssertionMessage(token)),
		}.Build())
	}

	// A standalone server signs in with its own credentials only.
	_, err := signIn("acct-1", "phone")
	wantCode(t, "assertion on a standalone server", err, connect.CodeFailedPrecondition)
	if info, err := public.GetAuthInfo(ctx, &authv1.GetAuthInfoRequest{}); err != nil || info.HasMavioDevUrl() {
		t.Errorf("GetAuthInfo of a standalone server = %v, %v; want no mavio.dev URL", info, err)
	}

	dev.AddClaimCode("code", cloudv1.Account_builder{Id: new("acct-1"), UserName: new("alice")}.Build())
	if _, err := public.ClaimServer(ctx, authv1.ClaimServerRequest_builder{ClaimCode: new("code")}.Build()); err != nil {
		t.Fatal(err)
	}
	if info, err := public.GetAuthInfo(ctx, &authv1.GetAuthInfoRequest{}); err != nil || info.GetMavioDevUrl() != dev.URL {
		t.Errorf("GetAuthInfo in mavio.dev mode = %v, %v; want the mavio.dev URL", info, err)
	}

	// The administrator signs in with an assertion of its account.
	in, err := signIn("acct-1", "phone")
	if err != nil || in.GetUser().GetName() != "alice" || !in.GetUser().GetAdmin() || in.GetAccessToken() == "" {
		t.Fatalf("SignInWithAssertion = %v, %v; want alice", in, err)
	}
	asAdmin := userv1connect.NewUserServiceClient(http.DefaultClient, srv.URL, withToken(in.GetAccessToken()))

	// An account without an access record has no user.
	_, err = signIn("acct-2", "tv")
	wantCode(t, "account without access", err, connect.CodeUnauthenticated)
	apply := func(c *cloudv1.Change) {
		t.Helper()
		if err := st.InTx(ctx, func(tx core.Store) error { return (&cloud.Applier{}).Apply(ctx, tx, c) }); err != nil {
			t.Fatal(err)
		}
	}
	apply(cloudv1.Change_builder{AccessGranted: cloudv1.AccessGranted_builder{
		Account: cloudv1.Account_builder{Id: new("acct-2"), UserName: new("bob")}.Build(),
		Access:  cloudv1.Access_builder{AllLibraries: new(true)}.Build(),
	}.Build()}.Build())
	bob, err := signIn("acct-2", "tv")
	if err != nil || bob.GetUser().GetName() != "bob" {
		t.Fatalf("SignInWithAssertion with access = %v, %v; want bob", bob, err)
	}
	bobID := bob.GetUser().GetId()

	// The assertion binds the device that signs it.
	d := device("tablet")
	token := dev.Assertion(key.ID(), "acct-2", device("other").GetPublicKey())
	_, err = public.SignInWithAssertion(ctx, authv1.SignInWithAssertionRequest_builder{
		Assertion: &token, Device: d, Signature: ed25519.Sign(deviceKey("tablet"), cloud.AssertionMessage(token)),
	}.Build())
	wantCode(t, "assertion of another device", err, connect.CodeUnauthenticated)

	// Accounts are mavio.dev's: the server neither creates users nor signs in with passwords.
	_, err = asAdmin.CreateUser(ctx, userv1.CreateUserRequest_builder{Name: new("carol"), Password: new("pw")}.Build())
	wantCode(t, "CreateUser", err, connect.CodeFailedPrecondition)
	_, err = public.Login(ctx, authv1.LoginRequest_builder{Name: new("alice"), Password: new("pw"), Device: device("web")}.Build())
	wantCode(t, "Login", err, connect.CodeFailedPrecondition)
	_, err = asAdmin.UpdateUser(ctx, userv1.UpdateUserRequest_builder{Id: &bobID, Name: new("robert")}.Build())
	wantCode(t, "rename an account's user", err, connect.CodeFailedPrecondition)
	_, err = asAdmin.UpdateUser(ctx, userv1.UpdateUserRequest_builder{Id: &bobID, Name: new("bob"), Disabled: new(true)}.Build())
	wantCode(t, "disable an account's user", err, connect.CodeFailedPrecondition)

	// The server's own policy stays the server's; access stays mavio.dev's.
	updated, err := asAdmin.UpdateUser(ctx, userv1.UpdateUserRequest_builder{
		Id: &bobID, Name: new("bob"), Policy: userv1.UserPolicy_builder{AllowTranscoding: new(false), MaxParentalRating: new(int32(7))}.Build(),
	}.Build())
	if pol := updated.GetUser().GetPolicy(); err != nil || pol.GetAllowTranscoding() || pol.HasMaxParentalRating() {
		t.Errorf("UpdateUser policy = %v, %v; want transcoding off, the rating from mavio.dev", pol, err)
	}
	_, err = asAdmin.DeleteUser(ctx, userv1.DeleteUserRequest_builder{Id: &bobID}.Build())
	wantCode(t, "delete a user with access", err, connect.CodeFailedPrecondition)

	// Revoked access signs the account out and lets the administrator purge its user.
	apply(cloudv1.Change_builder{AccessRevoked: cloudv1.AccessRevoked_builder{AccountId: new("acct-2")}.Build()}.Build())
	_, err = signIn("acct-2", "tv")
	wantCode(t, "revoked account", err, connect.CodeUnauthenticated)
	asBob := userv1connect.NewUserServiceClient(http.DefaultClient, srv.URL, withToken(bob.GetAccessToken()))
	_, err = asBob.GetCurrentUser(ctx, &userv1.GetCurrentUserRequest{})
	wantCode(t, "revoked account's token", err, connect.CodeUnauthenticated)
	if _, err := asAdmin.DeleteUser(ctx, userv1.DeleteUserRequest_builder{Id: &bobID}.Build()); err != nil {
		t.Errorf("DeleteUser of a revoked account: %v", err)
	}
}
