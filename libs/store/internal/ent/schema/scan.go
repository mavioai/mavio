package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/mavioai/mavio/libs/core"
)

// FolderState holds the schema of core.FolderState. Entries are only ever
// read whole, so they are stored as JSON.
type FolderState struct{ ent.Schema }

// Annotations of FolderState.
func (FolderState) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "folder_states"}}
}

// Fields of FolderState.
func (FolderState) Fields() []ent.Field {
	return []ent.Field{
		idField(),
		field.UUID("library_id", core.ID{}),
		field.String("path"),
		field.Time("mod_time").Default(time.Time{}),
		field.String("file_id").Default(""),
		field.JSON("entries", []core.FolderEntry{}).Optional(),
		field.Int64("generation").Default(0),
	}
}

// Edges of FolderState.
func (FolderState) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("library", Library.Type).Ref("folder_states").Field("library_id").Unique().Required(),
	}
}

// Indexes of FolderState.
func (FolderState) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("library_id", "path").Unique(),
		index.Fields("library_id", "generation"),
	}
}
