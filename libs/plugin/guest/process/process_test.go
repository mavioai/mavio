package process

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

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

func TestServeLifeline(t *testing.T) {
	dir, err := os.MkdirTemp("", "mavio-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	socket := filepath.Join(dir, "p.sock")
	t.Setenv(proc.EnvSocket, socket)
	t.Setenv(proc.EnvToken, "token")
	t.Setenv(proc.EnvLifeline, proc.LifelineStdin)
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stdin := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = stdin; _ = r.Close() })

	errc := make(chan error, 1)
	go func() { errc <- Serve(t.Context(), "dev.mavio.test") }()
	for {
		if _, err := os.Stat(socket); err == nil {
			break
		}
		select {
		case err := <-errc:
			t.Fatalf("Serve = %v before the lifeline ended", err)
		case <-time.After(10 * time.Millisecond):
		}
	}
	_ = w.Close()
	select {
	case err := <-errc:
		if err != nil {
			t.Errorf("Serve = %v, want nil", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Serve still runs after its lifeline ended")
	}
}
