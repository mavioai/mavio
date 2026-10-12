package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Job holds the schema of core.Job. Leasing and deduplication are
// dialect-specific SQL (sqlc); see queries/.
type Job struct{ ent.Schema }

// Annotations of Job.
func (Job) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "jobs"}}
}

// Fields of Job.
func (Job) Fields() []ent.Field {
	return []ent.Field{
		idField(),
		field.String("kind"),
		field.Bytes("payload").Optional(),
		field.String("unique_key").Default(""),
		field.String("state"),
		field.Int("priority").Default(0),
		field.Int("attempts").Default(0),
		field.Int("max_attempts"),
		field.Time("run_at"),
		field.String("lease_owner").Default(""),
		field.Time("lease_expires_at").Optional().Nillable(),
		field.Text("last_error").Default(""),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("finished_at").Optional().Nillable(),
	}
}

// Indexes of Job.
func (Job) Indexes() []ent.Index {
	return []ent.Index{
		// At most one pending job per unique key; a running job with the
		// key does not keep another from being queued, which runs after it.
		index.Fields("unique_key").Unique().
			Annotations(entsql.IndexWhere("unique_key <> '' AND state = 'pending'")),
		// The running jobs of a unique key, which keep a pending job with
		// the key from being leased.
		index.Fields("unique_key").StorageKey("job_unique_key_running").
			Annotations(entsql.IndexWhere("unique_key <> '' AND state = 'running'")),
		// Lease scans.
		index.Fields("state", "kind", "priority", "run_at"),
	}
}
