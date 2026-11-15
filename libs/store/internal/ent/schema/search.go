package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// SearchRecord holds the schema of core.SearchRecord: one row per query
// (its key) and voter, which names no user.
type SearchRecord struct{ ent.Schema }

// Annotations of SearchRecord.
func (SearchRecord) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "search_records"}}
}

// Fields of SearchRecord.
func (SearchRecord) Fields() []ent.Field {
	return []ent.Field{
		idField(),
		field.String("query").MaxLen(400),
		field.String("text").MaxLen(400),
		field.String("voter").MaxLen(64),
		field.Time("searched_at"),
	}
}

// Indexes of SearchRecord.
func (SearchRecord) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("query", "voter").Unique(),
		index.Fields("searched_at"),
	}
}
