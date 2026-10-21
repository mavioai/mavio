package rpc

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/apps/server/internal/auth"
	"github.com/mavioai/mavio/apps/server/internal/downloads"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library/storage"
	storagev1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/storage/v1"
	"github.com/mavioai/mavio/libs/rclone"
	"github.com/mavioai/mavio/libs/store"
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

// TestOfflineDownloads checks a remote's offline downloads: set only on a
// remote whose backend makes them, their folders defaulted, and removed
// with the remote.
func TestOfflineDownloads(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		switch r.URL.Path {
		case "/config/dump":
			_, _ = io.WriteString(w, `{"pp":{"type":"pikpak"},"disk":{"type":"local"}}`)
		case "/operations/fsinfo":
			_, _ = fmt.Fprintf(w, `{"Features":{"AddURL":%t}}`, strings.Contains(string(body), `"pp:"`))
		case "/config/delete":
			_, _ = io.WriteString(w, `{}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c, err := rclone.NewClient(rclone.Endpoint{URL: srv.URL}, nil)
	if err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(t.Context(), "sqlite:"+filepath.Join(t.TempDir(), "mavio.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	offline := &downloads.Offline{Remotes: &rclone.Remotes{Client: c}, Jobs: s.StorageJobs(), Configs: s.PluginConfigs()}
	svc := &StorageService{Client: c, Offline: offline}
	ctx := auth.WithPrincipal(t.Context(), auth.Principal{User: core.User{Kind: core.UserAdmin}})
	get := func(remote string) *storagev1.GetOfflineDownloadsResponse {
		t.Helper()
		got, err := svc.GetOfflineDownloads(ctx, storagev1.GetOfflineDownloadsRequest_builder{Remote: &remote}.Build())
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	if got := get("pp"); !got.GetSupported() || got.HasSettings() {
		t.Errorf("GetOfflineDownloads(pp) = %v, want supported, none set", got)
	}
	if got := get("disk"); got.GetSupported() {
		t.Errorf("GetOfflineDownloads(disk) = %v, want unsupported", got)
	}
	update := func(remote string, set *storagev1.OfflineDownloads) (*storagev1.UpdateOfflineDownloadsResponse, error) {
		return svc.UpdateOfflineDownloads(ctx, storagev1.UpdateOfflineDownloadsRequest_builder{Remote: &remote, Settings: set}.Build())
	}
	if _, err := update("disk", storagev1.OfflineDownloads_builder{Enabled: new(true)}.Build()); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("UpdateOfflineDownloads(disk) error = %v, want = failed precondition", err)
	}
	if _, err := update("pp", storagev1.OfflineDownloads_builder{MovieFolder: new("../x")}.Build()); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("UpdateOfflineDownloads(../x) error = %v, want = invalid argument", err)
	}
	got, err := update("pp", storagev1.OfflineDownloads_builder{Enabled: new(true), SeriesFolder: new("/TV/")}.Build())
	if err != nil || got.GetSettings().GetMovieFolder() != "movies" || got.GetSettings().GetSeriesFolder() != "TV" {
		t.Errorf("UpdateOfflineDownloads(pp) = %v, %v; want folders movies and TV", got, err)
	}
	if got := get("pp"); !got.GetSettings().GetEnabled() || got.GetSettings().GetSeriesFolder() != "TV" {
		t.Errorf("GetOfflineDownloads(pp) = %v, want the settings kept", got)
	}
	if _, err := svc.DeleteRemote(ctx, storagev1.DeleteRemoteRequest_builder{Name: new("pp")}.Build()); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := offline.Client(ctx, "pp"); ok || err != nil {
		t.Errorf("offline downloads of a deleted remote = %v, %v; want none", ok, err)
	}
}
