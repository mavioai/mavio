package httpserver_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/mavioai/mavio/libs/core"
	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1/libraryv1connect"
	sessionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/session/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/session/v1/sessionv1connect"
	userv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1/userv1connect"
)

// subscribe opens a device's event stream; its events arrive on the
// channel until the test ends.
func subscribe(t *testing.T, url, token string) <-chan *sessionv1.Event {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	stream, err := sessionv1connect.NewEventServiceClient(http.DefaultClient, url, withToken(token)).Subscribe(ctx, &sessionv1.SubscribeRequest{})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	out := make(chan *sessionv1.Event, 64)
	go func() {
		defer close(out)
		for stream.Receive() {
			out <- stream.Msg().GetEvent()
		}
	}()
	return out
}

// await returns the next event matching want within five seconds.
func await(t *testing.T, events <-chan *sessionv1.Event, what string, want func(*sessionv1.Event) bool) *sessionv1.Event {
	t.Helper()
	timeout := time.After(5 * time.Second)
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
			t.Fatalf("%s: no event within five seconds", what)
		}
	}
}

func TestEventsAndSessions(t *testing.T) {
	ctx := t.Context()
	url, s := startServer(t, nil)
	tvToken := signUp(t, url)
	phoneToken := login(t, url, "admin", "secret", "phone")
	phone := subscribe(t, url, phoneToken)
	phoneSession := await(t, phone, "connected", (*sessionv1.Event).HasConnected).GetConnected().GetSessionId()

	// The phone is online, the TV is not.
	sessions := sessionv1connect.NewSessionServiceClient(http.DefaultClient, url, withToken(tvToken))
	list, err := sessions.ListSessions(ctx, &sessionv1.ListSessionsRequest{})
	if err != nil || len(list.GetSessions()) != 2 {
		t.Fatalf("ListSessions = %v, %v", list, err)
	}
	if first := list.GetSessions()[0]; first.GetId() != phoneSession || !first.GetOnline() || first.GetDeviceId() != "phone" || list.GetSessions()[1].GetOnline() {
		t.Errorf("sessions = %v", list.GetSessions())
	}

	// The TV controls the phone.
	if _, err := sessions.SendCommand(ctx, sessionv1.SendCommandRequest_builder{
		SessionId: &phoneSession,
		Command:   sessionv1.Command_builder{Seek: sessionv1.Seek_builder{Position: durationpb.New(90 * time.Second)}.Build()}.Build(),
	}.Build()); err != nil {
		t.Fatal(err)
	}
	cmd := await(t, phone, "command", (*sessionv1.Event).HasCommand).GetCommand()
	if cmd.GetCommand().GetSeek().GetPosition().AsDuration() != 90*time.Second || cmd.GetFromUserName() != "admin" {
		t.Errorf("command = %v", cmd)
	}
	tvSession := list.GetSessions()[1].GetId()
	_, err = sessions.SendCommand(ctx, sessionv1.SendCommandRequest_builder{
		SessionId: &tvSession,
		Command:   sessionv1.Command_builder{Message: sessionv1.Message_builder{Text: new("hello")}.Build()}.Build(),
	}.Build())
	wantCode(t, "command to an offline device", err, connect.CodeFailedPrecondition)

	// A library created on the TV reaches the phone after the delay.
	libraries := libraryv1connect.NewLibraryServiceClient(http.DefaultClient, url, withToken(tvToken))
	created, err := libraries.CreateLibrary(ctx, libraryv1.CreateLibraryRequest_builder{Spec: libraryv1.LibrarySpec_builder{
		Name: new("Films"), Kind: new(libraryv1.LibraryKind_LIBRARY_KIND_MOVIES), Paths: []string{t.TempDir()},
	}.Build()}.Build())
	if err != nil {
		t.Fatal(err)
	}
	changed := await(t, phone, "library change", (*sessionv1.Event).HasLibraryChanged).GetLibraryChanged()
	if ids := changed.GetLibraryIds(); len(ids) != 1 || ids[0] != created.GetLibrary().GetId() {
		t.Errorf("library change = %v", changed)
	}

	// Another user may neither see the administrator's sessions nor
	// control them, and learns nothing of the films library.
	users := userv1connect.NewUserServiceClient(http.DefaultClient, url, withToken(tvToken))
	if _, err := users.CreateUser(ctx, userv1.CreateUserRequest_builder{
		Name: new("kid"), Password: new("pw"), Policy: userv1.UserPolicy_builder{AllLibraries: new(false)}.Build(),
	}.Build()); err != nil {
		t.Fatal(err)
	}
	kidToken := login(t, url, "kid", "pw", "tablet")
	kidEvents := subscribe(t, url, kidToken)
	await(t, kidEvents, "connected", (*sessionv1.Event).HasConnected)
	kid := sessionv1connect.NewSessionServiceClient(http.DefaultClient, url, withToken(kidToken))
	if list, err := kid.ListSessions(ctx, &sessionv1.ListSessionsRequest{}); err != nil || len(list.GetSessions()) != 1 {
		t.Errorf("kid's sessions = %v, %v", list, err)
	}
	_, err = kid.ListSessions(ctx, sessionv1.ListSessionsRequest_builder{AllUsers: new(true)}.Build())
	wantCode(t, "kid lists all sessions", err, connect.CodePermissionDenied)
	_, err = kid.SendCommand(ctx, sessionv1.SendCommandRequest_builder{
		SessionId: &phoneSession,
		Command:   sessionv1.Command_builder{Message: sessionv1.Message_builder{Text: new("hi")}.Build()}.Build(),
	}.Build())
	wantCode(t, "kid controls the admin's phone", err, connect.CodeNotFound)
	// The administrator sees every device, the kid's tablet online.
	all, err := sessions.ListSessions(ctx, sessionv1.ListSessionsRequest_builder{AllUsers: new(true)}.Build())
	if err != nil || len(all.GetSessions()) != 3 || all.GetSessions()[1].GetUserName() == "" {
		t.Errorf("all sessions = %v, %v", all, err)
	}

	// The TV's state of an item reaches the phone, not the kid.
	film := core.Item{ID: core.NewID(), LibraryID: core.MustParseID(created.GetLibrary().GetId()), Kind: core.KindMovie, Name: "Up"}
	if err := s.Items().Upsert(ctx, film); err != nil {
		t.Fatal(err)
	}
	movie := film.ID.String()
	data := userv1connect.NewUserDataServiceClient(http.DefaultClient, url, withToken(tvToken))
	if _, err := data.UpdateUserData(ctx, userv1.UpdateUserDataRequest_builder{ItemId: &movie, Favorite: new(true)}.Build()); err != nil {
		t.Fatal(err)
	}
	got := await(t, phone, "user data", (*sessionv1.Event).HasUserDataChanged).GetUserDataChanged()
	if d := got.GetUserData(); len(d) != 1 || d[0].GetItemId() != movie || !d[0].GetFavorite() {
		t.Errorf("user data = %v", got)
	}
	select {
	case e := <-kidEvents:
		if e.HasUserDataChanged() || e.HasLibraryChanged() {
			t.Errorf("kid got %v", e)
		}
	case <-time.After(300 * time.Millisecond):
	}
}

