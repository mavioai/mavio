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

// Item holds the schema of core.Item. Multi-valued attributes (genres, tags,
// studios, artists) live in ItemValue so they can be filtered portably.
type Item struct{ ent.Schema }

// Annotations of Item.
func (Item) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "items"}}
}

// Fields of Item.
func (Item) Fields() []ent.Field {
	return []ent.Field{
		idField(),
		field.UUID("library_id", core.ID{}),
		field.UUID("parent_id", core.ID{}).Optional().Nillable(),
		field.String("kind"),
		field.String("name"),
		field.String("sort_name"),
		// sort_key is the folded sort name used for ordering.
		field.String("sort_key"),
		field.String("original_title").Default(""),
		// search_key is the folded name and original title (lower case, no
		// diacritics, half-width) matched by substring search.
		field.String("search_key").Default(""),
		field.Text("overview").Default(""),
		field.String("tagline").Default(""),
		field.String("path").Default(""),
		field.Int("index_number").Optional().Nillable(),
		field.Int("parent_index_number").Optional().Nillable(),
		field.Int("index_number_end").Optional().Nillable(),
		field.Int("production_year").Default(0),
		field.Time("premiere_date").Optional().Nillable(),
		field.Time("end_date").Optional().Nillable(),
		field.Int64("runtime").GoType(time.Duration(0)).Default(0),
		field.String("official_rating").Default(""),
		field.Int("parental_rating").Default(0),
		field.Float("community_rating").Default(0),
		field.Float("critic_rating").Default(0),
		field.JSON("external_ids", map[string]string{}).Optional(),
		field.String("series_status").Default(""),
		field.String("extra").Default(""),
		field.UUID("owner_id", core.ID{}).Optional().Nillable(),
		field.Time("date_added"),
		field.Time("file_modified").Optional().Nillable(),
		field.Time("metadata_refreshed_at").Optional().Nillable(),
	}
}

// Edges of Item.
func (Item) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("library", Library.Type).Ref("items").Field("library_id").Unique().Required(),
		edge.To("children", Item.Type).Annotations(cascade()).
			From("parent").Field("parent_id").Unique(),
		edge.To("extras", Item.Type).Annotations(cascade()).
			From("owner").Field("owner_id").Unique(),
		edge.To("values", ItemValue.Type).Annotations(cascade()),
		edge.To("media_sources", MediaSource.Type).Annotations(cascade()),
		edge.To("images", Image.Type).Annotations(cascade()),
		edge.To("credits", Credit.Type).Annotations(cascade()),
		edge.To("user_data", UserData.Type).Annotations(cascade()),
	}
}

// Indexes of Item.
func (Item) Indexes() []ent.Index {
	return []ent.Index{
		// A path resolves to at most one item per library; virtual items have
		// no path.
		index.Fields("library_id", "path").Unique().
			Annotations(entsql.IndexWhere("path <> ''")),
		index.Fields("parent_id", "sort_key"),
		index.Fields("library_id", "kind", "sort_key"),
		index.Fields("owner_id"),
		index.Fields("date_added"),
	}
}

// ItemValue is one value of a multi-valued item attribute.
type ItemValue struct{ ent.Schema }

// Annotations of ItemValue.
func (ItemValue) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "item_values"}}
}

// Fields of ItemValue.
func (ItemValue) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("item_id", core.ID{}),
		// "genre", "tag", "studio", "artist" or "album_artist".
		field.String("kind"),
		field.String("value"),
		// Position within the item's list, to preserve order.
		field.Int("ord"),
	}
}

// Edges of ItemValue.
func (ItemValue) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("item", Item.Type).Ref("values").Field("item_id").Unique().Required(),
	}
}

// Indexes of ItemValue.
func (ItemValue) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("item_id", "kind", "ord").Unique(),
		index.Fields("kind", "value"),
	}
}
