package core

import (
	"fmt"
	"slices"
	"time"
)

// LibraryKind determines how a library's folders are interpreted.
type LibraryKind string

// Library kinds.
const (
	LibraryMovies      LibraryKind = "movies"
	LibraryShows       LibraryKind = "shows"
	LibraryMusic       LibraryKind = "music"
	LibraryMusicVideos LibraryKind = "music_videos"
	LibraryHomeVideos  LibraryKind = "home_videos"
	LibraryBooks       LibraryKind = "books"
	LibraryPhotos      LibraryKind = "photos"
	LibraryMixed       LibraryKind = "mixed"
)

// LibraryKinds lists every library kind.
var LibraryKinds = []LibraryKind{
	LibraryMovies, LibraryShows, LibraryMusic, LibraryMusicVideos,
	LibraryHomeVideos, LibraryBooks, LibraryPhotos, LibraryMixed,
}

// Valid reports whether k is a known library kind.
func (k LibraryKind) Valid() bool { return slices.Contains(LibraryKinds, k) }

// Library is a named set of root folders scanned as one collection.
type Library struct {
	ID   ID
	Name string
	Kind LibraryKind
	// Paths are absolute root folders. A path belongs to at most one library.
	Paths []string
	// ScanInterval is the period between scheduled reconciliation scans; zero
	// disables scheduled scans.
	ScanInterval time.Duration
	// PreferredLanguage and MetadataCountry steer metadata providers, as
	// ISO 639-1 and ISO 3166-1 alpha-2 codes.
	PreferredLanguage string
	MetadataCountry   string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// Validate checks the library's invariants.
func (l *Library) Validate() error {
	switch {
	case l.Name == "":
		return fmt.Errorf("%w: library name is required", ErrInvalid)
	case !l.Kind.Valid():
		return fmt.Errorf("%w: unknown library kind %q", ErrInvalid, l.Kind)
	case len(l.Paths) == 0:
		return fmt.Errorf("%w: library %q has no paths", ErrInvalid, l.Name)
	case l.ScanInterval < 0:
		return fmt.Errorf("%w: negative scan interval", ErrInvalid)
	}
	return nil
}
