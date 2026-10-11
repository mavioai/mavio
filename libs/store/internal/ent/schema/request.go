package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/mavioai/mavio/libs/core"
)

// Request holds the schema of core.Request. The item and the user who
// modified it are plain IDs: a request outlives the item wanted for it.
type Request struct{ ent.Schema }

// Annotations of Request.
func (Request) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "requests"}}
}

// Fields of Request.
func (Request) Fields() []ent.Field {
	return []ent.Field{
		idField(),
		field.String("kind"),
		field.Int("tmdb_id"),
		field.Int("tvdb_id").Default(0),
		field.String("title").Default(""),
		field.Int("year").Default(0),
		field.UUID("item_id", core.ID{}).Optional().Nillable(),
		field.JSON("seasons", []core.SeasonRequest{}).Optional(),
		// season_count is the number of seasons, which quotas of series
		// count.
		field.Int("season_count").Default(0),
		field.String("status"),
		field.UUID("requested_by", core.ID{}),
		field.UUID("modified_by", core.ID{}).Optional().Nillable(),
		field.Bool("ignore_quota").Default(false),
		field.String("message").Default(""),
		field.Time("created_at"),
		field.Time("updated_at"),
	}
}

// Edges of Request.
func (Request) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Ref("requests").Field("requested_by").Unique().Required(),
	}
}

// Indexes of Request.
func (Request) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("requested_by", "created_at"),
		index.Fields("kind", "tmdb_id"),
		index.Fields("item_id"),
		index.Fields("status"),
		index.Fields("created_at"),
	}
}

// Issue holds the schema of core.Issue.
type Issue struct{ ent.Schema }

// Annotations of Issue.
func (Issue) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "issues"}}
}

// Fields of Issue.
func (Issue) Fields() []ent.Field {
	return []ent.Field{
		idField(),
		field.UUID("item_id", core.ID{}),
		field.String("type"),
		field.String("status"),
		field.Int("season_number").Default(0),
		field.Int("episode_number").Default(0),
		field.UUID("created_by", core.ID{}),
		field.UUID("modified_by", core.ID{}).Optional().Nillable(),
		field.Time("created_at"),
		field.Time("updated_at"),
	}
}

// Edges of Issue.
func (Issue) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("item", Item.Type).Ref("issues").Field("item_id").Unique().Required(),
		edge.From("user", User.Type).Ref("issues").Field("created_by").Unique().Required(),
		edge.To("comments", IssueComment.Type).Annotations(cascade()),
	}
}

// Indexes of Issue.
func (Issue) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("created_by", "created_at"),
		index.Fields("item_id"),
		index.Fields("status"),
		index.Fields("created_at"),
	}
}

// BlockedTitle holds the schema of core.BlockedTitle: one row per kind
// and TMDB ID; deleting its administrator keeps it, without them.
type BlockedTitle struct{ ent.Schema }

// Annotations of BlockedTitle.
func (BlockedTitle) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "blocked_titles"}}
}

// Fields of BlockedTitle.
func (BlockedTitle) Fields() []ent.Field {
	return []ent.Field{
		idField(),
		field.String("kind"),
		field.Int("tmdb_id"),
		field.String("title").Default(""),
		field.UUID("user_id", core.ID{}).Optional().Nillable(),
		// tags is NULL for titles blocklisted by hand.
		field.Strings("tags").Optional(),
		field.Time("created_at"),
	}
}

// Edges of BlockedTitle.
func (BlockedTitle) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Ref("blocked_titles").Field("user_id").Unique(),
	}
}

// Indexes of BlockedTitle.
func (BlockedTitle) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("kind", "tmdb_id").Unique(),
		index.Fields("created_at"),
	}
}

// IssueComment holds the schema of core.IssueComment.
type IssueComment struct{ ent.Schema }

// Annotations of IssueComment.
func (IssueComment) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "issue_comments"}}
}

// Fields of IssueComment.
func (IssueComment) Fields() []ent.Field {
	return []ent.Field{
		idField(),
		field.UUID("issue_id", core.ID{}),
		field.UUID("user_id", core.ID{}),
		field.Text("message"),
		field.Time("created_at"),
		field.Time("updated_at"),
	}
}

// Edges of IssueComment.
func (IssueComment) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("issue", Issue.Type).Ref("comments").Field("issue_id").Unique().Required(),
		edge.From("user", User.Type).Ref("issue_comments").Field("user_id").Unique().Required(),
	}
}

// Indexes of IssueComment.
func (IssueComment) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("issue_id", "created_at"),
	}
}
