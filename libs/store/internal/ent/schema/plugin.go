package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
)

// PluginConfig holds the schema of core.PluginConfig. The configuration is
// kept as the administrator's JSON text, read and written whole.
type PluginConfig struct{ ent.Schema }

// Annotations of PluginConfig.
func (PluginConfig) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "plugin_configs"}}
}

// Fields of PluginConfig.
func (PluginConfig) Fields() []ent.Field {
	return []ent.Field{
		idField(),
		field.String("plugin_id").Unique().Immutable(),
		field.Text("config").Sensitive(),
		field.Time("updated_at").Default(time.Now),
	}
}
