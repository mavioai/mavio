package rpc

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"sync"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mavioai/mavio/apps/server/internal/auth"
	"github.com/mavioai/mavio/libs/core"
	authv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1/authv1connect"
)

// AuthPublicProcedures are the AuthService methods callable without a token.
var AuthPublicProcedures = []string{
	authv1connect.AuthServiceGetAuthInfoProcedure,
	authv1connect.AuthServiceCreateFirstUserProcedure,
	authv1connect.AuthServiceLoginProcedure,
}

// AuthService implements mavio.auth.v1.AuthService.
type AuthService struct {
	store core.Store
	now   func() time.Time
	// firstUser serializes CreateFirstUser, so that two concurrent calls
	// cannot both see an empty user table.
	firstUser sync.Mutex
}

var _ authv1connect.AuthServiceHandler = (*AuthService)(nil)

// NewAuthService returns an AuthService backed by store.
func NewAuthService(store core.Store) *AuthService {
	return &AuthService{store: store, now: time.Now}
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
	if user.PasswordHash == "" {
		// Users of authentication plugins cannot sign in until plugins
		// can be asked.
		auth.SpendPasswordCheck(req.GetPassword())
		return nil, failed
	}
	ok, rehash, err := auth.VerifyPassword(user.PasswordHash, req.GetPassword())
	if err != nil {
		slog.ErrorContext(ctx, "unreadable password hash", "user", user.ID, "err", err)
		return nil, failed
	}
	if !ok || user.Disabled {
		return nil, failed
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
