package rpc

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/apps/server/internal/auth"
	"github.com/mavioai/mavio/libs/core"
	authv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1"
)

// switcher returns the calling session when it may switch users.
func switcher(ctx context.Context) (auth.Principal, error) {
	p, err := principal(ctx)
	if err != nil {
		return p, err
	}
	if !p.Session.CanSwitch(&p.User) {
		return p, connect.NewError(connect.CodePermissionDenied, errors.New("only the administrator's devices switch users"))
	}
	return p, nil
}

// ListSwitchableUsers lists the enabled users the calling session may
// switch to.
func (s *AuthService) ListSwitchableUsers(ctx context.Context, _ *authv1.ListSwitchableUsersRequest) (*authv1.ListSwitchableUsersResponse, error) {
	if err := standaloneOnly(ctx, s.store); err != nil {
		return nil, err
	}
	p, err := switcher(ctx)
	if err != nil {
		return nil, err
	}
	users, err := s.store.Users().List(ctx)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	out := make([]*authv1.SwitchableUser, 0, len(users))
	for i := range users {
		u := &users[i]
		if u.Disabled {
			continue
		}
		creds, err := s.store.Credentials().List(ctx, u.ID)
		if err != nil {
			return nil, connectError(ctx, err)
		}
		out = append(out, authv1.SwitchableUser_builder{
			Id: new(u.ID.String()), Name: &u.Name, Admin: new(u.IsAdmin()), Managed: new(u.Kind == core.UserManaged),
			HasPin:  new(slices.ContainsFunc(creds, func(c core.Credential) bool { return c.Kind == core.CredentialPIN })),
			Current: new(u.ID == p.User.ID),
		}.Build())
	}
	return authv1.ListSwitchableUsersResponse_builder{Users: out}.Build(), nil
}

// SwitchUser makes the calling session another user's, checking that
// user's PIN. Wrong PINs slow down progressively per user.
func (s *AuthService) SwitchUser(ctx context.Context, req *authv1.SwitchUserRequest) (*authv1.SwitchUserResponse, error) {
	if err := standaloneOnly(ctx, s.store); err != nil {
		return nil, err
	}
	p, err := switcher(ctx)
	if err != nil {
		return nil, err
	}
	target, err := s.store.Users().Get(ctx, core.MustParseID(req.GetUserId()))
	if err != nil {
		return nil, connectError(ctx, err)
	}
	if target.Disabled {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New(target.Name+" is disabled"))
	}
	if target.ID != p.User.ID {
		if err := s.checkPIN(ctx, &target, req.GetPin()); err != nil {
			return nil, err
		}
		// The administrator who could switch first stays the switcher.
		by := p.Session.SwitcherID
		if by.IsZero() {
			by = p.User.ID
		}
		if err := s.store.AuthSessions().Switch(ctx, p.Session.ID, target.ID, by); err != nil {
			return nil, connectError(ctx, err)
		}
		s.Activity.Record(ctx, core.Activity{
			Type: "user.switched", UserID: target.ID, Title: p.User.Name + " switched to " + target.Name,
			Attributes: map[string]string{"device": p.Session.DeviceName, "from": p.User.ID.String()},
		})
	}
	described, err := describeUser(ctx, s.store, &target)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return authv1.SwitchUserResponse_builder{User: described}.Build(), nil
}

// checkPIN checks pin against the user's PIN, if it has one.
func (s *AuthService) checkPIN(ctx context.Context, u *core.User, pin string) error {
	stored, err := s.store.Credentials().Get(ctx, u.ID, core.CredentialPIN)
	switch {
	case errors.Is(err, core.ErrNotFound):
		return nil
	case err != nil:
		return connectError(ctx, err)
	case pin == "":
		return connect.NewError(connect.CodeFailedPrecondition, errors.New(u.Name+"'s PIN is required"))
	}
	key := "pin:" + u.ID.String()
	if wait := s.throttle.Wait(key); wait > 0 {
		return connect.NewError(connect.CodeResourceExhausted,
			fmt.Errorf("too many wrong PINs; try again in %s", wait.Round(time.Second)))
	}
	if ok, _, err := auth.VerifyPassword(stored.Data, pin); err != nil || !ok {
		s.throttle.Fail(key)
		return connect.NewError(connect.CodePermissionDenied, errors.New("wrong PIN"))
	}
	s.throttle.Succeed(key)
	return nil
}
