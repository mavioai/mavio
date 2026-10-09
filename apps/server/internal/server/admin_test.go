package server_test

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

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

// smokePackage builds a version of the smoke plugin into a zip package.
func smokePackage(t *testing.T, version string) []byte {
	t.Helper()
	dir := t.TempDir()
	build := exec.CommandContext(t.Context(), "go", "build", "-buildmode=c-shared", "-ldflags", "-X main.Version="+version,
		"-o", filepath.Join(dir, "plugin.wasm"), "github.com/mavioai/mavio/apps/server/internal/smoke/smokeplugin")
	build.Env = append(os.Environ(), "GOOS=wasip1", "GOARCH=wasm")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build plugin: %v\n%s", err, out)
	}
	wasm, err := os.ReadFile(filepath.Join(dir, "plugin.wasm"))
	if err != nil {
		t.Fatal(err)
	}
	schema := `{"type":"object","properties":{"api_key":{"type":"string","minLength":1}},"required":["api_key"]}`
	manifest := fmt.Sprintf(`{"id":"org.mavio.smoke","name":"Smoke","version":%q,"runtime":"RUNTIME_WASM",
		"capabilities":["CAPABILITY_METADATA_PROVIDER"],"configSchema":%q,"apiVersion":"1.0"}`, version, schema)
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, data := range map[string][]byte{"manifest.json": []byte(manifest), "plugin.wasm": wasm} {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write(data)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// catalog serves a plugin catalog offering two versions of the smoke
// plugin.
func catalog(t *testing.T) string {
	t.Helper()
	packages := map[string][]byte{"smoke-0.1.0.zip": smokePackage(t, "0.1.0"), "smoke-0.2.0.zip": smokePackage(t, "0.2.0")}
	var versions []map[string]string
	for _, v := range []string{"0.1.0", "0.2.0"} {
		sum := sha256.Sum256(packages["smoke-"+v+".zip"])
		versions = append(versions, map[string]string{
			"version": v, "api_version": "1.0", "runtime": "RUNTIME_WASM", "url": "smoke-" + v + ".zip", "sha256": hex.EncodeToString(sum[:]),
		})
	}
	index, _ := json.Marshal(map[string]any{"plugins": []any{map[string]any{"id": "org.mavio.smoke", "name": "Smoke", "versions": versions}}})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/catalog.json" {
			_, _ = w.Write(index)
			return
		}
		data, ok := packages[strings.TrimPrefix(r.URL.Path, "/")]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(data)
	}))
	t.Cleanup(srv.Close)
	return srv.URL + "/catalog.json"
}

// running is a server started for a test.
type running struct {
	url  string
	stop func()
}

func start(t *testing.T, cfg server.Config) running {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	cfg.Version, cfg.Logger = "v-test", slog.New(slog.DiscardHandler)
	runCtx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Run(runCtx, cfg, ln) }()
	stopped := false
	stop := func() {
		if stopped {
			return
		}
		stopped = true
		cancel()
		if err := <-done; err != nil {
			t.Errorf("Run: %v", err)
		}
	}
	t.Cleanup(stop)
	return running{url: "http://" + ln.Addr().String(), stop: stop}
}

