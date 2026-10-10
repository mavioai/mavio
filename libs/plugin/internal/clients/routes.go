package clients

import (
	"errors"
	"log/slog"
	"net/http"
	"net/http/httputil"

	"github.com/mavioai/mavio/libs/plugin/internal/abi"
)

// Routes returns the handler forwarding requests to a plugin's HTTP routes
// through rt, which reaches the plugin at "http://plugin". Request paths
// are relative to the plugin's route prefix. limit bounds request bodies
// when positive.
func Routes(rt http.RoundTripper, limit int64, log *slog.Logger) http.Handler {
	proxy := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			u := pr.Out.URL
			u.Scheme, u.Host = "http", "plugin"
			u.Path = abi.HTTPPrefix + pr.In.URL.Path
			u.RawPath = ""
			if pr.In.URL.RawPath != "" {
				u.RawPath = abi.HTTPPrefix + pr.In.URL.RawPath
			}
			pr.Out.Host = "plugin"
			pr.SetXForwarded()
		},
		Transport: rt,
		// Responses stream as the plugin writes them.
		FlushInterval: -1,
		ErrorLog:      slog.NewLogLogger(log.Handler(), slog.LevelWarn),
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
				http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
				return
			}
			log.WarnContext(r.Context(), "plugin route failed", "path", r.URL.Path, "err", err)
			http.Error(w, "plugin unavailable", http.StatusBadGateway)
		},
	}
	if limit <= 0 {
		return proxy
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, limit)
		proxy.ServeHTTP(w, r)
	})
}
