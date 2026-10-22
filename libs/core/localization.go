package core

import (
	"fmt"
	"strings"
	"time"
)

// FallbackMetadataLanguage is the language (ISO 639-1) the providers are
// asked in for items following their clients' languages: the providers'
// fullest, which the items keep and clients in other languages fall back
// to.
const FallbackMetadataLanguage = "en"

// ItemLocalization is an item's name, overview and tagline in a language
// other than the one the item keeps, as the providers give them or as
// edited; an empty field falls back to the item's.
type ItemLocalization struct {
	ItemID ID
	// Language is ISO 639-1, such as "zh".
	Language string
	Name     string
	Overview string
	Tagline  string
	// FetchedAt is when the providers were last asked.
	FetchedAt time.Time
}

// Validate checks the localization's invariants.
func (l *ItemLocalization) Validate() error {
	switch {
	case l.ItemID.IsZero():
		return fmt.Errorf("%w: localization has no item", ErrInvalid)
	case !ValidLanguage(l.Language):
		return fmt.Errorf("%w: localization of %s has language %q, not ISO 639-1", ErrInvalid, l.ItemID, l.Language)
	}
	return nil
}

// Apply puts the localization's fields over the item's, except those
// empty; the item sorts by its own name still.
func (l *ItemLocalization) Apply(it *Item) {
	if l.Name != "" {
		it.Name = l.Name
	}
	if l.Overview != "" {
		it.Overview = l.Overview
	}
	if l.Tagline != "" {
		it.Tagline = l.Tagline
	}
}

// ValidLanguage tells whether code is an ISO 639-1 language code, two
// lowercase letters.
func ValidLanguage(code string) bool {
	return len(code) == 2 && !strings.ContainsFunc(code, func(r rune) bool { return r < 'a' || r > 'z' })
}
