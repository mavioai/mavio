package core

import (
	"fmt"
	"slices"
	"time"
)

// Person is someone credited on items: an actor, director, author, artist…
type Person struct {
	ID          ID
	Name        string
	SortName    string
	Overview    string
	BirthDate   *time.Time
	DeathDate   *time.Time
	BirthPlace  string
	ExternalIDs map[Provider]string
}

// Validate checks the person's invariants.
func (p *Person) Validate() error {
	if p.ID.IsZero() || p.Name == "" {
		return fmt.Errorf("%w: person requires ID and name", ErrInvalid)
	}
	return nil
}

// CreditKind is the role in which a person is credited.
type CreditKind string

// Credit kinds.
const (
	CreditActor     CreditKind = "actor"
	CreditGuestStar CreditKind = "guest_star"
	CreditDirector  CreditKind = "director"
	CreditWriter    CreditKind = "writer"
	CreditProducer  CreditKind = "producer"
	CreditCreator   CreditKind = "creator"
	CreditComposer  CreditKind = "composer"
	CreditConductor CreditKind = "conductor"
	CreditLyricist  CreditKind = "lyricist"
	CreditArtist    CreditKind = "artist"
	CreditAuthor    CreditKind = "author"
	CreditNarrator  CreditKind = "narrator"
	CreditOther     CreditKind = "other"
)

// CreditKinds lists every credit kind.
var CreditKinds = []CreditKind{
	CreditActor, CreditGuestStar, CreditDirector, CreditWriter, CreditProducer, CreditCreator,
	CreditComposer, CreditConductor, CreditLyricist, CreditArtist, CreditAuthor, CreditNarrator, CreditOther,
}

// Valid reports whether k is a known credit kind.
func (k CreditKind) Valid() bool { return slices.Contains(CreditKinds, k) }

// Credit links a person to an item.
type Credit struct {
	ItemID   ID
	PersonID ID
	Kind     CreditKind
	// Role is the character played, or a more specific job title.
	Role  string
	Order int // billing order, ascending
}

// Validate checks the credit's invariants.
func (c *Credit) Validate() error {
	switch {
	case c.ItemID.IsZero() || c.PersonID.IsZero():
		return fmt.Errorf("%w: credit requires item and person", ErrInvalid)
	case !c.Kind.Valid():
		return fmt.Errorf("%w: unknown credit kind %q", ErrInvalid, c.Kind)
	}
	return nil
}
