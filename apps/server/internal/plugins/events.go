package plugins

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/plugin/manifest"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

// Event delivery limits; see docs/plugins.md §4.2.
const (
	// eventQueueSize bounds the events waiting for one plugin.
	eventQueueSize = 10_000
	// eventBatchSize bounds the events of one Consume call.
	eventBatchSize = 100
	// eventBatchDelay is how long events are gathered before a delivery.
	eventBatchDelay = time.Second
	// eventRetries is how often a failed batch is retried.
	eventRetries = 3
	// eventTimeout bounds one Consume call.
	eventTimeout = 30 * time.Second
)

// Publish queues an event for the event consumers whose patterns match
// its type. It never blocks: events beyond a plugin's queue are dropped.
func (m *Manager) Publish(a core.Activity) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, e := range m.plugins {
		id := e.manifest.GetId()
		if e.plugin == nil || !manifest.HasCapability(e.manifest, pluginv1.Capability_CAPABILITY_EVENT_CONSUMER) ||
			!manifest.ConsumesEvent(e.manifest, a.Type) {
			continue
		}
		q := m.queues[id]
		if q == nil {
			q = newEventQueue(id, m.log, func(ctx context.Context, events []core.Activity) error {
				return m.consume(ctx, id, events)
			})
			m.queues[id] = q
			go q.run(m.ctx)
		}
		q.push(a)
	}
}

// Wants reports whether a started event consumer takes events of a type.
func (m *Manager) Wants(eventType string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, e := range m.plugins {
		if e.plugin != nil && manifest.HasCapability(e.manifest, pluginv1.Capability_CAPABILITY_EVENT_CONSUMER) &&
			manifest.ConsumesEvent(e.manifest, eventType) {
			return true
		}
	}
	return false
}

// consume delivers a batch to a plugin; a plugin that is not ready drops
// it.
func (m *Manager) consume(ctx context.Context, id string, events []core.Activity) error {
	pl, ok := m.running(id)
	if !ok || pl.Events() == nil {
		m.log.DebugContext(ctx, "dropped events of a plugin that is not ready", "plugin", id, "count", len(events))
		return nil
	}
	req := &pluginv1.ConsumeRequest{}
	list := make([]*pluginv1.Event, len(events))
	for i := range events {
		list[i] = eventToProto(&events[i])
	}
	req.SetEvents(list)
	ctx, cancel := context.WithTimeout(ctx, eventTimeout)
	defer cancel()
	_, err := pl.Events().Consume(ctx, req)
	return err
}

// dropQueue stops delivering to a plugin, dropping what it waits for.
// m.mu must be held.
func (m *Manager) dropQueue(id string) {
	if q := m.queues[id]; q != nil {
		q.stop()
		delete(m.queues, id)
	}
}

// eventToProto describes an event to plugins.
func eventToProto(a *core.Activity) *pluginv1.Event {
	ev := pluginv1.Event_builder{
		Id: new(a.ID.String()), Type: &a.Type, Time: timestamppb.New(a.Time), Title: &a.Title, Message: &a.Message,
		Attributes: a.Attributes,
	}.Build()
	if !a.UserID.IsZero() {
		ev.SetUserId(a.UserID.String())
	}
	if !a.ItemID.IsZero() {
		ev.SetItemId(a.ItemID.String())
	}
	return ev
}

// eventQueue delivers one plugin's events in order, in batches.
type eventQueue struct {
	id      string
	log     *slog.Logger
	consume func(ctx context.Context, events []core.Activity) error
	// wake is signaled when events arrive; done when the queue stops.
	wake, done chan struct{}
	stopOnce   sync.Once

	mu     sync.Mutex
	events []core.Activity
	// full is set while events are being dropped, to warn once.
	full bool
}

func newEventQueue(id string, log *slog.Logger, consume func(context.Context, []core.Activity) error) *eventQueue {
	return &eventQueue{id: id, log: log, consume: consume, wake: make(chan struct{}, 1), done: make(chan struct{})}
}

func (q *eventQueue) push(a core.Activity) {
	q.mu.Lock()
	switch {
	case len(q.events) < eventQueueSize:
		q.events = append(q.events, a)
		q.full = false
	case !q.full:
		q.full = true
		q.log.Warn("event queue full; dropping events", "plugin", q.id)
	}
	q.mu.Unlock()
	select {
	case q.wake <- struct{}{}:
	default:
	}
}

// take removes up to n events from the queue's head.
func (q *eventQueue) take(n int) []core.Activity {
	q.mu.Lock()
	defer q.mu.Unlock()
	n = min(n, len(q.events))
	batch := q.events[:n:n]
	q.events = q.events[n:]
	if len(q.events) == 0 {
		q.events = nil // let the drained array go
	}
	return batch
}

func (q *eventQueue) stop() { q.stopOnce.Do(func() { close(q.done) }) }

// run delivers events until ctx ends or the queue stops: it gathers them
// for eventBatchDelay after the first arrives, then sends them in batches.
func (q *eventQueue) run(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		select {
		case <-q.done:
			cancel()
		case <-ctx.Done():
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case <-q.wake:
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(eventBatchDelay):
		}
		for {
			batch := q.take(eventBatchSize)
			if len(batch) == 0 {
				break
			}
			q.deliver(ctx, batch)
			if ctx.Err() != nil {
				return
			}
		}
	}
}

// deliver sends a batch, retrying failures with growing pauses before
// dropping it.
func (q *eventQueue) deliver(ctx context.Context, batch []core.Activity) {
	pause := time.Second
	for attempt := 0; ; attempt++ {
		err := q.consume(ctx, batch)
		if err == nil {
			return
		}
		if attempt == eventRetries {
			q.log.WarnContext(ctx, "dropped events the plugin failed to consume", "plugin", q.id, "count", len(batch), "err", err)
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(pause):
		}
		pause *= 2
	}
}
