package cloud_test

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/mavioai/mavio/apps/server/internal/cloud"
	"github.com/mavioai/mavio/libs/core"
	cloudv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/cloud/v1"
	userv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1"
)

func granted(a *cloudv1.Account, access *cloudv1.Access) *cloudv1.Change {
	return cloudv1.Change_builder{AccessGranted: cloudv1.AccessGranted_builder{Account: a, Access: access}.Build()}.Build()
}

func TestApply(t *testing.T) {
	ctx := t.Context()
	st := openStore(t)
	apply := (&cloud.Applier{}).Apply
	lib := core.NewID()
	run := func(c *cloudv1.Change) {
		t.Helper()
		if err := st.InTx(ctx, func(tx core.Store) error { return apply(ctx, tx, c) }); err != nil {
			t.Fatalf("Apply: %v", err)
		}
	}
	bob := account("acct-bob", "bob")
	user := func() core.User {
		t.Helper()
		u, err := st.Users().GetByAccountID(ctx, "acct-bob")
		if err != nil {
			t.Fatal(err)
		}
		return u
	}

	// A record creates the account's user with its access.
	run(granted(bob, cloudv1.Access_builder{
		LibraryIds: []string{lib.String(), "not an ID"}, MaxParentalRating: new(int32(13)),
		BlockedLabels: []string{"horror"}, AllowDownload: new(true),
	}.Build()))
	u := user()
	if u.Name != "bob" || u.Kind != core.UserMember || !slices.Equal(u.Access.Libraries, []core.ID{lib}) ||
		u.Access.MaxParentalRating == nil || *u.Access.MaxParentalRating != 13 || !u.Access.AllowDownload || !u.Policy.AllowTranscoding {
		t.Errorf("granted user = %+v", u)
	}
	if err := st.AuthSessions().Create(ctx, &core.AuthSession{UserID: u.ID, DeviceKey: make([]byte, 32), DeviceID: "phone", LastSeenAt: time.Now()}); err != nil {
		t.Fatal(err)
	}

	// Revoking keeps the user, disabled, and ends its sessions.
	run(cloudv1.Change_builder{AccessRevoked: cloudv1.AccessRevoked_builder{AccountId: new("acct-bob")}.Build()}.Build())
	if u := user(); !u.Disabled {
		t.Errorf("revoked user = %+v, want disabled", u)
	}
	if list, _ := st.AuthSessions().ListForUser(ctx, u.ID); len(list) != 0 {
		t.Errorf("revoked user's sessions = %d, want = 0", len(list))
	}

	// Access granted again finds the user, with every library.
	run(granted(bob, cloudv1.Access_builder{AllLibraries: new(true)}.Build()))
	if again := user(); again.ID != u.ID || again.Disabled || again.Access.Libraries != nil {
		t.Errorf("regranted user = %+v, want the same user with every library", again)
	}

	run(cloudv1.Change_builder{AccountChanged: account("acct-bob", "robert")}.Build())
	if u := user(); u.Name != "robert" {
		t.Errorf("renamed user = %q, want robert", u.Name)
	}

	// A managed account's user is managed, with its restriction profile.
	kid := cloudv1.Account_builder{
		Id: new("acct-kid"), UserName: new("kid"), Kind: cloudv1.AccountKind_ACCOUNT_KIND_MANAGED.Enum(),
		Restriction: userv1.RestrictionProfile_RESTRICTION_PROFILE_OLDER_CHILD.Enum(),
	}.Build()
	run(granted(kid, cloudv1.Access_builder{AllLibraries: new(true)}.Build()))
	if k, err := st.Users().GetByAccountID(ctx, "acct-kid"); err != nil || k.Kind != core.UserManaged || k.Restriction != core.RestrictionOlderChild {
		t.Errorf("managed account's user = %+v, %v; want managed, older_child", k, err)
	}
	kid.SetRestriction(userv1.RestrictionProfile_RESTRICTION_PROFILE_TEEN)
	run(cloudv1.Change_builder{AccountChanged: kid}.Build())
	if k, err := st.Users().GetByAccountID(ctx, "acct-kid"); err != nil || k.Restriction != core.RestrictionTeen {
		t.Errorf("restricted user = %+v, %v; want teen", k, err)
	}
	// A converted account's user becomes a member without a restriction.
	run(cloudv1.Change_builder{AccountChanged: account("acct-kid", "kid")}.Build())
	if k, err := st.Users().GetByAccountID(ctx, "acct-kid"); err != nil || k.Kind != core.UserMember || k.Restriction != core.RestrictionNone {
		t.Errorf("converted user = %+v, %v; want a member without a restriction", k, err)
	}

	// The administrator keeps its kind and access, and is never deleted.
	admin := core.User{Name: "alice", Kind: core.UserAdmin, AccountID: "acct-alice"}
	if err := st.Users().Create(ctx, &admin); err != nil {
		t.Fatal(err)
	}
	deleted := func(id string) *cloudv1.Change {
		return cloudv1.Change_builder{AccountRevoked: cloudv1.AccountRevoked_builder{
			AccountId: &id, Reason: cloudv1.AccountRevoked_REASON_DELETED.Enum(),
		}.Build()}.Build()
	}
	run(granted(account("acct-alice", "alice"), cloudv1.Access_builder{LibraryIds: []string{lib.String()}}.Build()))
	run(cloudv1.Change_builder{AccessRevoked: cloudv1.AccessRevoked_builder{AccountId: new("acct-alice")}.Build()}.Build())
	run(deleted("acct-alice"))
	if a, err := st.Users().Get(ctx, admin.ID); err != nil || !a.IsAdmin() || a.Disabled || a.Access.Libraries != nil {
		t.Errorf("administrator = %+v, %v; want unchanged", a, err)
	}

	// A deleted account's user goes with its data.
	run(deleted("acct-bob"))
	if _, err := st.Users().Get(ctx, u.ID); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("deleted account's user: %v, want ErrNotFound", err)
	}
	// Changes of unknown accounts and of no kind are skipped.
	run(cloudv1.Change_builder{AccessRevoked: cloudv1.AccessRevoked_builder{AccountId: new("acct-none")}.Build()}.Build())
	run(&cloudv1.Change{})
}
