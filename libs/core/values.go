package core

import (
	"fmt"
	"slices"
)

// ValueKind names a multi-valued item attribute, or the production year.
type ValueKind string

// Value kinds.
const (
	ValueGenre  ValueKind = "genre"
	ValueTag    ValueKind = "tag"
	ValueStudio ValueKind = "studio"
	// ValueArtist covers both Artists and AlbumArtists.
	ValueArtist ValueKind = "artist"
	// ValueYear lists production years in decimal, oldest first.
	ValueYear ValueKind = "year"
)

// ValueKinds lists every value kind.
var ValueKinds = []ValueKind{ValueGenre, ValueTag, ValueStudio, ValueArtist, ValueYear}

// Valid reports whether k is a known value kind.
func (k ValueKind) Valid() bool { return slices.Contains(ValueKinds, k) }

// ItemFilter selects the items that value lists and person lists count:
// present items that are not extras, of the given libraries and kinds,
// within a rating. Zero-valued fields do not filter.
type ItemFilter struct {
	// LibraryIDs restricts to items of these libraries; callers apply the
	// user's library policy here.
	LibraryIDs []ID
	Kinds      []ItemKind
	// MaxRating and SkipUnrated filter like the ItemQuery fields.
	MaxRating   int
	SkipUnrated bool
}

func (f *ItemFilter) validate() error {
	for _, k := range f.Kinds {
		if !k.Valid() {
			return fmt.Errorf("%w: unknown item kind %q", ErrInvalid, k)
		}
	}
	return nil
}

// ValueQuery lists the distinct values of one attribute across the items
// of Items, e.g. all genres of a library, with the number of items having
// each value, optionally filtered by a search term.
type ValueQuery struct {
	Kind  ValueKind
	Items ItemFilter
	// Search filters and ranks values like ItemQuery.Search; years cannot
	// be searched.
	Search string
	Limit  int // zero means MaxPageSize
	Offset int
}

// Validate checks the query's consistency.
func (q *ValueQuery) Validate() error {
	switch {
	case !q.Kind.Valid():
		return fmt.Errorf("%w: unknown value kind %q", ErrInvalid, q.Kind)
	case q.Kind == ValueYear && q.Search != "":
		return fmt.Errorf("%w: years cannot be searched", ErrInvalid)
	case q.Limit < 0 || q.Limit > MaxPageSize:
		return fmt.Errorf("%w: limit %d outside 0–%d", ErrInvalid, q.Limit, MaxPageSize)
	case q.Offset < 0:
		return fmt.Errorf("%w: negative offset", ErrInvalid)
	}
	return q.Items.validate()
}

// ValueCount is a value with the number of items having it.
type ValueCount struct {
	Value string
	Count int
}

// PersonQuery lists the people credited on the items of Items.
type PersonQuery struct {
	Items ItemFilter
	// CreditKinds restricts to credits of these kinds, e.g. actors.
	CreditKinds []CreditKind
	// Search filters and ranks people by name like ItemQuery.Search; empty
	// lists everyone by sort name.
	Search string
	Limit  int // zero means MaxPageSize
	Offset int
}

// Validate checks the query's consistency.
func (q *PersonQuery) Validate() error {
	switch {
	case q.Limit < 0 || q.Limit > MaxPageSize:
		return fmt.Errorf("%w: limit %d outside 0–%d", ErrInvalid, q.Limit, MaxPageSize)
	case q.Offset < 0:
		return fmt.Errorf("%w: negative offset", ErrInvalid)
	}
	for _, k := range q.CreditKinds {
		if !k.Valid() {
			return fmt.Errorf("%w: unknown credit kind %q", ErrInvalid, k)
		}
	}
	return q.Items.validate()
}

// PersonCount is a person with the number of items crediting them.
type PersonCount struct {
	Person Person
	Count  int
}
