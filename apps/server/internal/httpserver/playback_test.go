package httpserver_test

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/mavioai/mavio/apps/server/internal/playback"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/media/keyframes"
	"github.com/mavioai/mavio/libs/media/probe"
	playbackv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/playback/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/playback/v1/playbackv1connect"
)

// TestPlaybackHLS plays a Matroska movie through the API with real
// ffmpeg: remuxed to HLS for a client that takes its codecs but not
// Matroska, and transcoded to a smaller H.264 for one that takes no more
// than 160 pixels wide. Segments are fetched out of order, as seeking
// clients do, and decode with the announced codecs.
func TestPlaybackHLS(t *testing.T) {
	ctx := t.Context()
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("integration test: ffmpeg not found on PATH")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("integration test: ffprobe not found on PATH")
	}
	root := t.TempDir()
	path := filepath.Join(root, "Film (2020)", "Film.mkv")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	gen := exec.CommandContext(ctx, ffmpeg, "-loglevel", "error", "-f", "lavfi", "-i", "testsrc2=size=320x240:rate=24",
		"-f", "lavfi", "-i", "sine=frequency=440", "-t", "30",
		"-c:v", "libx264", "-g", "36", "-keyint_min", "36", "-sc_threshold", "0", "-c:a", "aac", path)
	if out, err := gen.CombinedOutput(); err != nil {
		t.Skipf("integration test: cannot generate the source: %v\n%s", err, out)
	}

	url, s := startServer(t, func(cfg *playback.Config) {
		if _, err := cfg.UseFFmpeg(ctx, ffmpeg, ffprobe); err != nil {
			t.Fatal(err)
		}
		cfg.SegmentLength = 4 * time.Second
	})
	movie := addMovie(t, s, root, path, ffprobe)
	client := playbackv1connect.NewPlaybackServiceClient(http.DefaultClient, url, withToken(signUp(t, url)))

	video := playbackv1.MediaKind_MEDIA_KIND_VIDEO
	hls := func(conditions ...*playbackv1.Condition) *playbackv1.ClientCapabilities {
		return playbackv1.ClientCapabilities_builder{
			Name: new("web"),
			DirectPlay: []*playbackv1.DirectPlayProfile{playbackv1.DirectPlayProfile_builder{
				Kind: &video, Container: new("mp4"), VideoCodec: new("h264"), AudioCodec: new("aac"),
			}.Build()},
			Transcoding: []*playbackv1.TranscodingProfile{playbackv1.TranscodingProfile_builder{
				Kind: &video, Protocol: new(playbackv1.Protocol_PROTOCOL_HLS), Container: new("mp4"),
				VideoCodec: new("h264"), AudioCodec: new("aac"), MaxAudioChannels: new(int32(2)),
			}.Build()},
			Codecs: []*playbackv1.CodecProfile{playbackv1.CodecProfile_builder{
				Kind: new(playbackv1.CodecKind_CODEC_KIND_VIDEO), Codec: new("h264"), Conditions: conditions,
			}.Build()},
		}.Build()
	}
	narrow := playbackv1.Condition_builder{
		Property: new(playbackv1.Property_PROPERTY_WIDTH), Op: new(playbackv1.Op_OP_LESS_THAN_EQUAL), Value: new("160"),
	}.Build()

	tests := []struct {
		name   string
		caps   *playbackv1.ClientCapabilities
		method playbackv1.PlayMethod
		reason playbackv1.TranscodeReason
		width  int
	}{
		{"remux", hls(), playbackv1.PlayMethod_PLAY_METHOD_DIRECT_STREAM, playbackv1.TranscodeReason_TRANSCODE_REASON_CONTAINER_NOT_SUPPORTED, 320},
		{"transcode", hls(narrow), playbackv1.PlayMethod_PLAY_METHOD_TRANSCODE, playbackv1.TranscodeReason_TRANSCODE_REASON_VIDEO_RESOLUTION_NOT_SUPPORTED, 160},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := client.StartPlayback(ctx, playbackv1.StartPlaybackRequest_builder{
				ItemId: new(movie.String()), Capabilities: tt.caps,
			}.Build())
			if err != nil {
				t.Fatalf("StartPlayback: %v", err)
			}
			if resp.GetMethod() != tt.method || !slices.Contains(resp.GetTranscodeReasons(), tt.reason) {
				t.Fatalf("StartPlayback = %v %v, want %v for %v", resp.GetMethod(), resp.GetTranscodeReasons(), tt.method, tt.reason)
			}
			base := url + "/" + strings.TrimSuffix(resp.GetUrl(), "master.m3u8")
			if master := string(get(t, url+"/"+resp.GetUrl())); !strings.Contains(master, `CODECS="avc1.`) || !strings.Contains(master, "\nmain.m3u8\n") {
				t.Errorf("master playlist:\n%s", master)
			}
			media := string(get(t, base+"main.m3u8"))
			segments := strings.Count(media, "#EXTINF:")
			if !strings.Contains(media, `#EXT-X-MAP:URI="init.mp4"`) || segments < 7 {
				t.Fatalf("media playlist:\n%s", media)
			}
			init := get(t, base+"init.mp4")
			for _, i := range []int{0, segments - 1, 2} {
				data := append(bytes.Clone(init), get(t, base+strconv.Itoa(i)+".mp4")...)
				codec, width := videoStream(t, ffprobe, data)
				if codec != "h264" || width != tt.width {
					t.Errorf("segment %d: video %s %dpx wide, want h264 %dpx", i, codec, width, tt.width)
				}
			}

			if _, err := client.StopPlayback(ctx, playbackv1.StopPlaybackRequest_builder{
				PlaybackId: new(resp.GetPlaybackId()), Position: durationpb.New(10 * time.Second),
			}.Build()); err != nil {
				t.Fatalf("StopPlayback: %v", err)
			}
			r, err := http.Get(base + "main.m3u8")
			if err != nil {
				t.Fatal(err)
			}
			_ = r.Body.Close()
			if r.StatusCode != http.StatusNotFound {
				t.Errorf("playlist after stop: %d, want 404", r.StatusCode)
			}
		})
	}
}

