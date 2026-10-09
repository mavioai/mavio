package library

import (
	"fmt"
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
