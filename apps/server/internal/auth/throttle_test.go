package auth

import (
	"testing"
	"time"
)

func TestThrottle(t *testing.T) {
	t0 := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	now := t0
	th := NewThrottle()
	th.now = func() time.Time { return now }

	for range FreeFailures {
		th.Fail("user:a", "addr:1")
	}
	if w := th.Wait("user:a"); w != 0 {
		t.Errorf("after the free failures: Wait = %v, want = 0", w)
	}
	// Each failure after them doubles the lockout.
	for i, want := range []time.Duration{time.Second, 2 * time.Second, 4 * time.Second} {
		th.Fail("user:a", "addr:1")
		if w := th.Wait("user:a"); w != want {
			t.Errorf("failure %d: Wait = %v, want = %v", FreeFailures+i+1, w, want)
		}
	}
	// Any locked out key holds the attempt back; others are free.
	if w := th.Wait("user:b", "addr:1"); w != 4*time.Second {
		t.Errorf("same address: Wait = %v, want = 4s", w)
	}
	if w := th.Wait("user:b", "addr:2"); w != 0 {
		t.Errorf("another user and address: Wait = %v, want = 0", w)
	}
	for range 30 {
		th.Fail("user:a")
	}
	if w := th.Wait("user:a"); w != MaxLockout {
		t.Errorf("after many failures: Wait = %v, want = %v", w, MaxLockout)
	}

	// Success forgets the user, not the address.
	th.Succeed("user:a")
	if w := th.Wait("user:a"); w != 0 {
		t.Errorf("after success: Wait = %v, want = 0", w)
	}
	// Keys are forgotten an hour after their last failure.
	now = t0.Add(FailureMemory)
	th.Fail("addr:2")
	if w := th.Wait("addr:1"); w != 0 {
		t.Errorf("an hour later: Wait = %v, want = 0", w)
	}
	if _, ok := th.keys["addr:1"]; ok {
		t.Error("a forgotten key is kept")
	}
}
