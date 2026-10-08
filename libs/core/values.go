package core

import (
	"fmt"
	"slices"
)

// ValueKind names a multi-valued item attribute.
type ValueKind string

// Value kinds.
const (
	ValueGenre  ValueKind = "genre"
	ValueTag    ValueKind = "tag"
	ValueStudio ValueKind = "studio"
	// ValueArtist covers both Artists and AlbumArtists.
	ValueArtist ValueKind = "artist"
)

// ValueKinds lists every value kind.
var ValueKinds = []ValueKind{ValueGenre, ValueTag, ValueStudio, ValueArtist}

// Valid reports whether k is a known value kind.
func (k ValueKind) Valid() bool { return slices.Contains(ValueKinds, k) }

// ValueQuery lists the distinct values of one attribute across items, e.g.
// all genres of a library, optionally filtered by a search term.
type ValueQuery struct {
	Kind ValueKind
	// LibraryIDs restricts the values to items of these libraries.
	LibraryIDs []ID
	// Search filters and ranks values like ItemQuery.Search.
	Search string
	Limit  int // zero means MaxPageSize
}

// Validate checks the query's consistency.
func (q *ValueQuery) Validate() error {
	switch {
	case !q.Kind.Valid():
		return fmt.Errorf("%w: unknown value kind %q", ErrInvalid, q.Kind)
	case q.Limit < 0 || q.Limit > MaxPageSize:
		return fmt.Errorf("%w: limit %d outside 0–%d", ErrInvalid, q.Limit, MaxPageSize)
	}
	return nil
}

// PersonQuery searches people.
type PersonQuery struct {
	// Search filters and ranks people by name like ItemQuery.Search; empty
	// lists everyone by sort name.
	Search string
	Limit  int // zero means MaxPageSize
}

// Validate checks the query's consistency.
func (q *PersonQuery) Validate() error {
	if q.Limit < 0 || q.Limit > MaxPageSize {
		return fmt.Errorf("%w: limit %d outside 0–%d", ErrInvalid, q.Limit, MaxPageSize)
	}
	return nil
}
