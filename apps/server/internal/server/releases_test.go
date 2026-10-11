package server_test

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"google.golang.org/protobuf/types/known/fieldmaskpb"

	"github.com/mavioai/mavio/apps/server/internal/server"
	acquisitionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1/acquisitionv1connect"
	authv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1/authv1connect"
	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1/libraryv1connect"
	systemv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1/systemv1connect"
)

// indexersSource is the indexers plugin's folder.
var indexersSource = filepath.Join("..", "..", "..", "..", "plugins", "indexers")

// indexersBuild builds the indexers plugin once per test binary.
var indexersBuild = sync.OnceValues(func() (string, error) {
	out := filepath.Join(smokeRoot, "indexers")
	build := exec.Command("go", "build", "-o", out, ".")
	build.Dir = indexersSource
	if msg, err := build.CombinedOutput(); err != nil {
		return "", fmt.Errorf("build the indexers plugin: %w\n%s", err, msg)
	}
	return out, nil
})

// installIndexers installs the indexers plugin into a new plugin folder.
func installIndexers(t *testing.T) string {
	t.Helper()
	bin, err := indexersBuild()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	dir := filepath.Join(root, "indexers")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	exe := "plugin"
	if filepath.Separator == '\\' {
		exe = "plugin.exe"
	}
	copyFile(t, bin, filepath.Join(dir, exe))
	if err := os.Chmod(filepath.Join(dir, exe), 0o755); err != nil {
		t.Fatal(err)
	}
	copyFile(t, filepath.Join(indexersSource, "manifest.json"), filepath.Join(dir, "manifest.json"))
	return root
}

// recorded returns a response Prowlarr's tests recorded.
func recorded(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(indexersSource, "testdata", "responses", filepath.FromSlash(name)))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// fakeIndexer answers an indexer's requests by path, the caps by the t
// parameter, and records the queries of the others.
type fakeIndexer struct {
	*httptest.Server
	mu      sync.Mutex
	queries []string
}

func newFakeIndexer(t *testing.T, contentType string, answer func(r *http.Request) []byte) *fakeIndexer {
	f := &fakeIndexer{}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("t") != "caps" {
			f.mu.Lock()
			f.queries = append(f.queries, r.URL.Path+"?"+r.URL.RawQuery)
			f.mu.Unlock()
		}
		body := answer(r)
		if body == nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", contentType)
		_, _ = w.Write(body)
	}))
	t.Cleanup(f.Close)
	return f
}

// took reports whether the indexer was sent a query with every part.
func (f *fakeIndexer) took(parts ...string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.ContainsFunc(f.queries, func(q string) bool {
		for _, p := range parts {
			if !strings.Contains(q, p) {
				return false
			}
		}
		return true
	})
}

func (f *fakeIndexer) sent() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.queries)
}

// newznabCaps are the caps of a Newznab indexer searching movies by IMDb
// ID and series by title, season and episode.
const newznabCaps = `<caps>
  <searching>
    <search available="yes" supportedParams="q"/>
    <tv-search available="yes" supportedParams="q,season,ep"/>
    <movie-search available="yes" supportedParams="q,imdbid"/>
  </searching>
  <categories>
    <category id="2000" name="Movies"/>
    <category id="5000" name="TV"/>
  </categories>
</caps>`

// torznabCaps are the caps of a Torznab indexer taking basic searches
// alone.
const torznabCaps = `<caps>
  <searching>
    <search available="yes" supportedParams="q"/>
    <tv-search available="no"/>
    <movie-search available="no"/>
  </searching>
  <categories>
    <category id="2000" name="Movies"><subcat id="2040" name="Movies/HD"/></category>
    <category id="5000" name="TV"/>
  </categories>
</caps>`

// e2eDefinition is a Cardigann definition of a public site at %s.
const e2eDefinition = `---
id: mavioe2e
name: Mavio E2E
description: A public site of the end-to-end test
language: en-US
type: public
links: ["%s/"]
caps:
  categorymappings:
    - {id: 1, cat: Movies, desc: "Movies"}
  modes:
    search: [q]
search:
  paths:
    - path: browse
  inputs:
    q: "{{ .Keywords }}"
  rows:
    selector: tr.row
  fields:
    category:
      text: 1
    title:
      selector: a.title
    details:
      selector: a.title
      attribute: href
    download:
      selector: a.dl
      attribute: href
    size:
      text: 2 GB
    seeders:
      text: 7
    leechers:
      text: 1
    date:
      text: now
`

