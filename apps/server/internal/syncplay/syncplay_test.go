package syncplay

import (
	"errors"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/mavioai/mavio/libs/core"
	sessionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/session/v1"
)

// inbox records the updates sent to each device.
type inbox struct {
	mu      sync.Mutex
	updates map[core.ID][]*sessionv1.SyncPlayUpdate
}

func (in *inbox) send(session core.ID, u *sessionv1.SyncPlayUpdate) {
	in.mu.Lock()
	defer in.mu.Unlock()
	in.updates[session] = append(in.updates[session], u)
}

// last returns a device's last update and forgets its updates.
func (in *inbox) last(session core.ID) *sessionv1.SyncPlayUpdate {
	in.mu.Lock()
	defer in.mu.Unlock()
	list := in.updates[session]
	in.updates[session] = nil
	if len(list) == 0 {
		return nil
	}
	return list[len(list)-1]
}

func member(name string) Member {
	return Member{Session: core.NewID(), User: core.User{ID: core.NewID(), Name: name}, DeviceName: name + "'s TV"}
}

func TestWatchTogether(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		in := &inbox{updates: map[core.ID][]*sessionv1.SyncPlayUpdate{}}
		m := New(in.send)
		ann, bob := member("ann"), member("bob")
		g := m.Create(ann, "Movie night")
		if _, err := m.Join(bob, core.MustParseID(g.GetId())); err != nil {
			t.Fatal(err)
		}
		if len(m.Groups()) != 1 || len(m.Groups()[0].GetMembers()) != 2 {
			t.Fatalf("groups = %v", m.Groups())
		}
		handle := func(mb Member, r Request) {
			t.Helper()
			if err := m.Handle(mb.Session, r); err != nil {
				t.Fatal(err)
			}
		}
		state := func(u *sessionv1.SyncPlayUpdate) sessionv1.SyncPlayState { return u.GetGroup().GetState() }
		const waiting, playing, paused = sessionv1.SyncPlayState_SYNC_PLAY_STATE_WAITING, sessionv1.SyncPlayState_SYNC_PLAY_STATE_PLAYING, sessionv1.SyncPlayState_SYNC_PLAY_STATE_PAUSED
		const unpause, pause, seek = sessionv1.SyncPlayCommandKind_SYNC_PLAY_COMMAND_KIND_UNPAUSE, sessionv1.SyncPlayCommandKind_SYNC_PLAY_COMMAND_KIND_PAUSE, sessionv1.SyncPlayCommandKind_SYNC_PLAY_COMMAND_KIND_SEEK

		// Ann queues a film at one minute; the group waits for both.
		film := core.NewID()
		handle(ann, SetQueue([]core.ID{film, core.NewID()}, 0, time.Minute))
		u := in.last(bob.Session)
		if state(u) != waiting || u.GetGroup().GetPosition().AsDuration() != time.Minute || len(u.GetGroup().GetQueue()) != 2 {
			t.Fatalf("after SetQueue = %v", u)
		}
		entry := core.MustParseID(u.GetGroup().GetQueue()[0].GetId())
		in.last(ann.Session)

		// Bob is slow; the delay before playing covers his ping twice.
		handle(bob, ReportPing(400*time.Millisecond))
		handle(ann, ReportState(entry, time.Minute, time.Now(), false, true))
		if u := in.last(bob.Session); state(u) != waiting || u.GetCommand() != nil {
			t.Errorf("one ready = %v", u)
		}
		handle(bob, ReportState(entry, time.Minute, time.Now(), false, true))
		start := time.Now().Add(800 * time.Millisecond)
		for _, mb := range []Member{ann, bob} {
			u := in.last(mb.Session)
			cmd := u.GetCommand()
			if state(u) != playing || cmd.GetKind() != unpause || !cmd.GetWhen().AsTime().Equal(start) || cmd.GetPosition().AsDuration() != time.Minute {
				t.Errorf("%s starts with %v", mb.User.Name, u)
			}
		}

		// Ten seconds into playing, Bob buffers: Ann pauses where the
		// group is.
		time.Sleep(10*time.Second + 800*time.Millisecond)
		handle(bob, ReportState(entry, time.Minute+10*time.Second, time.Now(), true, false))
		u = in.last(ann.Session)
		if state(u) != waiting || u.GetCommand().GetKind() != pause || u.GetCommand().GetPosition().AsDuration() != time.Minute+10*time.Second {
			t.Errorf("Ann while Bob buffers = %v", u)
		}
		// Bob reports ready too far behind: he is told to seek there.
		handle(bob, ReportState(entry, time.Minute+5*time.Second, time.Now(), false, true))
		if u := in.last(bob.Session); u.GetCommand().GetKind() != seek || u.GetCommand().GetPosition().AsDuration() != time.Minute+10*time.Second {
			t.Errorf("Bob behind = %v", u)
		}
		// Ready at the group's position, reported 200 ms ago while playing,
		// is within the offset.
		time.Sleep(time.Second)
		handle(bob, ReportState(entry, time.Minute+9800*time.Millisecond, time.Now().Add(-200*time.Millisecond), true, true))
		if u := in.last(ann.Session); state(u) != playing || u.GetCommand().GetKind() != unpause || u.GetCommand().GetPosition().AsDuration() != time.Minute+10*time.Second {
			t.Errorf("Ann after Bob caught up = %v", u)
		}
		in.last(bob.Session)

		// Both stay at the same position: a pause lands on both alike.
		time.Sleep(20 * time.Second)
		handle(ann, Pause())
		a, b := in.last(ann.Session), in.last(bob.Session)
		if state(a) != paused || a.GetCommand().GetKind() != pause || a.GetCommand().GetPosition().AsDuration() != b.GetCommand().GetPosition().AsDuration() ||
			a.GetCommand().GetPosition().AsDuration() != time.Minute+10*time.Second+20*time.Second-800*time.Millisecond {
			t.Errorf("pause = %v / %v", a, b)
		}

		// A seek while paused waits for both and stays paused.
		handle(bob, Seek(2*time.Minute))
		if u := in.last(ann.Session); state(u) != waiting || u.GetCommand().GetKind() != seek {
			t.Errorf("seek = %v", u)
		}
		for _, mb := range []Member{ann, bob} {
			handle(mb, ReportState(entry, 2*time.Minute, time.Now(), false, true))
		}
		if u := in.last(ann.Session); state(u) != paused {
			t.Errorf("after seeking while paused = %v", u)
		}

		// A newcomer makes the group wait; leaving while buffering lets it
		// go on.
		in.last(bob.Session)
		handle(ann, Unpause())
		cid := member("cid")
		if _, err := m.Join(cid, core.MustParseID(g.GetId())); err != nil {
			t.Fatal(err)
		}
		if u := in.last(ann.Session); state(u) != waiting || u.GetCommand().GetKind() != pause {
			t.Errorf("after Cid joined = %v", u)
		}
		m.Leave(cid.Session)
		if u := in.last(cid.Session); u.HasGroup() {
			t.Errorf("Cid's last update = %v, want leaving", u)
		}
		if u := in.last(ann.Session); state(u) != playing || u.GetCommand().GetKind() != unpause {
			t.Errorf("after Cid left = %v", u)
		}

		// The next entry waits for everyone again; past the end is not
		// found.
		handle(ann, Step(1))
		if u := in.last(bob.Session); state(u) != waiting || u.GetGroup().GetCurrentIndex() != 1 {
			t.Errorf("next = %v", u)
		}
		if err := m.Handle(ann.Session, Step(1)); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("past the end: %v, want ErrNotFound", err)
		}
		if err := m.Handle(cid.Session, Pause()); !errors.Is(err, ErrNotInGroup) {
			t.Errorf("outside a group: %v, want ErrNotInGroup", err)
		}

		// The last member leaving ends the group.
		m.Leave(ann.Session)
		m.Leave(bob.Session)
		if len(m.Groups()) != 0 {
			t.Errorf("groups = %v, want none", m.Groups())
		}
	})
}
