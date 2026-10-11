package indexer

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"
)

// backoffPeriods are how long an indexer is left out after failing, by
// how many times it failed in a row.
var backoffPeriods = []time.Duration{
	0,
	time.Minute,
	5 * time.Minute,
	15 * time.Minute,
	30 * time.Minute,
	time.Hour,
	3 * time.Hour,
	6 * time.Hour,
	12 * time.Hour,
	24 * time.Hour,
}

// startupGrace is how long after starting failures leave an indexer out
// for at most backoffPeriods[2].
const startupGrace = 15 * time.Minute

// Status is what is kept of an indexer between searches.
type Status struct {
	InitialFailure    time.Time `json:"initial_failure,omitzero"`
	MostRecentFailure time.Time `json:"most_recent_failure,omitzero"`
	EscalationLevel   int       `json:"escalation_level,omitempty"`
	DisabledUntil     time.Time `json:"disabled_until,omitzero"`
	// The failure that disabled the indexer.
	Failure       string            `json:"failure,omitempty"`
	Cookies       map[string]string `json:"cookies,omitempty"`
	CookiesExpire time.Time         `json:"cookies_expire,omitzero"`
	QueryTimes    []time.Time       `json:"queries,omitempty"`
	GrabTimes     []time.Time       `json:"grabs,omitempty"`
	// Values are what the indexer's site keeps across restarts, such as
	// API tokens and capabilities, by key.
	Values map[string]string `json:"values,omitempty"`
}

// Statuses keeps the statuses of indexers by ID: their backoff after
// failures, as Prowlarr's IndexerStatusService escalates it, their cookies
// and their recent queries and grabs, saved to a file when one is given.
type Statuses struct {
	mu      sync.Mutex
	file    string
	now     func() time.Time
	started time.Time
	m       map[string]*Status
}

// NewStatuses returns the statuses saved in file, or none when the file
// is "" or missing. now is the clock.
func NewStatuses(file string, now func() time.Time) (*Statuses, error) {
	s := &Statuses{file: file, now: now, started: now(), m: map[string]*Status{}}
	if file == "" {
		return s, nil
	}
	data, err := os.ReadFile(file)
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &s.m); err != nil {
		return nil, fmt.Errorf("read %s: %w", file, err)
	}
	return s, nil
}

// SetStarted sets when the plugin started, for the grace period after.
func (s *Statuses) SetStarted(t time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.started = t
}

// Get returns a copy of an indexer's status.
func (s *Statuses) Get(id string) Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.m[id]
	if st == nil {
		return Status{}
	}
	c := *st
	c.Cookies = maps.Clone(st.Cookies)
	c.Values = maps.Clone(st.Values)
	return c
}

// Value returns a value the indexer's site keeps; "" when there is none.
func (s *Statuses) Value(id, key string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if st := s.m[id]; st != nil {
		return st.Values[key]
	}
	return ""
}

// SetValue keeps a value of the indexer's site; "" forgets it.
func (s *Statuses) SetValue(id, key, value string) {
	if id == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.status(id)
	if st.Values[key] == value {
		return
	}
	if value == "" {
		delete(st.Values, key)
	} else {
		if st.Values == nil {
			st.Values = map[string]string{}
		}
		st.Values[key] = value
	}
	s.save()
}

// Disabled returns until when the indexer is left out, if it is.
func (s *Statuses) Disabled(id string) (until time.Time, failure string, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.m[id]
	if st == nil || !st.DisabledUntil.After(s.now()) {
		return time.Time{}, "", false
	}
	return st.DisabledUntil, st.Failure, true
}

// RecordSuccess lowers the indexer's escalation and enables it again.
func (s *Statuses) RecordSuccess(id string) {
	if id == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.m[id]
	if st == nil || st.EscalationLevel == 0 {
		return
	}
	st.EscalationLevel--
	st.DisabledUntil = time.Time{}
	st.Failure = ""
	s.save()
}

// RecordFailure leaves the indexer out for longer each time it fails in a
// row, at least for minBackoff.
func (s *Statuses) RecordFailure(id string, minBackoff time.Duration, failure string) {
	s.recordFailure(id, minBackoff, failure, true)
}

// RecordConnectionFailure leaves the indexer out without escalating, for
// failures to reach it.
func (s *Statuses) RecordConnectionFailure(id string, failure string) {
	s.recordFailure(id, 0, failure, false)
}

