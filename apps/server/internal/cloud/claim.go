package cloud

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"sync"
	"time"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/apps/server/internal/activity"
	"github.com/mavioai/mavio/libs/core"
	cloudv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/cloud/v1"
)

var (
	// ErrStandalone is returned when a standalone server is asked to sign
	// in with a Mavio account.
	ErrStandalone = errors.New("the server is standalone")
	// ErrNotClaimable is returned when the server is standalone or
	// claimed.
	ErrNotClaimable = errors.New("the server is standalone or already claimed")
	// ErrCodeRefused is returned when mavio.dev refuses a claim code.
	ErrCodeRefused = errors.New("mavio.dev refused the claim code")
	// errUnapproved is returned while an authorization request that claims
	// the server waits for an account to approve it.
	errUnapproved = errors.New("the authorization request is not approved yet")
)

// Claimable reports whether a server with these settings may be claimed:
// its mode is unset, or it is an unclaimed server in mavio.dev mode.
func Claimable(a core.AccountSettings) bool {
	return a.Mode == core.AccountModeUnset || a.Mode == core.AccountModeMavioDev && !a.Claimed()
}

// Claimer signs Mavio accounts in to the server's web UI and claims the
// server at mavio.dev.
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

	// pending are the authorization requests that claim the server, by
	// their expiry, until one is approved.
	pmu     sync.Mutex
	pending map[string]time.Time
}

// SignInRequest is what the web UI that starts signing in passes on to
// mavio.dev: where the browser comes back to, and the PKCE challenge and
// device key of the authorization code that signs it in.
type SignInRequest struct {
	RedirectURI   string
	CodeChallenge string
	DeviceKey     []byte
	State         string
}

// StartSignIn pushes the authorization request of a web UI to mavio.dev
// and returns the page where the account approves it and when it
// expires. While the server is claimable the request claims it, and
// Complete then claims the server once it is approved. It returns
// ErrStandalone, or another error when mavio.dev cannot be reached.
func (c *Claimer) StartSignIn(ctx context.Context, r SignInRequest) (string, time.Time, error) {
	settings, err := c.Store.Settings().Accounts(ctx)
	if err != nil {
		return "", time.Time{}, err
	}
	if settings.Mode == core.AccountModeStandalone {
		return "", time.Time{}, ErrStandalone
	}
	claim := Claimable(settings)
	req := cloudv1.CreateAuthorizationRequestRequest_builder{
		PublicKey: c.Client.key.Public(), Version: &c.Version,
		RedirectUri: &r.RedirectURI, CodeChallenge: &r.CodeChallenge, DeviceKey: r.DeviceKey, State: &r.State, Claim: &claim,
	}.Build()
	if c.Name != nil {
		req.SetName(c.Name())
	}
	resp, err := c.Client.CreateAuthorizationRequest(ctx, req)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("push the authorization request to mavio.dev: %w", err)
	}
	expires := resp.GetExpireTime().AsTime()
	if claim {
		c.pmu.Lock()
		if c.pending == nil {
			c.pending = map[string]time.Time{}
		}
		c.pending[resp.GetAuthorizationRequest()] = expires
		c.pmu.Unlock()
	}
	return resp.GetUrl(), expires, nil
}

// Complete claims the server when an account approved one of its
// authorization requests that claim it, the latest first; it does
// nothing when none was approved or the server is not claimable. It
// returns an error when mavio.dev cannot be reached.
func (c *Claimer) Complete(ctx context.Context) error {
	c.pmu.Lock()
	now := time.Now()
	maps.DeleteFunc(c.pending, func(_ string, expires time.Time) bool { return now.After(expires) })
	requests := slices.SortedFunc(maps.Keys(c.pending), func(a, b string) int { return c.pending[b].Compare(c.pending[a]) })
	c.pmu.Unlock()
	for _, id := range requests {
		_, err := c.claim(ctx, cloudv1.ClaimServerRequest_builder{AuthorizationRequest: &id}.Build())
		switch {
		case errors.Is(err, errUnapproved):
			continue
		case errors.Is(err, ErrCodeRefused):
			c.drop(id)
			continue
		case errors.Is(err, ErrNotClaimable):
			c.dropAll()
			return nil
		case err != nil:
			return err
		}
		c.dropAll()
		return nil
	}
	return nil
}

// drop forgets an authorization request.
func (c *Claimer) drop(id string) {
	c.pmu.Lock()
	defer c.pmu.Unlock()
	delete(c.pending, id)
}

// dropAll forgets every authorization request, once the server is claimed.
func (c *Claimer) dropAll() {
	c.pmu.Lock()
	defer c.pmu.Unlock()
	clear(c.pending)
}

// Claim claims the server with a claim code, making its account the
// administrator: its user, when the server has one, becomes the
// administrator, and any other administrator a member. A server claimed
// again keeps its cursor, so that it still applies the changes it missed.
// It returns ErrNotClaimable, ErrCodeRefused wrapped, or another error
// when mavio.dev cannot be reached.
func (c *Claimer) Claim(ctx context.Context, code string) (core.User, error) {
	return c.claim(ctx, cloudv1.ClaimServerRequest_builder{ClaimCode: &code}.Build())
}

// claim claims the server with a claim code or an authorization request.
func (c *Claimer) claim(ctx context.Context, req *cloudv1.ClaimServerRequest) (core.User, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	settings, err := c.Store.Settings().Accounts(ctx)
	if err != nil {
		return core.User{}, err
	}
	if !Claimable(settings) {
		return core.User{}, ErrNotClaimable
	}
	req.SetPublicKey(c.Client.key.Public())
	req.SetVersion(c.Version)
	if c.Name != nil {
		req.SetName(c.Name())
	}
	resp, err := c.Client.ClaimServer(ctx, req)
	if code := connect.CodeOf(err); code == connect.CodeFailedPrecondition && req.GetAuthorizationRequest() != "" {
		return core.User{}, errUnapproved
	} else if code == connect.CodeInvalidArgument || code == connect.CodeNotFound || code == connect.CodePermissionDenied {
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
