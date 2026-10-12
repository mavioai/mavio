package streaming

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/media/planner"
	"github.com/mavioai/mavio/libs/media/supervisor"
)

func newSubtitleStream(t *testing.T, l Layout, f *fakeFFmpeg) *Stream {
	t.Helper()
	args := func(start time.Duration, out planner.HLSOutput) []string {
		a := testArgs(start, out)
		for _, s := range out.Subtitles {
			a = append(a, "-vtt", s.Path)
		}
		return a
	}
	s, err := NewStream(Config{
		Dir: filepath.Join(t.TempDir(), "stream"), Layout: l, Args: args, Start: f.start,
		Subtitles:   []*core.MediaStream{{Index: 2, Kind: core.StreamSubtitle, Codec: "subrip"}},
		Supervision: supervisor.Config{CheckInterval: time.Hour, StopGrace: time.Second},
		MaxGap:      3 * time.Second, Poll: time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func subtitleOf(t *testing.T, s *Stream, index int) string {
	t.Helper()
	data, err := s.Subtitle(t.Context(), 2, index)
	if err != nil {
		t.Fatalf("Subtitle(2, %d): %v", index, err)
	}
	if !strings.HasPrefix(string(data), "WEBVTT\n") {
		t.Fatalf("Subtitle(2, %d) = %q, want = WebVTT", index, data)
	}
	return string(data)
}

func TestStreamSubtitles(t *testing.T) {
	f := &fakeFFmpeg{last: 29, interval: 5 * time.Millisecond}
	s := newSubtitleStream(t, encoded(t, 30), f)
	if _, err := s.Subtitle(t.Context(), 3, 0); !errors.Is(err, ErrSegmentRange) {
		t.Errorf("unknown stream: err = %v, want = %v", err, ErrSegmentRange)
	}
	// The first request starts the run and waits for it to read past the
	// segment.
	if got := subtitleOf(t, s, 0); !strings.Contains(got, "file0") || !strings.Contains(got, "file1") {
		t.Errorf("segment 0 = %q, want = the cues through file1", got)
	}
	read(t, s, 4)
	// Far ahead: a new run, as for the video.
	if got := subtitleOf(t, s, 20); !strings.Contains(got, "file20") || strings.Contains(got, "file3\n") {
		t.Errorf("segment 20 = %q, want = the new run's cues", got)
	}
	// Back to what the first run read: its cues, without another run.
	if got := subtitleOf(t, s, 2); !strings.Contains(got, "file2") {
		t.Errorf("segment 2 = %q", got)
	}
	want := []run{{0, 0}, {20 * time.Second, 20}}
	if runs := f.runsSoFar(); !slices.Equal(runs, want) {
		t.Errorf("runs = %+v, want = %+v", runs, want)
	}
}

func TestStreamSubtitlesLastSegment(t *testing.T) {
	f := &fakeFFmpeg{last: 2, interval: time.Millisecond}
	s := newSubtitleStream(t, encoded(t, 3), f)
	// The last segment needs no file after it.
	if got := subtitleOf(t, s, 2); !strings.Contains(got, "file2") {
		t.Errorf("segment 2 = %q", got)
	}
}
