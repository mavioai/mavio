package core

import (
	"fmt"
	"time"
)

// Invitation is a link through which someone joins a standalone server as
// a member (docs/accounts.md §4.3), with an access record the
// administrator preset, or through which a managed user's person sets a
// credential and makes it a member.
type Invitation struct {
	ID ID
	// TokenHash is the SHA-256 hash of the link's token, which is shown
	// once.
	TokenHash []byte
	// UserID is the managed user the invitation makes a member; zero for
	// an invitation that creates members.
	UserID ID
	// Access is the access record of the members it creates.
	Access LibraryAccess
	// MaxUses is how many members it may create; an invitation of a
	// managed user is used once.
	MaxUses   int
	Uses      int
	ExpiresAt time.Time
	CreatedAt time.Time
}

// MaxInvitationUses is the most members one invitation may create.
const MaxInvitationUses = 100

// Validate checks the invitation's invariants.
func (inv *Invitation) Validate() error {
	switch {
	case inv.ID.IsZero():
		return fmt.Errorf("%w: invitation requires an ID", ErrInvalid)
	case len(inv.TokenHash) != 32:
		return fmt.Errorf("%w: invitation %s requires a SHA-256 token hash", ErrInvalid, inv.ID)
	case inv.MaxUses < 1 || inv.MaxUses > MaxInvitationUses:
		return fmt.Errorf("%w: invitation %s may be used 1 to %d times", ErrInvalid, inv.ID, MaxInvitationUses)
	case !inv.UserID.IsZero() && inv.MaxUses != 1:
		return fmt.Errorf("%w: invitation %s of a managed user is used once", ErrInvalid, inv.ID)
	case inv.Uses < 0 || inv.Uses > inv.MaxUses:
		return fmt.Errorf("%w: invitation %s used %d of %d times", ErrInvalid, inv.ID, inv.Uses, inv.MaxUses)
	case inv.ExpiresAt.IsZero():
		return fmt.Errorf("%w: invitation %s requires an expiry", ErrInvalid, inv.ID)
	}
	return inv.Access.Validate()
}

// Usable reports whether the invitation may still be accepted at now.
func (inv *Invitation) Usable(now time.Time) bool {
	return inv.Uses < inv.MaxUses && now.Before(inv.ExpiresAt)
}
