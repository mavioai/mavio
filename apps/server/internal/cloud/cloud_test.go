package cloud_test

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/mavioai/mavio/apps/server/internal/cloud"
	"github.com/mavioai/mavio/apps/server/internal/cloud/cloudtest"
	"github.com/mavioai/mavio/apps/server/internal/identity"
	"github.com/mavioai/mavio/apps/server/internal/recovery"
	"github.com/mavioai/mavio/libs/core"
	cloudv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/cloud/v1"
	"github.com/mavioai/mavio/libs/store"
)

func openStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(t.Context(), "sqlite:"+filepath.Join(t.TempDir(), "mavio.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func account(id, name string) *cloudv1.Account {
	return cloudv1.Account_builder{Id: &id, UserName: &name, DisplayName: &name}.Build()
}

// eventually fails the test unless cond holds within five seconds.
func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); !cond(); time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting until %s", what)
		}
	}
}

func TestClaimable(t *testing.T) {
	for _, tc := range []struct {
		settings core.AccountSettings
		want     bool
	}{
		{core.AccountSettings{}, true},
		{core.AccountSettings{Mode: core.AccountModeStandalone}, false},
		{core.AccountSettings{Mode: core.AccountModeMavioDev}, true},
		{core.AccountSettings{Mode: core.AccountModeMavioDev, Administrator: "a"}, false},
	} {
		if got := cloud.Claimable(tc.settings); got != tc.want {
			t.Errorf("Claimable(%+v) = %v, want = %v", tc.settings, got, tc.want)
		}
	}
}

func TestClaim(t *testing.T) {
	ctx := t.Context()
	st := openStore(t)
	dev := cloudtest.New(t)
	key := identity.Generate()
	claims := 0
	c := &cloud.Claimer{Store: st, Client: cloud.NewClient(dev.URL, key, nil), Version: "1.0", Claimed: func() { claims++ }}

	if _, err := c.Claim(ctx, "wrong"); !errors.Is(err, cloud.ErrCodeRefused) {
		t.Errorf("Claim with an unknown code: %v, want ErrCodeRefused", err)
	}
	dev.AddClaimCode("code-1", account("acct-1", "alice"))
	alice, err := c.Claim(ctx, "code-1")
	if err != nil || alice.Name != "alice" || alice.Kind != core.UserAdmin || alice.AccountID != "acct-1" || claims != 1 {
		t.Fatalf("Claim = %+v, %v (claims %d); want alice as the administrator", alice, err, claims)
	}
	if a, _ := st.Settings().Accounts(ctx); !a.Claimed() || a.Administrator != "acct-1" {
		t.Errorf("settings = %+v, want claimed by acct-1", a)
	}
	dev.AddClaimCode("code-2", account("acct-2", "bob"))
	if _, err := c.Claim(ctx, "code-2"); !errors.Is(err, cloud.ErrNotClaimable) {
		t.Errorf("Claim of a claimed server: %v, want ErrNotClaimable", err)
	}

	// Claimed again after the command line unclaimed it, the server keeps
	// its cursor and alice becomes a member.
	dev.AddChange(key.ID(), nil)
	dev.AddChange(key.ID(), nil)
	if _, err := recovery.Unclaim(ctx, st, time.Now()); err != nil {
		t.Fatal(err)
	}
	bob, err := c.Claim(ctx, "code-2")
	if err != nil || bob.Name != "bob" || bob.Kind != core.UserAdmin {
		t.Fatalf("reclaim = %+v, %v; want bob as the administrator", bob, err)
	}
	if a, _ := st.Settings().Accounts(ctx); a.Administrator != "acct-2" || a.Cursor != 0 {
		t.Errorf("settings = %+v, want claimed by acct-2 at cursor 0", a)
	}
	if u, _ := st.Users().Get(ctx, alice.ID); u.Kind != core.UserMember {
		t.Errorf("alice = %v, want a member", u.Kind)
	}

	// The account of an existing user becomes its administrator again.
	if _, err := recovery.Unclaim(ctx, st, time.Now()); err != nil {
		t.Fatal(err)
	}
	dev.AddClaimCode("code-3", account("acct-1", "alice"))
	if again, err := c.Claim(ctx, "code-3"); err != nil || again.ID != alice.ID || again.Kind != core.UserAdmin {
		t.Errorf("reclaim by alice = %+v, %v; want alice's user as the administrator", again, err)
	}
	if u, _ := st.Users().Get(ctx, bob.ID); u.Kind != core.UserMember {
		t.Errorf("bob = %v, want a member", u.Kind)
	}

	standalone := openStore(t)
	if err := standalone.Settings().PutAccounts(ctx, &core.AccountSettings{Mode: core.AccountModeStandalone}); err != nil {
		t.Fatal(err)
	}
	c.Store = standalone
	dev.AddClaimCode("code-4", account("acct-4", "carol"))
	if _, err := c.Claim(ctx, "code-4"); !errors.Is(err, cloud.ErrNotClaimable) {
		t.Errorf("Claim of a standalone server: %v, want ErrNotClaimable", err)
	}
}

