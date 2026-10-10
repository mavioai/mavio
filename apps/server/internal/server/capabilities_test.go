package server_test

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
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
	userv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1/userv1connect"
)

// TestPluginCapabilities runs the assembled server with real ffmpeg and a
// plugin of every provider capability, in both runtimes; TestPluginPlatform
// covers the platform's other capabilities.
func TestPluginCapabilities(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("integration test: ffmpeg not found on PATH")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("integration test: ffprobe not found on PATH")
	}
	// A short video with sound, copied wherever the libraries need one.
	video := filepath.Join(t.TempDir(), "video.mkv")
	run(t, ffmpeg, "-v", "error", "-f", "lavfi", "-i", "testsrc2=size=160x90:rate=24", "-f", "lavfi", "-i", "sine=frequency=440",
		"-t", "6", "-c:v", "libx264", "-c:a", "aac", video)
	track := filepath.Join(t.TempDir(), "track.flac")
	run(t, ffmpeg, "-v", "error", "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=44100", "-t", "3", track)
	for _, rt := range []string{"wasm", "process"} {
		t.Run(rt, func(t *testing.T) { testPluginCapabilities(t, rt, ffmpeg, ffprobe, video, track) })
	}
}

// imageServer serves a JPEG at every .jpg path and a PNG at every other.
func imageServer(t *testing.T) *httptest.Server {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 20, 30))
	var jpg, pngData bytes.Buffer
	if err := jpeg.Encode(&jpg, img, nil); err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(&pngData, img); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".jpg") {
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write(jpg.Bytes())
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngData.Bytes())
	}))
	t.Cleanup(srv.Close)
	return srv
}

// copyFile copies a file to dst, making its folder, and settles it.
func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		t.Fatal(err)
	}
	settled(t, dst)
}

