package rpc

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mavioai/mavio/apps/server/internal/auth"
	"github.com/mavioai/mavio/libs/core"
	authv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1"
	userv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1"
)

// CreateInvitation makes an invitation link creating members, or
// converting a managed user into one.
func (s *UserService) CreateInvitation(ctx context.Context, req *userv1.CreateInvitationRequest) (*userv1.CreateInvitationResponse, error) {
	if err := standaloneOnly(ctx, s.store); err != nil {
		return nil, err
	}
	p, err := admin(ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	token, hash := auth.NewToken()
	inv := core.Invitation{
		TokenHash: hash,
		Access:    core.LibraryAccess{AllowDownload: true},
		MaxUses:   max(int(req.GetMaxUses()), 1),
		ExpiresAt: now.Add(req.GetValidFor().AsDuration()),
		CreatedAt: now,
	}
	title := p.User.Name + " created an invitation"
	if req.HasUserId() {
		u, err := s.store.Users().Get(ctx, core.MustParseID(req.GetUserId()))
		if err != nil {
			return nil, connectError(ctx, err)
		}
		if u.Kind != core.UserManaged {
			return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("only managed users are converted through invitations"))
		}
		if req.GetMaxUses() > 1 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("an invitation of a managed user is used once"))
		}
		inv.UserID, inv.Access = u.ID, u.Access
		title = p.User.Name + " invited " + u.Name + " to become a member"
	} else if req.HasAccess() {
		_, inv.Access = policyFromProto(req.GetAccess(), core.RequestPermissions{})
	}
	if err := s.store.Invitations().Create(ctx, &inv); err != nil {
		return nil, connectError(ctx, err)
	}
	s.Activity.Record(ctx, core.Activity{Type: "invitation.created", UserID: p.User.ID, Title: title})
	return userv1.CreateInvitationResponse_builder{Invitation: invitationToProto(&inv), Token: &token}.Build(), nil
}

// ListInvitations lists the invitations that have not expired.
func (s *UserService) ListInvitations(ctx context.Context, _ *userv1.ListInvitationsRequest) (*userv1.ListInvitationsResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	list, err := s.store.Invitations().List(ctx)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	now := time.Now()
	out := make([]*userv1.Invitation, 0, len(list))
	for i := range list {
		if now.Before(list[i].ExpiresAt) {
			out = append(out, invitationToProto(&list[i]))
		}
	}
	return userv1.ListInvitationsResponse_builder{Invitations: out}.Build(), nil
}

// DeleteInvitation revokes an invitation.
func (s *UserService) DeleteInvitation(ctx context.Context, req *userv1.DeleteInvitationRequest) (*userv1.DeleteInvitationResponse, error) {
	p, err := admin(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.store.Invitations().Delete(ctx, core.MustParseID(req.GetId())); err != nil {
		return nil, connectError(ctx, err)
	}
	s.Activity.Record(ctx, core.Activity{Type: "invitation.deleted", UserID: p.User.ID, Title: p.User.Name + " revoked an invitation"})
	return &userv1.DeleteInvitationResponse{}, nil
}

func invitationToProto(inv *core.Invitation) *userv1.Invitation {
	out := userv1.Invitation_builder{
		Id:         new(inv.ID.String()),
		Access:     accessToProto(&inv.Access),
		MaxUses:    new(int32(inv.MaxUses)),
		Uses:       new(int32(inv.Uses)),
		ExpireTime: timestamppb.New(inv.ExpiresAt),
		CreateTime: timestamppb.New(inv.CreatedAt),
	}.Build()
	if !inv.UserID.IsZero() {
		out.SetUserId(inv.UserID.String())
	}
	return out
}

// accessToProto converts a library access record into the fields of a
// policy that hold it.
func accessToProto(a *core.LibraryAccess) *userv1.UserPolicy {
	libraries := make([]string, len(a.Libraries))
	for i, id := range a.Libraries {
		libraries[i] = id.String()
	}
	return userv1.UserPolicy_builder{
		AllLibraries:      new(a.Libraries == nil),
		LibraryIds:        libraries,
		MaxParentalRating: int32Ptr(a.MaxParentalRating),
		BlockUnrated:      &a.BlockUnrated,
		AllowedLabels:     a.AllowedLabels,
		BlockedLabels:     a.BlockedLabels,
		AllowDownload:     &a.AllowDownload,
	}.Build()
}

// errNoInvitation answers for unknown, used up and expired invitations
// alike.
var errNoInvitation = connect.NewError(connect.CodeNotFound, errors.New("the invitation is invalid, used up or expired"))

// invitation returns the usable invitation of token.
func (s *AuthService) invitation(ctx context.Context, token string) (core.Invitation, error) {
	inv, err := s.store.Invitations().GetByTokenHash(ctx, auth.HashToken(token))
	switch {
	case errors.Is(err, core.ErrNotFound):
		return inv, errNoInvitation
	case err != nil:
		return inv, connectError(ctx, err)
	case !inv.Usable(s.now()):
		return inv, errNoInvitation
	}
	return inv, nil
}

// GetInvitation describes an invitation before it is accepted.
func (s *AuthService) GetInvitation(ctx context.Context, req *authv1.GetInvitationRequest) (*authv1.GetInvitationResponse, error) {
	if err := standaloneOnly(ctx, s.store); err != nil {
		return nil, err
	}
	inv, err := s.invitation(ctx, req.GetToken())
	if err != nil {
		return nil, err
	}
	out := authv1.GetInvitationResponse_builder{ExpireTime: timestamppb.New(inv.ExpiresAt)}.Build()
	if !inv.UserID.IsZero() {
		u, err := s.store.Users().Get(ctx, inv.UserID)
		if err != nil {
			return nil, connectError(ctx, err)
		}
		out.SetUserName(u.Name)
	}
	return out, nil
}

// StartInvitationPasskey begins creating the passkey of the member an
// invitation creates or converts.
func (s *AuthService) StartInvitationPasskey(ctx context.Context, req *authv1.StartInvitationPasskeyRequest) (*authv1.StartInvitationPasskeyResponse, error) {
	if err := standaloneOnly(ctx, s.store); err != nil {
		return nil, err
	}
	if s.Passkeys == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("passkeys are unavailable"))
	}
	inv, err := s.invitation(ctx, req.GetToken())
	if err != nil {
		return nil, err
	}
	// A new member's ID is chosen now, so that the passkey names it.
	u := core.User{ID: core.NewID(), Name: req.GetName()}
	if !inv.UserID.IsZero() {
		if u, err = s.store.Users().Get(ctx, inv.UserID); err != nil {
			return nil, connectError(ctx, err)
		}
		if req.GetName() != "" {
			u.Name = req.GetName()
		}
	} else if u.Name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("a name is required"))
	}
	rp, err := requestRelyingParty(ctx)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	id, options, err := s.Passkeys.StartEnrollment(rp, u, inv.ID)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return authv1.StartInvitationPasskeyResponse_builder{CeremonyId: &id, OptionsJson: new(string(options))}.Build(), nil
}

