package store

import (
	"time"

	"github.com/mavioai/mavio/libs/core"
)

// Times are stored with microsecond precision and returned in UTC.

func orNow(t time.Time) time.Time {
	if t.IsZero() {
		return time.Now()
	}
	return t
}

func nonZero(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

func zeroIfNil(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return t.UTC()
}

func utcPtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

// nonZeroID returns a pointer to id, nil when it is zero.
func nonZeroID(id core.ID) *core.ID {
	if id.IsZero() {
		return nil
	}
	return &id
}
