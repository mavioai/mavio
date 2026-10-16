package rpc

import (
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/mavioai/mavio/apps/server/internal/auth"
	"github.com/mavioai/mavio/libs/core"
	storagev1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/storage/v1"
	"github.com/mavioai/mavio/libs/rclone"
)

func TestStorageStatusTellsWhyNoRcd(t *testing.T) {
	ctx := auth.WithPrincipal(t.Context(), auth.Principal{User: core.User{Kind: core.UserAdmin}})
	cases := []struct {
		name    string
		svc     StorageService
		managed bool
		want    string
	}{
		{"why it failed", StorageService{Managed: true, Down: "rclone not found"}, true, "rclone not found"},
		{"no reason", StorageService{}, false, "the server reaches no rclone"},
	}
	for _, c := range cases {
		got, err := c.svc.GetStatus(ctx, &storagev1.GetStatusRequest{})
		if err != nil {
			t.Fatalf("%s: GetStatus: %v", c.name, err)
		}
		if got.GetAvailable() || got.GetError() != c.want || got.GetManaged() != c.managed {
			t.Errorf("%s: got = %v %q %v, want = false %q %v", c.name, got.GetAvailable(), got.GetError(), got.GetManaged(), c.want, c.managed)
		}
	}
}

func TestListProvidersLeavesOutHiddenNames(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"providers":[
			{"Name":"aliyundrive","Description":"Aliyun Drive"},
			{"Name":"aliyun","Description":"Aliyun Drive","Hide":true},
			{"Name":"s3","Description":"Amazon S3"}]}`)
	}))
	defer srv.Close()
	c, err := rclone.NewClient(rclone.Endpoint{URL: srv.URL}, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx := auth.WithPrincipal(t.Context(), auth.Principal{User: core.User{Kind: core.UserAdmin}})
	res, err := (&StorageService{Client: c}).ListProviders(ctx, &storagev1.ListProvidersRequest{})
	if err != nil {
		t.Fatalf("ListProviders: %v", err)
	}
	var got []string
	for _, p := range res.GetProviders() {
		got = append(got, p.GetName())
	}
	if want := []string{"aliyundrive", "s3"}; !slices.Equal(got, want) {
		t.Errorf("got = %v, want = %v", got, want)
	}
}