func TestClaimUnreachable(t *testing.T) {
	c := &cloud.Claimer{Store: openStore(t), Client: cloud.NewClient("http://127.0.0.1:1/api", identity.Generate(), nil)}
	_, err := c.Claim(t.Context(), "code")
	if err == nil || errors.Is(err, cloud.ErrCodeRefused) || errors.Is(err, cloud.ErrNotClaimable) {
		t.Errorf("Claim with mavio.dev unreachable: %v, want a connection error", err)
	}
}

func TestLink(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	st := openStore(t)
	dev := cloudtest.New(t)
	key := identity.Generate()
	client := cloud.NewClient(dev.URL, key, nil)

	var (
		mu      sync.Mutex
		applied []int64
	)
	count := func() int { mu.Lock(); defer mu.Unlock(); return len(applied) }
	movies := core.Library{Name: "Movies", Kind: core.LibraryMovies, Paths: []string{"/media/movies"}}
	if err := st.Libraries().Create(ctx, &movies); err != nil {
		t.Fatal(err)
	}
	link := cloud.NewLink(st, client, nil)
	link.MinBackoff, link.MaxBackoff, link.PingInterval = 10*time.Millisecond, 50*time.Millisecond, 10*time.Millisecond
	link.LibrariesInterval = 10 * time.Millisecond
	link.Apply = func(_ context.Context, _ core.Store, c *cloudv1.Change) error {
		mu.Lock()
		defer mu.Unlock()
		applied = append(applied, c.GetCursor())
		return nil
	}
	done := make(chan struct{})
	go func() { link.Run(ctx); close(done) }()

	// Unclaimed, the link waits for the claim.
	claimer := &cloud.Claimer{Store: st, Client: client, Claimed: link.Wake}
	dev.AddClaimCode("code", account("acct-1", "alice"))
	if _, err := claimer.Claim(ctx, "code"); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the link connects", func() bool { return dev.Connections(key.ID()) == 1 })

	// The link publishes the libraries, and again when they change.
	eventually(t, "the libraries are published", func() bool {
		libs := dev.Libraries(key.ID())
		return len(libs) == 1 && libs[0].GetId() == movies.ID.String() && libs[0].GetKind() == "movies"
	})
	if err := st.Libraries().Create(ctx, &core.Library{Name: "Shows", Kind: core.LibraryShows, Paths: []string{"/media/shows"}}); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the new library is published", func() bool { return len(dev.Libraries(key.ID())) == 2 })

	// A notice fetches the change it announces.
	dev.AddChange(key.ID(), nil)
	eventually(t, "the first change is applied", func() bool { return count() == 1 })

	// Changes made while the server is disconnected are fetched when it
	// reconnects.
	dev.Disconnect(key.ID())
	dev.AddChange(key.ID(), nil)
	dev.AddChange(key.ID(), nil)
	eventually(t, "the missed changes are applied", func() bool { return count() == 3 })
	if a, _ := st.Settings().Accounts(ctx); a.Cursor != 3 {
		t.Errorf("cursor = %d, want = 3", a.Cursor)
	}
	mu.Lock()
	if applied[0] != 1 || applied[1] != 2 || applied[2] != 3 {
		t.Errorf("applied = %v, want [1 2 3]", applied)
	}
	mu.Unlock()

	cancel()
	<-done
}
