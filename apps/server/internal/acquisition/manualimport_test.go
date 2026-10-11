package acquisition

import (
	"errors"
	"path/filepath"
	"slices"
	"testing"

	"github.com/mavioai/mavio/libs/acquire"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/release"
)

// TestManualImport checks a download an import specification blocks is
// listed with why and imported by hand anyway, as Radarr's
// ManualImportService.
func TestManualImport(t *testing.T) {
	e := newEnv(t)
	e.rt.FreeSpace = func(string) (int64, bool) { return 0, true }
	it := e.addMovie()
	dir := e.grabAndComplete(it, title1080+".mkv")
	if err := e.rt.CheckQueue(e.ctx); err != nil {
		t.Fatal(err)
	}
	d := e.download(hash1080)
	if d.State != core.DownloadImportBlocked {
		t.Fatalf("download = %s %v, want = blocked", d.State, d.Messages)
	}

	files, err := e.rt.ListManualImport(e.ctx, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	video := filepath.Join(dir, title1080+".mkv")
	if len(files) != 1 || files[0].Path != video || files[0].RelativePath != title1080+".mkv" ||
		files[0].Quality.Quality != release.Bluray1080p || !slices.Equal(files[0].Rejections, []string{msgNoFreeSpace}) {
		t.Fatalf("files = %+v, want = the video at Bluray-1080p rejected for free space", files)
	}

	if err := e.rt.ManualImport(e.ctx, d.ID, filepath.Join(dir, "other.mkv"), nil); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("ManualImport of another file: err = %v, want = not found", err)
	}
	q := acquire.QualityModel{Quality: release.Remux1080p, Revision: release.Revision{Version: 1}}
	if err := e.rt.ManualImport(e.ctx, d.ID, video, &q); err != nil {
		t.Fatal(err)
	}
	if d := e.download(hash1080); d.State != core.DownloadImported {
		t.Errorf("download = %s %v, want = imported", d.State, d.Messages)
	}
	if got, want := filepath.Base(e.item(it.ID).Path), "The Matrix (1999) Remux-1080p.mkv"; got != want {
		t.Errorf("imported file = %q, want = %q", got, want)
	}
	imported := e.history(core.HistoryImported)
	if len(imported) != 1 || imported[0].Quality.Quality != acquire.QualityID(release.Remux1080p) {
		t.Errorf("imported events = %+v, want = one of Remux-1080p", imported)
	}
	if _, err := e.rt.ListManualImport(e.ctx, d.ID); !errors.Is(err, core.ErrConflict) {
		t.Errorf("ListManualImport of an imported download: err = %v, want = conflict", err)
	}
}