// TestReleases runs the assembled server with the indexers plugin and
// indexers on the loopback interface answering with responses Prowlarr's
// tests recorded: ReleaseService searches Newznab, Torznab, a Cardigann
// definition fetched as indexers.prowlarr.com serves them and a site
// implemented in code, for a movie and an episode of the library by what
// each indexer can search by, and for text, and parses the releases.
func TestReleases(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("integration test: ffmpeg not found on PATH")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("integration test: ffprobe not found on PATH")
	}
	ctx := t.Context()
	movies, shows := t.TempDir(), t.TempDir()
	season := filepath.Join(shows, "White Collar (2009)", "Season 03")
	if err := os.MkdirAll(season, 0o755); err != nil {
		t.Fatal(err)
	}
	movie := filepath.Join(movies, "Out of the Past (1947).mkv")
	episode := filepath.Join(season, "White Collar S03E05.mkv")
	for _, file := range []string{movie, episode} {
		run(t, ffmpeg, "-loglevel", "error", "-f", "lavfi", "-i", "testsrc2=size=160x120:rate=24", "-t", "1", "-c:v", "libx264", file)
	}
	settled(t, movie, episode)

	// The Newznab indexer takes IMDb IDs for movies but no ID of series.
	newznab := newFakeIndexer(t, "application/rss+xml", func(r *http.Request) []byte {
		if r.URL.Query().Get("t") == "caps" {
			return []byte(newznabCaps)
		}
		return recorded(t, "Newznab/newznab_nzb_su.xml")
	})
	// The Torznab indexer takes basic searches alone.
	torznab := newFakeIndexer(t, "application/rss+xml", func(r *http.Request) []byte {
		if r.URL.Query().Get("t") == "caps" {
			return []byte(torznabCaps)
		}
		return recorded(t, "Torznab/torznab_morethantv.xml")
	})
	nzbindex := newFakeIndexer(t, "application/json", func(r *http.Request) []byte {
		if r.URL.Path == "/api/search" {
			return recorded(t, "NzbIndex/search.json")
		}
		return nil
	})
	site := newFakeIndexer(t, "text/html", func(r *http.Request) []byte {
		if r.URL.Path != "/browse" {
			return nil
		}
		return []byte(`<html><body><table>
<tr class="row"><td><a class="title" href="/t/1">Out.of.the.Past.1947.720p.WEB-DL.AAC2.0.H.264-CARD</a><a class="dl" href="/dl/1.torrent">get</a></td></tr>
</table></body></html>`)
	})
	var pkg bytes.Buffer
	zw := zip.NewWriter(&pkg)
	w, err := zw.Create("mavioe2e.yml")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Fprintf(w, e2eDefinition, site.URL)
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	definitions := newFakeIndexer(t, "application/zip", func(r *http.Request) []byte {
		if r.URL.Path == "/master/11/package.zip" {
			return pkg.Bytes()
		}
		return nil
	})

	srv := start(t, server.Config{
		Database: "sqlite:" + filepath.Join(t.TempDir(), "mavio.db"), FFmpeg: ffmpeg, FFprobe: ffprobe,
		TranscodeDir: t.TempDir(), CacheDir: t.TempDir(), MetadataDir: t.TempDir(),
		PluginDir: installIndexers(t), PluginDataDir: t.TempDir(),
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
	one := func(kind libraryv1.ItemKind) *libraryv1.Item {
		resp, err := items.ListItems(ctx, libraryv1.ListItemsRequest_builder{Kinds: []libraryv1.ItemKind{kind}}.Build())
		if err != nil {
			t.Fatalf("ListItems: %v", err)
		}
		if len(resp.GetItems()) != 1 {
			return nil
		}
		return resp.GetItems()[0]
	}
	var film, series, ep *libraryv1.Item
	waitFor(t, "the movie and the episode to be scanned", func() bool {
		film, series, ep = one(libraryv1.ItemKind_ITEM_KIND_MOVIE), one(libraryv1.ItemKind_ITEM_KIND_SERIES), one(libraryv1.ItemKind_ITEM_KIND_EPISODE)
		return film != nil && series != nil && ep != nil && ep.GetIndexNumber() == 5 && ep.GetParentIndexNumber() == 3
	})
	for id, ids := range map[string]map[string]string{
		film.GetId():   {"imdb": "tt0039689", "tmdb": "678"},
		series.GetId(): {"tvdb": "108611", "imdb": "tt1358522"},
	} {
		if _, err := meta.UpdateItem(ctx, libraryv1.UpdateItemRequest_builder{
			Id: new(id), Metadata: libraryv1.ItemMetadata_builder{ExternalIds: ids}.Build(),
			UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"external_ids"}},
		}.Build()); err != nil {
			t.Fatalf("UpdateItem: %v", err)
		}
	}

	cfg, err := json.Marshal(map[string]any{
		"indexers": []any{
			map[string]any{"id": "nzbsu", "definition": "newznab", "priority": 10, "settings": map[string]any{"base_url": newznab.URL, "api_key": "key"}},
			map[string]any{"id": "mtv", "definition": "torznab", "settings": map[string]any{"base_url": torznab.URL, "api_key": "key"}},
			map[string]any{"id": "nzbindex", "definition": "nzbindex", "settings": map[string]any{"base_url": nzbindex.URL, "api_key": "key"}},
			map[string]any{"id": "e2e", "definition": "mavioe2e"},
		},
		"definitions": map[string]any{"url": definitions.URL + "/master/11"},
	})
	if err != nil {
		t.Fatal(err)
	}
	system := systemv1connect.NewSystemServiceClient(http.DefaultClient, srv.url, token)
	if _, err := system.SetPluginConfig(ctx, systemv1.SetPluginConfigRequest_builder{
		PluginId: new("org.mavio.indexers"), ConfigJson: new(string(cfg)),
	}.Build()); err != nil {
		t.Fatalf("SetPluginConfig: %v", err)
	}

	releases := acquisitionv1connect.NewReleaseServiceClient(http.DefaultClient, srv.url, token)
	var indexers []string
	waitFor(t, "the indexers, the Cardigann one once its definition is fetched", func() bool {
		resp, err := releases.ListIndexers(ctx, &acquisitionv1.ListIndexersRequest{})
		if err != nil {
			return false
		}
		indexers = nil
		for _, ix := range resp.GetIndexers() {
			indexers = append(indexers, fmt.Sprintf("%s/%s %d", ix.GetPluginId(), ix.GetId(), ix.GetPriority()))
		}
		return len(indexers) == 4
	}, func() any { return indexers })
	if want := "org.mavio.indexers/nzbsu 10"; indexers[0] != want {
		t.Errorf("first indexer = %q, want = %q, the one of the lowest priority", indexers[0], want)
	}

	search := func(req *acquisitionv1.SearchReleasesRequest) map[string]*acquisitionv1.Release {
		t.Helper()
		resp, err := releases.SearchReleases(ctx, req)
		if err != nil {
			t.Fatalf("SearchReleases: %v", err)
		}
		for _, f := range resp.GetFailures() {
			t.Errorf("indexer %s failed: %s", f.GetIndexerId(), f.GetMessage())
		}
		found := map[string]*acquisitionv1.Release{}
		for _, r := range resp.GetReleases() {
			found[r.GetIndexerId()+" "+r.GetTitle()] = r
		}
		return found
	}
	parsed := func(r *acquisitionv1.Release) string {
		p := r.GetParsed()
		return fmt.Sprintf("%s %q %d S%dE%v %s %s", p.GetKind(), p.GetTitles(), p.GetYear(), p.GetSeason(), p.GetEpisodes(),
			p.GetQuality().GetName(), p.GetReleaseGroup())
	}

	// The movie: by its IMDb ID where the caps take it, else by its title
	// and year.
	found := search(acquisitionv1.SearchReleasesRequest_builder{ItemId: new(film.GetId())}.Build())
	for key, want := range map[string]string{
		"mtv Out of the Past 1947 1080p USA Blu-ray AVC DTS-HD MA 2.0-PCH": `RELEASE_KIND_MOVIE ["Out of the Past"] 1947 S0E[] BR-DISK PCH`,
		"e2e Out.of.the.Past.1947.720p.WEB-DL.AAC2.0.H.264-CARD":           `RELEASE_KIND_MOVIE ["Out of the Past"] 1947 S0E[] WEBDL-720p CARD`,
	} {
		r := found[key]
		if r == nil {
			t.Errorf("release %q not found in %v", key, slices.Sorted(mapsKeys(found)))
			continue
		}
		if got := parsed(r); got != want {
			t.Errorf("%s parsed = %s, want = %s", key, got, want)
		}
	}
	if r := found["mtv Out of the Past 1947 1080p USA Blu-ray AVC DTS-HD MA 2.0-PCH"]; r != nil && (r.GetSeeders() == 0 || r.GetSize() == 0) {
		t.Errorf("Torznab release lacks seeders or size: %v", r)
	}
	if !newznab.took("t=movie", "imdbid=0039689") {
		t.Errorf("Newznab queries = %q, want = a movie search by IMDb ID", newznab.sent())
	}
	if !torznab.took("t=search", "q=Out+of+the+Past+1947") {
		t.Errorf("Torznab queries = %q, want = a basic search of the title and year", torznab.sent())
	}
	if !site.took("q=Out+of+the+Past+1947") {
		t.Errorf("Cardigann queries = %q, want = a search of the title and year", site.sent())
	}

	// The episode: by its series' title with its season and number, as
	// the caps take no ID of the series.
	found = search(acquisitionv1.SearchReleasesRequest_builder{ItemId: new(ep.GetId())}.Build())
	key := "nzbsu White.Collar.S03E05.720p.HDTV.X264-DIMENSION"
	if r := found[key]; r == nil {
		t.Errorf("release %q not found in %v", key, slices.Sorted(mapsKeys(found)))
	} else if got, want := parsed(r), `RELEASE_KIND_EPISODE ["White Collar"] 0 S3E[5] HDTV-720p DIMENSION`; got != want {
		t.Errorf("%s parsed = %s, want = %s", key, got, want)
	}
	if !newznab.took("t=tvsearch", "q=White+Collar", "season=3", "ep=5") {
		t.Errorf("Newznab queries = %q, want = a series search by title, season and episode", newznab.sent())
	}

	// Text, of one indexer implemented in code.
	found = search(acquisitionv1.SearchReleasesRequest_builder{
		Query: new("ubuntu"), Indexers: []*acquisitionv1.IndexerRef{
			acquisitionv1.IndexerRef_builder{PluginId: new("org.mavio.indexers"), IndexerId: new("nzbindex")}.Build(),
		},
	}.Build())
	if len(found) == 0 {
		t.Fatal("NzbIndex found nothing")
	}
	for key, r := range found {
		if !strings.HasPrefix(key, "nzbindex ") || r.GetProtocol() != acquisitionv1.DownloadProtocol_DOWNLOAD_PROTOCOL_USENET || r.GetDownloadUrl() == "" {
			t.Errorf("release %q = %v, want = a Usenet release of NzbIndex", key, r)
		}
	}
	if !nzbindex.took("/api/search", "q=ubuntu") {
		t.Errorf("NzbIndex queries = %q, want = a search of ubuntu", nzbindex.sent())
	}

	// Only administrators search.
	anonymous := acquisitionv1connect.NewReleaseServiceClient(http.DefaultClient, srv.url)
	if _, err := anonymous.ListIndexers(ctx, &acquisitionv1.ListIndexersRequest{}); err == nil {
		t.Error("ListIndexers without a token succeeded, want = unauthenticated")
	}
}

func mapsKeys[V any](m map[string]V) func(func(string) bool) {
	return func(yield func(string) bool) {
		for k := range m {
			if !yield(k) {
				return
			}
		}
	}
}
