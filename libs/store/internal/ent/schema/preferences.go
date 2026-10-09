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

// DisplayPreferences holds the schema of core.DisplayPreferences.
type DisplayPreferences struct{ ent.Schema }

// Annotations of DisplayPreferences.
func (DisplayPreferences) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "display_preferences"}}
}

// Fields of DisplayPreferences.
func (DisplayPreferences) Fields() []ent.Field {
	return []ent.Field{
		idField(),
		field.UUID("user_id", core.ID{}),
		field.String("client"),
		field.String("view"),
		field.JSON("values", map[string]string{}).Optional(),
		field.Time("updated_at").Default(time.Now),
	}
}

// Edges of DisplayPreferences.
func (DisplayPreferences) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Ref("display_preferences").Field("user_id").Unique().Required(),
	}
}

// Indexes of DisplayPreferences.
func (DisplayPreferences) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "client", "view").Unique(),
	}
}