func (s *Statuses) recordFailure(id string, minBackoff time.Duration, failure string, escalate bool) {
	if id == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.status(id)
	now := s.now()
	st.MostRecentFailure = now
	st.Failure = failure
	maxLevel := len(backoffPeriods) - 1
	if st.EscalationLevel == 0 {
		st.InitialFailure = now
		st.EscalationLevel = 1
		escalate = false
	}
	inStartupGrace := s.started.Add(startupGrace).After(now)
	if escalate && !inStartupGrace {
		st.EscalationLevel = min(maxLevel, st.EscalationLevel+1)
	}
	if minBackoff > 0 {
		for st.EscalationLevel < maxLevel && backoffPeriods[st.EscalationLevel] < minBackoff {
			st.EscalationLevel++
		}
	}
	st.DisabledUntil = now.Add(backoffPeriods[min(maxLevel, st.EscalationLevel)])
	if inStartupGrace && minBackoff == 0 {
		if limit := now.Add(backoffPeriods[2]); limit.Before(st.DisabledUntil) {
			st.DisabledUntil = limit
		}
	}
	s.save()
}

// Cookies returns the indexer's cookies, nil when they expired.
func (s *Statuses) Cookies(id string) map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.m[id]
	if st == nil || (!st.CookiesExpire.IsZero() && st.CookiesExpire.Before(s.now())) {
		return nil
	}
	return maps.Clone(st.Cookies)
}

// SetCookies replaces the indexer's cookies, until expire when set.
func (s *Statuses) SetCookies(id string, cookies map[string]string, expire time.Time) {
	if id == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.status(id)
	if maps.Equal(st.Cookies, cookies) && st.CookiesExpire.Equal(expire) {
		return
	}
	st.Cookies = maps.Clone(cookies)
	st.CookiesExpire = expire
	s.save()
}

// limitWindow is the longest window limits count queries and grabs in.
const limitWindow = 24 * time.Hour

// RecordQuery counts a query of the indexer against its limit.
func (s *Statuses) RecordQuery(id string) {
	s.record(id, func(st *Status) *[]time.Time { return &st.QueryTimes })
}

// RecordGrab counts a grab from the indexer against its limit.
func (s *Statuses) RecordGrab(id string) {
	s.record(id, func(st *Status) *[]time.Time { return &st.GrabTimes })
}

func (s *Statuses) record(id string, times func(*Status) *[]time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	t := times(s.status(id))
	*t = append(slices.DeleteFunc(*t, func(x time.Time) bool { return now.Sub(x) > limitWindow }), now)
	s.save()
}

// Queries counts the indexer's queries in the window, and returns when
// the first of them leaves it.
func (s *Statuses) Queries(id string, window time.Duration) (int, time.Time) {
	return s.count(id, window, func(st *Status) []time.Time { return st.QueryTimes })
}

// Grabs counts the indexer's grabs like Queries.
func (s *Statuses) Grabs(id string, window time.Duration) (int, time.Time) {
	return s.count(id, window, func(st *Status) []time.Time { return st.GrabTimes })
}

func (s *Statuses) count(id string, window time.Duration, times func(*Status) []time.Time) (int, time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.m[id]
	if st == nil {
		return 0, time.Time{}
	}
	now := s.now()
	n, first := 0, time.Time{}
	for _, t := range times(st) {
		if now.Sub(t) <= window {
			if n == 0 {
				first = t.Add(window)
			}
			n++
		}
	}
	return n, first
}

// Remove forgets the statuses of indexers not in keep.
func (s *Statuses) Remove(keep func(id string) bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := len(s.m)
	maps.DeleteFunc(s.m, func(id string, _ *Status) bool { return !keep(id) })
	if len(s.m) != n {
		s.save()
	}
}

func (s *Statuses) status(id string) *Status {
	st := s.m[id]
	if st == nil {
		st = &Status{}
		s.m[id] = st
	}
	return st
}

// save writes the statuses to the file, if any; failures only lose them
// on restart.
func (s *Statuses) save() {
	if s.file == "" {
		return
	}
	data, err := json.Marshal(s.m)
	if err != nil {
		return
	}
	tmp := s.file + ".tmp"
	if err := os.MkdirAll(filepath.Dir(s.file), 0o755); err != nil {
		return
	}
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return
	}
	_ = os.Rename(tmp, s.file)
}
