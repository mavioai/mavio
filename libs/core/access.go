package core

import (
	"fmt"
	"slices"
	"strings"
)

// LibraryAccess is a user's library access record (docs/accounts.md §3.3):
// the libraries it reaches, the limits on the items it sees in them, and
// whether it may download.
type LibraryAccess struct {
	// Libraries limits access to these libraries; nil means all libraries.
	Libraries []ID
	ContentLimits
	AllowDownload bool
}

// ContentLimits hide items by content rating and by label. Labels are
// tags: an item has the tags of its ancestors too, so that labelling a
// series covers its episodes, and they match case-insensitively.
type ContentLimits struct {
	// MaxParentalRating is the highest rating score shown, compared with
	// InheritedRating; nil means unrestricted, zero allows only content
	// for all ages.
	MaxParentalRating *int
	// BlockUnrated hides items without a rating under a maximum.
	BlockUnrated bool
	// AllowedLabels, when set, shows only the items with one of them.
	AllowedLabels []string
	// BlockedLabels hides the items with any of them.
	BlockedLabels []string
}

// MaxLabels is the most labels a record may allow or block.
const MaxLabels = 100

// CanAccessLibrary reports whether the record allows the library.
func (a *LibraryAccess) CanAccessLibrary(id ID) bool {
	return a.Libraries == nil || slices.Contains(a.Libraries, id)
}

// CanAccess reports whether the record allows the item: its library is
// allowed and its limits allow it.
func (a *LibraryAccess) CanAccess(it *Item) bool {
	return a.CanAccessLibrary(it.LibraryID) && a.Allows(it)
}

// Validate checks the record's labels.
func (a *LibraryAccess) Validate() error {
	for _, labels := range [][]string{a.AllowedLabels, a.BlockedLabels} {
		if len(labels) > MaxLabels {
			return fmt.Errorf("%w: more than %d labels", ErrInvalid, MaxLabels)
		}
		for _, l := range labels {
			if LabelKey(l) == "" {
				return fmt.Errorf("%w: empty label", ErrInvalid)
			}
		}
	}
	return nil
}

// Allows reports whether the limits allow the item. The rating score is
// the item's own ParentalRating, or its InheritedRating when it has none;
// labels are matched against its InheritedTags.
func (c *ContentLimits) Allows(it *Item) bool {
	if c.MaxParentalRating != nil {
		score := it.ParentalRating
		if score == nil {
			score = it.InheritedRating
		}
		switch {
		case score == nil && c.BlockUnrated:
			return false
		case score != nil && *score > *c.MaxParentalRating:
			return false
		}
	}
	has := func(label string) bool { return slices.Contains(it.InheritedTags, LabelKey(label)) }
	if len(c.AllowedLabels) > 0 && !slices.ContainsFunc(c.AllowedLabels, has) {
		return false
	}
	return !slices.ContainsFunc(c.BlockedLabels, has)
}

// LabelKey is the form tags and labels are matched in: trimmed and
// lower-cased.
func LabelKey(tag string) string { return strings.ToLower(strings.TrimSpace(tag)) }
