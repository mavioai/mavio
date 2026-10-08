package core

import (
	"fmt"
	"time"
)

// JobState is the lifecycle state of a background job.
type JobState string

// Job states.
const (
	JobPending   JobState = "pending"   // waiting for RunAt
	JobRunning   JobState = "running"   // leased by a worker
	JobSucceeded JobState = "succeeded" //
	JobFailed    JobState = "failed"    // attempts exhausted
)

// Job is a unit of background work stored in the database, such as a library
// scan or a metadata refresh. Workers lease jobs; a lease that expires
// without completion makes the job available again.
type Job struct {
	ID   ID
	Kind string // e.g. "library.scan"
	// Payload is the kind-specific argument, typically JSON.
	Payload []byte
	// UniqueKey deduplicates pending work: enqueueing a job whose key matches
	// a pending or running job is a no-op. Empty means no deduplication.
	UniqueKey   string
	State       JobState
	Priority    int // higher runs first
	Attempts    int
	MaxAttempts int
	RunAt       time.Time
	// LeaseOwner and LeaseExpiresAt are set while the job is running.
	LeaseOwner     string
	LeaseExpiresAt time.Time
	LastError      string
	CreatedAt      time.Time
	FinishedAt     *time.Time
}

// Validate checks the job's invariants for enqueueing.
func (j *Job) Validate() error {
	switch {
	case j.ID.IsZero() || j.Kind == "":
		return fmt.Errorf("%w: job requires ID and kind", ErrInvalid)
	case j.MaxAttempts < 1:
		return fmt.Errorf("%w: job %s must allow at least one attempt", ErrInvalid, j.ID)
	}
	return nil
}

// RetryDelay returns the backoff before attempt n+1 after n failed attempts:
// 30s, 1m, 2m, … capped at one hour.
func RetryDelay(attempts int) time.Duration {
	d := 30 * time.Second
	for range max(attempts-1, 0) {
		d *= 2
		if d >= time.Hour {
			return time.Hour
		}
	}
	return d
}
