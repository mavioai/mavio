// Package events delivers what changes to the event streams of signed-in
// devices: library changes, users' item states, sessions, remote commands,
// scans and plugins. A Hub keeps the streams; Observe wraps the store so
// that what is written reaches the hub once committed.
package events

import (
	"bytes"
	"context"
	"errors"
	"iter"
	"log/slog"
	"maps"
	"slices"
	"sync"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mavioai/mavio/libs/core"
	sessionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/session/v1"
	userv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1"
)

// buffer is the number of events a stream may lag behind; a stream
// further behind is dropped, and its client reconnects.
const buffer = 256

// Config configures a Hub.
type Config struct {
	// Store loads the changed items to tell who may see them.
	Store core.Store
	// LibraryDelay gathers library changes for this long before sending
	// them; default five seconds.
	LibraryDelay time.Duration
	Logger       *slog.Logger
}

// Hub keeps the event streams of the signed-in devices.
type Hub struct {
	cfg Config
	log *slog.Logger

	mu   sync.Mutex
	subs map[core.ID]*Subscriber // by session
	// pending library changes, sent when the timer fires.
	timer   *time.Timer
	libs    map[core.ID]bool
	changed map[core.ID]bool
	removed map[core.ID]core.ID // item → library
	// scans are the running scan jobs, by job ID.
	scans map[core.ID]core.Job
	// offline are told of sessions going offline.
	offline []func(session core.ID)
}

// OnOffline registers f to learn of sessions going offline.
func (h *Hub) OnOffline(f func(session core.ID)) {
	h.mu.Lock()
	h.offline = append(h.offline, f)
	h.mu.Unlock()
}

// New returns a hub.
func New(cfg Config) *Hub {
	if cfg.LibraryDelay <= 0 {
		cfg.LibraryDelay = 5 * time.Second
	}
	log := cfg.Logger
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	h := &Hub{cfg: cfg, log: log, subs: map[core.ID]*Subscriber{}, scans: map[core.ID]core.Job{}}
	h.resetPending()
	return h
}

func (h *Hub) resetPending() {
	h.libs, h.changed, h.removed = map[core.ID]bool{}, map[core.ID]bool{}, map[core.ID]core.ID{}
}

// Subscriber is the event stream of one signed-in device.
type Subscriber struct {
	User      core.User
	SessionID core.ID
	ch        chan *sessionv1.Event
	done      chan struct{}
	closeOnce sync.Once
}

// Events delivers the stream's events.
func (s *Subscriber) Events() <-chan *sessionv1.Event { return s.ch }

// Done is closed when the stream ends on the server's side: replaced by a
// newer stream of the device, or too far behind.
func (s *Subscriber) Done() <-chan struct{} { return s.done }

func (s *Subscriber) close() { s.closeOnce.Do(func() { close(s.done) }) }

// Subscribe opens the event stream of a device, replacing its earlier one.
func (h *Hub) Subscribe(u core.User, session core.ID) *Subscriber {
	s := &Subscriber{User: u, SessionID: session, ch: make(chan *sessionv1.Event, buffer), done: make(chan struct{})}
	h.mu.Lock()
	old := h.subs[session]
	h.subs[session] = s
	h.mu.Unlock()
	if old != nil {
		old.close()
	}
	h.SessionsChanged(u.ID, session)
	return s
}

// Unsubscribe closes a stream unless a newer one replaced it.
func (h *Hub) Unsubscribe(s *Subscriber) {
	h.mu.Lock()
	current := h.subs[s.SessionID] == s
	if current {
		delete(h.subs, s.SessionID)
	}
	offline := slices.Clone(h.offline)
	h.mu.Unlock()
	s.close()
	if current {
		for _, f := range offline {
			f(s.SessionID)
		}
		h.SessionsChanged(s.User.ID, s.SessionID)
	}
}

// SessionUser returns the user of a session with an event stream open.
func (h *Hub) SessionUser(session core.ID) (core.User, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if s := h.subs[session]; s != nil {
		return s.User, true
	}
	return core.User{}, false
}

