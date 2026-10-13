// Package throttle keeps which subtitle providers are left out after
// failing, and until when, as Bazarr's throttled providers.
package throttle

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"sync"
	"time"
)

// countWindow and countLimit: failures of counted kinds throttle a
// provider when it fails countLimit times, each within countWindow of the
// first.
const (
	countWindow = 2 * time.Minute
	countLimit  = 5
)

// State is a throttled provider.
type State struct {
	// Reason is the kind of failure, as Bazarr's exception class.
	Reason string    `json:"reason"`
	Until  time.Time `json:"until"`
}

type count struct {
	n     int
	until time.Time
}

// Throttle keeps the throttled providers by name, saved to a file when
// one is given, and the counts of their recent failures in memory.
type Throttle struct {
	file string
	now  func() time.Time
	log  *slog.Logger

	mu     sync.Mutex
	states map[string]State
	counts map[string]count
}

// New returns the throttled providers saved in file, or none when the
// file is "" or missing. now is the clock; failures to save are logged to
// log.
func New(file string, now func() time.Time, log *slog.Logger) (*Throttle, error) {
	t := &Throttle{file: file, now: now, log: log, states: map[string]State{}, counts: map[string]count{}}
	if file == "" {
		return t, nil
	}
	data, err := os.ReadFile(file)
	if errors.Is(err, fs.ErrNotExist) {
		return t, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &t.states); err != nil {
		return nil, fmt.Errorf("read %s: %w", file, err)
	}
	return t, nil
}

// Throttled returns until when a provider is left out, if it is. A
// throttle that has passed is forgotten.
func (t *Throttle) Throttled(name string) (State, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	st, ok := t.states[name]
	if !ok {
		return State{}, false
	}
	if !t.now().Before(st.Until) {
		delete(t.states, name)
		t.save()
		return State{}, false
	}
	return st, true
}

// Fail records a provider's failure: it is throttled for d, at once or,
// when counted, once it has failed five times within two minutes. It
// reports whether the provider was throttled.
func (t *Throttle) Fail(name, reason string, d time.Duration, counted bool) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := t.now()
	if counted && !t.strike(name, now) {
		return false
	}
	t.states[name] = State{Reason: reason, Until: now.Add(d)}
	t.save()
	return true
}

// strike counts a failure, as Bazarr's throttled_count: true on the
// fifth within the window, which starts the count again.
func (t *Throttle) strike(name string, now time.Time) bool {
	c, ok := t.counts[name]
	if !ok || !c.until.After(now) {
		c = count{until: now.Add(countWindow)}
	}
	c.n++
	if c.n >= countLimit {
		delete(t.counts, name)
		return true
	}
	t.counts[name] = c
	return false
}

// Reset forgets the throttles, and the failures counted towards them, of
// the providers whose reason matches; nil resets every provider.
func (t *Throttle) Reset(match func(name, reason string) bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for name, st := range t.states {
		if match == nil || match(name, st.Reason) {
			delete(t.states, name)
			delete(t.counts, name)
		}
	}
	if match == nil {
		clear(t.counts)
	}
	t.save()
}

// Retain forgets the providers not in names, such as those no longer
// enabled.
func (t *Throttle) Retain(names func(string) bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for name := range t.states {
		if !names(name) {
			delete(t.states, name)
		}
	}
	t.save()
}

// save writes the states; it is called with mu held. A failure to save
// is logged, leaving them in memory.
func (t *Throttle) save() {
	if t.file == "" {
		return
	}
	if err := t.write(); err != nil {
		t.log.Warn("save throttled providers", "file", t.file, "err", err)
	}
}

func (t *Throttle) write() error {
	data, err := json.Marshal(t.states)
	if err != nil {
		return err
	}
	tmp := t.file + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, t.file)
}