// writeFile writes a file the scanner does not take for one being written.
func writeFile(t *testing.T, name, content string) {
	t.Helper()
	if err := os.WriteFile(name, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	settled(t, name)
}

func testPluginCapabilities(t *testing.T, rt, ffmpeg, ffprobe, video, track string) {
	ctx := t.Context()
	// Only WASM plugins may resolve folders.
	resolver := ""
	if rt == "wasm" {
		resolver = `,"CAPABILITY_RESOLVER"`
	}
	pluginDir := installSmoke(t, rt, `"capabilities":["CAPABILITY_METADATA_PROVIDER","CAPABILITY_AUTH_PROVIDER","CAPABILITY_NOTIFIER",
			"CAPABILITY_SUBTITLE_PROVIDER","CAPABILITY_SEGMENT_PROVIDER","CAPABILITY_IMAGE_PROVIDER","CAPABILITY_LOCAL_METADATA",
			"CAPABILITY_METADATA_SAVER","CAPABILITY_METADATA_PROCESSOR","CAPABILITY_LYRICS_PROVIDER","CAPABILITY_INTRO_PROVIDER",
			"CAPABILITY_IMAGE_GENERATOR","CAPABILITY_MEDIA_SOURCE_PROVIDER","CAPABILITY_PASSWORD_RESET"`+resolver+`],
		"localMetadataFiles":["*.title"]`)
	dataDir := filepath.Join(t.TempDir(), "data")
	smokeData := filepath.Join(dataDir, "org.mavio.smoke")

	films, home, music, photos := t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir()
	film := filepath.Join(films, "Farewell My Concubine (1993)", "Farewell My Concubine (1993).mkv")
	copyFile(t, video, film)
	if rt == "wasm" {
		copyFile(t, video, filepath.Join(films, "Claimed", "Odd (2001).mkv"))
		copyFile(t, video, filepath.Join(films, "Claimed", "skip (2002).mkv"))
		writeFile(t, filepath.Join(films, "Claimed", "claim.me"), "")
	}
	clip, other := filepath.Join(home, "clip.mkv"), filepath.Join(home, "other.mkv")
	copyFile(t, video, clip)
	copyFile(t, video, other)
	writeFile(t, filepath.Join(home, "clip.title"), "Local Title\n")
	song := filepath.Join(music, "Band", "Album", "01 - Song.flac")
	copyFile(t, track, song)
	trip := filepath.Join(photos, "Trip")
	photo := filepath.Join(trip, "beach.png")
	var pic bytes.Buffer
	if err := png.Encode(&pic, image.NewRGBA(image.Rect(0, 0, 4, 3))); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(trip, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, photo, pic.String())
	if rt == "wasm" {
		writeFile(t, filepath.Join(trip, "claim.me"), "")
	}

	srv := start(t, server.Config{
		Database: "sqlite:" + filepath.Join(t.TempDir(), "mavio.db"), FFmpeg: ffmpeg, FFprobe: ffprobe,
		TranscodeDir: t.TempDir(), CacheDir: t.TempDir(), MetadataDir: t.TempDir(), PluginDir: pluginDir, PluginDataDir: dataDir,
	})
	authClient := authv1connect.NewAuthServiceClient(http.DefaultClient, srv.url)
	device := func(id string) *authv1.Device {
		return authv1.Device_builder{Id: new(id), Name: new("Test"), Client: new("Test"), ClientVersion: new("0")}.Build()
	}
	first, err := authClient.CreateFirstUser(ctx, authv1.CreateFirstUserRequest_builder{
		Name: new("admin"), Password: new("secret"), Device: device("e2e"),
	}.Build())
	if err != nil {
		t.Fatalf("CreateFirstUser: %v", err)
	}
	token := withToken(first.GetAccessToken())
	system := systemv1connect.NewSystemServiceClient(http.DefaultClient, srv.url, token)
	images := imageServer(t)
	if _, err := system.SetPluginConfig(ctx, systemv1.SetPluginConfigRequest_builder{
		PluginId: new("org.mavio.smoke"), ConfigJson: new(`{"image_base":"` + images.URL + `"}`),
	}.Build()); err != nil {
		t.Fatalf("SetPluginConfig: %v", err)
	}

	// Notifiers hear of activity, such as a new account.
	users := userv1connect.NewUserServiceClient(http.DefaultClient, srv.url, token)
	if _, err := users.CreateUser(ctx, userv1.CreateUserRequest_builder{Name: new("kid"), Password: new("secret")}.Build()); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	waitFor(t, "the notifier to hear of the new user", func() bool {
		got, _ := os.ReadFile(filepath.Join(smokeData, "notified"))
		return slices.Contains(strings.Fields(string(got)), "user.created")
	})

	// Users of an authentication plugin sign in with the passwords it
	// accepts.
	if _, err := users.CreateUser(ctx, userv1.CreateUserRequest_builder{Name: new("directory"), AuthProvider: new("org.mavio.smoke")}.Build()); err != nil {
		t.Fatalf("CreateUser(directory): %v", err)
	}
	login := func(name, password string) error {
		_, err := authClient.Login(ctx, authv1.LoginRequest_builder{Name: new(name), Password: new(password), Device: device(name)}.Build())
		return err
	}
	if err := login("directory", "directory"); err != nil {
		t.Errorf("Login through the authentication plugin: %v", err)
	}
	if err := login("directory", "secret"); err == nil {
		t.Error("Login with a password the plugin rejects succeeded")
	}

	// The password reset plugin delivers the PIN that resets a password.
	settings, err := system.GetServerSettings(ctx, &systemv1.GetServerSettingsRequest{})
	if err != nil {
		t.Fatalf("GetServerSettings: %v", err)
	}
	set := settings.GetSettings()
	set.SetPasswordResetPlugin("org.mavio.smoke")
	if _, err := system.UpdateServerSettings(ctx, systemv1.UpdateServerSettingsRequest_builder{Settings: set}.Build()); err != nil {
		t.Fatalf("UpdateServerSettings: %v", err)
	}
	if _, err := authClient.ForgotPassword(ctx, authv1.ForgotPasswordRequest_builder{Name: new("kid")}.Build()); err != nil {
		t.Fatalf("ForgotPassword: %v", err)
	}
	var pin string
	waitFor(t, "the PIN to be delivered", func() bool {
		got, _ := os.ReadFile(filepath.Join(smokeData, "resets"))
		if f := strings.Fields(string(got)); len(f) == 2 && f[0] == "kid" {
			pin = f[1]
		}
		return pin != ""
	})
	if _, err := authClient.ResetPassword(ctx, authv1.ResetPasswordRequest_builder{
		Name: new("kid"), Pin: new(pin), NewPassword: new("reset"),
	}.Build()); err != nil {
		t.Fatalf("ResetPassword: %v", err)
	}
	if err := login("kid", "reset"); err != nil {
		t.Errorf("Login with the new password: %v", err)
	}

	libraries := libraryv1connect.NewLibraryServiceClient(http.DefaultClient, srv.url, token)
	for _, spec := range []*libraryv1.LibrarySpec{
		libraryv1.LibrarySpec_builder{Name: new("Films"), Kind: new(libraryv1.LibraryKind_LIBRARY_KIND_MOVIES), Paths: []string{films}}.Build(),
		libraryv1.LibrarySpec_builder{
			Name: new("Home"), Kind: new(libraryv1.LibraryKind_LIBRARY_KIND_HOME_VIDEOS), Paths: []string{home}, SaveLocalMetadata: new(true),
		}.Build(),
		libraryv1.LibrarySpec_builder{Name: new("Photos"), Kind: new(libraryv1.LibraryKind_LIBRARY_KIND_PHOTOS), Paths: []string{photos}}.Build(),
		libraryv1.LibrarySpec_builder{
			Name: new("Music"), Kind: new(libraryv1.LibraryKind_LIBRARY_KIND_MUSIC), Paths: []string{music}, DownloadLyrics: new(true),
		}.Build(),
	} {
		if _, err := libraries.CreateLibrary(ctx, libraryv1.CreateLibraryRequest_builder{Spec: spec}.Build()); err != nil {
			t.Fatalf("CreateLibrary(%s): %v", spec.GetName(), err)
		}
	}
	items := libraryv1connect.NewItemServiceClient(http.DefaultClient, srv.url, token)
	// item waits for the item of a path to be refreshed as ok says.
	item := func(kind libraryv1.ItemKind, path, what string, ok func(*libraryv1.GetItemResponse) bool) *libraryv1.GetItemResponse {
		t.Helper()
		var got *libraryv1.GetItemResponse
		waitFor(t, what, func() bool {
			list, err := items.ListItems(ctx, libraryv1.ListItemsRequest_builder{Kinds: []libraryv1.ItemKind{kind}}.Build())
			if err != nil {
				t.Fatal(err)
			}
			for _, it := range list.GetItems() {
				resp, err := items.GetItem(ctx, libraryv1.GetItemRequest_builder{Id: new(it.GetId())}.Build())
				if err != nil {
					t.Fatal(err)
				}
				if resp.GetItem().GetPath() == path {
					got = resp
					return ok(resp)
				}
			}
			return false
		}, func() any { return got.GetItem() })
		return got
	}
	hasImage := func(r *libraryv1.GetItemResponse, kind libraryv1.ImageKind) bool {
		return slices.ContainsFunc(r.GetItem().GetImages(), func(i *libraryv1.Image) bool { return i.GetKind() == kind })
	}

	// The film gets the metadata provider's metadata and poster, the image
	// provider's logo, the processor's tag and the segment provider's
	// segments.
	filmItem := item(libraryv1.ItemKind_ITEM_KIND_MOVIE, film, "the film's metadata, images, tag and segments", func(r *libraryv1.GetItemResponse) bool {
		return r.GetItem().GetName() == "Farewell My Concubine" && hasImage(r, libraryv1.ImageKind_IMAGE_KIND_PRIMARY) &&
			hasImage(r, libraryv1.ImageKind_IMAGE_KIND_LOGO) && slices.Contains(r.GetItem().GetTags(), "processed") && len(r.GetSegments()) == 2
	})

	// Local metadata names a video; the saver writes the other's beside it;
	// both get generated posters.
	clipItem := item(libraryv1.ItemKind_ITEM_KIND_VIDEO, clip, "the local title and a generated poster", func(r *libraryv1.GetItemResponse) bool {
		return r.GetItem().GetName() == "Local Title" && hasImage(r, libraryv1.ImageKind_IMAGE_KIND_PRIMARY)
	})
	item(libraryv1.ItemKind_ITEM_KIND_VIDEO, other, "a generated poster", func(r *libraryv1.GetItemResponse) bool {
		return hasImage(r, libraryv1.ImageKind_IMAGE_KIND_PRIMARY)
	})
	waitFor(t, "the saver to write other.mkv.title", func() bool {
		got, _ := os.ReadFile(filepath.Join(home, "other.mkv.title"))
		return string(got) == "other\n"
	})

	// A refresh downloads lyrics for the track.
	songItem := item(libraryv1.ItemKind_ITEM_KIND_TRACK, song, "the track", func(*libraryv1.GetItemResponse) bool { return true })
	waitFor(t, "lyrics to be downloaded", func() bool {
		_, err := os.Stat(strings.TrimSuffix(song, ".flac") + ".lrc")
		return err == nil
	})
	lyrics, err := items.GetLyrics(ctx, libraryv1.GetLyricsRequest_builder{ItemId: new(songItem.GetItem().GetId())}.Build())
	if err != nil || !lyrics.GetSynced() || len(lyrics.GetLines()) != 1 || lyrics.GetLines()[0].GetText() != songItem.GetItem().GetName() {
		t.Errorf("GetLyrics = %v, %v", lyrics, err)
	}

	// Photos reach plugins too.
	item(libraryv1.ItemKind_ITEM_KIND_PHOTO, photo, "the processed photo", func(r *libraryv1.GetItemResponse) bool {
		return slices.Contains(r.GetItem().GetTags(), "processed")
	})

	// The resolver claims folders and leaves out what it ignores.
	if rt == "wasm" {
		item(libraryv1.ItemKind_ITEM_KIND_PHOTO_ALBUM, trip, "the resolver's photo album",
			func(r *libraryv1.GetItemResponse) bool { return r.GetItem().GetName() == "Claimed Album" })
		item(libraryv1.ItemKind_ITEM_KIND_PHOTO, photo, "the resolver's photo",
			func(r *libraryv1.GetItemResponse) bool { return r.GetItem().GetName() == "beach.png" })
		item(libraryv1.ItemKind_ITEM_KIND_MOVIE, filepath.Join(films, "Claimed", "Odd (2001).mkv"), "the resolver's film",
			func(r *libraryv1.GetItemResponse) bool { return r.GetItem().GetName() == "Odd (2001).mkv" })
		list, err := items.ListItems(ctx, libraryv1.ListItemsRequest_builder{Kinds: []libraryv1.ItemKind{libraryv1.ItemKind_ITEM_KIND_MOVIE}}.Build())
		if err != nil {
			t.Fatal(err)
		}
		if slices.ContainsFunc(list.GetItems(), func(it *libraryv1.Item) bool { return strings.HasPrefix(it.GetName(), "skip") }) {
			t.Errorf("movies = %v, want none named skip", list.GetItems())
		}
	}

	// Subtitles are found and downloaded through the subtitle provider.
	meta := libraryv1connect.NewMetadataServiceClient(http.DefaultClient, srv.url, token)
	filmID := filmItem.GetItem().GetId()
	subs, err := meta.SearchSubtitles(ctx, libraryv1.SearchSubtitlesRequest_builder{ItemId: new(filmID), Language: new("en")}.Build())
	if err != nil || len(subs.GetSubtitles()) != 1 || subs.GetSubtitles()[0].GetProvider() != "org.mavio.smoke" {
		t.Fatalf("SearchSubtitles = %v, %v", subs, err)
	}
	sub := subs.GetSubtitles()[0]
	got, err := meta.DownloadSubtitle(ctx, libraryv1.DownloadSubtitleRequest_builder{
		ItemId: new(filmID), Provider: new(sub.GetProvider()), SubtitleId: new(sub.GetId()),
	}.Build())
	if err != nil || got.GetStream().GetLanguage() == "" || got.GetStream().GetKind() != libraryv1.StreamKind_STREAM_KIND_SUBTITLE {
		t.Errorf("DownloadSubtitle = %v, %v", got, err)
	}

	// The intro provider picks what plays before the film.
	if err := os.WriteFile(filepath.Join(smokeData, "intros"), []byte(clipItem.GetItem().GetId()+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	playback := playbackv1connect.NewPlaybackServiceClient(http.DefaultClient, srv.url, token)
	intros, err := playback.ListIntros(ctx, playbackv1.ListIntrosRequest_builder{ItemId: new(filmID)}.Build())
	if err != nil || len(intros.GetItems()) != 1 || intros.GetItems()[0].GetId() != clipItem.GetItem().GetId() {
		t.Errorf("ListIntros = %v, %v", intros, err)
	}

	// The media source provider adds a source read over HTTP, which plays
	// directly from where it is.
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, video)
	}))
	t.Cleanup(remote.Close)
	url := remote.URL + "/films/concubine.mkv"
	if err := os.WriteFile(filepath.Join(smokeData, "source"), []byte(url+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	sources, err := playback.ListMediaSources(ctx, playbackv1.ListMediaSourcesRequest_builder{ItemId: new(filmID)}.Build())
	if err != nil || len(sources.GetMediaSources()) != 2 || sources.GetMediaSources()[0].GetRemote() {
		t.Fatalf("ListMediaSources = %v, %v", sources, err)
	}
	rs := sources.GetMediaSources()[1]
	if !rs.GetRemote() || rs.GetName() != "Remote" || rs.GetPath() != url || len(rs.GetStreams()) != 2 {
		t.Errorf("remote source = %v", rs)
	}
	started, err := playback.StartPlayback(ctx, playbackv1.StartPlaybackRequest_builder{
		ItemId: new(filmID), MediaSourceId: new(rs.GetId()), Capabilities: playbackv1.ClientCapabilities_builder{
			DirectPlay: []*playbackv1.DirectPlayProfile{playbackv1.DirectPlayProfile_builder{
				Kind: playbackv1.MediaKind_MEDIA_KIND_VIDEO.Enum(), Container: new("mkv"), VideoCodec: new("h264"), AudioCodec: new("aac"),
			}.Build()},
		}.Build(),
	}.Build())
	if err != nil || started.GetMediaSourceId() != rs.GetId() {
		t.Fatalf("StartPlayback(remote) = %v, %v", started, err)
	}
	noRedirect := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := noRedirect.Get(srv.url + "/" + started.GetUrl())
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusFound || resp.Header.Get("Location") != url {
		t.Errorf("GET %s = %d → %q, want a redirect to %s", started.GetUrl(), resp.StatusCode, resp.Header.Get("Location"), url)
	}
}

// waitFor waits up to two minutes for ok; state tells, when given, what
// there was instead.
func waitFor(t *testing.T, what string, ok func() bool, state ...func() any) {
	t.Helper()
	for deadline := time.Now().Add(2 * time.Minute); !ok(); time.Sleep(100 * time.Millisecond) {
		if time.Now().After(deadline) {
			var got any
			if len(state) > 0 {
				got = state[0]()
			}
			t.Fatalf("waited two minutes for %s; got = %v", what, got)
		}
	}
}
