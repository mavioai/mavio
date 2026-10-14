package rpc

import (
	"bytes"
	"context"
	"errors"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mavioai/mavio/apps/server/internal/cloud"
	"github.com/mavioai/mavio/libs/core"
	authv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1"
)

// SignInWithAssertion signs a device in with a sign-in assertion of
// mavio.dev, for an account with access to the server.
func (s *AuthService) SignInWithAssertion(ctx context.Context, req *authv1.SignInWithAssertionRequest) (*authv1.SignInWithAssertionResponse, error) {
	mavioDev, err := inMavioDevMode(ctx, s.store)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	if !mavioDev || s.Assertions == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("the server is not in mavio.dev mode"))
	}
	failed := func(err error) error { return connect.NewError(connect.CodeUnauthenticated, err) }
	a, err := s.Assertions.Verify(ctx, req.GetAssertion(), req.GetSignature())
	switch {
	case errors.Is(err, cloud.ErrInvalidAssertion):
		return nil, failed(err)
	case err != nil:
		// New sign-ins wait while mavio.dev cannot be reached.
		return nil, connect.NewError(connect.CodeUnavailable, err)
	case !bytes.Equal(a.DeviceKey, req.GetDevice().GetPublicKey()):
		return nil, failed(errors.New("the assertion is for another device"))
	}
	user, err := s.store.Users().GetByAccountID(ctx, a.AccountID)
	if errors.Is(err, core.ErrNotFound) || err == nil && user.Disabled {
		return nil, failed(errors.New("the account has no access to the server"))
	} else if err != nil {
		return nil, connectError(ctx, err)
	}
	in, err := s.signIn(ctx, user, req.GetDevice(), "mavio.dev", func(core.Store) error { return nil })
	if err != nil {
		return nil, err
	}
	return authv1.SignInWithAssertionResponse_builder{
		AccessToken:           &in.token,
		AccessTokenExpireTime: timestamppb.New(in.expires),
		User:                  in.user,
		Session:               sessionToProto(&in.session, true),
	}.Build(), nil
}
