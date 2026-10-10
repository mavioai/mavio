package server_test

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/mavioai/mavio/apps/server/internal/server"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/plugin/manifest"
	authv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1/authv1connect"
	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1/libraryv1connect"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	sessionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/session/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/session/v1/sessionv1connect"
	systemv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1/systemv1connect"
	userv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1/userv1connect"
)

// smokeRoot holds the builds of the smoke plugin.
var smokeRoot string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "smoke")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	smokeRoot = dir
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

// smokeBuilds holds the smoke plugin built for each runtime, once per test
// binary.
var smokeBuilds = map[string]func() (string, error){
	"wasm":    sync.OnceValues(func() (string, error) { return buildSmoke("wasm") }),
	"process": sync.OnceValues(func() (string, error) { return buildSmoke("process") }),
}

// buildSmoke builds the smoke plugin for a runtime into smokeRoot.
func buildSmoke(rt string) (string, error) {
	out := filepath.Join(smokeRoot, rt)
	args := []string{"build", "-o", out}
	env := os.Environ()
	if rt == "wasm" {
		args = append(args, "-buildmode=c-shared")
		env = append(env, "GOOS=wasip1", "GOARCH=wasm")
	}
	build := exec.Command("go", append(args, "github.com/mavioai/mavio/apps/server/internal/smoke/smokeplugin")...)
	build.Env = env
	if msg, err := build.CombinedOutput(); err != nil {
		return "", fmt.Errorf("build the %s plugin: %w\n%s", rt, err, msg)
	}
	return out, nil
}

// installSmoke installs the smoke plugin built for a runtime, "wasm" or
// "process", into a plugin folder, with the given manifest fields besides
// its ID, name, version, runtime and API version.
func installSmoke(t *testing.T, rt, fields string) string {
	t.Helper()
	bin, err := smokeBuilds[rt]()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	dir := filepath.Join(root, "smoke")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	m := &pluginv1.Manifest{}
	m.SetRuntime(pluginv1.Runtime_RUNTIME_PROCESS)
	if rt == "wasm" {
		m.SetRuntime(pluginv1.Runtime_RUNTIME_WASM)
	}
	data, err := os.ReadFile(bin)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifest.Executable(dir, m), data, 0o755); err != nil {
		t.Fatal(err)
	}
	doc := fmt.Sprintf(`{"id":"org.mavio.smoke","name":"Smoke","version":"0.1.0","runtime":%q,"apiVersion":"1.0",%s}`, m.GetRuntime().String(), fields)
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// TestPluginPlatform runs the assembled server with a plugin using the
// plugin platform, without ffmpeg, in both runtimes: its tasks, data
// folder, events, HTTP routes and remote devices.
func TestPluginPlatform(t *testing.T) {
	for _, rt := range []string{"wasm", "process"} {
		t.Run(rt, func(t *testing.T) { testPluginPlatform(t, rt) })
	}
}

