package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/libs/core"
)

// touchInterval is how often a session's last activity is recorded.
const touchInterval = time.Minute

// Principal is the signed-in user making a request, through a session, an
// API key or a plugin's host API.
type Principal struct {
	User    core.User
	Session core.AuthSession
	// APIKey is the key of a request made with one; Session is zero then.
	APIKey core.ID
	// Plugin is the ID of the plugin making a host API request; Session is
	// zero then, unless it acts as one of its devices. A plugin acting as
	// itself has an administrator User with a zero ID.
	Plugin string
}

type principalKey struct{}

// WithPrincipal returns ctx carrying p.
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

// FromContext returns the principal of an authenticated request.
func FromContext(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok
}

// Interceptor authenticates Connect requests by their bearer token.
type Interceptor struct {
	store        core.Store
	public       map[string]bool
	userBound    map[string]bool
	sessionBound map[string]bool
	now          func() time.Time
}

var _ connect.Interceptor = (*Interceptor)(nil)

// NewInterceptor returns an interceptor that requires a valid token for
// every procedure except the public ones, given as Connect procedure names
// such as "/mavio.auth.v1.AuthService/Login".
func NewInterceptor(store core.Store, public ...string) *Interceptor {
	i := &Interceptor{
		store: store, public: make(map[string]bool, len(public)),
		userBound: map[string]bool{}, sessionBound: map[string]bool{}, now: time.Now,
	}
	for _, p := range public {
		i.public[p] = true
	}
	return i
}

// WrapUnary implements connect.Interceptor.
func (i *Interceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		if req.Spec().IsClient {
			return next(ctx, req)
		}
		ctx, err := i.authenticate(ctx, req.Spec(), req.Header())
		if err != nil {
			return nil, err
		}
		return next(ctx, req)
	}
}

// WrapStreamingClient implements connect.Interceptor.
func (i *Interceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

// WrapStreamingHandler implements connect.Interceptor.
func (i *Interceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		ctx, err := i.authenticate(ctx, conn.Spec(), conn.RequestHeader())
		if err != nil {
			return err
		}
		return next(ctx, conn)
	}
}

func (i *Interceptor) authenticate(ctx context.Context, spec connect.Spec, h http.Header) (context.Context, error) {
	procedure := spec.Procedure
	if g, ok := ctx.Value(pluginKey{}).(Plugin); ok {
		p, err := i.authenticatePlugin(ctx, g, procedure, spec.IdempotencyLevel == connect.IdempotencyNoSideEffects, h)
		if err != nil {
			return ctx, err
		}
		return WithPrincipal(ctx, p), nil
	}
	if i.public[procedure] {
		return ctx, nil
	}
	token, ok := bearer(h)
	if !ok {
		return ctx, connect.NewError(connect.CodeUnauthenticated, errors.New("missing bearer token"))
	}
	p, err := i.Resolve(ctx, token)
	if err != nil {
		return ctx, err
	}
	return WithPrincipal(ctx, p), nil
}

// Resolve returns the principal of a token: a session's, or an API key's,
// which acts as the administrator who created it while they still are one.
func (i *Interceptor) Resolve(ctx context.Context, token string) (Principal, error) {
	invalid := connect.NewError(connect.CodeUnauthenticated, errors.New("invalid or revoked token"))
	sess, err := i.store.AuthSessions().GetByTokenHash(ctx, HashToken(token))
	if errors.Is(err, core.ErrNotFound) {
		return i.resolveKey(ctx, token, invalid)
	} else if err != nil {
		return Principal{}, fmt.Errorf("look up session: %w", err)
	}
	user, err := i.store.Users().Get(ctx, sess.UserID)
	if errors.Is(err, core.ErrNotFound) {
		return Principal{}, invalid
	} else if err != nil {
		return Principal{}, fmt.Errorf("look up user: %w", err)
	}
	if user.Disabled {
		return Principal{}, connect.NewError(connect.CodeUnauthenticated, errors.New("user is disabled"))
	}
	if now := i.now(); now.Sub(sess.LastSeenAt) >= touchInterval {
		// A failed touch only loses activity time; the request goes on.
		if err := i.store.AuthSessions().Touch(ctx, sess.ID, now); err != nil {
			slog.WarnContext(ctx, "record session activity", "session", sess.ID, "err", err)
		} else {
			sess.LastSeenAt = now
		}
	}
	return Principal{User: user, Session: sess}, nil
}

// bearer returns the token of an "Authorization: Bearer" header.
func bearer(h http.Header) (string, bool) {
	scheme, token, ok := strings.Cut(h.Get("Authorization"), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	token = strings.TrimSpace(token)
	return token, token != ""
}

func (i *Interceptor) resolveKey(ctx context.Context, token string, invalid error) (Principal, error) {
	key, err := i.store.APIKeys().GetByTokenHash(ctx, HashToken(token))
	if errors.Is(err, core.ErrNotFound) {
		return Principal{}, invalid
	} else if err != nil {
		return Principal{}, fmt.Errorf("look up API key: %w", err)
	}
	user, err := i.store.Users().Get(ctx, key.UserID)
	if errors.Is(err, core.ErrNotFound) || err == nil && (user.Disabled || !user.Admin) {
		return Principal{}, invalid
	} else if err != nil {
		return Principal{}, fmt.Errorf("look up user: %w", err)
	}
	if now := i.now(); key.LastUsedAt == nil || now.Sub(*key.LastUsedAt) >= touchInterval {
		if err := i.store.APIKeys().Touch(ctx, key.ID, now); err != nil {
			slog.WarnContext(ctx, "record API key use", "key", key.ID, "err", err)
		}
	}
	return Principal{User: user, APIKey: key.ID}, nil
}