// AcceptInvitation creates a member, or converts the invitation's managed
// user into one, with a password or a passkey, and signs the device in.
func (s *AuthService) AcceptInvitation(ctx context.Context, req *authv1.AcceptInvitationRequest) (*authv1.AcceptInvitationResponse, error) {
	if err := standaloneOnly(ctx, s.store); err != nil {
		return nil, err
	}
	passkey := req.HasCeremonyId() || req.HasCredentialJson()
	switch {
	case passkey && req.HasPassword(), !passkey && !req.HasPassword():
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("either a password or a passkey is required"))
	case passkey && (!req.HasCeremonyId() || !req.HasCredentialJson()):
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("a passkey requires its ceremony and the authenticator's response"))
	case passkey && s.Passkeys == nil:
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("passkeys are unavailable"))
	}
	inv, err := s.invitation(ctx, req.GetToken())
	if err != nil {
		return nil, err
	}
	converting := !inv.UserID.IsZero()
	if !converting && !req.HasName() {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("a name is required"))
	}
	var cred core.Credential
	userID := inv.UserID
	if passkey {
		id, data, err := s.Passkeys.FinishEnrollment(req.GetCeremonyId(), inv.ID, []byte(req.GetCredentialJson()))
		switch {
		case errors.Is(err, auth.ErrPasskey):
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		case err != nil:
			return nil, connectError(ctx, err)
		case converting && id != inv.UserID:
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("the passkey is of another user"))
		}
		userID = id
		cred = core.Credential{Kind: core.CredentialPasskey, Name: req.GetPasskeyName(), Data: data}
	} else {
		cred = core.Credential{Kind: core.CredentialPassword, Data: auth.HashPassword(req.GetPassword())}
	}

	var (
		user core.User
		sess core.AuthSession
	)
	err = s.store.InTx(ctx, func(tx core.Store) error {
		now := s.now()
		if err := tx.Invitations().Use(ctx, inv.ID, now); errors.Is(err, core.ErrNotFound) {
			return errNoInvitation
		} else if err != nil {
			return err
		}
		if converting {
			var err error
			if user, err = tx.Users().Get(ctx, inv.UserID); err != nil {
				return err
			}
			if user.Kind != core.UserManaged {
				return connect.NewError(connect.CodeFailedPrecondition, errors.New(user.Name+" is no longer a managed user"))
			}
			user.Kind, user.Restriction, user.LastLoginAt = core.UserMember, core.RestrictionNone, &now
			if req.HasName() {
				user.Name = req.GetName()
			}
			if err := tx.Users().Update(ctx, &user); err != nil {
				return err
			}
		} else {
			requests := core.DefaultRequestSettings()
			if s.Requests != nil {
				requests = s.Requests()
			}
			user = core.User{
				ID:          userID,
				Name:        req.GetName(),
				Kind:        core.UserMember,
				Access:      inv.Access,
				Policy:      core.UserPolicy{AllowTranscoding: true, Requests: requests.DefaultPermissions},
				CreatedAt:   now,
				LastLoginAt: &now,
			}
			if err := tx.Users().Create(ctx, &user); err != nil {
				return err
			}
		}
		cred.UserID = user.ID
		if err := tx.Credentials().Put(ctx, &cred); err != nil {
			return err
		}
		var err error
		sess, err = s.createSession(ctx, tx, user.ID, req.GetDevice())
		return err
	})
	if err != nil {
		return nil, connectError(ctx, err)
	}
	title := user.Name + " joined through an invitation"
	if converting {
		title = user.Name + " became a member through an invitation"
	}
	s.Activity.Record(ctx, core.Activity{
		Type: "invitation.accepted", UserID: user.ID, Title: title,
		Attributes: map[string]string{"invitation": inv.ID.String(), "device": req.GetDevice().GetName()},
	})
	described, err := describeUser(ctx, s.store, &user)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	token, expires := s.tokens.Issue(sess.ID)
	return authv1.AcceptInvitationResponse_builder{
		AccessToken:           &token,
		AccessTokenExpireTime: timestamppb.New(expires),
		User:                  described,
		Session:               sessionToProto(&sess, true),
	}.Build(), nil
}
