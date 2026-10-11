package server_test

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"os/exec"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/apps/server/internal/server"
	"github.com/mavioai/mavio/libs/acquire"
	"github.com/mavioai/mavio/libs/core"
	acquisitionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1/acquisitionv1connect"
	authv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1/authv1connect"
	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1/libraryv1connect"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/playback/v1/playbackv1connect"
	systemv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1/systemv1connect"
	userv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1/userv1connect"
	"github.com/mavioai/mavio/libs/release"
)

// acquisitionServer is the assembled server of an acquisition end-to-end
// test, with the metadata plugins against fakes of their services, and
// the clients of its administrator.
type acquisitionServer struct {
	t               *testing.T
	url             string
	ffmpeg, ffprobe string
	fake            *fakeServices
	adminID         string
	recycleBin      string

	auth      authv1connect.AuthServiceClient
	system    systemv1connect.SystemServiceClient
	tasks     systemv1connect.TaskServiceClient
	activity  systemv1connect.ActivityServiceClient
	users     userv1connect.UserServiceClient
	libraries libraryv1connect.LibraryServiceClient
	items     libraryv1connect.ItemServiceClient
	playback  playbackv1connect.PlaybackServiceClient
	profiles  acquisitionv1connect.ProfileServiceClient
	wanted    acquisitionv1connect.WantedServiceClient
	downloads acquisitionv1connect.DownloadServiceClient
	releases  acquisitionv1connect.ReleaseServiceClient
	requests  acquisitionv1connect.RequestServiceClient
}

// startAcquisition starts the server with the metadata plugins answered
// by routes, besides serviceRoutes, and with the indexers and the
// download clients plugins when processPlugins is set. A request of the
// plugins no route answers fails the test. Acquisition is set up with a
// recycle bin, renaming, no free space check, and RSS syncs only when
// asked, so that none runs between the steps.
func startAcquisition(t *testing.T, routes map[string]string, processPlugins bool) *acquisitionServer {
	t.Helper()
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("integration test: ffmpeg not found on PATH")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("integration test: ffprobe not found on PATH")
	}
	// Sonarr's services and XEM, which scraper-servarr asks from the
	// start, know nothing unless routes tell otherwise.
	all := map[string]string{
		"services.sonarr.tv/v1/scenemapping": `[]`,
		"services.sonarr.tv/v1/dailyseries":  `[]`,
		"thexem.info/map/allNames":           `{"result": "success", "data": {}, "message": ""}`,
		"thexem.info/map/havemap":            `{"result": "success", "data": [], "message": ""}`,
	}
	maps.Copy(all, routes)
	fake := &fakeServices{mode: tmdbAcceptsRadarr, routes: all}
	t.Cleanup(func() {
		fake.mu.Lock()
		defer fake.mu.Unlock()
		if len(fake.unknown) > 0 {
			t.Errorf("requests answered 404: %v", fake.unknown)
		}
	})
	pluginDir := installScrapers(t)
	if processPlugins {
		installProcessPlugin(t, pluginDir, "indexers", indexersSource, indexersBuild)
		installProcessPlugin(t, pluginDir, "download-clients", downloadClientsSource, downloadClientsBuild)
	}
	srv := start(t, server.Config{
		Database: "sqlite:" + filepath.Join(t.TempDir(), "mavio.db"), FFmpeg: ffmpeg, FFprobe: ffprobe,
		TranscodeDir: t.TempDir(), CacheDir: t.TempDir(), MetadataDir: t.TempDir(),
		PluginDir: pluginDir, PluginDataDir: t.TempDir(), PluginFetch: &http.Client{Transport: fake},
	})
	auth := authv1connect.NewAuthServiceClient(http.DefaultClient, srv.url)
	first, err := auth.CreateFirstUser(t.Context(), authv1.CreateFirstUserRequest_builder{
		Name: new("admin"), Password: new("secret"), Device: device("e2e"),
	}.Build())
	if err != nil {
		t.Fatalf("CreateFirstUser: %v", err)
	}
	token := withToken(first.GetAccessToken())
	s := &acquisitionServer{
		t: t, url: srv.url, ffmpeg: ffmpeg, ffprobe: ffprobe, fake: fake, adminID: first.GetUser().GetId(), recycleBin: t.TempDir(),
		auth:      auth,
		system:    systemv1connect.NewSystemServiceClient(http.DefaultClient, srv.url, token),
		tasks:     systemv1connect.NewTaskServiceClient(http.DefaultClient, srv.url, token),
		activity:  systemv1connect.NewActivityServiceClient(http.DefaultClient, srv.url, token),
		users:     userv1connect.NewUserServiceClient(http.DefaultClient, srv.url, token),
		libraries: libraryv1connect.NewLibraryServiceClient(http.DefaultClient, srv.url, token),
		items:     libraryv1connect.NewItemServiceClient(http.DefaultClient, srv.url, token),
		playback:  playbackv1connect.NewPlaybackServiceClient(http.DefaultClient, srv.url, token),
		profiles:  acquisitionv1connect.NewProfileServiceClient(http.DefaultClient, srv.url, token),
		wanted:    acquisitionv1connect.NewWantedServiceClient(http.DefaultClient, srv.url, token),
		downloads: acquisitionv1connect.NewDownloadServiceClient(http.DefaultClient, srv.url, token),
		releases:  acquisitionv1connect.NewReleaseServiceClient(http.DefaultClient, srv.url, token),
		requests:  acquisitionv1connect.NewRequestServiceClient(http.DefaultClient, srv.url, token),
	}
	s.settings(func(settings *systemv1.ServerSettings) {
		acq := settings.GetAcquisition()
		acq.SetRecycleBin(s.recycleBin)
		acq.SetRssSyncInterval(0)
		acq.SetSkipFreeSpaceCheck(true)
		acq.SetRenameMovies(true)
		acq.SetRenameEpisodes(true)
	})
	return s
}

