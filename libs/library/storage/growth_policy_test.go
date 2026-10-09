package storage

import (
	"testing"
)

func TestDecideGrowth_StaticUnchanged(t *testing.T) {
	stamp := FileStamp{Size: 1000, MtimeNs: 1000000}
	in := GrowthInputs{
		Mode:      ModeStatic,
		AtOpen:    stamp,
		Now:       stamp,
		ReadPos:   1000,
		WallNowNs: 1000000 + RecentMtimeNs + 1, // older than RecentMtimeNs
	}

	d := DecideGrowth(in)
	if d.Mode != ModeStatic || d.Action != ActionEndOfFile {
		t.Fatalf("got (%v, %v), want (ModeStatic, ActionEndOfFile)", d.Mode, d.Action)
	}
}

func TestDecideGrowth_StaticSizeIncreased(t *testing.T) {
	atOpen := FileStamp{Size: 1000, MtimeNs: 1000000}
	now := FileStamp{Size: 2000, MtimeNs: 2000000}
	in := GrowthInputs{
		Mode:    ModeStatic,
		AtOpen:  atOpen,
		Now:     now,
		ReadPos: 1000,
	}

	d := DecideGrowth(in)
	if d.Mode != ModeGrowing || d.Action != ActionRetry {
		t.Fatalf("got (%v, %v), want (ModeGrowing, ActionRetry)", d.Mode, d.Action)
	}
}

func TestDecideGrowth_StaticRecentWriterOpen(t *testing.T) {
	stamp := FileStamp{Size: 1000, MtimeNs: 1000000}
	in := GrowthInputs{
		Mode:       ModeStatic,
		AtOpen:     stamp,
		Now:        stamp,
		ReadPos:    1000,
		WallNowNs:  1000000 + 1000, // recent (< 5s)
		WriterOpen: 1,
	}

	d := DecideGrowth(in)
	if d.Mode != ModeGrowing || d.Action != ActionWait {
		t.Fatalf("got (%v, %v), want (ModeGrowing, ActionWait)", d.Mode, d.Action)
	}
}

func TestDecideGrowth_StaticRecentWriterUnknown(t *testing.T) {
	stamp := FileStamp{Size: 1000, MtimeNs: 1000000}
	in := GrowthInputs{
		Mode:       ModeStatic,
		AtOpen:     stamp,
		Now:        stamp,
		ReadPos:    1000,
		WallNowNs:  1000000 + 1000, // recent (< 5s)
		WriterOpen: -1,
	}

	d := DecideGrowth(in)
	if d.Mode != ModeProbing || d.Action != ActionWait {
		t.Fatalf("got (%v, %v), want (ModeProbing, ActionWait)", d.Mode, d.Action)
	}
}

func TestDecideGrowth_ProbingGracePeriodExpired(t *testing.T) {
	stamp := FileStamp{Size: 1000, MtimeNs: 1000000}
	in := GrowthInputs{
		Mode:         ModeProbing,
		AtOpen:       stamp,
		Now:          stamp,
		ReadPos:      1000,
		ProbeStartUs: 1000,
		MonoNowUs:    1000 + ProbeGraceUs + 1, // grace period expired
	}

	d := DecideGrowth(in)
	if d.Mode != ModeStatic || d.Action != ActionEndOfFile {
		t.Fatalf("got (%v, %v), want (ModeStatic, ActionEndOfFile)", d.Mode, d.Action)
	}
}

func TestDecideGrowth_GrowingBytesAhead(t *testing.T) {
	atOpen := FileStamp{Size: 1000, MtimeNs: 1000000}
	now := FileStamp{Size: 1500, MtimeNs: 1500000}
	in := GrowthInputs{
		Mode:    ModeGrowing,
		AtOpen:  atOpen,
		Now:     now,
		ReadPos: 1000, // read position is behind available bytes
	}

	d := DecideGrowth(in)
	if d.Mode != ModeGrowing || d.Action != ActionRetry {
		t.Fatalf("got (%v, %v), want (ModeGrowing, ActionRetry)", d.Mode, d.Action)
	}
}

func TestDecideGrowth_GrowingStalledFinal(t *testing.T) {
	now := FileStamp{Size: 1000, MtimeNs: 1000000}
	in := GrowthInputs{
		Mode:         ModeGrowing,
		AtOpen:       now,
		Now:          now,
		ReadPos:      1000,
		LastGrowthUs: 1000,
		MonoNowUs:    1000 + IdleFinalUs + 1, // idle > 30s
		WriterOpen:   0,
	}

	d := DecideGrowth(in)
	if d.Mode != ModeFinal || d.Action != ActionEndOfFile {
		t.Fatalf("got (%v, %v), want (ModeFinal, ActionEndOfFile)", d.Mode, d.Action)
	}
}

func TestHasDownloadSuffix(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"/downloads/movie.mkv.part", true},
		{"/downloads/movie.mp4.crdownload", true},
		{"/downloads/movie.download", true},
		{"/downloads/movie.aria2", true},
		{"/downloads/movie.mkv.tmp", true},
		{"/downloads/movie.incomplete", true},
		{"/downloads/movie.!ut", true},
		{"/downloads/movie.mkv", false},
		{"/downloads/movie.mp4", false},
		{"/downloads/part.mkv", false},
	}

	for _, tc := range cases {
		if got := HasDownloadSuffix(tc.path); got != tc.want {
			t.Errorf("HasDownloadSuffix(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}
