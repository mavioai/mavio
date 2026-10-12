package playback

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/media/decision"
	"github.com/mavioai/mavio/libs/media/supervisor"
	"github.com/mavioai/mavio/tools/fixtures"
)

func TestExtractedSubtitles(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("integration test: ffmpeg not found on PATH")
	}
	if runtime.GOOS == "windows" {
		t.Skip("counts ffmpeg runs through a shell script")
	}
	path := fixtures.Require(t, "multi_track.mkv")
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	// A wrapper counts the runs.
	runs := filepath.Join(t.TempDir(), "runs")
	wrapper := filepath.Join(t.TempDir(), "ffmpeg")
	script := "#!/bin/sh\necho run >> '" + runs + "'\nexec '" + ffmpeg + "' \"$@\"\n"
	if err := os.WriteFile(wrapper, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	count := func() int {
		b, _ := os.ReadFile(runs)
		return strings.Count(string(b), "run")
	}
	ms := &core.MediaSource{
		ID: core.NewID(), Path: path, Size: fi.Size(), Modified: fi.ModTime(),
		Streams: []core.MediaStream{
			{Index: 0, Kind: core.StreamVideo, Codec: "h264"},
			{Index: 1, Kind: core.StreamAudio, Codec: "aac"},
			{Index: 2, Kind: core.StreamAudio, Codec: "ac3"},
			{Index: 3, Kind: core.StreamSubtitle, Codec: "subrip", Language: "eng"},
			{Index: 4, Kind: core.StreamSubtitle, Codec: "ass", Language: "jpn"},
		},
	}
	m := NewManager(Config{FFmpegPath: wrapper, MetadataDir: t.TempDir()})
	item := core.NewID()
	want := map[int]string{3: "Hello from Mavio.", 4: "こんにちは。"}

	// Playbacks asking at once extract the source once, every stream.
	var wg sync.WaitGroup
	for i := range 4 {
		st := &ms.Streams[3+i%2]
		wg.Go(func() {
			data, err := m.extracted(t.Context(), item, ms, st)
			if err != nil || !strings.Contains(string(data), want[st.Index]) {
				t.Errorf("stream %d = %q, %v; want = %q", st.Index, data, err, want[st.Index])
			}
		})
	}
	wg.Wait()
	if n := count(); n != 1 {
		t.Errorf("ffmpeg runs = %d, want = 1", n)
	}
	// ASS keeps its styling.
	if data, err := m.extracted(t.Context(), item, ms, &ms.Streams[4]); err != nil || !strings.Contains(string(data), "[Script Info]") {
		t.Errorf("ASS stream = %q, %v", data, err)
	}
	if _, err := m.extracted(t.Context(), item, ms, &ms.Streams[1]); !errors.Is(err, errNoSubtitle) {
		t.Errorf("audio stream: err = %v, want = %v", err, errNoSubtitle)
	}
	if n := count(); n != 1 {
		t.Errorf("ffmpeg runs = %d, want = 1", n)
	}
	// A changed file is extracted again.
	ms.Modified = ms.Modified.Add(time.Second)
	if _, err := m.extracted(t.Context(), item, ms, &ms.Streams[3]); err != nil {
		t.Fatal(err)
	}
	if n := count(); n != 2 {
		t.Errorf("ffmpeg runs after a change = %d, want = 2", n)
	}
}

// Embedded subtitles of files on cloud storage come from the transcode,
// and are never extracted.
func TestCloudSubtitles(t *testing.T) {
	e := newEnv(t)
	e.m.cfg.FFmpeg = func(context.Context, []string) (supervisor.Process, error) { return nil, errors.New("unused") }
	e.m.cfg.FFmpegPath = "ffmpeg"
	e.m.cfg.Class = func(context.Context, string) core.StorageClass { return core.StorageCloud }
	e.m.cfg.Builder = &decision.Builder{Transcoder: extractor{}}
	web := &decision.ClientCapabilities{
		Name:       "web",
		DirectPlay: []decision.DirectPlayProfile{{Kind: decision.Video, Container: "mp4", VideoCodec: "h264", AudioCodec: "aac"}},
		Transcoding: []decision.TranscodingProfile{{
			Kind: decision.Video, Context: decision.Streaming, Protocol: decision.HLS, Container: "mp4", VideoCodec: "h264", AudioCodec: "aac",
		}},
		Subtitles: []decision.SubtitleProfile{{Format: "vtt", Method: decision.SubtitleExternal}, {Format: "vtt", Method: decision.SubtitleHLS}},
	}
	p := e.start(t, Request{Client: web, SourceID: e.source, SubtitleStream: new(3)})
	if !p.HLS() {
		t.Fatalf("got = %s, want = HLS", p.Method)
	}
	embedded, external := &p.Subtitles[0], &p.Subtitles[1]
	if embedded.Method != decision.SubtitleHLS {
		t.Errorf("embedded subtitles = %s, want = %s", embedded.Method, decision.SubtitleHLS)
	}
	if live := p.liveSubtitles(); len(live) != 1 || live[0] != embedded.Stream {
		t.Errorf("live subtitles = %v, want = the embedded stream", live)
	}
	// The sidecar file is read as it is.
	if external.Method != decision.SubtitleExternal || p.live(external.Stream) {
		t.Errorf("external subtitles = %s, live %v", external.Method, p.live(external.Stream))
	}
	if _, err := e.m.loadSubtitle(t.Context(), p, embedded.Stream); !errors.Is(err, errNoSubtitle) {
		t.Errorf("load embedded: err = %v, want = %v", err, errNoSubtitle)
	}
}

// extractor stands in for an ffmpeg that writes subtitles while
// transcoding.
type extractor struct{ noTranscoder }

func (extractor) CanExtractSubtitles(string) bool { return true }
