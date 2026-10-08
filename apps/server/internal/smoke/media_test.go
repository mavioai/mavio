package smoke_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/media/decision"
	"github.com/mavioai/mavio/libs/media/hwaccel"
	"github.com/mavioai/mavio/libs/media/keyframes"
	"github.com/mavioai/mavio/libs/media/planner"
	"github.com/mavioai/mavio/libs/media/probe"
	"github.com/mavioai/mavio/libs/media/supervisor"
	"github.com/mavioai/mavio/tools/fixtures"
)

// TestPlayHDRMovie runs P3 end to end on an HDR10 HEVC file: it is probed
// and its keyframes extracted; a client that plays HEVC in Matroska plays
// it directly, a web client gets it transcoded to H.264 HLS from a seek
// position, on the hardware available, with ffmpeg supervised until it
// finishes.
func TestPlayHDRMovie(t *testing.T) {
	ctx := t.Context()
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("smoke test: ffmpeg not found on PATH")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("smoke test: ffprobe not found on PATH")
	}
	path := fixtures.Require(t, "hevc_main10_hdr10.mkv")

	// Validate and interrogate the ffmpeg build.
	det := &hwaccel.Detector{FFmpeg: ffmpeg, FFprobe: ffprobe}
	if _, err := det.Validate(ctx); err != nil {
		t.Fatal(err)
	}
	caps, err := det.Detect(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Probe the file and extract its keyframes.
	res, err := (&probe.Prober{FFprobe: ffprobe}).Probe(ctx, probe.Request{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	ms := res.Source
	ms.ID, ms.ItemID, ms.Path = core.NewID(), core.NewID(), path
	kf, err := (&keyframes.Extractor{FFprobe: ffprobe}).Extract(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	ms.Keyframes = kf.Keyframes
	if len(ms.Keyframes) == 0 || ms.Streams[0].VideoRangeType() != core.RangeTypeHDR10 {
		t.Fatalf("probe: got = %d keyframes, range %s", len(ms.Keyframes), ms.Streams[0].VideoRangeType())
	}
	src := &decision.Source{MediaSource: &ms}
	builder := &decision.Builder{Transcoder: caps}
	request := func(c *decision.ClientCapabilities) *decision.Request {
		return &decision.Request{
			Sources: []*decision.Source{src}, SourceID: ms.ID, Client: c, Context: decision.Streaming,
			EnableDirectPlay: true, AllowAudioStreamCopy: true, AllowVideoStreamCopy: true,
		}
	}

	// A player of HEVC HDR in Matroska plays the file as is.
	tv := &decision.ClientCapabilities{
		Name: "tv", MaxStreamingBitrate: 100_000_000,
		DirectPlay: []decision.DirectPlayProfile{{Kind: decision.Video, Container: "mkv", VideoCodec: "hevc,h264", AudioCodec: "ac3,aac"}},
		Codecs: []decision.CodecProfile{{Kind: decision.CodecVideo, Codec: "hevc", Conditions: []decision.Condition{
			{Property: decision.VideoRangeType, Op: decision.EqualsAny, Value: "sdr|hdr10|hlg", Optional: true},
		}}},
	}
	if d, err := builder.Video(request(tv)); err != nil || d.Method != decision.DirectPlay {
		t.Fatalf("tv: got = %+v, %v, want direct play", d, err)
	}

	// A web client gets H.264 and AAC in CMAF HLS.
	web := &decision.ClientCapabilities{
		Name: "web", MaxStreamingBitrate: 8_000_000,
		Transcoding: []decision.TranscodingProfile{{
			Kind: decision.Video, Context: decision.Streaming, Protocol: decision.HLS,
			Container: "mp4", VideoCodec: "h264", AudioCodec: "aac", MaxAudioChannels: 2,
		}},
	}
	d, err := builder.Video(request(web))
	if err != nil {
		t.Fatal(err)
	}
	if d.Method != decision.Transcode || d.Reasons&decision.VideoCodecNotSupported == 0 || d.Protocol != decision.HLS {
		t.Fatalf("web: got = %s %v %s", d.Method, d.Reasons, d.Protocol)
	}

	opts := planner.DefaultOptions()
	if caps.SupportsHwaccel("videotoolbox") {
		opts.Hardware = "videotoolbox"
	}
	p := &planner.Planner{Options: opts, Caps: caps}
	job := p.Job(d, time.Second)
	if job.VideoCodec != "h264" || job.AudioCodec != "aac" || job.AudioChannels != 2 {
		t.Fatalf("job: got = %s %s %d ch", job.VideoCodec, job.AudioCodec, job.AudioChannels)
	}
	dir := t.TempDir()
	args := p.HLSArgs(job, planner.HLSOutput{
		Playlist: filepath.Join(dir, "main.m3u8"), Segments: filepath.Join(dir, "seg%d.mp4"),
	})
	s, err := supervisor.Start(ctx, supervisor.Exec(ffmpeg), supervisor.Config{
		Args: args, IdleTimeout: time.Minute, PauseKey: caps.PauseKey,
	})
	if err != nil {
		t.Fatal(err)
	}
	<-s.Done()
	if err := s.Err(); err != nil {
		t.Fatalf("ffmpeg %s: %v", strings.Join(args, " "), err)
	}
	if prog := s.Progress(); !prog.Ended || prog.Position <= 0 {
		t.Errorf("progress: got = %+v", prog)
	}
	playlist, err := os.ReadFile(filepath.Join(dir, "main.m3u8"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(playlist), "init.mp4") || !strings.Contains(string(playlist), "seg0.mp4") {
		t.Fatalf("playlist: got = %s", playlist)
	}

	// The first segment starts at the seek position.
	out, err := exec.CommandContext(ctx, ffprobe, "-v", "error", "-show_entries", "stream=codec_name:format=start_time",
		"-of", "csv=p=0", filepath.Join(dir, "main.m3u8")).CombinedOutput()
	if err != nil {
		t.Fatalf("ffprobe playlist: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "h264") || !strings.Contains(string(out), "aac") {
		t.Errorf("output streams: got = %s", out)
	}
	lines := strings.Fields(string(out))
	start, err := strconv.ParseFloat(lines[len(lines)-1], 64)
	if err != nil || start < 0.9 || start > 1.2 {
		t.Errorf("start time: got = %s, want about 1s", lines[len(lines)-1])
	}
}
