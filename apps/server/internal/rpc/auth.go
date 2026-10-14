package rpc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"slices"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mavioai/mavio/apps/server/internal/activity"
	"github.com/mavioai/mavio/apps/server/internal/auth"
	"github.com/mavioai/mavio/apps/server/internal/plugins"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1/acquisitionv1connect"
	authv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1/authv1connect"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/home/v1/homev1connect"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1/libraryv1connect"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/playback/v1/playbackv1connect"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/session/v1/sessionv1connect"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1/systemv1connect"
	userv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1/userv1connect"
)

// AuthPublicProcedures are the AuthService methods callable without a token.
var AuthPublicProcedures = []string{
	authv1connect.AuthServiceGetAuthInfoProcedure,
	authv1connect.AuthServiceCreateFirstUserProcedure,
	authv1connect.AuthServiceLoginProcedure,
	authv1connect.AuthServiceStartPasskeySignInProcedure,
	authv1connect.AuthServiceFinishPasskeySignInProcedure,
	authv1connect.AuthServiceStartRefreshProcedure,
	authv1connect.AuthServiceRefreshSessionProcedure,
	authv1connect.AuthServiceStartQuickConnectProcedure,
	authv1connect.AuthServiceGetQuickConnectStateProcedure,
	authv1connect.AuthServiceLoginWithQuickConnectProcedure,
	authv1connect.AuthServiceForgotPasswordProcedure,
	authv1connect.AuthServiceResetPasswordProcedure,
}

// UserBoundProcedures act on the caller's own state, so a plugin calls them
// only while acting as a user (auth.Interceptor.RequireUser).
var UserBoundProcedures = []string{
	authv1connect.AuthServiceListSessionsProcedure,
	authv1connect.AuthServiceRevokeSessionProcedure,
	authv1connect.AuthServiceAuthorizeQuickConnectProcedure,
	authv1connect.AuthServiceCreateApiKeyProcedure,
	userv1connect.UserServiceGetCurrentUserProcedure,
	userv1connect.UserServiceUpdatePreferencesProcedure,
	userv1connect.UserDataServiceName,
	userv1connect.DisplayPreferencesServiceName,
	playbackv1connect.PlaybackServiceName,
	libraryv1connect.PlaylistServiceName,
	acquisitionv1connect.RequestServiceCreateRequestProcedure,
	acquisitionv1connect.RequestServiceGetQuotaProcedure,
	acquisitionv1connect.IssueServiceCreateIssueProcedure,
	acquisitionv1connect.IssueServiceAddCommentProcedure,
	systemv1connect.ActivityServiceListMyActivitiesProcedure,
	homev1connect.HomeServiceName,
}

// SessionBoundProcedures need the caller's signed-in session, which plugins
// do not have (auth.Interceptor.RequireSession).
var SessionBoundProcedures = []string{
	authv1connect.AuthServiceLogoutProcedure,
	userv1connect.UserServiceStartPasskeyRegistrationProcedure,
	userv1connect.UserServiceFinishPasskeyRegistrationProcedure,
	userv1connect.UserServiceStartTotpProcedure,
	userv1connect.UserServiceConfirmTotpProcedure,
	sessionv1connect.EventServiceName,
	sessionv1connect.SyncPlayServiceName,
}

// AuthService implements mavio.auth.v1.AuthService.
type AuthService struct {
	store  core.Store
	tokens *auth.AccessTokens
	nonces *auth.Nonces
	// throttle slows down failed sign-ins per user name and address.
	throttle *auth.Throttle
	now      func() time.Time
	// firstUser serializes CreateFirstUser, so that two concurrent calls
	// cannot both see an empty user table.
	firstUser sync.Mutex
	quick     *auth.QuickConnect
	// Activity records sign-ins and API keys; nil records none.
	Activity *activity.Log
	// Passkeys signs users in with passkeys; nil lets none sign in.
	Passkeys *auth.Passkeys
	// TOTP checks the codes of users with TOTP; nil lets none of them sign
	// in.
	TOTP *auth.TOTP
	// Authenticate checks the credentials of users of authentication
	// plugins; nil lets none of them sign in.
	Authenticate func(ctx context.Context, pluginID, name, password string) (plugins.AuthResult, error)
	// Info names the server to clients before they sign in; nil names
	// nothing.
	Info func() ServerInfo
	// ResetPlugin names the plugin delivering password reset PINs, empty
	// when there is none; nil means none.
	ResetPlugin func() string
	// StartReset has a plugin deliver a password reset PIN to a user.
	StartReset func(ctx context.Context, pluginID string, user core.User, pin string, expires time.Time) error
	resets     *auth.PasswordResets
}

