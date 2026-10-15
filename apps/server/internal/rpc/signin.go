package rpc

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mavioai/mavio/apps/server/internal/cloud"
	authv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1"
)

// StartSignIn pushes the authorization request of the web UI at the
// request's redirect URI to mavio.dev.
func (s *AuthService) StartSignIn(ctx context.Context, req *authv1.StartSignInRequest) (*authv1.StartSignInResponse, error) {
	if s.SignIn == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, errors.New("the server does not sign in with Mavio accounts"))
	}
	url, expires, err := s.SignIn.StartSignIn(ctx, cloud.SignInRequest{
		RedirectURI: req.GetRedirectUri(), CodeChallenge: req.GetCodeChallenge(), DeviceKey: req.GetDeviceKey(), State: req.GetState(),
	})
	switch {
	case errors.Is(err, cloud.ErrStandalone):
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	case connect.CodeOf(err) != connect.CodeUnknown:
		// Anything mavio.dev answered, or not reaching it.
		return nil, connect.NewError(connect.CodeUnavailable, err)
	case err != nil:
		return nil, connectError(ctx, err)
	}
	return authv1.StartSignInResponse_builder{Url: &url, ExpireTime: timestamppb.New(expires)}.Build(), nil
}