// OnlineSessions returns the sessions with an event stream open, by user.
func (h *Hub) OnlineSessions() map[core.ID][]core.ID {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := map[core.ID][]core.ID{}
	for id, s := range h.subs {
		out[s.User.ID] = append(out[s.User.ID], id)
	}
	return out
}

// Online reports whether a session has an event stream open.
func (h *Hub) Online(session core.ID) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.subs[session] != nil
}

// subscribers returns the open streams that want, at once.
func (h *Hub) subscribers(want func(*Subscriber) bool) []*Subscriber {
	h.mu.Lock()
	defer h.mu.Unlock()
	var out []*Subscriber
	for _, s := range h.subs {
		if want(s) {
			out = append(out, s)
		}
	}
	return out
}

// send queues an event on a stream, dropping a stream too far behind.
func (h *Hub) send(s *Subscriber, e *sessionv1.Event) {
	if !e.HasTime() {
		e.SetTime(timestamppb.Now())
	}
	select {
	case s.ch <- e:
	case <-s.done:
	default:
		h.log.Warn("event stream too far behind; dropped", "session", s.SessionID)
		h.Unsubscribe(s)
	}
}

// ToSession sends an event to one session; it reports false when the
// session is not online.
func (h *Hub) ToSession(session core.ID, e *sessionv1.Event) bool {
	subs := h.subscribers(func(s *Subscriber) bool { return s.SessionID == session })
	for _, s := range subs {
		h.send(s, e)
	}
	return len(subs) > 0
}

// SessionsChanged tells a user's devices and the administrators that a
// session came online, went offline or changed its playback.
func (h *Hub) SessionsChanged(userID, session core.ID) {
	e := sessionv1.Event_builder{SessionsChanged: sessionv1.SessionsChanged_builder{SessionId: new(session.String())}.Build()}.Build()
	for _, s := range h.subscribers(func(s *Subscriber) bool { return s.User.ID == userID || s.User.Admin }) {
		h.send(s, e)
	}
}

// UserDataChanged sends a user's new state of an item to their devices.
func (h *Hub) UserDataChanged(d core.UserData) {
	e := sessionv1.Event_builder{UserDataChanged: sessionv1.UserDataChanged_builder{
		UserData: []*userv1.UserData{UserDataToProto(&d)},
	}.Build()}.Build()
	for _, s := range h.subscribers(func(s *Subscriber) bool { return s.User.ID == d.UserID }) {
		h.send(s, e)
	}
}

// ToAdmins sends an event to the administrators' devices.
func (h *Hub) ToAdmins(e *sessionv1.Event) {
	for _, s := range h.subscribers(func(s *Subscriber) bool { return s.User.Admin }) {
		h.send(s, e)
	}
}

// scanJob is the kind of jobs whose states administrators follow.
const scanJob = "library.scan"

// JobLeased tells administrators that a scan started.
func (h *Hub) JobLeased(job core.Job) {
	if job.Kind != scanJob {
		return
	}
	h.mu.Lock()
	h.scans[job.ID] = job
	h.mu.Unlock()
	h.jobChanged(job, sessionv1.JobState_JOB_STATE_RUNNING)
}

// JobEnded tells administrators that a scan succeeded or failed.
func (h *Hub) JobEnded(id core.ID, failed bool) {
	h.mu.Lock()
	job, ok := h.scans[id]
	delete(h.scans, id)
	h.mu.Unlock()
	if !ok {
		return
	}
	state := sessionv1.JobState_JOB_STATE_SUCCEEDED
	if failed {
		state = sessionv1.JobState_JOB_STATE_FAILED
	}
	h.jobChanged(job, state)
}

