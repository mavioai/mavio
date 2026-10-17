package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
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

// PluginContainer holds the schema of core.PluginContainer.
type PluginContainer struct{ ent.Schema }

// Annotations of PluginContainer.
func (PluginContainer) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "plugin_containers"}}
}

// Fields of PluginContainer.
func (PluginContainer) Fields() []ent.Field {
	return []ent.Field{
		idField(),
		field.String("plugin_id").Immutable(),
		field.String("name").Immutable(),
		field.String("mode"),
		field.String("address").Default(""),
		field.JSON("secrets", map[string]string{}).Optional().Sensitive(),
		field.Bool("use_gpu").Default(false),
		field.String("proxy").Default("system"),
		field.Time("updated_at").Default(time.Now),
	}
}

// Indexes of PluginContainer.
func (PluginContainer) Indexes() []ent.Index {
	return []ent.Index{index.Fields("plugin_id", "name").Unique()}
}
