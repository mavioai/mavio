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
