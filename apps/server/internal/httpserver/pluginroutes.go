package httpserver

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/mavioai/mavio/apps/server/internal/auth"
)

// Headers naming the user of a request to a plugin's HTTP routes.
const (
	pluginUserID    = "Mavio-User-Id"
	pluginUserName  = "Mavio-User-Name"
	pluginUserAdmin = "Mavio-User-Admin"
)

// pluginCSP makes the pages plugins serve run in an opaque origin, apart
// from the server's storage.
const pluginCSP = "sandbox allow-scripts allow-forms allow-popups"

// pluginRoutes serves the HTTP routes of plugins under /plugins/{id}/. The
// routes authenticate requests themselves: a valid access token is replaced
// by headers naming its user, and those headers are dropped otherwise.
func pluginRoutes(authn *auth.Interceptor, routes func(id string) http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var h http.Handler
		if routes != nil {
			h = routes(id)
		}
		if h == nil {
			http.NotFound(w, r)
			return
		}
		r = r.Clone(r.Context())
		for _, name := range []string{pluginUserID, pluginUserName, pluginUserAdmin} {
			r.Header.Del(name)
		}
		scheme, token, ok := strings.Cut(r.Header.Get("Authorization"), " ")
		if ok && strings.EqualFold(scheme, "Bearer") {
			if p, err := authn.Resolve(r.Context(), strings.TrimSpace(token)); err == nil {
				r.Header.Del("Authorization")
				r.Header.Set(pluginUserID, p.User.ID.String())
				r.Header.Set(pluginUserName, p.User.Name)
				r.Header.Set(pluginUserAdmin, strconv.FormatBool(p.User.Admin))
			}
		}
		w.Header().Set("Content-Security-Policy", pluginCSP)
		http.StripPrefix("/plugins/"+id, h).ServeHTTP(w, r)
	})
}
