package activity

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store"
)

type notifier struct {
	mu  sync.Mutex
	got []string
}

func (n *notifier) Name() string { return "test" }

func (n *notifier) Notify(_ context.Context, a core.Activity) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.got = append(n.got, a.Type)
	return nil
}

func TestRecord(t *testing.T) {
	ctx := t.Context()
	db, err := store.Open(ctx, "sqlite:"+filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	n := &notifier{}
	l := New(Config{Store: db, Notifiers: func() []Notifier { return []Notifier{n} }})
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { _ = l.Run(runCtx); close(done) }()
	l.Record(ctx, core.Activity{Type: "user.login", Title: "admin signed in"})
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(10 * time.Millisecond) {
		n.mu.Lock()
		got := len(n.got)
		n.mu.Unlock()
		if got == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the notifier was not told")
		}
	}
	cancel()
	<-done
	page, err := db.Activities().List(ctx, core.ActivityQuery{})
	if err != nil || page.Total != 1 || page.Items[0].Severity != core.SeverityInfo {
		t.Errorf("stored = %+v, %v", page, err)
	}
	var nilLog *Log
	nilLog.Record(ctx, core.Activity{}) // a nil log records nothing
}

func TestPublish(t *testing.T) {
	ctx := t.Context()
	db, err := store.Open(ctx, "sqlite:"+filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	var got []core.Activity
	l := New(Config{
		Store: db, Events: func(a core.Activity) { got = append(got, a) },
		Wants: func(eventType string) bool { return eventType == "item.added" },
	})
	l.Record(ctx, core.Activity{Type: "user.login", Title: "admin signed in"})
	l.Publish(core.Activity{Type: "item.added", Title: "Movie"})
	if len(got) != 2 || got[0].Type != "user.login" || got[1].Type != "item.added" {
		t.Fatalf("events = %+v, want user.login and item.added", got)
	}
	if e := got[1]; e.ID.IsZero() || e.Time.IsZero() || e.Severity != core.SeverityInfo {
		t.Errorf("published event = %+v, want ID, time and severity", e)
	}
	// Published events stay out of the log.
	if page, err := db.Activities().List(ctx, core.ActivityQuery{}); err != nil || page.Total != 1 {
		t.Errorf("stored = %+v, %v, want only the logged activity", page, err)
	}
	if !l.Wants("item.added") || l.Wants("item.removed") {
		t.Error("Wants() does not follow the configuration")
	}
	var nilLog *Log
	nilLog.Publish(core.Activity{Type: "item.added"})
	if nilLog.Wants("item.added") {
		t.Error("a nil log wants events")
	}
}
