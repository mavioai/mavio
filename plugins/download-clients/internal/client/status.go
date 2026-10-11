package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"sync"
	"time"
)

// backoffPeriods are how long a client is left out after failing, by
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

// startupGrace is how long after starting failures leave a client out
// for at most backoffPeriods[2].
const startupGrace = 15 * time.Minute

// ClientStatus is what is kept of a client between calls.
type ClientStatus struct {
	InitialFailure    time.Time `json:"initial_failure,omitzero"`
	MostRecentFailure time.Time `json:"most_recent_failure,omitzero"`
	EscalationLevel   int       `json:"escalation_level,omitempty"`
	DisabledUntil     time.Time `json:"disabled_until,omitzero"`
	// The failure that disabled the client.
	Failure string `json:"failure,omitempty"`
}

// Statuses keeps the statuses of clients by ID: their backoff after
// failures, as Radarr's DownloadClientStatusService escalates it, saved to
// a file when one is given.
type Statuses struct {
	mu      sync.Mutex
	file    string
	now     func() time.Time
	started time.Time
	m       map[string]*ClientStatus
}

// NewStatuses returns the statuses saved in file, or none when the file
// is "" or missing. now is the clock.
func NewStatuses(file string, now func() time.Time) (*Statuses, error) {
	s := &Statuses{file: file, now: now, started: now(), m: map[string]*ClientStatus{}}
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

// Get returns a copy of a client's status.
func (s *Statuses) Get(id string) ClientStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.m[id]
	if st == nil {
		return ClientStatus{}
	}
	return *st
}

// Disabled returns until when the client is left out, if it is.
func (s *Statuses) Disabled(id string) (until time.Time, failure string, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.m[id]
	if st == nil || !st.DisabledUntil.After(s.now()) {
		return time.Time{}, "", false
	}
	return st.DisabledUntil, st.Failure, true
}

// RecordSuccess lowers the client's escalation and enables it again.
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

// RecordFailure leaves the client out for longer each time it fails in a
// row.
func (s *Statuses) RecordFailure(id string, failure string) {
	if id == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.status(id)
	now := s.now()
	st.MostRecentFailure = now
	escalate := true
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
	st.DisabledUntil = now.Add(backoffPeriods[min(maxLevel, st.EscalationLevel)])
	if inStartupGrace {
		if limit := now.Add(backoffPeriods[2]); limit.Before(st.DisabledUntil) {
			st.DisabledUntil = limit
		}
	}
	s.save()
}

// Remove forgets the statuses of clients not in keep.
func (s *Statuses) Remove(keep func(id string) bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := len(s.m)
	maps.DeleteFunc(s.m, func(id string, _ *ClientStatus) bool { return !keep(id) })
	if len(s.m) != n {
		s.save()
	}
}

func (s *Statuses) status(id string) *ClientStatus {
	st := s.m[id]
	if st == nil {
		st = &ClientStatus{}
		s.m[id] = st
	}
	return st
}

// save writes the statuses to the file, if any; failures only lose them
// on restart.
func (s *Statuses) save() {
	if s.file != "" {
		writeJSON(s.file, s.m)
	}
}
