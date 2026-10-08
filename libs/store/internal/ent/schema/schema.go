// Package schema defines the ent schema of the Mavio database. It is the
// single source of truth for the tables; Atlas derives the SQLite and
// PostgreSQL migrations from it.
package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/field"

	"github.com/mavioai/mavio/libs/core"
)

// idField is the UUIDv7 primary key shared by all entities.
func idField() ent.Field {
	return field.UUID("id", core.ID{}).Default(core.NewID).Immutable()
}

// cascade deletes rows when the referenced row is deleted.
func cascade() entsql.Annotation {
	return entsql.Annotation{OnDelete: entsql.Cascade}
}
