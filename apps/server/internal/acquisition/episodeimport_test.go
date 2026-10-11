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

// TestEpisodeImportSpaceExtrasAndCleanup checks episode imports check the free space
// of the series folder's volume per file, as Sonarr's
// FreeSpaceSpecification; import the extras of a file named by language,
// as Sonarr's SubtitleService; delete the download folder the files were
// moved out of once only samples are left, as Sonarr's
// DownloadedEpisodesImportService.ShouldDeleteFolder; and recycle the
// extras of the files upgrades replace, as Sonarr's ExtraFileService.
func TestEpisodeImportSpaceExtrasAndCleanup(t *testing.T) {
	e := newSeriesEnv(t)
	var asked []string
	free := int64(0)
	e.rt.FreeSpace = func(p string) (int64, bool) {
		asked = append(asked, p)
		return free, true
	}
	s := e.addSeries(81189, core.SeriesStandard)
	ep1 := e.episode(s.ID, 1, 1)
	const s01e01 = "Breaking.Bad.S01E01.720p.HDTV.x264-GRP"
	e.tvReleases(gb, s01e01)
	e.searchEpisodes(s.ID, ep1.ID)
	dir := e.completed(hashOf(s01e01), s01e01, s01e01+".mkv", "sample-"+s01e01+".mkv", s01e01+".en.srt", "Breaking.Bad.S01E02.de.srt")
	if err := os.MkdirAll(filepath.Join(dir, "Subs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Subs", "Breaking.Bad.S01E01.fr.srt"), []byte("fr"), 0o644); err != nil {
		t.Fatal(err)
	}
	e.client.setDownloads(pluginv1.Download_builder{
		ClientId: new("qb"), Id: new(hashOf(s01e01)), Title: new(s01e01), Status: new(pluginv1.DownloadStatus_DOWNLOAD_STATUS_COMPLETED),
		OutputPath: new("/downloads/" + s01e01), CanBeRemoved: new(true), CanMoveFiles: new(true),
	}.Build())

	e.checkQueue()
	d := e.download(hashOf(s01e01))
	if d.State != core.DownloadImportBlocked || !slices.ContainsFunc(d.Messages, func(m string) bool { return strings.Contains(m, msgNoFreeSpace) }) {
		t.Fatalf("download = %s %v, want = blocked for free space", d.State, d.Messages)
	}
	seriesDir := filepath.Join(e.showsDir, "Breaking Bad")
	if want := []string{seriesDir}; !slices.Equal(asked, want) {
		t.Errorf("free space asked of %v, want = %v", asked, want)
	}

	free = 100 * gb
	e.checkQueue()
	if d := e.download(hashOf(s01e01)); d.State != core.DownloadImported {
		t.Fatalf("download = %s %v, want = imported", d.State, d.Messages)
	}
	season := filepath.Join(seriesDir, "Season 1")
	base := "Breaking Bad - S01E01 - Pilot HDTV-720p"
	if got, want := subtitles(t, season), []string{base + ".en.srt", base + ".fr.srt"}; !slices.Equal(got, want) {
		t.Errorf("subtitles = %q, want = %q", got, want)
	}
	if _, err := os.Stat(dir); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("download folder stat = %v, want deleted", err)
	}

	// An upgrade recycles the old file's extras.
	const better = "Breaking.Bad.S01E01.1080p.WEB-DL.DD5.1.H.264-GRP"
	e.tvReleases(2*gb, better)
	e.clock = e.clock.Add(time.Hour)
	e.searchEpisodes(s.ID, ep1.ID)
	e.completed(hashOf(better), better, better+".mkv")
	e.checkQueue()
	if d := e.download(hashOf(better)); d.State != core.DownloadImported {
		t.Fatalf("upgrade download = %s %v, want = imported", d.State, d.Messages)
	}
	if got := subtitles(t, season); len(got) != 0 {
		t.Errorf("subtitles = %q, want = none", got)
	}
	for _, name := range []string{base + ".en.srt", base + ".fr.srt"} {
		if _, err := os.Stat(filepath.Join(e.binDir, "Breaking Bad", name)); err != nil {
			t.Errorf("%s not in the recycle bin: %v", name, err)
		}
	}
}

// subtitles returns the names of the subtitles in a folder, sorted.
func subtitles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, en := range entries {
		if filepath.Ext(en.Name()) == ".srt" {
			out = append(out, en.Name())
		}
	}
	slices.Sort(out)
	return out
}
