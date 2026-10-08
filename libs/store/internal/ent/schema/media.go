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

// MediaSource holds the schema of core.MediaSource. Streams, chapters and
// keyframes are only ever read whole, so they are stored as JSON.
type MediaSource struct{ ent.Schema }

// Annotations of MediaSource.
func (MediaSource) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "media_sources"}}
}

// Fields of MediaSource.
func (MediaSource) Fields() []ent.Field {
	return []ent.Field{
		idField(),
		field.UUID("item_id", core.ID{}),
		field.Int("ord"),
		field.String("path"),
		field.String("name").Default(""),
		field.String("container").Default(""),
		field.Int64("size").Default(0),
		field.Int64("duration").GoType(time.Duration(0)).Default(0),
		field.Int64("bitrate").Default(0),
		field.JSON("streams", []core.MediaStream{}).Optional(),
		field.JSON("chapters", []core.Chapter{}).Optional(),
		field.JSON("keyframes", []time.Duration{}).Optional(),
		field.Time("probed_at").Optional().Nillable(),
	}
}

// Edges of MediaSource.
func (MediaSource) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("item", Item.Type).Ref("media_sources").Field("item_id").Unique().Required(),
	}
}

// Indexes of MediaSource.
func (MediaSource) Indexes() []ent.Index {
	return []ent.Index{index.Fields("item_id", "ord").Unique()}
}

// Image holds the schema of core.Image. Exactly one of item_id and
// person_id is set, so deleting the owner deletes its images.
type Image struct{ ent.Schema }

// Annotations of Image.
func (Image) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "images"}}
}

// Fields of Image.
func (Image) Fields() []ent.Field {
	return []ent.Field{
		idField(),
		field.UUID("item_id", core.ID{}).Optional().Nillable(),
		field.UUID("person_id", core.ID{}).Optional().Nillable(),
		field.String("kind"),
		field.Int("index").Default(0),
		field.String("path").Default(""),
		field.String("remote_url").Default(""),
		field.Int("width").Default(0),
		field.Int("height").Default(0),
		field.String("blurhash").Default(""),
		field.Bytes("thumbhash").Optional(),
	}
}

// Edges of Image.
func (Image) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("item", Item.Type).Ref("images").Field("item_id").Unique(),
		edge.From("person", Person.Type).Ref("images").Field("person_id").Unique(),
	}
}

// Indexes of Image.
func (Image) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("item_id", "kind", "index"),
		index.Fields("person_id", "kind", "index"),
	}
}
