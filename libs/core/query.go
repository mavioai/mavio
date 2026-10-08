package core

import (
	"fmt"
	"slices"
)

// MaxPageSize is the largest Limit a query may request.
const MaxPageSize = 1000

// ItemQuery selects and orders items. Zero-valued fields do not filter.
type ItemQuery struct {
	// LibraryIDs restricts results to these libraries; callers apply the
	// user's library policy here.
	LibraryIDs []ID
	// ParentID restricts results to children of an item; with Recursive, to
	// all descendants.
	ParentID  ID
	Recursive bool
	Kinds     []ItemKind
	// IncludeExtras includes trailers and other extras, which are excluded by
	// default.
	IncludeExtras bool
	// Search matches names and original titles by full-text search.
	Search   string
	Genres   []string // any of
	Tags     []string // any of
	Studios  []string // any of
	PersonID ID
	YearFrom int
	YearTo   int
	// MaxRating is the maximum parental rating score; zero means
	// unrestricted. SkipUnrated also excludes items without a rating.
	MaxRating   int
	SkipUnrated bool

	// UserID enables the per-user filters and sorts below.
	UserID    ID
	Played    *bool
	Favorite  *bool
	Resumable bool // has a resume position

	Sort   []SortSpec
	Limit  int // zero means MaxPageSize
	Offset int
}

// SortField is an ordering key for item queries.
type SortField string

// Sort fields.
const (
	SortName            SortField = "name" // by SortName
	SortDateAdded       SortField = "date_added"
	SortPremiereDate    SortField = "premiere_date"
	SortProductionYear  SortField = "production_year"
	SortCommunityRating SortField = "community_rating"
	SortRuntime         SortField = "runtime"
	SortIndex           SortField = "index" // ParentIndexNumber, then IndexNumber
	SortRandom          SortField = "random"
	SortLastPlayed      SortField = "last_played" // requires UserID
	SortPlayCount       SortField = "play_count"  // requires UserID
)

// SortFields lists every sort field.
var SortFields = []SortField{
	SortName, SortDateAdded, SortPremiereDate, SortProductionYear, SortCommunityRating,
	SortRuntime, SortIndex, SortRandom, SortLastPlayed, SortPlayCount,
}

// SortSpec is one ordering key. Results are always tie-broken by ID.
type SortSpec struct {
	Field SortField
	Desc  bool
}

// Validate checks the query's consistency.
func (q *ItemQuery) Validate() error {
	switch {
	case q.Limit < 0 || q.Limit > MaxPageSize:
		return fmt.Errorf("%w: limit %d outside 0–%d", ErrInvalid, q.Limit, MaxPageSize)
	case q.Offset < 0:
		return fmt.Errorf("%w: negative offset", ErrInvalid)
	case q.Recursive && q.ParentID.IsZero():
		return fmt.Errorf("%w: recursive query without a parent", ErrInvalid)
	case q.YearFrom != 0 && q.YearTo != 0 && q.YearFrom > q.YearTo:
		return fmt.Errorf("%w: year range %d–%d is empty", ErrInvalid, q.YearFrom, q.YearTo)
	case q.UserID.IsZero() && (q.Played != nil || q.Favorite != nil || q.Resumable):
		return fmt.Errorf("%w: user filters require a user", ErrInvalid)
	}
	for _, k := range q.Kinds {
		if !k.Valid() {
			return fmt.Errorf("%w: unknown item kind %q", ErrInvalid, k)
		}
	}
	for _, s := range q.Sort {
		if !slices.Contains(SortFields, s.Field) {
			return fmt.Errorf("%w: unknown sort field %q", ErrInvalid, s.Field)
		}
		if q.UserID.IsZero() && (s.Field == SortLastPlayed || s.Field == SortPlayCount) {
			return fmt.Errorf("%w: sort by %s requires a user", ErrInvalid, s.Field)
		}
	}
	return nil
}

// PageSize returns the effective limit.
func (q *ItemQuery) PageSize() int {
	if q.Limit == 0 {
		return MaxPageSize
	}
	return q.Limit
}

// Page is one page of query results.
type Page[T any] struct {
	Items []T
	// Total is the number of results across all pages.
	Total int
}