var _ authv1connect.AuthServiceHandler = (*AuthService)(nil)

// NewAuthService returns an AuthService backed by store, issuing access
// tokens with tokens.
func NewAuthService(store core.Store, tokens *auth.AccessTokens) *AuthService {
	return &AuthService{
		store: store, tokens: tokens, nonces: auth.NewNonces(), throttle: auth.NewThrottle(), now: time.Now,
		quick: auth.NewQuickConnect(), resets: auth.NewPasswordResets(),
	}
}

// StartQuickConnect registers a device waiting to be signed in from
// another one.
func (s *AuthService) StartQuickConnect(_ context.Context, req *authv1.StartQuickConnectRequest) (*authv1.StartQuickConnectResponse, error) {
	d := req.GetDevice()
	r := s.quick.Start(auth.Device{
		ID: d.GetId(), Name: d.GetName(), Client: d.GetClient(), ClientVersion: d.GetClientVersion(), Key: d.GetPublicKey(),
	})
	return authv1.StartQuickConnectResponse_builder{
		Secret: &r.Secret, Code: &r.Code, ExpireTime: timestamppb.New(r.Expires),
	}.Build(), nil
}

// GetQuickConnectState tells whether a request was authorized.
func (s *AuthService) GetQuickConnectState(ctx context.Context, req *authv1.GetQuickConnectStateRequest) (*authv1.GetQuickConnectStateResponse, error) {
	r, err := s.quick.State(req.GetSecret())
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return authv1.GetQuickConnectStateResponse_builder{Authorized: &r.Authorized, ExpireTime: timestamppb.New(r.Expires)}.Build(), nil
}

// AuthorizeQuickConnect signs the device waiting with a code in as the
// calling user.
func (s *AuthService) AuthorizeQuickConnect(ctx context.Context, req *authv1.AuthorizeQuickConnectRequest) (*authv1.AuthorizeQuickConnectResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	d, err := s.quick.Authorize(req.GetCode(), func(d auth.Device) (core.AuthSession, error) {
		return s.createSession(ctx, s.store, p.User.ID, deviceToProto(d))
	})
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return authv1.AuthorizeQuickConnectResponse_builder{Device: deviceToProto(d)}.Build(), nil
}

// LoginWithQuickConnect gives an authorized device its access token.
func (s *AuthService) LoginWithQuickConnect(ctx context.Context, req *authv1.LoginWithQuickConnectRequest) (*authv1.LoginWithQuickConnectResponse, error) {
	r, err := s.quick.Claim(req.GetSecret())
	if err != nil {
		return nil, connectError(ctx, err)
	}
	user, err := s.store.Users().Get(ctx, r.Session.UserID)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	token, expires := s.tokens.Issue(r.Session.ID)
	return authv1.LoginWithQuickConnectResponse_builder{
		AccessToken: &token, AccessTokenExpireTime: timestamppb.New(expires),
		User: userToProto(&user, nil), Session: sessionToProto(&r.Session, true),
	}.Build(), nil
}

// StartRefresh gives a session's device a nonce to sign.
func (s *AuthService) StartRefresh(ctx context.Context, req *authv1.StartRefreshRequest) (*authv1.StartRefreshResponse, error) {
	id, err := core.ParseID(req.GetSessionId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if _, err := s.store.AuthSessions().Get(ctx, id); errors.Is(err, core.ErrNotFound) {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("no such session"))
	} else if err != nil {
		return nil, connectError(ctx, err)
	}
	nonce, expires := s.nonces.Start(id)
	return authv1.StartRefreshResponse_builder{Nonce: nonce, ExpireTime: timestamppb.New(expires)}.Build(), nil
}

