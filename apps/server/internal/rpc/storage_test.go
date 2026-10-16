package rpc

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/apps/server/internal/auth"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library/storage"
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

func TestListDisks(t *testing.T) {
	svc := &StorageService{Volumes: func(context.Context) ([]storage.Volume, error) {
		return []storage.Volume{
			{Path: "/mnt/nas", FSType: "nfs4", Kind: storage.KindUnknown},
			{Path: "/", FSType: "ext4", Kind: storage.KindLocalSSD, Total: 1000, Used: 400, Available: 600},
			{Path: "/run/media/me/USB", Name: "USB", FSType: "exfat", Kind: storage.KindLocalHDD, Total: 64, Used: 0, Available: 64},
		}, nil
	}}
	user := auth.WithPrincipal(t.Context(), auth.Principal{User: core.User{Kind: core.UserMember}})
	if _, err := svc.ListDisks(user, &storagev1.ListDisksRequest{}); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("ListDisks() by a member error = %v, want = permission denied", err)
	}
	ctx := auth.WithPrincipal(t.Context(), auth.Principal{User: core.User{Kind: core.UserAdmin}})
	res, err := svc.ListDisks(ctx, &storagev1.ListDisksRequest{})
	if err != nil {
		t.Fatalf("ListDisks: %v", err)
	}
	var got []string
	for _, d := range res.GetDisks() {
		got = append(got, fmt.Sprintf("%s %q %s %v %v/%v/%v", d.GetPath(), d.GetName(), d.GetKind(), d.HasTotalBytes(), d.GetTotalBytes(), d.GetUsedBytes(), d.GetAvailableBytes()))
	}
	want := []string{
		`/ "" DISK_KIND_SSD true 1000/400/600`,
		`/mnt/nas "" DISK_KIND_UNSPECIFIED false 0/0/0`,
		`/run/media/me/USB "USB" DISK_KIND_HDD true 64/0/64`,
	}
	if !slices.Equal(got, want) {
		t.Errorf("got = %q, want = %q", got, want)
	}
}

func TestGetRemoteUsage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), `"s3:"`) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = io.WriteString(w, `{"error":"S3 bucket media doesn't support about","status":500}`)
			return
		}
		_, _ = io.WriteString(w, `{"total":5000,"used":1200}`)
	}))
	defer srv.Close()
	c, err := rclone.NewClient(rclone.Endpoint{URL: srv.URL}, nil)
	if err != nil {
		t.Fatal(err)
	}
	svc := &StorageService{Client: c}
	ctx := auth.WithPrincipal(t.Context(), auth.Principal{User: core.User{Kind: core.UserAdmin}})
	u, err := svc.GetRemoteUsage(ctx, storagev1.GetRemoteUsageRequest_builder{Remote: new("115")}.Build())
	if err != nil || u.GetTotalBytes() != 5000 || u.GetUsedBytes() != 1200 || u.HasFreeBytes() {
		t.Errorf("GetRemoteUsage(115) = %v, %v, want = total 5000, used 1200, no free", u, err)
	}
	if _, err := svc.GetRemoteUsage(ctx, storagev1.GetRemoteUsageRequest_builder{Remote: new("s3")}.Build()); connect.CodeOf(err) != connect.CodeUnimplemented {
		t.Errorf("GetRemoteUsage(s3) error = %v, want = unimplemented", err)
	}
	if _, err := svc.GetRemoteUsage(ctx, storagev1.GetRemoteUsageRequest_builder{Remote: new("a:b")}.Build()); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("GetRemoteUsage(a:b) error = %v, want = invalid argument", err)
	}
}
