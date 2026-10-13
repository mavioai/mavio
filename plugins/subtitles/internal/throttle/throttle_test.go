package throttle

import (
	"log/slog"
	"path/filepath"
	"testing"
	"time"
)

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func newThrottle(t *testing.T, file string, c *clock) *Throttle {
	t.Helper()
	th, err := New(file, c.now, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	return th
}

// strikesUntilThrottled is Bazarr's _strikes_until_throttled.
func strikesUntilThrottled(th *Throttle) int {
	for strike := 1; strike < 10; strike++ {
		if th.Fail("provider", "TooManyRequests", time.Hour, true) {
			return strike
		}
	}
	return 0
}

// Bazarr's test_throttled_count_grants_new_retries_after_throttling: a
// provider throttled once is not throttled on sight afterwards.
func TestCountStartsAgainAfterThrottling(t *testing.T) {
	th := newThrottle(t, "", &clock{time.Unix(0, 0)})
	for range 2 {
		if got := strikesUntilThrottled(th); got != 5 {
			t.Errorf("strikes: got = %d, want = 5", got)
		}
	}
}

// Bazarr's test_throttled_count_ignores_failures_older_than_the_window.
func TestCountIgnoresOldFailures(t *testing.T) {
	c := &clock{time.Unix(0, 0)}
	th := newThrottle(t, "", c)
	for range 4 {
		th.Fail("provider", "TooManyRequests", time.Hour, true)
	}
	c.t = c.t.Add(countWindow + time.Second)
	if th.Fail("provider", "TooManyRequests", time.Hour, true) {
		t.Error("Fail: got = throttled, want = counted")
	}
	if got := th.counts["provider"].n; got != 1 {
		t.Errorf("count: got = %d, want = 1", got)
	}
	if _, ok := th.Throttled("provider"); ok {
		t.Error("Throttled: got = true, want = false")
	}
}

// Bazarr's test_reset_throttled_providers_also_resets_the_count; its
// test_throttled_count_tolerates_a_concurrent_removal needs no port, as
// counts are kept under a lock.
func TestResetForgetsCounts(t *testing.T) {
	th := newThrottle(t, "", &clock{time.Unix(0, 0)})
	th.Fail("provider", "AuthenticationError", time.Hour, false)
	for range 3 {
		th.Fail("provider", "TooManyRequests", time.Hour, true)
	}
	th.Reset(nil)
	if _, ok := th.counts["provider"]; ok {
		t.Error("count after reset: got = kept, want = forgotten")
	}
	if _, ok := th.Throttled("provider"); ok {
		t.Error("Throttled after reset: got = true, want = false")
	}
}

func TestThrottle(t *testing.T) {
	file := filepath.Join(t.TempDir(), "throttled.json")
	c := &clock{time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)}
	th := newThrottle(t, file, c)
	if !th.Fail("a", "AuthenticationError", 12*time.Hour, false) {
		t.Fatal("Fail: got = counted, want = throttled")
	}
	th.Fail("b", "DownloadLimitExceeded", 3*time.Hour, false)

	// The throttles survive a restart.
	th = newThrottle(t, file, c)
	st, ok := th.Throttled("a")
	if !ok || st.Reason != "AuthenticationError" || !st.Until.Equal(c.t.Add(12*time.Hour)) {
		t.Errorf("Throttled(a): got = %+v %v, want = AuthenticationError until %v", st, ok, c.t.Add(12*time.Hour))
	}

	// Only authentication and configuration failures are reset when the
	// settings change.
	th.Reset(func(_, reason string) bool { return reason == "AuthenticationError" })
	if _, ok := th.Throttled("a"); ok {
		t.Error("Throttled(a) after reset: got = true, want = false")
	}
	if _, ok := th.Throttled("b"); !ok {
		t.Error("Throttled(b) after reset: got = false, want = true")
	}

	c.t = c.t.Add(3 * time.Hour)
	if _, ok := th.Throttled("b"); ok {
		t.Error("Throttled(b) after 3 hours: got = true, want = false")
	}

	th.Fail("c", "TooManyRequests", time.Hour, false)
	th.Retain(func(name string) bool { return name != "c" })
	if _, ok := th.Throttled("c"); ok {
		t.Error("Throttled(c) after Retain: got = true, want = false")
	}
}
