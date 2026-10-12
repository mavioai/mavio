package server_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"google.golang.org/protobuf/types/known/fieldmaskpb"

	"github.com/mavioai/mavio/apps/server/internal/server"
	authv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1/authv1connect"
	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1/libraryv1connect"
)

// mavioTestKey is the TMDB API key the test builds scraper-tmdb with, as
// release builds build in Mavio's own.
const mavioTestKey = "mavio-test-key"

// scraperBuilds builds the metadata plugins acquisition uses, once per
// test binary.
var scraperBuilds = map[string]func() (string, error){
	"scraper-tmdb":    sync.OnceValues(func() (string, error) { return buildScraper("scraper-tmdb", "-X main.mavioKey="+mavioTestKey) }),
	"scraper-servarr": sync.OnceValues(func() (string, error) { return buildScraper("scraper-servarr", "") }),
}

// buildScraper builds a WASM plugin of the repository into smokeRoot.
func buildScraper(name, ldflags string) (string, error) {
	out := filepath.Join(smokeRoot, name+".wasm")
	build := exec.Command("go", "build", "-buildmode=c-shared", "-ldflags", ldflags, "-o", out, ".")
	build.Dir = filepath.Join("..", "..", "..", "..", "plugins", name)
	build.Env = append(os.Environ(), "GOOS=wasip1", "GOARCH=wasm", "CGO_ENABLED=0")
	if msg, err := build.CombinedOutput(); err != nil {
		return "", fmt.Errorf("build %s: %w\n%s", name, err, msg)
	}
	return out, nil
}

