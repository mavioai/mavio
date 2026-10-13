package library

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

func TestScanSidecarSubtitles(t *testing.T) {
	f := newScan(t, core.LibraryMovies)
	tree(t, f.root,
		"Up (2009)/Up (2009).mkv",
		"Up (2009)/Up (2009).简体.srt",
		"Up (2009)/Up (2009).cht.forced.ass",
		"Up (2009)/Up (2009).en.sdh.srt",
		"Up (2009)/Up (2009).srt",
		"Up (2009)/Up (2009).ja.idx",
		"Up (2009)/Up (2009).ja.sub",
		"Up (2009)/Upside Down.srt",
		"Up (2009)/notes.txt",
	)
	f.scan()
	up := f.item("Up (2009)/Up (2009).mkv")
	src := f.store.sources[up.ID]
	if len(src) != 1 {
		t.Fatalf("sources = %+v", src)
	}
	describe := func(streams []core.MediaStream) []string {
		var out []string
		for _, s := range streams {
			flags := ""
			if s.Forced {
				flags += " forced"
			}
			if s.HearingImpaired {
				flags += " sdh"
			}
			out = append(out, fmt.Sprintf("%d %s %s %s%s", s.Index, filepath.Base(s.ExternalPath), s.Codec, s.Language, flags))
		}
		return out
	}
	// Unflagged files named like the video first, richer formats before
	// plainer ones; forced and hearing-impaired ones after.
	want := []string{
		"0 Up (2009).srt subrip ",
		"1 Up (2009).简体.srt subrip zh-Hans",
		"2 Up (2009).ja.idx dvd_subtitle jpn",
		"3 Up (2009).en.sdh.srt subrip eng sdh",
		"4 Up (2009).cht.forced.ass ass zh-Hant forced",
	}
	if got := describe(src[0].Streams); !slices.Equal(got, want) {
		t.Errorf("sidecars:\n got = %q\nwant = %q", got, want)
	}

	// Probing keeps the sidecars after the embedded streams.
	merged := withSidecars([]core.MediaStream{{Index: 0, Kind: core.StreamVideo}, {Index: 1, Kind: core.StreamAudio}}, sidecarsOf(src[0].Streams))
	if len(merged) != 7 || merged[2].Index != 2 || merged[2].ExternalPath == "" || merged[6].Index != 6 {
		t.Errorf("merged = %+v", merged)
	}

	// A subtitle added later is found without probing the unchanged video.
	src[0].ProbedAt = f.clock
	f.store.sources[up.ID] = src
	tree(t, f.root, "Up (2009)/Up (2009).fr.srt")
	touchLater(t, filepath.Join(f.root, "Up (2009)"), time.Minute)
	if st := f.scan(); st.Probes != 0 {
		t.Errorf("probes = %d, want none for a new subtitle", st.Probes)
	}
	if got := describe(f.store.sources[up.ID][0].Streams); len(got) != 6 || !slices.Contains(got, "1 Up (2009).fr.srt subrip fre") {
		t.Errorf("after adding a subtitle = %q", got)
	}
}

