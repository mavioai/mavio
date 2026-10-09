package metadata

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPortedLyrics(t *testing.T) {
	portedCases(t, "lyrics/lrc_lyric_parser.json", ported{facts: map[string]string{
		"ParseElrcCues": "TestParseElrcCues",
	}})
}

// TestParseElrcCues ports Jellyfin's LrcLyricParserTests.ParseElrcCues.
func TestParseElrcCues(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "lyrics", "Fleetwood Mac - Rumors.elrc"))
	if err != nil {
		t.Fatal(err)
	}
	l, ok := ParseLyrics("Fleetwood Mac - Rumors.elrc", string(data))
	if !ok || len(l.Lines) != 31 {
		t.Fatalf("lines = %v %v, want 31", ok, l)
	}
	ticks := func(d time.Duration) int64 { return int64(d / 100) }
	line1 := l.Lines[0]
	if line1.Text != "Every night that goes between" || len(line1.Cues) != 5 {
		t.Fatalf("line 1 = %+v", line1)
	}
	c := line1.Cues
	if ticks(c[0].Start) != 68400000 || ticks(*c[0].End) != 72000000 || c[0].Position != 0 || c[0].EndPosition != 5 ||
		c[1].Position != 6 || c[1].EndPosition != 11 || c[2].Position != 12 {
		t.Errorf("line 1 cues = %+v", c)
	}
	line5 := l.Lines[4]
	if line5.Text != "Every night you do not come" || len(line5.Cues) != 6 ||
		ticks(line5.Cues[2].Start) != 375200000 || ticks(*line5.Cues[2].End) != 377300000 {
		t.Errorf("line 5 = %+v", line5)
	}
	last := l.Lines[len(l.Lines)-1]
	lc := last.Cues[len(last.Cues)-1]
	if last.Text != "I have always been a storm" || len(last.Cues) != 6 || ticks(lc.Start) != 2358000000 || lc.EndPosition != 26 || lc.End != nil {
		t.Errorf("last line = %+v, last cue %+v", last, lc)
	}
}

func TestParseLyrics(t *testing.T) {
	lrc := "[ar:Band]\n[ti:Song]\n[offset:+500]\n[00:10.00][00:30.00]Chorus\n[00:20.50]Verse 中文\n"
	l, ok := ParseLyrics("song.LRC", lrc)
	if !ok || !l.Metadata.Synced || l.Metadata.Artist != "Band" || l.Metadata.Title != "Song" || len(l.Lines) != 3 {
		t.Fatalf("lrc = %+v", l)
	}
	// Lines by time, the offset showing them half a second earlier.
	if *l.Lines[0].Start != 9500*time.Millisecond || l.Lines[1].Text != "Verse 中文" || *l.Lines[2].Start != 29500*time.Millisecond {
		t.Errorf("lines = %+v", l.Lines)
	}
	// Cues count UTF-16 code units.
	l, _ = ParseLyrics("x.elrc", "[00:01.00]<00:01.00>你好 <00:02.00>𝄞 world")
	if c := l.Lines[0].Cues; len(c) != 2 || c[0].EndPosition != 3 || c[1].Position != 3 || c[1].EndPosition != 11 {
		t.Errorf("unicode cues = %+v", c)
	}
	// Text without times reads as plain lines.
	l, ok = ParseLyrics("song.lrc", "first\r\n  second  \n")
	if !ok || l.Metadata.Synced || len(l.Lines) != 3 || l.Lines[1].Text != "second" || l.Lines[0].Start != nil {
		t.Errorf("plain lrc = %+v", l)
	}
	if l, ok := ParseLyrics("song.txt", "[00:01.00]not a time"); !ok || l.Metadata.Synced {
		t.Errorf("txt = %+v", l)
	}
	if _, ok := ParseLyrics("song.srt", "1"); ok {
		t.Error("an SRT file read as lyrics")
	}
}