// installScrapers installs the metadata plugins acquisition uses into a
// new plugin folder.
func installScrapers(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for name, build := range scraperBuilds {
		bin, err := build()
		if err != nil {
			t.Fatal(err)
		}
		dir := filepath.Join(root, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		copyFile(t, bin, filepath.Join(dir, "plugin.wasm"))
		copyFile(t, filepath.Join("..", "..", "..", "..", "plugins", name, "manifest.json"), filepath.Join(dir, "manifest.json"))
	}
	return root
}

// tmdbMode is how the fake TMDB answers.
type tmdbMode int

const (
	tmdbAcceptsRadarr tmdbMode = iota // Radarr's token works
	tmdbRejectsRadarr                 // only Mavio's key works
	tmdbDown                          // nothing works
)

// fakeServices answers the plugins' requests to TMDB, Radarr's metadata
// service and Sonarr's SkyHook as the hosts they name.
type fakeServices struct {
	mode tmdbMode
	// routes are answers of a test's own, by host and path, before
	// serviceRoutes.
	routes map[string]string

	mu      sync.Mutex
	unknown []string
	// served counts the requests answered, by host and path; queries
	// holds the query of the last one.
	served  map[string]int
	queries map[string]url.Values
	// keyed counts TMDB's answers to Mavio's key.
	keyed int
}

const radarrMatrix = `{"TmdbId": 603, "ImdbId": "tt0133093", "Title": "The Matrix", "Year": 1999,
	"Overview": "Radarr: a hacker learns the truth.", "Genres": ["Action"], "Runtime": 136}`

// The services' answers, by host and path. TMDB numbers the episode TVDB
// numbers S02E01 as S01E08.
var serviceRoutes = map[string]string{
	"api.radarr.video/v1/search": `[
		{"TmdbId": 604, "Title": "The Matrix Reloaded", "Year": 2003},` + radarrMatrix + `]`,
	"api.radarr.video/v1/movie/603":        radarrMatrix,
	"skyhook.sonarr.tv/v1/tvdb/search/en/": `[{"tvdbId": 81189, "title": "Breaking Bad", "firstAired": "2008-01-20"}]`,
	"skyhook.sonarr.tv/v1/tvdb/shows/en/81189": `{
		"tvdbId": 81189, "tmdbId": 1396, "imdbId": "tt0903747", "title": "Breaking Bad", "overview": "SkyHook: a teacher turns to crime.",
		"firstAired": "2008-01-20", "status": "Ended", "seasons": [{"seasonNumber": 1}, {"seasonNumber": 2}],
		"episodes": [
			{"tvdbId": 349232, "seasonNumber": 1, "episodeNumber": 1, "absoluteEpisodeNumber": 1, "title": "Pilot", "airDate": "2008-01-20"},
			{"tvdbId": 349235, "seasonNumber": 1, "episodeNumber": 2, "absoluteEpisodeNumber": 2, "title": "Cat's in the Bag...", "airDate": "2008-01-27"},
			{"tvdbId": 349236, "seasonNumber": 2, "episodeNumber": 1, "absoluteEpisodeNumber": 3, "title": "Seven Thirty-Seven", "airDate": "2009-03-08"}]}`,
	"api.themoviedb.org/3/movie/603": `{"id": 603, "title": "The Matrix", "overview": "TMDB: a hacker learns the truth.",
		"release_date": "1999-03-30", "external_ids": {"imdb_id": "tt0133093"}}`,
	"api.themoviedb.org/3/tv/1396": `{"id": 1396, "name": "Breaking Bad", "overview": "TMDB: a teacher turns to crime.",
		"first_air_date": "2008-01-20", "external_ids": {"imdb_id": "tt0903747", "tvdb_id": 81189}}`,
	"api.themoviedb.org/3/tv/1396/season/1": `{"id": 3572, "name": "Season 1", "season_number": 1}`,
	"api.themoviedb.org/3/tv/1396/season/2": `{"id": 3573, "name": "Season 2", "season_number": 2}`,
	"api.themoviedb.org/3/find/349232":      `{"tv_episode_results": [{"show_id": 1396, "season_number": 1, "episode_number": 1}]}`,
	"api.themoviedb.org/3/find/349236":      `{"tv_episode_results": [{"show_id": 1396, "season_number": 1, "episode_number": 8}]}`,
	"api.themoviedb.org/3/tv/1396/season/1/episode/1": `{"id": 62085, "name": "Pilot", "overview": "TMDB: Walter is diagnosed.",
		"external_ids": {"tvdb_id": 349232}}`,
	"api.themoviedb.org/3/tv/1396/season/1/episode/8": `{"id": 62092, "name": "Seven Thirty-Seven", "overview": "TMDB: Walt and Jesse face the aftermath.",
		"external_ids": {"tvdb_id": 349236}}`,
}

func (f *fakeServices) RoundTrip(req *http.Request) (*http.Response, error) {
	rec := httptest.NewRecorder()
	f.serve(rec, req)
	return rec.Result(), nil
}

func (f *fakeServices) serve(w http.ResponseWriter, r *http.Request) {
	if r.URL.Host == "api.themoviedb.org" {
		bearer := r.Header.Get("Authorization") != ""
		switch {
		case f.mode == tmdbDown:
			http.Error(w, "down", http.StatusServiceUnavailable)
			return
		case f.mode == tmdbRejectsRadarr && bearer:
			http.Error(w, `{"status_message": "Invalid API key"}`, http.StatusUnauthorized)
			return
		case !bearer && r.URL.Query().Get("api_key") != mavioTestKey:
			http.Error(w, `{"status_message": "Invalid API key"}`, http.StatusUnauthorized)
			return
		case !bearer:
			f.mu.Lock()
			f.keyed++
			f.mu.Unlock()
		}
	}
	body, ok := f.routes[r.URL.Host+r.URL.Path]
	if !ok {
		body, ok = serviceRoutes[r.URL.Host+r.URL.Path]
	}
	if ok {
		f.mu.Lock()
		if f.served == nil {
			f.served, f.queries = map[string]int{}, map[string]url.Values{}
		}
		f.served[r.URL.Host+r.URL.Path]++
		f.queries[r.URL.Host+r.URL.Path] = r.URL.Query()
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
		return
	}
	f.mu.Lock()
	f.unknown = append(f.unknown, r.URL.Host+r.URL.Path)
	f.mu.Unlock()
	http.NotFound(w, r)
}

// TestAcquisitionSources runs the assembled server with scraper-servarr
// and scraper-tmdb, without keys, against fakes of the services they use
// (P16's done criteria): a movie and a series are identified and
// refreshed whether TMDB accepts Radarr's token, rejects it so that
// Mavio's key is used, or is down; the series' wanted episodes are
// numbered as TVDB numbers them, and get TMDB's metadata by their TVDB
// IDs even where TMDB numbers them otherwise.
func TestAcquisitionSources(t *testing.T) {
	t.Parallel()
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("integration test: ffmpeg not found on PATH")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("integration test: ffprobe not found on PATH")
	}
	for _, tt := range []struct {
		name string
		mode tmdbMode
	}{
		{"TMDB accepts Radarr's token", tmdbAcceptsRadarr},
		{"TMDB rejects Radarr's token", tmdbRejectsRadarr},
		{"TMDB down", tmdbDown},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			testAcquisitionSources(t, ffmpeg, ffprobe, tt.mode)
		})
	}
}

