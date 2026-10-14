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

// SubtitleProfile holds the schema of core.SubtitleProfile. Its items are
// read and written whole, as JSON.
type SubtitleProfile struct{ ent.Schema }

// Annotations of SubtitleProfile.
func (SubtitleProfile) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "subtitle_profiles"}}
}

// Fields of SubtitleProfile.
func (SubtitleProfile) Fields() []ent.Field {
	return []ent.Field{
		idField(),
		field.String("name").Unique(),
		field.JSON("items", []core.SubtitleProfileItem{}),
		field.Int("cutoff").Default(0),
		field.JSON("must_contain", []string{}).Optional(),
		field.JSON("must_not_contain", []string{}).Optional(),
		field.Bool("original_format").Default(false),
		field.String("tag").Default(""),
	}
}

// Edges of SubtitleProfile.
func (SubtitleProfile) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("assignments", SubtitleProfileAssignment.Type).Annotations(cascade()),
	}
}

// SubtitleProfileAssignment holds the schema of
// core.SubtitleProfileAssignment: its target is a library or an item.
type SubtitleProfileAssignment struct{ ent.Schema }

// Annotations of SubtitleProfileAssignment.
func (SubtitleProfileAssignment) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "subtitle_profile_assignments"}}
}

// Fields of SubtitleProfileAssignment.
func (SubtitleProfileAssignment) Fields() []ent.Field {
	return []ent.Field{
		idField(),
		field.UUID("library_id", core.ID{}).Optional().Nillable().Unique(),
		field.UUID("item_id", core.ID{}).Optional().Nillable().Unique(),
		// A nil profile assigns none.
		field.UUID("profile_id", core.ID{}).Optional().Nillable(),
	}
}

// Edges of SubtitleProfileAssignment.
func (SubtitleProfileAssignment) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("library", Library.Type).Ref("subtitle_profile").Field("library_id").Unique(),
		edge.From("item", Item.Type).Ref("subtitle_profile").Field("item_id").Unique(),
		edge.From("profile", SubtitleProfile.Type).Ref("assignments").Field("profile_id").Unique(),
	}
}

// SubtitleHistoryEvent holds the schema of core.SubtitleHistoryEvent. The
// parent (the series of an episode) and the event upgraded from are plain
// IDs.
type SubtitleHistoryEvent struct{ ent.Schema }

// Annotations of SubtitleHistoryEvent.
func (SubtitleHistoryEvent) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "subtitle_history"}}
}

// Fields of SubtitleHistoryEvent.
func (SubtitleHistoryEvent) Fields() []ent.Field {
	return []ent.Field{
		idField(),
		field.String("action"),
		field.UUID("item_id", core.ID{}),
		field.UUID("parent_id", core.ID{}).Optional().Nillable(),
		field.Time("date"),
		field.String("language"),
		field.Bool("forced").Default(false),
		field.Bool("hearing_impaired").Default(false),
		field.String("provider").Default(""),
		field.String("subtitle_id").Default(""),
		field.Int("score").Default(0),
		field.Int("max_score").Default(0),
		field.Text("path").Default(""),
		field.Text("video_path").Default(""),
		field.Text("description").Default(""),
		field.JSON("matched", []string{}).Optional(),
		field.JSON("not_matched", []string{}).Optional(),
		field.UUID("upgraded_from", core.ID{}).Optional().Nillable(),
	}
}

// Edges of SubtitleHistoryEvent.
func (SubtitleHistoryEvent) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("item", Item.Type).Ref("subtitle_history").Field("item_id").Unique().Required(),
	}
}

// Indexes of SubtitleHistoryEvent.
func (SubtitleHistoryEvent) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("item_id", "date"),
		index.Fields("parent_id", "date"),
		index.Fields("date"),
	}
}

// SubtitleBlocklistEntry holds the schema of core.SubtitleBlocklistEntry.
type SubtitleBlocklistEntry struct{ ent.Schema }

// Annotations of SubtitleBlocklistEntry.
func (SubtitleBlocklistEntry) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "subtitle_blocklist"}}
}

// Fields of SubtitleBlocklistEntry.
func (SubtitleBlocklistEntry) Fields() []ent.Field {
	return []ent.Field{
		idField(),
		field.UUID("item_id", core.ID{}),
		field.UUID("parent_id", core.ID{}).Optional().Nillable(),
		field.String("provider"),
		field.String("subtitle_id"),
		field.String("language"),
		field.Bool("forced").Default(false),
		field.Bool("hearing_impaired").Default(false),
		field.Time("date"),
		field.Text("message").Default(""),
	}
}

// Edges of SubtitleBlocklistEntry.
func (SubtitleBlocklistEntry) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("item", Item.Type).Ref("subtitle_blocklist").Field("item_id").Unique().Required(),
	}
}

// Indexes of SubtitleBlocklistEntry.
func (SubtitleBlocklistEntry) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("item_id", "date"),
		index.Fields("date"),
	}
}

// SubtitleSearchAttempt holds the schema of core.SubtitleSearchAttempt.
type SubtitleSearchAttempt struct{ ent.Schema }

// Annotations of SubtitleSearchAttempt.
func (SubtitleSearchAttempt) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "subtitle_search_attempts"}}
}

// Fields of SubtitleSearchAttempt.
func (SubtitleSearchAttempt) Fields() []ent.Field {
	return []ent.Field{
		idField(),
		field.UUID("item_id", core.ID{}),
		field.String("language"),
		field.Bool("forced").Default(false),
		field.Bool("hearing_impaired").Default(false),
		field.Time("first"),
		field.Time("last"),
	}
}

// Edges of SubtitleSearchAttempt.
func (SubtitleSearchAttempt) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("item", Item.Type).Ref("subtitle_search_attempts").Field("item_id").Unique().Required(),
	}
}

// Indexes of SubtitleSearchAttempt.
func (SubtitleSearchAttempt) Indexes() []ent.Index {
	return []ent.Index{index.Fields("item_id", "language", "forced", "hearing_impaired").Unique()}
}

// SubtitleWanted holds the schema of core.SubtitleWanted, one row per
// item. The parent (the series of an episode) is a plain ID; the missing
// languages are read and written whole, as JSON.
type SubtitleWanted struct{ ent.Schema }

// Annotations of SubtitleWanted.
func (SubtitleWanted) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "subtitle_wanted"}}
}

// Fields of SubtitleWanted.
func (SubtitleWanted) Fields() []ent.Field {
	return []ent.Field{
		idField(),
		field.UUID("item_id", core.ID{}).Unique(),
		field.String("kind"),
		field.UUID("library_id", core.ID{}),
		field.UUID("parent_id", core.ID{}).Optional().Nillable(),
		field.Text("path"),
		field.JSON("missing", []core.SubtitleLanguage{}),
		field.Time("updated"),
	}
}

// Edges of SubtitleWanted.
func (SubtitleWanted) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("item", Item.Type).Ref("subtitle_wanted").Field("item_id").Unique().Required(),
	}
}

// Indexes of SubtitleWanted.
func (SubtitleWanted) Indexes() []ent.Index {
	return []ent.Index{index.Fields("kind", "library_id", "path")}
}
