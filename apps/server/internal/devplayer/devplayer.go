// Package devplayer serves a minimal web player for trying playback on
// real browsers during development: it signs in, lists videos and plays
// them through the playback API with hls.js, or natively in Safari.
package devplayer

import (
	"embed"
	"net/http"
)

//go:embed index.html
var files embed.FS

// Handler serves the player at /dev/player.
func Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /dev/player", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		http.ServeFileFS(w, r, files, "index.html")
	})
	return mux
}