// RefreshSession issues a new access token for the device's signature of
// its nonce, while the session's user is enabled and the session has not
// been idle for too long.
func (s *AuthService) RefreshSession(ctx context.Context, req *authv1.RefreshSessionRequest) (*authv1.RefreshSessionResponse, error) {
	failed := connect.NewError(connect.CodeUnauthenticated, errors.New("the session cannot be refreshed; sign in again"))
	id, err := core.ParseID(req.GetSessionId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if !s.nonces.Redeem(id, req.GetNonce()) {
		return nil, failed
	}
	sess, err := s.store.AuthSessions().Get(ctx, id)
	if errors.Is(err, core.ErrNotFound) {
		return nil, failed
	} else if err != nil {
		return nil, connectError(ctx, err)
	}
	if !auth.VerifyRefresh(sess.DeviceKey, id, req.GetNonce(), req.GetSignature()) {
		return nil, failed
	}
	settings, err := s.store.Settings().Get(ctx)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	now := s.now()
	if now.After(sess.LastSeenAt.AddDate(0, 0, settings.SessionIdleDays)) {
		return nil, failed
	}
	user, err := s.store.Users().Get(ctx, sess.UserID)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	if user.Disabled {
		return nil, failed
	}
	if err := s.store.AuthSessions().Touch(ctx, id, now); err != nil {
		return nil, connectError(ctx, err)
	}
	token, expires := s.tokens.Issue(id)
	return authv1.RefreshSessionResponse_builder{AccessToken: &token, AccessTokenExpireTime: timestamppb.New(expires)}.Build(), nil
}

func accountModeToProto(m core.AccountMode) authv1.AccountMode {
	switch m {
	case core.AccountModeStandalone:
		return authv1.AccountMode_ACCOUNT_MODE_STANDALONE
	case core.AccountModeMavioDev:
		return authv1.AccountMode_ACCOUNT_MODE_MAVIO_DEV
	default:
		return authv1.AccountMode_ACCOUNT_MODE_UNSPECIFIED
	}
}

func deviceToProto(d auth.Device) *authv1.Device {
	return authv1.Device_builder{Id: &d.ID, Name: &d.Name, Client: &d.Client, ClientVersion: &d.ClientVersion, PublicKey: d.Key}.Build()
}

// GetAuthInfo reports the server's mode, and whether setup still has to
// choose it.
func (s *AuthService) GetAuthInfo(ctx context.Context, _ *authv1.GetAuthInfoRequest) (*authv1.GetAuthInfoResponse, error) {
	accounts, err := s.store.Settings().Accounts(ctx)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	b := authv1.GetAuthInfoResponse_builder{
		SetupRequired: new(accounts.Mode == core.AccountModeUnset),
		Mode:          new(accountModeToProto(accounts.Mode)),
	}
	if s.Info != nil {
		i := s.Info()
		b.ServerId, b.ServerName, b.Version = &i.ID, &i.Name, &i.Version
	}
	return b.Build(), nil
}

// CreateFirstUser sets up a server in standalone mode while its mode is
// unset, creating the administrator and signing it in.
func (s *AuthService) CreateFirstUser(ctx context.Context, req *authv1.CreateFirstUserRequest) (*authv1.CreateFirstUserResponse, error) {
	s.firstUser.Lock()
	defer s.firstUser.Unlock()

	var (
		user core.User
		sess core.AuthSession
	)
	err := s.store.InTx(ctx, func(tx core.Store) error {
		accounts, err := tx.Settings().Accounts(ctx)
		if err != nil {
			return err
		}
		if accounts.Mode != core.AccountModeUnset {
			return connect.NewError(connect.CodeFailedPrecondition, errors.New("the server is already set up"))
		}
		if err := tx.Settings().PutAccounts(ctx, &core.AccountSettings{Mode: core.AccountModeStandalone}); err != nil {
			return err
		}
		now := s.now()
		user = core.User{
			Name:        req.GetName(),
			Kind:        core.UserAdmin,
			Policy:      core.UserPolicy{AllowTranscoding: true, AllowDownload: true},
			CreatedAt:   now,
			LastLoginAt: &now,
		}
		if err := tx.Users().Create(ctx, &user); err != nil {
			return err
		}
		if err := putSecret(ctx, tx, user.ID, core.CredentialPassword, req.GetPassword()); err != nil {
			return err
		}
		sess, err = s.createSession(ctx, tx, user.ID, req.GetDevice())
		return err
	})
	if err != nil {
		return nil, connectError(ctx, err)
	}
	slog.InfoContext(ctx, "created first user", "user", user.Name)
	token, expires := s.tokens.Issue(sess.ID)
	return authv1.CreateFirstUserResponse_builder{
		AccessToken:           &token,
		AccessTokenExpireTime: timestamppb.New(expires),
		User:                  userToProto(&user, nil),
		Session:               sessionToProto(&sess, true),
	}.Build(), nil
}

// Login checks a user's password and signs the device in. Failed
// sign-ins slow down progressively per user name and per address.
func (s *AuthService) Login(ctx context.Context, req *authv1.LoginRequest) (*authv1.LoginResponse, error) {
	keys := signInKeys(ctx, req.GetName())
	if err := s.throttled(keys); err != nil {
		return nil, err
	}
	resp, err := s.login(ctx, req)
	switch {
	case connect.CodeOf(err) == connect.CodeUnauthenticated:
		s.throttle.Fail(keys...)
	case err == nil:
		s.throttle.Succeed(keys[0])
	}
	return resp, err
}

// signInKeys are the throttle's keys of a sign-in as name: the name,
// whether or not a user has it, and the caller's address.
func signInKeys(ctx context.Context, name string) []string {
	keys := []string{"user:" + strings.ToLower(name)}
	if info, ok := connect.CallInfoForHandlerContext(ctx); ok {
		if host, _, err := net.SplitHostPort(info.Peer().Addr); err == nil {
			keys = append(keys, "addr:"+host)
		}
	}
	return keys
}

// throttled fails while any of keys is locked out.
func (s *AuthService) throttled(keys []string) error {
	if wait := s.throttle.Wait(keys...); wait > 0 {
		return connect.NewError(connect.CodeResourceExhausted,
			fmt.Errorf("too many failed sign-ins; try again in %s", wait.Round(time.Second)))
	}
	return nil
}

func (s *AuthService) login(ctx context.Context, req *authv1.LoginRequest) (*authv1.LoginResponse, error) {
	failed := connect.NewError(connect.CodeUnauthenticated, errors.New("wrong user name or password"))
	user, err := s.store.Users().GetByName(ctx, req.GetName())
	if errors.Is(err, core.ErrNotFound) {
		auth.SpendPasswordCheck(req.GetPassword())
		return nil, failed
	} else if err != nil {
		return nil, connectError(ctx, err)
	}
	loginFailed := func(reason string) error {
		s.Activity.Record(ctx, core.Activity{
			Type: "user.login_failed", Severity: core.SeverityWarning, UserID: user.ID,
			Title: "Failed sign-in as " + user.Name, Message: reason, Attributes: map[string]string{"device": req.GetDevice().GetName()},
		})
		return failed
	}
	var ok, rehash bool
	switch {
	case user.AuthProvider != "":
		if s.Authenticate == nil {
			auth.SpendPasswordCheck(req.GetPassword())
			return nil, loginFailed("no authentication plugins run")
		}
		res, err := s.Authenticate(ctx, user.AuthProvider, user.Name, req.GetPassword())
		if err != nil {
			slog.WarnContext(ctx, "authentication plugin failed", "user", user.ID, "plugin", user.AuthProvider, "err", err)
			return nil, loginFailed("the authentication plugin failed")
		}
		ok = res.Authenticated
		if !ok && res.Message != "" {
			slog.InfoContext(ctx, "authentication plugin rejected a sign-in", "user", user.ID, "reason", res.Message)
		}
	default:
		password, err := s.store.Credentials().Get(ctx, user.ID, core.CredentialPassword)
		if errors.Is(err, core.ErrNotFound) {
			auth.SpendPasswordCheck(req.GetPassword())
			return nil, loginFailed("no password")
		} else if err != nil {
			return nil, connectError(ctx, err)
		}
		ok, rehash, err = auth.VerifyPassword(password.Data, req.GetPassword())
		if err != nil {
			slog.ErrorContext(ctx, "unreadable password hash", "user", user.ID, "err", err)
			return nil, loginFailed("unreadable password hash")
		}
	}
	if !ok {
		return nil, loginFailed("wrong password")
	}
	if user.Disabled {
		return nil, loginFailed("disabled user")
	}
	totp, err := s.store.Credentials().Get(ctx, user.ID, core.CredentialTOTP)
	hasTOTP := err == nil
	switch {
	case errors.Is(err, core.ErrNotFound):
	case err != nil:
		return nil, connectError(ctx, err)
	case req.GetTotpCode() == "":
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("a TOTP code is required"))
	case s.TOTP == nil:
		return nil, loginFailed("TOTP is unavailable")
	default:
		used, ok, err := s.TOTP.Verify(user.ID, totp.Data, req.GetTotpCode(), totp.LastUsedAt)
		if err != nil {
			slog.ErrorContext(ctx, "unreadable TOTP secret", "user", user.ID, "err", err)
			return nil, loginFailed("unreadable TOTP secret")
		} else if !ok {
			return nil, loginFailed("wrong TOTP code")
		}
		totp.LastUsedAt = &used
	}
	in, err := s.signIn(ctx, user, req.GetDevice(), "password", func(tx core.Store) error {
		if rehash {
			if err := putSecret(ctx, tx, user.ID, core.CredentialPassword, req.GetPassword()); err != nil {
				return err
			}
		}
		if hasTOTP {
			return tx.Credentials().Put(ctx, &totp)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return authv1.LoginResponse_builder{
		AccessToken:           &in.token,
		AccessTokenExpireTime: timestamppb.New(in.expires),
		User:                  in.user,
		Session:               sessionToProto(&in.session, true),
	}.Build(), nil
}

// StartPasskeySignIn begins signing in with a passkey at the request's
// origin.
func (s *AuthService) StartPasskeySignIn(ctx context.Context, _ *authv1.StartPasskeySignInRequest) (*authv1.StartPasskeySignInResponse, error) {
	if s.Passkeys == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("passkeys are unavailable"))
	}
	rp, err := requestRelyingParty(ctx)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	id, options, err := s.Passkeys.StartSignIn(rp)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return authv1.StartPasskeySignInResponse_builder{CeremonyId: &id, OptionsJson: new(string(options))}.Build(), nil
}

// FinishPasskeySignIn signs a device in as the user whose passkey
// answered. Failures slow down per address, as Login's.
func (s *AuthService) FinishPasskeySignIn(ctx context.Context, req *authv1.FinishPasskeySignInRequest) (*authv1.FinishPasskeySignInResponse, error) {
	if s.Passkeys == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("passkeys are unavailable"))
	}
	keys := signInKeys(ctx, "")[1:]
	if err := s.throttled(keys); err != nil {
		return nil, err
	}
	failed := connect.NewError(connect.CodeUnauthenticated, errors.New("passkey not accepted"))
	user, cred, err := s.Passkeys.FinishSignIn(req.GetCeremonyId(), []byte(req.GetCredentialJson()), func(id core.ID) (core.User, []core.Credential, error) {
		u, err := s.store.Users().Get(ctx, id)
		if err != nil {
			return core.User{}, nil, err
		}
		creds, err := s.store.Credentials().List(ctx, id)
		return u, creds, err
	})
	switch {
	case errors.Is(err, auth.ErrPasskey):
		slog.InfoContext(ctx, "passkey sign-in failed", "err", err)
		s.throttle.Fail(keys...)
		return nil, failed
	case err != nil:
		return nil, connectError(ctx, err)
	case user.Disabled || user.Kind == core.UserManaged || user.AuthProvider != "":
		s.Activity.Record(ctx, core.Activity{
			Type: "user.login_failed", Severity: core.SeverityWarning, UserID: user.ID,
			Title: "Failed sign-in as " + user.Name, Message: "the user cannot sign in with a passkey",
			Attributes: map[string]string{"device": req.GetDevice().GetName()},
		})
		s.throttle.Fail(keys...)
		return nil, failed
	}
	now := s.now()
	cred.LastUsedAt = &now
	in, err := s.signIn(ctx, user, req.GetDevice(), "passkey", func(tx core.Store) error {
		return tx.Credentials().Put(ctx, &cred)
	})
	if err != nil {
		return nil, err
	}
	return authv1.FinishPasskeySignInResponse_builder{
		AccessToken:           &in.token,
		AccessTokenExpireTime: timestamppb.New(in.expires),
		User:                  in.user,
		Session:               sessionToProto(&in.session, true),
	}.Build(), nil
}