// video encodes a video of a frame size and length in seconds, with
// audio, and returns its path.
func (s *acquisitionServer) video(size string, seconds int) string {
	s.t.Helper()
	path := filepath.Join(s.t.TempDir(), size+".mkv")
	run(s.t, s.ffmpeg, "-loglevel", "error", "-f", "lavfi", "-i", "testsrc2=size="+size+":rate=1",
		"-f", "lavfi", "-i", "sine=frequency=440:sample_rate=8000", "-t", fmt.Sprint(seconds),
		"-c:v", "libx264", "-preset", "ultrafast", "-c:a", "aac", "-b:a", "16k", path)
	settled(s.t, path)
	return path
}

// settings changes the server's settings.
func (s *acquisitionServer) settings(edit func(*systemv1.ServerSettings)) {
	s.t.Helper()
	ctx := s.t.Context()
	got, err := s.system.GetServerSettings(ctx, &systemv1.GetServerSettingsRequest{})
	if err != nil {
		s.t.Fatalf("GetServerSettings: %v", err)
	}
	settings := got.GetSettings()
	edit(settings)
	if _, err := s.system.UpdateServerSettings(ctx, systemv1.UpdateServerSettingsRequest_builder{Settings: settings}.Build()); err != nil {
		s.t.Fatalf("UpdateServerSettings: %v", err)
	}
}

// library creates a library and returns its ID.
func (s *acquisitionServer) library(name string, kind libraryv1.LibraryKind, path string) string {
	s.t.Helper()
	lib, err := s.libraries.CreateLibrary(s.t.Context(), libraryv1.CreateLibraryRequest_builder{
		Spec: libraryv1.LibrarySpec_builder{Name: new(name), Kind: new(kind), Paths: []string{path}}.Build(),
	}.Build())
	if err != nil {
		s.t.Fatalf("CreateLibrary %s: %v", name, err)
	}
	return lib.GetLibrary().GetId()
}

// profile creates a quality profile upgrading up to a cutoff and returns
// its ID.
func (s *acquisitionServer) profile(kind release.Kind, cutoff release.Quality, qualities ...release.Quality) string {
	s.t.Helper()
	coreKind, protoKind := core.KindMovie, acquisitionv1.MediaKind_MEDIA_KIND_MOVIE
	if kind == release.Series {
		coreKind, protoKind = core.KindSeries, acquisitionv1.MediaKind_MEDIA_KIND_SERIES
	}
	stored, err := acquire.QualityProfileToCore(acquire.NewProfile(kind, "HD", cutoff, qualities, nil), coreKind)
	if err != nil {
		s.t.Fatal(err)
	}
	profile, err := s.profiles.CreateQualityProfile(s.t.Context(), acquisitionv1.CreateQualityProfileRequest_builder{
		Profile: acquisitionv1.QualityProfile_builder{
			Kind: new(protoKind), Name: new("HD"), UpgradeAllowed: new(true),
			Cutoff: new(int32(stored.Cutoff)), Items: profileItemsToProto(stored.Items),
		}.Build(),
	}.Build())
	if err != nil {
		s.t.Fatalf("CreateQualityProfile: %v", err)
	}
	return profile.GetProfile().GetId()
}

