package library

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

func enqueue(t *testing.T, m *memStore, kind, key string) core.Job {
	t.Helper()
	j := core.Job{ID: core.NewID(), Kind: kind, UniqueKey: key, MaxAttempts: 2, RunAt: m.clock()}
	if _, err := m.Jobs().Enqueue(t.Context(), &j); err != nil {
		t.Fatal(err)
	}
	return j
}

func (m *memStore) job(id core.ID) core.Job {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, j := range m.jobs {
		if j.ID == id {
			return j
		}
	}
	return core.Job{}
}

func TestWorker(t *testing.T) {
	m := newMemStore()
	ran := 0
	w := &Worker{Queue: m.Jobs(), Owner: "w1", Handlers: map[string]Handler{
		"ok": func(context.Context, core.Job) ([]core.Job, error) {
			ran++
			// The follow-up shares the job's key, so it can only be
			// enqueued once the job completed.
			return []core.Job{{ID: core.NewID(), Kind: "later", UniqueKey: "ok", MaxAttempts: 1, RunAt: m.clock().Add(time.Hour)}}, nil
		},
		"bad":   func(context.Context, core.Job) ([]core.Job, error) { return nil, errors.New("boom") },
		"panic": func(context.Context, core.Job) ([]core.Job, error) { panic("oops") },
	}}
	ok := enqueue(t, m, "ok", "ok")
	bad := enqueue(t, m, "bad", "")
	pan := enqueue(t, m, "panic", "")
	for range 3 {
		if ran, err := w.RunOne(t.Context()); !ran || err != nil {
			t.Fatalf("RunOne = %v, %v", ran, err)
		}
	}
	if ran, _ := w.RunOne(t.Context()); ran {
		t.Error("ran a job that is not due")
	}
	if got := m.job(ok.ID); got.State != core.JobSucceeded || ran != 1 {
		t.Errorf("ok = %+v", got)
	}
	if len(m.jobs) != 4 || m.jobs[3].Kind != "later" {
		t.Errorf("follow-up = %+v", m.jobs)
	}
	if got := m.job(bad.ID); got.State != core.JobPending || got.LastError != "boom" {
		t.Errorf("bad = %+v", got)
	}
	if got := m.job(pan.ID); got.LastError == "" {
		t.Errorf("panic = %+v", got)
	}
}

func TestWorkerExtendsLease(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		m := newMemStore()
		m.clock = time.Now
		w := &Worker{Queue: m.Jobs(), Owner: "w1", LeaseTTL: 30 * time.Second, Handlers: map[string]Handler{
			"long": func(ctx context.Context, _ core.Job) ([]core.Job, error) {
				time.Sleep(2 * time.Minute)
				return nil, ctx.Err()
			},
		}}
		j := enqueue(t, m, "long", "")
		if _, err := w.RunOne(t.Context()); err != nil {
			t.Fatal(err)
		}
		// A job longer than its lease completes because the lease was
		// extended.
		if got := m.job(j.ID); got.State != core.JobSucceeded {
			t.Errorf("job = %+v", got)
		}
	})
}

func TestWorkerStopsOnLostLease(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		m := newMemStore()
		m.clock = time.Now
		var cause error
		w := &Worker{Queue: m.Jobs(), Owner: "w1", LeaseTTL: 30 * time.Second, Handlers: map[string]Handler{
			"long": func(ctx context.Context, _ core.Job) ([]core.Job, error) {
				<-ctx.Done()
				cause = context.Cause(ctx)
				return nil, cause
			},
		}}
		j := enqueue(t, m, "long", "")
		go func() {
			// Another worker takes the job over.
			time.Sleep(5 * time.Second)
			m.mu.Lock()
			for i := range m.jobs {
				if m.jobs[i].ID == j.ID {
					m.jobs[i].LeaseOwner = "w2"
				}
			}
			m.mu.Unlock()
		}()
		if _, err := w.RunOne(t.Context()); !errors.Is(err, core.ErrConflict) {
			t.Errorf("RunOne = %v, want the conflict of failing a lost job", err)
		}
		if !errors.Is(cause, core.ErrConflict) {
			t.Errorf("cause = %v", cause)
		}
	})
}
