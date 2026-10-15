package server_test

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/apps/server/internal/server"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
	acquisitionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1/acquisitionv1connect"
	authv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1/authv1connect"
	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1/libraryv1connect"
	systemv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1/systemv1connect"
)

var subtitlesSource = filepath.Join("..", "..", "..", "..", "plugins", "subtitles")

// subtitlesBuild builds the subtitles plugin once per test binary.
var subtitlesBuild = sync.OnceValues(func() (string, error) {
	out := filepath.Join(smokeRoot, "subtitles")
	build := exec.Command("go", "build", "-o", out, ".")
	build.Dir = subtitlesSource
	if msg, err := build.CombinedOutput(); err != nil {
		return "", fmt.Errorf("build the subtitles plugin: %w\n%s", err, msg)
	}
	return out, nil
})

// frenchSubtitle is the subtitle the fake Subsarr serves.
const frenchSubtitle = "1\n00:00:01,000 --> 00:00:02,000\nBonjour, Neo.\n"

// TestSubtitles runs subtitle acquisition through the assembled server:
// the subtitles plugin, against a fake Subsarr, finds the French subtitles
// a film's profile wants; the administrator lists, deletes, uploads and
// modifies them.
func TestSubtitles(t *testing.T) {
	metadataDir := filepath.ToSlash(t.TempDir())
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("integration test: ffmpeg not found on PATH")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("integration test: ffprobe not found on PATH")
	}
	ctx := t.Context()
	var (
		mu       sync.Mutex
		searched url.Values
	)
	subsarr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/subtitles/search":
			mu.Lock()
			searched = r.URL.Query()
			mu.Unlock()
			_, _ = io.WriteString(w, `{"items": [{"id": 1, "language": "french", "hi": false,
				"download_url": "/download/1", "title": "The Matrix",
				"releases": ["The.Matrix.1999.1080p.BluRay.x264-GROUP"], "filename": "The.Matrix.1999.1080p.BluRay.x264-GROUP.srt"}]}`)
		case "/download/1":
			_, _ = io.WriteString(w, frenchSubtitle)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(subsarr.Close)
	pluginDir := t.TempDir()
	installProcessPlugin(t, pluginDir, "subtitles", subtitlesSource, subtitlesBuild)
	srv := start(t, server.Config{
		Database: "sqlite:" + filepath.Join(t.TempDir(), "mavio.db"), FFmpeg: ffmpeg, FFprobe: ffprobe,
		TranscodeDir: t.TempDir(), CacheDir: t.TempDir(), MetadataDir: metadataDir,
		PluginDir: pluginDir, PluginDataDir: t.TempDir(),
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
	items := libraryv1connect.NewItemServiceClient(http.DefaultClient, srv.url, token)
	subs := acquisitionv1connect.NewSubtitleServiceClient(http.DefaultClient, srv.url, token)

	// A film lies in a library whose profile wants French subtitles.
	movies := t.TempDir()
	folder := filepath.Join(movies, "The Matrix (1999)")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	video := filepath.Join(folder, "The Matrix (1999).mkv")
	run(t, ffmpeg, "-loglevel", "error", "-f", "lavfi", "-i", "testsrc2=size=320x240:rate=1",
		"-f", "lavfi", "-i", "sine=frequency=440:sample_rate=8000", "-t", "3",
		"-c:v", "libx264", "-preset", "ultrafast", "-c:a", "aac", "-b:a", "16k", "-metadata:s:a:0", "language=eng", video)
	settled(t, video)
	profile, err := subs.CreateSubtitleProfile(ctx, acquisitionv1.CreateSubtitleProfileRequest_builder{
		Profile: acquisitionv1.SubtitleProfile_builder{Name: new("French"), Items: []*acquisitionv1.SubtitleProfileItem{
			acquisitionv1.SubtitleProfileItem_builder{Id: new(int32(1)), Language: new("fr")}.Build(),
		}}.Build(),
	}.Build())
	if err != nil {
		t.Fatalf("CreateSubtitleProfile: %v", err)
	}
	lib, err := libraryv1connect.NewLibraryServiceClient(http.DefaultClient, srv.url, token).CreateLibrary(ctx, libraryv1.CreateLibraryRequest_builder{
		Spec: libraryv1.LibrarySpec_builder{Name: new("Films"), Kind: new(libraryv1.LibraryKind_LIBRARY_KIND_MOVIES), Paths: []string{movies}}.Build(),
	}.Build())
	if err != nil {
		t.Fatalf("CreateLibrary: %v", err)
	}
	if _, err := subs.AssignSubtitleProfile(ctx, acquisitionv1.AssignSubtitleProfileRequest_builder{
		TargetId: new(lib.GetLibrary().GetId()), ProfileId: new(profile.GetProfile().GetId()),
	}.Build()); err != nil {
		t.Fatalf("AssignSubtitleProfile: %v", err)
	}
	assigned, err := subs.ListSubtitleProfileAssignments(ctx, acquisitionv1.ListSubtitleProfileAssignmentsRequest_builder{
		TargetIds: []string{lib.GetLibrary().GetId()},
	}.Build())
	if err != nil {
		t.Fatalf("ListSubtitleProfileAssignments: %v", err)
	}
	if a := assigned.GetAssignments(); len(a) != 1 || a[0].GetProfileId() != profile.GetProfile().GetId() {
		t.Errorf("assignments = %v, want the library's French profile", a)
	}
	var movieID string
	waitFor(t, "the film scanned", func() bool {
		list, err := items.ListItems(ctx, libraryv1.ListItemsRequest_builder{Kinds: []libraryv1.ItemKind{libraryv1.ItemKind_ITEM_KIND_MOVIE}}.Build())
		if err != nil || len(list.GetItems()) != 1 {
			return false
		}
		movieID = list.GetItems()[0].GetId()
		return true
	})

	// The film wants French subtitles.
	waitFor(t, "the film wanting French subtitles", func() bool {
		wanted, err := subs.ListWantedSubtitles(ctx, acquisitionv1.ListWantedSubtitlesRequest_builder{
			Kind: acquisitionv1.SubtitleItemKind_SUBTITLE_ITEM_KIND_MOVIE.Enum(),
		}.Build())
		return err == nil && wanted.GetTotal() == 1 && wanted.GetItems()[0].GetItemId() == movieID &&
			len(wanted.GetItems()[0].GetMissing()) == 1 && wanted.GetItems()[0].GetMissing()[0].GetLanguage() == "fr"
	})

	// Subsarr is enabled once the film is probed and searched for at its
	// import, with no provider, so that it is still wanting them.
	waitFor(t, "the film probed and searched for", func() bool {
		jobs, err := tasks.ListJobs(ctx, systemv1.ListJobsRequest_builder{
			Kinds:  []string{library.JobProbe, library.JobRefresh, "subtitles.search"},
			States: []systemv1.JobState{systemv1.JobState_JOB_STATE_PENDING, systemv1.JobState_JOB_STATE_RUNNING},
		}.Build())
		return err == nil && len(jobs.GetJobs()) == 0
	})
	if _, err := system.SetPluginConfig(ctx, systemv1.SetPluginConfigRequest_builder{
		PluginId:   new("dev.mavio.subtitles"),
		ConfigJson: new(`{"providers": [{"name": "subsarr", "enabled": true, "settings": {"base_url": "` + subsarr.URL + `"}}]}`),
	}.Build()); err != nil {
		t.Fatalf("SetPluginConfig: %v", err)
	}
	waitFor(t, "Subsarr enabled", func() bool {
		resp, err := subs.ListSubtitleProviders(ctx, &acquisitionv1.ListSubtitleProvidersRequest{})
		if err != nil {
			return false
		}
		for _, p := range resp.GetProviders() {
			if p.GetId() == "dev.mavio.subtitles/subsarr" {
				return p.GetEnabled()
			}
		}
		return false
	})

	// The wanted search downloads them beside the film.
	runTask(t, tasks, "subtitles.wanted_movies")
	french := filepath.Join(folder, "The Matrix (1999).fr.srt")
	if data, err := os.ReadFile(french); err != nil || string(data) != frenchSubtitle {
		t.Fatalf("French subtitles = %q, %v, want = %q", data, err, frenchSubtitle)
	}
	mu.Lock()
	if searched.Get("language") != "french" {
		t.Errorf("Subsarr searched for %v, want = french", searched)
	}
	mu.Unlock()
	history, err := subs.ListSubtitleHistory(ctx, &acquisitionv1.ListSubtitleHistoryRequest{})
	if err != nil || len(history.GetEvents()) != 1 || history.GetEvents()[0].GetAction() != acquisitionv1.SubtitleAction_SUBTITLE_ACTION_DOWNLOADED ||
		history.GetEvents()[0].GetProvider() != "dev.mavio.subtitles/subsarr" {
		t.Fatalf("ListSubtitleHistory = %v, %v", history, err)
	}
	itemSubtitles := func() *acquisitionv1.GetItemSubtitlesResponse {
		t.Helper()
		resp, err := subs.GetItemSubtitles(ctx, acquisitionv1.GetItemSubtitlesRequest_builder{ItemId: new(movieID)}.Build())
		if err != nil {
			t.Fatalf("GetItemSubtitles: %v", err)
		}
		return resp
	}
	if got := itemSubtitles(); len(got.GetMissing()) != 0 || len(got.GetSubtitles()) != 1 || got.GetSubtitles()[0].GetPath() != french {
		t.Errorf("GetItemSubtitles = %v, want the French subtitles", got)
	}
	// The administrator replaces them with subtitles of their own,
	// without the hearing-impaired cues.
	if _, err := subs.DeleteSubtitle(ctx, acquisitionv1.DeleteSubtitleRequest_builder{ItemId: new(movieID), Path: new(french)}.Build()); err != nil {
		t.Fatalf("DeleteSubtitle: %v", err)
	}
	if _, err := os.Stat(french); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("deleted subtitles still there: %v", err)
	}
	uploaded, err := subs.UploadSubtitle(ctx, acquisitionv1.UploadSubtitleRequest_builder{
		ItemId: new(movieID), Data: []byte("1\n00:00:01,000 --> 00:00:02,000\n[MUSIQUE]\nSuivez le lapin blanc.\n"), Format: new("srt"),
		Language: acquisitionv1.SubtitleLanguageSpec_builder{Language: new("fr")}.Build(),
	}.Build())
	if err != nil || uploaded.GetPath() != french {
		t.Fatalf("UploadSubtitle = %v, %v", uploaded, err)
	}
	if _, err := subs.ModifySubtitle(ctx, acquisitionv1.ModifySubtitleRequest_builder{
		ItemId: new(movieID), Path: new(french), Mods: []string{"remove_HI"},
	}.Build()); err != nil {
		t.Fatalf("ModifySubtitle: %v", err)
	}
	if data, err := os.ReadFile(french); err != nil || strings.Contains(string(data), "MUSIQUE") || !strings.Contains(string(data), "Suivez le lapin blanc.") {
		t.Errorf("modified subtitles = %q, %v", data, err)
	}

	// Subtitles are saved in a folder of the video's folder, or in the
	// item's folder within the metadata folder.
	settings, err := subs.GetSubtitleSettings(ctx, &acquisitionv1.GetSubtitleSettingsRequest{})
	if err != nil {
		t.Fatalf("GetSubtitleSettings: %v", err)
	}
	s := settings.GetSettings()
	upload := func(folder acquisitionv1.SubtitleFolder, custom, lang, want string) {
		t.Helper()
		s.SetFolder(folder)
		s.SetCustomFolder(custom)
		if _, err := subs.UpdateSubtitleSettings(ctx, acquisitionv1.UpdateSubtitleSettingsRequest_builder{Settings: s}.Build()); err != nil {
			t.Fatalf("UpdateSubtitleSettings with folder %s: %v", custom, err)
		}
		up, err := subs.UploadSubtitle(ctx, acquisitionv1.UploadSubtitleRequest_builder{
			ItemId: new(movieID), Data: []byte("1\n00:00:01,000 --> 00:00:02,000\nHi\n"), Format: new("srt"),
			Language: acquisitionv1.SubtitleLanguageSpec_builder{Language: new(lang)}.Build(),
		}.Build())
		if err != nil || filepath.ToSlash(up.GetPath()) != want {
			t.Fatalf("UploadSubtitle into %s = %v, %v, want %s", custom, up, err, want)
		}
		if _, err := os.Stat(want); err != nil {
			t.Errorf("uploaded subtitle: %v", err)
		}
	}
	upload(acquisitionv1.SubtitleFolder_SUBTITLE_FOLDER_RELATIVE, "Subs", "de", filepath.ToSlash(folder)+"/Subs/The Matrix (1999).de.srt")
	upload(acquisitionv1.SubtitleFolder_SUBTITLE_FOLDER_METADATA, "", "es", library.SubtitleDir(metadataDir, mustID(t, movieID))+"/The Matrix (1999).es.srt")
	s.SetFolder(acquisitionv1.SubtitleFolder_SUBTITLE_FOLDER_RELATIVE)
	s.SetCustomFolder("Subs/de")
	if _, err := subs.UpdateSubtitleSettings(ctx, acquisitionv1.UpdateSubtitleSettingsRequest_builder{Settings: s}.Build()); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("UpdateSubtitleSettings with a nested relative folder: err = %v, want InvalidArgument", err)
	}
	s.SetFolder(acquisitionv1.SubtitleFolder_SUBTITLE_FOLDER_UNSPECIFIED)
	s.SetCustomFolder("")
	s.SetMods([]string{"remove_HI", "common"})
	if _, err := subs.UpdateSubtitleSettings(ctx, acquisitionv1.UpdateSubtitleSettingsRequest_builder{Settings: s}.Build()); err != nil {
		t.Errorf("UpdateSubtitleSettings: %v", err)
	}
	s.SetMods([]string{"no_such_mod"})
	if _, err := subs.UpdateSubtitleSettings(ctx, acquisitionv1.UpdateSubtitleSettingsRequest_builder{Settings: s}.Build()); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("UpdateSubtitleSettings with an unknown mod: err = %v, want InvalidArgument", err)
	}
}

func mustID(t *testing.T, s string) core.ID {
	t.Helper()
	id, err := core.ParseID(s)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
