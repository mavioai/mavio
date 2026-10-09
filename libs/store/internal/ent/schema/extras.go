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

// Trickplay holds the schema of core.Trickplay.
type Trickplay struct{ ent.Schema }

// Annotations of Trickplay.
func (Trickplay) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "trickplay"}}
}

// Fields of Trickplay.
func (Trickplay) Fields() []ent.Field {
	return []ent.Field{
		idField(),
		field.UUID("item_id", core.ID{}),
		field.Int("width"),
		field.Int("height"),
		field.Int("tile_width"),
		field.Int("tile_height"),
		field.Int("thumbnail_count"),
		field.Int64("interval").GoType(time.Duration(0)),
		field.Int("bandwidth").Default(0),
	}
}

// Edges of Trickplay.
func (Trickplay) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("item", Item.Type).Ref("trickplay").Field("item_id").Unique().Required(),
	}
}

// Indexes of Trickplay.
func (Trickplay) Indexes() []ent.Index {
	return []ent.Index{index.Fields("item_id", "width").Unique()}
}

// MediaSegment holds the schema of core.MediaSegment.
type MediaSegment struct{ ent.Schema }

// Annotations of MediaSegment.
func (MediaSegment) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "media_segments"}}
}

// Fields of MediaSegment.
func (MediaSegment) Fields() []ent.Field {
	return []ent.Field{
		idField(),
		field.UUID("item_id", core.ID{}),
		field.String("kind"),
		field.Int64("start").GoType(time.Duration(0)),
		field.Int64("end").GoType(time.Duration(0)),
		field.String("provider").Default(""),
	}
}

// Edges of MediaSegment.
func (MediaSegment) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("item", Item.Type).Ref("segments").Field("item_id").Unique().Required(),
	}
}

// Indexes of MediaSegment.
func (MediaSegment) Indexes() []ent.Index {
	return []ent.Index{index.Fields("item_id", "start")}
}
