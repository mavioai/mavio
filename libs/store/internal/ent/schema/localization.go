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

// ItemLocalization holds the schema of core.ItemLocalization.
type ItemLocalization struct{ ent.Schema }

// Annotations of ItemLocalization.
func (ItemLocalization) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "item_localizations"}}
}

// Fields of ItemLocalization.
func (ItemLocalization) Fields() []ent.Field {
	return []ent.Field{
		idField(),
		field.UUID("item_id", core.ID{}),
		field.String("language"),
		field.String("name").Default(""),
		field.Text("overview").Default(""),
		field.String("tagline").Default(""),
		// search_key and sort_key are the name's clean and sort forms,
		// which search matches as the item's.
		field.String("search_key").Default(""),
		field.String("sort_key").Default(""),
		field.Time("fetched_at"),
	}
}

// Edges of ItemLocalization.
func (ItemLocalization) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("item", Item.Type).Ref("localizations").Field("item_id").Unique().Required(),
	}
}

// Indexes of ItemLocalization.
func (ItemLocalization) Indexes() []ent.Index {
	return []ent.Index{index.Fields("item_id", "language").Unique()}
}
