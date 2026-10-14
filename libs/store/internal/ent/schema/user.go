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

// User holds the schema of core.User.
type User struct{ ent.Schema }

// Annotations of User.
func (User) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "users"}}
}

// Fields of User.
func (User) Fields() []ent.Field {
	return []ent.Field{
		idField(),
		field.String("name").NotEmpty(),
		// name_key is the lower-cased name; it enforces case-insensitive
		// uniqueness portably.
		field.String("name_key").Unique(),
		field.String("kind").GoType(core.UserKind("")),
		field.String("auth_provider").Default(""),
		field.String("restriction").GoType(core.RestrictionProfile("")).Default(""),
		field.Bool("disabled").Default(false),
		field.JSON("policy", core.UserPolicy{}),
		field.JSON("preferences", core.UserPreferences{}),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("last_login_at").Optional().Nillable(),
	}
}

// Indexes of User.
func (User) Indexes() []ent.Index {
	return []ent.Index{
		// A server has exactly one administrator.
		index.Fields("kind").Unique().Annotations(entsql.IndexWhere("kind = 'administrator'")),
	}
}

// Edges of User.
func (User) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("user_data", UserData.Type).Annotations(cascade()),
		edge.To("auth_sessions", AuthSession.Type).Annotations(cascade()),
		edge.To("credentials", Credential.Type).Annotations(cascade()),
		edge.To("playlists", Item.Type).Annotations(cascade()),
		edge.To("display_preferences", DisplayPreferences.Type).Annotations(cascade()),
		edge.To("api_keys", APIKey.Type).Annotations(cascade()),
		edge.To("requests", Request.Type).Annotations(cascade()),
		edge.To("issues", Issue.Type).Annotations(cascade()),
		edge.To("issue_comments", IssueComment.Type).Annotations(cascade()),
		edge.To("blocked_titles", BlockedTitle.Type).Annotations(entsql.Annotation{OnDelete: entsql.SetNull}),
	}
}

// Credential holds the schema of core.Credential.
type Credential struct{ ent.Schema }

// Annotations of Credential.
func (Credential) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "credentials"}}
}

// Fields of Credential.
func (Credential) Fields() []ent.Field {
	return []ent.Field{
		idField(),
		field.UUID("user_id", core.ID{}),
		field.String("kind").GoType(core.CredentialKind("")),
		field.String("name").Default(""),
		field.String("data").Sensitive(),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("last_used_at").Optional().Nillable(),
	}
}

// Edges of Credential.
func (Credential) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Ref("credentials").Field("user_id").Unique().Required(),
	}
}

// Indexes of Credential.
func (Credential) Indexes() []ent.Index {
	return []ent.Index{
		// A user has one credential of each kind but passkeys.
		index.Fields("user_id", "kind").Unique().Annotations(entsql.IndexWhere("kind <> 'passkey'")),
	}
}

// UserData holds the schema of core.UserData.
type UserData struct{ ent.Schema }

// Annotations of UserData.
func (UserData) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "user_data"}}
}

// Fields of UserData.
func (UserData) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("user_id", core.ID{}),
		field.UUID("item_id", core.ID{}),
		field.Bool("played").Default(false),
		field.Int("play_count").Default(0),
		field.Int64("position").GoType(time.Duration(0)).Default(0),
		field.Int("audio_stream").Optional().Nillable(),
		field.Int("subtitle_stream").Optional().Nillable(),
		field.Bool("favorite").Default(false),
		field.Float("rating").Optional().Nillable(),
		field.Time("last_played_at").Optional().Nillable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

// Edges of UserData.
func (UserData) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Ref("user_data").Field("user_id").Unique().Required(),
		edge.From("item", Item.Type).Ref("user_data").Field("item_id").Unique().Required(),
	}
}

// Indexes of UserData.
func (UserData) Indexes() []ent.Index {
	return []ent.Index{index.Fields("user_id", "item_id").Unique()}
}
