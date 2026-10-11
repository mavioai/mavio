package acquire

import (
	"testing"
	"time"
)

func TestStatusAt(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	days := func(n int) time.Time { return now.AddDate(0, 0, n) }
	for _, tc := range []struct {
		name string
		a    Availability
		want MovieStatus
	}{
		{"no dates", Availability{}, StatusAnnounced},
		{"in cinemas later", Availability{InCinemas: days(10)}, StatusAnnounced},
		{"in cinemas", Availability{InCinemas: days(-10)}, StatusInCinemas},
		{"in cinemas 91 days ago", Availability{InCinemas: days(-91)}, StatusReleased},
		{"in cinemas 91 days ago, physical later", Availability{InCinemas: days(-91), Physical: days(5)}, StatusInCinemas},
		{"digital today", Availability{InCinemas: days(-20), Digital: now}, StatusReleased},
		{"physical before cinemas", Availability{Physical: days(-1)}, StatusReleased},
	} {
		if got := tc.a.StatusAt(now); got != tc.want {
			t.Errorf("%s: StatusAt = %v, want = %v", tc.name, got, tc.want)
		}
	}
}

func TestShouldRefreshMovie(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	days := func(n int) time.Time { return now.AddDate(0, 0, n) }
	released := Availability{InCinemas: days(-400), Physical: days(-300)}
	for _, tc := range []struct {
		name string
		last time.Time
		a    Availability
		want bool
	}{
		{"never refreshed", time.Time{}, released, true},
		{"refreshed 181 days ago", days(-181), released, true},
		{"refreshed 6 hours ago, announced", now.Add(-6 * time.Hour), Availability{}, false},
		{"refreshed a day ago, announced", days(-1), Availability{InCinemas: days(30)}, true},
		{"refreshed a day ago, in cinemas", days(-1), Availability{InCinemas: days(-10)}, true},
		{"refreshed a day ago, released long ago", days(-1), released, false},
		{"refreshed a day ago, physical release 20 days ago", days(-1), Availability{InCinemas: days(-100), Physical: days(-20)}, true},
		{"refreshed a day ago, digital release only", days(-1), Availability{Digital: days(-100)}, true},
		{"refreshed a day ago, released 130 days after cinemas", days(-1), Availability{InCinemas: days(-130)}, false},
	} {
		if got := ShouldRefreshMovie(tc.last, &tc.a, now); got != tc.want {
			t.Errorf("%s: ShouldRefreshMovie = %v, want = %v", tc.name, got, tc.want)
		}
	}
}
