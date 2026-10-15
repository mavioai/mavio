package rpc

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mavioai/mavio/apps/server/internal/cloud"
	authv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1"
)

// StartClaim asks mavio.dev for a claim request for the web UI at the
// request's redirect URI.
func (s *AuthService) StartClaim(ctx context.Context, req *authv1.StartClaimRequest) (*authv1.StartClaimResponse, error) {
	if s.RequestClaim == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, errors.New("the server cannot be claimed"))
	}
	url, expires, err := s.RequestClaim(ctx, cloud.ClaimRequest{
		RedirectURI: req.GetRedirectUri(), CodeChallenge: req.GetCodeChallenge(), DeviceKey: req.GetDeviceKey(), State: req.GetState(),
	})
	switch {
	case errors.Is(err, cloud.ErrNotClaimable):
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	case connect.CodeOf(err) != connect.CodeUnknown:
		// Anything mavio.dev answered, or not reaching it.
		return nil, connect.NewError(connect.CodeUnavailable, err)
	case err != nil:
		return nil, connectError(ctx, err)
	}
	return authv1.StartClaimResponse_builder{Url: &url, ExpireTime: timestamppb.New(expires)}.Build(), nil
}
