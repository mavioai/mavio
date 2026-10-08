package process

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mavioai/mavio/libs/plugin/internal/proc"
)

func TestAuthenticate(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	h := authenticate("secret", ok)
	for header, want := range map[string]int{
		"":                         http.StatusUnauthorized,
		"Bearer wrong":             http.StatusUnauthorized,
		"secret":                   http.StatusUnauthorized,
		proc.AuthScheme + "secret": http.StatusNoContent,
	} {
		req := httptest.NewRequest(http.MethodPost, "/x", nil)
		if header != "" {
			req.Header.Set("Authorization", header)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Errorf("Authorization %q: status %d, want %d", header, rec.Code, want)
		}
	}
}

func TestShutdownAfter(t *testing.T) {
	done := make(chan struct{}, 1)
	h := shutdownAfter(http.NotFoundHandler(), done)
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/mavio.plugin.v1.PluginService/Health", nil))
	select {
	case <-done:
		t.Fatal("shutdown signalled after Health")
	default:
	}
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, proc.ShutdownPath, nil))
	select {
	case <-done:
	default:
		t.Fatal("no shutdown signal after Shutdown")
	}
}