// TestAdministration runs the assembled server through its API: an
// administrator changes the transcode folder and the next transcode
// writes there; a plugin is installed from a catalog, upgraded and
// uninstalled while the server serves; a backup is restored into a new
// server.
func TestAdministration(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("integration test: ffmpeg not found on PATH")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("integration test: ffprobe not found on PATH")
	}
	ctx := t.Context()
	media := t.TempDir()
	film := filepath.Join(media, "Up (2009)", "Up (2009).mkv")
	if err := os.MkdirAll(filepath.Dir(film), 0o755); err != nil {
		t.Fatal(err)
	}
	gen := exec.CommandContext(ctx, ffmpeg, "-loglevel", "error", "-f", "lavfi", "-i", "testsrc2=size=320x240:rate=24",
		"-f", "lavfi", "-i", "sine=frequency=440", "-t", "6", "-c:v", "libx264", "-g", "24", "-c:a", "aac", film)
	if out, err := gen.CombinedOutput(); err != nil {
		t.Skipf("integration test: cannot generate the film: %v\n%s", err, out)
	}
	catalogURL := catalog(t)
	pluginDir, backupDir := t.TempDir(), t.TempDir()
	srv := start(t, server.Config{
		Database: "sqlite:" + filepath.Join(t.TempDir(), "mavio.db"), FFmpeg: ffmpeg, FFprobe: ffprobe,
		TranscodeDir: t.TempDir(), CacheDir: t.TempDir(), MetadataDir: t.TempDir(), PluginDir: pluginDir, BackupDir: backupDir,
	})
	first, err := authv1connect.NewAuthServiceClient(http.DefaultClient, srv.url).CreateFirstUser(ctx, authv1.CreateFirstUserRequest_builder{
		Name: new("admin"), Password: new("secret"),
		Device: authv1.Device_builder{Id: new("e2e"), Name: new("Test"), Client: new("Test"), ClientVersion: new("0")}.Build(),
	}.Build())
	if err != nil {
		t.Fatalf("CreateFirstUser: %v", err)
	}
	token := withToken(first.GetAccessToken())
	system := systemv1connect.NewSystemServiceClient(http.DefaultClient, srv.url, token)

	// Settings: a plugin catalog and another transcode folder.
	got, err := system.GetServerSettings(ctx, &systemv1.GetServerSettingsRequest{})
	if err != nil {
		t.Fatalf("GetServerSettings: %v", err)
	}
	set := got.GetSettings()
	transcodes := t.TempDir()
	set.SetPluginCatalogs([]string{catalogURL})
	set.GetTranscoding().SetTranscodeDir(transcodes)
	set.GetTranscoding().SetEncoderPreset("ultrafast")
	if _, err := system.UpdateServerSettings(ctx, systemv1.UpdateServerSettingsRequest_builder{Settings: set}.Build()); err != nil {
		t.Fatalf("UpdateServerSettings: %v", err)
	}
	bad := proto.Clone(set).(*systemv1.ServerSettings)
	bad.GetTranscoding().SetTranscodeDir("relative")
	if _, err := system.UpdateServerSettings(ctx, systemv1.UpdateServerSettingsRequest_builder{Settings: bad}.Build()); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("relative transcode folder: %v, want invalid_argument", err)
	}

	// Plugins: install an old version, configure it, upgrade, uninstall.
	plugin := func() *systemv1.Plugin {
		t.Helper()
		list, err := system.ListPlugins(ctx, &systemv1.ListPluginsRequest{})
		if err != nil {
			t.Fatalf("ListPlugins: %v", err)
		}
		if len(list.GetPlugins()) == 0 {
			return nil
		}
		return list.GetPlugins()[0]
	}
	if _, err := system.InstallPlugin(ctx, systemv1.InstallPluginRequest_builder{PluginId: new("org.mavio.smoke"), Version: new("0.1.0")}.Build()); err != nil {
		t.Fatalf("InstallPlugin 0.1.0: %v", err)
	}
	if p := plugin(); p.GetManifest().GetVersion() != "0.1.0" || p.GetState() != systemv1.PluginState_PLUGIN_STATE_UNCONFIGURED {
		t.Fatalf("installed plugin: %v", p)
	}
	if _, err := system.SetPluginConfig(ctx, systemv1.SetPluginConfigRequest_builder{
		PluginId: new("org.mavio.smoke"), ConfigJson: new(`{"api_key":"secret"}`),
	}.Build()); err != nil {
		t.Fatalf("SetPluginConfig: %v", err)
	}
	installed, err := system.InstallPlugin(ctx, systemv1.InstallPluginRequest_builder{PluginId: new("org.mavio.smoke")}.Build())
	if err != nil {
		t.Fatalf("upgrade: %v", err)
	}
	if p := installed.GetPlugin(); p.GetManifest().GetVersion() != "0.2.0" || p.GetState() != systemv1.PluginState_PLUGIN_STATE_READY {
		t.Errorf("upgraded plugin: %v", p)
	}
	cat, err := system.ListCatalogPlugins(ctx, &systemv1.ListCatalogPluginsRequest{})
	if err != nil || len(cat.GetPlugins()) != 1 || cat.GetPlugins()[0].GetInstalledVersion() != "0.2.0" ||
		cat.GetPlugins()[0].GetVersions()[0].GetVersion() != "0.2.0" {
		t.Errorf("ListCatalogPlugins = %v, %v", cat, err)
	}
	if _, err := system.GetHealth(ctx, &systemv1.GetHealthRequest{}); err != nil {
		t.Errorf("GetHealth while plugins change: %v", err)
	}

	// The film is scraped by the upgraded plugin's configuration, then
	// transcoded into the new folder.
	libraries := libraryv1connect.NewLibraryServiceClient(http.DefaultClient, srv.url, token)
	if _, err := libraries.CreateLibrary(ctx, libraryv1.CreateLibraryRequest_builder{
		Spec: libraryv1.LibrarySpec_builder{Name: new("Films"), Kind: new(libraryv1.LibraryKind_LIBRARY_KIND_MOVIES), Paths: []string{media}}.Build(),
	}.Build()); err != nil {
		t.Fatalf("CreateLibrary: %v", err)
	}
	items := libraryv1connect.NewItemServiceClient(http.DefaultClient, srv.url, token)
	var movie *libraryv1.Item
	for deadline := time.Now().Add(time.Minute); movie == nil; time.Sleep(100 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the film was not scanned within a minute")
		}
		list, err := items.ListItems(ctx, libraryv1.ListItemsRequest_builder{Kinds: []libraryv1.ItemKind{libraryv1.ItemKind_ITEM_KIND_MOVIE}}.Build())
		if err != nil {
			t.Fatal(err)
		}
		if len(list.GetItems()) == 1 && list.GetItems()[0].GetDateAdded() != nil {
			movie = list.GetItems()[0]
		}
	}
	playback := playbackv1connect.NewPlaybackServiceClient(http.DefaultClient, srv.url, token)
	video := playbackv1.MediaKind_MEDIA_KIND_VIDEO
	var resp *playbackv1.StartPlaybackResponse
	for deadline := time.Now().Add(time.Minute); ; time.Sleep(200 * time.Millisecond) {
		resp, err = playback.StartPlayback(ctx, playbackv1.StartPlaybackRequest_builder{
			ItemId: new(movie.GetId()), MaxBitrate: new(int64(64_000)),
			Capabilities: playbackv1.ClientCapabilities_builder{
				Name: new("e2e"),
				Transcoding: []*playbackv1.TranscodingProfile{playbackv1.TranscodingProfile_builder{
					Kind: &video, Protocol: new(playbackv1.Protocol_PROTOCOL_HLS), Container: new("mp4"),
					VideoCodec: new("h264"), AudioCodec: new("aac"), MaxAudioChannels: new(int32(2)),
				}.Build()},
			}.Build(),
		}.Build())
		if err == nil || time.Now().After(deadline) {
			break
		}
	}
	if err != nil {
		t.Fatalf("StartPlayback: %v", err)
	}
	if resp.GetMethod() != playbackv1.PlayMethod_PLAY_METHOD_TRANSCODE {
		t.Fatalf("method = %v, want a transcode", resp.GetMethod())
	}
	base := srv.url + "/" + strings.TrimSuffix(resp.GetUrl(), "master.m3u8")
	_ = get(t, base+"main.m3u8")
	_ = get(t, base+"0.mp4")
	if entries, _ := os.ReadDir(transcodes); len(entries) != 1 || entries[0].Name() != resp.GetPlaybackId() {
		t.Errorf("transcode folder holds %v, want the playback's", entries)
	}
	if _, err := playback.StopPlayback(ctx, playbackv1.StopPlaybackRequest_builder{PlaybackId: new(resp.GetPlaybackId())}.Build()); err != nil {
		t.Errorf("StopPlayback: %v", err)
	}

	if _, err := system.UninstallPlugin(ctx, systemv1.UninstallPluginRequest_builder{PluginId: new("org.mavio.smoke")}.Build()); err != nil {
		t.Fatalf("UninstallPlugin: %v", err)
	}
	if p := plugin(); p != nil {
		t.Errorf("plugin after uninstalling: %v", p)
	}
	if entries, _ := os.ReadDir(pluginDir); len(entries) != 0 {
		t.Errorf("plugin folder holds %v after uninstalling", entries)
	}

	// The activity log recorded the changes.
	activities, err := systemv1connect.NewActivityServiceClient(http.DefaultClient, srv.url, token).ListActivities(ctx, &systemv1.ListActivitiesRequest{})
	if err != nil {
		t.Fatalf("ListActivities: %v", err)
	}
	types := map[string]bool{}
	for _, a := range activities.GetActivities() {
		types[a.GetType()] = true
	}
	for _, want := range []string{"settings.updated", "plugin.installed", "plugin.configured", "plugin.uninstalled"} {
		if !types[want] {
			t.Errorf("activities lack %s: %v", want, types)
		}
	}
	tasks, err := systemv1connect.NewTaskServiceClient(http.DefaultClient, srv.url, token).ListTasks(ctx, &systemv1.ListTasksRequest{})
	if err != nil || len(tasks.GetTasks()) != 2 || tasks.GetTasks()[0].GetLastRun() == nil {
		t.Errorf("ListTasks = %v, %v", tasks, err)
	}

	// A backup, downloaded, restores into a new server.
	backups := systemv1connect.NewBackupServiceClient(http.DefaultClient, srv.url, token)
	made, err := backups.CreateBackup(ctx, &systemv1.CreateBackupRequest{})
	if err != nil {
		t.Fatalf("CreateBackup: %v", err)
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.url+"/system/backups/"+made.GetBackup().GetName(), nil)
	req.Header.Set("Authorization", "Bearer "+first.GetAccessToken())
	dl, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(dl.Body)
	dl.Body.Close()
	if dl.StatusCode != http.StatusOK || int64(len(data)) != made.GetBackup().GetSizeBytes() {
		t.Fatalf("download = %d, %d bytes", dl.StatusCode, len(data))
	}
	file := filepath.Join(t.TempDir(), "backup.zip")
	if err := os.WriteFile(file, data, 0o600); err != nil {
		t.Fatal(err)
	}
	srv.stop()

	restored := start(t, server.Config{
		Database: "sqlite:" + filepath.Join(t.TempDir(), "restored.db"), FFmpeg: ffmpeg, FFprobe: ffprobe,
		TranscodeDir: t.TempDir(), CacheDir: t.TempDir(), MetadataDir: t.TempDir(), PluginDir: t.TempDir(), Restore: file,
	})
	login, err := authv1connect.NewAuthServiceClient(http.DefaultClient, restored.url).Login(ctx, authv1.LoginRequest_builder{
		Name: new("admin"), Password: new("secret"),
		Device: authv1.Device_builder{Id: new("e2e-2"), Name: new("Test"), Client: new("Test"), ClientVersion: new("0")}.Build(),
	}.Build())
	if err != nil {
		t.Fatalf("Login to the restored server: %v", err)
	}
	token = withToken(login.GetAccessToken())
	libs, err := libraryv1connect.NewLibraryServiceClient(http.DefaultClient, restored.url, token).ListLibraries(ctx, &libraryv1.ListLibrariesRequest{})
	if err != nil || len(libs.GetLibraries()) != 1 || libs.GetLibraries()[0].GetName() != "Films" {
		t.Errorf("restored libraries = %v, %v", libs, err)
	}
	restoredSet, err := systemv1connect.NewSystemServiceClient(http.DefaultClient, restored.url, token).GetServerSettings(ctx, &systemv1.GetServerSettingsRequest{})
	if err != nil || restoredSet.GetSettings().GetTranscoding().GetEncoderPreset() != "ultrafast" {
		t.Errorf("restored settings = %v, %v", restoredSet, err)
	}
}
