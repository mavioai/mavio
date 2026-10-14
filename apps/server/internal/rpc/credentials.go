package rpc

import (
	"context"
	"errors"
	"slices"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/apps/server/internal/auth"
	"github.com/mavioai/mavio/libs/core"
	userv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1"
)

// totpIssuer names the server in authenticator apps.
const totpIssuer = "Mavio"

// StartPasskeyRegistration begins creating a passkey of the caller at the
// request's origin.
func (s *UserService) StartPasskeyRegistration(ctx context.Context, _ *userv1.StartPasskeyRegistrationRequest) (*userv1.StartPasskeyRegistrationResponse, error) {
	if s.Passkeys == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("passkeys are unavailable"))
	}
	u, err := s.ownCredentials(ctx, false)
	if err != nil {
		return nil, err
	}
	rp, err := requestRelyingParty(ctx)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	creds, err := s.store.Credentials().List(ctx, u.ID)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	id, options, err := s.Passkeys.StartRegistration(rp, u, creds)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return userv1.StartPasskeyRegistrationResponse_builder{CeremonyId: &id, OptionsJson: new(string(options))}.Build(), nil
}

// FinishPasskeyRegistration stores the caller's new passkey.
func (s *UserService) FinishPasskeyRegistration(ctx context.Context, req *userv1.FinishPasskeyRegistrationRequest) (*userv1.FinishPasskeyRegistrationResponse, error) {
	if s.Passkeys == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("passkeys are unavailable"))
	}
	u, err := s.ownCredentials(ctx, false)
	if err != nil {
		return nil, err
	}
	data, err := s.Passkeys.FinishRegistration(req.GetCeremonyId(), u, []byte(req.GetCredentialJson()))
	if errors.Is(err, auth.ErrPasskey) {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	} else if err != nil {
		return nil, connectError(ctx, err)
	}
	c := core.Credential{UserID: u.ID, Kind: core.CredentialPasskey, Name: req.GetName(), Data: data}
	if err := s.store.Credentials().Put(ctx, &c); err != nil {
		return nil, connectError(ctx, err)
	}
	p, _ := principal(ctx)
	s.accountChanged(ctx, p, "user.passkey_added", "added a passkey to", u)
	return userv1.FinishPasskeyRegistrationResponse_builder{Passkey: passkeyToProto(&c)}.Build(), nil
}

// DeletePasskey removes a passkey of the caller, or of anyone for the
// administrator.
func (s *UserService) DeletePasskey(ctx context.Context, req *userv1.DeletePasskeyRequest) (*userv1.DeletePasskeyResponse, error) {
	p, u, err := s.selfOrAdmin(ctx, req.GetUserId())
	if err != nil {
		return nil, err
	}
	creds, err := s.store.Credentials().List(ctx, u.ID)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	id := core.MustParseID(req.GetId())
	if !slices.ContainsFunc(creds, func(c core.Credential) bool { return c.ID == id && c.Kind == core.CredentialPasskey }) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no such passkey"))
	}
	if err := s.store.Credentials().Remove(ctx, id); err != nil {
		return nil, connectError(ctx, err)
	}
	s.accountChanged(ctx, p, "user.passkey_removed", "removed a passkey of", u)
	return &userv1.DeletePasskeyResponse{}, nil
}

// StartTotp gives the caller a new TOTP secret to confirm.
func (s *UserService) StartTotp(ctx context.Context, _ *userv1.StartTotpRequest) (*userv1.StartTotpResponse, error) {
	if s.TOTP == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("TOTP is unavailable"))
	}
	u, err := s.ownCredentials(ctx, true)
	if err != nil {
		return nil, err
	}
	secret := s.TOTP.Enroll(u.ID)
	return userv1.StartTotpResponse_builder{
		Secret: &secret, Uri: new(auth.TOTPURI(totpIssuer, u.Name, secret)),
	}.Build(), nil
}

// ConfirmTotp turns the caller's TOTP on with a code of its new secret.
func (s *UserService) ConfirmTotp(ctx context.Context, req *userv1.ConfirmTotpRequest) (*userv1.ConfirmTotpResponse, error) {
	if s.TOTP == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("TOTP is unavailable"))
	}
	u, err := s.ownCredentials(ctx, true)
	if err != nil {
		return nil, err
	}
	sealed, used, ok := s.TOTP.Confirm(u.ID, req.GetCode())
	if !ok {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("wrong code, or no TOTP is being set up"))
	}
	c := core.Credential{UserID: u.ID, Kind: core.CredentialTOTP, Data: sealed, LastUsedAt: &used}
	if err := s.store.Credentials().Put(ctx, &c); err != nil {
		return nil, connectError(ctx, err)
	}
	p, _ := principal(ctx)
	s.accountChanged(ctx, p, "user.totp_enabled", "turned TOTP on for", u)
	return &userv1.ConfirmTotpResponse{}, nil
}

// DeleteTotp turns a user's TOTP off: the caller's, or anyone's for the
// administrator.
func (s *UserService) DeleteTotp(ctx context.Context, req *userv1.DeleteTotpRequest) (*userv1.DeleteTotpResponse, error) {
	p, u, err := s.selfOrAdmin(ctx, req.GetUserId())
	if err != nil {
		return nil, err
	}
	if err := s.store.Credentials().Delete(ctx, u.ID, core.CredentialTOTP); err != nil {
		return nil, connectError(ctx, err)
	}
	s.accountChanged(ctx, p, "user.totp_disabled", "turned TOTP off for", u)
	return &userv1.DeleteTotpResponse{}, nil
}

// ownCredentials returns the caller, who must be able to have passkeys,
// or TOTP when totp is set: not a managed user, and for passkeys not a
// user of an authentication plugin, which checks the user's password at
// every sign-in.
func (s *UserService) ownCredentials(ctx context.Context, totp bool) (core.User, error) {
	p, err := principal(ctx)
	if err != nil {
		return core.User{}, err
	}
	u, err := s.store.Users().Get(ctx, p.User.ID)
	if err != nil {
		return core.User{}, connectError(ctx, err)
	}
	switch {
	case u.Kind == core.UserManaged:
		return core.User{}, connect.NewError(connect.CodeFailedPrecondition, errors.New("managed users have no credentials"))
	case !totp && u.AuthProvider != "":
		return core.User{}, connect.NewError(connect.CodeFailedPrecondition, errors.New("the user signs in through "+u.AuthProvider))
	}
	return u, nil
}

// selfOrAdmin returns the caller and the user with ID id, when it is the
// caller or the caller is the administrator.
func (s *UserService) selfOrAdmin(ctx context.Context, id string) (auth.Principal, core.User, error) {
	p, err := principal(ctx)
	if err != nil {
		return p, core.User{}, err
	}
	userID := core.MustParseID(id)
	if userID != p.User.ID && !p.User.IsAdmin() {
		return p, core.User{}, connect.NewError(connect.CodePermissionDenied, errors.New("administrator required"))
	}
	u, err := s.store.Users().Get(ctx, userID)
	if err != nil {
		return p, core.User{}, connectError(ctx, err)
	}
	return p, u, nil
}
