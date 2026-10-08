package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store/internal/sqlcpg"
	"github.com/mavioai/mavio/libs/store/internal/sqlcsqlite"
)

// jobQueries is the dialect-neutral view of the sqlc job queries, expressed
// with the SQLite package's types. The PostgreSQL types have identical
// fields, so the adapter converts them directly.
type jobQueries interface {
	EnqueueJob(ctx context.Context, arg sqlcsqlite.EnqueueJobParams) (int64, error)
	LeaseJob(ctx context.Context, arg sqlcsqlite.LeaseJobParams) (sqlcsqlite.Job, error)
	GetLeasedJob(ctx context.Context, arg sqlcsqlite.GetLeasedJobParams) (sqlcsqlite.Job, error)
	ExtendJob(ctx context.Context, arg sqlcsqlite.ExtendJobParams) (int64, error)
	FinishJob(ctx context.Context, arg sqlcsqlite.FinishJobParams) (int64, error)
}

type pgJobQueries struct{ q *sqlcpg.Queries }

func (p pgJobQueries) EnqueueJob(ctx context.Context, arg sqlcsqlite.EnqueueJobParams) (int64, error) {
	return p.q.EnqueueJob(ctx, sqlcpg.EnqueueJobParams(arg))
}

func (p pgJobQueries) LeaseJob(ctx context.Context, arg sqlcsqlite.LeaseJobParams) (sqlcsqlite.Job, error) {
	j, err := p.q.LeaseJob(ctx, sqlcpg.LeaseJobParams(arg))
	return sqlcsqlite.Job(j), err
}

func (p pgJobQueries) GetLeasedJob(ctx context.Context, arg sqlcsqlite.GetLeasedJobParams) (sqlcsqlite.Job, error) {
	j, err := p.q.GetLeasedJob(ctx, sqlcpg.GetLeasedJobParams(arg))
	return sqlcsqlite.Job(j), err
}

func (p pgJobQueries) ExtendJob(ctx context.Context, arg sqlcsqlite.ExtendJobParams) (int64, error) {
	return p.q.ExtendJob(ctx, sqlcpg.ExtendJobParams(arg))
}

func (p pgJobQueries) FinishJob(ctx context.Context, arg sqlcsqlite.FinishJobParams) (int64, error) {
	return p.q.FinishJob(ctx, sqlcpg.FinishJobParams(arg))
}

type jobs struct{ s *Store }

// queries binds the job queries to the current transaction, or to the
// writer connection.
func (r jobs) queries() jobQueries {
	var db sqlcsqlite.DBTX
	if r.s.tx != nil {
		db = r.s.tx
	} else {
		db = r.s.dbs[0]
	}
	if r.s.dialect == DialectPostgres {
		return pgJobQueries{sqlcpg.New(db)}
	}
	return sqlcsqlite.New(db)
}

func (r jobs) Enqueue(ctx context.Context, job *core.Job) (bool, error) {
	if job.ID.IsZero() {
		job.ID = core.NewID()
	}
	if job.MaxAttempts == 0 {
		job.MaxAttempts = 1
	}
	if err := job.Validate(); err != nil {
		return false, err
	}
	now := time.Now()
	if job.RunAt.IsZero() {
		job.RunAt = now
	}
	n, err := r.queries().EnqueueJob(ctx, sqlcsqlite.EnqueueJobParams{
		ID:          job.ID,
		Kind:        job.Kind,
		Payload:     job.Payload,
		UniqueKey:   job.UniqueKey,
		Priority:    int64(job.Priority),
		MaxAttempts: int64(job.MaxAttempts),
		RunAt:       job.RunAt,
		CreatedAt:   now,
	})
	if err != nil {
		return false, fmt.Errorf("enqueue job %s: %w", job.Kind, err)
	}
	if n == 0 {
		return false, nil
	}
	job.State, job.CreatedAt = core.JobPending, now
	return true, nil
}