func TestSyncPlay(t *testing.T) {
	ctx := t.Context()
	url, s := startServer(t, nil)
	tvToken := signUp(t, url)
	phoneToken := login(t, url, "admin", "secret", "phone")
	lib := core.Library{Name: "Films", Kind: core.LibraryMovies, Paths: []string{t.TempDir()}}
	if err := s.Libraries().Create(ctx, &lib); err != nil {
		t.Fatal(err)
	}
	film := core.Item{ID: core.NewID(), LibraryID: lib.ID, Kind: core.KindMovie, Name: "Up"}
	if err := s.Items().Upsert(ctx, film); err != nil {
		t.Fatal(err)
	}
	tvSync := sessionv1connect.NewSyncPlayServiceClient(http.DefaultClient, url, withToken(tvToken))
	phoneSync := sessionv1connect.NewSyncPlayServiceClient(http.DefaultClient, url, withToken(phoneToken))
	isSync := func(e *sessionv1.Event) bool { return e.HasSyncPlay() }

	// Joining needs an event stream to learn of the group.
	_, err := tvSync.CreateGroup(ctx, sessionv1.CreateGroupRequest_builder{Name: new("Movie night")}.Build())
	wantCode(t, "create a group offline", err, connect.CodeFailedPrecondition)
	tv, phoneCtx := subscribe(t, url, tvToken), context.Background()
	phoneCtx, hangUp := context.WithCancel(phoneCtx)
	phoneStream, err := sessionv1connect.NewEventServiceClient(http.DefaultClient, url, withToken(phoneToken)).Subscribe(phoneCtx, &sessionv1.SubscribeRequest{})
	if err != nil {
		t.Fatal(err)
	}
	phone := make(chan *sessionv1.Event, 64)
	go func() {
		defer close(phone)
		for phoneStream.Receive() {
			phone <- phoneStream.Msg().GetEvent()
		}
	}()
	await(t, tv, "TV connected", (*sessionv1.Event).HasConnected)
	await(t, phone, "phone connected", (*sessionv1.Event).HasConnected)

	created, err := tvSync.CreateGroup(ctx, sessionv1.CreateGroupRequest_builder{Name: new("Movie night")}.Build())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := phoneSync.JoinGroup(ctx, sessionv1.JoinGroupRequest_builder{GroupId: new(created.GetGroup().GetId())}.Build()); err != nil {
		t.Fatal(err)
	}
	if list, err := phoneSync.ListGroups(ctx, &sessionv1.ListGroupsRequest{}); err != nil || len(list.GetGroups()) != 1 || len(list.GetGroups()[0].GetMembers()) != 2 {
		t.Fatalf("ListGroups = %v, %v", list, err)
	}

	// Both get the queue, report ready, and are told to start at the same
	// time and position.
	if _, err := tvSync.SetQueue(ctx, sessionv1.SetQueueRequest_builder{ItemIds: []string{film.ID.String()}}.Build()); err != nil {
		t.Fatal(err)
	}
	queued := await(t, phone, "queue", func(e *sessionv1.Event) bool { return isSync(e) && len(e.GetSyncPlay().GetGroup().GetQueue()) == 1 })
	entry := queued.GetSyncPlay().GetGroup().GetQueue()[0].GetId()
	for _, c := range []sessionv1connect.SyncPlayServiceClient{tvSync, phoneSync} {
		now, err := c.GetTime(ctx, &sessionv1.GetTimeRequest{})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := c.ReportState(ctx, sessionv1.ReportStateRequest_builder{
			EntryId: &entry, Position: durationpb.New(0), When: now.GetSendTime(), Ready: new(true),
		}.Build()); err != nil {
			t.Fatal(err)
		}
	}
	unpause := func(e *sessionv1.Event) bool {
		return isSync(e) && e.GetSyncPlay().GetCommand().GetKind() == sessionv1.SyncPlayCommandKind_SYNC_PLAY_COMMAND_KIND_UNPAUSE
	}
	a, b := await(t, tv, "TV starts", unpause).GetSyncPlay().GetCommand(), await(t, phone, "phone starts", unpause).GetSyncPlay().GetCommand()
	if !a.GetWhen().AsTime().Equal(b.GetWhen().AsTime()) || a.GetPosition().AsDuration() != b.GetPosition().AsDuration() {
		t.Errorf("start commands differ: %v / %v", a, b)
	}

	// The phone going offline leaves the group.
	hangUp()
	left := await(t, tv, "phone left", func(e *sessionv1.Event) bool { return isSync(e) && len(e.GetSyncPlay().GetGroup().GetMembers()) == 1 })
	if left.GetSyncPlay().GetGroup().GetState() != sessionv1.SyncPlayState_SYNC_PLAY_STATE_PLAYING {
		t.Errorf("group after the phone left = %v", left)
	}
	_, err = phoneSync.Pause(ctx, &sessionv1.PauseRequest{})
	wantCode(t, "pause outside a group", err, connect.CodeFailedPrecondition)
}
