package server_test

import (
	"bytes"
	"image/jpeg"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

// run runs a command, skipping the test when it fails.
func run(t *testing.T, name string, args ...string) {
	t.Helper()
	if out, err := exec.CommandContext(t.Context(), name, args...).CombinedOutput(); err != nil {
		t.Skipf("integration test: %s failed: %v\n%s", name, err, out)
	}
}

// TestMediaExtras runs the assembled server with real ffmpeg: a film with
// chapters and an attached font gets trickplay sheets, chapter images and
// segments from a plugin, and downloads as a progressive transcode that
// plays; a track gets its loudness measured and its lyrics read.
func TestMediaExtras(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("integration test: ffmpeg not found on PATH")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("integration test: ffprobe not found on PATH")
	}
	ctx := t.Context()
	films, music := t.TempDir(), t.TempDir()
	work := t.TempDir()
	chapters := filepath.Join(work, "chapters.txt")
	font := filepath.Join(work, "Sans.ttf")
	if err := os.WriteFile(chapters, []byte(";FFMETADATA1\n[CHAPTER]\nTIMEBASE=1/1000\nSTART=0\nEND=12000\ntitle=One\n"+
		"[CHAPTER]\nTIMEBASE=1/1000\nSTART=12000\nEND=24000\ntitle=Two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(font, []byte("not really a font"), 0o644); err != nil {
		t.Fatal(err)
	}
	film := filepath.Join(films, "Up (2009)", "Up (2009).mkv")
	_ = os.MkdirAll(filepath.Dir(film), 0o755)
	run(t, ffmpeg, "-v", "error", "-f", "lavfi", "-i", "testsrc2=size=320x180:rate=24", "-f", "lavfi", "-i", "sine=frequency=440",
		"-i", chapters, "-map", "0:v", "-map", "1:a", "-map_metadata", "2", "-map_chapters", "2", "-t", "24",
		"-vf", "pad=320:240:0:30:black", "-c:v", "libx264", "-g", "48", "-c:a", "aac",
		"-attach", font, "-metadata:s:t", "mimetype=font/ttf", film)
	track := filepath.Join(music, "Band", "Album", "01 - Song.flac")
	_ = os.MkdirAll(filepath.Dir(track), 0o755)
	run(t, ffmpeg, "-v", "error", "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=44100", "-t", "5", track)
	if err := os.WriteFile(strings.TrimSuffix(track, ".flac")+".lrc", []byte("[ti:Song]\n[00:01.00]Hello\n[00:03.00]World\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	srv := start(t, server.Config{
		Database: "sqlite:" + filepath.Join(t.TempDir(), "mavio.db"), FFmpeg: ffmpeg, FFprobe: ffprobe,
		TranscodeDir: t.TempDir(), CacheDir: t.TempDir(), MetadataDir: t.TempDir(),
		PluginDir: installPlugin(t, "CAPABILITY_METADATA_PROVIDER", "CAPABILITY_SEGMENT_PROVIDER"),
	})
	first, err := authv1connect.NewAuthServiceClient(http.DefaultClient, srv.url).CreateFirstUser(ctx, authv1.CreateFirstUserRequest_builder{
		Name: new("admin"), Password: new("secret"),
		Device: authv1.Device_builder{Id: new("e2e"), Name: new("Test"), Client: new("Test"), ClientVersion: new("0")}.Build(),
	}.Build())
	if err != nil {
		t.Fatalf("CreateFirstUser: %v", err)
	}
	token := withToken(first.GetAccessToken())
	if _, err := systemv1connect.NewSystemServiceClient(http.DefaultClient, srv.url, token).SetPluginConfig(ctx, systemv1.SetPluginConfigRequest_builder{
		PluginId: new("org.mavio.smoke"), ConfigJson: new(`{"api_key":"secret"}`),
	}.Build()); err != nil {
		t.Fatalf("SetPluginConfig: %v", err)
	}
	libraries := libraryv1connect.NewLibraryServiceClient(http.DefaultClient, srv.url, token)
	for _, spec := range []*libraryv1.LibrarySpec{
		libraryv1.LibrarySpec_builder{
			Name: new("Films"), Kind: new(libraryv1.LibraryKind_LIBRARY_KIND_MOVIES), Paths: []string{films},
			ExtractTrickplay: new(true), ExtractChapterImages: new(true),
		}.Build(),
		libraryv1.LibrarySpec_builder{
			Name: new("Music"), Kind: new(libraryv1.LibraryKind_LIBRARY_KIND_MUSIC), Paths: []string{music}, AnalyzeLoudness: new(true),
		}.Build(),
	} {
		if _, err := libraries.CreateLibrary(ctx, libraryv1.CreateLibraryRequest_builder{Spec: spec}.Build()); err != nil {
			t.Fatalf("CreateLibrary: %v", err)
		}
	}
	items := libraryv1connect.NewItemServiceClient(http.DefaultClient, srv.url, token)
	find := func(kind libraryv1.ItemKind) string {
		t.Helper()
		for deadline := time.Now().Add(time.Minute); ; time.Sleep(100 * time.Millisecond) {
			list, err := items.ListItems(ctx, libraryv1.ListItemsRequest_builder{Kinds: []libraryv1.ItemKind{kind}}.Build())
			if err != nil {
				t.Fatal(err)
			}
			if len(list.GetItems()) == 1 {
				return list.GetItems()[0].GetId()
			}
			if time.Now().After(deadline) {
				t.Fatalf("no %v within a minute", kind)
			}
		}
	}
	waitItem := func(id, what string, ok func(*libraryv1.GetItemResponse) bool) *libraryv1.GetItemResponse {
		t.Helper()
		for deadline := time.Now().Add(2 * time.Minute); ; time.Sleep(200 * time.Millisecond) {
			resp, err := items.GetItem(ctx, libraryv1.GetItemRequest_builder{Id: &id}.Build())
			if err != nil {
				t.Fatal(err)
			}
			if ok(resp) {
				return resp
			}
			if time.Now().After(deadline) {
				t.Fatalf("%s: not within two minutes: %v", what, resp)
			}
		}
	}

	// The film's extras.
	movie := find(libraryv1.ItemKind_ITEM_KIND_MOVIE)
	resp := waitItem(movie, "trickplay, chapter images and segments", func(r *libraryv1.GetItemResponse) bool {
		srcs := r.GetMediaSources()
		return len(r.GetTrickplay()) == 1 && len(r.GetSegments()) == 2 && len(srcs) == 1 && len(srcs[0].GetChapters()) == 2 &&
			srcs[0].GetChapters()[0].GetHasImage() && srcs[0].GetChapters()[1].GetHasImage()
	})
	tp := resp.GetTrickplay()[0]
	// 24 seconds every 10: three thumbnails; the borders are cropped away.
	if tp.GetWidth() != 320 || tp.GetHeight() != 180 || tp.GetThumbnailCount() != 3 || tp.GetSheetCount() != 1 {
		t.Errorf("trickplay = %v", tp)
	}
	sheet := get(t, srv.url+"/images/trickplay/"+movie+"/320/0.jpg")
	if cfg, err := jpeg.DecodeConfig(bytes.NewReader(sheet)); err != nil || cfg.Width != 3200 || cfg.Height != 1800 {
		t.Errorf("sheet = %+v, %v", cfg, err)
	}
	source := resp.GetMediaSources()[0].GetId()
	chapter := get(t, srv.url+"/images/chapters/"+movie+"/"+source+"/1?maxWidth=160&format=jpg")
	if cfg, err := jpeg.DecodeConfig(bytes.NewReader(chapter)); err != nil || cfg.Width != 160 {
		t.Errorf("chapter image = %+v, %v", cfg, err)
	}
	if s := resp.GetSegments(); s[0].GetKind() != libraryv1.SegmentKind_SEGMENT_KIND_INTRO || s[1].GetEnd().AsDuration() < 23*time.Second {
		t.Errorf("segments = %v", s)
	}

	// A download that has to be transcoded comes as one progressive file
	// that plays, and the font comes with it.
	playback := playbackv1connect.NewPlaybackServiceClient(http.DefaultClient, srv.url, token)
	video := playbackv1.MediaKind_MEDIA_KIND_VIDEO
	dl, err := playback.StartPlayback(ctx, playbackv1.StartPlaybackRequest_builder{
		ItemId: &movie, Download: new(true),
		Capabilities: playbackv1.ClientCapabilities_builder{
			Name: new("e2e"),
			Transcoding: []*playbackv1.TranscodingProfile{playbackv1.TranscodingProfile_builder{
				Kind: &video, Protocol: new(playbackv1.Protocol_PROTOCOL_HTTP), Container: new("mp4"), Context: new(playbackv1.Context_CONTEXT_STATIC),
				VideoCodec: new("h264"), AudioCodec: new("aac"), MaxAudioChannels: new(int32(2)),
			}.Build()},
		}.Build(),
	}.Build())
	if err != nil {
		t.Fatalf("StartPlayback download: %v", err)
	}
	if !strings.HasSuffix(dl.GetUrl(), "/stream.mp4") || dl.GetMethod() == playbackv1.PlayMethod_PLAY_METHOD_DIRECT_PLAY {
		t.Fatalf("download = %v", dl)
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.url+"/"+dl.GetUrl(), nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK || !strings.HasPrefix(res.Header.Get("Content-Disposition"), "attachment") || len(body) == 0 {
		t.Fatalf("download = %d %q, %d bytes", res.StatusCode, res.Header.Get("Content-Disposition"), len(body))
	}
	kept := filepath.Join(t.TempDir(), "Up.mp4")
	if err := os.WriteFile(kept, body, 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.CommandContext(ctx, ffprobe, "-v", "error", "-show_entries", "stream=codec_name:format=duration", "-of", "csv=p=0", kept).CombinedOutput()
	if err != nil || !bytes.Contains(out, []byte("h264")) || !bytes.Contains(out, []byte("aac")) {
		t.Errorf("downloaded file: %v\n%s", err, out)
	}
	// It plays: every frame decodes.
	if out, err := exec.CommandContext(ctx, ffmpeg, "-v", "error", "-xerror", "-i", kept, "-f", "null", "-").CombinedOutput(); err != nil {
		t.Errorf("playing the download: %v\n%s", err, out)
	}
	if len(dl.GetAttachments()) != 1 || dl.GetAttachments()[0].GetFileName() != "Sans.ttf" || dl.GetAttachments()[0].GetMimeType() != "font/ttf" {
		t.Fatalf("attachments = %v", dl.GetAttachments())
	}
	if got := get(t, srv.url+"/"+dl.GetAttachments()[0].GetUrl()); string(got) != "not really a font" {
		t.Errorf("font = %q", got)
	}

	// The track's loudness and lyrics.
	song := find(libraryv1.ItemKind_ITEM_KIND_TRACK)
	tr := waitItem(song, "loudness", func(r *libraryv1.GetItemResponse) bool { return r.GetItem().HasNormalizationGain() })
	if g := tr.GetItem().GetNormalizationGain(); g < -20 || g > 20 {
		t.Errorf("normalization gain = %v", g)
	}
	lyrics, err := items.GetLyrics(ctx, libraryv1.GetLyricsRequest_builder{ItemId: &song}.Build())
	if err != nil || !lyrics.GetSynced() || lyrics.GetTitle() != "Song" || len(lyrics.GetLines()) != 2 ||
		lyrics.GetLines()[1].GetStart().AsDuration() != 3*time.Second {
		t.Errorf("GetLyrics = %v, %v", lyrics, err)
	}
}
