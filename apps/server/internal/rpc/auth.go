package rpc

import (
	"context"
	"errors"
	"log/slog"
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
	authv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1/authv1connect"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1/libraryv1connect"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/playback/v1/playbackv1connect"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/session/v1/sessionv1connect"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1/userv1connect"
)

// AuthPublicProcedures are the AuthService methods callable without a token.
var AuthPublicProcedures = []string{
	authv1connect.AuthServiceGetAuthInfoProcedure,
	authv1connect.AuthServiceCreateFirstUserProcedure,
	authv1connect.AuthServiceLoginProcedure,
	authv1connect.AuthServiceStartQuickConnectProcedure,
	authv1connect.AuthServiceGetQuickConnectStateProcedure,
	authv1connect.AuthServiceLoginWithQuickConnectProcedure,
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
}

// SessionBoundProcedures need the caller's signed-in session, which plugins
// do not have (auth.Interceptor.RequireSession).
var SessionBoundProcedures = []string{
	authv1connect.AuthServiceLogoutProcedure,
	sessionv1connect.EventServiceName,
	sessionv1connect.SyncPlayServiceName,
}

// AuthService implements mavio.auth.v1.AuthService.
type AuthService struct {
	store core.Store
	now   func() time.Time
	// firstUser serializes CreateFirstUser, so that two concurrent calls
	// cannot both see an empty user table.
	firstUser sync.Mutex
	quick     *auth.QuickConnect
	// Activity records sign-ins and API keys; nil records none.
	Activity *activity.Log
	// Authenticate checks the credentials of users of authentication
	// plugins; nil lets none of them sign in.
	Authenticate func(ctx context.Context, pluginID, name, password string) (plugins.AuthResult, error)
}

var _ authv1connect.AuthServiceHandler = (*AuthService)(nil)

// NewAuthService returns an AuthService backed by store.
func NewAuthService(store core.Store) *AuthService {
	return &AuthService{store: store, now: time.Now, quick: auth.NewQuickConnect()}
}

// StartQuickConnect registers a device waiting to be signed in from
// another one.
func (s *AuthService) StartQuickConnect(_ context.Context, req *authv1.StartQuickConnectRequest) (*authv1.StartQuickConnectResponse, error) {
	d := req.GetDevice()
	r := s.quick.Start(auth.Device{ID: d.GetId(), Name: d.GetName(), Client: d.GetClient(), ClientVersion: d.GetClientVersion()})
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
	d, err := s.quick.Authorize(req.GetCode(), func(d auth.Device) (core.AuthSession, string, error) {
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
	return authv1.LoginWithQuickConnectResponse_builder{
		AccessToken: &r.Token, User: userToProto(&user), Session: sessionToProto(&r.Session, true),
	}.Build(), nil
}

func deviceToProto(d auth.Device) *authv1.Device {
	return authv1.Device_builder{Id: &d.ID, Name: &d.Name, Client: &d.Client, ClientVersion: &d.ClientVersion}.Build()
}

// GetAuthInfo reports whether the first user still has to be created.
func (s *AuthService) GetAuthInfo(ctx context.Context, _ *authv1.GetAuthInfoRequest) (*authv1.GetAuthInfoResponse, error) {
	users, err := s.store.Users().List(ctx)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return authv1.GetAuthInfoResponse_builder{SetupRequired: new(len(users) == 0)}.Build(), nil
}

// CreateFirstUser creates an administrator while no user exists and signs
// it in.
func (s *AuthService) CreateFirstUser(ctx context.Context, req *authv1.CreateFirstUserRequest) (*authv1.CreateFirstUserResponse, error) {
	s.firstUser.Lock()
	defer s.firstUser.Unlock()

	var (
		user  core.User
		sess  core.AuthSession
		token string
	)
	err := s.store.InTx(ctx, func(tx core.Store) error {
		users, err := tx.Users().List(ctx)
		if err != nil {
			return err
		}
		if len(users) > 0 {
			return connect.NewError(connect.CodeFailedPrecondition, errors.New("the server already has users"))
		}
		now := s.now()
		user = core.User{
			Name:         req.GetName(),
			PasswordHash: auth.HashPassword(req.GetPassword()),
			Admin:        true,
			Policy:       core.UserPolicy{AllowTranscoding: true, AllowDownload: true},
			CreatedAt:    now,
			LastLoginAt:  &now,
		}
		if err := tx.Users().Create(ctx, &user); err != nil {
			return err
		}
		sess, token, err = s.createSession(ctx, tx, user.ID, req.GetDevice())
		return err
	})
	if err != nil {
		return nil, connectError(ctx, err)
	}
	slog.InfoContext(ctx, "created first user", "user", user.Name)
	return authv1.CreateFirstUserResponse_builder{
		AccessToken: &token,
		User:        userToProto(&user),
		Session:     sessionToProto(&sess, true),
	}.Build(), nil
}

// Login checks a user's password and signs the device in.
func (s *AuthService) Login(ctx context.Context, req *authv1.LoginRequest) (*authv1.LoginResponse, error) {
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
	case user.PasswordHash == "":
		auth.SpendPasswordCheck(req.GetPassword())
		return nil, loginFailed("no password")
	default:
		ok, rehash, err = auth.VerifyPassword(user.PasswordHash, req.GetPassword())
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

	var (
		sess  core.AuthSession
		token string
	)
	err = s.store.InTx(ctx, func(tx core.Store) error {
		now := s.now()
		user.LastLoginAt = &now
		if rehash {
			user.PasswordHash = auth.HashPassword(req.GetPassword())
		}
		if err := tx.Users().Update(ctx, &user); err != nil {
			return err
		}
		var err error
		sess, token, err = s.createSession(ctx, tx, user.ID, req.GetDevice())
		return err
	})
	if err != nil {
		return nil, connectError(ctx, err)
	}
	s.Activity.Record(ctx, core.Activity{
		Type: "user.login", UserID: user.ID, Title: user.Name + " signed in",
		Attributes: map[string]string{"device": req.GetDevice().GetName(), "client": req.GetDevice().GetClient()},
	})
	return authv1.LoginResponse_builder{
		AccessToken: &token,
		User:        userToProto(&user),
		Session:     sessionToProto(&sess, true),
	}.Build(), nil
}

func (s *AuthService) createSession(ctx context.Context, tx core.Store, userID core.ID, d *authv1.Device) (core.AuthSession, string, error) {
	token, hash := auth.NewToken()
	now := s.now()
	sess := core.AuthSession{
		UserID:        userID,
		TokenHash:     hash,
		DeviceID:      d.GetId(),
		DeviceName:    d.GetName(),
		Client:        d.GetClient(),
		ClientVersion: d.GetClientVersion(),
		CreatedAt:     now,
		LastSeenAt:    now,
	}
	if err := tx.AuthSessions().Create(ctx, &sess); err != nil {
		return core.AuthSession{}, "", err
	}
	return sess, token, nil
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