func testPluginPlatform(t *testing.T, rt string) {
	ctx := t.Context()
	pluginDir := installSmoke(t, rt, `"capabilities":["CAPABILITY_TASK_RUNNER","CAPABILITY_EVENT_CONSUMER","CAPABILITY_HTTP_HANDLER",
			"CAPABILITY_DEVICE_CONTROLLER"],
		"tasks":[{"id":"count","name":"Count","description":"Counts its runs.","interval":"86400s"}],
		"permissions":{"events":["task.*","user.created"],"api":["mavio.playback.v1.PlaybackService"],"actAsUsers":true},
		"externalIdKinds":[{"key":"trakt","name":"Trakt","mediaKinds":["MEDIA_KIND_MOVIE","MEDIA_KIND_PERSON"],"urlTemplate":"https://trakt.tv/{id}"},
			{"key":"imdb","name":"Not IMDb","mediaKinds":["MEDIA_KIND_MOVIE"]}]`)
	dataDir := t.TempDir()
	srv := start(t, server.Config{
		Database: "sqlite:" + filepath.Join(t.TempDir(), "mavio.db"), FFprobe: "no-ffprobe", FFmpeg: "no-ffmpeg",
		TranscodeDir: t.TempDir(), CacheDir: t.TempDir(), MetadataDir: t.TempDir(), PluginDir: pluginDir, PluginDataDir: dataDir,
	})
	first, err := authv1connect.NewAuthServiceClient(http.DefaultClient, srv.url).CreateFirstUser(ctx, authv1.CreateFirstUserRequest_builder{
		Name: new("admin"), Password: new("secret"),
		Device: authv1.Device_builder{Id: new("e2e"), Name: new("Test"), Client: new("Test"), ClientVersion: new("0")}.Build(),
	}.Build())
	if err != nil {
		t.Fatalf("CreateFirstUser: %v", err)
	}
	tasks := systemv1connect.NewTaskServiceClient(http.DefaultClient, srv.url, withToken(first.GetAccessToken()))

	// The plugin's task is listed, scheduled, and runs on demand.
	const id = "plugin:org.mavio.smoke:count"
	task := func() *systemv1.Task {
		t.Helper()
		list, err := tasks.ListTasks(ctx, &systemv1.ListTasksRequest{})
		if err != nil {
			t.Fatalf("ListTasks: %v", err)
		}
		for _, task := range list.GetTasks() {
			if task.GetId() == id {
				return task
			}
		}
		t.Fatalf("ListTasks = %v, want %s", list, id)
		return nil
	}
	if got := task(); got.GetPluginId() != "org.mavio.smoke" || got.GetName() != "Count" ||
		got.GetInterval().AsDuration() != 24*time.Hour || got.GetNextRunTime() == nil {
		t.Errorf("plugin task = %v", got)
	}
	if _, err := tasks.RunTask(ctx, systemv1.RunTaskRequest_builder{Id: new(id)}.Build()); err != nil {
		t.Fatalf("RunTask: %v", err)
	}
	for deadline := time.Now().Add(30 * time.Second); task().GetLastRun() == nil; time.Sleep(100 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the task did not run within 30 s")
		}
	}
	if run := task().GetLastRun(); run.GetState() != systemv1.JobState_JOB_STATE_SUCCEEDED {
		t.Errorf("task run = %v", run)
	}
	if got, err := os.ReadFile(filepath.Join(dataDir, "org.mavio.smoke", "runs")); err != nil || string(got) != "1" {
		t.Errorf("runs in the plugin's data folder = %q, %v", got, err)
	}

	// The plugin consumes the events it asked for: the task's run and the
	// new account, but not sign-ins.
	users := userv1connect.NewUserServiceClient(http.DefaultClient, srv.url, withToken(first.GetAccessToken()))
	kid, err := users.CreateUser(ctx, userv1.CreateUserRequest_builder{Name: new("kid"), Password: new("secret")}.Build())
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	want := "task.completed plugin=org.mavio.smoke task=plugin:org.mavio.smoke:count\n" +
		"user.created by=" + first.GetUser().GetId() + "\n"
	events := filepath.Join(dataDir, "org.mavio.smoke", "events")
	for deadline := time.Now().Add(30 * time.Second); ; time.Sleep(100 * time.Millisecond) {
		got, _ := os.ReadFile(events)
		if string(got) == want {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("consumed events = %q, want %q (kid %s)", got, want, kid.GetUser().GetId())
		}
	}

	// The plugin's routes see the caller a valid token names, never the
	// token, and serve sandboxed pages; forged user headers are dropped.
	whoami := func(header, value string) (string, string) {
		t.Helper()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.url+"/plugins/org.mavio.smoke/whoami", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set(header, value)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil || resp.StatusCode != http.StatusOK {
			t.Fatalf("GET whoami = %d %s, %v", resp.StatusCode, body, err)
		}
		return string(body), resp.Header.Get("Content-Security-Policy")
	}
	body, csp := whoami("Authorization", "Bearer "+first.GetAccessToken())
	if want := `user="admin" admin="true" authorization=""`; body != want {
		t.Errorf("whoami with a token = %s, want %s", body, want)
	}
	if csp != "sandbox allow-scripts allow-forms allow-popups" {
		t.Errorf("Content-Security-Policy = %q", csp)
	}
	if body, _ := whoami("Mavio-User-Name", "admin"); body != `user="" admin="" authorization=""` {
		t.Errorf("whoami with a forged user = %s", body)
	}
	if body, _ := whoami("Authorization", "Basic eDp5"); body != `user="" admin="" authorization="Basic eDp5"` {
		t.Errorf("whoami with the plugin's own credentials = %s", body)
	}
	// The plugin learns how the server is reached.
	resp, err := http.Get(srv.url + "/plugins/org.mavio.smoke/server")
	if err != nil {
		t.Fatal(err)
	}
	info, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	host, _ := os.Hostname()
	if want := fmt.Sprintf(`name=%q version="v-test" http=%s https=0 base=""`, host, srv.url[strings.LastIndex(srv.url, ":")+1:]); string(info) != want {
		t.Errorf("GET server = %s, want %s", info, want)
	}
	resp, err = http.Get(srv.url + "/plugins/org.mavio.missing/whoami")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("route of a missing plugin = %d, want 404", resp.StatusCode)
	}

	// The plugin lists a remote device, which every user sees and may send
	// commands to; the plugin carries them out as the user and the device.
	setDevices := func(body string) {
		t.Helper()
		resp, err := http.Post(srv.url+"/plugins/org.mavio.smoke/devices", "text/plain", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		msg, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("POST devices = %d %s", resp.StatusCode, msg)
		}
	}
	setDevices("tv")
	kidToken, err := authv1connect.NewAuthServiceClient(http.DefaultClient, srv.url).Login(ctx, authv1.LoginRequest_builder{
		Name: new("kid"), Password: new("secret"),
		Device: authv1.Device_builder{Id: new("kid"), Name: new("Kid"), Client: new("Test"), ClientVersion: new("0")}.Build(),
	}.Build())
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	kidSessions := sessionv1connect.NewSessionServiceClient(http.DefaultClient, srv.url, withToken(kidToken.GetAccessToken()))
	device := func() *sessionv1.Session {
		t.Helper()
		list, err := kidSessions.ListSessions(ctx, &sessionv1.ListSessionsRequest{})
		if err != nil {
			t.Fatalf("ListSessions: %v", err)
		}
		for _, s := range list.GetSessions() {
			if s.GetPluginId() != "" {
				return s
			}
		}
		return nil
	}
	tv := device()
	if tv.GetPluginId() != "org.mavio.smoke" || tv.GetDeviceId() != "tv" || tv.GetDeviceName() != "Device tv" ||
		tv.GetClient() != "Smoke TV" || !tv.GetOnline() || tv.GetUserId() != "" {
		t.Fatalf("device session = %v", tv)
	}
	send := func(c *sessionv1.Command) error {
		_, err := kidSessions.SendCommand(ctx, sessionv1.SendCommandRequest_builder{SessionId: new(tv.GetId()), Command: c}.Build())
		return err
	}
	// Playing an item that does not exist gets as far as the playback.
	err = send(sessionv1.Command_builder{Play: sessionv1.Play_builder{ItemIds: []string{core.NewID().String()}}.Build()}.Build())
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("SendCommand(play) = %v, want not_found from the playback", err)
	}
	if err := send(sessionv1.Command_builder{PlayState: sessionv1.PlayState_builder{
		Command: sessionv1.PlayStateCommand_PLAY_STATE_COMMAND_PAUSE.Enum(),
	}.Build()}.Build()); err != nil {
		t.Errorf("SendCommand(pause) = %v", err)
	}
	if err := send(sessionv1.Command_builder{Seek: sessionv1.Seek_builder{Position: durationpb.New(time.Second)}.Build()}.Build()); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("SendCommand(seek) = %v, want failed_precondition", err)
	}
	if err := send(sessionv1.Command_builder{Volume: sessionv1.Volume_builder{Level: new(int32(30))}.Build()}.Build()); err != nil {
		t.Errorf("SendCommand(volume) = %v", err)
	}
	if err := send(sessionv1.Command_builder{Volume: &sessionv1.Volume{}}.Build()); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("SendCommand(empty volume) = %v, want invalid_argument", err)
	}
	if got, _ := os.ReadFile(filepath.Join(dataDir, "org.mavio.smoke", "volume")); string(got) != "true 30 false false\n" {
		t.Errorf("volume commands = %q", got)
	}
	// Listed again, it keeps its session; unlisted, it is gone.
	setDevices("tv")
	if again := device(); again.GetId() != tv.GetId() {
		t.Errorf("session listed again = %s, want %s", again.GetId(), tv.GetId())
	}
	setDevices("")
	if gone := device(); gone != nil {
		t.Errorf("unlisted device = %v", gone)
	}

	// The plugin's kind of external IDs is listed after the built-in ones;
	// it cannot redefine a built-in kind.
	meta := libraryv1connect.NewMetadataServiceClient(http.DefaultClient, srv.url, withToken(first.GetAccessToken()))
	kinds, err := meta.ListExternalIdKinds(ctx, libraryv1.ListExternalIdKindsRequest_builder{
		ItemKind: libraryv1.ItemKind_ITEM_KIND_MOVIE.Enum(),
	}.Build())
	if err != nil {
		t.Fatalf("ListExternalIdKinds: %v", err)
	}
	var keys []string
	for _, k := range kinds.GetKinds() {
		keys = append(keys, k.GetKey())
		if k.GetKey() == "imdb" && k.GetPluginId() != "" {
			t.Errorf("imdb kind = %v, want the built-in one", k)
		}
	}
	if last := kinds.GetKinds()[len(keys)-1]; last.GetKey() != "trakt" || last.GetPluginId() != "org.mavio.smoke" || !last.GetPersons() ||
		len(last.GetItemKinds()) != 1 || strings.Count(strings.Join(keys, ","), "imdb") != 1 {
		t.Errorf("movie external ID kinds = %q, last = %v", keys, last)
	}
}