// requestRelyingParty returns the relying party of the request's origin.
func requestRelyingParty(ctx context.Context) (auth.RelyingParty, error) {
	var origin string
	if info, ok := connect.CallInfoForHandlerContext(ctx); ok {
		origin = info.RequestHeader().Get("Origin")
	}
	return auth.RelyingPartyOf(origin)
}

// signedIn is a new session with its access token.
type signedIn struct {
	token   string
	expires time.Time
	session core.AuthSession
	user    *userv1.User
}

// signIn records the user's sign-in on the device by method, after
// writing what also writes, and starts its session.
func (s *AuthService) signIn(ctx context.Context, user core.User, d *authv1.Device, method string, also func(tx core.Store) error) (signedIn, error) {
	var sess core.AuthSession
	err := s.store.InTx(ctx, func(tx core.Store) error {
		now := s.now()
		user.LastLoginAt = &now
		if err := tx.Users().Update(ctx, &user); err != nil {
			return err
		}
		if err := also(tx); err != nil {
			return err
		}
		var err error
		sess, err = s.createSession(ctx, tx, user.ID, d)
		return err
	})
	if err != nil {
		return signedIn{}, connectError(ctx, err)
	}
	s.Activity.Record(ctx, core.Activity{
		Type: "user.login", UserID: user.ID, Title: user.Name + " signed in",
		Attributes: map[string]string{"device": d.GetName(), "client": d.GetClient(), "method": method},
	})
	described, err := describeUser(ctx, s.store, &user)
	if err != nil {
		return signedIn{}, connectError(ctx, err)
	}
	token, expires := s.tokens.Issue(sess.ID)
	return signedIn{token: token, expires: expires, session: sess, user: described}, nil
}

