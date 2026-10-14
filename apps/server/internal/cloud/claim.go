package cloud

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/apps/server/internal/activity"
	"github.com/mavioai/mavio/libs/core"
	cloudv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/cloud/v1"
)

var (
	// ErrNotClaimable is returned when the server is standalone or
	// claimed.
	ErrNotClaimable = errors.New("the server is standalone or already claimed")
	// ErrCodeRefused is returned when mavio.dev refuses a claim code.
	ErrCodeRefused = errors.New("mavio.dev refused the claim code")
)

// Claimable reports whether a server with these settings may be claimed:
// its mode is unset, or it is an unclaimed server in mavio.dev mode.
func Claimable(a core.AccountSettings) bool {
	return a.Mode == core.AccountModeUnset || a.Mode == core.AccountModeMavioDev && !a.Claimed()
}

// Claimer claims the server at mavio.dev.
type Claimer struct {
	Store  core.Store
	Client *Client
	// Name returns the name the server shows; nil sends none.
	Name    func() string
	Version string
	// Activity records claims; nil records none.
	Activity *activity.Log
	// Claimed is called after a claim, to start the link; nil calls
	// nothing.
	Claimed func()
	// mu serializes claims, so that one code's administrator is in place
	// before another claim checks the settings.
	mu sync.Mutex
}

// Claim claims the server with a claim code, making its account the
// administrator: its user, when the server has one, becomes the
// administrator, and any other administrator a member. A server claimed
// again keeps its cursor, so that it still applies the changes it missed.
// It returns ErrNotClaimable, ErrCodeRefused wrapped, or another error
// when mavio.dev cannot be reached.
func (c *Claimer) Claim(ctx context.Context, code string) (core.User, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	settings, err := c.Store.Settings().Accounts(ctx)
	if err != nil {
		return core.User{}, err
	}
	if !Claimable(settings) {
		return core.User{}, ErrNotClaimable
	}
	req := cloudv1.ClaimServerRequest_builder{ClaimCode: &code, PublicKey: c.Client.key.Public(), Version: &c.Version}.Build()
	if c.Name != nil {
		req.SetName(c.Name())
	}
	resp, err := c.Client.ClaimServer(ctx, req)
	if code := connect.CodeOf(err); code == connect.CodeInvalidArgument || code == connect.CodeNotFound || code == connect.CodePermissionDenied {
		return core.User{}, fmt.Errorf("%w: %w", ErrCodeRefused, err)
	} else if err != nil {
		return core.User{}, fmt.Errorf("claim at mavio.dev: %w", err)
	}
	account := resp.GetAdministrator()
	if account.GetId() == "" {
		return core.User{}, errors.New("claim at mavio.dev: no administrator in the response")
	}
	var admin core.User
	err = c.Store.InTx(ctx, func(tx core.Store) error {
		settings, err := tx.Settings().Accounts(ctx)
		if err != nil {
			return err
		}
		if !Claimable(settings) {
			return ErrNotClaimable
		}
		cursor := resp.GetCursor()
		if settings.Mode == core.AccountModeMavioDev {
			cursor = settings.Cursor
		}
		claimed := core.AccountSettings{Mode: core.AccountModeMavioDev, Administrator: account.GetId(), Cursor: cursor}
		if err := tx.Settings().PutAccounts(ctx, &claimed); err != nil {
			return err
		}
		admin, err = makeAdministrator(ctx, tx, account)
		return err
	})
	if err != nil {
		return core.User{}, err
	}
	slog.InfoContext(ctx, "claimed the server at mavio.dev", "account", account.GetId(), "user", admin.Name)
	c.Activity.Record(ctx, core.Activity{Type: "server.claimed", UserID: admin.ID, Title: admin.Name + " claimed the server at mavio.dev"})
	if c.Claimed != nil {
		c.Claimed()
	}
	return admin, nil
}

// makeAdministrator makes the account's user the administrator, creating
// it when the server has none, and demotes any other administrator.
func makeAdministrator(ctx context.Context, tx core.Store, account *cloudv1.Account) (core.User, error) {
	users, err := tx.Users().List(ctx)
	if err != nil {
		return core.User{}, err
	}
	for _, u := range users {
		if u.Kind == core.UserAdmin && u.AccountID != account.GetId() {
			u.Kind = core.UserMember
			if err := tx.Users().Update(ctx, &u); err != nil {
				return core.User{}, fmt.Errorf("demote %s: %w", u.Name, err)
			}
		}
	}
	u, err := tx.Users().GetByAccountID(ctx, account.GetId())
	switch {
	case errors.Is(err, core.ErrNotFound):
		u = core.User{
			Name:      account.GetUserName(),
			Kind:      core.UserAdmin,
			AccountID: account.GetId(),
			Access:    core.LibraryAccess{AllowDownload: true},
			Policy:    core.UserPolicy{AllowTranscoding: true},
			CreatedAt: time.Now(),
		}
		return u, tx.Users().Create(ctx, &u)
	case err != nil:
		return core.User{}, err
	}
	u.Kind, u.Disabled = core.UserAdmin, false
	return u, tx.Users().Update(ctx, &u)
}
