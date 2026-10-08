package core

import (
	"database/sql/driver"
	"fmt"
)

// Value implements driver.Valuer. IDs are stored in their canonical text
// form, which PostgreSQL accepts for uuid columns and SQLite stores as TEXT.
func (id ID) Value() (driver.Value, error) { return id.String(), nil }

// Scan implements sql.Scanner, accepting the text form or 16 raw bytes.
func (id *ID) Scan(src any) error {
	switch v := src.(type) {
	case string:
		return id.UnmarshalText([]byte(v))
	case []byte:
		if len(v) == len(id) {
			copy(id[:], v)
			return nil
		}
		return id.UnmarshalText(v)
	case nil:
		*id = NilID
		return nil
	default:
		return fmt.Errorf("%w: cannot scan %T into ID", ErrInvalid, src)
	}
}