func (s *AuthService) createSession(ctx context.Context, tx core.Store, userID core.ID, d *authv1.Device) (core.AuthSession, error) {
	now := s.now()
	sess := core.AuthSession{
		UserID:        userID,
		DeviceKey:     d.GetPublicKey(),
		DeviceID:      d.GetId(),
		DeviceName:    d.GetName(),
		Client:        d.GetClient(),
		ClientVersion: d.GetClientVersion(),
		CreatedAt:     now,
		LastSeenAt:    now,
	}
	if err := tx.AuthSessions().Create(ctx, &sess); err != nil {
		return core.AuthSession{}, err
	}
	return sess, nil
}

// Logout ends the calling session.
func (s *AuthService) Logout(ctx context.Context, _ *authv1.LogoutRequest) (*authv1.LogoutResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.store.AuthSessions().Delete(ctx, p.Session.ID); err != nil && !errors.Is(err, core.ErrNotFound) {
		return nil, connectError(ctx, err)
	}
	return &authv1.LogoutResponse{}, nil
}

// ListSessions lists the calling user's sessions.
func (s *AuthService) ListSessions(ctx context.Context, _ *authv1.ListSessionsRequest) (*authv1.ListSessionsResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	list, err := s.store.AuthSessions().ListForUser(ctx, p.User.ID)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	out := make([]*authv1.Session, len(list))
	for i := range list {
		out[i] = sessionToProto(&list[i], list[i].ID == p.Session.ID)
	}
	return authv1.ListSessionsResponse_builder{Sessions: out}.Build(), nil
}

