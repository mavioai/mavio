package indexer

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mavioai/mavio/plugins/indexers/internal/ported"
)

func TestStatusesCases(t *testing.T) {
	ported.Run(t, "indexer_status_service.json", ported.Cases{Facts: map[string]string{
		"should_cancel_backoff_on_success":               "TestStatusesSuccess",
		"should_not_store_update_if_already_okay":        "TestStatusesSuccess",
		"should_not_record_failure_for_unknown_provider": "TestStatusesUnknown",
	}})
	ported.Run(t, "indexer_service.json", ported.Cases{Skip: map[string]string{
		"should_remove_missing_indexers_on_startup": "indexers come from the plugin's configuration; Statuses.Remove forgets the statuses of removed ones",
	}})
}

// clock is a settable clock for statuses.
type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func newStatuses(t *testing.T, c *clock) (*Statuses, string) {
	t.Helper()
	file := filepath.Join(t.TempDir(), "status.json")
	s, err := NewStatuses(file, c.now)
	if err != nil {
		t.Fatalf("NewStatuses() error = %v", err)
	}
	s.SetStarted(c.t.Add(-time.Hour))
	return s, file
}

func saved(file string) bool {
	_, err := os.Stat(file)
	return err == nil
}

func TestStatusesSuccess(t *testing.T) {
	for _, tt := range []struct {
		level     int
		wantSaved bool
	}{{2, true}, {0, false}} {
		c := &clock{time.Date(2026, 10, 11, 9, 30, 0, 0, time.UTC)}
		s, file := newStatuses(t, c)
		s.m["1"] = &Status{EscalationLevel: tt.level, DisabledUntil: c.t.Add(time.Hour)}
		s.RecordSuccess("1")
		if got := saved(file); got != tt.wantSaved {
			t.Errorf("level %d: saved got = %v, want = %v", tt.level, got, tt.wantSaved)
		}
		if tt.level > 0 {
			if _, _, ok := s.Disabled("1"); ok {
				t.Errorf("level %d: Disabled() got = true, want = false", tt.level)
			}
		}
	}
}

func TestStatusesUnknown(t *testing.T) {
	c := &clock{time.Date(2026, 10, 11, 9, 30, 0, 0, time.UTC)}
	s, file := newStatuses(t, c)
	s.RecordFailure("", 0, "failed")
	if saved(file) || len(s.m) != 0 {
		t.Errorf("RecordFailure(\"\") got = %d statuses, want = none saved", len(s.m))
	}
}

func TestStatusesBackoff(t *testing.T) {
	start := time.Date(2026, 10, 11, 9, 0, 0, 0, time.UTC)
	for _, tt := range []struct {
		name       string
		sinceStart time.Duration
		failures   int
		minBackoff time.Duration
		want       time.Duration
	}{
		{"first", time.Hour, 1, 0, time.Minute},
		{"second", time.Hour, 2, 0, 5 * time.Minute},
		{"fourth", time.Hour, 4, 0, 30 * time.Minute},
		{"many", time.Hour, 20, 0, 24 * time.Hour},
		{"grace", time.Minute, 4, 0, time.Minute},
		{"minimum", time.Hour, 1, time.Hour, time.Hour},
		{"minimum in grace", time.Minute, 1, 2 * time.Hour, 3 * time.Hour},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c := &clock{start}
			s, _ := newStatuses(t, c)
			c.t = start.Add(tt.sinceStart)
			s.SetStarted(start)
			for range tt.failures {
				s.RecordFailure("1", tt.minBackoff, "failed")
			}
			until, failure, ok := s.Disabled("1")
			if !ok || failure != "failed" {
				t.Fatalf("Disabled() got = %v %q, want = true \"failed\"", ok, failure)
			}
			if got := until.Sub(c.t); got != tt.want {
				t.Errorf("backoff got = %v, want = %v", got, tt.want)
			}
		})
	}
}

func TestStatusesPersist(t *testing.T) {
	c := &clock{time.Date(2026, 10, 11, 9, 30, 0, 0, time.UTC)}
	s, file := newStatuses(t, c)
	s.RecordQuery("1")
	c.t = c.t.Add(2 * time.Hour)
	s.RecordQuery("1")
	s.RecordGrab("1")
	s.SetCookies("1", map[string]string{"uid": "7"}, c.t.Add(time.Hour))

	r, err := NewStatuses(file, c.now)
	if err != nil {
		t.Fatalf("NewStatuses() error = %v", err)
	}
	for _, tt := range []struct {
		name   string
		count  func(string, time.Duration) (int, time.Time)
		window time.Duration
		want   int
	}{
		{"queries 1h", r.Queries, time.Hour, 1},
		{"queries 24h", r.Queries, 24 * time.Hour, 2},
		{"grabs 24h", r.Grabs, 24 * time.Hour, 1},
	} {
		if got, _ := tt.count("1", tt.window); got != tt.want {
			t.Errorf("%s got = %d, want = %d", tt.name, got, tt.want)
		}
	}
	if got := r.Cookies("1")["uid"]; got != "7" {
		t.Errorf("Cookies() got = %q, want = \"7\"", got)
	}
	c.t = c.t.Add(2 * time.Hour)
	if got := r.Cookies("1"); got != nil {
		t.Errorf("expired Cookies() got = %v, want = nil", got)
	}
	r.Remove(func(string) bool { return false })
	if got, _ := r.Queries("1", 24*time.Hour); got != 0 {
		t.Errorf("Queries() after Remove got = %d, want = 0", got)
	}
}
