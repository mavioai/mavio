package indexer

import (
	"net/http"
	"strings"
)

// IsCloudflareProtected reports whether Cloudflare or DDoS-Guard answered
// instead of the indexer, as Prowlarr's CloudFlareDetectionService does.
func IsCloudflareProtected(r *Response) bool {
	switch strings.ToLower(r.Header.Get("Server")) {
	case "cloudflare", "cloudflare-nginx", "ddos-guard":
	default:
		return false
	}
	if r.StatusCode == http.StatusServiceUnavailable || r.StatusCode == http.StatusForbidden {
		html := r.Content()
		for _, title := range []string{"<title>Just a moment...</title>", "<title>Access denied</title>", "<title>Attention Required! | Cloudflare</title>"} {
			if strings.Contains(html, title) {
				return true
			}
		}
		if strings.TrimSpace(html) == "error code: 1020" || strings.Contains(strings.ToLower(html), "<title>ddos-guard</title>") {
			return true
		}
	}
	return r.Header.Get("Vary") == "Accept-Encoding,User-Agent" &&
		r.Header.Get("Content-Encoding") == "" &&
		strings.Contains(strings.ToLower(r.Content()), "ddos")
}