// RevokeSession ends one of the calling user's sessions.
func (s *AuthService) RevokeSession(ctx context.Context, req *authv1.RevokeSessionRequest) (*authv1.RevokeSessionResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	id, err := core.ParseID(req.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	// Other users' sessions are reported as missing.
	list, err := s.store.AuthSessions().ListForUser(ctx, p.User.ID)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	if !slices.ContainsFunc(list, func(s core.AuthSession) bool { return s.ID == id }) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no such session"))
	}
	if err := s.store.AuthSessions().Delete(ctx, id); err != nil {
		return nil, connectError(ctx, err)
	}
	return &authv1.RevokeSessionResponse{}, nil
}

// principal returns the signed-in caller; the auth interceptor guarantees
// one for every non-public procedure.
func principal(ctx context.Context) (auth.Principal, error) {
	p, ok := auth.FromContext(ctx)
	if !ok {
		return auth.Principal{}, connect.NewError(connect.CodeUnauthenticated, errors.New("not signed in"))
	}
	return p, nil
}

func sessionToProto(s *core.AuthSession, current bool) *authv1.Session {
	return authv1.Session_builder{
		Id: new(s.ID.String()),
		Device: authv1.Device_builder{
			Id:            &s.DeviceID,
			Name:          &s.DeviceName,
			Client:        &s.Client,
			ClientVersion: &s.ClientVersion,
			PublicKey:     s.DeviceKey,
		}.Build(),
		CreateTime:   timestamppb.New(s.CreatedAt),
		LastSeenTime: timestamppb.New(s.LastSeenAt),
		Current:      &current,
	}.Build()
}

