package acquisition

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
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
