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

// QualityDefinition holds the schema of core.QualityDefinition.
type QualityDefinition struct{ ent.Schema }

// Annotations of QualityDefinition.
func (QualityDefinition) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "quality_definitions"}}
}

// Fields of QualityDefinition.
func (QualityDefinition) Fields() []ent.Field {
	return []ent.Field{
		idField(),
		field.String("kind"),
		field.Int("quality"),
		field.String("title"),
		field.Float("min_size").Default(0),
		field.Float("max_size").Default(0),
		field.Float("preferred_size").Default(0),
	}
}

// Indexes of QualityDefinition.
func (QualityDefinition) Indexes() []ent.Index {
	return []ent.Index{index.Fields("kind", "quality").Unique()}
}

// QualityProfile holds the schema of core.QualityProfile. Its qualities
// and format scores are read and written whole, as JSON.
type QualityProfile struct{ ent.Schema }

// Annotations of QualityProfile.
func (QualityProfile) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "quality_profiles"}}
}

// Fields of QualityProfile.
func (QualityProfile) Fields() []ent.Field {
	return []ent.Field{
		idField(),
		field.String("kind"),
		field.String("name"),
		field.Bool("upgrade_allowed").Default(false),
		field.Int("cutoff").Default(0),
		field.JSON("items", []core.QualityProfileItem{}),
		field.Int("min_format_score").Default(0),
		field.Int("cutoff_format_score").Default(0),
		field.Int("min_upgrade_format_score").Default(0),
		field.JSON("format_scores", []core.FormatScore{}).Optional(),
		field.String("language").Default(""),
	}
}

// Edges of QualityProfile.
func (QualityProfile) Edges() []ent.Edge {
	return []ent.Edge{
		// Profiles in use cannot be deleted.
		edge.To("wanted", Wanted.Type),
	}
}

// Indexes of QualityProfile.
func (QualityProfile) Indexes() []ent.Index {
	return []ent.Index{index.Fields("kind", "name").Unique()}
}

// CustomFormat holds the schema of core.CustomFormat.
type CustomFormat struct{ ent.Schema }

// Annotations of CustomFormat.
func (CustomFormat) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "custom_formats"}}
}

// Fields of CustomFormat.
func (CustomFormat) Fields() []ent.Field {
	return []ent.Field{
		idField(),
		field.String("name").Unique(),
		field.Bool("include_when_renaming").Default(false),
		field.JSON("specifications", []core.CustomFormatSpec{}).Optional(),
	}
}

// DelayProfile holds the schema of core.DelayProfile.
type DelayProfile struct{ ent.Schema }

// Annotations of DelayProfile.
func (DelayProfile) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "delay_profiles"}}
}

// Fields of DelayProfile.
func (DelayProfile) Fields() []ent.Field {
	return []ent.Field{
		idField(),
		field.Bool("is_default").Default(false),
		field.Int("ord").Default(0),
		field.Bool("enable_usenet").Default(false),
		field.Bool("enable_torrent").Default(false),
		field.String("preferred_protocol"),
		field.Int64("usenet_delay").GoType(time.Duration(0)).Default(0),
		field.Int64("torrent_delay").GoType(time.Duration(0)).Default(0),
		field.Bool("bypass_if_highest_quality").Default(false),
		field.Bool("bypass_if_above_custom_format_score").Default(false),
		field.Int("minimum_custom_format_score").Default(0),
		field.JSON("tags", []string{}).Optional(),
	}
}

// Indexes of DelayProfile.
func (DelayProfile) Indexes() []ent.Index {
	return []ent.Index{
		// There is one default profile.
		index.Fields("is_default").Unique().Annotations(entsql.IndexWhere("is_default")),
		index.Fields("ord"),
	}
}

// ReleaseProfile holds the schema of core.ReleaseProfile.
type ReleaseProfile struct{ ent.Schema }

// Annotations of ReleaseProfile.
func (ReleaseProfile) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "release_profiles"}}
}

// Fields of ReleaseProfile.
func (ReleaseProfile) Fields() []ent.Field {
	return []ent.Field{
		idField(),
		field.String("name").Default(""),
		field.Bool("enabled").Default(false),
		field.JSON("required", []string{}).Optional(),
		field.JSON("ignored", []string{}).Optional(),
		field.String("indexer_id").Default(""),
		field.JSON("tags", []string{}).Optional(),
	}
}

// Wanted holds the schema of core.Wanted: one row per wanted item.
type Wanted struct{ ent.Schema }

// Annotations of Wanted.
func (Wanted) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "wanted"}}
}

// Fields of Wanted.
func (Wanted) Fields() []ent.Field {
	return []ent.Field{
		idField(),
		field.UUID("item_id", core.ID{}).Unique(),
		field.UUID("quality_profile_id", core.ID{}),
		field.String("minimum_availability").Default(""),
		field.String("monitor_new_items").Default(""),
		field.Bool("season_folder").Default(false),
		field.JSON("tags", []string{}).Optional(),
		field.Time("added_at"),
		field.Time("last_searched_at").Optional().Nillable(),
	}
}

// Edges of Wanted.
func (Wanted) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("item", Item.Type).Ref("wanted").Field("item_id").Unique().Required(),
		edge.From("quality_profile", QualityProfile.Type).Ref("wanted").Field("quality_profile_id").Unique().Required(),
	}
}