func (r jobs) Lease(ctx context.Context, owner string, kinds []string, ttl time.Duration) (core.Job, error) {
	if owner == "" || len(kinds) == 0 || ttl <= 0 {
		return core.Job{}, fmt.Errorf("%w: lease requires an owner, kinds and a positive TTL", core.ErrInvalid)
	}
	now := time.Now()
	j, err := r.queries().LeaseJob(ctx, sqlcsqlite.LeaseJobParams{
		Owner:     owner,
		ExpiresAt: sql.NullTime{Time: now.Add(ttl), Valid: true},
		Kinds:     kinds,
		Now:       now,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return core.Job{}, fmt.Errorf("lease job: %w", core.ErrNotFound)
	}
	if err != nil {
		return core.Job{}, fmt.Errorf("lease job: %w", err)
	}
	return toJob(j), nil
}

func (r jobs) Extend(ctx context.Context, id core.ID, owner string, ttl time.Duration) error {
	n, err := r.queries().ExtendJob(ctx, sqlcsqlite.ExtendJobParams{
		ExpiresAt: sql.NullTime{Time: time.Now().Add(ttl), Valid: true},
		ID:        id,
		Owner:     owner,
	})
	return leaseResult(n, err, "extend job "+id.String())
}

func (r jobs) Complete(ctx context.Context, id core.ID, owner string) error {
	now := time.Now()
	n, err := r.queries().FinishJob(ctx, sqlcsqlite.FinishJobParams{
		State:      string(core.JobSucceeded),
		FinishedAt: sql.NullTime{Time: now, Valid: true},
		RunAt:      now,
		ID:         id,
		Owner:      owner,
	})
	return leaseResult(n, err, "complete job "+id.String())
}

func (r jobs) Fail(ctx context.Context, id core.ID, owner string, cause error) error {
	msg := "unknown error"
	if cause != nil {
		msg = cause.Error()
	}
	return r.s.writeTx(ctx, func(tx *Store) error {
		q := jobs{tx}.queries()
		j, err := q.GetLeasedJob(ctx, sqlcsqlite.GetLeasedJobParams{ID: id, Owner: owner})
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("fail job %s: lease not held: %w", id, core.ErrConflict)
		}
		if err != nil {
			return fmt.Errorf("fail job %s: %w", id, err)
		}
		now := time.Now()
		params := sqlcsqlite.FinishJobParams{
			State:     string(core.JobPending),
			RunAt:     now.Add(core.RetryDelay(int(j.Attempts))),
			LastError: msg,
			ID:        id,
			Owner:     owner,
		}
		if j.Attempts >= j.MaxAttempts {
			params.State = string(core.JobFailed)
			params.RunAt = j.RunAt
			params.FinishedAt = sql.NullTime{Time: now, Valid: true}
		}
		n, err := q.FinishJob(ctx, params)
		return leaseResult(n, err, "fail job "+id.String())
	})
}

// leaseResult maps an update that matched no row to a lost lease.
func leaseResult(n int64, err error, what string) error {
	switch {
	case err != nil:
		return fmt.Errorf("%s: %w", what, err)
	case n == 0:
		return fmt.Errorf("%s: lease not held: %w", what, core.ErrConflict)
	}
	return nil
}

func toJob(j sqlcsqlite.Job) core.Job {
	out := core.Job{
		ID:          j.ID,
		Kind:        j.Kind,
		Payload:     j.Payload,
		UniqueKey:   j.UniqueKey,
		State:       core.JobState(j.State),
		Priority:    int(j.Priority),
		Attempts:    int(j.Attempts),
		MaxAttempts: int(j.MaxAttempts),
		RunAt:       j.RunAt.UTC(),
		LeaseOwner:  j.LeaseOwner,
		LastError:   j.LastError,
		CreatedAt:   j.CreatedAt.UTC(),
	}
	if j.LeaseExpiresAt.Valid {
		out.LeaseExpiresAt = j.LeaseExpiresAt.Time.UTC()
	}
	if j.FinishedAt.Valid {
		t := j.FinishedAt.Time.UTC()
		out.FinishedAt = &t
	}
	return out
}
