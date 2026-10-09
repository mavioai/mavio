package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// Library holds the schema of core.Library.
type Library struct{ ent.Schema }

// Annotations of Library.
func (Library) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "libraries"}}
}

// Fields of Library.
func (Library) Fields() []ent.Field {
	return []ent.Field{
		idField(),
		field.String("name").NotEmpty(),
		field.String("kind"),
		field.Strings("paths"),
		field.Int64("scan_interval").GoType(time.Duration(0)).Default(0),
		field.String("preferred_language").Default(""),
		field.String("metadata_country").Default(""),
		// scan_generation counts the library's scans.
		field.Int64("scan_generation").Default(0),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

// Edges of Library.
func (Library) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("items", Item.Type).Annotations(cascade()),
		edge.To("folder_states", FolderState.Type).Annotations(cascade()),
	}
}
