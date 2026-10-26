package rclone

import (
	"context"
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

// An Open given the info of the file's earlier opening does not stat it
// again: on 115 a stat lists the file's folder.
func TestOpenWithFileInfo(t *testing.T) {
	var stats atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/operations/stat" {
			stats.Add(1)
		}
		_, _ = io.WriteString(w, `{"item":{"Name":"a.mkv","Size":10}}`)
	}))
	defer srv.Close()
	c, err := NewClient(Endpoint{URL: srv.URL}, nil)
	if err != nil {
		t.Fatal(err)
	}
	s := &Storage{r: &Remotes{Client: c}, remote: "drive"}
	ctx := t.Context()
	f, err := s.Open(ctx, "dir/a.mkv")
	if err != nil {
		t.Fatal(err)
	}
	info, _ := f.Stat()
	_ = f.Close()
	for _, tc := range []struct {
		name      string
		ctx       context.Context
		wantStats int32
	}{
		{"same file", core.WithFileInfo(ctx, "dir/a.mkv", info), 0},
		{"other file", core.WithFileInfo(ctx, "dir/b.mkv", info), 1},
		{"info of another storage", core.WithFileInfo(ctx, "dir/a.mkv", fakeInfo{}), 1},
	} {
		stats.Store(0)
		f, err := s.Open(tc.ctx, "dir/a.mkv")
		if err != nil {
			t.Fatalf("%s: Open() error = %v", tc.name, err)
		}
		if got, _ := f.Stat(); got.Size() != 10 {
			t.Errorf("%s: size = %d, want = 10", tc.name, got.Size())
		}
		_ = f.Close()
		if got := stats.Load(); got != tc.wantStats {
			t.Errorf("%s: stats = %d, want = %d", tc.name, got, tc.wantStats)
		}
	}
}

type fakeInfo struct{ fs.FileInfo }

func TestThrottled(t *testing.T) {
	for _, tt := range []struct {
		status int
		msg    string
		want   bool
	}{
		{http.StatusTooManyRequests, "", true},
		{500, "115's firewall blocked the request as too frequent: HTTP error 405", true},
		{500, "HTTP error 429 (429 Too Many Requests) returned body: {}", true},
		{500, "googleapi: Error 403: User rate limit exceeded., userRateLimitExceeded", true},
		{500, "too_many_requests/..", true},
		{500, "baidu netdisk error 31034: hit api limit (see https://pan.baidu.com/union/doc/)", true},
		{500, "baidu netdisk error -9: file does not exist (see https://pan.baidu.com/union/doc/)", false},
		{500, "directory not found", false},
	} {
		if got := throttled(&Error{Status: tt.status, Message: tt.msg}); got != tt.want {
			t.Errorf("throttled(%d, %q) = %v, want = %v", tt.status, tt.msg, got, tt.want)
		}
	}
}
