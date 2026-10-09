package server_test

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/fieldmaskpb"

	"github.com/mavioai/mavio/apps/server/internal/server"
	authv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1/authv1connect"
	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1/libraryv1connect"
	systemv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1/systemv1connect"
)

// TestMetadataManagement runs the assembled server through its API: a film
// the metadata plugin took for another is identified again by an
// administrator and edited; the locked overview survives a refresh, and
// the poster chosen is written next to the film, where the scan of a new
// library reads it back with the NFO file.
func TestMetadataManagement(t *testing.T) {
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
	folder := filepath.Join(media, "Concubine (1993)")
	film := filepath.Join(folder, "Concubine (1993).mkv")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	gen := exec.CommandContext(ctx, ffmpeg, "-loglevel", "error", "-f", "lavfi", "-i", "testsrc2=size=320x240:rate=24",
		"-t", "2", "-c:v", "libx264", film)
	if out, err := gen.CombinedOutput(); err != nil {
		t.Skipf("integration test: cannot generate the film: %v\n%s", err, out)
	}
	// The plugin's images.
	artwork := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("\xff\xd8\xff\xe0" + r.URL.Path))
	}))
	t.Cleanup(artwork.Close)

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
			MetadataDir: t.TempDir(), PluginDir: pluginDir, Logger: slog.New(slog.DiscardHandler),
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
	system := systemv1connect.NewSystemServiceClient(http.DefaultClient, url, token)
	if _, err := system.SetPluginConfig(ctx, systemv1.SetPluginConfigRequest_builder{
		PluginId: new("org.mavio.smoke"), ConfigJson: new(`{"api_key":"secret","image_base":"` + artwork.URL + `"}`),
	}.Build()); err != nil {
		t.Fatalf("SetPluginConfig: %v", err)
	}

	libraries := libraryv1connect.NewLibraryServiceClient(http.DefaultClient, url, token)
	items := libraryv1connect.NewItemServiceClient(http.DefaultClient, url, token)
	meta := libraryv1connect.NewMetadataServiceClient(http.DefaultClient, url, token)
	createLibrary := func(name string, saveLocal bool) {
		t.Helper()
		if _, err := libraries.CreateLibrary(ctx, libraryv1.CreateLibraryRequest_builder{Spec: libraryv1.LibrarySpec_builder{
			Name: new(name), Kind: new(libraryv1.LibraryKind_LIBRARY_KIND_MOVIES), Paths: []string{media},
			SaveLocalMetadata: new(saveLocal), AutoCollections: new(true),
		}.Build()}.Build()); err != nil {
			t.Fatalf("CreateLibrary: %v", err)
		}
	}
	// waitFor returns the film once its metadata satisfies ok.
	waitFor := func(what string, ok func(*libraryv1.Item) bool) *libraryv1.Item {
		t.Helper()
		for deadline := time.Now().Add(time.Minute); ; time.Sleep(100 * time.Millisecond) {
			list, err := items.ListItems(ctx, libraryv1.ListItemsRequest_builder{Kinds: []libraryv1.ItemKind{libraryv1.ItemKind_ITEM_KIND_MOVIE}}.Build())
			if err != nil {
				t.Fatalf("ListItems: %v", err)
			}
			for _, it := range list.GetItems() {
				if ok(it) {
					return it
				}
			}
			if time.Now().After(deadline) {
				t.Fatalf("%s: not within a minute; items %v", what, list.GetItems())
			}
		}
	}

	// The scan takes the film for "The Concubine".
	createLibrary("Films", true)
	movie := waitFor("misidentified", func(it *libraryv1.Item) bool { return it.GetExternalIds()["tmdb"] == "117974" })

	// The administrator searches for the right film and identifies it.
	found, err := meta.SearchRemote(ctx, libraryv1.SearchRemoteRequest_builder{
		ItemId: new(movie.GetId()), Name: new("Farewell My Concubine"),
	}.Build())
	if err != nil || len(found.GetResults()) != 1 || found.GetResults()[0].GetProvider() != "org.mavio.smoke" {
		t.Fatalf("SearchRemote = %v, %v", found, err)
	}
	identified, err := meta.IdentifyItem(ctx, libraryv1.IdentifyItemRequest_builder{
		ItemId: new(movie.GetId()), ExternalIds: found.GetResults()[0].GetExternalIds(),
	}.Build())
	if err != nil {
		t.Fatalf("IdentifyItem: %v", err)
	}
	if it := identified.GetItem(); it.GetName() != "Farewell My Concubine" || it.GetOriginalTitle() != "霸王别姬" ||
		!slices.Equal(it.GetGenres(), []string{"Drama", "Romance"}) || it.GetCollectionName() != "Chen Kaige Classics" {
		t.Errorf("identified film: %v", it)
	}
	// It went into its providers' collection.
	collections, err := items.ListItems(ctx, libraryv1.ListItemsRequest_builder{Kinds: []libraryv1.ItemKind{libraryv1.ItemKind_ITEM_KIND_COLLECTION}}.Build())
	if err != nil || len(collections.GetItems()) != 1 || collections.GetItems()[0].GetName() != "Chen Kaige Classics" {
		t.Errorf("collections = %v, %v", collections, err)
	}

	// The administrator edits the overview and locks it.
	const overview = "Edited by the administrator."
	if _, err := meta.UpdateItem(ctx, libraryv1.UpdateItemRequest_builder{
		Id: new(movie.GetId()),
		Metadata: libraryv1.ItemMetadata_builder{
			Overview: new(overview), LockedFields: []libraryv1.MetadataField{libraryv1.MetadataField_METADATA_FIELD_OVERVIEW},
		}.Build(),
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"overview", "locked_fields"}},
	}.Build()); err != nil {
		t.Fatalf("UpdateItem: %v", err)
	}
	refreshed, err := meta.RefreshItem(ctx, libraryv1.RefreshItemRequest_builder{Id: new(movie.GetId()), ReplaceMetadata: new(true)}.Build())
	if err != nil {
		t.Fatalf("RefreshItem: %v", err)
	}
	if it := refreshed.GetItem(); it.GetOverview() != overview || it.GetName() != "Farewell My Concubine" ||
		!slices.Equal(it.GetLockedFields(), []libraryv1.MetadataField{libraryv1.MetadataField_METADATA_FIELD_OVERVIEW}) {
		t.Errorf("refreshed film: %v", it)
	}

	// The administrator chooses the second poster.
	posters, err := meta.ListRemoteImages(ctx, libraryv1.ListRemoteImagesRequest_builder{
		ItemId: new(movie.GetId()), Kind: new(libraryv1.ImageKind_IMAGE_KIND_PRIMARY),
	}.Build())
	if err != nil || len(posters.GetImages()) != 2 {
		t.Fatalf("ListRemoteImages = %v, %v", posters, err)
	}
	chosen := posters.GetImages()[1].GetUrl()
	if _, err := meta.SetItemImage(ctx, libraryv1.SetItemImageRequest_builder{
		ItemId: new(movie.GetId()), Kind: new(libraryv1.ImageKind_IMAGE_KIND_PRIMARY), Url: new(chosen),
	}.Build()); err != nil {
		t.Fatalf("SetItemImage: %v", err)
	}
	poster, err := os.ReadFile(filepath.Join(folder, "poster.jpg"))
	if want := "\xff\xd8\xff\xe0/10997/poster-alt.jpg"; err != nil || string(poster) != want {
		t.Errorf("poster beside the film = %q, %v, want = %q", poster, err, want)
	}
	nfo, err := os.ReadFile(filepath.Join(folder, "Concubine (1993).nfo"))
	if err != nil || !strings.Contains(string(nfo), "<lockedfields>Overview</lockedfields>") ||
		!strings.Contains(string(nfo), `<uniqueid type="tmdb">10997</uniqueid>`) {
		t.Errorf("NFO beside the film: %v\n%s", err, nfo)
	}

	// A new library over the same folder reads it all back.
	if _, err := libraries.DeleteLibrary(ctx, libraryv1.DeleteLibraryRequest_builder{Id: new(movie.GetLibraryId())}.Build()); err != nil {
		t.Fatalf("DeleteLibrary: %v", err)
	}
	createLibrary("Films again", false)
	again := waitFor("read back", func(it *libraryv1.Item) bool {
		return it.GetId() != movie.GetId() && it.GetOverview() == overview && len(it.GetImages()) > 0
	})
	if again.GetExternalIds()["tmdb"] != "10997" || again.GetName() != "Farewell My Concubine" {
		t.Errorf("film read back: %v", again)
	}
	i := slices.IndexFunc(again.GetImages(), func(img *libraryv1.Image) bool { return img.GetKind() == libraryv1.ImageKind_IMAGE_KIND_PRIMARY })
	if i < 0 {
		t.Fatalf("film read back without a poster: %v", again.GetImages())
	}
	if got := string(get(t, url+"/images/"+again.GetImages()[i].GetId())); got != string(poster) {
		t.Errorf("poster read back = %q, want = %q", got, poster)
	}
}
