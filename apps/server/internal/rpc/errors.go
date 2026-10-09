package rpc

import (
	"context"
	"errors"
	"log/slog"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/libs/core"
)

// connectError maps domain errors to Connect codes. Unexpected errors are
// logged and hidden from the client.
func connectError(ctx context.Context, err error) error {
	var ce *connect.Error
	switch {
	case errors.As(err, &ce):
		return ce
	case errors.Is(err, core.ErrNotFound):
		return connect.NewError(connect.CodeNotFound, err)
	case errors.Is(err, core.ErrConflict):
		return connect.NewError(connect.CodeAlreadyExists, err)
	case errors.Is(err, core.ErrInvalid):
		return connect.NewError(connect.CodeInvalidArgument, err)
	case errors.Is(err, context.Canceled):
		return connect.NewError(connect.CodeCanceled, err)
	case errors.Is(err, context.DeadlineExceeded):
		return connect.NewError(connect.CodeDeadlineExceeded, err)
	}
	slog.ErrorContext(ctx, "request failed", "err", err)
	return connect.NewError(connect.CodeInternal, errors.New("internal error"))
}
