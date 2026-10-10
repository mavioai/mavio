package events

import (
	"context"
	"errors"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
	"github.com/mavioai/mavio/libs/store"
)

// publisher keeps the events it is given as "type title key=value…".
type publisher struct {
	mu     sync.Mutex
	events []string
	logged []string
}

func describe(a core.Activity) string {
	parts := []string{a.Type, a.Title}
	for _, k := range slices.Sorted(maps.Keys(a.Attributes)) {
		parts = append(parts, k+"="+a.Attributes[k])
	}
	return strings.Join(parts, " ")
}

func (p *publisher) Record(_ context.Context, a core.Activity) {
	p.mu.Lock()
	p.logged = append(p.logged, a.Type)
	p.mu.Unlock()
	p.Publish(a)
}

func (p *publisher) Publish(a core.Activity) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, describe(a))
}

func (p *publisher) Wants(string) bool { return true }

// take returns and forgets the events so far.
func (p *publisher) take() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := p.events
	p.events = nil
	return out
}

func TestPublish(t *testing.T) {
	ctx := t.Context()
	s, err := store.Open(ctx, "sqlite:"+filepath.Join(t.TempDir(), "mavio.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	pub := &publisher{}
	db := Observe(s, New(Config{Store: s, Events: pub}))
	lib := core.Library{Name: "Films", Kind: core.LibraryMovies, Paths: []string{t.TempDir()}}
	if err := db.Libraries().Create(ctx, &lib); err != nil {
		t.Fatal(err)
	}
	l := lib.ID.String()
	film := core.Item{ID: core.NewID(), LibraryID: lib.ID, Kind: core.KindMovie, Name: "Film", Path: filepath.Join(lib.Paths[0], "film.mkv")}
	check := func(step string, want ...string) {
		t.Helper()
		if got := pub.take(); !slices.Equal(got, want) {
			t.Errorf("%s: events = %q, want %q", step, got, want)
		}
	}

	// Events of a transaction wait for its commit, and a rollback drops
	// them.
	err = db.InTx(ctx, func(tx core.Store) error {
		if err := tx.Items().Upsert(ctx, film); err != nil {
			return err
		}
		check("inside the transaction")
		return errors.New("rolled back")
	})
	if err == nil {
		t.Fatal("rolled back transaction succeeded")
	}
	check("after a rollback")
	if err := db.InTx(ctx, func(tx core.Store) error { return tx.Items().Upsert(ctx, film) }); err != nil {
		t.Fatal(err)
	}
	check("added", "item.added Film library="+l)
	if err := db.Items().Upsert(ctx, film); err != nil {
		t.Fatal(err)
	}
	check("updated", "item.updated Film library="+l)

	user := core.User{Name: "kid", PasswordHash: "x"}
	if err := s.Users().Create(ctx, &user); err != nil {
		t.Fatal(err)
	}
	if err := db.UserData().Put(ctx, &core.UserData{UserID: user.ID, ItemID: film.ID, Favorite: true, Position: 90 * time.Second}); err != nil {
		t.Fatal(err)
	}
	check("user data", "userdata.changed User data changed favorite=true played=false position=90000")

	if err := db.Items().Delete(ctx, film.ID); err != nil {
		t.Fatal(err)
	}
	check("removed", "item.removed Film library="+l)

	// Task runs end in task events, scans in scan events too; failures
	// are logged.
	run := func(job core.Job, cause error) {
		t.Helper()
		if _, err := db.Jobs().Enqueue(ctx, &job); err != nil {
			t.Fatal(err)
		}
		leased, err := db.Jobs().Lease(ctx, "test", []string{job.Kind}, time.Minute)
		if err != nil {
			t.Fatal(err)
		}
		if cause == nil {
			err = db.Jobs().Complete(ctx, leased.ID, "test")
		} else {
			err = db.Jobs().Fail(ctx, leased.ID, "test", cause)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	run(library.ScanJob(lib, time.Now(), false), nil)
	check("scan", "library.scanned Scan Films library="+l+" succeeded=true",
		"task.completed Scan Films completed library="+l+" task=library.scan:"+l)
	run(library.CleanupJob(time.Now(), false), errors.New("disk full"))
	check("failed cleanup", "task.failed Clean up jobs failed task=jobs.cleanup")
	if !slices.Equal(pub.logged, []string{"task.failed"}) {
		t.Errorf("logged = %q, want only the failure", pub.logged)
	}
}