// addMovie stores a movie in a new library at root, probed from path.
func addMovie(t *testing.T, s interface {
	Libraries() core.LibraryRepository
	Items() core.ItemRepository
	MediaSources() core.MediaSourceRepository
}, root, path, ffprobe string,
) core.ID {
	t.Helper()
	ctx := t.Context()
	lib := core.Library{Name: "Movies", Kind: core.LibraryMovies, Paths: []string{root}}
	if err := s.Libraries().Create(ctx, &lib); err != nil {
		t.Fatal(err)
	}
	res, err := (&probe.Prober{FFprobe: ffprobe}).Probe(ctx, probe.Request{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	kf, err := (&keyframes.Extractor{FFprobe: ffprobe}).Extract(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	item := core.Item{ID: core.NewID(), LibraryID: lib.ID, Kind: core.KindMovie, Name: "Film", Path: path, Runtime: res.Source.Duration}
	if err := s.Items().Upsert(ctx, item); err != nil {
		t.Fatal(err)
	}
	ms := res.Source
	ms.ID, ms.ItemID, ms.Path, ms.Keyframes = core.NewID(), item.ID, path, kf.Keyframes
	if err := s.MediaSources().Replace(ctx, item.ID, []core.MediaSource{ms}); err != nil {
		t.Fatal(err)
	}
	return item.ID
}

func get(t *testing.T, url string) []byte {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s = %d, %v\n%s", url, resp.StatusCode, err, body)
	}
	return body
}

// videoStream returns the codec and width of the video in an fMP4 file.
func videoStream(t *testing.T, ffprobe string, data []byte) (string, int) {
	t.Helper()
	file := filepath.Join(t.TempDir(), "segment.mp4")
	if err := os.WriteFile(file, data, 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.CommandContext(t.Context(), ffprobe, "-v", "error", "-select_streams", "v:0",
		"-show_entries", "stream=codec_name,width", "-of", "csv=p=0", file).CombinedOutput()
	if err != nil {
		t.Fatalf("ffprobe: %v\n%s", err, out)
	}
	codec, width, _ := strings.Cut(strings.TrimSpace(string(out)), ",")
	w, _ := strconv.Atoi(width)
	return codec, w
}
