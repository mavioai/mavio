// Package syncplay keeps the groups of devices that watch together, as
// Jellyfin's SyncPlay does: a group plays one queue, waits while any member
// buffers, and starts playing after a delay for the members' latency, so
// that they all play the same position at the same time. Members learn of
// every change through their event streams; a member whose stream ends
// stays in its group for OfflineGrace, which goes on without it meanwhile,
// and catches up with the group when its stream returns.
package syncplay

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"sync"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mavioai/mavio/libs/core"
	sessionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/session/v1"
)

// Timing, as in Jellyfin.
const (
	// DefaultPing is the least delay before a group starts to play.
	DefaultPing = 500 * time.Millisecond
	// MaxPlaybackOffset is how far a member reporting ready may be from
	// the group's position before it is told to seek.
	MaxPlaybackOffset = 500 * time.Millisecond
	// TimeSyncOffset is how far a member's report time may be from the
	// server's before it is ignored as unsynchronized.
	TimeSyncOffset = 2 * time.Second
	// OfflineGrace is how long a member whose event stream ended stays in
	// its group, for a device that reconnects.
	OfflineGrace = time.Minute
	// droppedFor is how long a device dropped from its group while
	// offline is remembered, to tell it so when it returns.
	droppedFor = 24 * time.Hour
)

// Errors of requests.
var (
	ErrNotInGroup = errors.New("syncplay: the device is in no group")
	ErrNoQueue    = errors.New("syncplay: the group has nothing to play")
)

// State is what a group does.
type State int

// Group states.
const (
	Idle State = iota
	Waiting
	Paused
	Playing
)

// Sender delivers an update to a member's device.
type Sender func(session core.ID, update *sessionv1.SyncPlayUpdate)

// Member is a device in a group.
type Member struct {
	Session    core.ID
	User       core.User
	DeviceName string
	Buffering  bool
	Ping       time.Duration
	// Offline says the member's event stream ended; the group does not
	// wait for it.
	Offline bool
}

// Entry is an item of a group's queue.
type Entry struct {
	ID     core.ID
	ItemID core.ID
}

type group struct {
	id      core.ID
	name    string
	state   State
	members []*Member
	queue   []Entry
	current int
	// The group is at position at positionTime; while playing it advances
	// from there.
	position     time.Duration
	positionTime time.Time
	// resume says the group plays once no member buffers; false pauses it.
	resume bool
}

// Manager keeps the groups.
type Manager struct {
	send Sender

	mu        sync.Mutex
	groups    map[core.ID]*group
	bySession map[core.ID]*group
	// leaving are the timers of offline members, which leave when they
	// fire; dropped are the devices that left so, by when.
	leaving map[core.ID]*time.Timer
	dropped map[core.ID]time.Time
}

// New returns a manager sending updates through send.
func New(send Sender) *Manager {
	return &Manager{
		send: send, groups: map[core.ID]*group{}, bySession: map[core.ID]*group{},
		leaving: map[core.ID]*time.Timer{}, dropped: map[core.ID]time.Time{},
	}
}

// Groups returns the groups.
func (m *Manager) Groups() []*sessionv1.SyncPlayGroup {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*sessionv1.SyncPlayGroup, 0, len(m.groups))
	for _, g := range m.groups {
		out = append(out, g.proto())
	}
	return out
}

// Queue returns the item IDs a group plays.
func (m *Manager) Queue(groupID core.ID) ([]core.ID, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	g := m.groups[groupID]
	if g == nil {
		return nil, fmt.Errorf("syncplay group %s: %w", groupID, core.ErrNotFound)
	}
	ids := make([]core.ID, len(g.queue))
	for i, e := range g.queue {
		ids[i] = e.ItemID
	}
	return ids, nil
}

// Members returns the members of a device's group, nil when it is in
// none.
func (m *Manager) Members(session core.ID) []Member {
	m.mu.Lock()
	defer m.mu.Unlock()
	g := m.bySession[session]
	if g == nil {
		return nil
	}
	out := make([]Member, len(g.members))
	for i, mb := range g.members {
		out[i] = *mb
	}
	return out
}

// Create makes a group with the member in it, leaving its earlier group.
func (m *Manager) Create(member Member, name string) *sessionv1.SyncPlayGroup {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.leave(member.Session)
	g := &group{id: core.NewID(), name: name, positionTime: time.Now()}
	m.groups[g.id] = g
	m.join(g, member)
	return g.proto()
}

