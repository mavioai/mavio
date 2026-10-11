package acquisition

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/release"
)

// TestNameExtras ports Radarr's SubtitleServiceFixture naming cases.
func TestNameExtras(t *testing.T) {
	folder := filepath.Join("downloads", "Movie.Title.2022")
	video := filepath.Join(folder, "Movie.Title.2022.mkv")
	for _, tc := range []struct {
		name  string
		files []string
		want  []string
	}{
		{"named as the video", []string{"Movie Title - 2022.srt"}, []string{".srt"}},
		{"two-letter code", []string{"Movie.Title.2022.en.srt"}, []string{".en.srt"}},
		{"language name", []string{"Movie.Title.2022.english.srt"}, []string{".en.srt"}},
		{"tags", []string{"Movie Title 2022_en_sdh_forced.srt"}, []string{".en.sdh.forced.srt"}},
		{"space", []string{"Movie_Title_2022 en.srt"}, []string{".en.srt"}},
		{"subfolder", []string{filepath.Join("Subs", "Movie.Title.2022", "2_en.srt")}, []string{".en.srt"}},
		{"no language", []string{"sub.srt"}, []string{".srt"}},
		{
			"several per language",
			[]string{"Movie.Title.2022.en.srt", "Movie.Title.2022.eng.srt", filepath.Join("Subs", "Movie_Title_2022_en_forced.srt"), filepath.Join("Subs", "Movie.Title.2022", "2_fr.srt")},
			[]string{".1.en.srt", ".2.en.srt", ".en.forced.srt", ".fr.srt"},
		},
		{
			"several per language with tags",
			[]string{"Movie.Title.2022.en.forced.cc.srt", "Movie.Title.2022.other.en.forced.cc.srt", "Movie.Title.2022.en.forced.sdh.srt", "Movie.Title.2022.en.forced.default.srt"},
			[]string{".1.en.forced.cc.srt", ".2.en.forced.cc.srt", ".en.forced.sdh.srt", ".en.forced.default.srt"},
		},
		{"other extras keep their suffix", []string{"Movie.Title.2022.nfo", "Movie.Title.2022-poster.jpg"}, []string{".nfo", "-poster.jpg"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var files []string
			for _, f := range tc.files {
				files = append(files, filepath.Join(folder, f))
			}
			var got []string
			for _, e := range nameExtras(video, files, release.Movie) {
				got = append(got, e.suffix)
			}
			if !slices.Equal(got, tc.want) {
				t.Errorf("suffixes = %q, want = %q", got, tc.want)
			}
		})
	}
}

// TestImportMatchesExtras checks which extras of a download imports take,
// as Radarr's SubtitleService.ImportFiles.
func TestImportMatchesExtras(t *testing.T) {
	folder := "The Matrix (1999)"
	base := "The Matrix (1999) Bluray-1080p"
	for _, tc := range []struct {
		name  string
		files []string
		want  []string
	}{
		{
			"named as the video or the movie",
			[]string{title1080 + ".en.srt", filepath.Join("Subs", "The.Matrix.1999", "2_fr.srt"), "The.Matrix.1999.de.srt", "Other.Movie.2005.srt"},
			[]string{base + ".en.srt", base + ".fr.srt", base + ".de.srt"},
		},
		{"any when the video is the only one", []string{filepath.Join("Subs", "sub.srt"), "The.Matrix.1999.1080p-sample.mkv"}, []string{base + ".srt"}},
		{"none when there are other videos", []string{filepath.Join("Subs", "sub.srt"), "Another.Video.mkv", "Third.Video.mkv"}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			it := e.addMovie()
			dir := e.grabAndComplete(it, title1080+".mkv")
			for _, f := range tc.files {
				p := filepath.Join(dir, f)
				if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(p, []byte(f), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if err := e.rt.CheckQueue(e.ctx); err != nil {
				t.Fatal(err)
			}
			if d := e.download(hash1080); d.State != core.DownloadImported {
				t.Fatalf("download = %s %v, want = imported", d.State, d.Messages)
			}
			var got []string
			entries, err := os.ReadDir(filepath.Join(e.libDir, folder))
			if err != nil {
				t.Fatal(err)
			}
			for _, en := range entries {
				if filepath.Ext(en.Name()) == ".srt" {
					got = append(got, en.Name())
				}
			}
			slices.Sort(got)
			want := slices.Sorted(slices.Values(tc.want))
			if !slices.Equal(got, want) {
				t.Errorf("subtitles = %q, want = %q", got, want)
			}
		})
	}
}

// TestUpgradeRecyclesOldExtras checks an upgrade recycles the extras of
// the file it replaces, as Radarr's ExtraService on MovieFileDeletedEvent.
func TestUpgradeRecyclesOldExtras(t *testing.T) {
	e := newEnv(t)
	it := e.addMovie()
	folder := filepath.Join(e.libDir, "The Matrix (1999)")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	old := "The Matrix (1999) Bluray-720p"
	for _, f := range []string{old + ".mkv", old + ".en.srt", old + ".1.fr.forced.ass", "folder.jpg", "The Matrix (1999) Other.en.srt"} {
		if err := os.WriteFile(filepath.Join(folder, f), []byte(f), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cur := e.item(it.ID)
	cur.Path, cur.Virtual = filepath.ToSlash(filepath.Join(folder, old+".mkv")), false
	if err := e.db.Items().Upsert(e.ctx, cur); err != nil {
		t.Fatal(err)
	}
	e.grabAndComplete(it, title1080+".mkv", title1080+".en.srt")
	if err := e.rt.CheckQueue(e.ctx); err != nil {
		t.Fatal(err)
	}
	if d := e.download(hash1080); d.State != core.DownloadImported {
		t.Fatalf("download = %s %v, want = imported", d.State, d.Messages)
	}
	for _, tc := range []struct {
		name string
		kept bool
	}{
		{old + ".en.srt", false},
		{old + ".1.fr.forced.ass", false},
		{"folder.jpg", true},
		{"The Matrix (1999) Other.en.srt", true},
		{"The Matrix (1999) Bluray-1080p.en.srt", true},
	} {
		_, err := os.Stat(filepath.Join(folder, tc.name))
		if kept := err == nil; kept != tc.kept {
			t.Errorf("%s kept = %v, want = %v", tc.name, kept, tc.kept)
		}
		if !tc.kept {
			if _, err := os.Stat(filepath.Join(e.binDir, "The Matrix (1999)", tc.name)); errors.Is(err, fs.ErrNotExist) {
				t.Errorf("%s not in the recycle bin", tc.name)
			}
		}
	}
}
