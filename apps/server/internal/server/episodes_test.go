package server_test

import (
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"google.golang.org/protobuf/types/known/fieldmaskpb"

	"github.com/mavioai/mavio/apps/server/internal/server"
	authv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1/authv1connect"
	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1/libraryv1connect"
)

// TestVirtualEpisodes runs the assembled server with an episode list
// plugin: a series with one episode file lists no virtual episodes until
// an administrator monitors it, which lists the episodes it lacks at once;
// queries show them only when asked.
func TestVirtualEpisodes(t *testing.T) {
	t.Parallel()
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
	season := filepath.Join(media, "Smoke Show", "Season 01")
	if err := os.MkdirAll(season, 0o755); err != nil {
		t.Fatal(err)
	}
	episode := filepath.Join(season, "Smoke Show S01E01.mkv")
	run(t, ffmpeg, "-loglevel", "error", "-f", "lavfi", "-i", "testsrc2=size=160x120:rate=24", "-t", "1", "-c:v", "libx264", episode)
	settled(t, episode)

	srv := start(t, server.Config{
		Database: "sqlite:" + filepath.Join(t.TempDir(), "mavio.db"), FFmpeg: ffmpeg, FFprobe: ffprobe,
		TranscodeDir: t.TempDir(), CacheDir: t.TempDir(), MetadataDir: t.TempDir(),
		PluginDir: installSmoke(t, "wasm", `"capabilities":["CAPABILITY_EPISODE_LIST_PROVIDER"]`),
	})
	first, err := authv1connect.NewAuthServiceClient(http.DefaultClient, srv.url).CreateFirstUser(ctx, authv1.CreateFirstUserRequest_builder{
		Name: new("admin"), Password: new("secret"), Device: device("e2e"),
	}.Build())
	if err != nil {
		t.Fatalf("CreateFirstUser: %v", err)
	}
	token := withToken(first.GetAccessToken())
	libraries := libraryv1connect.NewLibraryServiceClient(http.DefaultClient, srv.url, token)
	items := libraryv1connect.NewItemServiceClient(http.DefaultClient, srv.url, token)
	meta := libraryv1connect.NewMetadataServiceClient(http.DefaultClient, srv.url, token)
	if _, err := libraries.CreateLibrary(ctx, libraryv1.CreateLibraryRequest_builder{Spec: libraryv1.LibrarySpec_builder{
		Name: new("Shows"), Kind: new(libraryv1.LibraryKind_LIBRARY_KIND_SHOWS), Paths: []string{media},
	}.Build()}.Build()); err != nil {
		t.Fatalf("CreateLibrary: %v", err)
	}
	list := func(req *libraryv1.ListItemsRequest) []*libraryv1.Item {
		t.Helper()
		resp, err := items.ListItems(ctx, req)
		if err != nil {
			t.Fatalf("ListItems: %v", err)
		}
		return resp.GetItems()
	}
	episodes := func(includeVirtual bool) []*libraryv1.Item {
		return list(libraryv1.ListItemsRequest_builder{
			Kinds: []libraryv1.ItemKind{libraryv1.ItemKind_ITEM_KIND_EPISODE}, IncludeVirtual: new(includeVirtual),
			Sort: []*libraryv1.SortSpec{libraryv1.SortSpec_builder{Field: new(libraryv1.SortField_SORT_FIELD_INDEX)}.Build()},
		}.Build())
	}
	var series *libraryv1.Item
	waitFor(t, "the scan", func() bool {
		found := list(libraryv1.ListItemsRequest_builder{Kinds: []libraryv1.ItemKind{libraryv1.ItemKind_ITEM_KIND_SERIES}}.Build())
		if len(found) == 1 && len(episodes(false)) == 1 {
			series = found[0]
			return true
		}
		return false
	})
	// The library lists no virtual episodes.
	if got := episodes(true); len(got) != 1 || got[0].GetVirtual() {
		t.Fatalf("episodes before monitoring = %v, want the file's only", names(got))
	}

	// Monitoring the series lists every episode it lacks.
	if _, err := meta.UpdateItem(ctx, libraryv1.UpdateItemRequest_builder{
		Id: new(series.GetId()), Metadata: libraryv1.ItemMetadata_builder{Monitored: new(true)}.Build(),
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"monitored"}},
	}.Build()); err != nil {
		t.Fatalf("UpdateItem: %v", err)
	}
	want := []string{"Smoke Show S01E01", "Episode 1.2", "Episode 2.1", "Episode 2.2"}
	waitFor(t, "virtual episodes", func() bool { return slices.Equal(names(episodes(true)), want) },
		func() any { return names(episodes(true)) })
	for _, it := range episodes(true) {
		if it.GetVirtual() != (it.GetName() != "Smoke Show S01E01") {
			t.Errorf("episode %q virtual = %v", it.GetName(), it.GetVirtual())
		}
	}
	// Queries leave them out unless asked; season 2 is virtual.
	if got := names(episodes(false)); !slices.Equal(got, want[:1]) {
		t.Errorf("episodes without virtual ones = %v, want %v", got, want[:1])
	}
	seasons := list(libraryv1.ListItemsRequest_builder{
		Kinds: []libraryv1.ItemKind{libraryv1.ItemKind_ITEM_KIND_SEASON}, IncludeVirtual: new(true),
		Sort: []*libraryv1.SortSpec{libraryv1.SortSpec_builder{Field: new(libraryv1.SortField_SORT_FIELD_INDEX)}.Build()},
	}.Build())
	if len(seasons) != 2 || seasons[0].GetVirtual() || !seasons[1].GetVirtual() {
		t.Errorf("seasons = %v, want season 1 and a virtual season 2", seasons)
	}
	if got := list(libraryv1.ListItemsRequest_builder{Monitored: new(true)}.Build()); len(got) != 1 || got[0].GetId() != series.GetId() {
		t.Errorf("monitored items = %v, want the series", names(got))
	}
}