func TestSubtitleFolders(t *testing.T) {
	sub := []byte("1\n00:00:01,000 --> 00:00:02,000\nHi\n")
	paths := func(streams []core.MediaStream) []string {
		var out []string
		for _, s := range sidecarsOf(streams) {
			out = append(out, s.ExternalPath)
		}
		slices.Sort(out)
		return out
	}
	t.Run("relative", func(t *testing.T) {
		f := newScan(t, core.LibraryMovies)
		folder := SubtitleFolder{Kind: core.SubtitleFolderRelative, Relative: "Subs"}
		f.sc.SubtitleFolder = func() SubtitleFolder { return folder }
		tree(t, f.root, "Up (2009)/Up (2009).mkv", "Up (2009)/Up (2009).en.srt", "Up (2009)/Subs/Up (2009).ja.srt", "Up (2009)/Subs/Other.srt")
		f.scan()
		up := f.item("Up (2009)/Up (2009).mkv")
		dir := f.root + "/Up (2009)"
		want := []string{dir + "/Subs/Up (2009).ja.srt", dir + "/Up (2009).en.srt"}
		if got := paths(f.store.sources[up.ID][0].Streams); !slices.Equal(got, want) {
			t.Errorf("scanned = %q, want = %q", got, want)
		}
		// A file added to the subtitle folder changes the video's folder.
		f.store.sources[up.ID][0].ProbedAt = f.clock
		tree(t, f.root, "Up (2009)/Subs/Up (2009).fr.srt")
		touchLater(t, filepath.Join(f.root, "Up (2009)", "Subs"), time.Minute)
		f.scan()
		if got := paths(f.store.sources[up.ID][0].Streams); len(got) != 3 {
			t.Errorf("after adding = %q", got)
		}

		s := &Subtitles{Store: f.store, Folder: func() SubtitleFolder { return folder }}
		st, err := s.Save(t.Context(), f.lib, up.ID, core.ID{}, func(_ fs.FS, stem string) (string, error) { return stem + ".de.srt", nil }, sub)
		if err != nil || st.ExternalPath != dir+"/Subs/Up (2009).de.srt" || st.Language != "ger" {
			t.Fatalf("Save = %+v, %v", st, err)
		}
		if data, err := s.ReadFile(t.Context(), up.ID, st.ExternalPath); err != nil || string(data) != string(sub) {
			t.Errorf("ReadFile = %q, %v", data, err)
		}
		if err := s.Remove(t.Context(), f.lib, up.ID, st.ExternalPath); err != nil || exists(f.root, "Up (2009)/Subs/Up (2009).de.srt") {
			t.Errorf("Remove = %v", err)
		}
		if got := paths(f.store.sources[up.ID][0].Streams); len(got) != 3 {
			t.Errorf("after removing = %q", got)
		}
	})
	t.Run("metadata", func(t *testing.T) {
		f := newScan(t, core.LibraryMovies)
		meta := filepath.ToSlash(t.TempDir())
		folder := SubtitleFolder{Kind: core.SubtitleFolderMetadata, MetadataDir: meta}
		f.sc.SubtitleFolder = func() SubtitleFolder { return folder }
		tree(t, f.root, "Up (2009)/Up (2009).mkv", "Up (2009)/Up (2009).en.srt", "Cars (2006)/Cars (2006).mkv")
		f.scan()
		up := f.item("Up (2009)/Up (2009).mkv")
		dir := SubtitleDir(meta, up.ID)

		s := &Subtitles{Store: f.store, Folder: func() SubtitleFolder { return folder }}
		st, err := s.Save(t.Context(), f.lib, up.ID, core.ID{}, func(_ fs.FS, stem string) (string, error) { return stem + ".de.srt", nil }, sub)
		if err != nil || st.ExternalPath != dir+"/Up (2009).de.srt" || st.Language != "ger" {
			t.Fatalf("Save = %+v, %v", st, err)
		}
		want := []string{f.root + "/Up (2009)/Up (2009).en.srt", dir + "/Up (2009).de.srt"}
		if got := paths(f.store.sources[up.ID][0].Streams); !slices.Equal(got, want) {
			t.Errorf("after saving = %q, want = %q", got, want)
		}
		// A scan of the changed folder keeps the subtitles in the metadata
		// folder.
		tree(t, f.root, "Up (2009)/Up (2009).fr.srt")
		touchLater(t, filepath.Join(f.root, "Up (2009)"), time.Minute)
		f.scan()
		if got := paths(f.store.sources[up.ID][0].Streams); len(got) != 3 || !slices.Contains(got, dir+"/Up (2009).de.srt") {
			t.Errorf("after scanning = %q", got)
		}
		if err := s.WriteFile(t.Context(), up.ID, st.ExternalPath, []byte("changed")); err != nil {
			t.Fatal(err)
		}
		if data, err := s.ReadFile(t.Context(), up.ID, st.ExternalPath); err != nil || string(data) != "changed" {
			t.Errorf("ReadFile = %q, %v", data, err)
		}
		if err := s.Remove(t.Context(), f.lib, up.ID, st.ExternalPath); err != nil || exists(dir, "Up (2009).de.srt") {
			t.Errorf("Remove = %v", err)
		}
		if got := paths(f.store.sources[up.ID][0].Streams); len(got) != 2 {
			t.Errorf("after removing = %q", got)
		}
		// Another item's subtitles are not this one's.
		cars := f.item("Cars (2006)/Cars (2006).mkv")
		if _, err := s.ReadFile(t.Context(), cars.ID, st.ExternalPath); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("ReadFile of another item's subtitle = %v, want ErrNotFound", err)
		}
	})
}
