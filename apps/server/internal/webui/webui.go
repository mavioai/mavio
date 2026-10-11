package webui

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:dist
var dist embed.FS

// missing is the page of a server built without the web UI.
const missing = `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><title>Mavio</title></head>
<body><p>This server was built without its web UI. The API reference is at <a href="/api">/api</a>.</p></body></html>
`

// Handler serves the embedded web UI to GET and HEAD requests.
func Handler() http.Handler {
	files, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err) // dist is embedded, so it always exists
	}
	return handler(files)
}

func handler(files fs.FS) http.Handler {
	mux := http.NewServeMux()
	index, err := fs.ReadFile(files, "index.html")
	if err != nil {
		mux.HandleFunc("GET /", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(missing))
		})
		return mux
	}
	fileServer := http.FileServerFS(files)
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if info, err := fs.Stat(files, name); err == nil && !info.IsDir() && name != "index.html" {
			// Vite names the built assets by their content.
			if strings.HasPrefix(name, "assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			fileServer.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(index)
	})
	return mux
}
