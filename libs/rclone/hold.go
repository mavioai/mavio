package rclone

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Holds on a remote: after its backend failed to start, as when its login
// failed, or limited the rate of its calls, the remote's calls fail at once
// for a while instead of reaching the storage, which would log in again or
// be limited for longer. Each hold in a row lasts twice the one before.
const (
	holdMin = time.Minute
	holdMax = 30 * time.Minute
)

// holds are the remotes whose calls wait.
type holds struct {
	mu sync.Mutex
	m  map[string]*hold
	// now is the clock; time.Now when nil.
	now func() time.Time
}

type hold struct {
	until time.Time
	// next is how long the next hold in a row lasts.
	next  time.Duration
	cause *Error
}

func (h *holds) clock() time.Time {
	if h.now != nil {
		return h.now()
	}
	return time.Now()
}

// check returns the error of a call of method on a held remote, nil when
// the remote is not held.
func (h *holds) check(remote, method string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	hd := h.m[remote]
	if hd == nil {
		return nil
	}
	wait := hd.until.Sub(h.clock())
	if wait <= 0 {
		return nil
	}
	return &Error{
		Method:     method,
		Status:     hd.cause.Status,
		Message:    fmt.Sprintf("remote %s waits %s before it is called again, after: %s", remote, wait.Round(time.Second), hd.cause.Message),
		RetryAfter: wait,
	}
}

// start holds remote after a call failed with e, for at least what e asks,
// and sets e's RetryAfter to the hold.
func (h *holds) start(remote string, e *Error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.m == nil {
		h.m = map[string]*hold{}
	}
	hd := h.m[remote]
	if hd == nil {
		hd = &hold{next: holdMin}
		h.m[remote] = hd
	}
	d := max(hd.next, e.RetryAfter)
	hd.until, hd.next, hd.cause = h.clock().Add(d), min(d*2, holdMax), e
	e.RetryAfter = d
}

// release ends the hold of remote and its run, after a call succeeded or
// its configuration changed.
func (h *holds) release(remote string) {
	h.mu.Lock()
	delete(h.m, remote)
	h.mu.Unlock()
}

// throttled reports whether err tells that the storage limits the rate of
// calls or the data read: backends report most limits only in their
// messages, which the rcd answers with status 500.
func throttled(err error) bool {
	var e *Error
	if !errors.As(err, &e) {
		return false
	}
	if e.Status == http.StatusTooManyRequests {
		return true
	}
	m := strings.ToLower(e.Message)
	for _, s := range []string{"too many requests", "too frequent", "rate limit", "ratelimit", "quota exceeded", "quotaexceeded"} {
		if strings.Contains(m, s) {
			return true
		}
	}
	return false
}

// callRemote calls an rc method on remote unless the remote is held, and
// holds it when the call is throttled; started tells that the call starts
// the remote's backend, as operations/fsinfo does, whose failures hold the
// remote too. A call that succeeds releases the remote.
func (c *Client) callRemote(ctx context.Context, remote, method string, in, out any, started bool) error {
	if err := c.holds.check(remote, method); err != nil {
		return err
	}
	err := c.Call(ctx, method, in, out)
	var e *Error
	switch {
	case err == nil:
		c.holds.release(remote)
	case errors.As(err, &e) && (throttled(e) || started && e.Status != http.StatusNotFound):
		c.holds.start(remote, e)
	}
	return err
}

// RetryIn returns how long to wait before calling again, for core.RetryAfter.
func (e *Error) RetryIn() time.Duration { return e.RetryAfter }
