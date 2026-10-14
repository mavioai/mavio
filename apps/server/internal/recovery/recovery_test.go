package recovery_test

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/mavioai/mavio/apps/server/internal/auth"
	"github.com/mavioai/mavio/apps/server/internal/recovery"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store"
)

func TestResetPassword(t *testing.T) {
	ctx := t.Context()
	st, err := store.Open(ctx, "sqlite:"+filepath.Join(t.TempDir(), "mavio.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)

	if _, err := recovery.ResetPassword(ctx, st, "new", now); !errors.Is(err, recovery.ErrNotStandalone) {
		t.Errorf("before setup: %v, want ErrNotStandalone", err)
	}
	if err := st.Settings().PutAccounts(ctx, &core.AccountSettings{Mode: core.AccountModeStandalone}); err != nil {
		t.Fatal(err)
	}
	admin := core.User{Name: "admin", Kind: core.UserAdmin}
	member := core.User{Name: "bob", Kind: core.UserMember}
	for _, u := range []*core.User{&admin, &member} {
		if err := st.Users().Create(ctx, u); err != nil {
			t.Fatal(err)
		}
		if err := st.Credentials().Put(ctx, &core.Credential{UserID: u.ID, Kind: core.CredentialTOTP, Data: "sealed"}); err != nil {
			t.Fatal(err)
		}
		if err := st.AuthSessions().Create(ctx, &core.AuthSession{UserID: u.ID, DeviceKey: make([]byte, 32), DeviceID: u.Name, LastSeenAt: now}); err != nil {
			t.Fatal(err)
		}
	}

	got, err := recovery.ResetPassword(ctx, st, "new", now)
	if err != nil || got.ID != admin.ID {
		t.Fatalf("ResetPassword = %v, %v; want the administrator", got.Name, err)
	}
	password, err := st.Credentials().Get(ctx, admin.ID, core.CredentialPassword)
	if err != nil {
		t.Fatal(err)
	}
	if ok, _, err := auth.VerifyPassword(password.Data, "new"); err != nil || !ok {
		t.Errorf("the new password does not verify: %v", err)
	}
	if _, err := st.Credentials().Get(ctx, admin.ID, core.CredentialTOTP); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("administrator's TOTP: %v, want ErrNotFound", err)
	}
	if list, _ := st.AuthSessions().ListForUser(ctx, admin.ID); len(list) != 0 {
		t.Errorf("administrator's sessions = %d, want = 0", len(list))
	}
	// Other users are left alone.
	if _, err := st.Credentials().Get(ctx, member.ID, core.CredentialTOTP); err != nil {
		t.Errorf("member's TOTP: %v", err)
	}
	if list, _ := st.AuthSessions().ListForUser(ctx, member.ID); len(list) != 1 {
		t.Errorf("member's sessions = %d, want = 1", len(list))
	}
	page, err := st.Activities().List(ctx, core.ActivityQuery{Types: []string{"user.password_reset"}})
	if err != nil || len(page.Items) != 1 {
		t.Errorf("activities = %+v, %v; want the reset", page, err)
	}

	if err := st.Settings().PutAccounts(ctx, &core.AccountSettings{Mode: core.AccountModeMavioDev}); err != nil {
		t.Fatal(err)
	}
	if _, err := recovery.ResetPassword(ctx, st, "new", now); !errors.Is(err, recovery.ErrNotStandalone) {
		t.Errorf("in mavio.dev mode: %v, want ErrNotStandalone", err)
	}
}

func TestUnclaim(t *testing.T) {
	ctx := t.Context()
	st, err := store.Open(ctx, "sqlite:"+filepath.Join(t.TempDir(), "mavio.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)

	for _, mode := range []core.AccountMode{core.AccountModeUnset, core.AccountModeStandalone} {
		if err := st.Settings().PutAccounts(ctx, &core.AccountSettings{Mode: mode}); err != nil {
			t.Fatal(err)
		}
		if _, err := recovery.Unclaim(ctx, st, now); !errors.Is(err, recovery.ErrNotClaimed) {
			t.Errorf("Unclaim in mode %q: %v, want ErrNotClaimed", mode, err)
		}
	}
	claimed := core.AccountSettings{Mode: core.AccountModeMavioDev, Administrator: "acct-1", Cursor: 7}
	if err := st.Settings().PutAccounts(ctx, &claimed); err != nil {
		t.Fatal(err)
	}
	admin := core.User{Name: "alice", Kind: core.UserAdmin, AccountID: "acct-1"}
	if err := st.Users().Create(ctx, &admin); err != nil {
		t.Fatal(err)
	}
	if err := st.AuthSessions().Create(ctx, &core.AuthSession{UserID: admin.ID, DeviceKey: make([]byte, 32), DeviceID: "phone", LastSeenAt: now}); err != nil {
		t.Fatal(err)
	}

	got, err := recovery.Unclaim(ctx, st, now)
	if err != nil || got.ID != admin.ID {
		t.Fatalf("Unclaim = %v, %v; want the administrator", got.Name, err)
	}
	if a, _ := st.Settings().Accounts(ctx); a.Claimed() || a.Mode != core.AccountModeMavioDev || a.Cursor != 7 {
		t.Errorf("settings = %+v, want unclaimed in mavio.dev mode at cursor 7", a)
	}
	if u, _ := st.Users().Get(ctx, admin.ID); u.Kind != core.UserMember || u.AccountID != "acct-1" {
		t.Errorf("former administrator = %+v, want a member of its account", u)
	}
	if list, _ := st.AuthSessions().ListForUser(ctx, admin.ID); len(list) != 0 {
		t.Errorf("former administrator's sessions = %d, want = 0", len(list))
	}
	if _, err := recovery.Unclaim(ctx, st, now); !errors.Is(err, recovery.ErrNotClaimed) {
		t.Errorf("second Unclaim: %v, want ErrNotClaimed", err)
	}
}
