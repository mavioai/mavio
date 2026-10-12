package streaming

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/media/decision"
	"github.com/mavioai/mavio/libs/media/keyframes"
	"github.com/mavioai/mavio/libs/media/planner"
	"github.com/mavioai/mavio/libs/media/probe"
	"github.com/mavioai/mavio/libs/media/supervisor"
)

// TestStreamLiveSubtitles remuxes a 40 s file with a cue every two seconds
// through ffmpeg, seeking ahead and back, and checks that each segment's
// subtitles hold the cues starting within it.
func TestStreamLiveSubtitles(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("integration test: ffmpeg not found on PATH")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("integration test: ffprobe not found on PATH")
	}
	dir := t.TempDir()
	var srt strings.Builder
	for i := range 20 {
		fmt.Fprintf(&srt, "%d\n00:00:%02d,000 --> 00:00:%02d,500\ncue %d\n\n", i+1, 2*i, 2*i+1, i)
	}
	cues := filepath.Join(dir, "cues.srt")
	if err := os.WriteFile(cues, []byte(srt.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(dir, "source.mkv")
	gen := exec.Command(ffmpeg, "-loglevel", "error", "-f", "lavfi", "-i", "testsrc2=size=320x240:rate=24",
		"-f", "lavfi", "-i", "sine=frequency=440", "-i", cues, "-t", "40",
		"-map", "0:v", "-map", "1:a", "-map", "2:s",
		"-c:v", "libx264", "-g", "36", "-keyint_min", "36", "-sc_threshold", "0", "-c:a", "aac", "-c:s", "srt", src)
	if out, err := gen.CombinedOutput(); err != nil {
		t.Skipf("integration test: cannot generate the source: %v\n%s", err, out)
	}
	res, err := (&probe.Prober{FFprobe: ffprobe}).Probe(t.Context(), probe.Request{Path: src})
	if err != nil {
		t.Fatal(err)
	}
	kf, err := keyframes.FromFile(src)
	if err != nil {
		t.Fatal(err)
	}
	ms := res.Source
	ms.Path = src
	sub := &ms.Streams[2]
	job := &planner.Job{
		Delivery: planner.HLS, Source: &decision.Source{MediaSource: &ms}, Video: &ms.Streams[0], Audio: &ms.Streams[1],
		Request: &decision.Decision{}, VideoCodec: planner.Copy, AudioCodec: planner.Copy,
		AllowVideoCopy: true, AllowAudioCopy: true, SegmentContainer: "mp4", SegmentLength: 4 * time.Second,
	}
	p := &planner.Planner{Options: planner.DefaultOptions()}
	layout := CopiedLayout(kf.Keyframes, kf.Duration, 4*time.Second)
	s, err := NewStream(Config{
		Dir: filepath.Join(t.TempDir(), "stream"), Layout: layout, Subtitles: []*core.MediaStream{sub},
		Args: func(start time.Duration, out planner.HLSOutput) []string {
			j := *job
			j.Start = start
			return p.HLSArgs(&j, out)
		},
		Start:  supervisor.Exec(ffmpeg),
		MaxGap: 8 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })

	last := len(layout.Segments) - 1
	for _, i := range []int{0, 1, last - 1, last, 2} {
		data, err := s.Subtitle(t.Context(), sub.Index, i)
		if err != nil {
			t.Fatalf("Subtitle(%d): %v", i, err)
		}
		for c := range 20 {
			at := time.Duration(2*c) * time.Second
			if at >= layout.Start(i) && at < layout.End(i) && !strings.Contains(string(data), fmt.Sprintf("cue %d\n", c)) {
				t.Errorf("segment %d [%v, %v): got = %q, want = cue %d", i, layout.Start(i), layout.End(i), data, c)
			}
		}
	}
}
