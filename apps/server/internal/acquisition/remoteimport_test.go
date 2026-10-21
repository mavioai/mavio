package acquisition

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/rclone"
)

// TestImportFromRemote checks a download on an rclone remote is imported
// into a library on the same remote: a cloud drive's, whose files may
// move, by the rcd moving the file and deleting its emptied folder, and a
// seeding one by the rcd hard linking the file.
func TestImportFromRemote(t *testing.T) {
	bin, err := exec.LookPath("rclone")
	if err != nil {
		t.Skip("rclone is not installed")
	}
	for _, move := range []bool{true, false} {
		t.Run(map[bool]string{true: "move", false: "link"}[move], func(t *testing.T) {
			testImportFromRemote(t, bin, move)
		})
	}
}

func testImportFromRemote(t *testing.T, bin string, move bool) {
	e := newEnv(t)
	dir := t.TempDir()
	disk := filepath.Join(dir, "drive")
	download := filepath.Join(disk, "Downloads", "movies", title1080)
	if err := os.MkdirAll(download, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(disk, "Movies"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(download, title1080+".mkv"), []byte("movie"), 0o644); err != nil {
		t.Fatal(err)
	}
	d, err := rclone.StartDaemon(e.ctx, rclone.DaemonOptions{Path: bin, Config: filepath.Join(dir, "rclone.conf")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	if _, err := d.Client().CreateRemote(e.ctx, "drive", "alias", map[string]string{"remote": disk}); err != nil {
		t.Fatal(err)
	}
	proxy := rclone.NewProxy(d.Client(), nil)
	if err := proxy.Listen(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = proxy.Close() })
	remotes := &rclone.Remotes{Client: d.Client(), MediaURL: proxy.URL, Jobs: e.db.StorageJobs()}
	e.rt.Storages = &library.Storages{Remote: remotes.Open}
	e.lib.Paths = []string{"remote:drive:Movies"}
	if err := e.db.Libraries().Update(e.ctx, &e.lib); err != nil {
		t.Fatal(err)
	}

	it := e.addMovie()
	e.grabAndComplete(it)
	e.client.setDownloads(pluginv1.Download_builder{
		ClientId: new("qb"), Id: new(hash1080), Title: new(title1080), Status: new(pluginv1.DownloadStatus_DOWNLOAD_STATUS_COMPLETED),
		OutputPath: new("remote:drive:Downloads/movies/" + title1080), CanBeRemoved: new(move), CanMoveFiles: new(move),
	}.Build())
	if err := e.rt.CheckQueue(e.ctx); err != nil {
		t.Fatal(err)
	}
	if got := e.download(hash1080); got.State != core.DownloadImported {
		t.Fatalf("download = %s %v, want = imported", got.State, got.Messages)
	}
	imported, err := filepath.Glob(filepath.Join(disk, "Movies", "The Matrix (1999)", "*.mkv"))
	if err != nil || len(imported) != 1 {
		t.Fatalf("movie folder holds %v, %v, want the imported file", imported, err)
	}
	if data, err := os.ReadFile(imported[0]); err != nil || string(data) != "movie" {
		t.Errorf("imported file = %q, %v", data, err)
	}
	if move {
		if _, err := os.Stat(download); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("download folder: %v, want = deleted", err)
		}
	} else {
		a, _ := os.Stat(imported[0])
		b, err := os.Stat(filepath.Join(download, title1080+".mkv"))
		if err != nil || !os.SameFile(a, b) {
			t.Errorf("downloaded file = %v, %v, want the imported file linked to it", b, err)
		}
	}
	if jobs, err := e.db.StorageJobs().List(e.ctx, core.StorageJobTransfer); err != nil || len(jobs) != 0 {
		t.Errorf("transfer ledger = %v, %v, want = empty", jobs, err)
	}
}
