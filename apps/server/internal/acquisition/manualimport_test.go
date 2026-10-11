package acquisition

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

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

	files, err := e.rt.ListManualImport(e.ctx, ManualImportQuery{DownloadID: d.ID})
	if err != nil {
		t.Fatal(err)
	}
	video := filepath.Join(dir, title1080+".mkv")
	if len(files) != 1 || files[0].Path != video || files[0].RelativePath != title1080+".mkv" ||
		files[0].Quality.Quality != release.Bluray1080p || files[0].MovieID != it.ID || files[0].ReleaseGroup != "GROUP" || !slices.Equal(files[0].Rejections, []string{msgNoFreeSpace}) {
		t.Fatalf("files = %+v, want = the video at Bluray-1080p rejected for free space", files)
	}

	if err := e.rt.ManualImport(e.ctx, ManualImportRequest{DownloadID: d.ID, Path: filepath.Join(dir, "other.mkv")}); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("ManualImport of another file: err = %v, want = not found", err)
	}
	q := acquire.QualityModel{Quality: release.Remux1080p, Revision: release.Revision{Version: 1}}
	if err := e.rt.ManualImport(e.ctx, ManualImportRequest{DownloadID: d.ID, Path: video, Quality: &q}); err != nil {
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
	if _, err := e.rt.ListManualImport(e.ctx, ManualImportQuery{DownloadID: d.ID}); !errors.Is(err, core.ErrConflict) {
		t.Errorf("ListManualImport of an imported download: err = %v, want = conflict", err)
	}
}

// addInception adds a second wanted movie, Inception.
func (e *env) addInception() core.Item {
	e.t.Helper()
	it, _, err := e.rt.AddMovie(e.ctx, NewMovie{LibraryID: e.lib.ID, TmdbID: 27205, QualityProfileID: e.profile})
	if err != nil {
		e.t.Fatal(err)
	}
	premiere := time.Date(2010, 7, 15, 0, 0, 0, 0, time.UTC)
	it.Name, it.ProductionYear, it.PremiereDate = "Inception", 2010, &premiere
	it.ExternalIDs = map[core.Provider]string{core.ProviderTMDB: "27205", core.ProviderIMDb: "tt1375666"}
	if err := e.db.Items().Upsert(e.ctx, it); err != nil {
		e.t.Fatal(err)
	}
	return it
}

// TestManualImportOtherMovie checks a file of a tracked download is
// imported by hand for another movie than the download's, and the
// download completes, as Radarr's ManualImportService.
func TestManualImportOtherMovie(t *testing.T) {
	e := newEnv(t)
	e.rt.FreeSpace = func(string) (int64, bool) { return 0, true }
	it := e.addMovie()
	other := e.addInception()
	dir := e.grabAndComplete(it, title1080+".mkv")
	if err := e.rt.CheckQueue(e.ctx); err != nil {
		t.Fatal(err)
	}
	d := e.download(hash1080)
	files, err := e.rt.ListManualImport(e.ctx, ManualImportQuery{DownloadID: d.ID, MovieID: other.ID})
	if err != nil || len(files) != 1 || files[0].MovieID != other.ID {
		t.Fatalf("files = %+v, %v, want = the video for Inception", files, err)
	}
	if err := e.rt.ManualImport(e.ctx, ManualImportRequest{DownloadID: d.ID, Path: filepath.Join(dir, title1080+".mkv"), MovieID: other.ID}); err != nil {
		t.Fatal(err)
	}
	if got, want := filepath.Base(e.item(other.ID).Path), "Inception (2010) Bluray-1080p.mkv"; got != want {
		t.Errorf("imported file = %q, want = %q", got, want)
	}
	if got := e.item(it.ID); !got.Virtual {
		t.Errorf("The Matrix = %q, want = still without a file", got.Path)
	}
	if d := e.download(hash1080); d.State != core.DownloadImported {
		t.Errorf("download = %s %v, want = imported", d.State, d.Messages)
	}
	if imported := e.history(core.HistoryImported); len(imported) != 1 || imported[0].ItemID != other.ID {
		t.Errorf("imported events = %+v, want = one of Inception", imported)
	}
}

// TestManualImportFolder checks the files of a folder that is not a
// tracked download are listed with the movie their names match, and
// imported by hand with the languages and release group chosen, copied,
// as Radarr's ManualImportService.
func TestManualImportFolder(t *testing.T) {
	e := newEnv(t)
	it := e.addMovie()
	folder := filepath.Join(e.dlDir, "manual")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	const (
		matrix  = "The.Matrix.1999.FRENCH.1080p.BluRay.x264-GRP.mkv"
		unknown = "Some.Home.Video.2004.720p.mkv"
	)
	for _, f := range []string{matrix, unknown} {
		if err := os.WriteFile(filepath.Join(folder, f), []byte("content of "+f), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := e.rt.ListManualImport(e.ctx, ManualImportQuery{Folder: "relative"}); !errors.Is(err, core.ErrInvalid) {
		t.Errorf("ListManualImport of a relative folder: err = %v, want = invalid", err)
	}
	files, err := e.rt.ListManualImport(e.ctx, ManualImportQuery{Folder: folder})
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]ManualImportFile{}
	for _, f := range files {
		byName[f.RelativePath] = f
	}
	if f := byName[matrix]; f.MovieID != it.ID || f.Quality.Quality != release.Bluray1080p ||
		!slices.Equal(f.Languages, []release.Language{release.French}) || f.ReleaseGroup != "GRP" || len(f.Rejections) != 0 {
		t.Errorf("file = %+v, want = The Matrix at Bluray-1080p in French by GRP", f)
	}
	if f := byName[unknown]; !f.MovieID.IsZero() || !slices.Equal(f.Rejections, []string{msgUnknownMovie}) {
		t.Errorf("file = %+v, want = of an unknown movie", f)
	}
	if err := e.rt.ManualImport(e.ctx, ManualImportRequest{Folder: folder, Path: filepath.Join(folder, unknown)}); !errors.Is(err, core.ErrInvalid) {
		t.Errorf("ManualImport of an unknown movie's file: err = %v, want = invalid", err)
	}

	group := "MINE"
	if err := e.rt.ManualImport(e.ctx, ManualImportRequest{
		Folder: folder, Path: filepath.Join(folder, matrix), Languages: []release.Language{release.German}, ReleaseGroup: &group, Mode: ImportCopy,
	}); err != nil {
		t.Fatal(err)
	}
	if got := e.item(it.ID); got.Virtual || got.Path == "" {
		t.Errorf("The Matrix = %+v, want = imported", got)
	}
	if _, err := os.Stat(filepath.Join(folder, matrix)); err != nil {
		t.Errorf("copied file: %v, want = still in the folder", err)
	}
	imported := e.history(core.HistoryImported)
	if len(imported) != 1 || !slices.Equal(imported[0].Languages, []string{"German"}) || imported[0].Data[dataReleaseGroup] != group ||
		imported[0].Data[dataDownloadClient] != "" || imported[0].DownloadID != "" {
		t.Errorf("imported events = %+v, want = one in German by MINE without a download client", imported)
	}
}
