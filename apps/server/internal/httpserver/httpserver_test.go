package httpserver_test

import (
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
	"time"

	"github.com/mavioai/mavio/apps/server/internal/httpserver"
	systemv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1/systemv1connect"
)

func TestGetHealth(t *testing.T) {
	srv := httptest.NewServer(httpserver.Handler("v-test"))
	t.Cleanup(srv.Close)

	client := systemv1connect.NewSystemServiceClient(http.DefaultClient, srv.URL)
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
	srv := httptest.NewServer(httpserver.Handler("v-test"))
	t.Cleanup(srv.Close)

	client := systemv1connect.NewSystemServiceClient(http.DefaultClient, srv.URL)
	info, err := client.GetSystemInfo(t.Context(), &systemv1.GetSystemInfoRequest{})
	if err != nil {
		t.Fatalf("GetSystemInfo: %v", err)
	}
	if info.GetVersion() != "v-test" || info.GetOs() != runtime.GOOS || info.GetArch() != runtime.GOARCH {
		t.Errorf("info = %v", info)
	}
	if !info.HasStartTime() || info.GetStartTime().AsTime().After(time.Now()) {
		t.Errorf("start time = %v", info.GetStartTime())
	}
}
