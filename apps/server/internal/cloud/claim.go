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
	// errUnconfirmed is returned while a claim request waits for an
	// account to confirm it.
	errUnconfirmed = errors.New("the claim request is not confirmed yet")
	// errReplaced is returned for a claim request a later one replaced.
	errReplaced = errors.New("a later claim request replaced this one")
)

// ClaimPollInterval is how often the server asks mavio.dev for the
// outcome of its claim request.
const ClaimPollInterval = 2 * time.Second

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
	// PollInterval overrides ClaimPollInterval when not zero.
	PollInterval time.Duration
	// mu serializes claims, so that one code's administrator is in place
	// before another claim checks the settings.
	mu sync.Mutex

	// pending is the claim request waiting for its confirmation, with its
	// expiry; wake tells Run of a new one.
	pmu     sync.Mutex
	pending string
	expires time.Time
	wake    chan struct{}
}

// ClaimRequest is what the web UI that starts a claim passes on to
// mavio.dev: where the browser comes back to, and the PKCE challenge and
// device key of the authorization code that signs it in.
type ClaimRequest struct {
	RedirectURI   string
	CodeChallenge string
	DeviceKey     []byte
	State         string
}

// Start asks mavio.dev for a claim request, which replaces any earlier
// one, and returns the page where an account confirms it and when it
// expires; Run then claims the server once it is confirmed. It returns
// ErrNotClaimable, or another error when mavio.dev cannot be reached.
func (c *Claimer) Start(ctx context.Context, r ClaimRequest) (string, time.Time, error) {
	settings, err := c.Store.Settings().Accounts(ctx)
	if err != nil {
		return "", time.Time{}, err
	}
	if !Claimable(settings) {
		return "", time.Time{}, ErrNotClaimable
	}
	req := cloudv1.CreateClaimRequestRequest_builder{
		PublicKey: c.Client.key.Public(), Version: &c.Version,
		RedirectUri: &r.RedirectURI, CodeChallenge: &r.CodeChallenge, DeviceKey: r.DeviceKey, State: &r.State,
	}.Build()
	if c.Name != nil {
		req.SetName(c.Name())
	}
	resp, err := c.Client.CreateClaimRequest(ctx, req)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("ask mavio.dev for a claim request: %w", err)
	}
	expires := resp.GetExpireTime().AsTime()
	c.pmu.Lock()
	c.pending, c.expires = resp.GetClaimRequest(), expires
	if c.wake == nil {
		c.wake = make(chan struct{}, 1)
	}
	wake := c.wake
	c.pmu.Unlock()
	select {
	case wake <- struct{}{}:
	default:
	}
	return resp.GetUrl(), expires, nil
}

// Run claims the server once an account confirms its claim request,
// asking mavio.dev every PollInterval while one waits, until ctx ends.
func (c *Claimer) Run(ctx context.Context) {
	c.pmu.Lock()
	if c.wake == nil {
		c.wake = make(chan struct{}, 1)
	}
	wake := c.wake
	c.pmu.Unlock()
	ticker := time.NewTicker(or(c.PollInterval, ClaimPollInterval))
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-wake:
		case <-ticker.C:
		}
		c.pmu.Lock()
		request, expires := c.pending, c.expires
		c.pmu.Unlock()
		if request == "" {
			continue
		}
		if time.Now().After(expires) {
			c.drop(request)
			continue
		}
		_, err := c.claim(ctx, cloudv1.ClaimServerRequest_builder{ClaimRequest: &request}.Build())
		switch {
		case errors.Is(err, errUnconfirmed), errors.Is(err, errReplaced):
		case err == nil, errors.Is(err, ErrNotClaimable), errors.Is(err, ErrCodeRefused):
			c.drop(request)
		default:
			slog.WarnContext(ctx, "ask mavio.dev for the claim request's outcome", "err", err)
		}
	}
}

// current reports whether a claim request is the one waiting.
func (c *Claimer) current(request string) bool {
	c.pmu.Lock()
	defer c.pmu.Unlock()
	return c.pending == request
}

// drop forgets a claim request unless another replaced it.
func (c *Claimer) drop(request string) {
	c.pmu.Lock()
	defer c.pmu.Unlock()
	if c.pending == request {
		c.pending = ""
	}
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

// claim claims the server with a claim code or a claim request.
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
	if code := connect.CodeOf(err); code == connect.CodeFailedPrecondition && req.GetClaimRequest() != "" {
		return core.User{}, errUnconfirmed
	} else if r := req.GetClaimRequest(); err == nil && r != "" && !c.current(r) {
		return core.User{}, errReplaced
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