// Join adds a member to a group, leaving its earlier group.
func (m *Manager) Join(member Member, groupID core.ID) (*sessionv1.SyncPlayGroup, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	g := m.groups[groupID]
	if g == nil {
		return nil, fmt.Errorf("syncplay group %s: %w", groupID, core.ErrNotFound)
	}
	if m.bySession[member.Session] != g {
		m.leave(member.Session)
		m.join(g, member)
	}
	return g.proto(), nil
}

// join adds a member that has to catch up with the group.
func (m *Manager) join(g *group, member Member) {
	delete(m.dropped, member.Session)
	g.members = append(g.members, &member)
	m.bySession[member.Session] = g
	m.catchUp(g, &member, time.Now())
}

// catchUp has the group wait for a member that joined or came back.
func (m *Manager) catchUp(g *group, mb *Member, now time.Time) {
	mb.Buffering = g.state != Idle
	var cmd *sessionv1.SyncPlayCommand
	switch g.state {
	case Playing:
		// The others wait for the member.
		g.position, g.positionTime = g.positionAt(now), now
		g.state, g.resume = Waiting, true
		cmd = g.command(sessionv1.SyncPlayCommandKind_SYNC_PLAY_COMMAND_KIND_PAUSE, now)
	case Paused:
		g.state, g.resume = Waiting, false
	}
	m.broadcast(g, cmd)
}

// Leave removes a device from its group; a group without members ends.
func (m *Manager) Leave(session core.ID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.leave(session)
}

// Offline keeps a device whose event stream ended in its group for
// OfflineGrace; the group stops waiting for it meanwhile.
func (m *Manager) Offline(session core.ID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	g := m.bySession[session]
	if g == nil {
		return
	}
	mb := g.member(session)
	if mb.Offline {
		return
	}
	mb.Offline = true
	var t *time.Timer
	t = time.AfterFunc(OfflineGrace, func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		if m.leaving[session] != t {
			return
		}
		m.leave(session)
		now := time.Now()
		maps.DeleteFunc(m.dropped, func(_ core.ID, at time.Time) bool { return now.Sub(at) > droppedFor })
		m.dropped[session] = now
	})
	m.leaving[session] = t
	if g.state == Waiting && !g.buffering() {
		m.settle(g, time.Now())
	}
}

// Online tells a device whose event stream opened again what it missed:
// a member that was offline catches up with its group, one still online
// is sent the group, and one dropped while offline an empty update.
func (m *Manager) Online(session core.ID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	g := m.bySession[session]
	if g == nil {
		if _, ok := m.dropped[session]; ok {
			delete(m.dropped, session)
			m.send(session, &sessionv1.SyncPlayUpdate{})
		}
		return
	}
	mb := g.member(session)
	if !mb.Offline {
		m.send(session, sessionv1.SyncPlayUpdate_builder{Group: g.proto()}.Build())
		return
	}
	m.leaving[session].Stop()
	delete(m.leaving, session)
	mb.Offline = false
	m.catchUp(g, mb, time.Now())
}

func (m *Manager) leave(session core.ID) {
	g := m.bySession[session]
	if g == nil {
		return
	}
	if t := m.leaving[session]; t != nil {
		t.Stop()
		delete(m.leaving, session)
	}
	delete(m.bySession, session)
	g.members = slices.DeleteFunc(g.members, func(mb *Member) bool { return mb.Session == session })
	m.send(session, &sessionv1.SyncPlayUpdate{})
	if len(g.members) == 0 {
		delete(m.groups, g.id)
		return
	}
	// A buffering member leaving may let the group go on.
	if g.state == Waiting && !g.buffering() {
		m.settle(g, time.Now())
		return
	}
	m.broadcast(g, nil)
}

// Request is a change a member asks of its group.
type Request func(m *Manager, g *group, member *Member, now time.Time) error

// Handle applies a member's request to its group.
func (m *Manager) Handle(session core.ID, r Request) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	g := m.bySession[session]
	if g == nil {
		return ErrNotInGroup
	}
	return r(m, g, g.member(session), time.Now())
}

// SetQueue plays items from an entry and position once all members are
// ready.
func SetQueue(items []core.ID, start int, position time.Duration) Request {
	return func(m *Manager, g *group, _ *Member, now time.Time) error {
		if start < 0 || start >= len(items) {
			return fmt.Errorf("%w: start %d outside the queue", core.ErrInvalid, start)
		}
		g.queue = make([]Entry, len(items))
		for i, id := range items {
			g.queue[i] = Entry{ID: core.NewID(), ItemID: id}
		}
		g.current = start
		m.waitForAll(g, position, true, now, nil)
		return nil
	}
}

