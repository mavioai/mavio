package server_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/apps/server/internal/server"
	authv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1/authv1connect"
	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1/libraryv1connect"
	playbackv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/playback/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/playback/v1/playbackv1connect"
	systemv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1/systemv1connect"
)

// TestEndToEnd runs the assembled server with real ffmpeg and a metadata
// plugin, through its API only: a library is scanned, its film scraped by
// the plugin once an administrator configured it, and played over HLS.
func TestEndToEnd(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("integration test: ffmpeg not found on PATH")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("integration test: ffprobe not found on PATH")
	}
	ctx := t.Context()
	pluginDir := installPlugin(t)
	media := t.TempDir()
	film := filepath.Join(media, "Farewell My Concubine (1993)", "Farewell My Concubine (1993).mkv")
	if err := os.MkdirAll(filepath.Dir(film), 0o755); err != nil {
		t.Fatal(err)
	}
	gen := exec.CommandContext(ctx, ffmpeg, "-loglevel", "error", "-f", "lavfi", "-i", "testsrc2=size=320x240:rate=24",
		"-f", "lavfi", "-i", "sine=frequency=440", "-t", "12", "-c:v", "libx264", "-g", "48", "-c:a", "aac", film)
	if out, err := gen.CombinedOutput(); err != nil {
		t.Skipf("integration test: cannot generate the film: %v\n%s", err, out)
	}

	// The server, as cmd/mavio runs it.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	runCtx, stop := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- server.Run(runCtx, server.Config{
			Version: "v-test", Database: "sqlite:" + filepath.Join(t.TempDir(), "mavio.db"),
			FFmpeg: ffmpeg, FFprobe: ffprobe, TranscodeDir: t.TempDir(), CacheDir: t.TempDir(),
			PluginDir: pluginDir, Logger: slog.New(slog.DiscardHandler),
		}, ln)
	}()
	t.Cleanup(func() {
		stop()
		if err := <-done; err != nil {
			t.Errorf("Run: %v", err)
		}
	})
	url := "http://" + ln.Addr().String()

	first, err := authv1connect.NewAuthServiceClient(http.DefaultClient, url).CreateFirstUser(ctx, authv1.CreateFirstUserRequest_builder{
		Name: new("admin"), Password: new("secret"),
		Device: authv1.Device_builder{Id: new("e2e"), Name: new("Test"), Client: new("Test"), ClientVersion: new("0")}.Build(),
	}.Build())
	if err != nil {
		t.Fatalf("CreateFirstUser: %v", err)
	}
	token := withToken(first.GetAccessToken())

	// The plugin waits for the API key its schema requires.
	system := systemv1connect.NewSystemServiceClient(http.DefaultClient, url, token)
	plugins, err := system.ListPlugins(ctx, &systemv1.ListPluginsRequest{})
	if err != nil || len(plugins.GetPlugins()) != 1 || plugins.GetPlugins()[0].GetState() != systemv1.PluginState_PLUGIN_STATE_UNCONFIGURED {
		t.Fatalf("ListPlugins = %v, %v; want the plugin unconfigured", plugins, err)
	}
	if _, err := system.SetPluginConfig(ctx, systemv1.SetPluginConfigRequest_builder{
		PluginId: new("org.mavio.smoke"), ConfigJson: new(`{}`),
	}.Build()); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("SetPluginConfig without the key: %v, want invalid_argument", err)
	}
	set, err := system.SetPluginConfig(ctx, systemv1.SetPluginConfigRequest_builder{
		PluginId: new("org.mavio.smoke"), ConfigJson: new(`{"api_key":"secret"}`),
	}.Build())
	if err != nil || set.GetPlugin().GetState() != systemv1.PluginState_PLUGIN_STATE_READY {
		t.Fatalf("SetPluginConfig = %v, %v; want ready", set, err)
	}

	// Scan → scrape.
	libraries := libraryv1connect.NewLibraryServiceClient(http.DefaultClient, url, token)
	if _, err := libraries.CreateLibrary(ctx, libraryv1.CreateLibraryRequest_builder{
		Spec: libraryv1.LibrarySpec_builder{Name: new("Films"), Kind: new(libraryv1.LibraryKind_LIBRARY_KIND_MOVIES), Paths: []string{media}}.Build(),
	}.Build()); err != nil {
		t.Fatalf("CreateLibrary: %v", err)
	}
	items := libraryv1connect.NewItemServiceClient(http.DefaultClient, url, token)
	var movie *libraryv1.Item
	for deadline := time.Now().Add(time.Minute); movie == nil; time.Sleep(100 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the film was not scraped within a minute")
		}
		list, err := items.ListItems(ctx, libraryv1.ListItemsRequest_builder{
			Kinds: []libraryv1.ItemKind{libraryv1.ItemKind_ITEM_KIND_MOVIE},
		}.Build())
		if err != nil {
			t.Fatalf("ListItems: %v", err)
		}
		for _, it := range list.GetItems() {
			if it.GetOriginalTitle() == "霸王别姬" {
				movie = it
			}
		}
	}
	if movie.GetName() != "Farewell My Concubine" || !slices.Contains(movie.GetGenres(), "Drama") {
		t.Errorf("scraped film: name %q, genres %v", movie.GetName(), movie.GetGenres())
	}

	// Playback decision → HLS playback.
	playback := playbackv1connect.NewPlaybackServiceClient(http.DefaultClient, url, token)
	video := playbackv1.MediaKind_MEDIA_KIND_VIDEO
	resp, err := playback.StartPlayback(ctx, playbackv1.StartPlaybackRequest_builder{
		ItemId: new(movie.GetId()),
		Capabilities: playbackv1.ClientCapabilities_builder{
			Name: new("e2e"),
			DirectPlay: []*playbackv1.DirectPlayProfile{playbackv1.DirectPlayProfile_builder{
				Kind: &video, Container: new("mp4"), VideoCodec: new("h264"), AudioCodec: new("aac"),
			}.Build()},
			Transcoding: []*playbackv1.TranscodingProfile{playbackv1.TranscodingProfile_builder{
				Kind: &video, Protocol: new(playbackv1.Protocol_PROTOCOL_HLS), Container: new("mp4"),
				VideoCodec: new("h264"), AudioCodec: new("aac"), MaxAudioChannels: new(int32(2)),
			}.Build()},
		}.Build(),
	}.Build())
	if err != nil {
		t.Fatalf("StartPlayback: %v", err)
	}
	if resp.GetMethod() != playbackv1.PlayMethod_PLAY_METHOD_DIRECT_STREAM || !strings.HasSuffix(resp.GetUrl(), "master.m3u8") {
		t.Fatalf("StartPlayback = %v %s, want an HLS direct stream", resp.GetMethod(), resp.GetUrl())
	}
	base := url + "/" + strings.TrimSuffix(resp.GetUrl(), "master.m3u8")
	if master := string(get(t, base+"master.m3u8")); !strings.Contains(master, `CODECS="avc1.`) {
		t.Errorf("master playlist:\n%s", master)
	}
	media0 := string(get(t, base+"main.m3u8"))
	if !strings.Contains(media0, "#EXT-X-ENDLIST") || strings.Count(media0, "#EXTINF:") < 2 {
		t.Fatalf("media playlist:\n%s", media0)
	}
	segment := append(get(t, base+"init.mp4"), get(t, base+"0.mp4")...)
	file := filepath.Join(t.TempDir(), "segment.mp4")
	if err := os.WriteFile(file, segment, 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.CommandContext(ctx, ffprobe, "-v", "error", "-show_entries", "stream=codec_name",
		"-of", "csv=p=0", file).CombinedOutput()
	if err != nil || !bytes.Contains(out, []byte("h264")) || !bytes.Contains(out, []byte("aac")) {
		t.Errorf("first segment: %v\n%s", err, out)
	}
	if _, err := playback.StopPlayback(ctx, playbackv1.StopPlaybackRequest_builder{PlaybackId: new(resp.GetPlaybackId())}.Build()); err != nil {
		t.Errorf("StopPlayback: %v", err)
	}
}

