package downloads_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/apps/server/internal/downloads"
	"github.com/mavioai/mavio/libs/core"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/rclone"
	"github.com/mavioai/mavio/libs/store"
)

// fakeDrive is an rcd with one remote whose backend makes offline
// downloads.
type fakeDrive struct {
	mu     sync.Mutex
	tasks  []rclone.OfflineTask
	added  []map[string]any
	mkdirs []string
	purged []string
	// since is what each listoffline asked from.
	since []any
}

func (d *fakeDrive) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var in map[string]any
	_ = json.NewDecoder(r.Body).Decode(&in)
	d.mu.Lock()
	defer d.mu.Unlock()
	var out any = map[string]any{}
	switch r.URL.Path {
	case "/operations/fsinfo":
		out = map[string]any{"Features": map[string]bool{"AddURL": true}}
	case "/operations/stat":
		out = map[string]any{"item": map[string]any{"Name": in["remote"], "IsDir": true}}
	case "/operations/mkdir":
		d.mkdirs = append(d.mkdirs, in["fs"].(string)+in["remote"].(string))
	case "/operations/purge":
		d.purged = append(d.purged, in["fs"].(string)+"|"+in["remote"].(string))
	case "/operations/addurl":
		d.added = append(d.added, in)
		t := rclone.OfflineTask{ID: "t1", Name: "Show.S01.1080p", Status: rclone.OfflineInit}
		d.tasks = append(d.tasks, t)
		out = map[string]any{"task": t}
	case "/operations/listoffline":
		d.since = append(d.since, in["since"])
		out = map[string]any{"tasks": d.tasks}
	case "/operations/deleteoffline":
		d.tasks = nil
	default:
		w.WriteHeader(http.StatusNotFound)
		out = map[string]any{"error": "couldn't find method", "status": 404}
	}
	_ = json.NewEncoder(w).Encode(out)
}

func (d *fakeDrive) set(t rclone.OfflineTask) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.tasks = []rclone.OfflineTask{t}
}