// waitForAll moves the group to position and waits for every member.
func (m *Manager) waitForAll(g *group, position time.Duration, resume bool, now time.Time, cmd *sessionv1.SyncPlayCommand) {
	g.position, g.positionTime = position, now
	g.state, g.resume = Waiting, resume
	for _, mb := range g.members {
		mb.Buffering = true
	}
	m.broadcast(g, cmd)
}

// Unpause plays the group: at once when paused, once ready when waiting,
// and from the start of its entry when stopped.
func Unpause() Request {
	return func(m *Manager, g *group, _ *Member, now time.Time) error {
		switch g.state {
		case Idle:
			if len(g.queue) == 0 {
				return ErrNoQueue
			}
			m.waitForAll(g, 0, true, now, nil)
		case Paused:
			m.play(g, now)
		case Waiting:
			g.resume = true
			m.broadcast(g, nil)
		}
		return nil
	}
}

// Pause pauses the group where it is.
func Pause() Request {
	return func(m *Manager, g *group, _ *Member, now time.Time) error {
		switch g.state {
		case Playing:
			g.position, g.positionTime, g.state = g.positionAt(now), now, Paused
			m.broadcast(g, g.command(sessionv1.SyncPlayCommandKind_SYNC_PLAY_COMMAND_KIND_PAUSE, now))
		case Waiting:
			g.resume = false
			m.broadcast(g, nil)
		}
		return nil
	}
}

// Seek moves the group and waits for every member to be ready there.
func Seek(position time.Duration) Request {
	return func(m *Manager, g *group, _ *Member, now time.Time) error {
		if g.state == Idle {
			return ErrNoQueue
		}
		resume := g.state == Playing || g.state == Waiting && g.resume
		g.position, g.positionTime = position, now
		m.waitForAll(g, position, resume, now, g.command(sessionv1.SyncPlayCommandKind_SYNC_PLAY_COMMAND_KIND_SEEK, now))
		return nil
	}
}

// Stop stops the group.
func Stop() Request {
	return func(m *Manager, g *group, _ *Member, now time.Time) error {
		g.state, g.position, g.positionTime = Idle, 0, now
		for _, mb := range g.members {
			mb.Buffering = false
		}
		m.broadcast(g, g.command(sessionv1.SyncPlayCommandKind_SYNC_PLAY_COMMAND_KIND_STOP, now))
		return nil
	}
}

// Step plays the next (+1) or previous (-1) entry of the queue.
func Step(by int) Request {
	return func(m *Manager, g *group, _ *Member, now time.Time) error {
		next := g.current + by
		if next < 0 || next >= len(g.queue) {
			return fmt.Errorf("%w: no entry %d in the queue", core.ErrNotFound, next)
		}
		g.current = next
		m.waitForAll(g, 0, true, now, nil)
		return nil
	}
}

// ReportPing records a member's round trip to the server.
func ReportPing(ping time.Duration) Request {
	return func(_ *Manager, _ *group, mb *Member, _ time.Time) error {
		mb.Ping = ping
		return nil
	}
}

// ReportState records whether a member is ready to play entry at
// position, as it was at when, or buffering.
func ReportState(entry core.ID, position time.Duration, when time.Time, playing, ready bool) Request {
	return func(m *Manager, g *group, mb *Member, now time.Time) error {
		if g.state == Idle {
			return nil
		}
		// A member on another entry is told the queue again.
		if entry != g.queue[g.current].ID {
			mb.Buffering = true
			m.send(mb.Session, sessionv1.SyncPlayUpdate_builder{Group: g.proto()}.Build())
			return nil
		}
		if !ready {
			mb.Buffering = true
			switch g.state {
			case Playing:
				// The others pause where the group is and wait.
				g.position, g.positionTime = g.positionAt(now), now
				g.state, g.resume = Waiting, true
				m.broadcast(g, g.command(sessionv1.SyncPlayCommandKind_SYNC_PLAY_COMMAND_KIND_PAUSE, now))
			case Paused:
				g.state, g.resume = Waiting, false
				m.broadcast(g, nil)
			case Waiting:
				m.broadcast(g, nil)
			}
			return nil
		}
		if g.state != Waiting {
			return nil
		}
		// Where the member is now, from where it was when it reported,
		// unless its clock is off or it was paused.
		elapsed := now.Sub(when)
		if !playing || elapsed > TimeSyncOffset || elapsed < -TimeSyncOffset {
			elapsed = 0
		}
		if off := position + elapsed - g.position; off > MaxPlaybackOffset || off < -MaxPlaybackOffset {
			mb.Buffering = true
			m.send(mb.Session, sessionv1.SyncPlayUpdate_builder{
				Group: g.proto(), Command: g.command(sessionv1.SyncPlayCommandKind_SYNC_PLAY_COMMAND_KIND_SEEK, now),
			}.Build())
			return nil
		}
		mb.Buffering = false
		if g.buffering() {
			m.broadcast(g, nil)
			return nil
		}
		m.settle(g, now)
		return nil
	}
}

