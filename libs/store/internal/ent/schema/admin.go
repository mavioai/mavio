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

// Setting holds a group of core.ServerSettings by key, as JSON read and
// written whole.
type Setting struct{ ent.Schema }

// Annotations of Setting.
func (Setting) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "settings"}}
}

// Fields of Setting.
func (Setting) Fields() []ent.Field {
	return []ent.Field{
		idField(),
		field.String("key").Unique().Immutable(),
		field.Text("value"),
		field.Time("updated_at").Default(time.Now),
	}
}

// APIKey holds the schema of core.APIKey.
type APIKey struct{ ent.Schema }

// Annotations of APIKey.
func (APIKey) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "api_keys"}}
}

// Fields of APIKey.
func (APIKey) Fields() []ent.Field {
	return []ent.Field{
		idField(),
		field.UUID("user_id", core.ID{}),
		field.String("name").NotEmpty(),
		field.Bytes("token_hash").Unique().Sensitive(),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("last_used_at").Optional().Nillable(),
	}
}

// Edges of APIKey.
func (APIKey) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Ref("api_keys").Field("user_id").Unique().Required(),
	}
}

// Activity holds the schema of core.Activity. The user and item are kept
// as plain IDs: the log outlives them.
type Activity struct{ ent.Schema }

// Annotations of Activity.
func (Activity) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "activities"}}
}

// Fields of Activity.
func (Activity) Fields() []ent.Field {
	return []ent.Field{
		idField(),
		field.Time("time"),
		field.String("type"),
		field.String("severity"),
		field.String("title"),
		field.Text("message").Default(""),
		field.UUID("user_id", core.ID{}).Optional(),
		field.UUID("item_id", core.ID{}).Optional(),
		field.JSON("attributes", map[string]string{}).Optional(),
	}
}

// Indexes of Activity.
func (Activity) Indexes() []ent.Index {
	return []ent.Index{index.Fields("time")}
}