// CreateApiKey makes an API key acting as the calling administrator.
func (s *AuthService) CreateApiKey(ctx context.Context, req *authv1.CreateApiKeyRequest) (*authv1.CreateApiKeyResponse, error) { //nolint:revive // generated name
	p, err := admin(ctx)
	if err != nil {
		return nil, err
	}
	token, hash := auth.NewToken()
	k := core.APIKey{UserID: p.User.ID, Name: strings.TrimSpace(req.GetName()), TokenHash: hash, CreatedAt: s.now()}
	if err := s.store.APIKeys().Create(ctx, &k); err != nil {
		return nil, connectError(ctx, err)
	}
	s.Activity.Record(ctx, core.Activity{
		Type: "apikey.created", UserID: p.User.ID, Title: p.User.Name + " created the API key " + k.Name,
	})
	return authv1.CreateApiKeyResponse_builder{ApiKey: apiKeyToProto(&k), Token: &token}.Build(), nil
}

// ListApiKeys lists the API keys.
func (s *AuthService) ListApiKeys(ctx context.Context, _ *authv1.ListApiKeysRequest) (*authv1.ListApiKeysResponse, error) { //nolint:revive // generated name
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	keys, err := s.store.APIKeys().List(ctx)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	out := make([]*authv1.ApiKey, len(keys))
	for i := range keys {
		out[i] = apiKeyToProto(&keys[i])
	}
	return authv1.ListApiKeysResponse_builder{ApiKeys: out}.Build(), nil
}

