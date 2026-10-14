package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mavioai/mavio/apps/server/internal/auth"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store"
)

func TestAdminResetPassword(t *testing.T) {
	ctx := t.Context()
	home := t.TempDir()
	dsn := "sqlite:" + filepath.Join(home, "mavio.db")
	st, err := store.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Settings().PutAccounts(ctx, &core.AccountSettings{Mode: core.AccountModeStandalone}); err != nil {
		t.Fatal(err)
	}
	admin := core.User{Name: "root", Kind: core.UserAdmin}
	if err := st.Users().Create(ctx, &admin); err != nil {
		t.Fatal(err)
	}

	resetPassword := func(stdin string) (string, error) {
		cmd := newCommand()
		var out bytes.Buffer
		cmd.SetIn(strings.NewReader(stdin))
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"admin", "reset-password", "--home", home})
		err := cmd.ExecuteContext(ctx)
		return out.String(), err
	}
	if _, err := resetPassword("\n"); err == nil {
		t.Error("an empty password was accepted")
	}
	out, err := resetPassword("hunter2\n")
	if err != nil || !strings.Contains(out, "administrator root") {
		t.Fatalf("reset-password = %q, %v", out, err)
	}
	password, err := st.Credentials().Get(ctx, admin.ID, core.CredentialPassword)
	if err != nil {
		t.Fatal(err)
	}
	if ok, _, err := auth.VerifyPassword(password.Data, "hunter2"); err != nil || !ok {
		t.Errorf("the new password does not verify: %v", err)
	}
}

func TestAdminUnclaim(t *testing.T) {
	ctx := t.Context()
	home := t.TempDir()
	st, err := store.Open(ctx, "sqlite:"+filepath.Join(home, "mavio.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Settings().PutAccounts(ctx, &core.AccountSettings{Mode: core.AccountModeMavioDev, Administrator: "acct-1"}); err != nil {
		t.Fatal(err)
	}
	if err := st.Users().Create(ctx, &core.User{Name: "alice", Kind: core.UserAdmin, AccountID: "acct-1"}); err != nil {
		t.Fatal(err)
	}
	cmd := newCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"admin", "unclaim", "--home", home})
	if err := cmd.ExecuteContext(ctx); err != nil || !strings.Contains(out.String(), "alice is now a member") {
		t.Fatalf("unclaim = %q, %v", out.String(), err)
	}
	if a, _ := st.Settings().Accounts(ctx); a.Claimed() {
		t.Errorf("settings = %+v, want unclaimed", a)
	}
}
