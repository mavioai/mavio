package rpc

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/apps/server/internal/cloud"
	authv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1"
)

// ClaimServer claims the server at mavio.dev with a claim code.
func (s *AuthService) ClaimServer(ctx context.Context, req *authv1.ClaimServerRequest) (*authv1.ClaimServerResponse, error) {
	if s.Claim == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, errors.New("the server cannot be claimed"))
	}
	user, err := s.Claim(ctx, req.GetClaimCode())
	switch {
	case errors.Is(err, cloud.ErrNotClaimable):
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	case errors.Is(err, cloud.ErrCodeRefused):
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	case connect.CodeOf(err) != connect.CodeUnknown:
		// Anything else mavio.dev answered, or not reaching it.
		return nil, connect.NewError(connect.CodeUnavailable, err)
	case err != nil:
		return nil, connectError(ctx, err)
	}
	return authv1.ClaimServerResponse_builder{User: userToProto(&user, nil)}.Build(), nil
}