// plugins configures the indexers and the download clients plugins and
// waits for the indexers and clients to be ready.
func (s *acquisitionServer) plugins(indexers, clients []any) {
	s.t.Helper()
	ctx := s.t.Context()
	for id, cfg := range map[string]any{
		"org.mavio.indexers":         map[string]any{"indexers": indexers},
		"org.mavio.download-clients": map[string]any{"clients": clients},
	} {
		data, err := json.Marshal(cfg)
		if err != nil {
			s.t.Fatal(err)
		}
		if _, err := s.system.SetPluginConfig(ctx, systemv1.SetPluginConfigRequest_builder{PluginId: new(id), ConfigJson: new(string(data))}.Build()); err != nil {
			s.t.Fatalf("SetPluginConfig %s: %v", id, err)
		}
	}
	waitFor(s.t, "the indexers and the download clients", func() bool {
		ix, err := s.releases.ListIndexers(ctx, &acquisitionv1.ListIndexersRequest{})
		if err != nil || len(ix.GetIndexers()) != len(indexers) {
			return false
		}
		dc, err := s.downloads.ListDownloadClients(ctx, &acquisitionv1.ListDownloadClientsRequest{})
		return err == nil && len(dc.GetClients()) == len(clients)
	})
}

// runTask runs a task and waits for its run to finish.
func (s *acquisitionServer) runTask(id string) {
	s.t.Helper()
	runTask(s.t, s.tasks, id)
}

// sceneMappings waits for the scene mappings and daily series to be read
// from Sonarr's services, and the scene numbering of the wanted series
// from XEM.
func (s *acquisitionServer) sceneMappings() {
	s.t.Helper()
	waitFor(s.t, "the scene mappings", func() bool {
		before := s.served("services.sonarr.tv/v1/dailyseries")
		s.runTask("acquisition.scene_mappings")
		return s.served("services.sonarr.tv/v1/dailyseries") > before && s.served("services.sonarr.tv/v1/scenemapping") > 0
	})
}

// served counts the requests a route answered.
func (s *acquisitionServer) served(path string) int {
	s.fake.mu.Lock()
	defer s.fake.mu.Unlock()
	return s.fake.served[path]
}

// queue lists the downloads, finished ones too.
func (s *acquisitionServer) queue() []*acquisitionv1.QueueItem {
	s.t.Helper()
	resp, err := s.downloads.ListQueue(s.t.Context(), acquisitionv1.ListQueueRequest_builder{IncludeFinished: new(true)}.Build())
	if err != nil {
		s.t.Fatalf("ListQueue: %v", err)
	}
	return resp.GetItems()
}

// describeQueue describes the downloads, for failures.
func (s *acquisitionServer) describeQueue() any {
	var out []string
	for _, q := range s.queue() {
		out = append(out, fmt.Sprintf("%s %s %s %v %q", q.GetTitle(), q.GetState(), q.GetStatus(), q.GetRemainingSize(), q.GetMessages()))
	}
	return out
}

// describeWanted describes, for failures, the downloads and what decides
// on a wanted item's releases: its runtime, which bounds their sizes, its
// history, and the releases a search finds for it with their rejections
// as they stand now.
func (s *acquisitionServer) describeWanted(itemID string) func() any {
	return func() any {
		ctx := s.t.Context()
		out := []any{"queue", s.describeQueue()}
		if it, err := s.items.GetItem(ctx, libraryv1.GetItemRequest_builder{Id: new(itemID)}.Build()); err == nil {
			out = append(out, "runtime", it.GetItem().GetRuntime().AsDuration())
		}
		if resp, err := s.downloads.ListHistory(ctx, acquisitionv1.ListHistoryRequest_builder{ItemId: new(itemID)}.Build()); err == nil {
			var history []string
			for _, h := range resp.GetEvents() {
				history = append(history, fmt.Sprintf("%s %s %s", h.GetType(), h.GetSourceTitle(), h.GetQuality()))
			}
			out = append(out, "history", history)
		}
		resp, err := s.releases.SearchReleases(ctx, acquisitionv1.SearchReleasesRequest_builder{ItemId: new(itemID)}.Build())
		if err != nil {
			return append(out, "search", err)
		}
		var found []string
		for _, r := range resp.GetReleases() {
			var rejections []string
			for _, rj := range r.GetRejections() {
				rejections = append(rejections, rj.GetMessage())
			}
			found = append(found, fmt.Sprintf("%s %q", r.GetTitle(), rejections))
		}
		return append(out, "search", found, resp.GetFailures())
	}
}