func TestOffline(t *testing.T) {
	ctx := t.Context()
	drive := &fakeDrive{}
	srv := httptest.NewServer(drive)
	defer srv.Close()
	c, err := rclone.NewClient(rclone.Endpoint{URL: srv.URL}, nil)
	if err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(ctx, "sqlite:"+filepath.Join(t.TempDir(), "mavio.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	o := &downloads.Offline{Remotes: &rclone.Remotes{Client: c}, Jobs: s.StorageJobs(), Configs: s.PluginConfigs()}

	for _, bad := range []string{"/local", "../up", "a/../b", "."} {
		if err := o.SetClient(ctx, downloads.OfflineClient{Remote: "pp", Folder: bad}); !errors.Is(err, core.ErrInvalid) {
			t.Errorf("SetClient(folder %q) error = %v, want = %v", bad, err, core.ErrInvalid)
		}
	}
	if err := o.SetClient(ctx, downloads.OfflineClient{Remote: "pp", Enabled: true, Folder: "Downloads/mavio"}); err != nil {
		t.Fatal(err)
	}
	clients, err := o.ListDownloadClients(ctx, &pluginv1.ListDownloadClientsRequest{})
	if err != nil || len(clients.GetClients()) != 1 || clients.GetClients()[0].GetPriority() != 1 ||
		clients.GetClients()[0].GetProtocol() != pluginv1.DownloadProtocol_DOWNLOAD_PROTOCOL_TORRENT {
		t.Fatalf("ListDownloadClients = %v, %v", clients, err)
	}
	test, err := o.TestDownloadClient(ctx, pluginv1.TestDownloadClientRequest_builder{ClientId: new("pp")}.Build())
	if err != nil || test.GetFailure() != "" {
		t.Errorf("TestDownloadClient = %v, %v", test, err)
	}

	// Without downloads the drive isn't asked, but the folder is told.
	list, err := o.ListDownloads(ctx, pluginv1.ListDownloadsRequest_builder{IncludeStatuses: new(true)}.Build())
	if err != nil || len(list.GetDownloads()) != 0 || len(list.GetStatuses()) != 1 || len(drive.since) != 0 {
		t.Errorf("ListDownloads = %v, %v after listoffline %v; want the status alone, without asking the drive", list, err, drive.since)
	}

	const magnet = "magnet:?xt=urn:btih:0123456789abcdef0123456789abcdef01234567&dn=Show"
	const hash = "0123456789ABCDEF0123456789ABCDEF01234567"
	add := func(link string, blocked ...string) (*pluginv1.AddDownloadResponse, error) {
		return o.AddDownload(ctx, pluginv1.AddDownloadRequest_builder{
			ClientId: new("pp"), Title: new("Show S01"), Kind: new(pluginv1.DownloadKind_DOWNLOAD_KIND_SERIES),
			MagnetUrl: new(link), BlocklistedInfoHashes: blocked,
		}.Build())
	}
	if _, err := add(""); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("AddDownload(torrent file) error = %v, want FailedPrecondition", err)
	}
	if _, err := add(magnet, hash); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("AddDownload(blocklisted) error = %v, want FailedPrecondition", err)
	}
	resp, err := add(magnet)
	if err != nil || resp.GetDownloadId() != hash {
		t.Fatalf("AddDownload = %v, %v; want ID %s", resp, err, hash)
	}
	if len(drive.added) != 1 || drive.added[0]["fs"] != "pp:Downloads/mavio" || drive.added[0]["url"] != magnet || len(drive.mkdirs) != 1 {
		t.Errorf("rcd got addurl %v after mkdir %v", drive.added, drive.mkdirs)
	}
	if _, err := add(magnet); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("AddDownload(again) error = %v, want FailedPrecondition", err)
	}

	get := func() *pluginv1.Download {
		t.Helper()
		resp, err := o.GetDownload(ctx, pluginv1.GetDownloadRequest_builder{ClientId: new("pp"), DownloadId: new(hash)}.Build())
		if err != nil {
			t.Fatal(err)
		}
		return resp.GetDownload()
	}
	if d := get(); d.GetStatus() != pluginv1.DownloadStatus_DOWNLOAD_STATUS_QUEUED || d.GetTitle() != "Show S01" ||
		d.GetKind() != pluginv1.DownloadKind_DOWNLOAD_KIND_SERIES || d.GetCategory() != "series" {
		t.Errorf("download = %v, want the queued series", d)
	}
	drive.set(rclone.OfflineTask{ID: "t1", Name: "Show.S01.1080p", Size: 1000, Progress: 50, Speed: 100, Status: rclone.OfflineRunning})
	if d := get(); d.GetStatus() != pluginv1.DownloadStatus_DOWNLOAD_STATUS_DOWNLOADING || d.GetRemainingSize() != 500 ||
		d.GetRemainingTime().AsDuration() != 5*time.Second {
		t.Errorf("download = %v, want 500 bytes left, 5 s", d)
	}
	drive.set(rclone.OfflineTask{ID: "t1", Name: "Show.S01.1080p", Size: 1000, Progress: 100, Status: rclone.OfflineComplete})
	const out = "remote:pp:Downloads/mavio/Show.S01.1080p"
	if d := get(); d.GetStatus() != pluginv1.DownloadStatus_DOWNLOAD_STATUS_COMPLETED || d.GetOutputPath() != out || !d.GetCanMoveFiles() {
		t.Errorf("download = %v, want completed at %s", d, out)
	}
	// A drive that no longer lists the completed download leaves it
	// completed.
	drive.set(rclone.OfflineTask{ID: "other"})
	list, err = o.ListDownloads(ctx, pluginv1.ListDownloadsRequest_builder{IncludeStatuses: new(true)}.Build())
	if err != nil || len(list.GetDownloads()) != 1 || list.GetDownloads()[0].GetStatus() != pluginv1.DownloadStatus_DOWNLOAD_STATUS_COMPLETED ||
		len(list.GetStatuses()) != 1 {
		t.Errorf("ListDownloads = %v, %v; want the completed download, not the drive's other", list, err)
	}
	if since, _ := drive.since[len(drive.since)-1].(string); since == "" {
		t.Errorf("listoffline since = %v, want the time of the oldest download", drive.since)
	}

	if _, err := o.RemoveDownload(ctx, pluginv1.RemoveDownloadRequest_builder{ClientId: new("pp"), DownloadId: new(hash), DeleteData: new(true)}.Build()); err != nil {
		t.Fatal(err)
	}
	if len(drive.purged) != 1 || drive.purged[0] != "pp:|Downloads/mavio/Show.S01.1080p" {
		t.Errorf("purged = %v, want the download's folder", drive.purged)
	}
	if _, err := o.GetDownload(ctx, pluginv1.GetDownloadRequest_builder{ClientId: new("pp"), DownloadId: new(hash)}.Build()); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("GetDownload after RemoveDownload error = %v, want NotFound", err)
	}
}

// TestOfflineRemoveClient checks that removing a remote's offline
// downloads, as when the remote is removed, forgets the downloads sent to
// it and leaves another remote's.
func TestOfflineRemoveClient(t *testing.T) {
	ctx := t.Context()
	s, err := store.Open(ctx, "sqlite:"+filepath.Join(t.TempDir(), "mavio.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	o := &downloads.Offline{Jobs: s.StorageJobs(), Configs: s.PluginConfigs()}
	for _, remote := range []string{"pp", "pp2"} {
		if err := o.SetClient(ctx, downloads.OfflineClient{Remote: remote, Enabled: true}); err != nil {
			t.Fatal(err)
		}
		if err := s.StorageJobs().Put(ctx, &core.StorageJob{Kind: core.StorageJobOffline, Key: remote + "/H", Remote: remote}); err != nil {
			t.Fatal(err)
		}
	}
	if err := o.RemoveClient(ctx, "pp"); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := o.Client(ctx, "pp"); ok || err != nil {
		t.Errorf("Client(pp) = %v, %v; want none", ok, err)
	}
	if c, ok, err := o.Client(ctx, "pp2"); !ok || err != nil || !c.Enabled {
		t.Errorf("Client(pp2) = %v, %v, %v; want it kept", c, ok, err)
	}
	jobs, err := s.StorageJobs().List(ctx, core.StorageJobOffline)
	if err != nil || len(jobs) != 1 || jobs[0].Key != "pp2/H" {
		t.Errorf("jobs = %v, %v; want pp2's alone", jobs, err)
	}
}
