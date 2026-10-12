package planner

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/media/decision"
	"github.com/mavioai/mavio/libs/media/probe"
	"github.com/mavioai/mavio/tools/fixtures"
)

// TestHLSSubtitles runs remuxes that write the embedded subtitles as WebVTT
// beside the segments and checks their cues keep the source's times.
func TestHLSSubtitles(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("integration test: ffmpeg not found on PATH")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("integration test: ffprobe not found on PATH")
	}
	path := fixtures.Require(t, "multi_track.mkv")
	res, err := (&probe.Prober{FFprobe: ffprobe}).Probe(t.Context(), probe.Request{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	ms := res.Source
	ms.ID, ms.Path = core.NewID(), path
	src := &decision.Source{MediaSource: &ms}
	var subs []*core.MediaStream
	for i := range ms.Streams {
		if ms.Streams[i].Kind == core.StreamSubtitle {
			subs = append(subs, &ms.Streams[i])
		}
	}
	if len(subs) != 2 {
		t.Fatalf("subtitle streams: got = %d, want = 2", len(subs))
	}
	for _, tt := range []struct {
		start    time.Duration
		eng, jpn string
	}{
		{0, "00:00.000 --> 00:01.000\nHello from Mavio.", "00:00.000 --> 00:01.000\nこんにちは。"},
		// A seeking transcode keeps the source's times; it starts at the
		// keyframe the seek lands on.
		{time.Second, "00:01.000 --> 00:02.000\nSecond line.", "00:01.000 --> 00:02.000\n二行目。"},
	} {
		t.Run(tt.start.String(), func(t *testing.T) {
			d, err := (&decision.Builder{}).Video(&decision.Request{
				Sources: []*decision.Source{src}, Client: hlsClient(false), Context: decision.Streaming,
				AllowAudioStreamCopy: true, AllowVideoStreamCopy: true,
			})
			if err != nil {
				t.Fatal(err)
			}
			p := &Planner{Options: DefaultOptions()}
			j := p.Job(d, tt.start)
			dir := t.TempDir()
			out := HLSOutput{Playlist: filepath.Join(dir, "main.m3u8"), Segments: filepath.Join(dir, "seg%d.mp4")}
			for _, s := range subs {
				out.Subtitles = append(out.Subtitles, SubtitleOutput{Stream: s, Path: filepath.Join(dir, s.Language+".vtt")})
			}
			args := p.HLSArgs(j, out)
			if b, err := exec.CommandContext(t.Context(), ffmpeg, args...).CombinedOutput(); err != nil {
				t.Fatalf("ffmpeg %s: %v\n%s", strings.Join(args, " "), err, b)
			}
			for lang, want := range map[string]string{"eng": tt.eng, "jpn": tt.jpn} {
				b, err := os.ReadFile(filepath.Join(dir, lang+".vtt"))
				if err != nil {
					t.Fatal(err)
				}
				if !strings.HasPrefix(string(b), "WEBVTT") || !strings.Contains(string(b), want) {
					t.Errorf("%s: got = %q, want = a cue %q", lang, b, want)
				}
			}
		})
	}
}