func (h *Hub) jobChanged(job core.Job, state sessionv1.JobState) {
	b := sessionv1.JobChanged_builder{JobId: new(job.ID.String()), Kind: &job.Kind, State: &state}
	if lib := libraryOfJob(job); !lib.IsZero() {
		b.LibraryId = new(lib.String())
	}
	h.ToAdmins(sessionv1.Event_builder{JobChanged: b.Build()}.Build())
}

// ItemsChanged records changed and removed items and changed libraries,
// sent together after the library delay. removed maps items to their
// libraries.
func (h *Hub) ItemsChanged(libs []core.ID, changed []core.ID, removed map[core.ID]core.ID) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, id := range libs {
		h.libs[id] = true
	}
	for _, id := range changed {
		h.changed[id] = true
	}
	for id, lib := range removed {
		h.removed[id] = lib
		h.libs[lib] = true
		delete(h.changed, id)
	}
	if h.timer == nil {
		h.timer = time.AfterFunc(h.cfg.LibraryDelay, h.flush)
	}
}

// flush sends the gathered library changes, each subscriber the items and
// libraries it may see.
func (h *Hub) flush() {
	h.mu.Lock()
	libs, changed, removed := h.libs, h.changed, h.removed
	h.resetPending()
	h.timer = nil
	h.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	items := map[core.ID]core.Item{}
	for id := range changed {
		it, err := h.cfg.Store.Items().Get(ctx, id)
		switch {
		case errors.Is(err, core.ErrNotFound):
		case err != nil:
			h.log.Warn("load changed item", "item", id, "err", err)
		default:
			items[id] = it
			libs[it.LibraryID] = true
		}
	}
	kinds := map[core.ID]core.LibraryKind{}
	for id := range libs {
		if lib, err := h.cfg.Store.Libraries().Get(ctx, id); err == nil {
			kinds[id] = lib.Kind
		}
	}
	// Every user has the playlists library, holding their own playlists.
	canSeeLibrary := func(u *core.User, lib core.ID) bool {
		return kinds[lib] == core.LibraryPlaylists || u.Policy.CanAccessLibrary(lib)
	}
	for _, s := range h.subscribers(func(*Subscriber) bool { return true }) {
		b := sessionv1.LibraryChanged_builder{}
		for _, id := range sortedIDs(maps.Keys(libs)) {
			if canSeeLibrary(&s.User, id) {
				b.LibraryIds = append(b.LibraryIds, id.String())
			}
		}
		for _, id := range sortedIDs(maps.Keys(items)) {
			if it := items[id]; s.User.CanAccess(&it) {
				b.ChangedItemIds = append(b.ChangedItemIds, id.String())
			}
		}
		for _, id := range sortedIDs(maps.Keys(removed)) {
			if canSeeLibrary(&s.User, removed[id]) {
				b.RemovedItemIds = append(b.RemovedItemIds, id.String())
			}
		}
		if len(b.LibraryIds) > 0 {
			h.send(s, sessionv1.Event_builder{LibraryChanged: b.Build()}.Build())
		}
	}
}

// sortedIDs returns IDs in order, for events that read the same each time.
func sortedIDs(ids iter.Seq[core.ID]) []core.ID {
	return slices.SortedFunc(ids, func(a, b core.ID) int { return bytes.Compare(a[:], b[:]) })
}

// UserDataToProto describes a user's state of an item.
func UserDataToProto(d *core.UserData) *userv1.UserData {
	b := userv1.UserData_builder{
		ItemId:    new(d.ItemID.String()),
		Played:    &d.Played,
		PlayCount: new(int32(d.PlayCount)),
		Favorite:  &d.Favorite,
		Rating:    d.Rating,
	}
	if d.Position > 0 {
		b.Position = durationpb.New(d.Position)
	}
	if d.AudioStream != nil {
		b.AudioStream = new(int32(*d.AudioStream))
	}
	if d.SubtitleStream != nil {
		b.SubtitleStream = new(int32(*d.SubtitleStream))
	}
	if d.LastPlayedAt != nil {
		b.LastPlayedTime = timestamppb.New(*d.LastPlayedAt)
	}
	return b.Build()
}
