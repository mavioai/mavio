package rpc

import (
	"context"

	"github.com/mavioai/mavio/apps/server/internal/auth"
)

// playbackURL signs u, a path under media/{playback}/, for the caller's
// session until the playback ends; callers without a session get it as it
// is.
func playbackURL(ctx context.Context, u string) string {
	if u == "" {
		return ""
	}
	if signed := auth.MediaURL(ctx, u, 2, false); signed != "" {
		return signed
	}
	return u
}

// imageURL signs p, an image's path whose first scope segments name it,
// for the caller's session; it is empty for callers without one.
func imageURL(ctx context.Context, p string, scope int) string {
	return auth.MediaURL(ctx, p, scope, true)
}