// download returns the download of a release, or nil.
func (s *acquisitionServer) download(title string) *acquisitionv1.QueueItem {
	for _, q := range s.queue() {
		if q.GetTitle() == title {
			return q
		}
	}
	return nil
}

// waitForDownload checks the queue until the download of a release is in
// a state.
func (s *acquisitionServer) waitForDownload(title string, state acquisitionv1.DownloadState) *acquisitionv1.QueueItem {
	s.t.Helper()
	var q *acquisitionv1.QueueItem
	waitFor(s.t, fmt.Sprintf("%s %v", title, state), func() bool {
		s.runTask("acquisition.queue")
		q = s.download(title)
		return q.GetState() == state
	}, s.describeQueue)
	return q
}

// history lists the history of an item of event types.
func (s *acquisitionServer) history(itemID string, types ...acquisitionv1.HistoryEventType) []*acquisitionv1.HistoryEvent {
	s.t.Helper()
	resp, err := s.downloads.ListHistory(s.t.Context(), acquisitionv1.ListHistoryRequest_builder{ItemId: new(itemID), Types: types}.Build())
	if err != nil {
		s.t.Fatalf("ListHistory: %v", err)
	}
	return resp.GetEvents()
}

// item returns an item.
func (s *acquisitionServer) item(id string) *libraryv1.Item {
	s.t.Helper()
	resp, err := s.items.GetItem(s.t.Context(), libraryv1.GetItemRequest_builder{Id: new(id)}.Build())
	if err != nil {
		s.t.Fatalf("GetItem: %v", err)
	}
	return resp.GetItem()
}

// children lists the items of a kind below a parent, virtual ones too.
func (s *acquisitionServer) children(parentID string, kind libraryv1.ItemKind) []*libraryv1.Item {
	s.t.Helper()
	resp, err := s.items.ListItems(s.t.Context(), libraryv1.ListItemsRequest_builder{
		ParentId: new(parentID), Recursive: new(true), Kinds: []libraryv1.ItemKind{kind}, IncludeVirtual: new(true),
	}.Build())
	if err != nil {
		s.t.Fatalf("ListItems: %v", err)
	}
	return resp.GetItems()
}

// episode returns an episode of a series by its season and number.
func (s *acquisitionServer) episode(seriesID string, season, number int32) *libraryv1.Item {
	s.t.Helper()
	for _, it := range s.children(seriesID, libraryv1.ItemKind_ITEM_KIND_EPISODE) {
		if it.GetParentIndexNumber() == season && it.GetIndexNumber() == number {
			return it
		}
	}
	s.t.Fatalf("series %s has no episode S%02dE%02d", seriesID, season, number)
	return nil
}

// addSeries adds a series of a library monitoring all its episodes, into
// season folders, waits for its episodes and returns its ID.
func (s *acquisitionServer) addSeries(libraryID, profileID string, tvdbID int, typ libraryv1.SeriesType, episodes int) string {
	s.t.Helper()
	added, err := s.wanted.AddSeries(s.t.Context(), acquisitionv1.AddSeriesRequest_builder{
		LibraryId: new(libraryID), TvdbId: new(int32(tvdbID)), QualityProfileId: new(profileID),
		SeriesType: new(typ), SeasonFolder: new(true), Monitor: new(acquisitionv1.SeriesMonitor_SERIES_MONITOR_ALL),
	}.Build())
	if err != nil {
		s.t.Fatalf("AddSeries %d: %v", tvdbID, err)
	}
	id := added.GetItemId()
	waitFor(s.t, fmt.Sprintf("the %d episodes of %d", episodes, tvdbID), func() bool {
		return len(s.children(id, libraryv1.ItemKind_ITEM_KIND_EPISODE)) == episodes
	})
	return id
}

// user creates a user of a policy, signs them in, and returns their ID
// and the option authenticating their calls.
func (s *acquisitionServer) user(name string, policy *userv1.UserPolicy) (string, connect.ClientOption) {
	s.t.Helper()
	ctx := s.t.Context()
	created, err := s.users.CreateUser(ctx, userv1.CreateUserRequest_builder{Name: new(name), Password: new("pw"), Policy: policy}.Build())
	if err != nil {
		s.t.Fatalf("CreateUser %s: %v", name, err)
	}
	signedIn, err := s.auth.Login(ctx, authv1.LoginRequest_builder{Name: new(name), Password: new("pw"), Device: device(name)}.Build())
	if err != nil {
		s.t.Fatalf("Login %s: %v", name, err)
	}
	return created.GetUser().GetId(), withToken(signedIn.GetAccessToken())
}
