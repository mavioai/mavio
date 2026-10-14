package rpc

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/libs/core"
)

// errMavioDevMode answers the RPCs that sign in with or manage the
// server's own users and credentials in mavio.dev mode, where mavio.dev
// keeps them (docs/accounts.md §8).
var errMavioDevMode = connect.NewError(connect.CodeFailedPrecondition,
	errors.New("the server's users are Mavio accounts, managed at mavio.dev"))

// standaloneOnly fails with FAILED_PRECONDITION in mavio.dev mode.
func standaloneOnly(ctx context.Context, store core.Store) error {
	mavioDev, err := inMavioDevMode(ctx, store)
	if err != nil {
		return connectError(ctx, err)
	}
	if mavioDev {
		return errMavioDevMode
	}
	return nil
}

func inMavioDevMode(ctx context.Context, store core.Store) (bool, error) {
	accounts, err := store.Settings().Accounts(ctx)
	return accounts.Mode == core.AccountModeMavioDev, err
}
