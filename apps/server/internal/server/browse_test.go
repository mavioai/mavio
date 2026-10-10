package server_test

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/apps/server/internal/server"
	authv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1/authv1connect"
	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1/libraryv1connect"
	userv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1/userv1connect"
)

// TestBrowsing runs the assembled server through its API only: a films
// and a shows library are scanned with their NFO files, then browsed
// through the views, latest items, next up and genres; a user limited to
// content rated PG builds a playlist and never sees the episodes of a
// series rated above that.
func TestBrowsing(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("integration test: ffmpeg not found on PATH")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("integration test: ffprobe not found on PATH")
	}
	ctx := t.Context()
	films, shows := t.TempDir(), t.TempDir()
	write := func(path, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// One generated video, copied for every file.
	video := filepath.Join(t.TempDir(), "video.mkv")
	gen := exec.CommandContext(ctx, ffmpeg, "-loglevel", "error", "-f", "lavfi", "-i", "testsrc2=size=64x64:rate=24",
		"-t", "1", "-c:v", "libx264", video)
	if out, err := gen.CombinedOutput(); err != nil {
		t.Skipf("integration test: cannot generate a video: %v\n%s", err, out)
	}
	settled(t, video)
	data, err := os.ReadFile(video)
	if err != nil {
		t.Fatal(err)
	}
	for name, rating := range map[string]string{"Up (2009)": "PG", "Heat (1995)": "R"} {
		write(filepath.Join(films, name, name+".mkv"), string(data))
		write(filepath.Join(films, name, "movie.nfo"), fmt.Sprintf(`<movie><mpaa>%s</mpaa><genre>%s</genre></movie>`,
			rating, map[string]string{"PG": "Animation", "R": "Crime"}[rating]))
	}
	for series, rating := range map[string]string{"Bluey": "TV-Y", "Lost": "TV-14"} {
		write(filepath.Join(shows, series, "tvshow.nfo"), fmt.Sprintf(`<tvshow><title>%s</title><mpaa>%s</mpaa><genre>%s</genre></tvshow>`,
			series, rating, map[string]string{"TV-Y": "Animation", "TV-14": "Drama"}[rating]))
		for n := 1; n <= 2; n++ {
			write(filepath.Join(shows, series, "Season 1", fmt.Sprintf("%s S01E%02d.mkv", series, n)), string(data))
		}
	}

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
			Logger: slog.New(slog.DiscardHandler),
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
		Name: new("admin"), Password: new("secret"), Device: device("e2e"),
	}.Build())
	if err != nil {
		t.Fatalf("CreateFirstUser: %v", err)
	}
	token := withToken(first.GetAccessToken())
	libraries := libraryv1connect.NewLibraryServiceClient(http.DefaultClient, url, token)
	libs := map[string]string{}
	for name, spec := range map[string]struct {
		kind libraryv1.LibraryKind
		path string
	}{"Films": {libraryv1.LibraryKind_LIBRARY_KIND_MOVIES, films}, "Shows": {libraryv1.LibraryKind_LIBRARY_KIND_SHOWS, shows}} {
		created, err := libraries.CreateLibrary(ctx, libraryv1.CreateLibraryRequest_builder{
			Spec: libraryv1.LibrarySpec_builder{Name: &name, Kind: &spec.kind, Paths: []string{spec.path}}.Build(),
		}.Build())
		if err != nil {
			t.Fatalf("CreateLibrary: %v", err)
		}
		libs[name] = created.GetLibrary().GetId()
	}

	// Wait for the scans and the refreshes reading the NFO files.
	items := libraryv1connect.NewItemServiceClient(http.DefaultClient, url, token)
	list := func(c libraryv1connect.ItemServiceClient, b libraryv1.ListItemsRequest_builder) []*libraryv1.Item {
		t.Helper()
		resp, err := c.ListItems(ctx, b.Build())
		if err != nil {
			t.Fatalf("ListItems: %v", err)
		}
		return resp.GetItems()
	}
	byName := []*libraryv1.SortSpec{libraryv1.SortSpec_builder{Field: new(libraryv1.SortField_SORT_FIELD_NAME)}.Build()}
	kinds := func(k ...libraryv1.ItemKind) []libraryv1.ItemKind { return k }
	movie, series, episode := libraryv1.ItemKind_ITEM_KIND_MOVIE, libraryv1.ItemKind_ITEM_KIND_SERIES, libraryv1.ItemKind_ITEM_KIND_EPISODE
	for deadline := time.Now().Add(time.Minute); ; time.Sleep(100 * time.Millisecond) {
		if time.Now().After(deadline) {
			for _, it := range list(items, libraryv1.ListItemsRequest_builder{}) {
				t.Logf("%s %s rating %q genres %v", it.GetKind(), it.GetName(), it.GetOfficialRating(), it.GetGenres())
			}
			t.Fatal("the libraries were not scanned and refreshed within a minute")
		}
		rated := 0
		for _, it := range list(items, libraryv1.ListItemsRequest_builder{Kinds: kinds(movie, series)}) {
			if it.GetOfficialRating() != "" && len(it.GetGenres()) > 0 {
				rated++
			}
		}
		if rated == 4 && len(list(items, libraryv1.ListItemsRequest_builder{Kinds: kinds(episode)})) == 4 {
			break
		}
	}

	// The views, the latest of each library and the genres.
	views, err := libraries.ListLibraries(ctx, &libraryv1.ListLibrariesRequest{})
	if err != nil || len(views.GetLibraries()) != 2 {
		t.Fatalf("views = %v, %v", views, err)
	}
	latest := func(c libraryv1connect.ItemServiceClient, lib string) []string {
		t.Helper()
		resp, err := c.ListLatestItems(ctx, libraryv1.ListLatestItemsRequest_builder{LibraryIds: []string{lib}}.Build())
		if err != nil {
			t.Fatalf("ListLatestItems: %v", err)
		}
		out := names(resp.GetItems())
		slices.Sort(out)
		return out
	}
	if got := latest(items, libs["Shows"]); !slices.Equal(got, []string{"Bluey", "Lost"}) {
		t.Errorf("latest shows = %q, want the series of their new episodes", got)
	}
	if got := latest(items, libs["Films"]); !slices.Equal(got, []string{"Heat", "Up"}) {
		t.Errorf("latest films = %q", got)
	}
	genres := func(c libraryv1connect.ItemServiceClient) []string {
		t.Helper()
		resp, err := c.ListValues(ctx, libraryv1.ListValuesRequest_builder{Kind: new(libraryv1.ValueKind_VALUE_KIND_GENRE)}.Build())
		if err != nil {
			t.Fatalf("ListValues: %v", err)
		}
		var out []string
		for _, v := range resp.GetValues() {
			out = append(out, fmt.Sprintf("%s %d", v.GetValue(), v.GetItemCount()))
		}
		return out
	}
	if got := genres(items); !slices.Equal(got, []string{"Animation 2", "Crime 1", "Drama 1"}) {
		t.Errorf("genres = %q", got)
	}
	animation := list(items, libraryv1.ListItemsRequest_builder{Genres: []string{"animation"}, Sort: byName})
	if got := names(animation); !slices.Equal(got, []string{"Bluey", "Up"}) {
		t.Errorf("animation = %q", got)
	}

	// Next up follows the episode played.
	lost := list(items, libraryv1.ListItemsRequest_builder{Kinds: kinds(series), Search: new("lost")})[0]
	lostEpisodes := list(items, libraryv1.ListItemsRequest_builder{
		ParentId: new(lost.GetId()), Recursive: new(true), Kinds: kinds(episode),
		Sort: []*libraryv1.SortSpec{libraryv1.SortSpec_builder{Field: new(libraryv1.SortField_SORT_FIELD_INDEX)}.Build()},
	})
	data2 := userv1connect.NewUserDataServiceClient(http.DefaultClient, url, token)
	if _, err := data2.UpdateUserData(ctx, userv1.UpdateUserDataRequest_builder{ItemId: new(lostEpisodes[0].GetId()), Played: new(true)}.Build()); err != nil {
		t.Fatal(err)
	}
	next, err := items.ListNextUp(ctx, libraryv1.ListNextUpRequest_builder{}.Build())
	if err != nil || len(next.GetItems()) != 1 || next.GetItems()[0].GetId() != lostEpisodes[1].GetId() || next.GetItems()[0].GetSeriesName() != "Lost" {
		t.Errorf("next up = %v, %v; want Lost's second episode", next, err)
	}

	// A user allowed content rated up to PG sees the PG film and the
	// TV-Y series, not the R film or the episodes of the TV-14 series.
	users := userv1connect.NewUserServiceClient(http.DefaultClient, url, token)
	if _, err := users.CreateUser(ctx, userv1.CreateUserRequest_builder{
		Name: new("kid"), Password: new("pw"), Policy: userv1.UserPolicy_builder{AllLibraries: new(true), MaxParentalRating: new(int32(10))}.Build(),
	}.Build()); err != nil {
		t.Fatal(err)
	}
	signedIn, err := authv1connect.NewAuthServiceClient(http.DefaultClient, url).Login(ctx, authv1.LoginRequest_builder{
		Name: new("kid"), Password: new("pw"), Device: device("tablet"),
	}.Build())
	if err != nil {
		t.Fatal(err)
	}
	kidToken := withToken(signedIn.GetAccessToken())
	kid := libraryv1connect.NewItemServiceClient(http.DefaultClient, url, kidToken)
	if got := names(list(kid, libraryv1.ListItemsRequest_builder{Kinds: kinds(movie, series), Sort: byName})); !slices.Equal(got, []string{"Bluey", "Up"}) {
		t.Errorf("kid's films and series = %q", got)
	}
	if got := list(kid, libraryv1.ListItemsRequest_builder{Kinds: kinds(episode)}); len(got) != 2 || got[0].GetSeriesName() != "Bluey" || got[1].GetSeriesName() != "Bluey" {
		t.Errorf("kid's episodes = %v", got)
	}
	_, err = kid.GetItem(ctx, libraryv1.GetItemRequest_builder{Id: new(lostEpisodes[1].GetId())}.Build())
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("kid gets Lost's episode: %v, want not_found", err)
	}
	if got := latest(kid, libs["Shows"]); !slices.Equal(got, []string{"Bluey"}) {
		t.Errorf("kid's latest shows = %q", got)
	}
	if got := genres(kid); !slices.Equal(got, []string{"Animation 2"}) {
		t.Errorf("kid's genres = %q", got)
	}

	// The kid's playlist of Bluey holds its episodes; Lost cannot be added.
	bluey := list(kid, libraryv1.ListItemsRequest_builder{Kinds: kinds(series)})[0]
	playlists := libraryv1connect.NewPlaylistServiceClient(http.DefaultClient, url, kidToken)
	pl, err := playlists.CreatePlaylist(ctx, libraryv1.CreatePlaylistRequest_builder{Name: new("Bluey"), ItemIds: []string{bluey.GetId()}}.Build())
	if err != nil {
		t.Fatal(err)
	}
	inPlaylist := list(kid, libraryv1.ListItemsRequest_builder{ParentId: new(pl.GetPlaylist().GetId())})
	if got := names(inPlaylist); len(got) != 2 || inPlaylist[0].GetIndexNumber() != 1 || inPlaylist[1].GetIndexNumber() != 2 {
		t.Errorf("kid's playlist = %q", got)
	}
	_, err = playlists.AddToPlaylist(ctx, libraryv1.AddToPlaylistRequest_builder{Id: new(pl.GetPlaylist().GetId()), ItemIds: []string{lost.GetId()}}.Build())
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("kid adds Lost: %v, want not_found", err)
	}
}

func device(id string) *authv1.Device {
	return authv1.Device_builder{Id: &id, Name: new("Test"), Client: new("Test"), ClientVersion: new("0")}.Build()
}

func names(items []*libraryv1.Item) []string {
	out := []string{}
	for _, it := range items {
		out = append(out, it.GetName())
	}
	return out
}
