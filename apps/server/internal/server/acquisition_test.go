package server_test

import (
	"bytes"
	"crypto/sha1" //nolint:gosec // made-up BitTorrent v1 info hashes.
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net"
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
	"time"

	"github.com/mavioai/mavio/apps/server/internal/server"
	"github.com/mavioai/mavio/libs/acquire"
	"github.com/mavioai/mavio/libs/core"
	acquisitionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1/acquisitionv1connect"
	authv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1/authv1connect"
	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1/libraryv1connect"
	playbackv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/playback/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/playback/v1/playbackv1connect"
	systemv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1/systemv1connect"
	"github.com/mavioai/mavio/libs/release"
)

// downloadClientsSource is the download clients plugin's folder.
var downloadClientsSource = filepath.Join("..", "..", "..", "..", "plugins", "download-clients")

// downloadClientsBuild builds the download clients plugin once per test
// binary.
var downloadClientsBuild = sync.OnceValues(func() (string, error) {
	out := filepath.Join(smokeRoot, "download-clients")
	build := exec.Command("go", "build", "-o", out, ".")
	build.Dir = downloadClientsSource
	if msg, err := build.CombinedOutput(); err != nil {
		return "", fmt.Errorf("build the download clients plugin: %w\n%s", err, msg)
	}
	return out, nil
})

// installProcessPlugin installs a process plugin built from source into
// the plugin folder root.
func installProcessPlugin(t *testing.T, root, name, source string, build func() (string, error)) {
	t.Helper()
	bin, err := build()
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, name)
	exe := "plugin"
	if filepath.Separator == '\\' {
		exe = "plugin.exe"
	}
	copyFile(t, bin, filepath.Join(dir, exe))
	if err := os.Chmod(filepath.Join(dir, exe), 0o755); err != nil {
		t.Fatal(err)
	}
	copyFile(t, filepath.Join(source, "manifest.json"), filepath.Join(dir, "manifest.json"))
}

// offered is a release the fake Newznab indexer lists.
type offered struct {
	id, title string
	size      int64
	published time.Time
	// categories are the release's Newznab categories, Movies > HD when
	// empty.
	categories []int
	// attrs are the release's further Newznab attributes, such as tvdbid,
	// season and episode; The Matrix's IMDb ID when nil.
	attrs [][2]string
}

// newznabCategoryNames are the names of the Newznab categories releases
// are offered in.
var newznabCategoryNames = map[int]string{2000: "Movies", 2040: "Movies &gt; HD", 5000: "TV", 5040: "TV &gt; HD", 5070: "TV &gt; Anime"}

// fakeNewznab is a Newznab indexer listing the releases offered, for
// every search and its RSS feed alike, and serving their NZBs; or, when
// torznab is set, a Torznab indexer listing them with their magnet links.
type fakeNewznab struct {
	*httptest.Server
	torznab bool

	mu       sync.Mutex
	releases []offered
	// queries are the queries of the searches and RSS feeds asked for.
	queries []url.Values
}

// newFakeTorznab returns a Torznab indexer listing the releases offered,
// with their magnet links and fifty seeders.
func newFakeTorznab(t *testing.T) *fakeNewznab {
	f := &fakeNewznab{torznab: true}
	f.Server = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.Close)
	return f
}

// infoHash is the info hash of a release's torrent, in lower-case hex.
func infoHash(id string) string {
	sum := sha1.Sum([]byte(id)) //nolint:gosec // a made-up BitTorrent v1 info hash.
	return hex.EncodeToString(sum[:])
}

// magnet is the magnet link of a release's torrent.
func magnet(rel offered) string {
	return "magnet:?xt=urn:btih:" + infoHash(rel.id) + "&dn=" + url.QueryEscape(rel.title)
}

// searched returns the queries asked for so far.
func (f *fakeNewznab) searched() []url.Values {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.queries)
}

func newFakeNewznab(t *testing.T) *fakeNewznab {
	f := &fakeNewznab{}
	f.Server = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.Close)
	return f
}

// offer adds releases to those listed.
func (f *fakeNewznab) offer(rs ...offered) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.releases = append(f.releases, rs...)
}

// nzb is the NZB of a release.
func nzb(id string) []byte {
	return fmt.Appendf(nil, `<?xml version="1.0" encoding="utf-8"?>
<!DOCTYPE nzb PUBLIC "-//newzBin//DTD NZB 1.1//EN" "http://www.newzbin.com/DTD/nzb/nzb-1.1.dtd">
<nzb xmlns="http://www.newzbin.com/DTD/2003/nzb">
  <file poster="e2e@mavio" date="1700000000" subject="%[1]s.mkv (1/1)">
    <groups><group>alt.binaries.mavio</group></groups>
    <segments><segment bytes="1024" number="1">%[1]s@mavio</segment></segments>
  </file>
</nzb>`, id)
}

