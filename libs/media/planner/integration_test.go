package planner

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/media/decision"
	"github.com/mavioai/mavio/libs/media/hwaccel"
	"github.com/mavioai/mavio/libs/media/probe"
	"github.com/mavioai/mavio/tools/fixtures"
)

// hlsClient plays H.264 (and optionally HEVC) with AAC from fMP4 HLS and
// nothing directly, so every source is remuxed or transcoded.
func hlsClient(hevc bool) *decision.ClientCapabilities {
	video := decision.Names("h264")
	var codecs []decision.CodecProfile
	if hevc {
		video = "hevc,h264"
	} else {
		// Width is applied to the whole transcode, so only the H.264-only
		// client limits it.
		codecs = []decision.CodecProfile{{
			Kind: decision.CodecVideo, Codec: "h264",
			Conditions: []decision.Condition{{Property: decision.Width, Op: decision.LessThanEqual, Value: "640"}},
		}}
	}
	return &decision.ClientCapabilities{
		Name:                "test",
		MaxStreamingBitrate: 20_000_000,
		Transcoding: []decision.TranscodingProfile{{
			Kind: decision.Video, Context: decision.Streaming, Protocol: decision.HLS,
			Container: "mp4", VideoCodec: video, AudioCodec: "aac", MaxAudioChannels: 2,
		}},
		Codecs: codecs,
	}
}

// TestHLSTranscodes runs real transcodes of the fixtures and checks that
// the segments decode.
func TestHLSTranscodes(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("integration test: ffmpeg not found on PATH")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("integration test: ffprobe not found on PATH")
	}
	caps, err := (&hwaccel.Detector{FFmpeg: ffmpeg, FFprobe: ffprobe}).Detect(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	hardware := []string{""}
	if runtime.GOOS == "darwin" && caps.SupportsHwaccel("videotoolbox") {
		hardware = append(hardware, "videotoolbox")
	}
	prober := &probe.Prober{FFprobe: ffprobe}
	tests := []struct {
		fixture string
		hevc    bool
		copy    bool // the video is copied
	}{
		{"h264_aac.mp4", false, true}, // already 640 wide
		{"hevc_main10_hdr10.mkv", false, false},
		{"hevc_main10_hdr10.mkv", true, true},
		{"mpeg2_interlaced.ts", false, false},
		{"vp9_opus.webm", false, false},
	}
	for _, hw := range hardware {
		for _, tt := range tests {
			name := tt.fixture
			if tt.hevc {
				name += "/hevc-client"
			}
			if hw != "" {
				name += "/" + hw
			}
			t.Run(name, func(t *testing.T) {
				path := fixtures.Require(t, tt.fixture)
				res, err := prober.Probe(t.Context(), probe.Request{Path: path})
				if err != nil {
					t.Fatal(err)
				}
				ms := res.Source
				ms.ID, ms.Path = core.NewID(), path
				src := &decision.Source{MediaSource: &ms}
				d, err := (&decision.Builder{}).Video(&decision.Request{
					Sources: []*decision.Source{src}, Client: hlsClient(tt.hevc), Context: decision.Streaming,
					EnableDirectPlay: true, AllowAudioStreamCopy: true, AllowVideoStreamCopy: true,
				})
				if err != nil {
					t.Fatal(err)
				}
				opts := DefaultOptions()
				opts.Hardware = hw
				p := &Planner{Options: opts, Caps: caps}
				j := p.Job(d, 0)
				if got := j.VideoCodec == Copy; got != tt.copy {
					t.Errorf("video copied: got = %v, want = %v (%s)", got, tt.copy, j.VideoCodec)
				}
				dir := t.TempDir()
				args := p.HLSArgs(j, HLSOutput{Playlist: filepath.Join(dir, "main.m3u8"), Segments: filepath.Join(dir, "seg%d.mp4")})
				if out, err := exec.CommandContext(t.Context(), ffmpeg, args...).CombinedOutput(); err != nil {
					t.Fatalf("ffmpeg %s: %v\n%s", strings.Join(args, " "), err, out)
				}
				playlist, err := os.ReadFile(filepath.Join(dir, "main.m3u8"))
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(string(playlist), "#EXT-X-MAP:URI=\"init.mp4\"") || !strings.Contains(string(playlist), "seg0.mp4") {
					t.Fatalf("playlist: got = %s", playlist)
				}
				// The init segment and first media segment must decode.
				joinedSeg := filepath.Join(dir, "joined.mp4")
				init, _ := os.ReadFile(filepath.Join(dir, "init.mp4"))
				seg, _ := os.ReadFile(filepath.Join(dir, "seg0.mp4"))
				if err := os.WriteFile(joinedSeg, append(init, seg...), 0o644); err != nil {
					t.Fatal(err)
				}
				out, err := exec.CommandContext(t.Context(), ffprobe, "-v", "error", "-show_entries", "stream=codec_name,width",
					"-of", "csv=p=0", joinedSeg).CombinedOutput()
				if err != nil {
					t.Fatalf("ffprobe segment: %v\n%s", err, out)
				}
				hasVideo := strings.Contains(string(out), "h264") || strings.Contains(string(out), "hevc")
				if !strings.Contains(string(out), "aac") || !hasVideo {
					t.Errorf("segment streams: got = %s", out)
				}
				if !tt.copy {
					for _, w := range []string{",1280", ",1920"} {
						if strings.Contains(string(out), w) {
							t.Errorf("width limit ignored: got = %s", out)
						}
					}
				}
			})
		}
	}
}
