package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Library holds the schema of core.Library.
type Library struct{ ent.Schema }

// Annotations of Library.
func (Library) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "libraries"}}
}

// Fields of Library.
func (Library) Fields() []ent.Field {
	return []ent.Field{
		idField(),
		field.String("name").NotEmpty(),
		field.String("kind"),
		field.Strings("paths"),
		field.Int64("scan_interval").GoType(time.Duration(0)).Default(0),
		field.String("preferred_language").Default(""),
		field.String("metadata_country").Default(""),
		field.Bool("save_local_metadata").Default(false),
		field.Bool("auto_collections").Default(false),
		field.Bool("extract_trickplay").Default(false),
		field.Bool("extract_chapter_images").Default(false),
		field.Bool("analyze_loudness").Default(false),
		// scan_generation counts the library's scans.
		field.Int64("scan_generation").Default(0),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

// Indexes of Library.
func (Library) Indexes() []ent.Index {
	return []ent.Index{
		// The server keeps one library of each curated kind.
		index.Fields("kind").Unique().
			Annotations(entsql.IndexWhere("kind IN ('collections', 'playlists')")),
	}
}

// Edges of Library.
func (Library) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("items", Item.Type).Annotations(cascade()),
		edge.To("folder_states", FolderState.Type).Annotations(cascade()),
	}
}
