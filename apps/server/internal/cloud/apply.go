package cloud

import (
	"context"
	"errors"
	"time"

	"github.com/mavioai/mavio/libs/core"
	cloudv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/cloud/v1"
	userv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1"
)

// Applier applies mavio.dev's changes to the server's users
// (docs/accounts.md §3.3, §5.5).
type Applier struct {
	// Requests returns the request settings, whose default permissions
	// accounts given access get; nil uses the defaults.
	Requests func() core.RequestSettings
}

// Apply applies one change in tx; changes of kinds it does not know are
// skipped.
func (a *Applier) Apply(ctx context.Context, tx core.Store, c *cloudv1.Change) error {
	switch c.WhichKind() {
	case cloudv1.Change_AccessGranted_case:
		return a.grant(ctx, tx, c.GetAccessGranted())
	case cloudv1.Change_AccessRevoked_case:
		return revoke(ctx, tx, c.GetAccessRevoked().GetAccountId())
	case cloudv1.Change_AccountChanged_case:
		account := c.GetAccountChanged()
		u, err := tx.Users().GetByAccountID(ctx, account.GetId())
		if errors.Is(err, core.ErrNotFound) {
			return nil
		} else if err != nil {
			return err
		}
		u.Name = account.GetUserName()
		setKind(&u, account)
		return tx.Users().Update(ctx, &u)
	case cloudv1.Change_AccountRevoked_case:
		r := c.GetAccountRevoked()
		u, err := tx.Users().GetByAccountID(ctx, r.GetAccountId())
		if errors.Is(err, core.ErrNotFound) {
			return nil
		} else if err != nil {
			return err
		}
		// The administrator is kept, as the server needs one; the command
		// line unclaims it for another account.
		if r.GetReason() == cloudv1.AccountRevoked_REASON_DELETED && !u.IsAdmin() {
			return tx.Users().Delete(ctx, u.ID)
		}
		return endSessions(ctx, tx, u.ID)
	}
	return nil
}

// grant creates the account's user, or gives it the access record. A user
// whose access was revoked finds its data as it was.
func (a *Applier) grant(ctx context.Context, tx core.Store, g *cloudv1.AccessGranted) error {
	account := g.GetAccount()
	access, err := accessFromProto(g.GetAccess())
	if err != nil {
		return err
	}
	u, err := tx.Users().GetByAccountID(ctx, account.GetId())
	if errors.Is(err, core.ErrNotFound) {
		requests := core.DefaultRequestSettings()
		if a.Requests != nil {
			requests = a.Requests()
		}
		u = core.User{
			Name:      account.GetUserName(),
			Kind:      core.UserMember,
			AccountID: account.GetId(),
			Access:    access,
			Policy:    core.UserPolicy{AllowTranscoding: true, Requests: requests.DefaultPermissions},
			CreatedAt: time.Now(),
		}
		setKind(&u, account)
		return tx.Users().Create(ctx, &u)
	} else if err != nil {
		return err
	}
	if u.IsAdmin() {
		return nil // the administrator reaches everything
	}
	u.Name, u.Access, u.Disabled = account.GetUserName(), access, false
	setKind(&u, account)
	return tx.Users().Update(ctx, &u)
}

// restrictions maps the restriction profiles of accounts.
var restrictions = map[userv1.RestrictionProfile]core.RestrictionProfile{
	userv1.RestrictionProfile_RESTRICTION_PROFILE_YOUNGER_CHILD: core.RestrictionYoungerChild,
	userv1.RestrictionProfile_RESTRICTION_PROFILE_OLDER_CHILD:   core.RestrictionOlderChild,
	userv1.RestrictionProfile_RESTRICTION_PROFILE_TEEN:          core.RestrictionTeen,
}

// setKind makes a user managed with the account's restriction profile, or
// a member once the account is converted; the administrator stays as it is.
func setKind(u *core.User, account *cloudv1.Account) {
	if u.IsAdmin() {
		return
	}
	if account.GetKind() == cloudv1.AccountKind_ACCOUNT_KIND_MANAGED {
		u.Kind, u.Restriction = core.UserManaged, restrictions[account.GetRestriction()]
		return
	}
	u.Kind, u.Restriction = core.UserMember, core.RestrictionNone
}

// revoke takes an account's access away, keeping its user and data and
// ending its sessions.
func revoke(ctx context.Context, tx core.Store, accountID string) error {
	u, err := tx.Users().GetByAccountID(ctx, accountID)
	if errors.Is(err, core.ErrNotFound) {
		return nil
	} else if err != nil {
		return err
	}
	if u.IsAdmin() {
		return nil
	}
	u.Disabled = true
	if err := tx.Users().Update(ctx, &u); err != nil {
		return err
	}
	return endSessions(ctx, tx, u.ID)
}

func endSessions(ctx context.Context, tx core.Store, userID core.ID) error {
	sessions, err := tx.AuthSessions().ListForUser(ctx, userID)
	if err != nil {
		return err
	}
	for _, s := range sessions {
		if err := tx.AuthSessions().Delete(ctx, s.ID); err != nil && !errors.Is(err, core.ErrNotFound) {
			return err
		}
	}
	return nil
}

// accessFromProto converts a record, skipping library IDs that are not
// the server's, which would otherwise hold the link up for good.
func accessFromProto(a *cloudv1.Access) (core.LibraryAccess, error) {
	out := core.LibraryAccess{
		ContentLimits: core.ContentLimits{
			BlockUnrated:  a.GetBlockUnrated(),
			AllowedLabels: a.GetAllowedLabels(),
			BlockedLabels: a.GetBlockedLabels(),
		},
		AllowDownload: a.GetAllowDownload(),
	}
	if a.HasMaxParentalRating() {
		out.MaxParentalRating = new(int(a.GetMaxParentalRating()))
	}
	if !a.GetAllLibraries() {
		out.Libraries = make([]core.ID, 0, len(a.GetLibraryIds()))
		for _, id := range a.GetLibraryIds() {
			if parsed, err := core.ParseID(id); err == nil {
				out.Libraries = append(out.Libraries, parsed)
			}
		}
	}
	return out, nil
}
