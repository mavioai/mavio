package httpserver_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/apps/server/internal/httpserver"
	"github.com/mavioai/mavio/apps/server/internal/images"
	"github.com/mavioai/mavio/apps/server/internal/playback"
	systemv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1/systemv1connect"
	"github.com/mavioai/mavio/libs/store"
)

// newServer serves the handler tree over a fresh SQLite database.
func newServer(t *testing.T) string {
	url, _ := startServer(t, nil)
	return url
}

// startServer serves the handler tree over a fresh SQLite database, with
// playback configured by configure, if given.
func startServer(t *testing.T, configure func(*playback.Config)) (string, *store.Store) {
	t.Helper()
	s, err := store.Open(t.Context(), "sqlite:"+filepath.Join(t.TempDir(), "mavio.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	cfg := playback.Config{Store: s, Dir: t.TempDir()}
	if configure != nil {
		configure(&cfg)
	}
	h, err := httpserver.Handler(httpserver.Options{
		Version: "v-test", Store: s, Database: s.Dialect(), Playbacks: playback.NewManager(cfg),
		Images: images.New(images.Config{Store: s, Dir: t.TempDir()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv.URL, s
}

// withToken sends token as the bearer token of every request.
func withToken(token string) connect.ClientOption {
	return connect.WithInterceptors(connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			req.Header().Set("Authorization", "Bearer "+token)
			return next(ctx, req)
		}
	}))
}

func TestGetHealth(t *testing.T) {
	client := systemv1connect.NewSystemServiceClient(http.DefaultClient, newServer(t))
	resp, err := client.GetHealth(t.Context(), &systemv1.GetHealthRequest{})
	if err != nil {
		t.Fatalf("GetHealth: %v", err)
	}
	if got, want := resp.GetStatus(), systemv1.GetHealthResponse_STATUS_SERVING; got != want {
		t.Errorf("status = %v, want %v", got, want)
	}
	if got, want := resp.GetVersion(), "v-test"; got != want {
		t.Errorf("version = %q, want %q", got, want)
	}
}

func TestGetSystemInfo(t *testing.T) {
	url := newServer(t)
	_, err := systemv1connect.NewSystemServiceClient(http.DefaultClient, url).GetSystemInfo(t.Context(), &systemv1.GetSystemInfoRequest{})
	if got, want := connect.CodeOf(err), connect.CodeUnauthenticated; got != want {
		t.Errorf("GetSystemInfo without a token: code = %v, want %v", got, want)
	}

	token := signUp(t, url)
	client := systemv1connect.NewSystemServiceClient(http.DefaultClient, url, withToken(token))
	info, err := client.GetSystemInfo(t.Context(), &systemv1.GetSystemInfoRequest{})
	if err != nil {
		t.Fatalf("GetSystemInfo: %v", err)
	}
	if info.GetVersion() != "v-test" || info.GetOs() != runtime.GOOS || info.GetArch() != runtime.GOARCH || info.GetDatabase() != "sqlite" {
		t.Errorf("info = %v", info)
	}
	if !info.HasStartTime() || info.GetStartTime().AsTime().After(time.Now()) {
		t.Errorf("start time = %v", info.GetStartTime())
	}
}
