package httpserver

import (
	"net/http"
	"strings"
)

// corsExposed are the response headers that scripts of other origins may
// read: the gRPC-Web status of Connect, and what media and downloads need.
var corsExposed = strings.Join([]string{
	"Grpc-Status", "Grpc-Message", "Grpc-Status-Details-Bin",
	"Content-Range", "Accept-Ranges", "Content-Length", "Content-Disposition",
}, ", ")

// cors lets web UIs served by other servers call every route. Credentials
// are bearer tokens and unguessable IDs, never cookies, so any origin is
// allowed and nothing it sends is ambient.
func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Origin") == "" {
			next.ServeHTTP(w, r)
			return
		}
		h := w.Header()
		h.Set("Access-Control-Allow-Origin", "*")
		if r.Method != http.MethodOptions || r.Header.Get("Access-Control-Request-Method") == "" {
			h.Set("Access-Control-Expose-Headers", corsExposed)
			next.ServeHTTP(w, r)
			return
		}
		h.Add("Vary", "Access-Control-Request-Headers")
		h.Set("Access-Control-Allow-Methods", "GET, HEAD, POST, PUT, DELETE")
		if req := r.Header.Get("Access-Control-Request-Headers"); req != "" {
			h.Set("Access-Control-Allow-Headers", req)
		}
		if r.Header.Get("Access-Control-Request-Private-Network") == "true" {
			h.Set("Access-Control-Allow-Private-Network", "true")
		}
		h.Set("Access-Control-Max-Age", "7200")
		w.WriteHeader(http.StatusNoContent)
	})
}
