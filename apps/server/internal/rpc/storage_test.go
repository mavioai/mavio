package rpc

import (
	"testing"

	"github.com/mavioai/mavio/apps/server/internal/auth"
	"github.com/mavioai/mavio/libs/core"
	storagev1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/storage/v1"
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