// Download holds the schema of core.Download.
type Download struct{ ent.Schema }

// Annotations of Download.
func (Download) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "downloads"}}
}

// Fields of Download.
func (Download) Fields() []ent.Field {
	return []ent.Field{
		idField(),
		field.String("plugin_id"),
		field.String("client_id"),
		field.String("download_id"),
		field.UUID("item_id", core.ID{}),
		field.JSON("item_ids", []core.ID{}).Optional(),
		field.String("title").Default(""),
		field.String("indexer_id").Default(""),
		field.String("indexer").Default(""),
		field.String("protocol"),
		field.JSON("quality", core.QualityRef{}),
		field.JSON("languages", []string{}).Optional(),
		field.Int64("size").Default(0),
		field.String("state"),
		field.JSON("messages", []string{}).Optional(),
		field.Time("grabbed_at"),
		field.Time("updated_at"),
	}
}

// Edges of Download.
func (Download) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("item", Item.Type).Ref("downloads").Field("item_id").Unique().Required(),
	}
}

// Indexes of Download.
func (Download) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("plugin_id", "client_id", "download_id").Unique(),
		index.Fields("item_id"),
		index.Fields("grabbed_at"),
	}
}

// HistoryEvent holds the schema of core.HistoryEvent. The parent (the
// series of an episode) is a plain ID: its episodes' events go with them.
type HistoryEvent struct{ ent.Schema }

// Annotations of HistoryEvent.
func (HistoryEvent) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "history_events"}}
}

// Fields of HistoryEvent.
func (HistoryEvent) Fields() []ent.Field {
	return []ent.Field{
		idField(),
		field.String("type"),
		field.UUID("item_id", core.ID{}),
		field.UUID("parent_id", core.ID{}).Optional().Nillable(),
		field.Time("date"),
		field.String("source_title").Default(""),
		field.JSON("quality", core.QualityRef{}),
		field.JSON("languages", []string{}).Optional(),
		field.String("download_id").Default(""),
		field.JSON("data", map[string]string{}).Optional(),
	}
}

// Edges of HistoryEvent.
func (HistoryEvent) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("item", Item.Type).Ref("history").Field("item_id").Unique().Required(),
	}
}

// Indexes of HistoryEvent.
func (HistoryEvent) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("item_id", "date"),
		index.Fields("parent_id", "date"),
		index.Fields("download_id"),
		index.Fields("date"),
	}
}

// BlocklistEntry holds the schema of core.BlocklistEntry.
type BlocklistEntry struct{ ent.Schema }

// Annotations of BlocklistEntry.
func (BlocklistEntry) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "blocklist"}}
}

// Fields of BlocklistEntry.
func (BlocklistEntry) Fields() []ent.Field {
	return []ent.Field{
		idField(),
		field.UUID("item_id", core.ID{}),
		field.JSON("item_ids", []core.ID{}).Optional(),
		field.String("source_title"),
		field.JSON("quality", core.QualityRef{}),
		field.JSON("languages", []string{}).Optional(),
		field.Time("date"),
		field.Time("published_date").Optional().Nillable(),
		field.Int64("size").Default(0),
		field.String("protocol"),
		field.String("indexer_id").Default(""),
		field.String("indexer").Default(""),
		field.JSON("indexer_flags", []string{}).Optional(),
		field.Text("message").Default(""),
		field.String("torrent_info_hash").Default(""),
	}
}

// Edges of BlocklistEntry.
func (BlocklistEntry) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("item", Item.Type).Ref("blocklist").Field("item_id").Unique().Required(),
	}
}

// Indexes of BlocklistEntry.
func (BlocklistEntry) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("item_id", "date"),
		index.Fields("date"),
	}
}

// PendingRelease holds the schema of core.PendingRelease.
type PendingRelease struct{ ent.Schema }

// Annotations of PendingRelease.
func (PendingRelease) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "pending_releases"}}
}

// Fields of PendingRelease.
func (PendingRelease) Fields() []ent.Field {
	return []ent.Field{
		idField(),
		field.UUID("item_id", core.ID{}),
		field.JSON("item_ids", []core.ID{}).Optional(),
		field.String("title").Default(""),
		field.Time("added"),
		field.String("reason"),
		// release is the JSON libs/acquire encodes the release as.
		field.Text("release"),
	}
}

// Edges of PendingRelease.
func (PendingRelease) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("item", Item.Type).Ref("pending_releases").Field("item_id").Unique().Required(),
	}
}

// Indexes of PendingRelease.
func (PendingRelease) Indexes() []ent.Index {
	return []ent.Index{index.Fields("item_id", "added")}
}

// RemotePathMapping holds the schema of core.RemotePathMapping.
type RemotePathMapping struct{ ent.Schema }

// Annotations of RemotePathMapping.
func (RemotePathMapping) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "remote_path_mappings"}}
}

// Fields of RemotePathMapping.
func (RemotePathMapping) Fields() []ent.Field {
	return []ent.Field{
		idField(),
		field.String("host"),
		field.String("remote_path"),
		field.String("local_path"),
	}
}

// Indexes of RemotePathMapping.
func (RemotePathMapping) Indexes() []ent.Index {
	return []ent.Index{index.Fields("host", "remote_path").Unique()}
}
