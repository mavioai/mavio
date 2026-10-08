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

// Person holds the schema of core.Person.
type Person struct{ ent.Schema }

// Annotations of Person.
func (Person) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "people"}}
}

// Fields of Person.
func (Person) Fields() []ent.Field {
	return []ent.Field{
		idField(),
		field.String("name").NotEmpty(),
		// name_key is the lower-cased name for case-insensitive lookups.
		field.String("name_key"),
		field.String("sort_name").Default(""),
		// search_key is the clean form of the name (Jellyfin's CleanName);
		// sort_key the Jellyfin sort form of the sort name.
		field.String("search_key").Default(""),
		field.String("sort_key").Default(""),
		field.Text("overview").Default(""),
		field.Time("birth_date").Optional().Nillable(),
		field.Time("death_date").Optional().Nillable(),
		field.String("birth_place").Default(""),
		field.JSON("external_ids", map[string]string{}).Optional(),
	}
}

// Edges of Person.
func (Person) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("credits", Credit.Type).Annotations(cascade()),
		edge.To("images", Image.Type).Annotations(cascade()),
	}
}

// Indexes of Person.
func (Person) Indexes() []ent.Index {
	return []ent.Index{index.Fields("name_key")}
}

// Credit holds the schema of core.Credit.
type Credit struct{ ent.Schema }

// Annotations of Credit.
func (Credit) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "credits"}}
}

// Fields of Credit.
func (Credit) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("item_id", core.ID{}),
		field.UUID("person_id", core.ID{}),
		field.String("kind"),
		field.String("role").Default(""),
		field.Int("ord").Default(0),
	}
}

// Edges of Credit.
func (Credit) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("item", Item.Type).Ref("credits").Field("item_id").Unique().Required(),
		edge.From("person", Person.Type).Ref("credits").Field("person_id").Unique().Required(),
	}
}

// Indexes of Credit.
func (Credit) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("item_id", "ord"),
		index.Fields("person_id"),
	}
}
