package rclone

import (
	"errors"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

func TestHolds(t *testing.T) {
	// answer is what the fake rcd answers every remote call with: its
	// status and error message, status 200 for success.
	var answer atomic.Value
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		a := answer.Load().([2]string)
		if a[0] == "200" {
			switch r.URL.Path {
			case "/operations/fsinfo":
				_, _ = io.WriteString(w, `{"Features":{}}`)
			default:
				_, _ = io.WriteString(w, `{"item":{"Name":"x","IsDir":true},"list":[]}`)
			}
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, `{"error":"`+a[1]+`","status":`+a[0]+`}`)
	}))
	defer srv.Close()
	c, err := NewClient(Endpoint{URL: srv.URL}, nil)
	if err != nil {
		t.Fatal(err)
	}
	clock := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	c.holds.now = func() time.Time { return clock }
	r := &Remotes{Client: c, now: func() time.Time { return clock }}
	ctx := t.Context()

	set := func(status, msg string) { answer.Store([2]string{status, msg}) }
	stat := func() error {
		s := &Storage{r: r, remote: "drive"}
		_, err := s.Stat(ctx, "x")
		return err
	}
	expect := func(name string, err error, wantCalls int32, wantWait time.Duration) {
		t.Helper()
		if got := calls.Swap(0); got != wantCalls {
			t.Errorf("%s: calls of the rcd = %d, want = %d", name, got, wantCalls)
		}
		if got := core.RetryAfter(err); got != wantWait {
			t.Errorf("%s: RetryAfter(%v) = %v, want = %v", name, err, got, wantWait)
		}
	}

	// A throttled call holds the remote for a minute: calls fail without
	// reaching the rcd, telling how long they wait.
	set("500", "115's firewall blocked the request as too frequent: HTTP error 405")
	expect("throttled", stat(), 1, time.Minute)
	clock = clock.Add(20 * time.Second)
	expect("held", stat(), 0, 40*time.Second)
	// Another remote is not held.
	if err := c.callRemote(ctx, "other", "operations/stat", map[string]any{}, nil, false); err == nil {
		t.Error("other remote: error = nil, want the rcd's")
	}
	calls.Store(0)
	// The next hold in a row lasts twice as long.
	clock = clock.Add(time.Minute)
	expect("throttled again", stat(), 1, 2*time.Minute)

	// A call that fails otherwise does not hold, and a success releases.
	clock = clock.Add(3 * time.Minute)
	set("500", "directory not found in listing")
	expect("other failure", stat(), 1, 0)
	expect("other failure again", stat(), 1, 0)
	set("200", "")
	if err := stat(); err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	calls.Store(0)
	set("500", "rate limit exceeded")
	expect("throttled after success", stat(), 1, time.Minute)

	// A remote whose backend fails to start, as on a failed login, is held
	// too: features are not asked of it again until the hold ends or its
	// configuration changes.
	clock = clock.Add(2 * time.Minute)
	set("500", "cloud115: cookie check failed: not logged in")
	if _, err := r.Open(ctx, "drive", "dir"); err == nil {
		t.Fatal("Open() error = nil")
	}
	calls.Store(0)
	_, err = r.Open(ctx, "drive", "dir")
	expect("held login", err, 0, 2*time.Minute)
	var e *Error
	if !errors.As(err, &e) || errors.Is(err, fs.ErrNotExist) {
		t.Errorf("held Open() error = %v, want an *Error not taken for a missing file", err)
	}
	set("200", "")
	r.Forget("drive")
	if _, err := r.Open(ctx, "drive", "dir"); err != nil {
		t.Errorf("Open() after Forget error = %v", err)
	}
	// Objects not found do not hold a remote whose features are asked.
	set("404", "object not found")
	r.Forget("drive")
	_, _ = r.Open(ctx, "drive", "dir")
	calls.Store(0)
	_, _ = r.Open(ctx, "drive", "dir")
	expect("not found", nil, 1, 0)
}
