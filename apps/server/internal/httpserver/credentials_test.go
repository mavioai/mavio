package httpserver_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/http"
	"testing"
	"time"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/apps/server/internal/auth/passkeytest"
	authv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1/authv1connect"
	userv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1/userv1connect"
)

// fromOrigin sends origin as the Origin header of every request, as a
// browser at it would.
func fromOrigin(origin string) connect.ClientOption {
	return connect.WithInterceptors(connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			req.Header().Set("Origin", origin)
			return next(ctx, req)
		}
	}))
}

// totpCode is the TOTP code of a base32 secret at a time (RFC 6238).
func totpCode(t *testing.T, secret string, at time.Time) string {
	t.Helper()
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	if err != nil {
		t.Fatal(err)
	}
	h := hmac.New(sha1.New, key)
	_ = binary.Write(h, binary.BigEndian, at.Unix()/30)
	sum := h.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	return fmt.Sprintf("%06d", binary.BigEndian.Uint32(sum[offset:])&0x7fffffff%1_000_000)
}

func TestPasskeysAndTOTP(t *testing.T) {
	ctx := t.Context()
	url := newServer(t)
	const origin = "https://media.example.com"
	adminToken := signUp(t, url)
	asAdmin := userv1connect.NewUserServiceClient(http.DefaultClient, url, withToken(adminToken))
	bob, err := asAdmin.CreateUser(ctx, userv1.CreateUserRequest_builder{Name: new("bob"), Password: new("pw")}.Build())
	if err != nil {
		t.Fatal(err)
	}
	bobID := bob.GetUser().GetId()
	bobToken := login(t, url, "bob", "pw", "bob-laptop")
	asBob := userv1connect.NewUserServiceClient(http.DefaultClient, url, withToken(bobToken), fromOrigin(origin))
	signIn := authv1connect.NewAuthServiceClient(http.DefaultClient, url, fromOrigin(origin))

	// Passkeys are created at an origin, for the administrator and members.
	_, err = userv1connect.NewUserServiceClient(http.DefaultClient, url, withToken(bobToken)).
		StartPasskeyRegistration(ctx, &userv1.StartPasskeyRegistrationRequest{})
	wantCode(t, "passkey without an origin", err, connect.CodeInvalidArgument)
	_, err = userv1connect.NewUserServiceClient(http.DefaultClient, url, withToken(bobToken), fromOrigin("http://192.168.1.10:8096")).
		StartPasskeyRegistration(ctx, &userv1.StartPasskeyRegistrationRequest{})
	wantCode(t, "passkey at an IP address", err, connect.CodeInvalidArgument)
	register := func(users userv1connect.UserServiceClient, a *passkeytest.Authenticator, name string) *userv1.Passkey {
		t.Helper()
		start, err := users.StartPasskeyRegistration(ctx, &userv1.StartPasskeyRegistrationRequest{})
		if err != nil {
			t.Fatalf("StartPasskeyRegistration: %v", err)
		}
		response, err := a.Create([]byte(start.GetOptionsJson()))
		if err != nil {
			t.Fatal(err)
		}
		done, err := users.FinishPasskeyRegistration(ctx, userv1.FinishPasskeyRegistrationRequest_builder{
			CeremonyId: new(start.GetCeremonyId()), Name: &name, CredentialJson: new(string(response)),
		}.Build())
		if err != nil {
			t.Fatalf("FinishPasskeyRegistration: %v", err)
		}
		return done.GetPasskey()
	}
	bobsPhone, adminsPhone := passkeytest.New(origin), passkeytest.New(origin)
	passkey := register(asBob, bobsPhone, "phone")
	if passkey.GetName() != "phone" || passkey.GetRpId() != "media.example.com" {
		t.Errorf("passkey = %v", passkey)
	}
	register(userv1connect.NewUserServiceClient(http.DefaultClient, url, withToken(adminToken), fromOrigin(origin)), adminsPhone, "phone")
	if me, err := asBob.GetCurrentUser(ctx, &userv1.GetCurrentUserRequest{}); err != nil || len(me.GetUser().GetPasskeys()) != 1 {
		t.Errorf("GetCurrentUser = %v, %v", me, err)
	}

	// A passkey signs its user in, without a name or password.
	withPasskey := func(a *passkeytest.Authenticator) (*authv1.FinishPasskeySignInResponse, error) {
		t.Helper()
		start, err := signIn.StartPasskeySignIn(ctx, &authv1.StartPasskeySignInRequest{})
		if err != nil {
			t.Fatalf("StartPasskeySignIn: %v", err)
		}
		response, err := a.Get([]byte(start.GetOptionsJson()))
		if err != nil {
			t.Fatal(err)
		}
		return signIn.FinishPasskeySignIn(ctx, authv1.FinishPasskeySignInRequest_builder{
			CeremonyId: new(start.GetCeremonyId()), CredentialJson: new(string(response)), Device: device("phone"),
		}.Build())
	}
	for name, a := range map[string]*passkeytest.Authenticator{"admin": adminsPhone, "bob": bobsPhone} {
		resp, err := withPasskey(a)
		if err != nil || resp.GetUser().GetName() != name {
			t.Fatalf("passkey sign-in as %s = %v, %v", name, resp, err)
		}
		if mustCurrent(t, userv1connect.NewUserServiceClient(http.DefaultClient, url, withToken(resp.GetAccessToken()))).GetName() != name {
			t.Errorf("passkey session of %s signs in someone else", name)
		}
	}

	// TOTP is a second factor to passwords.
	start, err := asBob.StartTotp(ctx, &userv1.StartTotpRequest{})
	if err != nil {
		t.Fatalf("StartTotp: %v", err)
	}
	now := time.Now()
	wrong := "000000"
	if wrong == totpCode(t, start.GetSecret(), now) {
		wrong = "111111"
	}
	_, err = asBob.ConfirmTotp(ctx, userv1.ConfirmTotpRequest_builder{Code: &wrong}.Build())
	wantCode(t, "confirming with a wrong code", err, connect.CodeInvalidArgument)
	if _, err := asBob.ConfirmTotp(ctx, userv1.ConfirmTotpRequest_builder{Code: new(totpCode(t, start.GetSecret(), now))}.Build()); err != nil {
		t.Fatalf("ConfirmTotp: %v", err)
	}
	if me, err := asBob.GetCurrentUser(ctx, &userv1.GetCurrentUserRequest{}); err != nil || !me.GetUser().GetHasTotp() {
		t.Errorf("GetCurrentUser after TOTP = %v, %v", me, err)
	}
	password := func(code string) error {
		req := authv1.LoginRequest_builder{Name: new("bob"), Password: new("pw"), Device: device("bob-tv")}.Build()
		if code != "" {
			req.SetTotpCode(code)
		}
		_, err := signIn.Login(ctx, req)
		return err
	}
	wantCode(t, "password without a TOTP code", password(""), connect.CodeFailedPrecondition)
	wantCode(t, "a used TOTP code", password(totpCode(t, start.GetSecret(), now)), connect.CodeUnauthenticated)
	if err := password(totpCode(t, start.GetSecret(), now.Add(30*time.Second))); err != nil {
		t.Errorf("Login with the next TOTP code: %v", err)
	}
	if resp, err := withPasskey(bobsPhone); err != nil || resp.GetUser().GetName() != "bob" {
		t.Errorf("passkey sign-in with TOTP on = %v, %v", resp, err)
	}
	if _, err := asAdmin.DeleteTotp(ctx, userv1.DeleteTotpRequest_builder{UserId: &bobID}.Build()); err != nil {
		t.Fatalf("DeleteTotp by the administrator: %v", err)
	}
	if err := password(""); err != nil {
		t.Errorf("Login after TOTP was turned off: %v", err)
	}

	// A removed passkey signs nobody in.
	_, err = asBob.DeletePasskey(ctx, userv1.DeletePasskeyRequest_builder{UserId: new(mustCurrent(t, asAdmin).GetId()), Id: new(passkey.GetId())}.Build())
	wantCode(t, "deleting another user's passkey", err, connect.CodePermissionDenied)
	if _, err := asBob.DeletePasskey(ctx, userv1.DeletePasskeyRequest_builder{UserId: &bobID, Id: new(passkey.GetId())}.Build()); err != nil {
		t.Fatalf("DeletePasskey: %v", err)
	}
	_, err = withPasskey(bobsPhone)
	wantCode(t, "sign-in with a removed passkey", err, connect.CodeUnauthenticated)
}
