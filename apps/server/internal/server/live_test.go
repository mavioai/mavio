package server_test

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/mavioai/mavio/apps/server/internal/server"
	authv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1/authv1connect"
	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1/libraryv1connect"
	playbackv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/playback/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/playback/v1/playbackv1connect"
	sessionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/session/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/session/v1/sessionv1connect"
)

// TestLiveEvents runs the assembled server through its API only: a TV and
// a phone of one user hold event streams; the TV learns of the film a scan
// found and of the phone's playback, controls the phone, and both watch
// the film together in a SyncPlay group, told to start at the same time
// and position.
func TestLiveEvents(t *testing.T) {
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
	film := filepath.Join(media, "Up (2009)", "Up (2009).mkv")
	if err := os.MkdirAll(filepath.Dir(film), 0o755); err != nil {
		t.Fatal(err)
	}
	gen := exec.CommandContext(ctx, ffmpeg, "-loglevel", "error", "-f", "lavfi", "-i", "testsrc2=size=64x64:rate=24",
		"-t", "4", "-c:v", "libx264", film)
	if out, err := gen.CombinedOutput(); err != nil {
		t.Skipf("integration test: cannot generate the film: %v\n%s", err, out)
	}
	settled(t, film)

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
	auth := authv1connect.NewAuthServiceClient(http.DefaultClient, url)
	first, err := auth.CreateFirstUser(ctx, authv1.CreateFirstUserRequest_builder{
		Name: new("admin"), Password: new("secret"), Device: device("tv"),
	}.Build())
	if err != nil {
		t.Fatal(err)
	}
	phoneLogin, err := auth.Login(ctx, authv1.LoginRequest_builder{Name: new("admin"), Password: new("secret"), Device: device("phone")}.Build())
	if err != nil {
		t.Fatal(err)
	}
	tvToken, phoneToken := withToken(first.GetAccessToken()), withToken(phoneLogin.GetAccessToken())
	tv, phone := events(t, url, tvToken), events(t, url, phoneToken)
	await(t, tv, "TV connected", (*sessionv1.Event).HasConnected)
	phoneSession := await(t, phone, "phone connected", (*sessionv1.Event).HasConnected).GetConnected().GetSessionId()

	// The scan's film reaches the TV.
	if _, err := libraryv1connect.NewLibraryServiceClient(http.DefaultClient, url, tvToken).CreateLibrary(ctx, libraryv1.CreateLibraryRequest_builder{
		Spec: libraryv1.LibrarySpec_builder{Name: new("Films"), Kind: new(libraryv1.LibraryKind_LIBRARY_KIND_MOVIES), Paths: []string{media}}.Build(),
	}.Build()); err != nil {
		t.Fatal(err)
	}
	changed := await(t, tv, "the scanned film", func(e *sessionv1.Event) bool { return len(e.GetLibraryChanged().GetChangedItemIds()) > 0 })
	filmID := changed.GetLibraryChanged().GetChangedItemIds()[0]
	item, err := libraryv1connect.NewItemServiceClient(http.DefaultClient, url, tvToken).GetItem(ctx, libraryv1.GetItemRequest_builder{Id: &filmID}.Build())
	if err != nil || item.GetItem().GetName() != "Up" {
		t.Fatalf("changed item = %v, %v", item, err)
	}

	// The phone plays the film; the TV sees it playing and learns the
	// state the phone reached.
	playback := playbackv1connect.NewPlaybackServiceClient(http.DefaultClient, url, phoneToken)
	video := playbackv1.MediaKind_MEDIA_KIND_VIDEO
	started, err := playback.StartPlayback(ctx, playbackv1.StartPlaybackRequest_builder{
		ItemId: &filmID,
		Capabilities: playbackv1.ClientCapabilities_builder{Name: new("e2e"), DirectPlay: []*playbackv1.DirectPlayProfile{
			playbackv1.DirectPlayProfile_builder{Kind: &video, Container: new("mkv"), VideoCodec: new("h264")}.Build(),
		}}.Build(),
	}.Build())
	if err != nil {
		t.Fatal(err)
	}
	await(t, tv, "the phone's session changed", func(e *sessionv1.Event) bool { return e.GetSessionsChanged().GetSessionId() == phoneSession })
	sessions := sessionv1connect.NewSessionServiceClient(http.DefaultClient, url, tvToken)
	list, err := sessions.ListSessions(ctx, &sessionv1.ListSessionsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	var playing *sessionv1.NowPlaying
	for _, s := range list.GetSessions() {
		if s.GetId() == phoneSession {
			playing = s.GetNowPlaying()
		}
	}
	if playing.GetItem().GetId() != filmID || playing.GetPlaybackId() != started.GetPlaybackId() {
		t.Errorf("the phone plays %v", playing)
	}
	if _, err := playback.ReportProgress(ctx, playbackv1.ReportProgressRequest_builder{
		PlaybackId: new(started.GetPlaybackId()), Position: durationpb.New(3 * time.Second),
	}.Build()); err != nil {
		t.Fatal(err)
	}
	state := await(t, tv, "the phone's progress", (*sessionv1.Event).HasUserDataChanged).GetUserDataChanged().GetUserData()[0]
	if state.GetItemId() != filmID || !state.GetPlayed() {
		t.Errorf("the phone's state of the film = %v, want played", state)
	}

	// The TV pauses the phone.
	if _, err := sessions.SendCommand(ctx, sessionv1.SendCommandRequest_builder{
		SessionId: &phoneSession,
		Command: sessionv1.Command_builder{PlayState: sessionv1.PlayState_builder{
			Command: new(sessionv1.PlayStateCommand_PLAY_STATE_COMMAND_PAUSE),
		}.Build()}.Build(),
	}.Build()); err != nil {
		t.Fatal(err)
	}
	if cmd := await(t, phone, "the TV's command", (*sessionv1.Event).HasCommand).GetCommand(); cmd.GetCommand().GetPlayState().GetCommand() != sessionv1.PlayStateCommand_PLAY_STATE_COMMAND_PAUSE {
		t.Errorf("command = %v", cmd)
	}

	// Both watch the film together.
	tvSync := sessionv1connect.NewSyncPlayServiceClient(http.DefaultClient, url, tvToken)
	phoneSync := sessionv1connect.NewSyncPlayServiceClient(http.DefaultClient, url, phoneToken)
	group, err := tvSync.CreateGroup(ctx, sessionv1.CreateGroupRequest_builder{Name: new("Movie night")}.Build())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := phoneSync.JoinGroup(ctx, sessionv1.JoinGroupRequest_builder{GroupId: new(group.GetGroup().GetId())}.Build()); err != nil {
		t.Fatal(err)
	}
	if _, err := phoneSync.SetQueue(ctx, sessionv1.SetQueueRequest_builder{ItemIds: []string{filmID}, StartPosition: durationpb.New(time.Second)}.Build()); err != nil {
		t.Fatal(err)
	}
	queued := await(t, tv, "the queue", func(e *sessionv1.Event) bool { return len(e.GetSyncPlay().GetGroup().GetQueue()) == 1 })
	entry := queued.GetSyncPlay().GetGroup().GetQueue()[0].GetId()
	for _, c := range []sessionv1connect.SyncPlayServiceClient{tvSync, phoneSync} {
		now, err := c.GetTime(ctx, &sessionv1.GetTimeRequest{})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := c.ReportState(ctx, sessionv1.ReportStateRequest_builder{
			EntryId: &entry, Position: durationpb.New(time.Second), When: now.GetSendTime(), Ready: new(true),
		}.Build()); err != nil {
			t.Fatal(err)
		}
	}
	unpause := func(e *sessionv1.Event) bool {
		return e.GetSyncPlay().GetCommand().GetKind() == sessionv1.SyncPlayCommandKind_SYNC_PLAY_COMMAND_KIND_UNPAUSE
	}
	a := await(t, tv, "the TV starts", unpause).GetSyncPlay().GetCommand()
	b := await(t, phone, "the phone starts", unpause).GetSyncPlay().GetCommand()
	if !a.GetWhen().AsTime().Equal(b.GetWhen().AsTime()) || a.GetPosition().AsDuration() != time.Second || b.GetPosition().AsDuration() != time.Second {
		t.Errorf("start commands = %v / %v; want the same time at one second", a, b)
	}
}

// events opens a device's event stream; its events arrive on the channel
// until the test ends.
func events(t *testing.T, url string, token connect.ClientOption) <-chan *sessionv1.Event {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	stream, err := sessionv1connect.NewEventServiceClient(http.DefaultClient, url, token).Subscribe(ctx, &sessionv1.SubscribeRequest{})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	out := make(chan *sessionv1.Event, 256)
	go func() {
		defer close(out)
		for stream.Receive() {
			out <- stream.Msg().GetEvent()
		}
	}()
	return out
}

// await returns the next event matching want within 30 seconds.
func await(t *testing.T, events <-chan *sessionv1.Event, what string, want func(*sessionv1.Event) bool) *sessionv1.Event {
	t.Helper()
	timeout := time.After(30 * time.Second)
	for {
		select {
		case e, ok := <-events:
			if !ok {
				t.Fatalf("%s: the stream ended", what)
			}
			if want(e) {
				return e
			}
		case <-timeout:
			t.Fatalf("%s: no event within 30 seconds", what)
		}
	}
}