// RevokeApiKey deletes an API key.
func (s *AuthService) RevokeApiKey(ctx context.Context, req *authv1.RevokeApiKeyRequest) (*authv1.RevokeApiKeyResponse, error) { //nolint:revive // generated name
	p, err := admin(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.store.APIKeys().Delete(ctx, core.MustParseID(req.GetId())); err != nil {
		return nil, connectError(ctx, err)
	}
	s.Activity.Record(ctx, core.Activity{Type: "apikey.revoked", UserID: p.User.ID, Title: p.User.Name + " revoked an API key"})
	return &authv1.RevokeApiKeyResponse{}, nil
}

func apiKeyToProto(k *core.APIKey) *authv1.ApiKey {
	out := authv1.ApiKey_builder{
		Id: new(k.ID.String()), Name: &k.Name, UserId: new(k.UserID.String()), CreateTime: timestamppb.New(k.CreatedAt),
	}.Build()
	if k.LastUsedAt != nil {
		out.SetLastUseTime(timestamppb.New(*k.LastUsedAt))
	}
	return out
}

// ForgotPassword has the password reset plugin deliver a PIN to a user
// who signs in with a password. It answers alike for every name, so that
// it tells nothing about which users exist.
func (s *AuthService) ForgotPassword(ctx context.Context, req *authv1.ForgotPasswordRequest) (*authv1.ForgotPasswordResponse, error) {
	pluginID := ""
	if s.ResetPlugin != nil {
		pluginID = s.ResetPlugin()
	}
	if pluginID == "" || s.StartReset == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("passwords cannot be reset; ask an administrator"))
	}
	expires := s.now().Add(auth.ResetTimeout)
	resp := authv1.ForgotPasswordResponse_builder{ExpireTime: timestamppb.New(expires)}.Build()
	user, err := s.store.Users().GetByName(ctx, req.GetName())
	if err != nil && !errors.Is(err, core.ErrNotFound) {
		return nil, connectError(ctx, err)
	}
	if err != nil || user.Disabled || user.AuthProvider != "" {
		// As costly as starting a reset.
		auth.SpendPasswordCheck("")
		return resp, nil
	}
	pin, expires := s.resets.Start(user.ID)
	// The PIN is delivered in the background, so that the answer takes
	// as long for every name.
	ctx = context.WithoutCancel(ctx)
	go func() {
		ctx, cancel := context.WithTimeout(ctx, time.Minute)
		defer cancel()
		if err := s.StartReset(ctx, pluginID, user, pin, expires); err != nil {
			s.resets.Cancel(user.ID)
			slog.WarnContext(ctx, "password reset plugin failed", "user", user.ID, "plugin", pluginID, "err", err)
			s.Activity.Record(ctx, core.Activity{
				Type: "user.password_reset_failed", Severity: core.SeverityWarning, UserID: user.ID,
				Title: "Failed to send a password reset PIN to " + user.Name, Message: err.Error(),
			})
			return
		}
		s.Activity.Record(ctx, core.Activity{
			Type: "user.password_reset_started", UserID: user.ID, Title: "Password reset PIN sent to " + user.Name,
		})
	}()
	return resp, nil
}

// ResetPassword sets a new password with a PIN ForgotPassword delivered,
// ending the user's sessions.
func (s *AuthService) ResetPassword(ctx context.Context, req *authv1.ResetPasswordRequest) (*authv1.ResetPasswordResponse, error) {
	failed := connect.NewError(connect.CodeUnauthenticated, errors.New("wrong user name or PIN"))
	user, err := s.store.Users().GetByName(ctx, req.GetName())
	if errors.Is(err, core.ErrNotFound) {
		auth.SpendPasswordCheck(req.GetPin())
		return nil, failed
	} else if err != nil {
		return nil, connectError(ctx, err)
	}
	if !s.resets.Redeem(user.ID, req.GetPin()) {
		s.Activity.Record(ctx, core.Activity{
			Type: "user.password_reset_failed", Severity: core.SeverityWarning, UserID: user.ID,
			Title: "Failed password reset of " + user.Name, Message: "wrong PIN",
		})
		return nil, failed
	}
	err = s.store.InTx(ctx, func(tx core.Store) error {
		u, err := tx.Users().Get(ctx, user.ID)
		if err != nil {
			return err
		}
		if u.Disabled || u.AuthProvider != "" {
			return failed
		}
		if err := putSecret(ctx, tx, u.ID, core.CredentialPassword, req.GetNewPassword()); err != nil {
			return err
		}
		if u.Kind == core.UserManaged {
			u.Kind, u.Restriction = core.UserMember, core.RestrictionNone
			if err := tx.Users().Update(ctx, &u); err != nil {
				return err
			}
		}
		sessions, err := tx.AuthSessions().ListForUser(ctx, u.ID)
		if err != nil {
			return err
		}
		for _, sess := range sessions {
			if err := tx.AuthSessions().Delete(ctx, sess.ID); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, connectError(ctx, err)
	}
	s.Activity.Record(ctx, core.Activity{
		Type: "user.password_reset", UserID: user.ID, Title: "Password of " + user.Name + " reset",
	})
	return &authv1.ResetPasswordResponse{}, nil
}

// putSecret hashes secret and stores it as the user's credential of kind.
func putSecret(ctx context.Context, tx core.Store, userID core.ID, kind core.CredentialKind, secret string) error {
	return tx.Credentials().Put(ctx, &core.Credential{UserID: userID, Kind: kind, Data: auth.HashPassword(secret)})
}