// installPlugin builds the smoke tests' metadata provider for WASM into a
// plugin folder, with a schema that requires an API key as TMDB's does.
func installPlugin(t *testing.T, capabilities ...string) string {
	t.Helper()
	if len(capabilities) == 0 {
		capabilities = []string{"CAPABILITY_METADATA_PROVIDER"}
	}
	root := t.TempDir()
	dir := filepath.Join(root, "smoke")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	build := exec.CommandContext(t.Context(), "go", "build", "-buildmode=c-shared", "-o", filepath.Join(dir, "plugin.wasm"),
		"github.com/mavioai/mavio/apps/server/internal/smoke/smokeplugin")
	build.Env = append(os.Environ(), "GOOS=wasip1", "GOARCH=wasm")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build plugin: %v\n%s", err, out)
	}
	schema := `{"type":"object","properties":{"api_key":{"type":"string","minLength":1}},"required":["api_key"]}`
	manifest := fmt.Sprintf(`{"id":"org.mavio.smoke","name":"Smoke","version":"0.1.0","runtime":"RUNTIME_WASM",
		"capabilities":["%s"],"configSchema":%q,"apiVersion":"1.0"}`, strings.Join(capabilities, `","`), schema)
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// withToken sends token as the bearer token of every request, unary or
// streaming.
func withToken(token string) connect.ClientOption {
	return connect.WithInterceptors(bearer(token))
}

type bearer string

func (b bearer) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		req.Header().Set("Authorization", "Bearer "+string(b))
		return next(ctx, req)
	}
}

func (b bearer) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return func(ctx context.Context, spec connect.Spec) connect.StreamingClientConn {
		conn := next(ctx, spec)
		conn.RequestHeader().Set("Authorization", "Bearer "+string(b))
		return conn
	}
}

func (b bearer) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return next
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
