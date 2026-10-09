package rpc

import (
	"context"
	"fmt"

	"buf.build/go/protovalidate"
	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"
)

// NewValidateInterceptor rejects unary requests that break their
// protovalidate rules with INVALID_ARGUMENT.
func NewValidateInterceptor() (connect.Interceptor, error) {
	v, err := protovalidate.New()
	if err != nil {
		return nil, fmt.Errorf("protovalidate: %w", err)
	}
	return connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			if msg, ok := req.Any().(proto.Message); ok && !req.Spec().IsClient {
				if err := v.Validate(msg); err != nil {
					return nil, connect.NewError(connect.CodeInvalidArgument, err)
				}
			}
			return next(ctx, req)
		}
	}), nil
}
