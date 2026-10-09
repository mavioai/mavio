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
		// sort_key is the Jellyfin-style sort key of the sort name (articles
		// and punctuation removed, numbers padded, transliterated); it orders
		// names and matches the sort form of search terms.
		field.String("sort_key"),
		field.String("original_title").Default(""),
		// search_key is the clean form of the name (Jellyfin's CleanName),
		// matched and ranked by search; original_key is the lower-cased
		// original title without diacritics, matched by raw search terms.
		field.String("search_key").Default(""),
		field.String("original_key").Default(""),
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
		field.String("custom_rating").Default(""),
		field.Int("parental_rating").Default(0),
		// inherited_rating is parental_rating, or for an unrated item its
		// nearest rated ancestor's; rating filters compare it.
		field.Int("inherited_rating").Default(0),
		field.Float("community_rating").Default(0),
		field.Float("critic_rating").Default(0),
		field.JSON("external_ids", map[string]string{}).Optional(),
		field.JSON("production_locations", []string{}).Optional(),
		field.JSON("remote_trailers", []string{}).Optional(),
		field.String("collection_name").Default(""),
		field.String("aspect_ratio").Default(""),
		field.String("video_3d_format").Default(""),
		field.String("album").Default(""),
		field.String("series_status").Default(""),
		// air_days are time.Weekday values.
		field.JSON("air_days", []int{}).Optional(),
		field.String("air_time").Default(""),
		field.String("display_order").Default(""),
		field.Int("airs_before_season_number").Optional().Nillable(),
		field.Int("airs_after_season_number").Optional().Nillable(),
		field.Int("airs_before_episode_number").Optional().Nillable(),
		field.String("metadata_language").Default(""),
		field.String("metadata_country").Default(""),
		field.Bool("locked").Default(false),
		field.JSON("locked_fields", []string{}).Optional(),
		field.String("extra").Default(""),
		field.UUID("owner_id", core.ID{}).Optional().Nillable(),
		// user_id is the user a playlist belongs to.
		field.UUID("user_id", core.ID{}).Optional().Nillable(),
		field.Time("date_added"),
		field.Time("file_modified").Optional().Nillable(),
		field.Time("metadata_refreshed_at").Optional().Nillable(),
		field.Int64("scan_generation").Default(0),
		field.Time("missing_since").Optional().Nillable(),
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
		edge.From("user", User.Type).Ref("playlists").Field("user_id").Unique(),
		// links are the entries of a collection or playlist; linked_in the
		// entries linking an item.
		edge.To("links", ItemLink.Type).Annotations(cascade()),
		edge.To("linked_in", ItemLink.Type).Annotations(cascade()),
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
		index.Fields("library_id", "scan_generation"),
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
		// value_key is the clean form of the value (Jellyfin's CleanValue),
		// for search, filters and grouping; sort_key is its sort form.
		field.String("value_key").Default(""),
		field.String("sort_key").Default(""),
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
		index.Fields("kind", "value_key"),
	}
}

// ItemLink is an entry of a collection or playlist.
type ItemLink struct{ ent.Schema }

// Annotations of ItemLink.
func (ItemLink) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "item_links"}}
}

// Fields of ItemLink.
func (ItemLink) Fields() []ent.Field {
	return []ent.Field{
		idField(),
		field.UUID("container_id", core.ID{}),
		field.UUID("item_id", core.ID{}),
		// Position within the container.
		field.Int("ord"),
	}
}

// Edges of ItemLink.
func (ItemLink) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("container", Item.Type).Ref("links").Field("container_id").Unique().Required(),
		edge.From("item", Item.Type).Ref("linked_in").Field("item_id").Unique().Required(),
	}
}

// Indexes of ItemLink.
func (ItemLink) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("container_id", "ord"),
		index.Fields("item_id"),
	}
}
