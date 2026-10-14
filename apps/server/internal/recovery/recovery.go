package recovery

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/mavioai/mavio/apps/server/internal/auth"
	"github.com/mavioai/mavio/libs/core"
)

// ErrNotStandalone means the server has no administrator of its own to
// recover: it is not set up, or its administrator is a Mavio account.
var ErrNotStandalone = errors.New("the server is not in standalone mode")

// ResetPassword gives the administrator of a standalone server a new
// password, turns off its TOTP and ends its sessions, and returns the
// administrator. A running server sees the change at once, as it reads
// sessions and credentials from the database.
func ResetPassword(ctx context.Context, store core.Store, password string, now time.Time) (core.User, error) {
	if password == "" {
		return core.User{}, fmt.Errorf("%w: empty password", core.ErrInvalid)
	}
	var admin core.User
	err := store.InTx(ctx, func(tx core.Store) error {
		accounts, err := tx.Settings().Accounts(ctx)
		if err != nil {
			return err
		}
		switch accounts.Mode {
		case core.AccountModeStandalone:
		case core.AccountModeUnset:
			return fmt.Errorf("%w: it is not set up yet; start it and create the administrator", ErrNotStandalone)
		default:
			return fmt.Errorf("%w: its administrator is a Mavio account, recovered at mavio.dev", ErrNotStandalone)
		}
		if admin, err = administrator(ctx, tx); err != nil {
			return err
		}
		if err := tx.Credentials().Put(ctx, &core.Credential{
			UserID: admin.ID, Kind: core.CredentialPassword, Data: auth.HashPassword(password), CreatedAt: now,
		}); err != nil {
			return err
		}
		if err := tx.Credentials().Delete(ctx, admin.ID, core.CredentialTOTP); err != nil {
			return err
		}
		sessions, err := tx.AuthSessions().ListForUser(ctx, admin.ID)
		if err != nil {
			return err
		}
		for _, s := range sessions {
			if err := tx.AuthSessions().Delete(ctx, s.ID); err != nil {
				return err
			}
		}
		return tx.Activities().Add(ctx, &core.Activity{
			Time: now, Type: "user.password_reset", Severity: core.SeverityWarning, UserID: admin.ID,
			Title: "Password of " + admin.Name + " reset from the command line",
		})
	})
	return admin, err
}

// administrator returns the server's one administrator.
func administrator(ctx context.Context, store core.Store) (core.User, error) {
	users, err := store.Users().List(ctx)
	if err != nil {
		return core.User{}, err
	}
	for _, u := range users {
		if u.IsAdmin() {
			return u, nil
		}
	}
	return core.User{}, fmt.Errorf("%w: the server has no administrator", core.ErrNotFound)
}

// ErrNotClaimed means the server has no claim to undo: it is not in
// mavio.dev mode, or no account claims it.
var ErrNotClaimed = errors.New("the server is not claimed at mavio.dev")

// Unclaim returns a server in mavio.dev mode to the unclaimed state, so
// that another account claims it when the administrator's account is
// lost: the administrator becomes a member and its sessions end, and the
// users and their data stay. It returns the former administrator.
func Unclaim(ctx context.Context, store core.Store, now time.Time) (core.User, error) {
	var admin core.User
	err := store.InTx(ctx, func(tx core.Store) error {
		accounts, err := tx.Settings().Accounts(ctx)
		if err != nil {
			return err
		}
		if !accounts.Claimed() {
			return ErrNotClaimed
		}
		accounts.Administrator = ""
		if err := tx.Settings().PutAccounts(ctx, &accounts); err != nil {
			return err
		}
		admin, err = administrator(ctx, tx)
		if errors.Is(err, core.ErrNotFound) {
			return nil
		} else if err != nil {
			return err
		}
		admin.Kind = core.UserMember
		if err := tx.Users().Update(ctx, &admin); err != nil {
			return err
		}
		sessions, err := tx.AuthSessions().ListForUser(ctx, admin.ID)
		if err != nil {
			return err
		}
		for _, s := range sessions {
			if err := tx.AuthSessions().Delete(ctx, s.ID); err != nil {
				return err
			}
		}
		return tx.Activities().Add(ctx, &core.Activity{
			Time: now, Type: "server.unclaimed", Severity: core.SeverityWarning, UserID: admin.ID,
			Title: "The server was unclaimed from the command line; " + admin.Name + " is no longer its administrator",
		})
	})
	return admin, err
}
