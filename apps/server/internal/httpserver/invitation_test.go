package httpserver_test

import (
	"net/http"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/mavioai/mavio/apps/server/internal/auth/passkeytest"
	authv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1/authv1connect"
	userv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1/userv1connect"
)

func TestInvitations(t *testing.T) {
	ctx := t.Context()
	url := newServer(t)
	const origin = "https://media.example.com"
	asAdmin := userv1connect.NewUserServiceClient(http.DefaultClient, url, withToken(signUp(t, url)))
	public := authv1connect.NewAuthServiceClient(http.DefaultClient, url, fromOrigin(origin))
	day := durationpb.New(24 * time.Hour)

	// An invitation for two members, with preset access.
	created, err := asAdmin.CreateInvitation(ctx, userv1.CreateInvitationRequest_builder{
		MaxUses: new(int32(2)), ValidFor: day,
		Access: userv1.UserPolicy_builder{BlockedLabels: []string{"Horror"}, AllowDownload: new(false)}.Build(),
	}.Build())
	if err != nil || created.GetToken() == "" || created.GetInvitation().GetMaxUses() != 2 {
		t.Fatalf("CreateInvitation = %v, %v", created, err)
	}
	token := created.GetToken()
	if got, err := public.GetInvitation(ctx, authv1.GetInvitationRequest_builder{Token: &token}.Build()); err != nil || got.HasUserName() {
		t.Errorf("GetInvitation = %v, %v; want one creating members", got, err)
	}
	_, err = public.GetInvitation(ctx, authv1.GetInvitationRequest_builder{Token: new("wrong")}.Build())
	wantCode(t, "unknown invitation", err, connect.CodeNotFound)

	accept := func(b authv1.AcceptInvitationRequest_builder) (*authv1.AcceptInvitationResponse, error) {
		b.Token = &token
		if b.Device == nil {
			b.Device = device("joiner")
		}
		return public.AcceptInvitation(ctx, b.Build())
	}
	_, err = accept(authv1.AcceptInvitationRequest_builder{Name: new("carol")})
	wantCode(t, "accept without a credential", err, connect.CodeInvalidArgument)
	_, err = accept(authv1.AcceptInvitationRequest_builder{Password: new("pw")})
	wantCode(t, "accept without a name", err, connect.CodeInvalidArgument)
	_, err = accept(authv1.AcceptInvitationRequest_builder{Name: new("admin"), Password: new("pw")})
	wantCode(t, "accept with a taken name", err, connect.CodeAlreadyExists)

	// One member joins with a password and gets the preset access.
	carol, err := accept(authv1.AcceptInvitationRequest_builder{Name: new("carol"), Password: new("pw")})
	if err != nil {
		t.Fatalf("AcceptInvitation with a password: %v", err)
	}
	if u := carol.GetUser(); u.GetAdmin() || u.GetManaged() || u.GetPolicy().GetAllowDownload() || len(u.GetPolicy().GetBlockedLabels()) != 1 {
		t.Errorf("carol = %v; want a member with the preset access", u)
	}
	if mustCurrent(t, userv1connect.NewUserServiceClient(http.DefaultClient, url, withToken(carol.GetAccessToken()))).GetName() != "carol" {
		t.Error("carol's session signs in someone else")
	}
	login(t, url, "carol", "pw", "carol-laptop")

	// Another joins with a passkey, which then signs it in.
	phone := passkeytest.New(origin)
	start, err := public.StartInvitationPasskey(ctx, authv1.StartInvitationPasskeyRequest_builder{Token: &token, Name: new("dave")}.Build())
	if err != nil {
		t.Fatalf("StartInvitationPasskey: %v", err)
	}
	response, err := phone.Create([]byte(start.GetOptionsJson()))
	if err != nil {
		t.Fatal(err)
	}
	dave, err := accept(authv1.AcceptInvitationRequest_builder{
		Name: new("dave"), CeremonyId: new(start.GetCeremonyId()), CredentialJson: new(string(response)), PasskeyName: new("phone"),
	})
	if err != nil || len(dave.GetUser().GetPasskeys()) != 1 || dave.GetUser().GetPasskeys()[0].GetName() != "phone" {
		t.Fatalf("AcceptInvitation with a passkey = %v, %v", dave, err)
	}
	signIn, err := public.StartPasskeySignIn(ctx, &authv1.StartPasskeySignInRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if response, err = phone.Get([]byte(signIn.GetOptionsJson())); err != nil {
		t.Fatal(err)
	}
	if in, err := public.FinishPasskeySignIn(ctx, authv1.FinishPasskeySignInRequest_builder{
		CeremonyId: new(signIn.GetCeremonyId()), CredentialJson: new(string(response)), Device: device("dave-phone"),
	}.Build()); err != nil || in.GetUser().GetName() != "dave" {
		t.Errorf("dave's passkey sign-in = %v, %v", in, err)
	}

	// The invitation is used up.
	_, err = accept(authv1.AcceptInvitationRequest_builder{Name: new("eve"), Password: new("pw")})
	wantCode(t, "accept a used up invitation", err, connect.CodeNotFound)
	_, err = public.GetInvitation(ctx, authv1.GetInvitationRequest_builder{Token: &token}.Build())
	wantCode(t, "describe a used up invitation", err, connect.CodeNotFound)
	if list, err := asAdmin.ListInvitations(ctx, &userv1.ListInvitationsRequest{}); err != nil || len(list.GetInvitations()) != 1 || list.GetInvitations()[0].GetUses() != 2 {
		t.Errorf("ListInvitations = %v, %v", list, err)
	}

	// An invitation converts a managed user, which keeps its ID.
	kid, err := asAdmin.CreateUser(ctx, userv1.CreateUserRequest_builder{Name: new("kid"), Managed: new(true)}.Build())
	if err != nil {
		t.Fatal(err)
	}
	_, err = asAdmin.CreateInvitation(ctx, userv1.CreateInvitationRequest_builder{UserId: new(carol.GetUser().GetId()), ValidFor: day}.Build())
	wantCode(t, "invite a member", err, connect.CodeFailedPrecondition)
	convert, err := asAdmin.CreateInvitation(ctx, userv1.CreateInvitationRequest_builder{UserId: new(kid.GetUser().GetId()), ValidFor: day}.Build())
	if err != nil {
		t.Fatal(err)
	}
	token = convert.GetToken()
	if got, err := public.GetInvitation(ctx, authv1.GetInvitationRequest_builder{Token: &token}.Build()); err != nil || got.GetUserName() != "kid" {
		t.Errorf("GetInvitation = %v, %v; want kid's", got, err)
	}
	grown, err := accept(authv1.AcceptInvitationRequest_builder{Password: new("grown"), Device: device("kid-phone")})
	if err != nil || grown.GetUser().GetId() != kid.GetUser().GetId() || grown.GetUser().GetManaged() || grown.GetUser().GetName() != "kid" {
		t.Fatalf("converting AcceptInvitation = %v, %v", grown, err)
	}
	login(t, url, "kid", "grown", "kid-laptop")

	// Revoked invitations stop working.
	revoked, err := asAdmin.CreateInvitation(ctx, userv1.CreateInvitationRequest_builder{ValidFor: day}.Build())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := asAdmin.DeleteInvitation(ctx, userv1.DeleteInvitationRequest_builder{Id: new(revoked.GetInvitation().GetId())}.Build()); err != nil {
		t.Fatal(err)
	}
	token = revoked.GetToken()
	_, err = accept(authv1.AcceptInvitationRequest_builder{Name: new("eve"), Password: new("pw")})
	wantCode(t, "accept a revoked invitation", err, connect.CodeNotFound)
	_, err = userv1connect.NewUserServiceClient(http.DefaultClient, url, withToken(carol.GetAccessToken())).
		CreateInvitation(ctx, userv1.CreateInvitationRequest_builder{ValidFor: day}.Build())
	wantCode(t, "member creates an invitation", err, connect.CodePermissionDenied)
}

func TestSwitchUsers(t *testing.T) {
	ctx := t.Context()
	url := newServer(t)
	adminToken := signUp(t, url)
	asAdmin := userv1connect.NewUserServiceClient(http.DefaultClient, url, withToken(adminToken))
	switching := authv1connect.NewAuthServiceClient(http.DefaultClient, url, withToken(adminToken))
	kid, err := asAdmin.CreateUser(ctx, userv1.CreateUserRequest_builder{Name: new("kid"), Managed: new(true), Pin: new("1234")}.Build())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := asAdmin.CreateUser(ctx, userv1.CreateUserRequest_builder{Name: new("bob"), Password: new("pw")}.Build()); err != nil {
		t.Fatal(err)
	}
	if _, err := asAdmin.SetPin(ctx, userv1.SetPinRequest_builder{UserId: new(mustCurrent(t, asAdmin).GetId()), Pin: new("9999")}.Build()); err != nil {
		t.Fatal(err)
	}
	bobToken := login(t, url, "bob", "pw", "bob-phone")

	list, err := switching.ListSwitchableUsers(ctx, &authv1.ListSwitchableUsersRequest{})
	if err != nil || len(list.GetUsers()) != 3 {
		t.Fatalf("ListSwitchableUsers = %v, %v", list, err)
	}
	for _, u := range list.GetUsers() {
		if u.GetCurrent() != (u.GetName() == "admin") || u.GetHasPin() != (u.GetName() != "bob") {
			t.Errorf("switchable user %v", u)
		}
	}
	asBob := authv1connect.NewAuthServiceClient(http.DefaultClient, url, withToken(bobToken))
	_, err = asBob.ListSwitchableUsers(ctx, &authv1.ListSwitchableUsersRequest{})
	wantCode(t, "member lists switchable users", err, connect.CodePermissionDenied)
	_, err = asBob.SwitchUser(ctx, authv1.SwitchUserRequest_builder{UserId: new(kid.GetUser().GetId())}.Build())
	wantCode(t, "member switches", err, connect.CodePermissionDenied)

	switchTo := func(id, pin string) (*authv1.SwitchUserResponse, error) {
		b := authv1.SwitchUserRequest_builder{UserId: &id}
		if pin != "" {
			b.Pin = &pin
		}
		return switching.SwitchUser(ctx, b.Build())
	}
	kidID := kid.GetUser().GetId()
	_, err = switchTo(kidID, "")
	wantCode(t, "switch without a PIN", err, connect.CodeFailedPrecondition)
	_, err = switchTo(kidID, "0000")
	wantCode(t, "switch with a wrong PIN", err, connect.CodePermissionDenied)
	if got, err := switchTo(kidID, "1234"); err != nil || got.GetUser().GetName() != "kid" {
		t.Fatalf("SwitchUser = %v, %v", got, err)
	}
	// The same token now acts as the kid, which can switch back.
	if mustCurrent(t, asAdmin).GetName() != "kid" {
		t.Error("the switched session still acts as the administrator")
	}
	list, err = switching.ListSwitchableUsers(ctx, &authv1.ListSwitchableUsersRequest{})
	if err != nil || len(list.GetUsers()) != 3 {
		t.Fatalf("ListSwitchableUsers after switching = %v, %v", list, err)
	}
	_, err = asAdmin.ListUsers(ctx, &userv1.ListUsersRequest{})
	wantCode(t, "switched session manages users", err, connect.CodePermissionDenied)
	adminID := ""
	for _, u := range list.GetUsers() {
		if u.GetAdmin() {
			adminID = u.GetId()
		}
	}
	_, err = switchTo(adminID, "")
	wantCode(t, "switch back without the administrator's PIN", err, connect.CodeFailedPrecondition)
	if got, err := switchTo(adminID, "9999"); err != nil || !got.GetUser().GetAdmin() {
		t.Fatalf("switch back = %v, %v", got, err)
	}

	// Wrong PINs lock the user out progressively, after three free ones.
	for range 4 {
		_, err = switchTo(kidID, "0000")
		wantCode(t, "wrong PIN", err, connect.CodePermissionDenied)
	}
	_, err = switchTo(kidID, "1234")
	wantCode(t, "switch while locked out", err, connect.CodeResourceExhausted)
}
