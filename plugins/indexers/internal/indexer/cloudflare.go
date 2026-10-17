package indexer

import "github.com/mavioai/mavio/libs/plugin/challenge"

// IsCloudflareProtected reports whether Cloudflare or DDoS-Guard answered
// instead of the indexer, as Prowlarr's CloudFlareDetectionService does.
func IsCloudflareProtected(r *Response) bool {
	return challenge.Guarded(r.Header) && challenge.IsProtected(r.StatusCode, r.Header, []byte(r.Content()))
}
