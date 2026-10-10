package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/libs/core"
)

// UserHeader names the user a plugin's host API request acts as.
const UserHeader = "Mavio-User"

// Plugin is what a running plugin may do through the host API.
type Plugin struct {
	ID string
	// Allows reports whether the plugin's scopes cover a Connect procedure;
	// readOnly tells whether the procedure has no side effects.
	Allows func(procedure string, readOnly bool) bool
	// ActAsUsers lets requests act as the user named in UserHeader.
	ActAsUsers bool
}

type pluginKey struct{}

// PluginHandler serves next as the host API of plugin p: requests act as
// the plugin, whatever credentials they carry.
func PluginHandler(p Plugin, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(context.WithValue(r.Context(), pluginKey{}, p))
		r.Header.Del("Authorization")
		r.Header.Del("Cookie")
		next.ServeHTTP(w, r)
	})
}

// RequireUser names the procedures that act on the caller's own state and
// so need a user; entries are procedures or whole services
// ("mavio.user.v1.UserDataService"). A plugin calling them without acting
// as a user fails with failed_precondition.
func (i *Interceptor) RequireUser(procedures ...string) *Interceptor {
	for _, p := range procedures {
		i.userBound[strings.Trim(p, "/")] = true
	}
	return i
}

// RequireSession names the procedures that need a signed-in session, in the
// form of RequireUser. Plugins have none.
func (i *Interceptor) RequireSession(procedures ...string) *Interceptor {
	for _, p := range procedures {
		i.sessionBound[strings.Trim(p, "/")] = true
	}
	return i
}

func bound(set map[string]bool, procedure string) bool {
	procedure = strings.TrimPrefix(procedure, "/")
	service, _, _ := strings.Cut(procedure, "/")
	return set[procedure] || set[service]
}

// authenticatePlugin authorizes a host API request of plugin g.
func (i *Interceptor) authenticatePlugin(ctx context.Context, g Plugin, procedure string, readOnly bool, h http.Header) (Principal, error) {
	if !g.Allows(procedure, readOnly) {
		return Principal{}, connect.NewError(connect.CodePermissionDenied,
			fmt.Errorf("plugin %s may not call %s; add its service to permissions.api", g.ID, procedure))
	}
	if bound(i.sessionBound, procedure) {
		return Principal{}, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("%s needs a signed-in session", procedure))
	}
	name := h.Get(UserHeader)
	if name == "" {
		if bound(i.userBound, procedure) {
			return Principal{}, connect.NewError(connect.CodeFailedPrecondition,
				fmt.Errorf("%s acts on a user's own state; name the user in %s", procedure, UserHeader))
		}
		return Principal{User: core.User{Name: "plugin " + g.ID, Admin: true}, Plugin: g.ID}, nil
	}
	if !g.ActAsUsers {
		return Principal{}, connect.NewError(connect.CodePermissionDenied,
			fmt.Errorf("plugin %s may not act as users; set permissions.act_as_users", g.ID))
	}
	user, err := i.store.Users().GetByName(ctx, name)
	if errors.Is(err, core.ErrNotFound) || err == nil && user.Disabled {
		return Principal{}, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("no enabled user %q", name))
	} else if err != nil {
		return Principal{}, fmt.Errorf("look up user: %w", err)
	}
	return Principal{User: user, Plugin: g.ID}, nil
}

// ResolveRequest authenticates a plain HTTP request by its bearer token or,
// on a plugin's host API, by the plugin's scopes for procedure, which is
// treated as read-only.
func (i *Interceptor) ResolveRequest(r *http.Request, procedure string) (Principal, error) {
	if g, ok := r.Context().Value(pluginKey{}).(Plugin); ok {
		return i.authenticatePlugin(r.Context(), g, procedure, true, r.Header)
	}
	token, ok := bearer(r.Header)
	if !ok {
		return Principal{}, connect.NewError(connect.CodeUnauthenticated, errors.New("missing bearer token"))
	}
	return i.Resolve(r.Context(), token)
}
