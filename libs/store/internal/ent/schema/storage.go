package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// StorageJob holds the schema of core.StorageJob.
type StorageJob struct{ ent.Schema }

// Annotations of StorageJob.
func (StorageJob) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "storage_jobs"}}
}

// Fields of StorageJob.
func (StorageJob) Fields() []ent.Field {
	return []ent.Field{
		idField(),
		field.String("kind").Immutable(),
		field.String("key").Immutable(),
		field.String("remote"),
		field.String("execute_id").Optional(),
		field.Int64("job_id").Optional(),
		field.JSON("data", map[string]string{}).Optional(),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now),
	}
}

// Indexes of StorageJob.
func (StorageJob) Indexes() []ent.Index {
	return []ent.Index{index.Fields("kind", "key").Unique()}
}
