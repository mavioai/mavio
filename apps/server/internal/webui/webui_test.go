package webui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestHandler(t *testing.T) {
	files := fstest.MapFS{
		"index.html":      {Data: []byte("<html>ui</html>")},
		"assets/app-1.js": {Data: []byte("js")},
		"favicon.svg":     {Data: []byte("<svg/>")},
	}
	tests := []struct {
		path, body, cache string
	}{
		{"/", "<html>ui</html>", "no-cache"},
		{"/index.html", "<html>ui</html>", "no-cache"},
		{"/libraries/42", "<html>ui</html>", "no-cache"},
		{"/assets", "<html>ui</html>", "no-cache"},
		{"/assets/app-1.js", "js", "public, max-age=31536000, immutable"},
		{"/favicon.svg", "<svg/>", ""},
	}
	h := handler(files)
	for _, tt := range tests {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.path, nil))
		if got := rec.Body.String(); rec.Code != http.StatusOK || got != tt.body {
			t.Errorf("%s: got = %d %q, want = 200 %q", tt.path, rec.Code, got, tt.body)
		}
		if got := rec.Header().Get("Cache-Control"); got != tt.cache {
			t.Errorf("%s: got = Cache-Control %q, want = %q", tt.path, got, tt.cache)
		}
	}
}

func TestHandlerMethods(t *testing.T) {
	rec := httptest.NewRecorder()
	handler(fstest.MapFS{"index.html": {}}).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST: got = %d, want = %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandlerWithoutUI(t *testing.T) {
	rec := httptest.NewRecorder()
	handler(fstest.MapFS{".gitkeep": {}}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if got := rec.Body.String(); !strings.Contains(got, "without its web UI") {
		t.Errorf("got = %q, want the page saying the UI is missing", got)
	}
}
