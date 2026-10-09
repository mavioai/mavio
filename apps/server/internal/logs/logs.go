// Package logs keeps the server's recent log records in memory, for
// administrators to read through the API, while passing every record on to
// the server's log.
package logs

import (
	"context"
	"log/slog"
	"slices"
	"sync"
	"time"
)

// Record is a kept log record.
type Record struct {
	Time       time.Time
	Level      slog.Level
	Message    string
	Attributes map[string]string
}

// Ring keeps the last records.
type Ring struct {
	mu      sync.Mutex
	records []Record
	next    int
	full    bool
}

// NewRing keeps up to size records.
func NewRing(size int) *Ring { return &Ring{records: make([]Record, size)} }

func (r *Ring) add(rec Record) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.records[r.next] = rec
	r.next = (r.next + 1) % len(r.records)
	r.full = r.full || r.next == 0
}

// Records returns the kept records at least as severe as min and no older
// than since, newest first, at most limit.
func (r *Ring) Records(min slog.Level, since time.Time, limit int) []Record {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := r.next
	if r.full {
		n = len(r.records)
	}
	var out []Record
	for i := 1; i <= n && len(out) < limit; i++ {
		rec := r.records[(r.next-i+len(r.records))%len(r.records)]
		if rec.Level >= min && !rec.Time.Before(since) {
			out = append(out, rec)
		}
	}
	return out
}

// Handler passes records to next and keeps those at least as severe as
// level in a ring.
func (r *Ring) Handler(next slog.Handler, level slog.Leveler) slog.Handler {
	return &handler{next: next, ring: r, level: level}
}

type handler struct {
	next   slog.Handler
	ring   *Ring
	level  slog.Leveler
	attrs  []slog.Attr
	groups []string
}

func (h *handler) Enabled(ctx context.Context, l slog.Level) bool {
	return l >= h.level.Level() || h.next.Enabled(ctx, l)
}

func (h *handler) Handle(ctx context.Context, rec slog.Record) error {
	if rec.Level >= h.level.Level() {
		attrs := map[string]string{}
		prefix := ""
		for _, g := range h.groups {
			prefix += g + "."
		}
		for _, a := range h.attrs {
			flatten(attrs, "", a)
		}
		rec.Attrs(func(a slog.Attr) bool {
			flatten(attrs, prefix, a)
			return true
		})
		h.ring.add(Record{Time: rec.Time, Level: rec.Level, Message: rec.Message, Attributes: attrs})
	}
	if h.next.Enabled(ctx, rec.Level) {
		return h.next.Handle(ctx, rec)
	}
	return nil
}

func flatten(out map[string]string, prefix string, a slog.Attr) {
	v := a.Value.Resolve()
	if v.Kind() == slog.KindGroup {
		for _, g := range v.Group() {
			flatten(out, prefix+a.Key+".", g)
		}
		return
	}
	out[prefix+a.Key] = v.String()
}

func (h *handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	c := *h
	prefix := ""
	for _, g := range h.groups {
		prefix += g + "."
	}
	c.attrs = slices.Clone(h.attrs)
	for _, a := range attrs {
		c.attrs = append(c.attrs, slog.Attr{Key: prefix + a.Key, Value: a.Value})
	}
	c.next = h.next.WithAttrs(attrs)
	return &c
}

func (h *handler) WithGroup(name string) slog.Handler {
	c := *h
	c.groups = append(slices.Clip(h.groups), name)
	c.next = h.next.WithGroup(name)
	return &c
}
