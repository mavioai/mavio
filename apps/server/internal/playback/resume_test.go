package playback

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/media/decision"
	"github.com/mavioai/mavio/libs/media/supervisor"
	"github.com/mavioai/mavio/libs/streaming"
)

func TestStartPosition(t *testing.T) {
	s := time.Second
	layout, err := streaming.EncodedLayout(6*s, 100*s)
	if err != nil {
		t.Fatal(err)
	}
	hls := &Playback{stream: &streaming.Stream{}, layout: layout}
	direct := &Playback{Method: decision.DirectPlay, Decision: &decision.Decision{Source: &decision.Source{
		MediaSource: &core.MediaSource{Keyframes: []time.Duration{0, 10 * s, 20 * s, 40 * s}},
	}}}
	transcode := &Playback{Method: decision.Transcode, Decision: direct.Decision}
	cases := []struct {
		name string
		p    *Playback
		at   time.Duration
		want time.Duration
	}{
		{"hls from the start", hls, 0, 0},
		{"hls in a segment", hls, 50 * s, 48 * s},
		{"hls at a boundary", hls, 54 * s, 54 * s},
		{"direct play near a keyframe", direct, 23 * s, 20 * s},
		{"direct play five seconds on", direct, 25 * s, 20 * s},
		{"direct play far from one", direct, 30 * s, 30 * s},
		{"direct play on a keyframe", direct, 40 * s, 40 * s},
		{"direct play past the last", direct, 44 * s, 40 * s},
		{"transcode keeps the position", transcode, 23 * s, 23 * s},
	}
	for _, c := range cases {
		if got := c.p.startPosition(c.at); got != c.want {
			t.Errorf("%s: startPosition(%v) = %v, want = %v", c.name, c.at, got, c.want)
		}
	}
}

// A playback resuming over HLS starts ffmpeg at once, at the segment it
// begins with, rather than at the beginning when the client asks for the
// initialization segment.
func TestResumeFoldsSeek(t *testing.T) {
	e := newEnv(t)
	var mu sync.Mutex
	var runs [][]string
	e.m.cfg.FFmpeg = func(_ context.Context, args []string) (supervisor.Process, error) {
		mu.Lock()
		runs = append(runs, args)
		mu.Unlock()
		return nil, errors.New("not ffmpeg")
	}
	hls := &decision.ClientCapabilities{
		Name: "hls",
		Transcoding: []decision.TranscodingProfile{{
			Kind: decision.Video, Context: decision.Streaming, Protocol: decision.HLS, Container: "mp4", VideoCodec: "h264", AudioCodec: "aac",
		}},
		Subtitles: []decision.SubtitleProfile{{Format: "vtt", Method: decision.SubtitleExternal}},
	}
	p := e.start(t, Request{Client: hls, Start: 40*time.Minute + 3*time.Second})
	if p.StartPosition != 40*time.Minute {
		t.Errorf("StartPosition = %v, want = 40m0s", p.StartPosition)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(runs) != 1 {
		t.Fatalf("ffmpeg runs = %d, want = 1", len(runs))
	}
	if i := slices.Index(runs[0], "-ss"); i < 0 || runs[0][i+1] != "00:40:00.000" {
		t.Errorf("ffmpeg args = %v, want = -ss 00:40:00.000", runs[0])
	}
}