func (f *fakeNewznab) serve(w http.ResponseWriter, r *http.Request) {
	if id, ok := strings.CutPrefix(r.URL.Path, "/getnzb/"); ok {
		w.Header().Set("Content-Type", "application/x-nzb")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.nzb"`, f.title(strings.TrimSuffix(id, ".nzb"))))
		_, _ = w.Write(nzb(strings.TrimSuffix(id, ".nzb")))
		return
	}
	if r.URL.Path != "/api" || r.URL.Query().Get("apikey") != "nzbkey" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/rss+xml")
	if r.URL.Query().Get("t") == "caps" {
		_, _ = w.Write([]byte(newznabCaps))
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queries = append(f.queries, r.URL.Query())
	ns, namespace, enclosure := "newznab", "http://www.newznab.com/DTD/2010/feeds/attributes/", "application/x-nzb"
	if f.torznab {
		ns, namespace, enclosure = "torznab", "http://torznab.com/schemas/2015/feed", "application/x-bittorrent"
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<?xml version="1.0" encoding="utf-8"?>
<rss version="2.0" xmlns:atom="http://www.w3.org/2005/Atom" xmlns:%s="%s">
<channel><title>E2E</title>`, ns, namespace)
	fmt.Fprintf(&b, `<%s:response offset="0" total="%d"/>`, ns, len(f.releases))
	for _, rel := range f.releases {
		link := f.URL + "/getnzb/" + rel.id + ".nzb"
		if f.torznab {
			link = f.URL + "/gettorrent/" + rel.id + ".torrent"
		}
		categories, attrs := rel.categories, rel.attrs
		if len(categories) == 0 {
			categories = []int{2000, 2040}
		}
		if attrs == nil {
			attrs = [][2]string{{"imdb", "0133093"}}
		}
		fmt.Fprintf(&b, `<item><title>%s</title><guid isPermaLink="true">%s/details/%s</guid><link>%s</link>
<pubDate>%s</pubDate><category>%s</category>
<enclosure url="%s" length="%d" type="%s"/>
`, html.EscapeString(rel.title), f.URL, rel.id, link, rel.published.Format(time.RFC1123Z), newznabCategoryNames[categories[len(categories)-1]], link, rel.size, enclosure)
		for _, c := range categories {
			fmt.Fprintf(&b, `<%s:attr name="category" value="%d"/>`, ns, c)
		}
		fmt.Fprintf(&b, `<%[1]s:attr name="size" value="%[2]d"/><%[1]s:attr name="guid" value="%[3]s"/>`, ns, rel.size, rel.id)
		if f.torznab {
			attrs = append(slices.Clip(attrs), [][2]string{{"magneturl", magnet(rel)}, {"infohash", infoHash(rel.id)}, {"seeders", "50"}, {"peers", "60"}}...)
		}
		for _, a := range attrs {
			fmt.Fprintf(&b, `<%s:attr name="%s" value="%s"/>`, ns, a[0], html.EscapeString(a[1]))
		}
		b.WriteString("\n</item>")
	}
	b.WriteString(`</channel></rss>`)
	_, _ = w.Write([]byte(b.String()))
}

func (f *fakeNewznab) title(id string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.releases {
		if r.id == id {
			return r.title
		}
	}
	return id
}

// sabJob is a download of the fake SABnzbd.
type sabJob struct {
	id, name, category string
	nzb                []byte
	// state is "Downloading", "Completed" or "Failed".
	state, failMessage string
	deleted            bool
}

// fakeSABnzbd answers SABnzbd's API as SABnzbd 4.3 does, for the modes
// the download clients plugin uses: jobs added are downloading until the
// test completes or fails them, completed ones in a job folder of their
// category below the complete folder.
type fakeSABnzbd struct {
	*httptest.Server
	complete string

	mu   sync.Mutex
	jobs []*sabJob
}

const sabAPIKey = "sabkey"

func newFakeSABnzbd(t *testing.T, complete string) *fakeSABnzbd {
	f := &fakeSABnzbd{complete: complete}
	f.Server = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.Close)
	return f
}

func (f *fakeSABnzbd) serve(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if r.URL.Path != "/api" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if q.Get("apikey") != sabAPIKey {
		_ = json.NewEncoder(w).Encode(map[string]any{"status": false, "error": "API Key Incorrect"})
		return
	}
	answer := func(v any) { _ = json.NewEncoder(w).Encode(v) }
	f.mu.Lock()
	defer f.mu.Unlock()
	switch q.Get("mode") {
	case "version":
		answer(map[string]any{"version": "4.3.2"})
	case "get_config":
		answer(map[string]any{"config": map[string]any{
			"misc": map[string]any{
				"complete_dir": f.complete, "pre_check": 0, "enable_tv_sorting": 0, "enable_movie_sorting": 0, "enable_date_sorting": 0,
				"history_retention_option": "all", "history_retention_number": 1,
			},
			"categories": []any{
				map[string]any{"name": "*", "dir": ""},
				map[string]any{"name": "movies", "dir": "movies"},
				map[string]any{"name": "tv", "dir": "tv"},
			},
			"sorters": []any{},
		}})
	case "fullstatus":
		answer(map[string]any{"status": map[string]any{"completedir": f.complete}})
	case "addfile":
		file, header, err := r.FormFile("name")
		if err != nil {
			answer(map[string]any{"status": false, "error": err.Error()})
			return
		}
		data, _ := io.ReadAll(file)
		job := &sabJob{
			id: fmt.Sprintf("SABnzbd_nzo_%d", len(f.jobs)+1), name: strings.TrimSuffix(header.Filename, ".nzb"),
			category: q.Get("cat"), nzb: data, state: "Downloading",
		}
		f.jobs = append(f.jobs, job)
		answer(map[string]any{"status": true, "nzo_ids": []string{job.id}})
	case "queue", "history":
		if q.Get("name") == "delete" {
			for _, j := range f.jobs {
				if j.id == q.Get("value") {
					j.deleted = true
				}
			}
			answer(map[string]any{"status": true})
			return
		}
		var slots []any
		for _, j := range f.jobs {
			if j.deleted || (q.Get("category") != "" && q.Get("category") != j.category) {
				continue
			}
			switch {
			case q.Get("mode") == "queue" && j.state == "Downloading":
				slots = append(slots, map[string]any{
					"status": "Downloading", "timeleft": "0:10:00", "mb": "6000.00", "mbleft": "1500.00",
					"filename": j.name, "priority": "Normal", "cat": j.category, "nzo_id": j.id,
				})
			case q.Get("mode") == "history" && j.state != "Downloading":
				slots = append(slots, map[string]any{
					"status": j.state, "fail_message": j.failMessage, "bytes": 6000 << 20, "category": j.category,
					"storage": filepath.Join(f.complete, j.category, j.name), "nzo_id": j.id, "name": j.name,
				})
			}
		}
		if q.Get("mode") == "queue" {
			answer(map[string]any{"queue": map[string]any{"paused": false, "my_home": "/home/sab", "slots": slots}})
		} else {
			answer(map[string]any{"history": map[string]any{"slots": slots}})
		}
	default:
		answer(map[string]any{"status": false, "error": "not implemented"})
	}
}

// job returns the job of a release by its NZB, or nil.
func (f *fakeSABnzbd) job(id string) *sabJob {
	want := nzb(id)
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, j := range f.jobs {
		if bytes.Equal(j.nzb, want) {
			return j
		}
	}
	return nil
}

// finish completes a job with a video file in its job folder, as SABnzbd
// leaves a download it unpacked, or fails it with a message.
func (f *fakeSABnzbd) finish(t *testing.T, j *sabJob, video, failure string) {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if failure != "" {
		j.state, j.failMessage = "Failed", failure
		return
	}
	f.completeLocked(t, j, video, j.name+".mkv")
}

// completeWith completes a job with a copy of a video file under each
// name in its job folder, as SABnzbd leaves a season pack it unpacked.
func (f *fakeSABnzbd) completeWith(t *testing.T, j *sabJob, video string, names ...string) {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	f.completeLocked(t, j, video, names...)
}

// completeLocked completes a job with its files; f.mu is held.
func (f *fakeSABnzbd) completeLocked(t *testing.T, j *sabJob, video string, names ...string) {
	t.Helper()
	for _, name := range names {
		copyFile(t, video, filepath.Join(f.complete, j.category, j.name, name))
	}
	j.state = "Completed"
}

func (f *fakeSABnzbd) removed(j *sabJob) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return j.deleted
}

// profileItemsToProto returns the items of a quality profile.
func profileItemsToProto(items []core.QualityProfileItem) []*acquisitionv1.QualityProfileItem {
	out := make([]*acquisitionv1.QualityProfileItem, len(items))
	for i, it := range items {
		out[i] = acquisitionv1.QualityProfileItem_builder{
			Id: new(int32(it.ID)), Name: new(it.Name), Quality: new(int32(it.Quality)),
			Items: profileItemsToProto(it.Items), Allowed: new(it.Allowed),
		}.Build()
	}
	return out
}

// TestAcquisition runs the assembled server with the indexers and the
// download clients plugins against a Newznab indexer and SABnzbd faked on
// the loopback interface, and the metadata plugins against fakes of their
// services (P18's done criteria): a wanted movie is found, sent to
// SABnzbd, tracked while it downloads, imported under Radarr's naming
// once SABnzbd completes it, and played; a better release an RSS sync
// finds upgrades it, the replaced file going to the recycle bin; a
// download SABnzbd fails is blocklisted and replaced by a new search,
// which upgrades the movie to its cutoff.
func TestAcquisition(t *testing.T) {
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

	// The films the downloads hold: longer than ten minutes, so that they
	// are no samples of a movie of The Matrix's runtime, with audio, and
	// of the resolutions of the releases, which imports read the quality
	// from.
	film, film1080 := filepath.Join(t.TempDir(), "film.mkv"), filepath.Join(t.TempDir(), "film1080.mkv")
	for path, size := range map[string]string{film: "1280x720", film1080: "1920x1080"} {
		run(t, ffmpeg, "-loglevel", "error", "-f", "lavfi", "-i", "testsrc2=size="+size+":rate=1",
			"-f", "lavfi", "-i", "sine=frequency=440:sample_rate=8000", "-t", "610",
			"-c:v", "libx264", "-preset", "ultrafast", "-c:a", "aac", "-b:a", "16k", path)
		settled(t, path)
	}

	movies, recycle := t.TempDir(), t.TempDir()
	indexer := newFakeNewznab(t)
	sab := newFakeSABnzbd(t, t.TempDir())
	fake := &fakeServices{mode: tmdbAcceptsRadarr}
	pluginDir := installScrapers(t)
	installProcessPlugin(t, pluginDir, "indexers", indexersSource, indexersBuild)
	installProcessPlugin(t, pluginDir, "download-clients", downloadClientsSource, downloadClientsBuild)
	srv := start(t, server.Config{
		Database: "sqlite:" + filepath.Join(t.TempDir(), "mavio.db"), FFmpeg: ffmpeg, FFprobe: ffprobe,
		TranscodeDir: t.TempDir(), CacheDir: t.TempDir(), MetadataDir: t.TempDir(),
		PluginDir: pluginDir, PluginDataDir: t.TempDir(), PluginFetch: &http.Client{Transport: fake},
	})
	first, err := authv1connect.NewAuthServiceClient(http.DefaultClient, srv.url).CreateFirstUser(ctx, authv1.CreateFirstUserRequest_builder{
		Name: new("admin"), Password: new("secret"), Device: device("e2e"),
	}.Build())
	if err != nil {
		t.Fatalf("CreateFirstUser: %v", err)
	}
	token := withToken(first.GetAccessToken())
	system := systemv1connect.NewSystemServiceClient(http.DefaultClient, srv.url, token)
	tasks := systemv1connect.NewTaskServiceClient(http.DefaultClient, srv.url, token)
	libraries := libraryv1connect.NewLibraryServiceClient(http.DefaultClient, srv.url, token)
	items := libraryv1connect.NewItemServiceClient(http.DefaultClient, srv.url, token)
	playback := playbackv1connect.NewPlaybackServiceClient(http.DefaultClient, srv.url, token)
	profiles := acquisitionv1connect.NewProfileServiceClient(http.DefaultClient, srv.url, token)
	wanted := acquisitionv1connect.NewWantedServiceClient(http.DefaultClient, srv.url, token)
	downloads := acquisitionv1connect.NewDownloadServiceClient(http.DefaultClient, srv.url, token)
	releases := acquisitionv1connect.NewReleaseServiceClient(http.DefaultClient, srv.url, token)

	// The administrator sets acquisition up: a recycle bin, renaming
	// movies; RSS syncs only when asked, so that none runs between the
	// steps.
	got, err := system.GetServerSettings(ctx, &systemv1.GetServerSettingsRequest{})
	if err != nil {
		t.Fatalf("GetServerSettings: %v", err)
	}
	settings := got.GetSettings()
	acq := settings.GetAcquisition()
	acq.SetRecycleBin(recycle)
	acq.SetRssSyncInterval(0)
	acq.SetSkipFreeSpaceCheck(true)
	acq.SetRenameMovies(true)
	if !acq.GetCompletedDownloadHandling() || !acq.GetAutoRedownloadFailed() || !acq.GetAutoRedownloadFailedFromInteractiveSearch() {
		t.Errorf("default acquisition settings = %v, want = completed download handling and redownloading failed", acq)
	}
	if _, err := system.UpdateServerSettings(ctx, systemv1.UpdateServerSettingsRequest_builder{Settings: settings}.Build()); err != nil {
		t.Fatalf("UpdateServerSettings: %v", err)
	}
	lib, err := libraries.CreateLibrary(ctx, libraryv1.CreateLibraryRequest_builder{
		Spec: libraryv1.LibrarySpec_builder{Name: new("Movies"), Kind: new(libraryv1.LibraryKind_LIBRARY_KIND_MOVIES), Paths: []string{movies}}.Build(),
	}.Build())
	if err != nil {
		t.Fatalf("CreateLibrary: %v", err)
	}
	for id, cfg := range map[string]any{
		"org.mavio.indexers": map[string]any{"indexers": []any{
			map[string]any{"id": "nzb", "definition": "newznab", "settings": map[string]any{"base_url": indexer.URL, "api_key": "nzbkey"}},
		}},
		"org.mavio.download-clients": map[string]any{"clients": []any{
			map[string]any{"id": "sab", "name": "SABnzbd", "definition": "sabnzbd", "settings": map[string]any{
				"host": "127.0.0.1", "port": sab.Listener.Addr().(*net.TCPAddr).Port, "api_key": sabAPIKey, "movie_category": "movies", "tv_category": "tv",
			}},
		}},
	} {
		data, err := json.Marshal(cfg)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := system.SetPluginConfig(ctx, systemv1.SetPluginConfigRequest_builder{PluginId: new(id), ConfigJson: new(string(data))}.Build()); err != nil {
			t.Fatalf("SetPluginConfig %s: %v", id, err)
		}
	}
	waitFor(t, "the indexer and the download client", func() bool {
		ix, err := releases.ListIndexers(ctx, &acquisitionv1.ListIndexersRequest{})
		if err != nil || len(ix.GetIndexers()) != 1 {
			return false
		}
		dc, err := downloads.ListDownloadClients(ctx, &acquisitionv1.ListDownloadClientsRequest{})
		return err == nil && len(dc.GetClients()) == 1 && dc.GetClients()[0].GetProtocol() == acquisitionv1.DownloadProtocol_DOWNLOAD_PROTOCOL_USENET
	})

	// A profile upgrading up to Bluray-1080p, from HDTV-720p.
	hd := acquire.NewProfile(release.Movie, "HD", release.Bluray1080p, []release.Quality{release.HDTV720p, release.WEBDL1080p, release.Bluray1080p}, nil)
	stored, err := acquire.QualityProfileToCore(hd, core.KindMovie)
	if err != nil {
		t.Fatal(err)
	}
	profile, err := profiles.CreateQualityProfile(ctx, acquisitionv1.CreateQualityProfileRequest_builder{
		Profile: acquisitionv1.QualityProfile_builder{
			Kind: new(acquisitionv1.MediaKind_MEDIA_KIND_MOVIE), Name: new("HD"), UpgradeAllowed: new(true),
			Cutoff: new(int32(stored.Cutoff)), Items: profileItemsToProto(stored.Items),
		}.Build(),
	}.Build())
	if err != nil {
		t.Fatalf("CreateQualityProfile: %v", err)
	}

	// The movie is added by its TMDB ID and identified.
	added, err := wanted.AddMovie(ctx, acquisitionv1.AddMovieRequest_builder{
		LibraryId: new(lib.GetLibrary().GetId()), TmdbId: new(int32(603)), QualityProfileId: new(profile.GetProfile().GetId()),
	}.Build())
	if err != nil {
		t.Fatalf("AddMovie: %v", err)
	}
	movieID := added.GetItemId()
	item := func() *libraryv1.Item {
		t.Helper()
		resp, err := items.GetItem(ctx, libraryv1.GetItemRequest_builder{Id: new(movieID)}.Build())
		if err != nil {
			t.Fatalf("GetItem: %v", err)
		}
		return resp.GetItem()
	}
	if it := item(); it.GetName() != "The Matrix" || it.GetProductionYear() != 1999 || !it.GetVirtual() || it.GetExternalIds()["imdb"] != "tt0133093" {
		t.Fatalf("added movie = %q (%d) virtual=%v %v, want = The Matrix (1999), virtual, identified", it.GetName(), it.GetProductionYear(), it.GetVirtual(), it.GetExternalIds())
	}

	// What the server tells of the movie's downloads.
	runQueueCheck := func() {
		t.Helper()
		if _, err := tasks.RunTask(ctx, systemv1.RunTaskRequest_builder{Id: new("acquisition.queue")}.Build()); err != nil {
			t.Fatalf("RunTask: %v", err)
		}
	}
	queue := func() []*acquisitionv1.QueueItem {
		t.Helper()
		resp, err := downloads.ListQueue(ctx, acquisitionv1.ListQueueRequest_builder{ItemId: new(movieID), IncludeFinished: new(true)}.Build())
		if err != nil {
			t.Fatalf("ListQueue: %v", err)
		}
		return resp.GetItems()
	}
	describeQueue := func() any {
		var out []string
		for _, q := range queue() {
			out = append(out, fmt.Sprintf("%s %s %s %v %q", q.GetTitle(), q.GetState(), q.GetStatus(), q.GetRemainingSize(), q.GetMessages()))
		}
		return out
	}
	download := func(title string) *acquisitionv1.QueueItem {
		for _, q := range queue() {
			if q.GetTitle() == title {
				return q
			}
		}
		return nil
	}
	history := func(types ...acquisitionv1.HistoryEventType) []*acquisitionv1.HistoryEvent {
		t.Helper()
		resp, err := downloads.ListHistory(ctx, acquisitionv1.ListHistoryRequest_builder{ItemId: new(movieID), Types: types}.Build())
		if err != nil {
			t.Fatalf("ListHistory: %v", err)
		}
		return resp.GetEvents()
	}
	// grabbed waits for a release to be sent to SABnzbd and tracked, and
	// returns its job.
	grabbed := func(rel offered, quality string) *sabJob {
		t.Helper()
		var q *acquisitionv1.QueueItem
		waitFor(t, rel.title+" grabbed", func() bool {
			q = download(rel.title)
			return q != nil && sab.job(rel.id) != nil
		}, describeQueue)
		if q.GetState() != acquisitionv1.DownloadState_DOWNLOAD_STATE_DOWNLOADING || q.GetQuality() != quality ||
			q.GetClientPluginId() != "org.mavio.download-clients" || q.GetClientId() != "sab" || q.GetDownloadId() != sab.job(rel.id).id ||
			q.GetProtocol() != acquisitionv1.DownloadProtocol_DOWNLOAD_PROTOCOL_USENET || q.GetIndexer().GetIndexerId() != "nzb" {
			t.Errorf("queued %s = %v, want = downloading %s in SABnzbd, from the indexer", rel.title, q, quality)
		}
		if j := sab.job(rel.id); j.category != "movies" {
			t.Errorf("SABnzbd job %s in category %q, want = movies", j.name, j.category)
		}
		return sab.job(rel.id)
	}
	// imported completes a download and waits for its import.
	imported := func(rel offered, j *sabJob, want string) {
		t.Helper()
		// Downloading first: the client's progress is tracked.
		waitFor(t, rel.title+" tracked downloading", func() bool {
			runQueueCheck()
			q := download(rel.title)
			return q.GetStatus() == acquisitionv1.DownloadStatus_DOWNLOAD_STATUS_DOWNLOADING && q.GetRemainingSize() == 1500<<20
		}, describeQueue)
		video := film
		if strings.Contains(rel.title, "1080p") {
			video = film1080
		}
		sab.finish(t, j, video, "")
		waitFor(t, rel.title+" imported", func() bool {
			runQueueCheck()
			return download(rel.title).GetState() == acquisitionv1.DownloadState_DOWNLOAD_STATE_IMPORTED
		}, describeQueue)
		if _, err := os.Stat(want); err != nil {
			t.Errorf("imported file: %v", err)
		}
		// The imported job is removed from SABnzbd with its folder, as
		// the client removes completed downloads.
		waitFor(t, j.name+" removed from SABnzbd", func() bool { return sab.removed(j) })
		if _, err := os.Stat(filepath.Join(sab.complete, j.category, j.name)); !os.IsNotExist(err) {
			t.Errorf("job folder of %s after the import: %v, want = removed", j.name, err)
		}
		if it := item(); it.GetVirtual() || it.GetPath() != filepath.ToSlash(want) {
			t.Errorf("movie after the import: virtual=%v path=%q, want = %q", it.GetVirtual(), it.GetPath(), filepath.ToSlash(want))
		}
		// The scan of the library reads the file as the movie's source.
		var sources []*libraryv1.MediaSource
		waitFor(t, "the media source of "+filepath.Base(want), func() bool {
			resp, err := playback.ListMediaSources(ctx, playbackv1.ListMediaSourcesRequest_builder{ItemId: new(movieID)}.Build())
			if err != nil {
				t.Fatalf("ListMediaSources: %v", err)
			}
			sources = resp.GetMediaSources()
			return len(sources) == 1 && sources[0].GetPath() == filepath.ToSlash(want) && len(sources[0].GetStreams()) == 2
		}, func() any { return sources })
	}

	folder := filepath.Join(movies, "The Matrix (1999)")
	day := time.Now().Add(-48 * time.Hour)
	hdtv := offered{id: "aaa1", title: "The.Matrix.1999.720p.HDTV.x264-GRPA", size: 4 << 30, published: day}
	webdl := offered{id: "bbb2", title: "The.Matrix.1999.1080p.WEB-DL.DD5.1.H.264-GRPB", size: 6 << 30, published: day}
	blurayFailing := offered{id: "ccc3", title: "The.Matrix.1999.1080p.BluRay.x264-GRPC", size: 9 << 30, published: day}
	blurayReplacement := offered{id: "ddd4", title: "The.Matrix.1999.1080p.BluRay.x264-GRPD", size: 9 << 30, published: day.Add(-time.Hour)}

	// 1. A search finds the HDTV release, which is grabbed, tracked and
	// imported under Radarr's naming, and played.
	indexer.offer(hdtv)
	if resp, err := wanted.SearchWanted(ctx, acquisitionv1.SearchWantedRequest_builder{Filter: new(acquisitionv1.WantedFilter_WANTED_FILTER_MISSING)}.Build()); err != nil || resp.GetQueued() != 1 {
		t.Fatalf("SearchWanted = %v, %v; want = the movie queued", resp, err)
	}
	job := grabbed(hdtv, "HDTV-720p")
	first720 := filepath.Join(folder, "The Matrix (1999) HDTV-720p.mkv")
	imported(hdtv, job, first720)
	grabs, imports := history(acquisitionv1.HistoryEventType_HISTORY_EVENT_TYPE_GRABBED), history(acquisitionv1.HistoryEventType_HISTORY_EVENT_TYPE_IMPORTED)
	if len(grabs) != 1 || grabs[0].GetSourceTitle() != hdtv.title || len(imports) != 1 || imports[0].GetData()["importedPath"] != filepath.ToSlash(first720) {
		t.Errorf("history: grabs %v, imports %v", grabs, imports)
	}
	resp, err := playback.StartPlayback(ctx, playbackv1.StartPlaybackRequest_builder{
		ItemId: new(movieID),
		Capabilities: playbackv1.ClientCapabilities_builder{
			Name: new("e2e"),
			DirectPlay: []*playbackv1.DirectPlayProfile{playbackv1.DirectPlayProfile_builder{
				Kind: new(playbackv1.MediaKind_MEDIA_KIND_VIDEO), Container: new("mkv"), VideoCodec: new("h264"), AudioCodec: new("aac"),
			}.Build()},
		}.Build(),
	}.Build())
	if err != nil {
		t.Fatalf("StartPlayback: %v", err)
	}
	if resp.GetMethod() != playbackv1.PlayMethod_PLAY_METHOD_DIRECT_PLAY {
		t.Fatalf("StartPlayback = %v, want = direct play", resp)
	}
	want, err := os.ReadFile(film)
	if err != nil {
		t.Fatal(err)
	}
	if body := get(t, srv.url+"/"+resp.GetUrl()); !bytes.Equal(body, want) {
		t.Errorf("played %d bytes, want = the %d of the imported file", len(body), len(want))
	}
	if _, err := playback.StopPlayback(ctx, playbackv1.StopPlaybackRequest_builder{PlaybackId: new(resp.GetPlaybackId())}.Build()); err != nil {
		t.Errorf("StopPlayback: %v", err)
	}

	// 2. An RSS sync finds a WEB-DL, an upgrade below the cutoff: it is
	// imported, the HDTV file going to the recycle bin.
	unmet := func() []string {
		resp, err := wanted.ListWanted(ctx, acquisitionv1.ListWantedRequest_builder{Filter: new(acquisitionv1.WantedFilter_WANTED_FILTER_CUTOFF_UNMET)}.Build())
		if err != nil {
			t.Fatalf("ListWanted: %v", err)
		}
		var out []string
		for _, m := range resp.GetMovies() {
			out = append(out, m.GetName()+" "+m.GetQuality())
		}
		return out
	}
	if got := unmet(); !slices.Equal(got, []string{"The Matrix HDTV-720p"}) {
		t.Errorf("cutoff unmet = %q, want = The Matrix at HDTV-720p", got)
	}
	indexer.offer(webdl)
	if _, err := wanted.SyncRSS(ctx, &acquisitionv1.SyncRSSRequest{}); err != nil {
		t.Fatalf("SyncRSS: %v", err)
	}
	job = grabbed(webdl, "WEBDL-1080p")
	upgraded := filepath.Join(folder, "The Matrix (1999) WEBDL-1080p.mkv")
	imported(webdl, job, upgraded)
	if _, err := os.Stat(first720); !os.IsNotExist(err) {
		t.Errorf("replaced file: %v, want = gone", err)
	}
	if _, err := os.Stat(filepath.Join(recycle, "The Matrix (1999)", "The Matrix (1999) HDTV-720p.mkv")); err != nil {
		t.Errorf("replaced file in the recycle bin: %v", err)
	}
	if deleted := history(acquisitionv1.HistoryEventType_HISTORY_EVENT_TYPE_FILE_DELETED); len(deleted) != 1 || deleted[0].GetData()["reason"] != "Upgrade" {
		t.Errorf("deleted files = %v, want = the HDTV file, upgraded", deleted)
	}

	// 3. A search grabs a Blu-ray, which SABnzbd fails: it is blocklisted,
	// and the search that follows grabs another, which is imported.
	indexer.offer(blurayFailing)
	if _, err := wanted.SearchWanted(ctx, acquisitionv1.SearchWantedRequest_builder{ItemIds: []string{movieID}}.Build()); err != nil {
		t.Fatalf("SearchWanted: %v", err)
	}
	job = grabbed(blurayFailing, "Bluray-1080p")
	indexer.offer(blurayReplacement)
	sab.finish(t, job, "", "Repair failed, not enough repair blocks (100 short)")
	var failed []*acquisitionv1.HistoryEvent
	waitFor(t, "the failed download handled", func() bool {
		runQueueCheck()
		failed = history(acquisitionv1.HistoryEventType_HISTORY_EVENT_TYPE_DOWNLOAD_FAILED)
		return download(blurayFailing.title).GetState() == acquisitionv1.DownloadState_DOWNLOAD_STATE_FAILED && len(failed) > 0
	}, describeQueue)
	if len(failed) != 1 || failed[0].GetSourceTitle() != blurayFailing.title || !strings.Contains(failed[0].GetData()["message"], "Repair failed") {
		t.Errorf("failed downloads = %v, want = %s with SABnzbd's message", failed, blurayFailing.title)
	}
	block, err := downloads.ListBlocklist(ctx, acquisitionv1.ListBlocklistRequest_builder{ItemId: new(movieID)}.Build())
	if err != nil || len(block.GetEntries()) != 1 || block.GetEntries()[0].GetSourceTitle() != blurayFailing.title ||
		!strings.Contains(block.GetEntries()[0].GetMessage(), "Repair failed") || block.GetEntries()[0].GetIndexer().GetIndexerId() != "nzb" {
		t.Errorf("ListBlocklist = %v, %v; want = %s with SABnzbd's message", block, err, blurayFailing.title)
	}
	waitFor(t, "the failed job removed from SABnzbd", func() bool { return sab.removed(job) })
	job = grabbed(blurayReplacement, "Bluray-1080p")
	// The failed release stays rejected as blocklisted.
	found, err := releases.SearchReleases(ctx, acquisitionv1.SearchReleasesRequest_builder{ItemId: new(movieID)}.Build())
	if err != nil {
		t.Fatalf("SearchReleases: %v", err)
	}
	i := slices.IndexFunc(found.GetReleases(), func(r *acquisitionv1.Release) bool { return r.GetTitle() == blurayFailing.title })
	if i < 0 {
		t.Errorf("SearchReleases found no %s", blurayFailing.title)
	} else if r := found.GetReleases()[i]; !slices.ContainsFunc(r.GetRejections(), func(rj *acquisitionv1.Rejection) bool { return rj.GetReason() == "blocklisted" }) {
		t.Errorf("rejections of the failed release = %v, want = blocklisted", r.GetRejections())
	}
	final := filepath.Join(folder, "The Matrix (1999) Bluray-1080p.mkv")
	imported(blurayReplacement, job, final)
	if _, err := os.Stat(filepath.Join(recycle, "The Matrix (1999)", "The Matrix (1999) WEBDL-1080p.mkv")); err != nil {
		t.Errorf("replaced WEB-DL in the recycle bin: %v", err)
	}
	if got := unmet(); len(got) != 0 {
		t.Errorf("cutoff unmet = %q, want = none once at Bluray-1080p", got)
	}
}