func testAcquisitionSources(t *testing.T, ffmpeg, ffprobe string, mode tmdbMode) {
	ctx := t.Context()
	movies, shows := t.TempDir(), t.TempDir()
	season := filepath.Join(shows, "Breaking Bad", "Season 01")
	if err := os.MkdirAll(season, 0o755); err != nil {
		t.Fatal(err)
	}
	movie := filepath.Join(movies, "The Matrix (1999).mkv")
	episode := filepath.Join(season, "Breaking Bad S01E01.mkv")
	for _, file := range []string{movie, episode} {
		run(t, ffmpeg, "-loglevel", "error", "-f", "lavfi", "-i", "testsrc2=size=160x120:rate=24", "-t", "1", "-c:v", "libx264", file)
	}
	settled(t, movie, episode)

	fake := &fakeServices{mode: mode}
	t.Cleanup(func() {
		fake.mu.Lock()
		defer fake.mu.Unlock()
		if len(fake.unknown) > 0 {
			t.Logf("requests answered 404: %v", fake.unknown)
		}
	})
	srv := start(t, server.Config{
		Database: "sqlite:" + filepath.Join(t.TempDir(), "mavio.db"), FFmpeg: ffmpeg, FFprobe: ffprobe,
		TranscodeDir: t.TempDir(), CacheDir: t.TempDir(), MetadataDir: t.TempDir(),
		PluginDir: installScrapers(t), PluginFetch: &http.Client{Transport: fake},
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
	for _, spec := range []*libraryv1.LibrarySpec{
		libraryv1.LibrarySpec_builder{Name: new("Movies"), Kind: new(libraryv1.LibraryKind_LIBRARY_KIND_MOVIES), Paths: []string{movies}}.Build(),
		libraryv1.LibrarySpec_builder{Name: new("Shows"), Kind: new(libraryv1.LibraryKind_LIBRARY_KIND_SHOWS), Paths: []string{shows}}.Build(),
	} {
		if _, err := libraries.CreateLibrary(ctx, libraryv1.CreateLibraryRequest_builder{Spec: spec}.Build()); err != nil {
			t.Fatalf("CreateLibrary: %v", err)
		}
	}
	list := func(kind libraryv1.ItemKind) []*libraryv1.Item {
		t.Helper()
		resp, err := items.ListItems(ctx, libraryv1.ListItemsRequest_builder{
			Kinds: []libraryv1.ItemKind{kind}, IncludeVirtual: new(true),
			Sort: []*libraryv1.SortSpec{libraryv1.SortSpec_builder{Field: new(libraryv1.SortField_SORT_FIELD_INDEX)}.Build()},
		}.Build())
		if err != nil {
			t.Fatalf("ListItems: %v", err)
		}
		return resp.GetItems()
	}
	// TMDB's overviews win when TMDB answers; otherwise Servarr's stay.
	source := "TMDB: "
	if mode == tmdbDown {
		source = ""
	}
	overviewFrom := func(it *libraryv1.Item, radarr string) bool {
		if source == "" {
			return strings.HasPrefix(it.GetOverview(), radarr)
		}
		return strings.HasPrefix(it.GetOverview(), source)
	}
	describe := func(kind libraryv1.ItemKind) func() any {
		return func() any {
			var out []string
			for _, it := range list(kind) {
				out = append(out, fmt.Sprintf("%s %v %q", it.GetName(), it.GetExternalIds(), it.GetOverview()))
			}
			return out
		}
	}

	// The movie and the series are identified and refreshed.
	waitFor(t, "the movie identified", func() bool {
		m := list(libraryv1.ItemKind_ITEM_KIND_MOVIE)
		return len(m) == 1 && m[0].GetExternalIds()["tmdb"] == "603" && m[0].GetExternalIds()["imdb"] == "tt0133093" && overviewFrom(m[0], "Radarr: ")
	}, describe(libraryv1.ItemKind_ITEM_KIND_MOVIE))
	var series *libraryv1.Item
	waitFor(t, "the series identified", func() bool {
		s := list(libraryv1.ItemKind_ITEM_KIND_SERIES)
		if len(s) == 1 && s[0].GetExternalIds()["tvdb"] == "81189" && s[0].GetExternalIds()["tmdb"] == "1396" && overviewFrom(s[0], "SkyHook: ") {
			series = s[0]
			return true
		}
		return false
	}, describe(libraryv1.ItemKind_ITEM_KIND_SERIES))
	waitFor(t, "the episode refreshed", func() bool {
		e := list(libraryv1.ItemKind_ITEM_KIND_EPISODE)
		return len(e) == 1 && e[0].GetExternalIds()["tvdb"] == "349232" && (source == "" || overviewFrom(e[0], ""))
	}, describe(libraryv1.ItemKind_ITEM_KIND_EPISODE))

	// Monitoring the series lists the episodes it lacks as TVDB numbers
	// them.
	if _, err := meta.UpdateItem(ctx, libraryv1.UpdateItemRequest_builder{
		Id: new(series.GetId()), Metadata: libraryv1.ItemMetadata_builder{Monitored: new(true)}.Build(),
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"monitored"}},
	}.Build()); err != nil {
		t.Fatalf("UpdateItem: %v", err)
	}
	numbered := func() []string {
		var out []string
		for _, it := range list(libraryv1.ItemKind_ITEM_KIND_EPISODE) {
			out = append(out, fmt.Sprintf("S%02dE%02d %s virtual=%v", it.GetParentIndexNumber(), it.GetIndexNumber(), it.GetExternalIds()["tvdb"], it.GetVirtual()))
		}
		return out
	}
	want := []string{"S01E01 349232 virtual=false", "S01E02 349235 virtual=true", "S02E01 349236 virtual=true"}
	waitFor(t, "the wanted episodes", func() bool { return slices.Equal(numbered(), want) }, func() any { return numbered() })

	// A wanted episode TMDB numbers S01E08 gets TMDB's metadata by its
	// TVDB ID.
	var wanted *libraryv1.Item
	for _, it := range list(libraryv1.ItemKind_ITEM_KIND_EPISODE) {
		if it.GetExternalIds()["tvdb"] == "349236" {
			wanted = it
		}
	}
	resp, err := meta.RefreshItem(ctx, libraryv1.RefreshItemRequest_builder{Id: new(wanted.GetId())}.Build())
	if err != nil {
		t.Fatalf("RefreshItem: %v", err)
	}
	got := resp.GetItem()
	if got.GetName() != "Seven Thirty-Seven" || got.GetParentIndexNumber() != 2 || got.GetIndexNumber() != 1 {
		t.Errorf("refreshed episode = %q S%02dE%02d, want = Seven Thirty-Seven S02E01", got.GetName(), got.GetParentIndexNumber(), got.GetIndexNumber())
	}
	wantOverview := ""
	if source != "" {
		wantOverview = "TMDB: Walt and Jesse face the aftermath."
	}
	if got.GetOverview() != wantOverview {
		t.Errorf("refreshed episode overview = %q, want = %q", got.GetOverview(), wantOverview)
	}
	// Mavio's key is used once TMDB rejects Radarr's token, and only then.
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if keyed := fake.keyed > 0; keyed != (mode == tmdbRejectsRadarr) {
		t.Errorf("TMDB answered Mavio's key %d times in mode %d", fake.keyed, mode)
	}
}