// settle ends waiting once no member buffers: playing or paused.
func (m *Manager) settle(g *group, now time.Time) {
	if g.resume {
		m.play(g, now)
		return
	}
	g.state, g.positionTime = Paused, now
	m.broadcast(g, g.command(sessionv1.SyncPlayCommandKind_SYNC_PLAY_COMMAND_KIND_PAUSE, now))
}

// play starts the group after a delay that lets every member receive the
// command: twice the highest ping, at least DefaultPing.
func (m *Manager) play(g *group, now time.Time) {
	delay := DefaultPing
	for _, mb := range g.members {
		delay = max(delay, 2*mb.Ping)
	}
	g.state, g.positionTime = Playing, now.Add(delay)
	m.broadcast(g, g.command(sessionv1.SyncPlayCommandKind_SYNC_PLAY_COMMAND_KIND_UNPAUSE, g.positionTime))
}

// buffering reports whether the group waits for any online member.
func (g *group) buffering() bool {
	return slices.ContainsFunc(g.members, func(mb *Member) bool { return mb.Buffering && !mb.Offline })
}

// member returns a member of the group by its session.
func (g *group) member(session core.ID) *Member {
	return g.members[slices.IndexFunc(g.members, func(mb *Member) bool { return mb.Session == session })]
}

// positionAt is where the group is at t.
func (g *group) positionAt(t time.Time) time.Duration {
	if g.state == Playing && t.After(g.positionTime) {
		return g.position + t.Sub(g.positionTime)
	}
	return g.position
}

// command tells members to act at when, at the group's position then.
func (g *group) command(kind sessionv1.SyncPlayCommandKind, when time.Time) *sessionv1.SyncPlayCommand {
	b := sessionv1.SyncPlayCommand_builder{
		Kind: &kind, When: timestamppb.New(when), Position: durationpb.New(g.positionAt(when)),
	}
	if len(g.queue) > 0 {
		b.EntryId = new(g.queue[g.current].ID.String())
	}
	return b.Build()
}

// broadcast sends the group, and cmd if any, to every member.
func (m *Manager) broadcast(g *group, cmd *sessionv1.SyncPlayCommand) {
	update := sessionv1.SyncPlayUpdate_builder{Group: g.proto(), Command: cmd}.Build()
	for _, mb := range g.members {
		m.send(mb.Session, update)
	}
}

var states = map[State]sessionv1.SyncPlayState{
	Idle: sessionv1.SyncPlayState_SYNC_PLAY_STATE_IDLE, Waiting: sessionv1.SyncPlayState_SYNC_PLAY_STATE_WAITING,
	Paused: sessionv1.SyncPlayState_SYNC_PLAY_STATE_PAUSED, Playing: sessionv1.SyncPlayState_SYNC_PLAY_STATE_PLAYING,
}

func (g *group) proto() *sessionv1.SyncPlayGroup {
	b := sessionv1.SyncPlayGroup_builder{
		Id: new(g.id.String()), Name: &g.name, State: new(states[g.state]), CurrentIndex: new(int32(g.current)),
		Position: durationpb.New(g.position), PositionTime: timestamppb.New(g.positionTime),
	}
	for _, mb := range g.members {
		b.Members = append(b.Members, sessionv1.SyncPlayMember_builder{
			SessionId: new(mb.Session.String()), UserName: &mb.User.Name, DeviceName: &mb.DeviceName, Buffering: &mb.Buffering,
		}.Build())
	}
	for _, e := range g.queue {
		b.Queue = append(b.Queue, sessionv1.SyncPlayQueueEntry_builder{Id: new(e.ID.String()), ItemId: new(e.ItemID.String())}.Build())
	}
	return b.Build()
}
