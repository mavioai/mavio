package core_test

import (
	"errors"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

func TestInvitation(t *testing.T) {
	at := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	valid := func() core.Invitation {
		return core.Invitation{ID: core.NewID(), TokenHash: make([]byte, 32), MaxUses: 2, ExpiresAt: at}
	}
	tests := []struct {
		name  string
		edit  func(*core.Invitation)
		valid bool
	}{
		{"valid", func(*core.Invitation) {}, true},
		{"no ID", func(inv *core.Invitation) { inv.ID = core.ID{} }, false},
		{"short hash", func(inv *core.Invitation) { inv.TokenHash = make([]byte, 16) }, false},
		{"no uses", func(inv *core.Invitation) { inv.MaxUses = 0 }, false},
		{"too many uses", func(inv *core.Invitation) { inv.MaxUses = core.MaxInvitationUses + 1 }, false},
		{"used beyond", func(inv *core.Invitation) { inv.Uses = 3 }, false},
		{"conversion used twice", func(inv *core.Invitation) { inv.UserID = core.NewID() }, false},
		{"conversion", func(inv *core.Invitation) { inv.UserID, inv.MaxUses = core.NewID(), 1 }, true},
		{"no expiry", func(inv *core.Invitation) { inv.ExpiresAt = time.Time{} }, false},
		{"empty label", func(inv *core.Invitation) { inv.Access.BlockedLabels = []string{" "} }, false},
	}
	for _, tt := range tests {
		inv := valid()
		tt.edit(&inv)
		err := inv.Validate()
		if got := err == nil; got != tt.valid || (err != nil && !errors.Is(err, core.ErrInvalid)) {
			t.Errorf("%s: Validate() = %v, want valid = %v", tt.name, err, tt.valid)
		}
	}

	inv := valid()
	for _, c := range []struct {
		uses int
		now  time.Time
		want bool
	}{
		{0, at.Add(-time.Second), true},
		{1, at.Add(-time.Second), true},
		{2, at.Add(-time.Second), false},
		{0, at, false},
	} {
		inv.Uses = c.uses
		if got := inv.Usable(c.now); got != c.want {
			t.Errorf("Usable with %d uses at %v = %v, want = %v", c.uses, c.now, got, c.want)
		}
	}
}

func TestAuthSessionCanSwitch(t *testing.T) {
	admin := core.User{Kind: core.UserAdmin}
	kid := core.User{Kind: core.UserManaged}
	var plain, switched core.AuthSession
	switched.SwitcherID = core.NewID()
	for _, c := range []struct {
		name string
		s    core.AuthSession
		u    core.User
		want bool
	}{
		{"administrator", plain, admin, true},
		{"member", plain, kid, false},
		{"switched", switched, kid, true},
	} {
		if got := c.s.CanSwitch(&c.u); got != c.want {
			t.Errorf("%s: CanSwitch = %v, want = %v", c.name, got, c.want)
		}
	}
}
