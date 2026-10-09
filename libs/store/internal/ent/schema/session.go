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

// AuthSession holds the schema of core.AuthSession.
type AuthSession struct{ ent.Schema }

// Annotations of AuthSession.
func (AuthSession) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "auth_sessions"}}
}

// Fields of AuthSession.
func (AuthSession) Fields() []ent.Field {
	return []ent.Field{
		idField(),
		field.UUID("user_id", core.ID{}),
		field.Bytes("token_hash").Unique().Sensitive(),
		field.String("device_id"),
		field.String("device_name").Default(""),
		field.String("client").Default(""),
		field.String("client_version").Default(""),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("last_seen_at").Default(time.Now),
	}
}

// Edges of AuthSession.
func (AuthSession) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Ref("auth_sessions").Field("user_id").Unique().Required(),
	}
}

// Indexes of AuthSession.
func (AuthSession) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "device_id").Unique(),
	}
}
