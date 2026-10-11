package acquisition

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

// grabAndComplete grabs the 1080p release of the movie and lists its
// download as completed with files, returning the download's folder.
func (e *env) grabAndComplete(it core.Item, files ...string) string {
	e.t.Helper()
	e.indexer.set(e.both(), nil)
	if err := e.rt.SearchMovie(e.ctx, it.ID, false); err != nil {
		e.t.Fatal(err)
	}
	dir := e.completed(hash1080, title1080, files...)
	e.clock = e.clock.Add(time.Hour)
	return dir
}

// TestImportChecksFreeSpaceOfMovieFolder checks imports check the free
// space of the volume of the movie's folder, as Radarr's
// FreeSpaceSpecification.
func TestImportChecksFreeSpaceOfMovieFolder(t *testing.T) {
	e := newEnv(t)
	var asked []string
	free := int64(0)
	e.rt.FreeSpace = func(p string) (int64, bool) {
		asked = append(asked, p)
		return free, true
	}
	it := e.addMovie()
	e.grabAndComplete(it, title1080+".mkv")
	if err := e.rt.CheckQueue(e.ctx); err != nil {
		t.Fatal(err)
	}
	d := e.download(hash1080)
	if d.State != core.DownloadImportBlocked || !slices.ContainsFunc(d.Messages, func(m string) bool { return strings.Contains(m, msgNoFreeSpace) }) {
		t.Errorf("download = %s %v, want = blocked for free space", d.State, d.Messages)
	}
	want := filepath.Join(e.libDir, "The Matrix (1999)")
	if !slices.Equal(asked, []string{want}) {
		t.Errorf("free space asked of %v, want = [%s]", asked, want)
	}

	free = 100 * gb
	if err := e.rt.CheckQueue(e.ctx); err != nil {
		t.Fatal(err)
	}
	if d := e.download(hash1080); d.State != core.DownloadImported {
		t.Errorf("download = %s %v, want = imported with free space", d.State, d.Messages)
	}
}

// TestImportDeletesEmptiedDownloadFolder checks a download folder an
// import moved its movie out of is deleted when only samples are left, as
// Radarr's DownloadedMovieImportService.ShouldDeleteFolder.
func TestImportDeletesEmptiedDownloadFolder(t *testing.T) {
	for _, tc := range []struct {
		name    string
		files   []string
		move    bool
		deleted bool
	}{
		{"only a sample left", []string{title1080 + ".mkv", "The.Matrix.1999.1080p.BluRay.x264-GROUP-sample.mkv", "readme.nfo"}, true, true},
		{"another video left", []string{title1080 + ".mkv", "The.Matrix.1999.Extra.mkv"}, true, false},
		{"a large RAR left", []string{title1080 + ".mkv", "big.rar"}, true, false},
		{"copied", []string{title1080 + ".mkv"}, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			it := e.addMovie()
			dir := e.grabAndComplete(it, tc.files...)
			if slices.Contains(tc.files, "big.rar") {
				if err := os.WriteFile(filepath.Join(dir, "big.rar"), make([]byte, rarLeftOver+1), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			e.client.setDownloads(pluginv1.Download_builder{
				ClientId: new("qb"), Id: new(hash1080), Title: new(title1080), Status: new(pluginv1.DownloadStatus_DOWNLOAD_STATUS_COMPLETED),
				OutputPath: new("/downloads/" + title1080), CanBeRemoved: new(true), CanMoveFiles: new(tc.move),
			}.Build())
			if err := e.rt.CheckQueue(e.ctx); err != nil {
				t.Fatal(err)
			}
			if d := e.download(hash1080); d.State != core.DownloadImported {
				t.Fatalf("download = %s %v, want = imported", d.State, d.Messages)
			}
			_, err := os.Stat(dir)
			if deleted := errors.Is(err, fs.ErrNotExist); deleted != tc.deleted {
				t.Errorf("download folder deleted = %v, want = %v", deleted, tc.deleted)
			}
			if _, err := os.Stat(e.dlDir); err != nil {
				t.Errorf("downloads folder: %v, want = kept", err)
			}
		})
	}
}
